package session

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// A terminal write can lose SQLite writer admission to a busy writer. Before the
// retry, a lost settlement left the delegation running with no child session and
// no error: nothing woke the parent and nothing ever finalized the row.
const (
	subagentFinalizeAttempts    = 5
	subagentFinalizeAttemptTime = defaultLifecycleTimeout
	subagentFinalizeBackoff     = 100 * time.Millisecond
	subagentFinalizeMaxDelay    = time.Second
	// subagentFinalizeBudget covers every attempt and its backoff, so a caller
	// context that expires mid-attempt cannot cancel the remaining retries.
	subagentFinalizeBudget = 30 * time.Second
)

// finalizeSubagentWithRetry settles a delegation, retrying only the failure mode
// where the store could not admit the write in time.
func (s *subagentService) finalizeSubagentWithRetry(
	ctx context.Context,
	in store.SubagentFinalize,
) (store.SessionSubagent, bool, error) {
	retryCtx, cancelRetry := context.WithTimeout(context.WithoutCancel(ctx), subagentFinalizeBudget)
	defer cancelRetry()
	delay := subagentFinalizeBackoff
	var lastErr error
	for attempt := range subagentFinalizeAttempts {
		if attempt > 0 {
			if !waitForSubagentFinalize(retryCtx, delay) {
				return store.SessionSubagent{}, false, errors.Join(lastErr, retryCtx.Err())
			}
			delay = min(2*delay, subagentFinalizeMaxDelay)
		}
		attemptCtx, cancel := context.WithTimeout(retryCtx, subagentFinalizeAttemptTime)
		settled, changed, err := s.store.FinalizeSubagent(attemptCtx, in)
		cancel()
		if err == nil {
			return settled, changed, nil
		}
		lastErr = err
		if !isSubagentFinalizeRetryable(err) {
			return settled, changed, err
		}
	}
	return store.SessionSubagent{}, false, lastErr
}

// isSubagentFinalizeRetryable reports whether the write failed because admission
// timed out, rather than because the row or the request is invalid.
func isSubagentFinalizeRetryable(err error) bool {
	return errors.Is(err, context.DeadlineExceeded)
}

func waitForSubagentFinalize(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
