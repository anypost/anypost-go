// Package anypost is the official Go client for the Anypost email API.
//
// Create a client with an API key (or set ANYPOST_API_KEY and pass an empty
// string), then call resource methods. Every method takes a context.Context
// for cancellation and timeout.
//
//	client, err := anypost.New("ap_your_api_key")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	sent, err := client.Email.Send(ctx, &anypost.SendEmailRequest{
//	    From:    "Acme <you@yourdomain.com>",
//	    To:      []string{"someone@example.com"},
//	    Subject: "Hello",
//	    HTML:    "<p>It worked.</p>",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(sent.ID)
//
// A failed call returns an *anypost.Error; recover it with errors.As and branch
// on its Type. Keep the API key server-side; it is a bearer credential.
package anypost

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultBaseURL    = "https://api.anypost.com/v1"
	defaultTimeout    = 30 * time.Second
	defaultMaxRetries = 2

	// envAPIKey is the environment variable read when New is given an empty key.
	envAPIKey = "ANYPOST_API_KEY"
)

// Client is the entry point to the Anypost API. Construct it with New. It is
// safe for concurrent use by multiple goroutines.
type Client struct {
	// Email holds send operations (/email, /email/batch).
	Email *EmailService
	// Domains holds sending-domain operations (/domains).
	Domains *DomainsService
	// APIKeys holds API-key operations (/api-keys).
	APIKeys *APIKeysService
	// Templates holds template operations (/templates), including draft/publish.
	Templates *TemplatesService
	// Suppressions holds suppression-list operations (/suppressions).
	Suppressions *SuppressionsService
	// Webhooks holds webhook operations (/webhooks), including test and rotation.
	Webhooks *WebhooksService
	// Events holds read access to the event stream (/events).
	Events *EventsService

	identity *IdentityService
}

// Option configures a Client in New.
type Option func(*clientConfig)

type clientConfig struct {
	baseURL        string
	timeout        time.Duration
	maxRetries     int
	httpClient     *http.Client
	defaultHeaders http.Header
}

// WithBaseURL overrides the API base URL. Defaults to the production endpoint.
func WithBaseURL(url string) Option {
	return func(c *clientConfig) { c.baseURL = url }
}

// WithTimeout sets the per-request timeout. Defaults to 30s. A zero or negative
// value disables the client-imposed timeout (the context still applies).
func WithTimeout(d time.Duration) Option {
	return func(c *clientConfig) { c.timeout = d }
}

// WithMaxRetries sets the number of automatic retries for transient failures
// (429/502/503 and network errors). Defaults to 2. Set 0 to disable.
func WithMaxRetries(n int) Option {
	return func(c *clientConfig) { c.maxRetries = n }
}

// WithHTTPClient supplies a custom *http.Client. Use this to inject a transport
// (proxy, custom TLS, or a test RoundTripper). The client's own per-request
// timeout still applies on top via context.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *clientConfig) { c.httpClient = hc }
}

// WithDefaultHeader adds a header sent on every request.
func WithDefaultHeader(name, value string) Option {
	return func(c *clientConfig) {
		if c.defaultHeaders == nil {
			c.defaultHeaders = http.Header{}
		}
		c.defaultHeaders.Set(name, value)
	}
}

// New creates a Client. If apiKey is empty, it falls back to the
// ANYPOST_API_KEY environment variable; if neither is set, it returns an error.
func New(apiKey string, opts ...Option) (*Client, error) {
	key := apiKey
	if key == "" {
		key = os.Getenv(envAPIKey)
	}
	if key == "" {
		return nil, errors.New("anypost: an API key is required; pass it to New or set " + envAPIKey)
	}

	cfg := clientConfig{
		baseURL:    defaultBaseURL,
		timeout:    defaultTimeout,
		maxRetries: defaultMaxRetries,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	hc := cfg.httpClient
	if hc == nil {
		hc = &http.Client{}
	}

	core := &httpClient{
		apiKey:         key,
		baseURL:        strings.TrimRight(cfg.baseURL, "/"),
		http:           hc,
		timeout:        cfg.timeout,
		maxRetries:     cfg.maxRetries,
		defaultHeaders: cfg.defaultHeaders,
		sleep:          time.Sleep,
		jitter:         defaultJitter,
	}

	return &Client{
		Email:        &EmailService{http: core},
		Domains:      &DomainsService{http: core},
		APIKeys:      &APIKeysService{http: core},
		Templates:    &TemplatesService{http: core},
		Suppressions: &SuppressionsService{http: core},
		Webhooks:     &WebhooksService{http: core},
		Events:       &EventsService{http: core},
		identity:     &IdentityService{http: core},
	}, nil
}

// Whoami identifies the team and permission level behind the current API key.
func (c *Client) Whoami(ctx context.Context, opts ...RequestOption) (*WhoamiResponse, error) {
	return c.identity.Whoami(ctx, opts...)
}
