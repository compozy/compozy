package contract

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/compozy/compozy/internal/session"
)

const (
	PromptAttachmentKindImage = "image"
	PromptAttachmentKindFile  = "file"

	// DefaultPromptAttachmentLimit is the v1 cap until config wires in.
	// seam: config [session.attachments].max_files_per_prompt
	DefaultPromptAttachmentLimit = 10
)

var promptAttachmentIDPattern = regexp.MustCompile(`^att_[0-9a-f]{64}$`)

// ValidatePromptAttachments checks identity, kind, and the caller-provided count cap.
func ValidatePromptAttachments(refs []PromptAttachmentRef, maxCount int) error {
	if maxCount <= 0 {
		maxCount = DefaultPromptAttachmentLimit
	}
	if len(refs) > maxCount {
		return fmt.Errorf("attachments exceed max count %d", maxCount)
	}
	for index, ref := range refs {
		if err := ref.Validate(); err != nil {
			return fmt.Errorf("attachment %d: %w", index, err)
		}
	}
	return nil
}

// Validate checks one attachment reference.
func (r PromptAttachmentRef) Validate() error {
	id := strings.TrimSpace(r.ID)
	if !promptAttachmentIDPattern.MatchString(id) {
		return fmt.Errorf("id %q must match att_<64-hex>", r.ID)
	}
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("name is required")
	}
	switch strings.TrimSpace(r.Kind) {
	case PromptAttachmentKindImage, PromptAttachmentKindFile:
	default:
		return fmt.Errorf("kind %q must be %q or %q", r.Kind, PromptAttachmentKindImage, PromptAttachmentKindFile)
	}
	return nil
}

// AttachmentMetaFromRefs converts wire refs into session attachment metadata.
func AttachmentMetaFromRefs(refs []PromptAttachmentRef) []session.AttachmentMeta {
	if len(refs) == 0 {
		return nil
	}
	out := make([]session.AttachmentMeta, 0, len(refs))
	for _, ref := range refs {
		out = append(out, session.AttachmentMeta{
			ID:       strings.TrimSpace(ref.ID),
			Name:     strings.TrimSpace(ref.Name),
			MIMEType: strings.TrimSpace(ref.MIMEType),
			Bytes:    ref.Bytes,
			SHA256:   strings.TrimSpace(ref.SHA256),
			Kind:     strings.TrimSpace(ref.Kind),
			Width:    ref.Width,
			Height:   ref.Height,
		})
	}
	return out
}

// PromptAttachmentRefsFromMeta converts session attachment metadata into wire refs.
func PromptAttachmentRefsFromMeta(items []session.AttachmentMeta) []PromptAttachmentRef {
	if len(items) == 0 {
		return nil
	}
	out := make([]PromptAttachmentRef, 0, len(items))
	for _, item := range items {
		out = append(out, PromptAttachmentRef{
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
