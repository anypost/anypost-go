package anypost

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ErrorType is the stable, machine-readable classification of an API error.
// Branch on this rather than on the HTTP status: the type is part of the API
// contract, the status is not.
type ErrorType string

const (
	ErrorTypeValidation          ErrorType = "validation_error"
	ErrorTypeAuthentication      ErrorType = "authentication_error"
	ErrorTypePermission          ErrorType = "permission_error"
	ErrorTypeNotFound            ErrorType = "not_found"
	ErrorTypeConflict            ErrorType = "conflict"
	ErrorTypeIdempotencyConflict ErrorType = "idempotency_concurrent"
	ErrorTypeIdempotencyMismatch ErrorType = "idempotency_mismatch"
	ErrorTypeWebhookRotation     ErrorType = "webhook_rotation_in_progress"
	ErrorTypeRateLimit           ErrorType = "rate_limit_exceeded"
	ErrorTypePayloadTooLarge     ErrorType = "payload_too_large"
	ErrorTypeProvisioning        ErrorType = "provisioning_error"
	ErrorTypeInternal            ErrorType = "internal_error"
	// ErrorTypeConnection is set when no HTTP response was received (a network
	// failure, timeout, or context cancellation). Status is then 0 and the
	// underlying cause is available via errors.Unwrap.
	ErrorTypeConnection ErrorType = "connection_error"
)

// Error is the single error type returned by every SDK call that fails. A
// request that reached the API and came back non-2xx carries Type, Status, and
// (when sent) RequestID; a request that never got a response carries
// ErrorTypeConnection, a zero Status, and a wrapped cause.
//
// Use errors.As to recover it, then switch on Type:
//
//	var apiErr *anypost.Error
//	if errors.As(err, &apiErr) {
//	    switch apiErr.Type {
//	    case anypost.ErrorTypeValidation:
//	        // apiErr.ValidationErrors: field -> messages
//	    case anypost.ErrorTypeRateLimit:
//	        // apiErr.RetryAfter
//	    }
//	}
type Error struct {
	// Type is the stable, machine-readable error type. Branch on this.
	Type ErrorType
	// Message is the human-readable description from the API.
	Message string
	// Status is the HTTP status code, or 0 when no response was received.
	Status int
	// RequestID is the server-assigned request id, when the response carried
	// one. Quote it in support requests.
	RequestID string
	// ValidationErrors maps a field path to its list of problems. Populated
	// only for ErrorTypeValidation.
	ValidationErrors map[string][]string
	// RetryAfter is the server-advised wait before retrying. Populated only for
	// ErrorTypeRateLimit when the response carried a Retry-After header.
	RetryAfter time.Duration
	// Body is the raw response body, for inspection beyond the parsed fields.
	Body []byte

	cause error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Status == 0 {
		return e.Message
	}
	if e.RequestID != "" {
		return fmt.Sprintf("anypost: %s (type=%s, status=%d, request_id=%s)", e.Message, e.Type, e.Status, e.RequestID)
	}
	return fmt.Sprintf("anypost: %s (type=%s, status=%d)", e.Message, e.Type, e.Status)
}

// Unwrap returns the underlying cause for a connection error, enabling
// errors.Is/errors.As against the wrapped transport error.
func (e *Error) Unwrap() error { return e.cause }

var requestIDHeaders = []string{
	"Anypost-Request-Id",
	"X-Anypost-Request-Id",
	"X-Request-Id",
}

func newConnectionError(message string, cause error) *Error {
	return &Error{Type: ErrorTypeConnection, Message: message, cause: cause}
}

// errorEnvelope is the canonical error body: {"error": {type, message, errors?}}.
// The one documented exception is 413, which returns {"error": "payload_too_large"};
// errorString captures that flat form.
type errorEnvelope struct {
	Error   *errorBody `json:"error"`
	Message string     `json:"message"`
}

type errorBody struct {
	Type    string              `json:"type"`
	Message string              `json:"message"`
	Errors  map[string][]string `json:"errors"`
}

// flatEnvelope matches the flat {"error": "<code>", "message"?} form.
type flatEnvelope struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// parseError maps an HTTP response into an *Error. It keys primarily on the
// canonical error.type, falling back to the status when the type is absent or
// unrecognized.
func parseError(status int, body []byte, header http.Header) *Error {
	requestID := readRequestID(header)

	var (
		errType string
		message string
		fields  map[string][]string
	)

	var env errorEnvelope
	if err := json.Unmarshal(body, &env); err == nil && env.Error != nil {
		// Canonical envelope: {"error": {type, message, errors?}}.
		errType = env.Error.Type
		message = env.Error.Message
		fields = env.Error.Errors
	} else {
		// Flat envelope: {"error": "<code>", "message"?}.
		var flat flatEnvelope
		if err := json.Unmarshal(body, &flat); err == nil && flat.Error != "" {
			errType = flat.Error
			message = flat.Message
			if message == "" {
				message = strings.ReplaceAll(errType, "_", " ")
			}
		}
	}

	if errType == "" {
		errType = string(typeFromStatus(status))
	}
	if message == "" {
		message = defaultMessage(status)
	}

	e := &Error{
		Type:      ErrorType(errType),
		Message:   message,
		Status:    status,
		RequestID: requestID,
		Body:      body,
	}

	switch e.Type {
	case ErrorTypeValidation:
		e.ValidationErrors = fields
	case ErrorTypeRateLimit:
		e.RetryAfter = retryAfter(header)
	}

	// Unknown type with a retry/validation status still gets its extra data.
	if e.ValidationErrors == nil && (status == http.StatusBadRequest || status == http.StatusUnprocessableEntity) {
		e.ValidationErrors = fields
	}
	if e.RetryAfter == 0 && status == http.StatusTooManyRequests {
		e.RetryAfter = retryAfter(header)
	}

	return e
}

func typeFromStatus(status int) ErrorType {
	switch status {
	case http.StatusUnauthorized:
		return ErrorTypeAuthentication
	case http.StatusForbidden:
		return ErrorTypePermission
	case http.StatusNotFound:
		return ErrorTypeNotFound
	case http.StatusConflict:
		return ErrorTypeConflict
	case http.StatusRequestEntityTooLarge:
		return ErrorTypePayloadTooLarge
	case http.StatusTooManyRequests:
		return ErrorTypeRateLimit
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return ErrorTypeValidation
	default:
		if status >= 500 {
			return ErrorTypeInternal
		}
		return "api_error"
	}
}

func defaultMessage(status int) string {
	return fmt.Sprintf("Anypost request failed with status %d.", status)
}

func readRequestID(header http.Header) string {
	for _, name := range requestIDHeaders {
		if v := header.Get(name); v != "" {
			return v
		}
	}
	return ""
}

// retryAfter parses a Retry-After header (delta-seconds or HTTP-date) into a
// duration, clamped at zero. Returns 0 when the header is absent or unparseable.
func retryAfter(header http.Header) time.Duration {
	value := header.Get("Retry-After")
	if value == "" {
		return 0
	}
	if secs, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
		if secs < 0 {
			return 0
		}
		return time.Duration(secs * float64(time.Second))
	}
	if t, err := http.ParseTime(value); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}
