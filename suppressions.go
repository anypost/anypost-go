package anypost

import "context"

// SuppressionReason is why an address is suppressed.
type SuppressionReason string

const (
	SuppressionReasonPermanentBounce SuppressionReason = "permanent_bounce"
	SuppressionReasonComplaint       SuppressionReason = "complaint"
	SuppressionReasonUnsubscribed    SuppressionReason = "unsubscribed"
	SuppressionReasonManual          SuppressionReason = "manual"
)

// SuppressionOrigin is the provenance of a suppression row.
type SuppressionOrigin string

const (
	SuppressionOriginAuto   SuppressionOrigin = "auto"
	SuppressionOriginManual SuppressionOrigin = "manual"
)

// Suppression is a suppressed recipient address, scoped to a topic.
type Suppression struct {
	// ID is the sup_-prefixed id, for log correlation. Lookups/deletes key on
	// (email, topic).
	ID string `json:"id"`
	// Email is the suppressed address, normalized to lowercase.
	Email string `json:"email"`
	// Topic this suppression applies to. "*" means every topic.
	Topic  string            `json:"topic"`
	Reason SuppressionReason `json:"reason"`
	Origin SuppressionOrigin `json:"origin"`
	// Classification is a bounce classification or ARF feedback-type, nil for
	// manual entries.
	Classification *string `json:"classification"`
	// SMTPCode is the SMTP reply code from the bounce, nil for complaints and
	// manual entries.
	SMTPCode *int `json:"smtp_code"`
	// Note is a free-form note attached at creation.
	Note *string `json:"note"`
	// SuppressedAt is when the suppression was first observed.
	SuppressedAt string `json:"suppressed_at"`
	// ExpiresAt is always nil: suppressions do not expire. Kept for
	// compatibility and will be removed in a future major version.
	ExpiresAt *string `json:"expires_at"`
	CreatedAt string  `json:"created_at"`
}

// SuppressionListParams are the filters for SuppressionsService.List.
type SuppressionListParams struct {
	ListParams
	// EmailContains is a case-insensitive substring match against the address.
	EmailContains string
	// Topic restricts to a topic. "*" for global entries.
	Topic  string
	Reason SuppressionReason
	Origin SuppressionOrigin
}

// SuppressionCreateParams is the body for SuppressionsService.Create.
type SuppressionCreateParams struct {
	Email string `json:"email"`
	// Topic scopes the suppression. Omit or "*" to block every topic.
	Topic string `json:"topic,omitempty"`
	// Note is an optional internal annotation, preserved across automatic
	// re-suppressions.
	Note string `json:"note,omitempty"`
}

// SuppressionBatchItem is one entry in SuppressionsService.CreateBatch.
type SuppressionBatchItem struct {
	Email string `json:"email"`
	// Topic scopes the suppression. Omit or "*" to block every topic.
	Topic string `json:"topic,omitempty"`
	// Reason is free-form. permanent_bounce, complaint, unsubscribed, and
	// manual are recorded as given, common equivalents (bounce, hard bounce,
	// spam report, opt-out) are mapped to them, and anything else records manual.
	Reason string `json:"reason,omitempty"`
	// Note is an optional internal annotation (max 2000 characters).
	Note string `json:"note,omitempty"`
}

// SuppressionBatchInvalid is an item CreateBatch rejected as not a valid address.
type SuppressionBatchInvalid struct {
	// Index is the item's 0-based position in the request.
	Index int `json:"index"`
	// Email is the submitted value, nil if it was not a string.
	Email *string `json:"email"`
}

// SuppressionBatchResult is the outcome of SuppressionsService.CreateBatch.
// Imported + AlreadySuppressed + len(Invalid) equals the number of items sent.
type SuppressionBatchResult struct {
	// Imported is the number of suppressions this call created.
	Imported int `json:"imported"`
	// AlreadySuppressed counts items whose (email, topic) was already
	// suppressed, including repeats within the batch. They are left unchanged.
	AlreadySuppressed int                       `json:"already_suppressed"`
	Invalid           []SuppressionBatchInvalid `json:"invalid"`
}

