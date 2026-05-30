package anypost

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
// Content is the raw file bytes (for example, the result of os.ReadFile). The
// SDK base64-encodes it on the wire via Go's standard JSON encoding of a byte
// slice — do not pre-encode it.
type Attachment struct {
	// Filename is the file name shown to the recipient.
	Filename string `json:"filename"`
	// Content is the raw file bytes; encoded to base64 on the wire.
	Content []byte `json:"content"`
	// ContentType is the MIME type. Defaults to application/octet-stream
	// server-side when empty.
	ContentType string `json:"content_type,omitempty"`
	// ContentID marks the attachment inline, referenced from the HTML via cid:.
	ContentID string `json:"content_id,omitempty"`
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
