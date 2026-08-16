package contract

import (
	"time"

	attachmentspkg "github.com/compozy/compozy/internal/attachments"
)

// SessionAttachmentPayload is the metadata returned for one stored session attachment.
type SessionAttachmentPayload struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	MIMEType  string    `json:"mime_type"`
	Bytes     int64     `json:"bytes"`
	SHA256    string    `json:"sha256"`
	Kind      string    `json:"kind"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	CreatedAt time.Time `json:"created_at"`
}

// SessionAttachmentUploadResponse is returned after an attachment is stored.
type SessionAttachmentUploadResponse struct {
	Attachment SessionAttachmentPayload `json:"attachment"`
}

// SessionAttachmentPayloadFromRef converts store metadata to its API form.
func SessionAttachmentPayloadFromRef(ref attachmentspkg.AttachmentRef) SessionAttachmentPayload {
	return SessionAttachmentPayload{
		ID:        ref.ID,
		Name:      ref.Name,
		MIMEType:  ref.MIMEType,
		Bytes:     ref.Bytes,
		SHA256:    ref.SHA256,
		Kind:      ref.Kind,
		Width:     ref.Width,
		Height:    ref.Height,
		CreatedAt: ref.CreatedAt.UTC(),
	}
}
