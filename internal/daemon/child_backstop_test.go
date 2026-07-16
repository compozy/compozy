package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/rundb"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestChildBackstopBudget(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		policy model.StallPolicy
		want   time.Duration
	}{
		{
			name:   "Should disarm when the stall policy is disabled",
			policy: model.StallPolicy{Enabled: false, IdleTimeout: time.Minute, ChildTimeout: 2 * time.Minute},
			want:   0,
		},
		{
			name:   "Should disarm when no budget is configured",
			policy: model.StallPolicy{Enabled: true},
			want:   0,
		},
		{
			name:   "Should use the configured child timeout when it exceeds the idle window",
			policy: model.StallPolicy{Enabled: true, IdleTimeout: 3 * time.Minute, ChildTimeout: 6 * time.Minute},
			want:   6 * time.Minute,
		},
		{
			name:   "Should widen a child timeout equal to the idle window",
			policy: model.StallPolicy{Enabled: true, IdleTimeout: 3 * time.Minute, ChildTimeout: 3 * time.Minute},
			want:   6 * time.Minute,
		},
		{
			name:   "Should widen a child timeout below the idle window",
			policy: model.StallPolicy{Enabled: true, IdleTimeout: 3 * time.Minute, ChildTimeout: time.Minute},
			want:   6 * time.Minute,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := childBackstopBudget(tc.policy)
			if got != tc.want {
				t.Fatalf("childBackstopBudget(%#v) = %s, want %s", tc.policy, got, tc.want)
			}
			// ADR-003 nested budgets: whenever the backstop is armed its budget must be
			// strictly greater than the fast watchdog's idle window, so the in-attempt
			// layer always gets the first chance to self-heal.
			if got > 0 && got <= tc.policy.IdleTimeout {
				t.Fatalf("budget %s must be strictly greater than idle timeout %s", got, tc.policy.IdleTimeout)
			}
		})
	}
}

func TestChildLivenessTracksDurableProgress(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)

	t.Run("Should reset the stall clock when the sequence advances", func(t *testing.T) {
		t.Parallel()
		liveness := childLiveness{lastSeen: base}
		if !liveness.advanced(7, base.Add(time.Minute)) {
			t.Fatal("advanced(7) = false, want true for a higher sequence")
		}
		if liveness.lastSeq != 7 || !liveness.lastSeen.Equal(base.Add(time.Minute)) {
			t.Fatalf("liveness = %#v, want sequence 7 seen at base+1m", liveness)
		}
	})

	t.Run("Should not reset the stall clock on a repeated or lower sequence", func(t *testing.T) {
		t.Parallel()
		liveness := childLiveness{lastSeq: 7, lastSeen: base}
		for _, seq := range []uint64{0, 6, 7} {
			if liveness.advanced(seq, base.Add(time.Minute)) {
				t.Fatalf("advanced(%d) = true, want false", seq)
			}
		}
		if !liveness.lastSeen.Equal(base) {
			t.Fatalf("lastSeen = %s, want the clock untouched at %s", liveness.lastSeen, base)
		}
	})

	t.Run("Should report wedged only once the whole budget elapsed without progress", func(t *testing.T) {
		t.Parallel()
		budget := 6 * time.Minute
		liveness := childLiveness{lastSeen: base}
		if liveness.wedged(base.Add(budget-time.Nanosecond), budget) {
			t.Fatal("wedged() = true just under the budget, want false")
		}
		if !liveness.wedged(base.Add(budget), budget) {
			t.Fatal("wedged() = false at the budget, want true")
		}
	})

	t.Run("Should never wedge a child advancing on every observation", func(t *testing.T) {
		t.Parallel()
		budget := 6 * time.Minute
		liveness := childLiveness{lastSeen: base}
		now := base
		// Far more elapsed time than the budget, but progress on every observation.
		for seq := uint64(1); seq <= 100; seq++ {
			now = now.Add(budget / 2)
			if !liveness.advanced(seq, now) {
				t.Fatalf("advanced(%d) = false, want true", seq)
			}
			if liveness.wedged(now, budget) {
				t.Fatalf("wedged() = true after %s of advancing progress, want false", now.Sub(base))
			}
		}
	})

	t.Run("Should stay disarmed when the budget is zero", func(t *testing.T) {
		t.Parallel()
		liveness := childLiveness{lastSeen: base}
		if liveness.wedged(base.Add(24*time.Hour), 0) {
			t.Fatal("wedged() = true with a zero budget, want false")
		}
	})
}

