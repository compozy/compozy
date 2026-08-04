package daemon

import (
	"context"
	"time"

	taskpkg "github.com/compozy/compozy/internal/task"
)

type loopActionLivenessPolicy struct {
	actionTimeout    time.Duration
	silenceWindow    time.Duration
	deathStreakLimit int
	timeoutReason    string
	leaseDuration    time.Duration
}

func (r *loopActionRuntime) actionLivenessPolicyForRun(
	ctx context.Context,
	run taskpkg.Run,
) (loopActionLivenessPolicy, error) {
	actionTimeout, authoredTimeout, err := r.actionTimeoutSpecForRun(ctx, run)
	if err != nil {
		return loopActionLivenessPolicy{}, err
	}
	silenceWindow, err := r.actionSilenceWindowForRun(ctx, run)
	if err != nil {
		return loopActionLivenessPolicy{}, err
	}
	deathStreakLimit, err := r.actionDeathStreakLimitForRun(ctx, run)
	if err != nil {
		return loopActionLivenessPolicy{}, err
	}
	timeoutReason := loopActionReasonNodeTimeout
	if authoredTimeout {
		timeoutReason, err = r.actionTimeoutReasonForRun(ctx, run)
		if err != nil {
			return loopActionLivenessPolicy{}, err
		}
	}
	return loopActionLivenessPolicy{
		actionTimeout:    actionTimeout,
		silenceWindow:    silenceWindow,
		deathStreakLimit: deathStreakLimit,
		timeoutReason:    timeoutReason,
		leaseDuration:    leaseDurationForActionTimeout(actionTimeout),
	}, nil
}
