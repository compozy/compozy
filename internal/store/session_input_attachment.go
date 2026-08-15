package store

import "strings"

const (
	SessionInputAttachmentKindImage = "image"
	SessionInputAttachmentKindFile  = "file"
)

// SessionInputAttachment is the storage-native attachment snapshot on queued
// input and prompt admissions.
type SessionInputAttachment struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	MIMEType string `json:"mime_type"`
	Bytes    int64  `json:"bytes"`
	SHA256   string `json:"sha256"`
	Kind     string `json:"kind"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

// Normalize returns trimmed attachment identity fields.
func (a SessionInputAttachment) Normalize() SessionInputAttachment {
	a.ID = strings.TrimSpace(a.ID)
	a.Name = strings.TrimSpace(a.Name)
	a.MIMEType = strings.TrimSpace(a.MIMEType)
	a.SHA256 = strings.TrimSpace(a.SHA256)
	a.Kind = strings.TrimSpace(a.Kind)
	return a
}

func cloneSessionInputAttachments(items []SessionInputAttachment) []SessionInputAttachment {
	cloned := make([]SessionInputAttachment, 0, len(items))
	for _, item := range items {
		cloned = append(cloned, item.Normalize())
	}
	return cloned
}
