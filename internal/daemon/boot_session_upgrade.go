package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/sessiondb"
)

// bootSessionWorkers bounds per-session boot work that has to open a session database.
const bootSessionWorkers = 4

type sessionDatabaseUpgrader interface {
	UpgradeSessionDatabase(context.Context, string) error
}

type bootSessionUpgradeOutcome int

const (
	bootSessionUpgradeSkipped bootSessionUpgradeOutcome = iota
	bootSessionUpgradeUnchanged
	bootSessionUpgradeOpened
	bootSessionUpgradeRefused
)

// bootSessionUpgradeProgress serializes the restart journal's counts across the worker pool.
type bootSessionUpgradeProgress struct {
	mu        sync.Mutex
	report    globaldb.MigrationProgressFunc
	total     int64
	completed int64
}

func (p *bootSessionUpgradeProgress) start() {
	p.report.Report(globaldb.MigrationProgress{
		Stage: globaldb.MigrationStageSessionHistoryUpgrade, Total: p.total,
	})
}

// advance counts finished sessions. The last one is reported by the stage clearing instead.
func (p *bootSessionUpgradeProgress) advance(count int64) {
	if count <= 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.completed += count
	if p.completed >= p.total {
		return
	}
	p.report.Report(globaldb.MigrationProgress{
		Stage: globaldb.MigrationStageSessionHistoryUpgrade, Completed: p.completed, Total: p.total,
	})
}

// upgradeBootSessionDatabases brings every retained session database to the current schema and
// projection. A session whose files are byte-identical to the ones a previous boot verified
// against the same schema head is not opened again; every other session is verified by a bounded
// worker pool. A coordinated-restart replacement reports each session to the restart journal, so
// the desktop shows real counts while a large history upgrades instead of a frozen install stage.
func upgradeBootSessionDatabases(
	ctx context.Context,
	state *bootState,
	infos []*session.Info,
	progress globaldb.MigrationProgressFunc,
	watermarks *bootSessionWatermarks,
) ([]*session.Info, error) {
	upgrader, ok := state.sessions.(sessionDatabaseUpgrader)
	if !ok {
		return infos, nil
	}
	startedAt := time.Now()
	tracker := &bootSessionUpgradeProgress{report: progress, total: int64(len(infos))}
	tracker.start()
	// The journal clears the stage on every exit so a later boot step never inherits its idle window.
	defer progress.Report(globaldb.MigrationProgress{Stage: globaldb.MigrationStageSessionHistoryUpgrade, Done: true})

	outcomes := make([]bootSessionUpgradeOutcome, len(infos))
	pending := make([]int, 0, len(infos))
	var settled int64
	for index, info := range infos {
		switch {
		case info == nil || strings.TrimSpace(info.ID) == "":
			settled++
		case watermarks.verified(info.ID):
			outcomes[index] = bootSessionUpgradeUnchanged
			settled++
		default:
			pending = append(pending, index)
		}
	}
	tracker.advance(settled)

	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(bootSessionWorkers)
	for _, index := range pending {
		if groupCtx.Err() != nil {
			break
		}
		info := infos[index]
		group.Go(func() error {
			outcome, err := upgradeBootSessionDatabase(groupCtx, state, upgrader, info)
			if err != nil {
				return err
			}
			if outcome == bootSessionUpgradeOpened {
				watermarks.recordVerified(info.ID)
			}
			outcomes[index] = outcome
			tracker.advance(1)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("daemon: retained session database upgrade canceled: %w", err)
	}

	ready := make([]*session.Info, 0, len(infos))
	counts := make(map[bootSessionUpgradeOutcome]int, 4)
	for index, outcome := range outcomes {
		counts[outcome]++
		if outcome == bootSessionUpgradeUnchanged || outcome == bootSessionUpgradeOpened {
			ready = append(ready, infos[index])
		}
	}
	state.logger.InfoContext(ctx, "daemon: retained session databases ready",
		"sessions", len(infos),
		"unchanged_sessions", counts[bootSessionUpgradeUnchanged],
		"opened_sessions", counts[bootSessionUpgradeOpened],
		"refused_sessions", counts[bootSessionUpgradeRefused],
		"duration_ms", time.Since(startedAt).Milliseconds(),
	)
	return ready, nil
}

func upgradeBootSessionDatabase(
	ctx context.Context,
	state *bootState,
	upgrader sessionDatabaseUpgrader,
	info *session.Info,
) (bootSessionUpgradeOutcome, error) {
	if err := ctx.Err(); err != nil {
		return bootSessionUpgradeSkipped, fmt.Errorf("daemon: retained session database upgrade canceled: %w", err)
	}
	err := upgrader.UpgradeSessionDatabase(ctx, info.ID)
	if err == nil {
		return bootSessionUpgradeOpened, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return bootSessionUpgradeSkipped,
			fmt.Errorf("daemon: retained session database upgrade canceled: %w", ctxErr)
	}
	identityRefused := errors.Is(err, sessiondb.ErrSessionDBOwnerMismatch) ||
		errors.Is(err, sessiondb.ErrSessionDBOwnerMissing) ||
		errors.Is(err, sessiondb.ErrSessionDBFamilyChanged)
	if identityRefused &&
		!errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		state.logger.WarnContext(ctx, "daemon: retained session database refused",
			"session_id", info.ID,
			"workspace_id", info.WorkspaceID,
			"error", err,
		)
		return bootSessionUpgradeRefused, nil
	}
	return bootSessionUpgradeSkipped, fmt.Errorf("daemon: upgrade retained session database %q: %w", info.ID, err)
}