func TestChildStallPolicyFallsBackToDefaults(t *testing.T) {
	t.Parallel()

	policy := childStallPolicy(nil)
	if !policy.Enabled {
		t.Fatal("childStallPolicy(nil).Enabled = false, want the on-by-default policy")
	}
	if childBackstopBudget(policy) <= 0 {
		t.Fatal("childStallPolicy(nil) must arm the backstop")
	}
}

// backstopEnv wires a run manager whose clock and per-run event stores are fully
// under test control, so the durable backstop is exercised without wall time. Every
// goroutine it starts is owned by the env and joined before the stores close.
type backstopEnv struct {
	globalDB *globaldb.GlobalDB
	manager  *RunManager
	stores   map[string]*rundb.RunDB
	cancel   context.CancelFunc
	ctx      context.Context
	wg       sync.WaitGroup
}

func newBackstopEnv(t *testing.T, now func() time.Time, runIDs ...string) *backstopEnv {
	t.Helper()

	paths := mustHomePaths(t)
	t.Setenv("HOME", filepath.Dir(paths.HomeDir))
	db := openDaemonGlobalDB(t, paths)
	workspace := registerDaemonWorkspace(t, db)

	storeRoot := t.TempDir()
	stores := make(map[string]*rundb.RunDB, len(runIDs))
	for _, runID := range runIDs {
		dir := filepath.Join(storeRoot, runID)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", dir, err)
		}
		store, err := rundb.Open(context.Background(), filepath.Join(dir, "run.db"))
		if err != nil {
			t.Fatalf("rundb.Open(%q) error = %v", runID, err)
		}
		t.Cleanup(func() {
			_ = store.Close()
		})
		stores[runID] = store
	}

	manager, err := NewRunManager(RunManagerConfig{
		GlobalDB:         db,
		LifecycleContext: context.Background(),
		Now:              now,
		OpenRunDB: func(_ context.Context, runID string) (*rundb.RunDB, error) {
			store, ok := stores[runID]
			if !ok {
				return nil, fmt.Errorf("no test run store for %q", runID)
			}
			return store, nil
		},
	})
	if err != nil {
		t.Fatalf("NewRunManager() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	env := &backstopEnv{globalDB: db, manager: manager, stores: stores, ctx: ctx, cancel: cancel}
	// Registered after the store cleanups, so it runs first: every env goroutine is
	// stopped and joined before any database it touches is closed.
	t.Cleanup(func() {
		env.cancel()
		env.wg.Wait()
	})
	for _, runID := range runIDs {
		env.seedRunningChild(t, workspace.ID, runID)
	}
	return env
}

func (e *backstopEnv) seedRunningChild(t *testing.T, workspaceID string, runID string) {
	t.Helper()
	if _, err := e.globalDB.PutRun(context.Background(), globaldb.Run{
		RunID:            runID,
		WorkspaceID:      workspaceID,
		ParentRunID:      "parent-run",
		Mode:             runModeTask,
		Status:           runStatusRunning,
		PresentationMode: "stream",
		StartedAt:        time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("PutRun(%q) error = %v", runID, err)
	}
}

// activate registers an active run for the child and settles its global row on the
// given terminal status as soon as the manager cancels it, which is how a real
// child reacts to m.Cancel.
func (e *backstopEnv) activate(t *testing.T, runID string, terminal string) *activeRun {
	t.Helper()
	ctx, cancel := context.WithCancel(e.ctx)
	active := &activeRun{runID: runID, ctx: ctx, cancel: cancel, done: make(chan struct{})}
	e.manager.setActive(active)

	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		defer cancel()
		<-ctx.Done()
		e.settle(runID, terminal)
	}()
	return active
}

// settle drives one child to a terminal status. Errors are ignored on purpose:
// it runs on non-test goroutines, and every caller asserts on the resulting row.
func (e *backstopEnv) settle(runID string, status string) {
	row, err := e.globalDB.GetRun(context.Background(), runID)
	if err != nil || isTerminalRunStatus(row.Status) {
		return
	}
	row.Status = status
	_, _ = e.globalDB.UpdateRun(context.Background(), row)
}

func (e *backstopEnv) appendEvent(runID string, index int) {
	_, _ = e.stores[runID].AppendSyntheticEvent(
		context.Background(),
		eventspkg.EventKindJobQueued,
		kinds.JobQueuedPayload{Index: index},
	)
}

func (e *backstopEnv) sequence(t *testing.T, runID string) uint64 {
	t.Helper()
	seq, err := e.stores[runID].CurrentMaxSequence(context.Background())
	if err != nil {
		t.Fatalf("CurrentMaxSequence(%q) error = %v", runID, err)
	}
	return seq
}

// steppingClock advances by a fixed step on every reading, so elapsed time is a
// function of how often the backstop looked at the clock rather than of wall time
// or goroutine scheduling. onRead optionally records durable progress before the
// reading is returned: because the backstop reads the clock before it reads the
// sequence, such a child is observed as advancing on every single poll, no matter
// how large the step. That is the "any progress resets the window" contract.
// Readings are serialized, so the recorded sequence is strictly monotonic.
type steppingClock struct {
	mu       sync.Mutex
	readings int
	base     time.Time
	step     time.Duration
	onRead   func(reading int)
}

func (c *steppingClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readings++
	if c.onRead != nil {
		c.onRead(c.readings)
	}
	return c.base.Add(time.Duration(c.readings) * c.step)
}

func waitForChildRow(t *testing.T, rows <-chan globaldb.Run) globaldb.Run {
	t.Helper()
	select {
	case row := <-rows:
		return row
	case <-time.After(10 * time.Second):
		t.Fatal("waitForTaskMultiChild never returned; the backstop failed to unblock the join")
		return globaldb.Run{}
	}
}

func (e *backstopEnv) awaitChildAsync(runID string, policy model.StallPolicy) (<-chan globaldb.Run, <-chan error) {
	rows := make(chan globaldb.Run, 1)
	errs := make(chan error, 1)
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		row, err := e.manager.waitForTaskMultiChild(e.ctx, runID, policy)
		errs <- err
		rows <- row
	}()
	return rows, errs
}

