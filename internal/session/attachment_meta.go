package session

import (
	"strings"

	attachmentspkg "github.com/compozy/compozy/internal/attachments"
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

// AttachmentMetaFromRef converts retained metadata into the session-owned prompt snapshot.
func AttachmentMetaFromRef(ref attachmentspkg.AttachmentRef) AttachmentMeta {
	return normalizeAttachmentMeta(AttachmentMeta{
		ID: ref.ID, Name: ref.Name, MIMEType: ref.MIMEType, Bytes: ref.Bytes,
		SHA256: ref.SHA256, Kind: ref.Kind, Width: ref.Width, Height: ref.Height,
	})
}

func normalizeAttachmentMeta(item AttachmentMeta) AttachmentMeta {
	item.ID = strings.TrimSpace(item.ID)
	item.Name = strings.TrimSpace(item.Name)
	item.MIMEType = strings.TrimSpace(item.MIMEType)
	item.SHA256 = strings.TrimSpace(item.SHA256)
	item.Kind = strings.TrimSpace(item.Kind)
	return item
}

func cloneAttachmentMeta(items []AttachmentMeta) []AttachmentMeta {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]AttachmentMeta, 0, len(items))
	for _, item := range items {
		cloned = append(cloned, normalizeAttachmentMeta(item))
	}
	return cloned
}

func storeAttachments(items []AttachmentMeta) []store.SessionInputAttachment {
	if len(items) == 0 {
		return []store.SessionInputAttachment{}
	}
	out := make([]store.SessionInputAttachment, 0, len(items))
	for _, item := range items {
		item = normalizeAttachmentMeta(item)
		out = append(out, store.SessionInputAttachment{
			ID:       item.ID,
			Name:     item.Name,
			MIMEType: item.MIMEType,
			Bytes:    item.Bytes,
			SHA256:   item.SHA256,
			Kind:     item.Kind,
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
		normalized := item.Normalize()
		out = append(out, AttachmentMeta{
			ID:       normalized.ID,
			Name:     normalized.Name,
			MIMEType: normalized.MIMEType,
			Bytes:    item.Bytes,
			SHA256:   normalized.SHA256,
			Kind:     normalized.Kind,
			Width:    item.Width,
			Height:   item.Height,
		})
	}
	return out
}

func attachmentFingerprintIdentities(items []AttachmentMeta) []string {
	identities := make([]string, 0, len(items))
	for _, item := range items {
		identity := strings.TrimSpace(item.ID)
		if identity == "" {
			identity = strings.TrimSpace(item.SHA256)
		}
		if identity == "" {
			continue
		}
		identities = append(identities, identity)
	}
	return identities
}
