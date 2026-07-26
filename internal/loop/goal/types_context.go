package goal

import (
	"context"
	"time"

	"github.com/compozy/agh/internal/loop"
)

// ContextUsage is one sequenced provider context-usage observation.
type ContextUsage struct {
	Used       int64
	Size       int64
	Sequence   int64
	Known      bool
	ReportedAt time.Time
}

// RecordContextUsageRequest advances freshness for one exact binding and checkpoint phase.
type RecordContextUsageRequest struct {
	Key                  TurnKey
	ExpectedControlEpoch int64
	ExpectedBindingEpoch int64
	ExpectedPhase        string
	SessionID            string
	BindingHandle        string
	Usage                ContextUsage
}

// ContextHealth reads context pressure and advertised session commands.
type ContextHealth interface {
	Usage(context.Context, loop.ActionSessionBinding) (ContextUsage, error)
	HasAdvertisedCommand(context.Context, loop.ActionSessionBinding, string) (bool, error)
}
