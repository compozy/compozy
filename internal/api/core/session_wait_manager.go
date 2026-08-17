package core

import (
	"context"

	"github.com/compozy/compozy/internal/session"
)

// SessionWaitManager owns bounded, gapless session badge waits.
type SessionWaitManager interface {
	WaitForBadge(context.Context, session.WaitRequest) (session.WaitOutcome, error)
}
