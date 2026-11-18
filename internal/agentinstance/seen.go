package agentinstance

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// LocalOperator identifies the single authenticated human authority of the local daemon.
const LocalOperator = "local-operator"

type SeenResult struct {
	LastSeenAt   time.Time
	Interactions []store.SessionAttentionCommit
}

type SeenStore interface {
	LastSeen(context.Context, string, string) (*time.Time, error)
	MarkSeen(context.Context, string, string, time.Time) (SeenResult, error)
}
