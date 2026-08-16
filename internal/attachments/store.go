package attachments

import (
	"context"
	"io"
	"time"
)

// PutRequest supplies one attachment write and its session scope.
type PutRequest struct {
	WorkspaceID string
	SessionID   string
	Name        string
	Data        []byte
}

// AttachmentReader reads retained attachments within a workspace session scope.
type AttachmentReader interface {
	Open(ctx context.Context, workspaceID string, sessionID string, id string) (io.ReadCloser, AttachmentRef, error)
	Stat(ctx context.Context, workspaceID string, sessionID string, id string) (AttachmentRef, error)
}

// AttachmentWriter mutates retained attachments within a workspace session scope.
type AttachmentWriter interface {
	Put(ctx context.Context, request PutRequest) (AttachmentRef, error)
	Delete(ctx context.Context, workspaceID string, sessionID string, id string) error
}

// AttachmentSweeper applies retention to durable attachments.
type AttachmentSweeper interface {
	Sweep(ctx context.Context) error
}

// Store persists content-addressed session attachments on a durable root.
type Store interface {
	AttachmentReader
	AttachmentWriter
	AttachmentSweeper
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

// FilesystemStoreConfig supplies filesystem attachment store dependencies.
type FilesystemStoreConfig struct {
	Root          string
	Retention     AttachmentRetention
	Limits        StoreLimits
	RetentionPins RetentionPinSource
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