func TestWaitForTaskMultiChildReapsWedgedChild(t *testing.T) {
	const runID = "wedged-child"
	// The child never writes a durable event, so its high-water sequence stays at 0
	// while one poll's worth of clock readings burns the whole backstop budget.
	policy := model.StallPolicy{Enabled: true, IdleTimeout: time.Minute, ChildTimeout: 2 * time.Minute}
	clock := &steppingClock{base: time.Date(2026, 7, 8, 9, 0, 0, 0, time.UTC), step: policy.ChildTimeout}
	env := newBackstopEnv(t, clock.Now, runID)
	env.activate(t, runID, runStatusCancelled)

	rows, errs := env.awaitChildAsync(runID, policy)

	row := waitForChildRow(t, rows)
	if err := <-errs; err != nil {
		t.Fatalf("waitForTaskMultiChild() error = %v", err)
	}
	if row.Status != runStatusCancelled {
		t.Fatalf("wedged child status = %q, want %q", row.Status, runStatusCancelled)
	}
	if got := env.sequence(t, runID); got != 0 {
		t.Fatalf("wedged child sequence = %d, want 0", got)
	}
}

func TestWaitForTaskMultiChildNeverReapsAdvancingChild(t *testing.T) {
	const (
		runID           = "advancing-child"
		completeAtRead  = 8
		budgetMultiples = 10
	)
	policy := model.StallPolicy{Enabled: true, IdleTimeout: time.Minute, ChildTimeout: 2 * time.Minute}
	clock := &steppingClock{
		base: time.Date(2026, 7, 8, 9, 0, 0, 0, time.UTC),
		step: budgetMultiples * policy.ChildTimeout,
	}

	var env *backstopEnv
	clock.onRead = func(reading int) {
		if env == nil {
			return
		}
		env.appendEvent(runID, reading)
		if reading >= completeAtRead {
			env.settle(runID, runStatusCompleted)
		}
	}
	env = newBackstopEnv(t, clock.Now, runID)
	active := env.activate(t, runID, runStatusCancelled)

	rows, errs := env.awaitChildAsync(runID, policy)
	row := waitForChildRow(t, rows)
	if err := <-errs; err != nil {
		t.Fatalf("waitForTaskMultiChild() error = %v", err)
	}
	if row.Status != runStatusCompleted {
		t.Fatalf("advancing child status = %q, want %q", row.Status, runStatusCompleted)
	}
	if active.ctx.Err() != nil {
		t.Fatal("the backstop canceled a child that kept advancing its durable sequence")
	}
	if got := env.sequence(t, runID); got < completeAtRead {
		t.Fatalf("advancing child sequence = %d, want at least %d recorded advances", got, completeAtRead)
	}
}

