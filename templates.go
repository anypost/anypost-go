package anypost

import "context"

// TemplateKind is a template's authoring format. Immutable once a template exists.
type TemplateKind string

const (
	TemplateKindHTML     TemplateKind = "html"
	TemplateKindMarkdown TemplateKind = "markdown"
)

// Template is a reusable email template. The Subject/HTML/Text/Markdown fields
// hold the published content and are nil until first published. Edits land in a
// draft; Publish promotes the draft. Sends always use the published content.
type Template struct {
	// ID is the template_-prefixed id.
	ID string `json:"id"`
	// Name is the identifier, unique within the team.
	Name string `json:"name"`
	// Subject is the published subject line, nil until first published.
	Subject *string      `json:"subject"`
	Kind    TemplateKind `json:"kind"`
	// HTML is the published HTML body, nil until first published.
	HTML *string `json:"html"`
	// Text is the published, machine-derived plain-text body, nil until first
	// published.
	Text *string `json:"text"`
	// Markdown is the published emailmd source, set only for kind=markdown.
	Markdown *string `json:"markdown"`
	// HasDraft reports whether an unpublished draft is pending.
	HasDraft bool `json:"has_draft"`
	// PublishedAt is when last published, or nil if never.
	PublishedAt *string `json:"published_at"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// TemplateDraft is the unpublished draft content for a template.
type TemplateDraft struct {
	Subject *string `json:"subject"`
	HTML    *string `json:"html"`
	// Text is always machine-derived from the draft's HTML/Markdown.
	Text      *string `json:"text"`
	Markdown  *string `json:"markdown"`
	UpdatedAt string  `json:"updated_at"`
}

// TemplateCreateParams is the body for TemplatesService.Create. The new template
// starts unpublished. For kind=html supply HTML; for kind=markdown supply
// Markdown. The plain-text body is always derived server-side.
type TemplateCreateParams struct {
	Name    string  `json:"name"`
	Subject *string `json:"subject,omitempty"`
	// Kind defaults to html and is immutable once the template exists.
	Kind     TemplateKind `json:"kind,omitempty"`
	HTML     *string      `json:"html,omitempty"`
	Markdown *string      `json:"markdown,omitempty"`
}

// TemplateUpdateParams is the body for TemplatesService.Update. Only Name is
// mutable; content is draft-versioned.
type TemplateUpdateParams struct {
	Name string `json:"name"`
}

// TemplateDraftParams is the body for TemplatesService.UpdateDraft. For kind=html
// supply HTML; for kind=markdown supply Markdown.
type TemplateDraftParams struct {
	Subject  *string `json:"subject,omitempty"`
	HTML     *string `json:"html,omitempty"`
	Markdown *string `json:"markdown,omitempty"`
}

// TemplateDuplicateParams is the body for TemplatesService.Duplicate.
type TemplateDuplicateParams struct {
	// Name for the copy. Defaults to "<source name> (copy)" when omitted.
	Name string `json:"name,omitempty"`
}

// TemplatesService holds the /templates operations, including the draft/publish
// flow. Access it via Client.Templates.
type TemplatesService struct {
	http *httpClient
}

// List returns one page of the team's templates, newest-first.
func (s *TemplatesService) List(ctx context.Context, params ListParams, opts ...RequestOption) (*Page[Template], error) {
	return s.fetchPage(ctx, params, opts)
}

func (s *TemplatesService) fetchPage(ctx context.Context, params ListParams, opts []RequestOption) (*Page[Template], error) {
	q := newQuery()
	params.apply(q)
	env, err := request[pageEnvelope[Template]](ctx, s.http, "GET", "/templates", nil, false, q, opts)
	if err != nil {
		return nil, err
	}
	return newPage(*env, func(ctx context.Context, after string) (*Page[Template], error) {
		params.After = after
		return s.fetchPage(ctx, params, opts)
	}), nil
}

// Create makes a template. It starts unpublished — publish it before sending.
func (s *TemplatesService) Create(ctx context.Context, params *TemplateCreateParams, opts ...RequestOption) (*Template, error) {
	return request[Template](ctx, s.http, "POST", "/templates", params, false, nil, opts)
}

// Get retrieves a template, including its published content.
func (s *TemplatesService) Get(ctx context.Context, id string, opts ...RequestOption) (*Template, error) {
	return request[Template](ctx, s.http, "GET", "/templates/"+enc(id), nil, false, nil, opts)
}

// Update changes a template's name. Body content lives on the draft.
func (s *TemplatesService) Update(ctx context.Context, id string, params *TemplateUpdateParams, opts ...RequestOption) (*Template, error) {
	return request[Template](ctx, s.http, "PATCH", "/templates/"+enc(id), params, false, nil, opts)
}

// Delete permanently removes a template.
func (s *TemplatesService) Delete(ctx context.Context, id string, opts ...RequestOption) error {
	return requestNoContent(ctx, s.http, "DELETE", "/templates/"+enc(id), nil, nil, opts)
}

// Duplicate copies a template. The copy starts unpublished with a draft seeded
// from the source's current editable content. Pass nil params to accept the
// default name.
func (s *TemplatesService) Duplicate(ctx context.Context, id string, params *TemplateDuplicateParams, opts ...RequestOption) (*Template, error) {
	var body any
	if params != nil {
		body = params
	}
	return request[Template](ctx, s.http, "POST", "/templates/"+enc(id)+"/duplicate", body, false, nil, opts)
}

// GetDraft retrieves the template's unpublished draft. It returns a not_found
// error if none exists.
func (s *TemplatesService) GetDraft(ctx context.Context, id string, opts ...RequestOption) (*TemplateDraft, error) {
	return request[TemplateDraft](ctx, s.http, "GET", "/templates/"+enc(id)+"/draft", nil, false, nil, opts)
}

// UpdateDraft creates or updates the template's draft. Idempotent upsert;
// published content is untouched.
func (s *TemplatesService) UpdateDraft(ctx context.Context, id string, params *TemplateDraftParams, opts ...RequestOption) (*TemplateDraft, error) {
	return request[TemplateDraft](ctx, s.http, "PATCH", "/templates/"+enc(id)+"/draft", params, false, nil, opts)
}

// DeleteDraft discards the template's draft without touching published content.
func (s *TemplatesService) DeleteDraft(ctx context.Context, id string, opts ...RequestOption) error {
	return requestNoContent(ctx, s.http, "DELETE", "/templates/"+enc(id)+"/draft", nil, nil, opts)
}

// Publish promotes the draft into the published slot, consuming the draft.
func (s *TemplatesService) Publish(ctx context.Context, id string, opts ...RequestOption) (*Template, error) {
	return request[Template](ctx, s.http, "POST", "/templates/"+enc(id)+"/publish", nil, false, nil, opts)
}
