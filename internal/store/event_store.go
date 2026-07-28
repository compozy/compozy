package store

import (
	"context"
	"time"
)

// EventReader reads durable session events and turn history.
type EventReader interface {
	Query(ctx context.Context, query EventQuery) ([]SessionEvent, error)
	History(ctx context.Context, query EventQuery) ([]TurnHistory, error)
}

// EventWriter appends session events and token usage.
type EventWriter interface {
	Record(ctx context.Context, event SessionEvent) error
	RecordTokenUsage(ctx context.Context, usage TokenUsage) error
}

// EventCloser releases a session event-store handle.
type EventCloser interface {
	Close(ctx context.Context) error
}

// EventReadCloser is the complete contract exposed by read-only event stores.
type EventReadCloser interface {
	EventReader
	EventCloser
}

// EventMetadata is the content-excluded projection of a durable session event.
type EventMetadata struct {
	ID        string
	SessionID string
	Sequence  int64
	TurnID    string
	Type      string
	AgentName string
	Timestamp time.Time
}

// EventMetadataReader reads the sequence and projection needed by replay cursors.
type EventMetadataReader interface {
	MaxEventSequence(ctx context.Context) (int64, error)
	QueryEventMetadata(ctx context.Context, query EventQuery) ([]EventMetadata, error)
}

// EventMetadataReadCloser owns a read-only metadata handle for one session.
type EventMetadataReadCloser interface {
	EventMetadataReader
	EventCloser
}

// SessionEventMetadataOpener opens the metadata reader for one known session.
type SessionEventMetadataOpener func(
	ctx context.Context,
	sessionID string,
) (EventMetadataReadCloser, error)

// EventRecorder is the complete mutable per-session event-store contract.
type EventRecorder interface {
	EventReadCloser
	EventWriter
}

// EventArchiver marks closed replay spans without deleting their forensic rows.
type EventArchiver interface {
	ArchiveEvents(ctx context.Context, request EventArchiveRequest) (EventArchiveResult, error)
}
