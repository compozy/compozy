package daemon

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/session"
)

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
	return false, nil
}
