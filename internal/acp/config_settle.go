package acp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/store"
)

const (
	// defaultConfigSettleBudget bounds how long one confirmation waits for the
	// provider's next config_option_update.
	defaultConfigSettleBudget = 2 * time.Second
	configSettleAttempts      = 3
)

// errConfigSettleTimeout reports that no configuration revision arrived inside the
// budget. Callers treat it as a final unconfirmed readback, not as a provider error.
var errConfigSettleTimeout = errors.New("acp: config revision wait timed out")

// WithConfigSettleBudget overrides how long a configuration confirmation waits for
// the provider to publish a newer revision.
func WithConfigSettleBudget(budget time.Duration) Option {
	return func(driver *Driver) {
		if budget > 0 {
			driver.configSettleBudget = budget
		}
	}
}

// applyConfigOption applies one configuration selection and waits for the provider to
// publish it. Providers may apply a value and announce it in a config_option_update
// that arrives after the response returns, so a single readback is not proof of
// rejection. The wait is event-driven: it wakes on the next revision bump instead of
// polling or re-sending on a timer.
func (d *Driver) applyConfigOption(
	ctx context.Context,
	process *AgentProcess,
	apply func(context.Context) error,
	confirmed func() bool,
) error {
	budget := defaultConfigSettleBudget
	if d != nil && d.configSettleBudget > 0 {
		budget = d.configSettleBudget
	}
	for range configSettleAttempts {
		if err := apply(ctx); err != nil {
			return err
		}
		if confirmed() {
			return nil
		}
		// Every send keeps its confirmation window, including the last one: a
		// provider may publish the value after any of them. The response merge
		// itself bumps the revision, so the wait starts after it and only a later
		// update counts as new information.
		since, _ := process.configRevisionAndChange()
		if err := waitForConfigRevision(ctx, process, since, budget); err != nil {
			if errors.Is(err, errConfigSettleTimeout) {
				// No newer revision arrived: re-sending would just repeat the same call.
				return errConfigOptionUnconfirmed
			}
			return err
		}
		if confirmed() {
			return nil
		}
	}
	return errConfigOptionUnconfirmed
}

// waitForConfigRevision blocks until the process publishes a revision newer than
// since, the budget expires, the caller cancels, or the provider exits.
func waitForConfigRevision(
	ctx context.Context,
	process *AgentProcess,
	since uint64,
	budget time.Duration,
) error {
	if process == nil {
		return nil
	}
	timer := time.NewTimer(budget)
	defer timer.Stop()
	for {
		revision, changed := process.configRevisionAndChange()
		if revision != since {
			return nil
		}
		select {
		case <-changed:
		case <-timer.C:
			return errConfigSettleTimeout
		case <-ctx.Done():
			return fmt.Errorf("acp: wait for config revision: %w", ctx.Err())
		case <-process.Done():
			cause := errProcessExited
			if waitErr := process.Wait(); waitErr != nil {
				cause = errors.Join(cause, waitErr)
			}
			return WrapFailure(
				store.FailureProcess,
				"ACP subprocess exited while waiting for a configuration revision",
				fmt.Errorf("acp: wait for config revision: %w", cause),
			)
		}
	}
}
