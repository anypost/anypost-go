package anypost

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"
)

const testSecret = "whsec_testsecret"

// sign builds a valid Anypost-Signature header for the payload at the given
// timestamp, using one or more secrets (the first is "current").
func sign(timestamp int64, payload string, secrets ...string) string {
	header := fmt.Sprintf("t=%d", timestamp)
	for _, secret := range secrets {
		mac := hmac.New(sha256.New, []byte(secret))
		fmt.Fprintf(mac, "%d.%s", timestamp, payload)
		header += ",v1=" + hex.EncodeToString(mac.Sum(nil))
	}
	return header
}

func TestVerifyWebhookSignatureSuccess(t *testing.T) {
	payload := `{"batch_id":"batch_1","timestamp":1700000000,"events":[]}`
	now := int64(1700000000)
	header := sign(now, payload, testSecret)

	if err := VerifyWebhookSignature([]byte(payload), header, testSecret, WithNow(now)); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestVerifyAcceptsRotatedSecret(t *testing.T) {
	payload := `{"hello":"world"}`
	now := int64(1700000000)
	// Header carries both the old and new secret's signatures during rotation.
	header := sign(now, payload, "whsec_old", "whsec_new")

	// A receiver still configured with only the new secret verifies.
	if err := VerifyWebhookSignature([]byte(payload), header, "whsec_new", WithNow(now)); err != nil {
		t.Fatalf("verify with rotated secret: %v", err)
	}
}

func TestVerifyRejectsBadSignature(t *testing.T) {
	payload := `{"hello":"world"}`
	now := int64(1700000000)
	header := sign(now, payload, "whsec_wrong")

	err := VerifyWebhookSignature([]byte(payload), header, testSecret, WithNow(now))
	assertReason(t, err, ReasonNoMatch)
}

func TestVerifyRejectsStaleTimestamp(t *testing.T) {
	payload := `{}`
	signedAt := int64(1700000000)
	header := sign(signedAt, payload, testSecret)

	// 10 minutes later, default tolerance is 5 minutes.
	err := VerifyWebhookSignature([]byte(payload), header, testSecret, WithNow(signedAt+600))
	assertReason(t, err, ReasonTimestampOutOfTolerance)
}

func TestVerifyToleranceCanBeDisabled(t *testing.T) {
	payload := `{}`
	signedAt := int64(1700000000)
	header := sign(signedAt, payload, testSecret)

	if err := VerifyWebhookSignature([]byte(payload), header, testSecret, WithNow(signedAt+99999), WithTolerance(0)); err != nil {
		t.Fatalf("verify with tolerance disabled: %v", err)
	}
}

func TestVerifyMalformedHeaders(t *testing.T) {
	payload := `{}`
	cases := []struct {
		name   string
		header string
		reason WebhookVerificationReason
	}{
		{"empty", "", ReasonMalformedHeader},
		{"no timestamp", "v1=abc", ReasonNoTimestamp},
		{"no signature", "t=1700000000", ReasonNoSignatures},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := VerifyWebhookSignature([]byte(payload), tc.header, testSecret, WithNow(1700000000))
			assertReason(t, err, tc.reason)
		})
	}
}

func TestUnwrapWebhookEvent(t *testing.T) {
	payload := `{"batch_id":"batch_9","timestamp":1700000000,"events":[{"id":"ev_1","type":"email.delivered","occurred_at":"2026-04-30T00:00:00Z","data":{"email_id":"email_1"}}]}`
	now := int64(1700000000)
	header := sign(now, payload, testSecret)

	delivery, err := UnwrapWebhookEvent([]byte(payload), header, testSecret, WithNow(now))
	if err != nil {
		t.Fatalf("unwrap: %v", err)
	}
	if delivery.BatchID != "batch_9" || len(delivery.Events) != 1 {
		t.Fatalf("delivery = %+v", delivery)
	}
	ev := delivery.Events[0]
	if ev.Type != "email.delivered" || ev.Data["email_id"] != "email_1" {
		t.Fatalf("event = %+v", ev)
	}
}

func TestDefaultToleranceConstant(t *testing.T) {
	if DefaultWebhookTolerance != 300*time.Second {
		t.Fatalf("DefaultWebhookTolerance = %v", DefaultWebhookTolerance)
	}
}

func assertReason(t *testing.T, err error, want WebhookVerificationReason) {
	t.Helper()
	if err == nil {
		t.Fatal("expected a verification error")
	}
	verr, ok := err.(*WebhookVerificationError)
	if !ok {
		t.Fatalf("error is not *WebhookVerificationError: %T", err)
	}
	if verr.Reason != want {
		t.Fatalf("reason = %q, want %q", verr.Reason, want)
	}
}
