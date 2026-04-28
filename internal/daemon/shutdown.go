package daemon

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/store/globaldb"
)

// RunPurgeResult captures the terminal runs removed by one purge operation.
type RunPurgeResult struct {
	PurgedRunIDs []string
}

type journalDropTotals struct {
	terminal    uint64
	nonTerminal uint64
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

// ActiveRunCountsByMode returns the current active-run breakdown by run mode.
func (m *RunManager) ActiveRunCountsByMode() map[string]int {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	counts := make(map[string]int, len(m.active))
	for _, active := range m.active {
		if active == nil {
			continue
		}
		counts[strings.TrimSpace(active.mode)]++
	}
	return counts
}

// TerminalTotalsByModeAndStatus reports daemon-lifetime terminal outcome totals
// keyed by mode then status.
func (m *RunManager) TerminalTotalsByModeAndStatus() map[string]map[string]uint64 {
	if m == nil {
		return nil
	}
	m.metricsMu.RLock()
	defer m.metricsMu.RUnlock()

	out := make(map[string]map[string]uint64)
	for key, total := range m.terminalTotals {
		mode, status, ok := splitRunManagerMetricKey(key)
		if !ok {
			continue
		}
		if out[mode] == nil {
			out[mode] = make(map[string]uint64)
		}
		out[mode][status] = total
	}
	return out
}

// ACPStallTotalsByMode reports daemon-lifetime ACP stall counts keyed by mode.
func (m *RunManager) ACPStallTotalsByMode() map[string]uint64 {
	if m == nil {
		return nil
	}
	m.metricsMu.RLock()
	defer m.metricsMu.RUnlock()

	out := make(map[string]uint64, len(m.acpStallTotals))
	for mode, total := range m.acpStallTotals {
		out[mode] = total
	}
	return out
}

// JournalSubmitDropTotals reports daemon-lifetime journal submit drop counts
// grouped by dropped event kind.
func (m *RunManager) JournalSubmitDropTotals() (uint64, uint64) {
	if m == nil {
		return 0, 0
	}
	m.metricsMu.RLock()
	defer m.metricsMu.RUnlock()
	return m.journalTerminalDrops, m.journalNonTerminalDrops
}

// IncompleteRunCount reports how many runs this daemon has observed with sticky
// persisted integrity issues.
func (m *RunManager) IncompleteRunCount() int {
	if m == nil {
		return 0
	}
	m.metricsMu.RLock()
	defer m.metricsMu.RUnlock()
	return len(m.incompleteRunIDs)
}

func (m *RunManager) recordTerminalOutcome(mode string, status string) {
	if m == nil {
		return
	}
	metricKey := joinRunManagerMetricKey(mode, status)
	if metricKey == "" {
		return
	}
	m.metricsMu.Lock()
	defer m.metricsMu.Unlock()
	m.terminalTotals[metricKey]++
}

func (m *RunManager) recordJournalDropTotals(runID string, terminal uint64, nonTerminal uint64) {
	if m == nil || (terminal == 0 && nonTerminal == 0) {
		return
	}
	trimmedRunID := strings.TrimSpace(runID)
	if trimmedRunID == "" {
		return
	}
	m.metricsMu.Lock()
	defer m.metricsMu.Unlock()
	if m.journalDropsByRun == nil {
		m.journalDropsByRun = make(map[string]journalDropTotals)
	}
	previous := m.journalDropsByRun[trimmedRunID]
	if terminal > previous.terminal {
		m.journalTerminalDrops += terminal - previous.terminal
		previous.terminal = terminal
	}
	if nonTerminal > previous.nonTerminal {
		m.journalNonTerminalDrops += nonTerminal - previous.nonTerminal
		previous.nonTerminal = nonTerminal
	}
	m.journalDropsByRun[trimmedRunID] = previous
}

func (m *RunManager) recordIncompleteRun(runID string) {
	if m == nil {
		return
	}
	trimmedRunID := strings.TrimSpace(runID)
	if trimmedRunID == "" {
		return
	}
	m.metricsMu.Lock()
	defer m.metricsMu.Unlock()
	m.incompleteRunIDs[trimmedRunID] = struct{}{}
}

func joinRunManagerMetricKey(left string, right string) string {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return ""
	}
	// Use NUL as the separator because daemon run modes and statuses are
	// normalized identifiers, so this avoids delimiter collisions without
	// needing extra escaping or tuple allocations.
	return left + "\x00" + right
}

func splitRunManagerMetricKey(raw string) (string, string, bool) {
	parts := strings.SplitN(raw, "\x00", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	if parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
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
		return errors.Join(m.closeRunDBCache(ctx), m.closeWorkspaceEventBus(ctx))
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

	waitCtx, cancel := boundedLifecycleContext(ctx, m.shutdownDrainTimeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		m.runWG.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-waitCtx.Done():
	}
	return errors.Join(m.closeRunDBCache(ctx), m.closeWorkspaceEventBus(ctx))
}

func (m *RunManager) closeWorkspaceEventBus(ctx context.Context) error {
	if m == nil || m.workspaceEvents == nil {
		return nil
	}
	return m.workspaceEvents.Close(ctx)
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
	purgedRunIDs := make([]string, 0, len(candidates))
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
		purgedRunIDs = append(purgedRunIDs, run.RunID)
	}
	if err := m.globalDB.DeleteRuns(listCtx, purgedRunIDs); err != nil {
		return result, err
	}
	result.PurgedRunIDs = append(result.PurgedRunIDs, purgedRunIDs...)
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