// SuppressionsService holds the /suppressions operations. Entries key on
// (email, topic). Access it via Client.Suppressions.
type SuppressionsService struct {
	http *httpClient
}

// List returns one page of the team's suppressions, newest-first.
func (s *SuppressionsService) List(ctx context.Context, params SuppressionListParams, opts ...RequestOption) (*Page[Suppression], error) {
	return s.fetchPage(ctx, params, opts)
}

func (s *SuppressionsService) fetchPage(ctx context.Context, params SuppressionListParams, opts []RequestOption) (*Page[Suppression], error) {
	q := newQuery()
	params.ListParams.apply(q)
	q.set("email_contains", params.EmailContains)
	q.set("topic", params.Topic)
	q.set("reason", string(params.Reason))
	q.set("origin", string(params.Origin))
	env, err := request[pageEnvelope[Suppression]](ctx, s.http, "GET", "/suppressions", nil, false, q, opts)
	if err != nil {
		return nil, err
	}
	return newPage(*env, func(ctx context.Context, after string) (*Page[Suppression], error) {
		params.After = after
		return s.fetchPage(ctx, params, opts)
	}), nil
}

// Create adds a manual suppression. Defaults to topic "*" (every topic). It
// returns a validation_error if an active entry for the same (email, topic)
// exists.
func (s *SuppressionsService) Create(ctx context.Context, params *SuppressionCreateParams, opts ...RequestOption) (*Suppression, error) {
	return request[Suppression](ctx, s.http, "POST", "/suppressions", params, false, nil, opts)
}

// CreateBatch adds up to 1,000 manual suppressions in one request. Entries that
// are already suppressed are counted in AlreadySuppressed and left unchanged, so
// a batch is safe to retry. Invalid addresses come back in Invalid instead of
// failing the batch. It returns a validation_error, adding nothing, for a
// malformed topic, an empty or oversized list, or a batch that would pass the
// team's suppression limit.
func (s *SuppressionsService) CreateBatch(ctx context.Context, items []SuppressionBatchItem, opts ...RequestOption) (*SuppressionBatchResult, error) {
	body := struct {
		Suppressions []SuppressionBatchItem `json:"suppressions"`
	}{Suppressions: items}
	return request[SuppressionBatchResult](ctx, s.http, "POST", "/suppressions/batch", body, false, nil, opts)
}

// Get retrieves the suppression for an (email, topic) pair. Use "*" as the topic
// for the global row. It returns a not_found error if the pair isn't suppressed.
func (s *SuppressionsService) Get(ctx context.Context, email, topic string, opts ...RequestOption) (*Suppression, error) {
	return request[Suppression](ctx, s.http, "GET", "/suppressions/"+enc(email)+"/"+enc(topic), nil, false, nil, opts)
}

// Delete removes the single (email, topic) row. Other topics are untouched.
func (s *SuppressionsService) Delete(ctx context.Context, email, topic string, opts ...RequestOption) error {
	return requestNoContent(ctx, s.http, "DELETE", "/suppressions/"+enc(email)+"/"+enc(topic), nil, nil, opts)
}

// ListForEmail returns every suppression on file for an address, across all
// topics. It returns a not_found error if the address has no active
// suppressions.
func (s *SuppressionsService) ListForEmail(ctx context.Context, email string, opts ...RequestOption) ([]Suppression, error) {
	env, err := request[struct {
		Data []Suppression `json:"data"`
	}](ctx, s.http, "GET", "/suppressions/"+enc(email), nil, false, nil, opts)
	if err != nil {
		return nil, err
	}
	return env.Data, nil
}

// DeleteForEmail removes an address from the suppression list across every topic.
func (s *SuppressionsService) DeleteForEmail(ctx context.Context, email string, opts ...RequestOption) error {
	return requestNoContent(ctx, s.http, "DELETE", "/suppressions/"+enc(email), nil, nil, opts)
}
