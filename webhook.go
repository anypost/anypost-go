package anypost

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// DefaultWebhookTolerance is the default maximum age of a webhook delivery,
// measured from its signed timestamp. Deliveries older than this are rejected
// to bound replay of a captured request.
const DefaultWebhookTolerance = 300 * time.Second

// WebhookVerificationReason is the machine-readable cause of a signature
// verification failure. Branch on it rather than the message.
type WebhookVerificationReason string

const (
	// ReasonMalformedHeader means the Anypost-Signature header could not be parsed.
	ReasonMalformedHeader WebhookVerificationReason = "malformed_header"
	// ReasonNoTimestamp means the header carried no t= component.
	ReasonNoTimestamp WebhookVerificationReason = "no_timestamp"
	// ReasonNoSignatures means the header carried no v1= component.
	ReasonNoSignatures WebhookVerificationReason = "no_signatures"
	// ReasonTimestampOutOfTolerance means the delivery is older than the tolerance.
	ReasonTimestampOutOfTolerance WebhookVerificationReason = "timestamp_out_of_tolerance"
	// ReasonNoMatch means no v1= component matched the computed signature.
	ReasonNoMatch WebhookVerificationReason = "no_match"
)

// WebhookVerificationError is returned when a webhook delivery's signature
// cannot be verified.
type WebhookVerificationError struct {
	// Reason is the machine-readable cause. Branch on this.
	Reason  WebhookVerificationReason
	Message string
}

func (e *WebhookVerificationError) Error() string { return e.Message }

// WebhookDelivery is the outer envelope of a webhook delivery: one batch of one
// or more events.
type WebhookDelivery struct {
	// BatchID identifies this batch. Stable across retries — de-duplicate on it.
	BatchID string `json:"batch_id"`
	// Timestamp is the Unix timestamp the batch was signed with.
	Timestamp int64                  `json:"timestamp"`
	Events    []WebhookDeliveryEvent `json:"events"`
}

// WebhookDeliveryEvent is one event inside a WebhookDelivery.
type WebhookDeliveryEvent struct {
	// ID is the unique event id. Stable across retries — de-duplicate on it.
	ID string `json:"id"`
	// Type is a WebhookEventType or "webhook.test".
	Type       string `json:"type"`
	OccurredAt string `json:"occurred_at"`
	// Data always carries email_id; the rest depends on the event type.
	Data map[string]any `json:"data"`
}

// verifyConfig holds the resolved options for VerifyWebhookSignature.
type verifyConfig struct {
	tolerance time.Duration
	now       int64 // unix seconds; 0 means "use the real clock"
}

// VerifyOption configures webhook signature verification.
type VerifyOption func(*verifyConfig)

// WithTolerance overrides the maximum delivery age. A zero value disables the
// freshness check.
func WithTolerance(d time.Duration) VerifyOption {
	return func(c *verifyConfig) { c.tolerance = d }
}

// WithNow overrides the current time (Unix seconds) used for the freshness
// check. For tests.
func WithNow(unixSeconds int64) VerifyOption {
	return func(c *verifyConfig) { c.now = unixSeconds }
}

// VerifyWebhookSignature verifies the signature on an Anypost webhook delivery.
//
// Pass the raw request body (the exact bytes received, before JSON parsing),
// the Anypost-Signature header value, and the webhook's signing secret. It
// returns nil on success and a *WebhookVerificationError otherwise.
//
// The header may carry more than one v1= component during a secret rotation; a
// match on any one passes, so deliveries keep verifying across a rotation.
func VerifyWebhookSignature(payload []byte, signatureHeader, secret string, opts ...VerifyOption) error {
	cfg := verifyConfig{tolerance: DefaultWebhookTolerance}
	for _, opt := range opts {
		opt(&cfg)
	}

	timestamp, signatures, err := parseSignatureHeader(signatureHeader)
	if err != nil {
		return err
	}

	if cfg.tolerance > 0 {
		now := cfg.now
		if now == 0 {
			now = time.Now().Unix()
		}
		if now-timestamp > int64(cfg.tolerance.Seconds()) {
			return &WebhookVerificationError{
				Reason:  ReasonTimestampOutOfTolerance,
				Message: fmt.Sprintf("Timestamp %d is older than the %ds tolerance.", timestamp, int64(cfg.tolerance.Seconds())),
			}
		}
	}

	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%d.%s", timestamp, payload)
	expected := []byte(hex.EncodeToString(mac.Sum(nil)))

	// Constant-time over every candidate: accumulate without early exit.
	matched := false
	for _, candidate := range signatures {
		if hmac.Equal([]byte(candidate), expected) {
			matched = true
		}
	}
	if !matched {
		return &WebhookVerificationError{
			Reason:  ReasonNoMatch,
			Message: "No signature in the header matched the computed signature.",
		}
	}
	return nil
}

// UnwrapWebhookEvent verifies a delivery and returns its parsed body. It is a
// thin wrapper over VerifyWebhookSignature that unmarshals only after the
// signature checks out.
func UnwrapWebhookEvent(payload []byte, signatureHeader, secret string, opts ...VerifyOption) (*WebhookDelivery, error) {
	if err := VerifyWebhookSignature(payload, signatureHeader, secret, opts...); err != nil {
		return nil, err
	}
	var delivery WebhookDelivery
	if err := json.Unmarshal(payload, &delivery); err != nil {
		return nil, &WebhookVerificationError{
			Reason:  ReasonMalformedHeader,
			Message: fmt.Sprintf("decoding webhook payload: %v", err),
		}
	}
	return &delivery, nil
}

func parseSignatureHeader(header string) (timestamp int64, signatures []string, err error) {
	if header == "" {
		return 0, nil, &WebhookVerificationError{Reason: ReasonMalformedHeader, Message: "The Anypost-Signature header is empty."}
	}

	haveTimestamp := false
	for _, part := range strings.Split(header, ",") {
		eq := strings.IndexByte(part, '=')
		if eq == -1 {
			continue
		}
		key := strings.TrimSpace(part[:eq])
		value := strings.TrimSpace(part[eq+1:])
		switch key {
		case "t":
			if ts, perr := strconv.ParseInt(value, 10, 64); perr == nil {
				timestamp = ts
				haveTimestamp = true
			}
		case "v1":
			signatures = append(signatures, value)
		}
	}

	if !haveTimestamp {
		return 0, nil, &WebhookVerificationError{Reason: ReasonNoTimestamp, Message: "The Anypost-Signature header has no timestamp (t=)."}
	}
	if len(signatures) == 0 {
		return 0, nil, &WebhookVerificationError{Reason: ReasonNoSignatures, Message: "The Anypost-Signature header has no v1= signature."}
	}
	return timestamp, signatures, nil
}