func TestWaitForTaskMultiChildReapsOnlyTheWedgedSibling(t *testing.T) {
	const (
		wedgedID       = "sibling-wedged"
		advancingID    = "sibling-advancing"
		completeAtRead = 8
	)
	policy := model.StallPolicy{Enabled: true, IdleTimeout: time.Minute, ChildTimeout: 2 * time.Minute}
	clock := &steppingClock{base: time.Date(2026, 7, 8, 9, 0, 0, 0, time.UTC), step: policy.ChildTimeout}

	var env *backstopEnv
	// Only the advancing sibling records durable progress; the wedged one stays at
	// sequence 0 while the shared clock runs well past the backstop budget.
	clock.onRead = func(reading int) {
		if env == nil {
			return
		}
		env.appendEvent(advancingID, reading)
		if reading >= completeAtRead {
			env.settle(advancingID, runStatusCompleted)
		}
	}
	env = newBackstopEnv(t, clock.Now, wedgedID, advancingID)
	env.activate(t, wedgedID, runStatusCancelled)
	advancingActive := env.activate(t, advancingID, runStatusCancelled)

	wedgedRows, wedgedErrs := env.awaitChildAsync(wedgedID, policy)
	advancingRows, advancingErrs := env.awaitChildAsync(advancingID, policy)

	wedgedRow := waitForChildRow(t, wedgedRows)
	advancingRow := waitForChildRow(t, advancingRows)
	if err := errors.Join(<-wedgedErrs, <-advancingErrs); err != nil {
		t.Fatalf("waitForTaskMultiChild() error = %v", err)
	}
	if wedgedRow.Status != runStatusCancelled {
		t.Fatalf("wedged sibling status = %q, want %q", wedgedRow.Status, runStatusCancelled)
	}
	if advancingRow.Status != runStatusCompleted {
		t.Fatalf("advancing sibling status = %q, want %q", advancingRow.Status, runStatusCompleted)
	}
	if advancingActive.ctx.Err() != nil {
		t.Fatal("reaping the wedged sibling canceled the advancing one")
	}
	if got := env.sequence(t, wedgedID); got != 0 {
		t.Fatalf("wedged sibling sequence = %d, want 0", got)
	}
}

func TestWaitForTaskMultiChildDisarmedPolicyNeverReaps(t *testing.T) {
	const runID = "unguarded-child"
	// An hour of clock per reading: an armed backstop would reap this silent child on
	// the first poll and return its terminal row. The disabled policy must instead
	// leave the wait entirely to the parent context.
	clock := &steppingClock{base: time.Date(2026, 7, 8, 9, 0, 0, 0, time.UTC), step: time.Hour}
	env := newBackstopEnv(t, clock.Now, runID)
	env.activate(t, runID, runStatusCancelled)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	disabled := model.StallPolicy{Enabled: false, IdleTimeout: time.Minute, ChildTimeout: 2 * time.Minute}
	if _, err := env.manager.waitForTaskMultiChild(ctx, runID, disabled); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waitForTaskMultiChild() error = %v, want the parent deadline", err)
	}
}

// stallBackstopOverrides sets a tight, real-time backstop budget for the
// integration batches below. The in-attempt watchdog never arms in these runs
// because execution is stubbed, so the daemon backstop is the only thing that can
// break the wedged child out of the join.
func stallBackstopOverrides(runID string) string {
	return fmt.Sprintf(
		`{"run_id":%q,"stall":{"enabled":true,"timeout":"200ms","child_timeout":"500ms"}}`,
		runID,
	)
}

