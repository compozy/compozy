package goal

import (
	"context"
	"math"
	"sync/atomic"

	"github.com/compozy/agh/internal/loop"
)

type usageTracker struct {
	cumulative atomic.Int64
	reported   atomic.Bool
	sink       loop.ActionUsageReporter
}

func newUsageTracker(floor int64, sink loop.ActionUsageReporter) *usageTracker {
	tracker := &usageTracker{sink: sink}
	if floor < 0 {
		floor = 0
	}
	tracker.cumulative.Store(floor)
	tracker.reported.Store(floor > 0)
	return tracker
}

func (t *usageTracker) snapshot() (int64, bool) {
	if t == nil {
		return 0, false
	}
	return t.cumulative.Load(), t.reported.Load()
}

func (t *usageTracker) operationBase() int64 {
	value, _ := t.snapshot()
	return value
}

func (t *usageTracker) operationReporter(base int64) loop.ActionUsageReporter {
	return loop.ActionUsageReporterFunc(func(operationTokens int64) {
		t.observeOperation(base, operationTokens, true)
	})
}

func (t *usageTracker) observeOperation(base int64, operationTokens int64, reported bool) {
	if t == nil || !reported || base < 0 || operationTokens < 0 {
		return
	}
	candidate := int64(math.MaxInt64)
	if operationTokens <= math.MaxInt64-base {
		candidate = base + operationTokens
	}
	for {
		current := t.cumulative.Load()
		if candidate <= current {
			t.reported.Store(true)
			return
		}
		if t.cumulative.CompareAndSwap(current, candidate) {
			t.reported.Store(true)
			if t.sink != nil {
				t.sink.ReportActionTokensUsed(candidate)
			}
			return
		}
	}
}

func (e *Executor) flushBudget(
	ctx context.Context,
	segment *segmentState,
	boundary BudgetBoundary,
	phase string,
	turn int,
	operationID string,
	operationBase int64,
) (BudgetDecision, error) {
	liveTokens, reported := segment.usage.snapshot()
	return e.budget.FlushAndCheck(ctx, BudgetBoundarySnapshot{
		Key:                  segment.key,
		TaskRunID:            segment.input.CorrelationID,
		ExpectedControlEpoch: segment.checkpoint.ControlEpoch,
		ExpectedBindingEpoch: segment.checkpoint.BindingEpoch,
		ExpectedPhase:        segment.checkpoint.Phase,
		ExpectedQueueEntryID: segment.checkpoint.QueueEntryID,
		ExpectedPromptID:     segment.checkpoint.PromptID,
		Boundary:             boundary,
		Phase:                phase,
		Turn:                 int64(turn),
		OperationID:          operationID,
		OperationBaseTokens:  operationBase,
		LiveTokensUsed:       liveTokens,
		TokensReported:       reported,
	})
}
