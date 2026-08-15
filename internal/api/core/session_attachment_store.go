package core

import (
	"context"
	"io"

	attachmentspkg "github.com/compozy/compozy/internal/attachments"
)

// SessionAttachmentStore is the attachment persistence surface consumed by API handlers.
type SessionAttachmentStore interface {
	Put(ctx context.Context, workspaceID string, sessionID string, name string, data []byte) (
		attachmentspkg.AttachmentRef,
		error,
	)
	Open(ctx context.Context, workspaceID string, sessionID string, id string) (
		io.ReadCloser,
		attachmentspkg.AttachmentRef,
		error,
	)
	Delete(ctx context.Context, workspaceID string, sessionID string, id string) error
}
