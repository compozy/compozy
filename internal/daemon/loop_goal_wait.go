package daemon

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

type goalSubagentWakeReader interface {
	HasPendingSubagentWake(context.Context, string, string) (bool, error)
}

// HasLiveWait uses the same fresh authoritative readers as session supervision.
func (r *loopGoalRuntime) HasLiveWait(ctx context.Context, binding loop.ActionSessionBinding) (bool, error) {
	if r.waitSources == nil {
		return false, nil
	}
	if binding.SessionID == "" {
		return false, errors.New("goal wait session is unavailable")
	}
	for _, inspect := range []func(context.Context, string) ([]session.WorkSignal, error){
		r.waitSources.monitors, r.waitSources.children,
	} {
		signals, err := inspect(ctx, binding.SessionID)
		if err != nil {
			return false, err
		}
		if len(signals) > 0 {
			return true, nil
		}
	}
	// Child finalization publishes its terminal before queueing the wake. Keep that
	// handoff, and the wake turn itself, inside the wait so the Goal sees its result.
	reader, ok := r.waitSources.state.registry.(goalSubagentWakeReader)
	if !ok {
		return false, errors.New("goal subagent wake reader is unavailable")
	}
	return reader.HasPendingSubagentWake(ctx, string(binding.WorkspaceID), binding.SessionID)
}

// RecoverWaitSession revives only a daemon-interrupted binding, without submitting a Goal prompt.
func (r *loopGoalRuntime) RecoverWaitSession(ctx context.Context, binding loop.ActionSessionBinding) error {
	if r.loopActionSessionBinder == nil || r.loopActionSessionBinder.sessions == nil {
		return errors.New("goal bound session manager is unavailable")
	}
	sessions := r.loopActionSessionBinder.sessions
	info, err := sessions.Status(ctx, binding.SessionID)
	if err != nil {
		return err
	}
	if info == nil {
		return errors.New("goal bound session is unavailable")
	}
	if info.State != session.StateStopped {
		return nil
	}
	if info.StopReason != store.StopShutdown {
		return errors.New("goal bound session was stopped explicitly")
	}
	resumer, ok := sessions.(interface {
		ResumeWaitingSession(context.Context, string) error
	})
	if !ok {
		return errors.New("goal bound session recovery is unavailable")
	}
	return resumer.ResumeWaitingSession(ctx, binding.SessionID)
}
