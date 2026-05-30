package anypost

import "context"

// SendEmailRequest is a single message to send.
//
// For a standalone Send, From and To are required, and at least one of Text,
// HTML, or TemplateID must be set (the API enforces this). As a batch entry,
// From (and any other shared field) may be omitted to inherit the batch
// Defaults — leave it zero and set EmailBatchRequest.Defaults.
type SendEmailRequest struct {
	// From is the sender address on a verified domain, bare or
	// "Display Name <addr@host>". Required for a standalone send; omit on a
	// batch entry to inherit Defaults.From.
	From string `json:"from,omitempty"`
	// To holds 1-50 primary recipients. Combined To+CC+BCC must be <= 50.
	To []string `json:"to,omitempty"`
	CC []string `json:"cc,omitempty"`
	// BCC recipients. Counts against the combined recipient cap.
	BCC []string `json:"bcc,omitempty"`
	// ReplyTo holds one address or up to 10.
	ReplyTo []string `json:"reply_to,omitempty"`
	// Subject is required unless a referenced template supplies it.
	Subject string `json:"subject,omitempty"`
	Text    string `json:"text,omitempty"`
	HTML    string `json:"html,omitempty"`
	// TemplateID references a published template (template_<uuid>). Cannot be
	// combined with inline Text/HTML.
	TemplateID string `json:"template_id,omitempty"`
	// Headers are custom message headers. At most 25 survive server-side.
	Headers map[string]string `json:"headers,omitempty"`
	// Attachments holds up to 20 inline attachments.
	Attachments []Attachment `json:"attachments,omitempty"`
	// Tags holds up to 10 free-form labels ([A-Za-z0-9_-]{1,64}).
	Tags []string `json:"tags,omitempty"`
	// Campaign is a stream-segmentation label ([A-Za-z0-9_-]{1,64}).
	Campaign string `json:"campaign,omitempty"`
	// Topic is the suppression scope / topic bucket ([a-z0-9_.-]{1,64}).
	Topic string `json:"topic,omitempty"`
	// Tracking overrides the domain's open/click defaults for this message.
	Tracking *Tracking `json:"tracking,omitempty"`
	// Variables is the Handlebars substitution map. Encoded JSON must be <= 64 KB.
	Variables map[string]any `json:"variables,omitempty"`
	// Unsubscribe controls one-click unsubscribe header injection.
	Unsubscribe *Unsubscribe `json:"unsubscribe,omitempty"`
}

// EmailBatchRequest is the body for a batch send: 1-100 messages, with optional
// batch-wide defaults.
type EmailBatchRequest struct {
	// Defaults fills any field an entry omits. To is excluded — recipients are
	// always per-entry. Reuse SendEmailRequest, leaving To zero.
	Defaults *SendEmailRequest `json:"defaults,omitempty"`
	// Emails holds the 1-100 messages in the batch.
	Emails []SendEmailRequest `json:"emails"`
}

// SendResponse is returned by a successful single send.
type SendResponse struct {
	// ID is the public message identifier (email_<uuidv7>).
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
}

// BatchSummary tallies a batch's per-entry outcomes.
type BatchSummary struct {
	Total  int `json:"total"`
	Queued int `json:"queued"`
	Failed int `json:"failed"`
}

// BatchItemError is the inner error on a failed batch entry.
type BatchItemError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// BatchItemResult is one entry's outcome in a batch send. Discriminate on
// Status: "queued" entries carry ID and CreatedAt; "failed" entries carry Error.
type BatchItemResult struct {
	Status string `json:"status"`
	// Index is the zero-based position in the request Emails slice.
	Index     int             `json:"index"`
	ID        string          `json:"id,omitempty"`
	CreatedAt string          `json:"created_at,omitempty"`
	Error     *BatchItemError `json:"error,omitempty"`
}

// BatchResponse is returned from a batch send. A mixed-outcome batch (HTTP 207)
// is a success, not an error: inspect each entry's Status. Data[i].Index == i.
type BatchResponse struct {
	Summary BatchSummary      `json:"summary"`
	Data    []BatchItemResult `json:"data"`
}

// EmailService holds the /email operations. Access it via Client.Email.
type EmailService struct {
	http *httpClient
}

// Send sends a single message. All addresses in To/CC/BCC share one envelope.
// It returns the queued message id; a failure returns an *Error.
//
// When retries are enabled and no WithIdempotencyKey is supplied, the client
// generates one so a retried send cannot deliver twice. Pass WithIdempotencyKey
// to dedupe across process restarts.
func (s *EmailService) Send(ctx context.Context, email *SendEmailRequest, opts ...RequestOption) (*SendResponse, error) {
	return request[SendResponse](ctx, s.http, "POST", "/email", email, true, nil, opts)
}

// SendBatch sends 1-100 independent messages in one request. A mixed-outcome
// batch (HTTP 207) returns normally — inspect each entry's Status in Data; it
// does not return an error.
func (s *EmailService) SendBatch(ctx context.Context, batch *EmailBatchRequest, opts ...RequestOption) (*BatchResponse, error) {
	return request[BatchResponse](ctx, s.http, "POST", "/email/batch", batch, true, nil, opts)
}
