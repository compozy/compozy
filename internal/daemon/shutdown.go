package daemon

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/store/globaldb"
)

// RunPurgeResult captures the terminal runs removed by one purge operation.
type RunPurgeResult struct {
	PurgedRunIDs []string
}

// ActiveRunCount returns the number of runs still owned by the live daemon.
func (m *RunManager) ActiveRunCount() int {
	if m == nil {
		return 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.active)
}

// PurgeTerminalRuns deletes terminal run directories and durable index rows
// using the configured retention policy without requiring a live run manager.
func PurgeTerminalRuns(
	ctx context.Context,
	db *globaldb.GlobalDB,
	settings RunLifecycleSettings,
) (RunPurgeResult, error) {
	manager := &RunManager{
		globalDB: db,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
	return manager.Purge(ctx, settings)
}

// Shutdown applies the daemon stop semantics for active runs. Without force it
// returns a conflict while active runs exist. With force it cancels every run
// and waits only up to the configured drain timeout for their terminal cleanup.
func (m *RunManager) Shutdown(ctx context.Context, force bool) error {
	if m == nil {
		return nil
	}

	activeRuns := m.activeSnapshot()
	if len(activeRuns) == 0 {
		return nil
	}
	if !force {
		return apicore.NewProblem(
			http.StatusConflict,
			"daemon_active_runs",
			"daemon has active runs",
			map[string]any{
				"active_run_count": len(activeRuns),
				"run_ids":          activeRunIDs(activeRuns),
			},
			nil,
		)
	}

	for _, run := range activeRuns {
		run.setCloseTimeout(m.shutdownDrainTimeout)
		if run.markCancelRequested() {
			run.cancel()
		}
	}

	waitCtx, cancel := context.WithTimeout(detachContext(ctx), m.shutdownDrainTimeout)
	defer cancel()

	for _, run := range activeRuns {
		select {
		case <-run.done:
		case <-waitCtx.Done():
			return nil
		}
	}
	return nil
}

// Purge deletes terminal run directories and their durable index rows according
// to the configured oldest-first retention policy.
func (m *RunManager) Purge(ctx context.Context, settings RunLifecycleSettings) (RunPurgeResult, error) {
	if m == nil || m.globalDB == nil {
		return RunPurgeResult{}, errors.New("daemon: run manager global db is required")
	}

	listCtx := detachContext(ctx)
	candidates, err := m.globalDB.ListTerminalRunsForPurge(listCtx, globaldb.RunRetentionPolicy{
		KeepTerminalDays: settings.KeepTerminalDays,
		KeepMax:          settings.KeepMax,
		Now:              m.now(),
	})
	if err != nil {
		return RunPurgeResult{}, err
	}

	result := RunPurgeResult{PurgedRunIDs: make([]string, 0, len(candidates))}
	for i := range candidates {
		run := &candidates[i]
		if m.getActive(run.RunID) != nil {
			continue
		}

		runArtifacts, err := model.ResolveHomeRunArtifacts(run.RunID)
		if err != nil {
			return result, err
		}
		if err := os.RemoveAll(runArtifacts.RunDir); err != nil {
			return result, err
		}
		if err := m.globalDB.DeleteRun(listCtx, run.RunID); err != nil {
			return result, err
		}
		result.PurgedRunIDs = append(result.PurgedRunIDs, run.RunID)
	}
	return result, nil
}

func (m *RunManager) activeSnapshot() []*activeRun {
	m.mu.RLock()
	defer m.mu.RUnlock()

	runs := make([]*activeRun, 0, len(m.active))
	for _, run := range m.active {
		runs = append(runs, run)
	}
	return runs
}

func activeRunIDs(runs []*activeRun) []string {
	ids := make([]string, 0, len(runs))
	for _, run := range runs {
		if run == nil {
			continue
		}
		ids = append(ids, run.runID)
	}
	return ids
}
