package anypost

import (
	"context"
	"strings"
)

// EventType is a customer-facing event type in the event stream. The same set
// is emitted via webhooks; operational events are never returned here.
type EventType string

const (
	EventSent         EventType = "email.sent"
	EventDelivered    EventType = "email.delivered"
	EventDelayed      EventType = "email.delayed"
	EventBounced      EventType = "email.bounced"
	EventComplained   EventType = "email.complained"
	EventSuppressed   EventType = "email.suppressed"
	EventUnsubscribed EventType = "email.unsubscribed"
	EventOpened       EventType = "email.opened"
	EventClicked      EventType = "email.clicked"
)

// Event is a single email-pipeline event for the team. Every field is always
// present; fields that don't apply to a given event type are null on the wire
// (nil pointers / zero values here) rather than absent.
type Event struct {
	// ID is the stable id for log correlation. Not addressable — there is no
	// GET /events/{id}.
	ID   string    `json:"id"`
	Type EventType `json:"type"`
	// OccurredAt is the ISO 8601 UTC timestamp when the event was observed.
	OccurredAt string `json:"occurred_at"`
	// EmailID is the email_<uuidv7> id minted when the message was accepted.
	EmailID *string `json:"email_id"`
	// MessageID is the RFC 5322 Message-ID: header, when one was stamped.
	MessageID *string `json:"message_id"`
	// From is the envelope From: address.
	From *string `json:"from"`
	// FromDomain is the From: domain, lowercased.
	FromDomain *string `json:"from_domain"`
	// Recipient is the single recipient this event refers to.
	Recipient *string `json:"recipient"`
	// Subject is the captured Subject: header, truncated at the capture limit.
	Subject *string `json:"subject"`
	// Campaign is the originating send's campaign value.
	Campaign *string `json:"campaign"`
	// TemplateID is the public id of the template the originating send used.
	TemplateID *string `json:"template_id"`
	// Topic is the send-time topic the message was tagged with.
	Topic *string `json:"topic"`
	// Tags are the customer-supplied tags from the originating send.
	Tags []string `json:"tags"`
	// SMTPCode is the SMTP reply code observed, or nil without an SMTP exchange.
	SMTPCode *int `json:"smtp_code"`
	// BounceType is the bounce type (e.g. Hard, Soft). Only on email.bounced.
	BounceType *string `json:"bounce_type"`
	// BounceClassification is the bounce classification. Only on email.bounced.
	BounceClassification *string `json:"bounce_classification"`
	// Attempt is the delivery attempt number, or nil for non-delivery events.
	Attempt *int `json:"attempt"`
}

// EventListParams are the filters for EventsService.List. The window defaults to
// the last 24 hours and is clamped to the plan's retention. All filters are
// exact-match except Tags (hasAny).
type EventListParams struct {
	ListParams
	// Start is the ISO 8601 start of the window (inclusive).
	Start string
	// End is the ISO 8601 end of the window (exclusive).
	End       string
	EventType EventType
	// Recipient is an exact recipient address.
	Recipient string
	// EmailID restricts to one message's email_<uuidv7> id.
	EmailID string
	// MessageID is an exact Message-ID: header match.
	MessageID string
	// Domain is a sending-domain hostname (not the domain_<uuid> id).
	Domain string
	Topic  string
	// Campaign is a case-sensitive exact match.
	Campaign string
	// TemplateID is the template the originating send used.
	TemplateID string
	// Tags restricts to events carrying any of these tags (hasAny). Up to 10.
	Tags []string
}

// EventsService holds read access to the /events stream. List-only — events are
// not addressable by id. Access it via Client.Events.
type EventsService struct {
	http *httpClient
}

// List returns one page of the team's events, newest-first.
func (s *EventsService) List(ctx context.Context, params EventListParams, opts ...RequestOption) (*Page[Event], error) {
	return s.fetchPage(ctx, params, opts)
}

func (s *EventsService) fetchPage(ctx context.Context, params EventListParams, opts []RequestOption) (*Page[Event], error) {
	q := newQuery()
	params.ListParams.apply(q)
	q.set("start", params.Start)
	q.set("end", params.End)
	q.set("event_type", string(params.EventType))
	q.set("recipient", params.Recipient)
	q.set("email_id", params.EmailID)
	q.set("message_id", params.MessageID)
	q.set("domain", params.Domain)
	q.set("topic", params.Topic)
	q.set("campaign", params.Campaign)
	q.set("template_id", params.TemplateID)
	// Sent comma-separated (tags=a,b); the API matches with hasAny.
	if len(params.Tags) > 0 {
		q.set("tags", strings.Join(params.Tags, ","))
	}
	env, err := request[pageEnvelope[Event]](ctx, s.http, "GET", "/events", nil, false, q, opts)
	if err != nil {
		return nil, err
	}
	return newPage(*env, func(ctx context.Context, after string) (*Page[Event], error) {
		params.After = after
		return s.fetchPage(ctx, params, opts)
	}), nil
}
