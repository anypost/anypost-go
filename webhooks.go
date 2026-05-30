package anypost

import "context"

// WebhookEventType is an event type a webhook can subscribe to.
type WebhookEventType string

const (
	WebhookEventSent         WebhookEventType = "email.sent"
	WebhookEventDelivered    WebhookEventType = "email.delivered"
	WebhookEventDelayed      WebhookEventType = "email.delayed"
	WebhookEventBounced      WebhookEventType = "email.bounced"
	WebhookEventComplained   WebhookEventType = "email.complained"
	WebhookEventSuppressed   WebhookEventType = "email.suppressed"
	WebhookEventUnsubscribed WebhookEventType = "email.unsubscribed"
	WebhookEventOpened       WebhookEventType = "email.opened"
	WebhookEventClicked      WebhookEventType = "email.clicked"
)

// WebhookStatus is a webhook's delivery state. Only "active" and "disabled" can
// be set through the API; "circuit_disabled" is server-managed.
type WebhookStatus string

const (
	WebhookStatusActive          WebhookStatus = "active"
	WebhookStatusDisabled        WebhookStatus = "disabled"
	WebhookStatusCircuitDisabled WebhookStatus = "circuit_disabled"
)

// Webhook is a webhook subscription. The signing secret is never returned here.
type Webhook struct {
	ID     string             `json:"id"`
	Name   string             `json:"name"`
	URL    string             `json:"url"`
	Events []WebhookEventType `json:"events"`
	Status WebhookStatus      `json:"status"`
	// SigningSecretPrefix is the first 12 characters of the signing secret.
	SigningSecretPrefix string `json:"signing_secret_prefix"`
	// SigningSecretPreviousPrefix is the prefix of the previous secret while a
	// rotation grace window is open, else nil.
	SigningSecretPreviousPrefix *string `json:"signing_secret_previous_prefix"`
	// SigningSecretGraceExpiresAt is when the rotation grace window ends, or nil.
	SigningSecretGraceExpiresAt *string `json:"signing_secret_grace_expires_at"`
	LastDeliveryAt              *string `json:"last_delivery_at"`
	CreatedAt                   string  `json:"created_at"`
}

// WebhookWithSecret is a webhook with its full signing secret. Returned only on
// create and rotate-secret.
type WebhookWithSecret struct {
	Webhook
	// SigningSecret is the full signing secret (whsec_...). Returned once; store
	// it securely.
	SigningSecret string `json:"signing_secret"`
}

// WebhookTestResult is the outcome of a synchronous test delivery. A bad
// endpoint never returns an error — read Delivered and StatusCode.
type WebhookTestResult struct {
	// Delivered is true only when the endpoint returned a 2xx status.
	Delivered bool `json:"delivered"`
	// StatusCode is the HTTP status the endpoint returned, or nil on a network
	// failure.
	StatusCode *int `json:"status_code"`
	// LatencyMS is wall-clock time from request start to response or error.
	LatencyMS int `json:"latency_ms"`
	// Error is a human-readable failure reason, or nil on success.
	Error *string `json:"error"`
	// ResponseBodyPreview is a truncated preview of the endpoint's response body.
	ResponseBodyPreview *string `json:"response_body_preview"`
}

// WebhookCreateParams is the body for WebhooksService.Create.
type WebhookCreateParams struct {
	Name string `json:"name"`
	// URL is an https:// endpoint to receive signed deliveries.
	URL string `json:"url"`
	// Events is at least one event type to subscribe to.
	Events []WebhookEventType `json:"events"`
}

// WebhookUpdateParams is the body for WebhooksService.Update.
type WebhookUpdateParams struct {
	Name   string             `json:"name"`
	URL    string             `json:"url"`
	Events []WebhookEventType `json:"events"`
	// Status sets "disabled" to pause delivery or "active" to resume.
	Status WebhookStatus `json:"status"`
}

// WebhooksService holds the /webhooks operations. Access it via Client.Webhooks.
type WebhooksService struct {
	http *httpClient
}

// List returns one page of the team's webhooks, newest-first.
func (s *WebhooksService) List(ctx context.Context, params ListParams, opts ...RequestOption) (*Page[Webhook], error) {
	return s.fetchPage(ctx, params, opts)
}

func (s *WebhooksService) fetchPage(ctx context.Context, params ListParams, opts []RequestOption) (*Page[Webhook], error) {
	q := newQuery()
	params.apply(q)
	env, err := request[pageEnvelope[Webhook]](ctx, s.http, "GET", "/webhooks", nil, false, q, opts)
	if err != nil {
		return nil, err
	}
	return newPage(*env, func(ctx context.Context, after string) (*Page[Webhook], error) {
		params.After = after
		return s.fetchPage(ctx, params, opts)
	}), nil
}

// Create makes a webhook. The full SigningSecret is on this response only —
// store it now to verify future deliveries; later reads return only the prefix.
func (s *WebhooksService) Create(ctx context.Context, params *WebhookCreateParams, opts ...RequestOption) (*WebhookWithSecret, error) {
	return request[WebhookWithSecret](ctx, s.http, "POST", "/webhooks", params, false, nil, opts)
}

// Get retrieves a webhook. The signing secret is never returned — only its prefix.
func (s *WebhooksService) Get(ctx context.Context, id string, opts ...RequestOption) (*Webhook, error) {
	return request[Webhook](ctx, s.http, "GET", "/webhooks/"+enc(id), nil, false, nil, opts)
}

// Update changes a webhook's name, URL, events, and status. It does not rotate
// the signing secret — use RotateSecret.
func (s *WebhooksService) Update(ctx context.Context, id string, params *WebhookUpdateParams, opts ...RequestOption) (*Webhook, error) {
	return request[Webhook](ctx, s.http, "PATCH", "/webhooks/"+enc(id), params, false, nil, opts)
}

// Delete permanently removes a webhook.
func (s *WebhooksService) Delete(ctx context.Context, id string, opts ...RequestOption) error {
	return requestNoContent(ctx, s.http, "DELETE", "/webhooks/"+enc(id), nil, nil, opts)
}

// Test sends one synthetic webhook.test event and reports the outcome. One-shot,
// not retried, and absent from delivery history. It returns the result even when
// the endpoint fails — read Delivered and StatusCode. Works on a disabled
// webhook too.
func (s *WebhooksService) Test(ctx context.Context, id string, opts ...RequestOption) (*WebhookTestResult, error) {
	return request[WebhookTestResult](ctx, s.http, "POST", "/webhooks/"+enc(id)+"/test", nil, false, nil, opts)
}

// RotateSecret rotates the signing secret. The new secret is on this response
// only. The previous secret stays valid for a 24h grace window. Rotating again
// before the window ends returns a webhook_rotation_in_progress conflict.
func (s *WebhooksService) RotateSecret(ctx context.Context, id string, opts ...RequestOption) (*WebhookWithSecret, error) {
	return request[WebhookWithSecret](ctx, s.http, "POST", "/webhooks/"+enc(id)+"/rotate-secret", nil, false, nil, opts)
}
