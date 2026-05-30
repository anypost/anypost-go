package anypost

import (
	"context"
	"net/http"
	"testing"
)

func TestSendSerializesBody(t *testing.T) {
	client, mock := newTestClient(t, jsonResponse(202, `{"id":"email_018f","created_at":"2026-04-30T12:00:00.123Z"}`))

	sent, err := client.Email.Send(context.Background(), &SendEmailRequest{
		From:    "Acme <you@yourdomain.com>",
		To:      []string{"a@example.com", "b@example.com"},
		CC:      []string{"team@example.com"},
		ReplyTo: []string{"support@yourdomain.com"},
		Subject: "Receipt",
		HTML:    "<p>Thanks</p>",
		Tags:    []string{"receipt"},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if sent.ID != "email_018f" {
		t.Fatalf("id = %q", sent.ID)
	}

	req := mock.last()
	if req.method != http.MethodPost || req.url != "https://api.test/v1/email" {
		t.Fatalf("unexpected request %s %s", req.method, req.url)
	}
	if ct := req.header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q", ct)
	}
	body := req.json(t)
	if body["from"] != "Acme <you@yourdomain.com>" {
		t.Fatalf("from = %v", body["from"])
	}
	if to, ok := body["to"].([]any); !ok || len(to) != 2 || to[0] != "a@example.com" {
		t.Fatalf("to = %v", body["to"])
	}
	if body["reply_to"].([]any)[0] != "support@yourdomain.com" {
		t.Fatalf("reply_to = %v", body["reply_to"])
	}
	// Omitted optional fields are not serialized.
	if _, present := body["bcc"]; present {
		t.Fatalf("bcc should be omitted, got %v", body["bcc"])
	}
	if _, present := body["text"]; present {
		t.Fatalf("text should be omitted")
	}
}

func TestSendSetsAutomaticIdempotencyKey(t *testing.T) {
	client, mock := newTestClient(t, jsonResponse(202, `{"id":"email_1"}`))
	if _, err := client.Email.Send(context.Background(), &SendEmailRequest{From: "you@x.com", To: []string{"a@example.com"}, Text: "hi"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	key := mock.last().header.Get("Idempotency-Key")
	if len(key) != 36 {
		t.Fatalf("expected an auto-generated uuid idempotency key, got %q", key)
	}
}

func TestSendHonorsExplicitIdempotencyKey(t *testing.T) {
	client, mock := newTestClient(t, jsonResponse(202, `{"id":"email_1"}`))
	if _, err := client.Email.Send(context.Background(),
		&SendEmailRequest{From: "you@x.com", To: []string{"a@example.com"}, Text: "hi"},
		WithIdempotencyKey("order-4823"),
	); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got := mock.last().header.Get("Idempotency-Key"); got != "order-4823" {
		t.Fatalf("Idempotency-Key = %q", got)
	}
}

func TestAttachmentsAreBase64Encoded(t *testing.T) {
	client, mock := newTestClient(t, jsonResponse(202, `{"id":"email_1"}`))
	if _, err := client.Email.Send(context.Background(), &SendEmailRequest{
		From:        "you@x.com",
		To:          []string{"a@example.com"},
		Subject:     "Report",
		Text:        "Attached.",
		Attachments: []Attachment{{Filename: "hello.txt", Content: []byte("hello"), ContentType: "text/plain"}},
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	body := mock.last().json(t)
	att := body["attachments"].([]any)[0].(map[string]any)
	if att["filename"] != "hello.txt" {
		t.Fatalf("filename = %v", att["filename"])
	}
	if att["content"] != "aGVsbG8=" { // base64("hello")
		t.Fatalf("content = %v, want base64(hello)", att["content"])
	}
	if att["content_type"] != "text/plain" {
		t.Fatalf("content_type = %v", att["content_type"])
	}
}

func TestSendBatchReturnsMixedOutcomes(t *testing.T) {
	resp := `{"summary":{"total":2,"queued":1,"failed":1},"data":[` +
		`{"status":"queued","index":0,"id":"email_1","created_at":"2026-04-30T12:00:00Z"},` +
		`{"status":"failed","index":1,"error":{"type":"validation_error","message":"bad"}}]}`
	client, mock := newTestClient(t, jsonResponse(207, resp))

	result, err := client.Email.SendBatch(context.Background(), &EmailBatchRequest{
		Emails: []SendEmailRequest{
			{From: "you@x.com", To: []string{"a@example.com"}, Subject: "A", Text: ".."},
			{From: "you@x.com", To: []string{"b@example.com"}, Subject: "B", Text: ".."},
		},
	})
	if err != nil {
		t.Fatalf("SendBatch should not error on 207: %v", err)
	}
	if result.Summary.Failed != 1 || result.Summary.Queued != 1 {
		t.Fatalf("summary = %+v", result.Summary)
	}
	if result.Data[0].Status != "queued" || result.Data[0].ID != "email_1" {
		t.Fatalf("data[0] = %+v", result.Data[0])
	}
	if result.Data[1].Error == nil || result.Data[1].Error.Type != "validation_error" {
		t.Fatalf("data[1] = %+v", result.Data[1])
	}
	if mock.last().url != "https://api.test/v1/email/batch" {
		t.Fatalf("url = %s", mock.last().url)
	}
}

func TestBatchEntriesInheritDefaultsFrom(t *testing.T) {
	client, mock := newTestClient(t, jsonResponse(202, `{"summary":{},"data":[]}`))
	// Entries leave From zero; defaults supplies it (matching the other SDKs).
	if _, err := client.Email.SendBatch(context.Background(), &EmailBatchRequest{
		Defaults: &SendEmailRequest{From: "you@yourdomain.com"},
		Emails: []SendEmailRequest{
			{To: []string{"a@example.com"}, Subject: "Hi A", Text: ".."},
			{To: []string{"b@example.com"}, Subject: "Hi B", Text: ".."},
		},
	}); err != nil {
		t.Fatalf("SendBatch: %v", err)
	}
	body := mock.last().json(t)
	defaults := body["defaults"].(map[string]any)
	if defaults["from"] != "you@yourdomain.com" {
		t.Fatalf("defaults.from = %v", defaults["from"])
	}
	entry := body["emails"].([]any)[0].(map[string]any)
	if _, present := entry["from"]; present {
		t.Fatalf("entry from should be omitted so the default applies, got %v", entry["from"])
	}
	// The defaults entry omits the per-entry `to`.
	if _, present := defaults["to"]; present {
		t.Fatalf("defaults.to should be omitted, got %v", defaults["to"])
	}
}
