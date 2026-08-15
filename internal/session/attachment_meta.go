package session

import (
	"strings"

	"github.com/compozy/compozy/internal/store"
)

const (
	AttachmentKindImage = "image"
	AttachmentKindFile  = "file"
)

// AttachmentMeta is the session-owned attachment snapshot on a prompt.
type AttachmentMeta struct {
	ID       string
	Name     string
	MIMEType string
	Bytes    int64
	SHA256   string
	Kind     string
	Width    int
	Height   int
}

func cloneAttachmentMeta(items []AttachmentMeta) []AttachmentMeta {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]AttachmentMeta, 0, len(items))
	for _, item := range items {
		cloned = append(cloned, AttachmentMeta{
			ID:       strings.TrimSpace(item.ID),
			Name:     strings.TrimSpace(item.Name),
			MIMEType: strings.TrimSpace(item.MIMEType),
			Bytes:    item.Bytes,
			SHA256:   strings.TrimSpace(item.SHA256),
			Kind:     strings.TrimSpace(item.Kind),
			Width:    item.Width,
			Height:   item.Height,
		})
	}
	return cloned
}

func storeAttachments(items []AttachmentMeta) []store.SessionInputAttachment {
	if len(items) == 0 {
		return []store.SessionInputAttachment{}
	}
	out := make([]store.SessionInputAttachment, 0, len(items))
	for _, item := range items {
		out = append(out, store.SessionInputAttachment{
			ID:       strings.TrimSpace(item.ID),
			Name:     strings.TrimSpace(item.Name),
			MIMEType: strings.TrimSpace(item.MIMEType),
			Bytes:    item.Bytes,
			SHA256:   strings.TrimSpace(item.SHA256),
			Kind:     strings.TrimSpace(item.Kind),
			Width:    item.Width,
			Height:   item.Height,
		})
	}
	return out
}

func attachmentMetaFromStore(items []store.SessionInputAttachment) []AttachmentMeta {
	if len(items) == 0 {
		return nil
	}
	out := make([]AttachmentMeta, 0, len(items))
	for _, item := range items {
		out = append(out, AttachmentMeta{
			ID:       strings.TrimSpace(item.ID),
			Name:     strings.TrimSpace(item.Name),
			MIMEType: strings.TrimSpace(item.MIMEType),
			Bytes:    item.Bytes,
			SHA256:   strings.TrimSpace(item.SHA256),
			Kind:     strings.TrimSpace(item.Kind),
			Width:    item.Width,
			Height:   item.Height,
		})
	}
	return out
}

func attachmentFingerprintDigests(items []AttachmentMeta) []string {
	digests := make([]string, 0, len(items))
	for _, item := range items {
		digest := strings.TrimSpace(item.SHA256)
		if digest == "" {
			digest = strings.TrimSpace(item.ID)
		}
		if digest == "" {
			continue
		}
		digests = append(digests, digest)
	}
	return digests
}
