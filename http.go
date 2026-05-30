package anypost

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	mathrand "math/rand/v2"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// defaultJitter returns a value in [0, 1) for full-jitter backoff.
func defaultJitter() float64 { return mathrand.Float64() }

// enc percent-encodes a single path segment, escaping everything outside the
// RFC 3986 unreserved set (so "@", "/", "*", and the like are encoded). This
// matches the path encoding of the other Anypost SDKs.
func enc(s string) string {
	const unreserved = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_.~"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if strings.IndexByte(unreserved, c) >= 0 {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

const (
	maxBackoff  = 8 * time.Second
	baseBackoff = 500 * time.Millisecond
)

var retryableStatus = map[int]bool{
	http.StatusTooManyRequests:    true, // 429
	http.StatusBadGateway:         true, // 502
	http.StatusServiceUnavailable: true, // 503
}

// httpClient owns the transport and implements the request loop: header
// assembly, retries with full-jitter backoff, idempotency keys, and error
// mapping. It is shared by every resource.
type httpClient struct {
	apiKey         string
	baseURL        string // normalized, no trailing slash
	http           *http.Client
	timeout        time.Duration
	maxRetries     int
	defaultHeaders http.Header

	// Injectable for tests; default to time.Sleep and a real PRNG.
	sleep  func(time.Duration)
	jitter func() float64
}

// requestConfig holds the resolved per-call overrides from RequestOptions.
type requestConfig struct {
	idempotencyKey string
	headers        http.Header
}

// RequestOption overrides behavior for a single call.
type RequestOption func(*requestConfig)

// WithIdempotencyKey sets the Idempotency-Key for a send. Reusing a key with an
// identical body replays the stored result; reusing it with a different body
// fails with ErrorTypeIdempotencyMismatch. Only the send endpoints honor it.
func WithIdempotencyKey(key string) RequestOption {
	return func(c *requestConfig) { c.idempotencyKey = key }
}

// WithHeader adds (or overrides) a header on a single request.
func WithHeader(name, value string) RequestOption {
	return func(c *requestConfig) {
		if c.headers == nil {
			c.headers = http.Header{}
		}
		c.headers.Set(name, value)
	}
}

func resolveConfig(opts []RequestOption) requestConfig {
	var cfg requestConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// query is a small wrapper over url.Values that skips empty values.
type query struct{ values url.Values }

func newQuery() *query { return &query{values: url.Values{}} }

func (q *query) set(key, value string) {
	if value != "" {
		q.values.Set(key, value)
	}
}

func (q *query) setInt(key string, value int) {
	q.values.Set(key, strconv.Itoa(value))
}

func (q *query) encode() string {
	if q == nil {
		return ""
	}
	return q.values.Encode()
}

// do performs a request and returns the raw response body on success, or an
// *Error on failure. It retries 429/502/503 and transport errors up to
// maxRetries times with full-jitter exponential backoff, honoring Retry-After.
func (c *httpClient) do(ctx context.Context, method, path string, body any, idempotent bool, q *query, cfg requestConfig) ([]byte, error) {
	var payload []byte
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, &Error{Type: ErrorTypeConnection, Message: fmt.Sprintf("encoding request body: %v", err), cause: err}
		}
		payload = encoded
	}

	requestURL := c.baseURL + path
	if encoded := q.encode(); encoded != "" {
		requestURL += "?" + encoded
	}

	header := c.buildHeader(body != nil, idempotent, cfg)

	for attempt := 0; ; attempt++ {
		respBody, header2, status, err := c.attempt(ctx, method, requestURL, payload, header)
		if err != nil {
			// Transport failure: no HTTP response. Retry unless the context was
			// canceled by the caller or we are out of attempts.
			if attempt < c.maxRetries && ctx.Err() == nil {
				c.backoff(attempt, nil)
				continue
			}
			return nil, newConnectionError(connectionMessage(err), err)
		}

		if status >= 200 && status < 300 {
			return respBody, nil
		}

		if retryableStatus[status] && attempt < c.maxRetries {
			c.backoff(attempt, header2)
			continue
		}

		return nil, parseError(status, respBody, header2)
	}
}

// attempt performs one HTTP round trip with a per-request timeout derived from
// the client timeout (composed with the caller's context).
func (c *httpClient) attempt(ctx context.Context, method, requestURL string, payload []byte, header http.Header) ([]byte, http.Header, int, error) {
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	var reader io.Reader
	if payload != nil {
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, reader)
	if err != nil {
		return nil, nil, 0, err
	}
	req.Header = header.Clone()

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, 0, err
	}
	return respBody, resp.Header, resp.StatusCode, nil
}

func (c *httpClient) buildHeader(hasBody, idempotent bool, cfg requestConfig) http.Header {
	header := http.Header{}
	header.Set("Authorization", "Bearer "+c.apiKey)
	header.Set("Accept", "application/json")
	header.Set("User-Agent", userAgent())
	for name, values := range c.defaultHeaders {
		for _, v := range values {
			header.Set(name, v)
		}
	}
	if hasBody {
		header.Set("Content-Type", "application/json")
	}
	if idempotent {
		switch {
		case cfg.idempotencyKey != "":
			header.Set("Idempotency-Key", cfg.idempotencyKey)
		case c.maxRetries > 0:
			// Auto-key so built-in retries of a send cannot deliver twice.
			header.Set("Idempotency-Key", uuidV4())
		}
	}
	for name, values := range cfg.headers {
		for _, v := range values {
			header.Set(name, v)
		}
	}
	return header
}

// backoff sleeps before the next retry: Retry-After when present, otherwise
// full-jitter exponential backoff capped at maxBackoff.
func (c *httpClient) backoff(attempt int, header http.Header) {
	if header != nil {
		if after := retryAfter(header); after > 0 {
			if after > maxBackoff {
				after = maxBackoff
			}
			c.sleep(after)
			return
		}
	}
	ceiling := baseBackoff * time.Duration(1<<attempt)
	if ceiling > maxBackoff {
		ceiling = maxBackoff
	}
	c.sleep(time.Duration(c.jitter() * float64(ceiling)))
}

func connectionMessage(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "Request timed out before a response."
	}
	if errors.Is(err, context.Canceled) {
		return "Request was canceled before a response."
	}
	return "Could not reach Anypost: " + err.Error()
}

func userAgent() string {
	return fmt.Sprintf("anypost-go/%s %s", Version, runtime.Version())
}

// uuidV4 returns a random v4 UUID. Used to auto-generate idempotency keys.
func uuidV4() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand should never fail; fall back to a timestamp-derived key
		// rather than panic in a request path.
		return fmt.Sprintf("anypost-%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// request performs a call and decodes a JSON object response into *T.
func request[T any](ctx context.Context, c *httpClient, method, path string, body any, idempotent bool, q *query, opts []RequestOption) (*T, error) {
	respBody, err := c.do(ctx, method, path, body, idempotent, q, resolveConfig(opts))
	if err != nil {
		return nil, err
	}
	out := new(T)
	if len(respBody) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return nil, &Error{Type: ErrorTypeConnection, Message: fmt.Sprintf("decoding response: %v", err), cause: err}
	}
	return out, nil
}

// requestNoContent performs a call that returns no body (e.g. a 204 delete).
func requestNoContent(ctx context.Context, c *httpClient, method, path string, body any, q *query, opts []RequestOption) error {
	_, err := c.do(ctx, method, path, body, false, q, resolveConfig(opts))
	return err
}
