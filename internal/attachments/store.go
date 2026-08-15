package attachments

import (
	"context"
	"io"
	"time"
)

// Store persists content-addressed session attachments on a durable root.
type Store interface {
	Put(ctx context.Context, workspaceID string, sessionID string, name string, data []byte) (AttachmentRef, error)
	Open(ctx context.Context, workspaceID string, sessionID string, id string) (io.ReadCloser, AttachmentRef, error)
	Stat(ctx context.Context, workspaceID string, sessionID string, id string) (AttachmentRef, error)
	Delete(ctx context.Context, workspaceID string, sessionID string, id string) error
	Sweep(ctx context.Context) error
}

// ScopeLease serializes attachment mutations with session and workspace deletion.
type ScopeLease interface {
	AcquireScopeLease(ctx context.Context, workspaceID string, sessionID string) (ScopeLeaseGuard, error)
}

// ScopeLeaseGuard owns one exclusive attachment scope until Release.
// MarkDeleted permanently fences new writes for that scope in this store instance.
type ScopeLeaseGuard interface {
	MarkDeleted()
	Release()
}

// StoreLimits carries operator-configured admission bounds into the filesystem store.
type StoreLimits struct {
	MaxFileBytes int64
	AllowedMIME  []string
}

// RetentionPinSource returns durable attachment IDs that retention must preserve.
type RetentionPinSource interface {
	PinnedAttachmentIDs(ctx context.Context, sessionID string) (map[string]struct{}, error)
}

type attachmentMeta struct {
	Name      string    `json:"name"`
	MIMEType  string    `json:"mime_type"`
	Bytes     int64     `json:"bytes"`
	SHA256    string    `json:"sha256"`
	Kind      string    `json:"kind"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	CreatedAt time.Time `json:"created_at"`
}
