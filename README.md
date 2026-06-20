# Anypost Go SDK

The official Go client for the [Anypost](https://anypost.com) email API.

Requires Go 1.23+. Zero dependencies (standard library only). Every call takes a
`context.Context` and is safe for concurrent use.

This README covers the SDK itself: installation, idioms, and configuration. For
platform concepts and the full field-level API reference, see the [Anypost
documentation](https://anypost.com/docs).

## Install

```bash
go get github.com/anypost/anypost-go
```

```go
import "github.com/anypost/anypost-go"
```

The package name is `anypost`.

## Quickstart

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/anypost/anypost-go"
)

func main() {
	client, err := anypost.New("ap_your_api_key")
	if err != nil {
		log.Fatal(err)
	}

	sent, err := client.Email.Send(context.Background(), &anypost.SendEmailRequest{
		From:    "YourCo <you@yourdomain.com>",
		To:      []string{"you@example.com"},
		Subject: "Welcome to Anypost",
		HTML:    "<p>Hello, inbox!</p>",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(sent.ID)
}
```

`anypost.New("")` reads the key from `ANYPOST_API_KEY` instead. Keep the key
server-side; it is a bearer credential.

## Sending

One of `Text`, `HTML`, or `TemplateID` is required. All recipients in `To`,
`CC`, and `BCC` share one envelope and count against a combined limit of 50.

```go
sent, err := client.Email.Send(ctx, &anypost.SendEmailRequest{
	From:    "YourCo <you@yourdomain.com>",
	To:      []string{"a@example.com", "b@example.com"},
	CC:      []string{"team@example.com"},
	ReplyTo: []string{"support@yourdomain.com"},
	Subject: "Receipt #4823",
	HTML:    "<p>Thanks for your order.</p>",
	Text:    "Thanks for your order.",
	Tags:    []string{"receipt"},
})
```

`Attachment.Content` is the raw file bytes: pass what `os.ReadFile` returns and
the SDK base64-encodes it on the wire. Do not pre-encode it. The request body is
capped at 5 MB.

```go
pdf, err := os.ReadFile("report.pdf")
if err != nil {
	log.Fatal(err)
}

_, err = client.Email.Send(ctx, &anypost.SendEmailRequest{
	From:    "YourCo <you@yourdomain.com>",
	To:      []string{"someone@example.com"},
	Subject: "Your report",
	Text:    "Attached.",
	Attachments: []anypost.Attachment{
		{Filename: "report.pdf", Content: pdf},
	},
})
```

Send with a published template and per-recipient variables:

```go
_, err := client.Email.Send(ctx, &anypost.SendEmailRequest{
	From:       "YourCo <you@yourdomain.com>",
	To:         []string{"someone@example.com"},
	TemplateID: "template_018f2c5e-3a40-7a91-9c25-3a0b1d5e6f78",
	Variables:  map[string]any{"name": "Ada", "plan": "pro"},
})
```

See the [send reference](https://anypost.com/docs/reference/emails) for the
complete field list.

## Batch

Send 1 to 100 independent messages in one request. `Defaults` fills any field an
entry omits. Leave an entry's `From` (and any other shared field) zero to inherit
the default; an entry that sets its own value wins. `To` is always per-entry.

```go
result, err := client.Email.SendBatch(ctx, &anypost.EmailBatchRequest{
	Defaults: &anypost.SendEmailRequest{From: "YourCo <you@yourdomain.com>"},
	Emails: []anypost.SendEmailRequest{
		{To: []string{"a@example.com"}, Subject: "Hi A", Text: "..."},
		{To: []string{"b@example.com"}, Subject: "Hi B", Text: "..."},
	},
})
```

A batch with mixed outcomes returns HTTP `207` and does not return an error.
Inspect each entry's `Status` rather than treating it as a failure:

```go
fmt.Printf("%+v\n", result.Summary) // {Total, Queued, Failed}

for _, entry := range result.Data {
	if entry.Status == "queued" {
		fmt.Println(entry.Index, entry.ID)
	} else {
		fmt.Println(entry.Index, entry.Error.Type, entry.Error.Message)
	}
}
```

## Domains

Manage sending domains under `client.Domains`. Add a domain, publish the records
it returns, then verify.

```go
domain, err := client.Domains.Create(ctx, &anypost.DomainCreateParams{Name: "example.com"})
if err != nil {
	log.Fatal(err)
}
for _, r := range domain.DNSRecords {
	fmt.Printf("%s %s -> %s\n", r.Type, r.Name, r.Value)
}

checked, err := client.Domains.Verify(ctx, domain.ID)
if err != nil {
	log.Fatal(err)
}
if checked.Status != "verified" && checked.VerificationFailure != nil {
	// Verify returns the current domain even while pending; it is not an error.
	fmt.Println(checked.VerificationFailure.Code)
}
```

`Get`, `Update` (tracking config only), and `Delete` round out the resource. See
[Domains](https://anypost.com/docs/reference/domains) for the verification
lifecycle and field reference.

## API keys

Manage keys under `client.APIKeys`. The plaintext secret comes back only once, on
`Create`, as `Key`:

```go
created, err := client.APIKeys.Create(ctx, &anypost.APIKeyCreateParams{
	Name:           "Production server",
	Permissions:    anypost.PermissionSendOnly,
	AllowedDomains: []string{"example.com"},
})
if err != nil {
	log.Fatal(err)
}
fmt.Println(created.Key) // store now; never retrievable again
```

`Get` returns metadata only (`KeyPrefix`, never the secret); `Update` and
`Delete` round out the resource. See [API
keys](https://anypost.com/docs/reference/api-keys) for the permission model and
cache propagation.

## Templates

Templates use a draft/published model: edits land in a draft, and `Publish`
promotes it. A template can't be used for sending until it's published.

```go
tmpl, err := client.Templates.Create(ctx, &anypost.TemplateCreateParams{
	Name: "Welcome email",
	Kind: anypost.TemplateKindHTML,
	HTML: anypost.String("<h1>Welcome, {{ name }}</h1>"),
})
if err != nil {
	log.Fatal(err)
}

_, err = client.Templates.Publish(ctx, tmpl.ID)
```

The pointer-string fields (`Subject`, `HTML`, `Markdown`) distinguish "unset"
from an explicit empty string. `anypost.String` is a helper for setting them.

`Kind` (`html` or `markdown`) is immutable once set. `GetDraft`, `UpdateDraft`,
`DeleteDraft`, `Duplicate`, `Get`, `Update` (name only), and `Delete` round out
the resource. Send a published template with `TemplateID` (see
[Sending](#sending)). See
[Templates](https://anypost.com/docs/reference/templates) for the full model.

## Suppressions

A suppression blocks sends to an address, scoped to a `Topic`. The wildcard `*`
blocks every topic; a specific topic (e.g. `marketing`) leaves transactional
traffic untouched.

```go
_, err := client.Suppressions.Create(ctx, &anypost.SuppressionCreateParams{
	Email: "alice@example.com",
	Topic: "marketing",
	Note:  "Customer requested removal",
})

err = client.Suppressions.Delete(ctx, "alice@example.com", "marketing")
```

`Get`, `List` (with `EmailContains`, `Topic`, `Reason`, and `Origin` filters),
`ListForEmail`, and `DeleteForEmail` round out the resource. See
[Suppressions](https://anypost.com/docs/reference/suppressions) for scoping and
the automatic-suppression rules for bounces and complaints.

## Webhooks

Manage webhook subscriptions under `client.Webhooks`. The `SigningSecret` comes
back only once, on `Create`; later reads return only `SigningSecretPrefix`.

```go
wh, err := client.Webhooks.Create(ctx, &anypost.WebhookCreateParams{
	Name:   "Production events",
	URL:    "https://hooks.example.com/anypost",
	Events: []anypost.WebhookEventType{anypost.WebhookEventDelivered, anypost.WebhookEventBounced, anypost.WebhookEventComplained},
})
if err != nil {
	log.Fatal(err)
}
fmt.Println(wh.SigningSecret) // store now; never retrievable again
```

`Update`, `Test`, `RotateSecret`, `Get`, `List`, and `Delete` round out the
resource. See [Webhooks](https://anypost.com/docs/reference/webhooks) for the
event catalog, status transitions, and the secret-rotation grace window.

### Verifying deliveries

`anypost.VerifyWebhookSignature` and `anypost.UnwrapWebhookEvent` are plain
functions: they need the signing secret, not an API key, so call them in your
handler without a client. Pass the **raw** request body (the exact bytes, before
JSON parsing), the `Anypost-Signature` header, and the secret.

```go
func handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	sig := r.Header.Get("Anypost-Signature")

	delivery, err := anypost.UnwrapWebhookEvent(body, sig, signingSecret)
	if err != nil {
		var verr *anypost.WebhookVerificationError
		errors.As(err, &verr) // verr.Reason: ReasonNoMatch, ReasonTimestampOutOfTolerance, ...
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	for _, event := range delivery.Events {
		// event.Type, event.Data["email_id"], ...
	}
	w.WriteHeader(http.StatusOK)
}
```

Reach for `VerifyWebhookSignature` when something else has already parsed the
body: keep the raw bytes for the verify step, then use your parsed value once it
passes. Deliveries older than five minutes are rejected by default to bound
replay; `WithTolerance` widens, narrows, or disables (`0`) that check, and
`WithNow` overrides the clock in tests. During a secret rotation the header
carries a `v1=` component per active secret, and a match on any one passes, so
deliveries keep verifying while you redeploy.

## Events

`client.Events.List` pages the team's event stream, newest-first. The window
defaults to the last 24 hours and is clamped to your plan's retention. Events are
read-only and not addressable by id, so there is no `Get`.

```go
page, err := client.Events.List(ctx, anypost.EventListParams{EventType: anypost.EventBounced})
if err != nil {
	log.Fatal(err)
}
for _, e := range page.Data {
	fmt.Println(e.OccurredAt, e.Recipient, e.BounceClassification)
}
```

Filter by `Start`, `End`, `EventType`, `Recipient`, `EmailID`, `MessageID`,
`Domain`, `Topic`, `Campaign`, `TemplateID`, and `Tags`, a slice that matches an
event carrying *any* of the given tags. Every other filter is exact-match. This
is also how you backfill the gap after a webhook endpoint was disabled: page the
events that occurred during the outage once it's healthy. See
[Events](https://anypost.com/docs/reference/events) for the field reference.

## Pagination

List endpoints return a `*Page[T]` with `Data`, `HasMore`, and `NextCursor`. Read
one page, call `Next` to fetch the following one, or range over `All` to walk
every item across pages, re-fetching as it goes.

```go
page, err := client.Domains.List(ctx, anypost.ListParams{Limit: 50})
page.Data       // this page's items
page.HasMore    // whether another page exists
page.NextCursor // pass to ListParams.After to fetch it yourself

for domain, err := range page.All(ctx) { // every domain, across all pages
	if err != nil {
		return err
	}
	fmt.Println(domain.Name)
}
```

## Errors

A failed request returns an `*anypost.Error`. Recover it with `errors.As` and
switch on `Type`, which is the stable, machine-readable `error.type`. Branch on
it rather than on the HTTP status.

```go
sent, err := client.Email.Send(ctx, message)
if err != nil {
	var apiErr *anypost.Error
	if errors.As(err, &apiErr) {
		switch apiErr.Type {
		case anypost.ErrorTypeValidation:
			fmt.Println(apiErr.ValidationErrors) // field -> messages
		case anypost.ErrorTypeRateLimit:
			fmt.Println(apiErr.RetryAfter) // time.Duration
		default:
			fmt.Println(apiErr.Type, apiErr.Status, apiErr.RequestID)
		}
	}
	return err
}
```

| `Type` constant | `error.type` | Status |
|---|---|---|
| `ErrorTypeValidation` | `validation_error` | `400`, `422` |
| `ErrorTypeAuthentication` | `authentication_error` | `401` |
| `ErrorTypePermission` | `permission_error` | `403` |
| `ErrorTypeNotFound` | `not_found` | `404` |
| `ErrorTypeConflict` / `ErrorTypeIdempotencyConflict` / `ErrorTypeWebhookRotation` | `conflict`, `idempotency_concurrent`, `webhook_rotation_in_progress` | `409` |
| `ErrorTypeIdempotencyMismatch` | `idempotency_mismatch` | `422` |
| `ErrorTypeRateLimit` | `rate_limit_exceeded` | `429` |
| `ErrorTypePayloadTooLarge` | `payload_too_large` | `413` |
| `ErrorTypeInternal` / `ErrorTypeProvisioning` | `internal_error`, `provisioning_error` | `5xx` |
| `ErrorTypeConnection` | `connection_error` | none |

Every API-level error carries `Type`, `Status`, `RequestID`, `Message`, and the
raw `Body`. A connection error (no response) carries `ErrorTypeConnection`, a
zero `Status`, and the underlying transport error via `errors.Unwrap`.

## Retries and idempotency

The client retries `429`, `502`, `503`, and network failures up to `maxRetries`
times (default 2), with exponential backoff and full jitter. It honors
`Retry-After`.

Sends are made safe to retry automatically: when retries are enabled and you do
not pass an idempotency key, the client generates one and reuses it across
attempts, so a retried send cannot deliver twice. Pass your own key to dedupe
across process restarts:

```go
client.Email.Send(ctx, message, anypost.WithIdempotencyKey("order-4823"))
```

## Configuration

```go
client, err := anypost.New("ap_your_api_key",
	anypost.WithBaseURL("https://api.anypost.com/v1"),
	anypost.WithTimeout(30*time.Second),
	anypost.WithMaxRetries(2),
	anypost.WithHTTPClient(&http.Client{}),
	anypost.WithDefaultHeader("X-My-Header", "value"),
)
```

| Option | Default | Description |
|---|---|---|
| `WithBaseURL` | `https://api.anypost.com/v1` | API base URL. |
| `WithTimeout` | 30s | Per-request timeout, composed with the call's context. |
| `WithMaxRetries` | 2 | Automatic retries for transient failures. |
| `WithHTTPClient` | `&http.Client{}` | Custom client/transport (proxy, TLS, tests). |
| `WithDefaultHeader` | none | Extra header sent on every request (repeatable). |

Pass an empty string as the API key to read `ANYPOST_API_KEY` from the
environment.

## License

MIT
