package daemon

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

// bootSessionRestartCandidate reports whether restart capture or history repair can act on a
// session at all. Both read only the session's metadata and events database.
func bootSessionRestartCandidate(info *session.Info) bool {
	if info == nil || strings.TrimSpace(info.ID) == "" {
		return false
	}
	return info.StopReason == store.StopShutdown || bootShouldRepairSession(info)
}

// bootUnsettledSessions drops the sessions whose restart capture and history repair already
// proved to be a no-op for their current files, so neither pass reads them again.
func bootUnsettledSessions(infos []*session.Info, watermarks *bootSessionWatermarks) []*session.Info {
	if watermarks == nil {
		return infos
	}
	unsettled := make([]*session.Info, 0, len(infos))
	for _, info := range infos {
		if bootSessionRestartCandidate(info) && watermarks.settled(info.ID) {
			continue
		}
		unsettled = append(unsettled, info)
	}
	return unsettled
}

// repairBootSessionHistories inspects and repairs interrupted histories with a bounded worker
// pool: each session's eligibility read and its conditional repair run in the same job, so no
// retained history is opened outside the pool. A session the pass inspected without changing or
// diagnosing anything is recorded as settled.
func repairBootSessionHistories(
	ctx context.Context,
	state *bootState,
	infos []*session.Info,
	watermarks *bootSessionWatermarks,
) error {
	startedAt := time.Now()
	var (
		mu       sync.Mutex
		clean    = make([]string, 0)
		repaired int
	)
	candidates := 0
	var group errgroup.Group
	group.SetLimit(bootSessionWorkers)
	for _, info := range infos {
		if ctx.Err() != nil {
			break
		}
		if !bootSessionRestartCandidate(info) {
			continue
		}
		candidates++
		group.Go(func() error {
			outcome, err := repairBootSessionHistory(ctx, state, info)
			if err != nil {
				return err
			}
			mu.Lock()
			defer mu.Unlock()
			if outcome.repaired {
				repaired++
			}
			if outcome.clean {
				clean = append(clean, info.ID)
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("daemon: boot session repair canceled: %w", err)
	}
	for _, id := range clean {
		watermarks.recordSettled(id)
	}
	state.logger.InfoContext(ctx, "daemon: boot session history repair complete",
		"inspected_sessions", candidates,
		"repaired_sessions", repaired,
		"duration_ms", time.Since(startedAt).Milliseconds(),
	)
	return nil
}

// bootSessionHistoryOutcome reports what one session's job did: repaired when the repair ran,
// clean when the session needed nothing or its repair left no error diagnostic.
type bootSessionHistoryOutcome struct {
	repaired bool
	clean    bool
}

// repairBootSessionHistory is one pool job: it reads whether the session needs repair and, when
// it does, repairs it. A per-session failure is logged and isolated; only cancellation aborts.
func repairBootSessionHistory(
	ctx context.Context,
	state *bootState,
	info *session.Info,
) (bootSessionHistoryOutcome, error) {
	if err := ctx.Err(); err != nil {
		return bootSessionHistoryOutcome{}, fmt.Errorf("daemon: boot session repair canceled: %w", err)
	}
	needed, err := bootSessionNeedsRepair(ctx, state.sessions, info)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return bootSessionHistoryOutcome{}, fmt.Errorf("daemon: boot session repair canceled: %w", ctxErr)
		}
		state.logger.Warn("daemon: inspect restart repair failed", "session_id", info.ID, "error", err)
		return bootSessionHistoryOutcome{}, nil
	}
	if !needed {
		return bootSessionHistoryOutcome{clean: true}, nil
	}
	if err := ctx.Err(); err != nil {
		return bootSessionHistoryOutcome{}, fmt.Errorf("daemon: boot session repair canceled: %w", err)
	}
	result, err := state.sessions.RepairSession(ctx, session.RepairOpts{SessionID: info.ID})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return bootSessionHistoryOutcome{}, fmt.Errorf("daemon: boot session repair canceled: %w", ctxErr)
		}
		state.logger.Warn("daemon: boot session repair failed", "session_id", info.ID, "error", err)
		return bootSessionHistoryOutcome{repaired: true}, nil
	}
	logBootSessionRepair(state, result)
	return bootSessionHistoryOutcome{
		repaired: true,
		clean:    repairIssueCount(result, session.RepairSeverityError) == 0,
	}, nil
}
