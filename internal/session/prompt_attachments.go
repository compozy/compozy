package session

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	attachmentspkg "github.com/compozy/compozy/internal/attachments"
)

var (
	// ErrPromptImagesUnsupported reports an image prompt rejected by negotiated ACP capabilities.
	ErrPromptImagesUnsupported = errors.New("session: prompt images unsupported")
	// ErrPromptFilesUnsupported reports an embedded file prompt rejected by negotiated ACP capabilities.
	ErrPromptFilesUnsupported = errors.New("session: prompt files unsupported")
	// ErrPromptAttachmentDigestMismatch reports attachment bytes that differ from admitted metadata.
	ErrPromptAttachmentDigestMismatch = errors.New("session: prompt attachment digest mismatch")
)

// AttachmentOpener is the attachment storage surface consumed during prompt dispatch.
type AttachmentOpener interface {
	Open(
		ctx context.Context,
		workspaceID string,
		sessionID string,
		id string,
	) (io.ReadCloser, attachmentspkg.AttachmentRef, error)
}

func (m *Manager) resolvePromptAttachments(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	metas []AttachmentMeta,
	caps acp.Caps,
) ([]acp.PromptAttachment, error) {
	if len(metas) == 0 {
		return nil, nil
	}
	if err := validatePromptAttachmentCaps(metas, caps); err != nil {
		return nil, err
	}
	if m.attachmentOpener == nil {
		return nil, errors.New("session: attachment opener is required")
	}

	resolved := make([]acp.PromptAttachment, 0, len(metas))
	for _, meta := range metas {
		attachment, err := m.readPromptAttachment(ctx, workspaceID, sessionID, meta)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, attachment)
	}
	return resolved, nil
}

func validatePromptAttachmentCaps(metas []AttachmentMeta, caps acp.Caps) error {
	for _, meta := range metas {
		mimeType := strings.ToLower(strings.TrimSpace(meta.MIMEType))
		switch {
		case strings.HasPrefix(mimeType, "image/") && !caps.PromptImage:
			return fmt.Errorf(
				"the selected agent does not accept image input; choose a multimodal agent, remove the attachments, or send text only: %w",
				ErrPromptImagesUnsupported,
			)
		case mimeType == "application/pdf" && !caps.PromptEmbeddedContext:
			return fmt.Errorf(
				"the selected agent does not accept file input; choose an agent with embedded context, remove the attachments, or send text only: %w",
				ErrPromptFilesUnsupported,
			)
		}
	}
	return nil
}

func (m *Manager) readPromptAttachment(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	meta AttachmentMeta,
) (acp.PromptAttachment, error) {
	reader, ref, err := m.attachmentOpener.Open(ctx, workspaceID, sessionID, strings.TrimSpace(meta.ID))
	if err != nil {
		return acp.PromptAttachment{}, fmt.Errorf(
			"session: open prompt attachment %q: %w",
			strings.TrimSpace(meta.Name),
			err,
		)
	}
	data, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil || closeErr != nil {
		return acp.PromptAttachment{}, errors.Join(
			wrapPromptAttachmentReadError(meta.Name, readErr),
			wrapPromptAttachmentCloseError(meta.Name, closeErr),
		)
	}

	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	if expected := strings.ToLower(strings.TrimSpace(meta.SHA256)); expected == "" || digest != expected {
		return acp.PromptAttachment{}, fmt.Errorf(
			"session: verify prompt attachment %q: %w",
			strings.TrimSpace(meta.Name),
			ErrPromptAttachmentDigestMismatch,
		)
	}

	name := strings.TrimSpace(meta.Name)
	if name == "" {
		name = strings.TrimSpace(ref.Name)
	}
	mimeType := strings.TrimSpace(meta.MIMEType)
	if mimeType == "" {
		mimeType = strings.TrimSpace(ref.MIMEType)
	}
	kind := strings.TrimSpace(meta.Kind)
	if kind == "" {
		kind = strings.TrimSpace(ref.Kind)
	}
	return acp.PromptAttachment{Name: name, MIMEType: mimeType, Kind: kind, Data: data}, nil
}

func wrapPromptAttachmentReadError(name string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("session: read prompt attachment %q: %w", strings.TrimSpace(name), err)
}

func wrapPromptAttachmentCloseError(name string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("session: close prompt attachment %q: %w", strings.TrimSpace(name), err)
}
