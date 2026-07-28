package goal

import (
	"context"
)

func (e *Executor) recordContextUsage(
	ctx context.Context,
	segment *segmentState,
	usage ContextUsage,
) (bool, error) {
	if !usage.Known {
		return false, nil
	}
	updated, err := e.store.RecordContextUsage(ctx, RecordContextUsageRequest{
		Key:                  segment.key,
		ExpectedControlEpoch: segment.checkpoint.ControlEpoch,
		ExpectedBindingEpoch: segment.checkpoint.BindingEpoch,
		ExpectedPhase:        segment.checkpoint.Phase,
		SessionID:            segment.binding.SessionID,
		BindingHandle:        segment.binding.Handle,
		Usage:                usage,
	})
	if err != nil {
		return false, err
	}
	segment.checkpoint = updated
	return updated.ContextState == contextStateKnown && updated.UsageSequence != nil &&
		*updated.UsageSequence == usage.Sequence, nil
}

func contextUsageSequence(usage *ContextUsage) *int64 {
	if usage == nil || !usage.Known {
		return nil
	}
	sequence := usage.Sequence
	return &sequence
}
