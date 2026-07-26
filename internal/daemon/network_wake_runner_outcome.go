package daemon

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
)

func (r *networkWakeRunner) collectWakeOutcome(
	ctx context.Context,
	events <-chan acp.AgentEvent,
	promptErr error,
	startedAt time.Time,
) store.NetworkWakeOutcome {
	outcome := store.NetworkWakeOutcome{State: store.NetworkWakeStateFailed}
	if promptErr != nil {
		outcome.Reason = "network wake prompt failed"
		return r.finishWakeUsage(outcome, acp.TokenUsage{}, startedAt)
	}
	var usage acp.TokenUsage
	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				outcome.State = store.NetworkWakeStateDeadlineExceeded
				outcome.Reason = "network wake deadline exceeded"
			} else {
				outcome.State = store.NetworkWakeStateCanceled
				outcome.Reason = "network wake canceled"
			}
			return r.finishWakeUsage(outcome, usage, startedAt)
		case event, ok := <-events:
			if !ok {
				outcome.Reason = "network wake event stream closed without terminal event"
				return r.finishWakeUsage(outcome, usage, startedAt)
			}
			if event.Usage != nil {
				usage = usage.Merge(*event.Usage)
			}
			switch event.Type {
			case acp.EventTypeError:
				outcome.Reason = "network wake prompt failed"
				return r.finishWakeUsage(outcome, usage, startedAt)
			case acp.EventTypeDone:
				if event.PromptStopReason == acp.PromptStopReasonCancelled {
					outcome.State = store.NetworkWakeStateCanceled
					outcome.Reason = "network wake canceled"
				} else {
					outcome.State = store.NetworkWakeStateSucceeded
				}
				return r.finishWakeUsage(outcome, usage, startedAt)
			}
		}
	}
}

func (r *networkWakeRunner) finishWakeUsage(
	outcome store.NetworkWakeOutcome,
	usage acp.TokenUsage,
	startedAt time.Time,
) store.NetworkWakeOutcome {
	outcome.ActualWallTime = max(r.now().UTC().Sub(startedAt), 0)
	if usage.InputTokens == nil || usage.OutputTokens == nil {
		outcome.UsageState = store.NetworkWakeUsageUnavailable
		return outcome
	}
	outcome.UsageState = store.NetworkWakeUsageActual
	outcome.ActualInputTokens = max(*usage.InputTokens, 0)
	outcome.ActualOutputTokens = max(*usage.OutputTokens, 0)
	return outcome
}