func TestTaskMultiParallelBackstopUnblocksBatchWithWedgedChild(t *testing.T) {
	t.Parallel()
	t.Run("Should reap the wedged child and let siblings finish the parallel batch", func(t *testing.T) {
		requireGitForTaskMulti(t)
		const parentRunID = "task-multi-parallel-backstop"
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			buildRunID: taskMultiRunIDBuilder(parentRunID),
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
				if cfg.Name == "wedge" {
					// A wedged agent: alive, but emitting nothing durable, forever.
					<-ctx.Done()
					return ctx.Err()
				}
				return nil
			},
		})
		writeTaskMultiWorkflow(t, env, "alpha", "pending")
		writeTaskMultiWorkflow(t, env, "wedge", "pending")
		commitTaskMultiGitWorkspace(t, env.workspaceRoot)

		run, err := env.manager.StartTaskRunMultiple(
			context.Background(),
			env.workspaceRoot,
			apicore.TaskRunMultipleRequest{
				Workspace:        env.workspaceRoot,
				Slugs:            []string{"alpha", "wedge"},
				Mode:             "parallel",
				ParallelLimit:    2,
				PresentationMode: defaultPresentationMode,
				RuntimeOverrides: rawJSON(t, stallBackstopOverrides(parentRunID)),
			},
		)
		if err != nil {
			t.Fatalf("StartTaskRunMultiple(parallel) error = %v", err)
		}

		// The parent join returns only because the backstop reaped the wedged child.
		parentRow := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
			return isTerminalRunStatus(row.Status)
		})
		if parentRow.Status != runStatusFailed {
			t.Fatalf("parent status = %q, want %q", parentRow.Status, runStatusFailed)
		}
		requireTaskMultiChildRow(t, env, "child-alpha", run.RunID, runStatusCompleted)
		requireTaskMultiChildRow(t, env, "child-wedge", run.RunID, runStatusCancelled)
	})
}

func TestTaskMultiEnqueuedBackstopUnblocksQueueWithWedgedChild(t *testing.T) {
	t.Parallel()
	t.Run("Should reap the wedged child so the enqueued batch reaches a terminal state", func(t *testing.T) {
		const parentRunID = "task-multi-enqueued-backstop"
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			buildRunID: taskMultiRunIDBuilder(parentRunID),
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
				if cfg.Name == "wedge" {
					<-ctx.Done()
					return ctx.Err()
				}
				return nil
			},
		})
		writeTaskMultiWorkflow(t, env, "wedge", "pending")
		writeTaskMultiWorkflow(t, env, "beta", "pending")

		run, err := env.manager.StartTaskRunMultiple(
			context.Background(),
			env.workspaceRoot,
			apicore.TaskRunMultipleRequest{
				Workspace:        env.workspaceRoot,
				Slugs:            []string{"wedge", "beta"},
				Mode:             "enqueued",
				PresentationMode: defaultPresentationMode,
				RuntimeOverrides: rawJSON(t, stallBackstopOverrides(parentRunID)),
			},
		)
		if err != nil {
			t.Fatalf("StartTaskRunMultiple(enqueued) error = %v", err)
		}

		parentRow := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
			return isTerminalRunStatus(row.Status)
		})
		if parentRow.Status != runStatusFailed {
			t.Fatalf("parent status = %q, want %q", parentRow.Status, runStatusFailed)
		}
		requireTaskMultiChildRow(t, env, "child-wedge", run.RunID, runStatusCancelled)
		// Enqueued mode is fail-fast: reaping the head of the queue resolves the batch
		// instead of hanging it, and the remaining item is reported as canceled.
		snapshot, err := env.manager.RunMultipleSnapshot(context.Background(), run.RunID)
		if err != nil {
			t.Fatalf("RunMultipleSnapshot() error = %v", err)
		}
		assertTaskMultiItems(t, snapshot.Items, []apicore.TaskRunMultipleItem{
			{Slug: "wedge", Status: taskMultiItemStatusCanceled, RunID: "child-wedge"},
			{Slug: "beta", Status: taskMultiItemStatusCanceled},
		})
	})
}
