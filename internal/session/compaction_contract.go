package session

import (
	"context"

	"github.com/compozy/agh/internal/store"
)

// CompactionRequest carries one exact persisted replay span to the memory boundary.
type CompactionRequest struct {
	WorkspaceID  string
	SessionID    string
	AgentName    string
	FromSequence int64
	ToSequence   int64
	Events       []store.SessionEvent
}

// CompactionResult carries the durable summary produced before event archiving.
type CompactionResult struct {
	Summary string
}

// CompactionHandler updates durable memory coverage for one replay span.
type CompactionHandler interface {
	CompactSessionContext(ctx context.Context, request CompactionRequest) (CompactionResult, error)
}
