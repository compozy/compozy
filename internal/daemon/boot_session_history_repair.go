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

// repairBootSessionHistories repairs interrupted histories with a bounded worker pool. A session
// the pass inspected without changing or diagnosing anything is recorded as settled.
func repairBootSessionHistories(
	ctx context.Context,
	state *bootState,
	infos []*session.Info,
	watermarks *bootSessionWatermarks,
) error {
	startedAt := time.Now()
	clean := make([]string, 0)
	repairs := make([]*session.Info, 0)
	candidates := 0
	for _, info := range infos {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("daemon: boot session repair canceled: %w", err)
		}
		if !bootSessionRestartCandidate(info) {
			continue
		}
		candidates++
		needed, err := bootSessionNeedsRepair(ctx, state.sessions, info)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return fmt.Errorf("daemon: boot session repair canceled: %w", ctxErr)
			}
			state.logger.Warn("daemon: inspect restart repair failed", "session_id", info.ID, "error", err)
			continue
		}
		if !needed {
			clean = append(clean, info.ID)
			continue
		}
		repairs = append(repairs, info)
	}

	var cleanMu sync.Mutex
	var group errgroup.Group
	group.SetLimit(bootSessionWorkers)
	for _, info := range repairs {
		if ctx.Err() != nil {
			break
		}
		group.Go(func() error {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("daemon: boot session repair canceled: %w", err)
			}
			result, repairErr := state.sessions.RepairSession(ctx, session.RepairOpts{SessionID: info.ID})
			if repairErr != nil {
				if ctxErr := ctx.Err(); ctxErr != nil {
					return fmt.Errorf("daemon: boot session repair canceled: %w", ctxErr)
				}
				state.logger.Warn(
					"daemon: boot session repair failed",
					"session_id", info.ID,
					"error", repairErr,
				)
				return nil
			}
			logBootSessionRepair(state, result)
			if repairIssueCount(result, session.RepairSeverityError) == 0 {
				cleanMu.Lock()
				clean = append(clean, info.ID)
				cleanMu.Unlock()
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
		"repaired_sessions", len(repairs),
		"duration_ms", time.Since(startedAt).Milliseconds(),
	)
	return nil
}
