package anypost

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// Permissions is the permission level of an API key.
type Permissions string

const (
	// PermissionFull grants management and send access.
	PermissionFull Permissions = "full"
	// PermissionSendOnly grants send access only.
	PermissionSendOnly Permissions = "send_only"
)

// Attachment is one inline attachment on a message.
//
// Set the content exactly one of two ways: Content for raw file bytes (for
// example, the result of os.ReadFile), which the SDK base64-encodes on the
// wire, or ContentBase64 when the data is already base64 at rest, which is
// sent through verbatim so nothing is decoded just to be re-encoded.
type Attachment struct {
	// Filename is the file name shown to the recipient.
	Filename string `json:"filename"`
	// Content is the raw file bytes; encoded to base64 on the wire. Leave nil
	// when setting ContentBase64 instead.
	Content []byte `json:"-"`
	// ContentBase64 is already-base64-encoded content, sent verbatim. Leave
	// empty when setting Content instead.
	ContentBase64 string `json:"-"`
	// ContentType is the MIME type. Defaults to application/octet-stream
	// server-side when empty.
	ContentType string `json:"content_type,omitempty"`
	// ContentID marks the attachment inline, referenced from the HTML via cid:.
	ContentID string `json:"content_id,omitempty"`
}

// MarshalJSON writes the wire shape, where the content is always a single
// base64 "content" field. Returns an error when the two content fields are set
// together or not at all — json.Marshal surfaces it, so the send fails before
// any request goes out.
func (a Attachment) MarshalJSON() ([]byte, error) {
	if len(a.Content) > 0 && a.ContentBase64 != "" {
		return nil, fmt.Errorf(
			"anypost: attachment %q sets both Content and ContentBase64; set exactly one", a.Filename)
	}

	content := a.ContentBase64
	if content == "" {
		if len(a.Content) == 0 {
			return nil, fmt.Errorf(
				"anypost: attachment %q has no content; set Content (raw bytes) or ContentBase64", a.Filename)
		}
		content = base64.StdEncoding.EncodeToString(a.Content)
	}

	// An alias type so the marshaller does not recurse back into this method,
	// with content spliced in as the already-encoded string.
	type wire Attachment
	return json.Marshal(struct {
		wire
		Content string `json:"content"`
	}{
		wire:    wire(a),
		Content: content,
	})
}

// Tracking overrides the sending domain's open/click tracking defaults for one
// message. A nil field leaves that dimension at the domain default.
type Tracking struct {
	// Opens injects the open-tracking pixel into the HTML body when non-nil.
	Opens *bool `json:"opens,omitempty"`
	// Clicks rewrites links for click tracking when non-nil.
	Clicks *bool `json:"clicks,omitempty"`
}

// UnsubscribeMode is the one-click unsubscribe behavior for a send.
type UnsubscribeMode string

const (
	// UnsubscribeGenerate mints a per-recipient signed token and injects RFC
	// 8058 unsubscribe headers. Requires a Topic on the send.
	UnsubscribeGenerate UnsubscribeMode = "generate"
	// UnsubscribeNone injects nothing — for transactional sends that must not
	// carry unsubscribe semantics.
	UnsubscribeNone UnsubscribeMode = "none"
)

// Unsubscribe configures one-click unsubscribe headers for a send.
type Unsubscribe struct {
	Mode UnsubscribeMode `json:"mode"`
	// DisplayName is the human-readable label rendered on the hosted
	// confirmation page.
	DisplayName string `json:"display_name,omitempty"`
}

// Bool is a helper for setting an optional *bool field, e.g.
// Tracking{Opens: anypost.Bool(true)}.
func Bool(v bool) *bool { return &v }

// String is a helper for setting an optional *string field, e.g.
// TemplateCreateParams{HTML: anypost.String("<h1>Hi</h1>")}.
func String(v string) *string { return &v }
