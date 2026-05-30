package anypost

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestRetriesOnRetryableStatus(t *testing.T) {
	for _, status := range []int{429, 502, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			client, mock := newTestClient(t,
				jsonResponse(status, `{"error":{"type":"internal_error","message":"transient"}}`),
				jsonResponse(202, `{"id":"email_ok"}`),
			)
			sent, err := client.Email.Send(context.Background(), &SendEmailRequest{From: "you@x.com", To: []string{"a@example.com"}, Text: "hi"})
			if err != nil {
				t.Fatalf("Send: %v", err)
			}
			if sent.ID != "email_ok" {
				t.Fatalf("id = %q", sent.ID)
			}
			if mock.count() != 2 {
				t.Fatalf("expected 2 attempts, got %d", mock.count())
			}
		})
	}
}

func TestRetriesReuseIdempotencyKey(t *testing.T) {
	client, mock := newTestClient(t,
		jsonResponse(503, `{}`),
		jsonResponse(202, `{"id":"email_ok"}`),
	)
	if _, err := client.Email.Send(context.Background(), &SendEmailRequest{From: "you@x.com", To: []string{"a@example.com"}, Text: "hi"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	first := mock.requests[0].header.Get("Idempotency-Key")
	second := mock.requests[1].header.Get("Idempotency-Key")
	if first == "" || first != second {
		t.Fatalf("idempotency key must be reused across retries: %q vs %q", first, second)
	}
}

func TestRetriesOnNetworkError(t *testing.T) {
	client, mock := newTestClient(t,
		networkError(errors.New("connection reset")),
		jsonResponse(202, `{"id":"email_ok"}`),
	)
	if _, err := client.Email.Send(context.Background(), &SendEmailRequest{From: "you@x.com", To: []string{"a@example.com"}, Text: "hi"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if mock.count() != 2 {
		t.Fatalf("expected 2 attempts, got %d", mock.count())
	}
}

func TestNoRetryOnClientError(t *testing.T) {
	client, mock := newTestClient(t, jsonResponse(422, `{"error":{"type":"validation_error","message":"bad"}}`))
	if _, err := client.Email.Send(context.Background(), &SendEmailRequest{From: "you@x.com", To: []string{"a@example.com"}, Text: "hi"}); err == nil {
		t.Fatal("expected an error")
	}
	if mock.count() != 1 {
		t.Fatalf("422 must not be retried, got %d attempts", mock.count())
	}
}

func TestExhaustsRetriesThenReturnsError(t *testing.T) {
	client, mock := newTestClient(t,
		jsonResponse(503, `{}`),
		jsonResponse(503, `{}`),
		jsonResponse(503, `{"error":{"type":"internal_error","message":"still down"}}`),
	)
	_, err := client.Email.Send(context.Background(), &SendEmailRequest{From: "you@x.com", To: []string{"a@example.com"}, Text: "hi"})
	if err == nil {
		t.Fatal("expected an error after exhausting retries")
	}
	// 1 initial + 2 retries = 3 attempts.
	if mock.count() != 3 {
		t.Fatalf("expected 3 attempts, got %d", mock.count())
	}
}

func TestBackoffHonorsRetryAfter(t *testing.T) {
	client, mock := newTestClient(t,
		jsonResponse(503, `{}`),
		jsonResponse(202, `{"id":"email_ok"}`),
	)
	var slept []time.Duration
	client.Email.http.sleep = func(d time.Duration) { slept = append(slept, d) }

	header := http.Header{}
	header.Set("Retry-After", "1")
	mock.results[0].header = header

	if _, err := client.Email.Send(context.Background(), &SendEmailRequest{From: "you@x.com", To: []string{"a@example.com"}, Text: "hi"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(slept) != 1 || slept[0] != time.Second {
		t.Fatalf("expected one 1s sleep from Retry-After, got %v", slept)
	}
}
