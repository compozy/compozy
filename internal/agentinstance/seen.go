package agentinstance

import (
	"context"
	"time"
)

// LocalOperator identifies the single authenticated human authority of the local daemon.
const LocalOperator = "local-operator"

type SeenStore interface {
	LastSeen(context.Context, string, string) (*time.Time, error)
	MarkSeen(context.Context, string, string, time.Time) (time.Time, error)
}
