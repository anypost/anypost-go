package anypost

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

// sendErr issues a send against one canned response (no retries) and returns the
// resulting *Error.
func sendErr(t *testing.T, resp cannedResponse) *Error {
	t.Helper()
	client, _ := newTestClientWithRetries(t, 0, resp)
	_, err := client.Email.Send(context.Background(), &SendEmailRequest{From: "you@x.com", To: []string{"a@example.com"}, Text: "hi"})
	if err == nil {
		t.Fatal("expected an error")
	}
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error is not *anypost.Error: %T", err)
	}
	return apiErr
}

func TestErrorTypeMapping(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   ErrorType
	}{
		{"validation", 422, `{"error":{"type":"validation_error","message":"bad","errors":{"to":["required"]}}}`, ErrorTypeValidation},
		{"authentication", 401, `{"error":{"type":"authentication_error","message":"nope"}}`, ErrorTypeAuthentication},
		{"permission", 403, `{"error":{"type":"permission_error","message":"nope"}}`, ErrorTypePermission},
		{"not_found", 404, `{"error":{"type":"not_found","message":"gone"}}`, ErrorTypeNotFound},
		{"conflict", 409, `{"error":{"type":"idempotency_concurrent","message":"inflight"}}`, ErrorTypeIdempotencyConflict},
		{"mismatch", 422, `{"error":{"type":"idempotency_mismatch","message":"diff"}}`, ErrorTypeIdempotencyMismatch},
		{"internal", 500, `{"error":{"type":"internal_error","message":"boom"}}`, ErrorTypeInternal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			apiErr := sendErr(t, jsonResponse(tc.status, tc.body))
			if apiErr.Type != tc.want {
				t.Fatalf("Type = %q, want %q", apiErr.Type, tc.want)
			}
			if apiErr.Status != tc.status {
				t.Fatalf("Status = %d, want %d", apiErr.Status, tc.status)
			}
		})
	}
}

func TestValidationErrorExposesFields(t *testing.T) {
	apiErr := sendErr(t, jsonResponse(422, `{"error":{"type":"validation_error","message":"bad","errors":{"to":["is required","is invalid"]}}}`))
	if apiErr.Type != ErrorTypeValidation {
		t.Fatalf("Type = %q", apiErr.Type)
	}
	if got := apiErr.ValidationErrors["to"]; len(got) != 2 || got[0] != "is required" {
		t.Fatalf("ValidationErrors[to] = %v", got)
	}
}

func TestPayloadTooLargeFlatEnvelope(t *testing.T) {
	// 413 uses the flat {"error":"payload_too_large"} form.
	apiErr := sendErr(t, jsonResponse(413, `{"error":"payload_too_large"}`))
	if apiErr.Type != ErrorTypePayloadTooLarge {
		t.Fatalf("Type = %q, want payload_too_large", apiErr.Type)
	}
	if apiErr.Status != 413 {
		t.Fatalf("Status = %d", apiErr.Status)
	}
}

func TestRateLimitParsesRetryAfter(t *testing.T) {
	header := http.Header{}
	header.Set("Retry-After", "2")
	apiErr := sendErr(t, jsonResponseWithHeader(429, `{"error":{"type":"rate_limit_exceeded","message":"slow down"}}`, header))
	if apiErr.Type != ErrorTypeRateLimit {
		t.Fatalf("Type = %q", apiErr.Type)
	}
	if apiErr.RetryAfter.Seconds() != 2 {
		t.Fatalf("RetryAfter = %v, want 2s", apiErr.RetryAfter)
	}
}

func TestRequestIDIsCaptured(t *testing.T) {
	header := http.Header{}
	header.Set("Anypost-Request-Id", "req_abc123")
	apiErr := sendErr(t, jsonResponseWithHeader(404, `{"error":{"type":"not_found","message":"gone"}}`, header))
	if apiErr.RequestID != "req_abc123" {
		t.Fatalf("RequestID = %q", apiErr.RequestID)
	}
}

func TestUnknownTypeFallsBackToStatus(t *testing.T) {
	// No recognized type, status drives classification.
	apiErr := sendErr(t, jsonResponse(404, `{}`))
	if apiErr.Type != ErrorTypeNotFound {
		t.Fatalf("Type = %q, want not_found from status", apiErr.Type)
	}
}

func TestConnectionError(t *testing.T) {
	client, _ := newTestClientWithRetries(t, 0, networkError(errors.New("dial tcp: connection refused")))
	_, err := client.Email.Send(context.Background(), &SendEmailRequest{From: "you@x.com", To: []string{"a@example.com"}, Text: "hi"})
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error is not *anypost.Error: %T", err)
	}
	if apiErr.Type != ErrorTypeConnection {
		t.Fatalf("Type = %q, want connection_error", apiErr.Type)
	}
	if apiErr.Status != 0 {
		t.Fatalf("Status = %d, want 0", apiErr.Status)
	}
	if errors.Unwrap(apiErr) == nil {
		t.Fatal("connection error should wrap the underlying cause")
	}
}
