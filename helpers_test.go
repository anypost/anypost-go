package anypost

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// cannedResponse is one queued reply from the mock transport: either an HTTP
// response (status/body/header) or a transport-level error.
type cannedResponse struct {
	status int
	body   string
	header http.Header
	err    error
}

func jsonResponse(status int, body string) cannedResponse {
	return cannedResponse{status: status, body: body}
}

func jsonResponseWithHeader(status int, body string, header http.Header) cannedResponse {
	return cannedResponse{status: status, body: body, header: header}
}

func networkError(err error) cannedResponse {
	return cannedResponse{err: err}
}

// recordedRequest captures one outbound request for assertions.
type recordedRequest struct {
	method string
	url    string
	header http.Header
	body   []byte
}

// json decodes the recorded request body into a generic map.
func (r recordedRequest) json(t *testing.T) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(r.body, &out); err != nil {
		t.Fatalf("decoding recorded body: %v (body=%q)", err, r.body)
	}
	return out
}

// mockTransport replays canned responses in order and records every request.
type mockTransport struct {
	results  []cannedResponse
	idx      int
	requests []recordedRequest
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
		_ = req.Body.Close()
	}
	m.requests = append(m.requests, recordedRequest{
		method: req.Method,
		url:    req.URL.String(),
		header: req.Header.Clone(),
		body:   body,
	})

	if m.idx >= len(m.results) {
		return nil, fmt.Errorf("mock: no canned response for request #%d (%s %s)", m.idx, req.Method, req.URL)
	}
	r := m.results[m.idx]
	m.idx++
	if r.err != nil {
		return nil, r.err
	}
	header := r.header
	if header == nil {
		header = http.Header{}
	}
	return &http.Response{
		StatusCode: r.status,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(r.body)),
		Request:    req,
	}, nil
}

func (m *mockTransport) last() recordedRequest {
	return m.requests[len(m.requests)-1]
}

func (m *mockTransport) count() int { return len(m.requests) }

// newTestClient builds a Client wired to a mock transport with the default
// retry count (2). Retry sleeps are disabled and jitter is pinned to 1.0 so
// backoff is deterministic.
func newTestClient(t *testing.T, results ...cannedResponse) (*Client, *mockTransport) {
	return newTestClientWithRetries(t, defaultMaxRetries, results...)
}

// newTestClientWithRetries is newTestClient with an explicit retry count. Use 0
// to assert error mapping without consuming extra canned responses.
func newTestClientWithRetries(t *testing.T, retries int, results ...cannedResponse) (*Client, *mockTransport) {
	t.Helper()
	mock := &mockTransport{results: results}
	client, err := New("ap_test_key",
		WithBaseURL("https://api.test/v1"),
		WithHTTPClient(&http.Client{Transport: mock}),
		WithMaxRetries(retries),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Shared core: every service points at the same *httpClient. Disable real
	// sleeps and pin jitter to 1.0 so backoff is deterministic in tests.
	client.Email.http.sleep = func(time.Duration) {}
	client.Email.http.jitter = func() float64 { return 1.0 }
	return client, mock
}
