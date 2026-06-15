package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/core/model"
	workspacecfg "github.com/compozy/compozy/internal/core/workspace"
	"github.com/compozy/compozy/internal/store/globaldb"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestRunManagerTaskRunMultipleRunsChildrenSequentially(t *testing.T) {
	t.Parallel()

	t.Run("Should run children sequentially", func(t *testing.T) {
		started := make(chan string, 2)
		releaseAlpha := make(chan struct{})
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			buildRunID: taskMultiRunIDBuilder("task-multi-sequential"),
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
				started <- cfg.Name + ":" + cfg.RunID
				if cfg.Name != "alpha" {
					return nil
				}
				select {
				case <-releaseAlpha:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			},
		})
		writeTaskMultiWorkflow(t, env, "alpha", "pending")
		writeTaskMultiWorkflow(t, env, "beta", "pending")

		parent := startTaskMultiRun(t, env, "task-multi-sequential", []string{"alpha", "beta"})
		if parent.Mode != runModeTaskMulti {
			t.Fatalf("parent mode = %q, want %q", parent.Mode, runModeTaskMulti)
		}
		waitForString(t, started, "alpha:child-alpha")
		waitForCondition(t, 5*time.Second, "task_multi active-run accounting", func() bool {
			counts := env.manager.ActiveRunCountsByMode()
			return env.manager.ActiveRunCount() == 2 && counts[runModeTaskMulti] == 1 && counts[runModeTask] == 1
		})
		assertNoTaskMultiStart(t, started, "beta before alpha completes")

		close(releaseAlpha)
		waitForString(t, started, "beta:child-beta")
		parentRow := waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
			return row.Status == runStatusCompleted
		})
		if parentRow.Mode != runModeTaskMulti {
			t.Fatalf("parent row mode = %q, want %q", parentRow.Mode, runModeTaskMulti)
		}

		alpha := requireTaskMultiChildRow(t, env, "child-alpha", parent.RunID, runStatusCompleted)
		beta := requireTaskMultiChildRow(t, env, "child-beta", parent.RunID, runStatusCompleted)
		if alpha.Mode != runModeTask || beta.Mode != runModeTask {
			t.Fatalf("child modes = %q/%q, want %q", alpha.Mode, beta.Mode, runModeTask)
		}

		runs, err := env.manager.List(
			context.Background(),
			apicore.RunListQuery{Workspace: env.workspaceRoot, Limit: 10},
		)
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(runs) != 3 {
			t.Fatalf("run count = %d, want parent plus two children", len(runs))
		}
		if !hasRunMode(runs, runModeTaskMulti) {
			t.Fatalf("runs missing %q mode: %#v", runModeTaskMulti, runs)
		}

		snapshot, err := env.manager.RunMultipleSnapshot(context.Background(), parent.RunID)
		if err != nil {
			t.Fatalf("RunMultipleSnapshot() error = %v", err)
		}
		wantItems := []apicore.TaskRunMultipleItem{
			{Slug: "alpha", Status: taskMultiItemStatusCompleted, RunID: "child-alpha"},
			{Slug: "beta", Status: taskMultiItemStatusCompleted, RunID: "child-beta"},
		}
		assertTaskMultiItems(t, snapshot.Items, wantItems)
		requireRunEvent(t, parent.RunID, eventspkg.EventKindTaskRunMultipleStarted)
		requireRunEvent(t, parent.RunID, eventspkg.EventKindTaskRunMultipleItemQueued)
		requireRunEvent(t, parent.RunID, eventspkg.EventKindTaskRunMultipleChildStarted)
		requireRunEvent(t, parent.RunID, eventspkg.EventKindTaskRunMultipleChildCompleted)
		requireRunEvent(t, parent.RunID, eventspkg.EventKindTaskRunMultipleQueueCompleted)
	})
}

func TestRunManagerTaskRunMultipleStopsOnFirstChildFailure(t *testing.T) {
	t.Parallel()

	t.Run("Should stop on first child failure", func(t *testing.T) {
		var betaStarted bool
		var mu sync.Mutex
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			buildRunID: taskMultiRunIDBuilder("task-multi-failure"),
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(_ context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
				if cfg.Name == "beta" {
					mu.Lock()
					betaStarted = true
					mu.Unlock()
					return nil
				}
				return errors.New("alpha failed")
			},
		})
		writeTaskMultiWorkflow(t, env, "alpha", "pending")
		writeTaskMultiWorkflow(t, env, "beta", "pending")

		parent := startTaskMultiRun(t, env, "task-multi-failure", []string{"alpha", "beta"})
		parentRow := waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
			return row.Status == runStatusFailed
		})
		if !strings.Contains(parentRow.ErrorText, "alpha failed") {
			t.Fatalf("parent error = %q, want child failure text", parentRow.ErrorText)
		}
		requireTaskMultiChildRow(t, env, "child-alpha", parent.RunID, runStatusFailed)
		if _, err := env.globalDB.GetRun(context.Background(), "child-beta"); !errors.Is(err, globaldb.ErrRunNotFound) {
			t.Fatalf("GetRun(child-beta) error = %v, want ErrRunNotFound", err)
		}
		mu.Lock()
		started := betaStarted
		mu.Unlock()
		if started {
			t.Fatal("beta child started after first child failed")
		}

		snapshot, err := env.manager.RunMultipleSnapshot(context.Background(), parent.RunID)
		if err != nil {
			t.Fatalf("RunMultipleSnapshot() error = %v", err)
		}
		wantItems := []apicore.TaskRunMultipleItem{
			{Slug: "alpha", Status: taskMultiItemStatusFailed, RunID: "child-alpha", ErrorText: "alpha failed"},
			{Slug: "beta", Status: taskMultiItemStatusCanceled, ErrorText: "alpha failed"},
		}
		assertTaskMultiItems(t, snapshot.Items, wantItems)
		requireRunEvent(t, parent.RunID, eventspkg.EventKindTaskRunMultipleItemCanceled)
		requireRunEvent(t, parent.RunID, eventspkg.EventKindTaskRunMultipleQueueCanceled)
	})
}

func TestRunManagerTaskRunMultipleStartChildFailureCancelsQueuedItems(t *testing.T) {
	t.Parallel()

	t.Run("Should mark failed child and cancel queued items when child start fails", func(t *testing.T) {
		t.Parallel()

		parentRunID := "task-multi-start-child-failure"
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			buildRunID: func(cfg *model.RuntimeConfig) (string, error) {
				if cfg != nil && strings.TrimSpace(cfg.ParentRunID) == parentRunID {
					return "", errors.New("child id allocation failed")
				}
				return taskMultiRunIDBuilder(parentRunID)(cfg)
			},
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
		})
		writeTaskMultiWorkflow(t, env, "alpha", "pending")
		writeTaskMultiWorkflow(t, env, "beta", "pending")

		parent := startTaskMultiRun(t, env, parentRunID, []string{"alpha", "beta"})
		parentRow := waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
			return row.Status == runStatusFailed
		})
		if !strings.Contains(parentRow.ErrorText, "child id allocation failed") {
			t.Fatalf("parent error = %q, want child start failure", parentRow.ErrorText)
		}
		if _, err := env.globalDB.GetRun(
			context.Background(),
			"child-alpha",
		); !errors.Is(
			err,
			globaldb.ErrRunNotFound,
		) {
			t.Fatalf("GetRun(child-alpha) error = %v, want ErrRunNotFound", err)
		}
		if _, err := env.globalDB.GetRun(context.Background(), "child-beta"); !errors.Is(err, globaldb.ErrRunNotFound) {
			t.Fatalf("GetRun(child-beta) error = %v, want ErrRunNotFound", err)
		}

		snapshot, err := env.manager.RunMultipleSnapshot(context.Background(), parent.RunID)
		if err != nil {
			t.Fatalf("RunMultipleSnapshot() error = %v", err)
		}
		wantItems := []apicore.TaskRunMultipleItem{
			{Slug: "alpha", Status: taskMultiItemStatusFailed, ErrorText: "child id allocation failed"},
			{Slug: "beta", Status: taskMultiItemStatusCanceled, ErrorText: "child id allocation failed"},
		}
		assertTaskMultiItems(t, snapshot.Items, wantItems)
		requireRunEvent(t, parent.RunID, eventspkg.EventKindTaskRunMultipleChildFailed)
		requireRunEvent(t, parent.RunID, eventspkg.EventKindTaskRunMultipleItemCanceled)
		requireRunEvent(t, parent.RunID, eventspkg.EventKindTaskRunMultipleQueueCanceled)
	})
}

func TestRunManagerTaskRunMultipleCancellationCancelsActiveAndQueuedChildren(t *testing.T) {
	t.Parallel()

	t.Run("Should cancel active and queued children", func(t *testing.T) {
		started := make(chan string, 1)
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			buildRunID: taskMultiRunIDBuilder("task-multi-cancel"),
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
				started <- cfg.Name + ":" + cfg.RunID
				<-ctx.Done()
				return ctx.Err()
			},
		})
		writeTaskMultiWorkflow(t, env, "alpha", "pending")
		writeTaskMultiWorkflow(t, env, "beta", "pending")

		parent := startTaskMultiRun(t, env, "task-multi-cancel", []string{"alpha", "beta"})
		waitForString(t, started, "alpha:child-alpha")
		if err := env.manager.Cancel(context.Background(), parent.RunID); err != nil {
			t.Fatalf("Cancel(parent) error = %v", err)
		}

		waitForRun(t, env.globalDB, "child-alpha", func(row globaldb.Run) bool {
			return row.Status == runStatusCancelled
		})
		parentRow := waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
			return row.Status == runStatusCancelled
		})
		if parentRow.EndedAt == nil {
			t.Fatal("parent EndedAt = nil, want terminal timestamp")
		}
		if _, err := env.globalDB.GetRun(context.Background(), "child-beta"); !errors.Is(err, globaldb.ErrRunNotFound) {
			t.Fatalf("GetRun(child-beta) error = %v, want ErrRunNotFound", err)
		}

		snapshot, err := env.manager.RunMultipleSnapshot(context.Background(), parent.RunID)
		if err != nil {
			t.Fatalf("RunMultipleSnapshot() error = %v", err)
		}
		wantItems := []apicore.TaskRunMultipleItem{
			{Slug: "alpha", Status: taskMultiItemStatusCanceled, RunID: "child-alpha"},
			{Slug: "beta", Status: taskMultiItemStatusCanceled},
		}
		assertTaskMultiItems(t, snapshot.Items, wantItems)
		requireRunEvent(t, parent.RunID, eventspkg.EventKindTaskRunMultipleItemCanceled)
		requireRunEvent(t, parent.RunID, eventspkg.EventKindTaskRunMultipleQueueCanceled)
	})
}

func TestRunManagerTaskRunMultiplePreflightRejectsInvalidInputBeforeParentRun(t *testing.T) {
	t.Parallel()

	t.Run("Should reject unsupported mode", func(t *testing.T) {
		t.Parallel()

		env := newRunManagerTestEnv(t, runManagerTestDeps{buildRunID: taskMultiRunIDBuilder("task-multi-invalid-mode")})
		writeTaskMultiWorkflow(t, env, "alpha", "pending")

		_, err := env.manager.StartTaskRunMultiple(
			context.Background(),
			env.workspaceRoot,
			apicore.TaskRunMultipleRequest{
				Workspace:        env.workspaceRoot,
				Slugs:            []string{"alpha"},
				Mode:             "unsupported",
				PresentationMode: defaultPresentationMode,
				RuntimeOverrides: rawJSON(t, `{"run_id":"task-multi-invalid-mode"}`),
			},
		)
		assertProblemStatus(t, err, http.StatusUnprocessableEntity)
		if _, err := env.globalDB.GetRun(
			context.Background(),
			"task-multi-invalid-mode",
		); !errors.Is(
			err,
			globaldb.ErrRunNotFound,
		) {
			t.Fatalf("GetRun(task-multi-invalid-mode) error = %v, want ErrRunNotFound", err)
		}
	})

	t.Run("Should reject duplicate slugs", func(t *testing.T) {
		t.Parallel()

		env := newRunManagerTestEnv(t, runManagerTestDeps{buildRunID: taskMultiRunIDBuilder("task-multi-duplicate")})
		writeTaskMultiWorkflow(t, env, "alpha", "pending")

		_, err := env.manager.StartTaskRunMultiple(
			context.Background(),
			env.workspaceRoot,
			apicore.TaskRunMultipleRequest{
				Workspace:        env.workspaceRoot,
				Slugs:            []string{"alpha", "alpha"},
				PresentationMode: defaultPresentationMode,
				RuntimeOverrides: rawJSON(t, `{"run_id":"task-multi-duplicate"}`),
			},
		)
		assertProblemStatus(t, err, http.StatusUnprocessableEntity)
		if _, err := env.globalDB.GetRun(
			context.Background(),
			"task-multi-duplicate",
		); !errors.Is(
			err,
			globaldb.ErrRunNotFound,
		) {
			t.Fatalf("GetRun(task-multi-duplicate) error = %v, want ErrRunNotFound", err)
		}
	})

	t.Run("Should reject completed workflow before creating parent", func(t *testing.T) {
		t.Parallel()

		env := newRunManagerTestEnv(t, runManagerTestDeps{buildRunID: taskMultiRunIDBuilder("task-multi-completed")})
		writeTaskMultiWorkflow(t, env, "alpha", "pending")
		writeTaskMultiWorkflow(t, env, "beta", "completed")

		_, err := env.manager.StartTaskRunMultiple(
			context.Background(),
			env.workspaceRoot,
			apicore.TaskRunMultipleRequest{
				Workspace:        env.workspaceRoot,
				Slugs:            []string{"alpha", "beta"},
				PresentationMode: defaultPresentationMode,
				RuntimeOverrides: rawJSON(t, `{"run_id":"task-multi-completed"}`),
			},
		)
		assertProblemStatus(t, err, http.StatusConflict)
		if _, err := env.globalDB.GetRun(
			context.Background(),
			"task-multi-completed",
		); !errors.Is(
			err,
			globaldb.ErrRunNotFound,
		) {
			t.Fatalf("GetRun(task-multi-completed) error = %v, want ErrRunNotFound", err)
		}
	})
}

func TestResolveTaskMultiMode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "Should default empty mode to enqueued", want: "enqueued"},
		{name: "Should preserve enqueued mode", raw: " enqueued ", want: "enqueued"},
		{name: "Should accept parallel mode", raw: " parallel ", want: "parallel"},
		{name: "Should reject unsupported mode", raw: "unsupported", wantErr: true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolveTaskMultiMode(tc.raw)
			if tc.wantErr {
				assertProblemStatus(t, err, http.StatusUnprocessableEntity)
				return
			}
			if err != nil {
				t.Fatalf("resolveTaskMultiMode(%q) error = %v", tc.raw, err)
			}
			if got != tc.want {
				t.Fatalf("resolveTaskMultiMode(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestRemapTaskMultiChildRuntime(t *testing.T) {
	t.Parallel()

	newBase := func() *model.RuntimeConfig {
		return &model.RuntimeConfig{
			WorkspaceRoot: "/original/workspace",
			Name:          "alpha",
			TasksDir:      model.TaskDirectoryForWorkspace("/original/workspace", "alpha"),
			Model:         "custom-model",
			AutoCommit:    true,
			Concurrent:    4,
			AddDirs:       []string{"docs", "scripts"},
			ParentRunID:   "stale-parent",
		}
	}

	t.Run("Should set WorkspaceRoot to the worktree path", func(t *testing.T) {
		t.Parallel()
		got, err := remapTaskMultiChildRuntime(newBase(), "/wt/01-alpha", "alpha", "parent-1")
		if err != nil {
			t.Fatalf("remapTaskMultiChildRuntime() error = %v", err)
		}
		if got.WorkspaceRoot != "/wt/01-alpha" {
			t.Fatalf("WorkspaceRoot = %q, want /wt/01-alpha", got.WorkspaceRoot)
		}
	})

	t.Run("Should set TasksDir to the slug task directory inside the worktree", func(t *testing.T) {
		t.Parallel()
		got, err := remapTaskMultiChildRuntime(newBase(), "/wt/01-alpha", "alpha", "parent-1")
		if err != nil {
			t.Fatalf("remapTaskMultiChildRuntime() error = %v", err)
		}
		want := model.TaskDirectoryForWorkspace("/wt/01-alpha", "alpha")
		if got.TasksDir != want {
			t.Fatalf("TasksDir = %q, want %q", got.TasksDir, want)
		}
	})

	t.Run("Should set ParentRunID and preserve unrelated runtime overrides", func(t *testing.T) {
		t.Parallel()
		base := newBase()
		got, err := remapTaskMultiChildRuntime(base, "/wt/01-alpha", "alpha", "parent-1")
		if err != nil {
			t.Fatalf("remapTaskMultiChildRuntime() error = %v", err)
		}
		if got.ParentRunID != "parent-1" {
			t.Fatalf("ParentRunID = %q, want parent-1", got.ParentRunID)
		}
		if got.Model != "custom-model" || !got.AutoCommit || got.Concurrent != 4 {
			t.Fatalf("unrelated overrides not preserved: %#v", got)
		}
		if len(got.AddDirs) != 2 || got.AddDirs[0] != "docs" || got.AddDirs[1] != "scripts" {
			t.Fatalf("AddDirs = %#v, want [docs scripts]", got.AddDirs)
		}
		if base.WorkspaceRoot != "/original/workspace" || base.ParentRunID != "stale-parent" {
			t.Fatalf("base config mutated by remap: %#v", base)
		}
	})

	t.Run("Should reject invalid inputs", func(t *testing.T) {
		t.Parallel()
		if _, err := remapTaskMultiChildRuntime(nil, "/wt", "alpha", "p"); err == nil {
			t.Fatal("nil base error = nil, want error")
		}
		if _, err := remapTaskMultiChildRuntime(newBase(), "  ", "alpha", "p"); err == nil {
			t.Fatal("empty worktree path error = nil, want error")
		}
		if _, err := remapTaskMultiChildRuntime(newBase(), "/wt", "  ", "p"); err == nil {
			t.Fatal("empty slug error = nil, want error")
		}
	})
}

func TestRequireTaskMultiWorktreeTaskDir(t *testing.T) {
	t.Parallel()

	t.Run("Should return the slug task directory when present", func(t *testing.T) {
		t.Parallel()
		worktree := t.TempDir()
		want := model.TaskDirectoryForWorkspace(worktree, "alpha")
		if err := os.MkdirAll(want, 0o755); err != nil {
			t.Fatalf("mkdir worktree task dir: %v", err)
		}
		got, err := requireTaskMultiWorktreeTaskDir(worktree, "alpha")
		if err != nil {
			t.Fatalf("requireTaskMultiWorktreeTaskDir() error = %v", err)
		}
		if got != want {
			t.Fatalf("task dir = %q, want %q", got, want)
		}
	})

	t.Run("Should return a slug-specific error when the task directory is missing", func(t *testing.T) {
		t.Parallel()
		worktree := t.TempDir()
		_, err := requireTaskMultiWorktreeTaskDir(worktree, "beta")
		if err == nil {
			t.Fatal("requireTaskMultiWorktreeTaskDir() error = nil, want missing task dir error")
		}
		if !strings.Contains(err.Error(), "beta") || !strings.Contains(err.Error(), "missing task directory") {
			t.Fatalf("error = %v, want slug-specific missing task directory error", err)
		}
	})
}

func TestRunManagerRunTaskMultiParallelQueueResolvesBaseBeforeChildren(t *testing.T) {
	t.Parallel()

	t.Run("Should fail without allocating worktrees when the parent base cannot resolve", func(t *testing.T) {
		t.Parallel()

		allocatorCalls := 0
		allocator := &taskMultiWorktreeAllocator{
			run: func(_ context.Context, _ string, args ...string) (string, error) {
				allocatorCalls++
				if strings.Join(args, " ") == "rev-parse --abbrev-ref HEAD" {
					return taskMultiWorktreeHeadRef, nil
				}
				t.Fatalf("unexpected git command after detached base: %v", args)
				return "", nil
			},
		}
		m := &RunManager{worktreeAllocator: allocator}
		prepared := &preparedTaskMulti{
			mode:      "parallel",
			workspace: globaldb.Workspace{RootDir: "/repo"},
			items:     []preparedTaskMultiItem{{slug: "alpha"}, {slug: "beta"}},
		}
		active := &activeRun{runID: "task-multi-parallel-base", taskMulti: prepared}

		err := m.runTaskMultiParallelQueue(active, prepared, len(prepared.items))
		if err == nil || !strings.Contains(err.Error(), "detached HEAD") {
			t.Fatalf("runTaskMultiParallelQueue() error = %v, want detached HEAD failure", err)
		}
		if allocatorCalls != 1 {
			t.Fatalf("allocator git calls = %d, want exactly 1 base read before failing", allocatorCalls)
		}
	})
}

func TestRunManagerTaskRunMultipleParallelRegistersChildrenUnderWorktreeWorkspaces(t *testing.T) {
	t.Parallel()

	t.Run("Should register and remap parallel children onto isolated worktree workspaces", func(t *testing.T) {
		requireGitForTaskMulti(t)

		var (
			mu       sync.Mutex
			captured = map[string]*model.RuntimeConfig{}
		)
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			buildRunID: taskMultiRunIDBuilder("task-multi-parallel-register"),
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(_ context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
				mu.Lock()
				captured[cfg.Name] = cfg.Clone()
				mu.Unlock()
				return nil
			},
		})
		writeTaskMultiWorkflow(t, env, "alpha", "pending")
		writeTaskMultiWorkflow(t, env, "beta", "pending")
		commitTaskMultiGitWorkspace(t, env.workspaceRoot)

		parent := startTaskMultiParallelRun(t, env, "task-multi-parallel-register", []string{"alpha", "beta"})
		parentRow := waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
			return row.Status == runStatusCompleted
		})

		originalWorkspace, err := env.globalDB.Get(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("Get(original workspace) error = %v", err)
		}
		if parentRow.WorkspaceID != originalWorkspace.ID {
			t.Fatalf("parent WorkspaceID = %q, want original workspace %q", parentRow.WorkspaceID, originalWorkspace.ID)
		}

		for _, slug := range []string{"alpha", "beta"} {
			childRunID := "child-" + slug
			childRow := requireTaskMultiChildRow(t, env, childRunID, parent.RunID, runStatusCompleted)
			if childRow.WorkspaceID == originalWorkspace.ID {
				t.Fatalf(
					"%s WorkspaceID = original %q, want a distinct worktree workspace",
					childRunID,
					childRow.WorkspaceID,
				)
			}
			childWorkspace, err := env.globalDB.Get(context.Background(), childRow.WorkspaceID)
			if err != nil {
				t.Fatalf("Get(child workspace %s) error = %v", childRunID, err)
			}
			if !strings.Contains(childWorkspace.RootDir, filepath.Join("state", "worktrees")) {
				t.Fatalf(
					"%s workspace root = %q, want under the worktrees state dir",
					childRunID,
					childWorkspace.RootDir,
				)
			}
			mu.Lock()
			cfg := captured[slug]
			mu.Unlock()
			if cfg == nil {
				t.Fatalf("no captured runtime config for %s", slug)
			}
			if cfg.WorkspaceRoot != childWorkspace.RootDir {
				t.Fatalf("%s runtime WorkspaceRoot = %q, want worktree workspace root %q",
					slug, cfg.WorkspaceRoot, childWorkspace.RootDir)
			}
			if want := model.TaskDirectoryForWorkspace(childWorkspace.RootDir, slug); cfg.TasksDir != want {
				t.Fatalf("%s runtime TasksDir = %q, want %q", slug, cfg.TasksDir, want)
			}
			if cfg.ParentRunID != parent.RunID {
				t.Fatalf("%s runtime ParentRunID = %q, want %q", slug, cfg.ParentRunID, parent.RunID)
			}
		}

		wantCommit := runGitOutput(t, env.workspaceRoot, "rev-parse", "HEAD")
		snapshot, err := env.manager.RunMultipleSnapshot(context.Background(), parent.RunID)
		if err != nil {
			t.Fatalf("RunMultipleSnapshot() error = %v", err)
		}
		if len(snapshot.Items) != 2 {
			t.Fatalf("snapshot items = %d, want 2", len(snapshot.Items))
		}
		for idx := range snapshot.Items {
			item := snapshot.Items[idx]
			if item.Status != taskMultiItemStatusCompleted {
				t.Fatalf("item %s status = %q, want completed", item.Slug, item.Status)
			}
			if !strings.Contains(item.WorktreePath, filepath.Join("state", "worktrees")) {
				t.Fatalf("item %s WorktreePath = %q, want under the worktrees state dir", item.Slug, item.WorktreePath)
			}
			if item.BaseBranch != "main" {
				t.Fatalf("item %s BaseBranch = %q, want main", item.Slug, item.BaseBranch)
			}
			if item.BaseCommit != wantCommit {
				t.Fatalf("item %s BaseCommit = %q, want %q", item.Slug, item.BaseCommit, wantCommit)
			}
			if item.WorktreeStatus != taskMultiWorktreeStatusPreserved {
				t.Fatalf("item %s WorktreeStatus = %q, want %q",
					item.Slug, item.WorktreeStatus, taskMultiWorktreeStatusPreserved)
			}
		}

		assertTaskMultiWorktreeMetadataBeforeChildStart(t, parent.RunID, "alpha")
	})
}

func TestRunManagerTaskRunMultipleIncludeCompletedStartsCompletedWorkflow(t *testing.T) {
	t.Parallel()

	t.Run("Should start completed workflow when include_completed is true", func(t *testing.T) {
		started := make(chan string, 2)
		seenIncludeCompleted := make(chan bool, 2)
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			buildRunID: taskMultiRunIDBuilder("task-multi-include-completed"),
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(_ context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
				started <- cfg.Name + ":" + cfg.RunID
				seenIncludeCompleted <- cfg.IncludeCompleted
				return nil
			},
		})
		writeTaskMultiWorkflow(t, env, "alpha", "pending")
		writeTaskMultiWorkflow(t, env, "beta", "completed")

		parent, err := env.manager.StartTaskRunMultiple(
			context.Background(),
			env.workspaceRoot,
			apicore.TaskRunMultipleRequest{
				Workspace:        env.workspaceRoot,
				Slugs:            []string{"alpha", "beta"},
				PresentationMode: defaultPresentationMode,
				RuntimeOverrides: rawJSON(t, `{"run_id":"task-multi-include-completed","include_completed":true}`),
			},
		)
		if err != nil {
			t.Fatalf("StartTaskRunMultiple(include completed) error = %v", err)
		}

		waitForString(t, started, "alpha:child-alpha")
		waitForString(t, started, "beta:child-beta")
		for range 2 {
			if !waitForBool(t, seenIncludeCompleted) {
				t.Fatal("child run saw IncludeCompleted=false, want true")
			}
		}
		waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
			return row.Status == runStatusCompleted
		})
		snapshot, err := env.manager.RunMultipleSnapshot(context.Background(), parent.RunID)
		if err != nil {
			t.Fatalf("RunMultipleSnapshot() error = %v", err)
		}
		wantItems := []apicore.TaskRunMultipleItem{
			{Slug: "alpha", Status: taskMultiItemStatusCompleted, RunID: "child-alpha"},
			{Slug: "beta", Status: taskMultiItemStatusCompleted, RunID: "child-beta"},
		}
		assertTaskMultiItems(t, snapshot.Items, wantItems)
	})
}

func TestRunManagerTaskRunMultipleChildPollReturnsRunLookupErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should return child run lookup errors from poll", func(t *testing.T) {
		env := newRunManagerTestEnv(t, runManagerTestDeps{})
		if err := env.globalDB.Close(); err != nil {
			t.Fatalf("Close global DB: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		_, err := env.manager.waitForTaskMultiChild(ctx, "child-alpha")
		if err == nil {
			t.Fatal("waitForTaskMultiChild() error = nil, want run lookup error")
		}
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			t.Fatalf("waitForTaskMultiChild() error = %v, want lookup error before context cancellation", err)
		}
		if !strings.Contains(err.Error(), "load child run child-alpha") {
			t.Fatalf("waitForTaskMultiChild() error = %v, want child lookup context", err)
		}
	})
}

func TestRunManagerTaskRunMultiplePollLookupErrorCancelsActiveChild(t *testing.T) {
	t.Parallel()

	t.Run("Should cancel active child after poll lookup error", func(t *testing.T) {
		childStarted := make(chan struct{})
		childCanceled := make(chan error, 1)
		var startedOnce sync.Once
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			buildRunID: taskMultiRunIDBuilder("task-multi-poll-cancel"),
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
				if cfg.Name != "alpha" {
					return nil
				}
				startedOnce.Do(func() { close(childStarted) })
				<-ctx.Done()
				childCanceled <- ctx.Err()
				return ctx.Err()
			},
		})
		writeTaskMultiWorkflow(t, env, "alpha", "pending")
		parent := startTaskMultiRun(t, env, "task-multi-poll-cancel", []string{"alpha"})
		waitForClosed(t, childStarted, "alpha child start")
		parentActive := requireActiveRun(t, env.manager, parent.RunID)
		childActive := requireActiveRun(t, env.manager, "child-alpha")
		defer parentActive.cancel()
		defer childActive.cancel()

		if err := env.globalDB.Close(); err != nil {
			t.Fatalf("Close global DB: %v", err)
		}

		select {
		case err := <-childCanceled:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("child context error = %v, want context.Canceled", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("child was not canceled after parent %s hit child lookup error", parent.RunID)
		}
		waitForClosed(t, childActive.done, "alpha child cleanup")
		waitForClosed(t, parentActive.done, "parent multi-run cleanup")
	})
}

func TestRunManagerTaskRunMultipleSnapshotRejectsNonParentRun(t *testing.T) {
	t.Parallel()

	t.Run("Should reject snapshot for non-parent run", func(t *testing.T) {
		env := newRunManagerTestEnv(t, runManagerTestDeps{})

		run := env.startTaskRun(t, "task-run-not-multi", nil)
		waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
			return isTerminalRunStatus(row.Status)
		})

		_, err := env.manager.RunMultipleSnapshot(context.Background(), run.RunID)
		assertProblemStatus(t, err, http.StatusUnprocessableEntity)
	})
}

func TestRunManagerTaskRunMultipleParentRuntimeStartFailureDoesNotStartChildren(t *testing.T) {
	t.Parallel()

	t.Run("Should not start children when parent runtime start fails", func(t *testing.T) {
		var childStarted atomic.Bool
		runtimeManager := &stubRuntimeManager{startErr: errors.New("runtime failed to start")}
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			buildRunID:   taskMultiRunIDBuilder("task-multi-parent-start-failure"),
			openRunScope: newTestOpenRunScope(runtimeManager),
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error {
				childStarted.Store(true)
				return nil
			},
		})
		writeTaskMultiWorkflow(t, env, "alpha", "pending")

		parent := startTaskMultiRun(t, env, "task-multi-parent-start-failure", []string{"alpha"})
		row := waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
			return row.Status == runStatusFailed
		})
		if !strings.Contains(row.ErrorText, "runtime failed to start") {
			t.Fatalf("parent error = %q, want runtime failure", row.ErrorText)
		}
		if childStarted.Load() {
			t.Fatal("child started after parent runtime start failure")
		}
		if _, err := env.globalDB.GetRun(
			context.Background(),
			"child-alpha",
		); !errors.Is(
			err,
			globaldb.ErrRunNotFound,
		) {
			t.Fatalf("GetRun(child-alpha) error = %v, want ErrRunNotFound", err)
		}
	})
}

func TestTaskMultiSnapshotBuilderReconstructsWorktreeMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should apply worktree path before child run id exists", func(t *testing.T) {
		t.Parallel()

		builder := newTaskMultiSnapshotBuilder()
		mustApplyTaskMultiEvent(t, builder, eventspkg.EventKindTaskRunMultipleStarted, kinds.TaskRunMultiplePayload{
			Mode:  "parallel",
			Slugs: []string{"alpha", "beta"},
			Total: 2,
		})
		mustApplyTaskMultiEvent(t, builder, eventspkg.EventKindTaskRunMultipleItemQueued, kinds.TaskRunMultiplePayload{
			Slug:           "alpha",
			Status:         taskMultiItemStatusQueued,
			WorktreePath:   "/wt/01-alpha",
			BaseBranch:     "main",
			BaseCommit:     "abc123",
			WorktreeStatus: "preserved",
		})

		items := builder.snapshotItems()
		want := apicore.TaskRunMultipleItem{
			Slug:           "alpha",
			Status:         taskMultiItemStatusQueued,
			WorktreePath:   "/wt/01-alpha",
			BaseBranch:     "main",
			BaseCommit:     "abc123",
			WorktreeStatus: "preserved",
		}
		if items[0] != want {
			t.Fatalf("alpha item = %#v, want %#v", items[0], want)
		}
		if items[0].RunID != "" {
			t.Fatalf("alpha RunID = %q, want empty before child start", items[0].RunID)
		}
	})

	t.Run("Should preserve child run id and error text alongside metadata", func(t *testing.T) {
		t.Parallel()

		builder := newTaskMultiSnapshotBuilder()
		mustApplyTaskMultiEvent(t, builder, eventspkg.EventKindTaskRunMultipleItemQueued, kinds.TaskRunMultiplePayload{
			Slug:           "alpha",
			Status:         taskMultiItemStatusQueued,
			WorktreePath:   "/wt/01-alpha",
			BaseBranch:     "main",
			BaseCommit:     "abc123",
			WorktreeStatus: "preserved",
		})
		mustApplyTaskMultiEvent(
			t,
			builder,
			eventspkg.EventKindTaskRunMultipleChildStarted,
			kinds.TaskRunMultiplePayload{
				Slug:         "alpha",
				Status:       taskMultiItemStatusRunning,
				ChildRunID:   "child-alpha",
				WorktreePath: "/wt/01-alpha",
			},
		)
		mustApplyTaskMultiEvent(t, builder, eventspkg.EventKindTaskRunMultipleChildFailed, kinds.TaskRunMultiplePayload{
			Slug:       "alpha",
			Status:     taskMultiItemStatusFailed,
			ChildRunID: "child-alpha",
			Error:      "alpha failed",
		})

		items := builder.snapshotItems()
		want := apicore.TaskRunMultipleItem{
			Slug:           "alpha",
			Status:         taskMultiItemStatusFailed,
			RunID:          "child-alpha",
			ErrorText:      "alpha failed",
			WorktreePath:   "/wt/01-alpha",
			BaseBranch:     "main",
			BaseCommit:     "abc123",
			WorktreeStatus: "preserved",
		}
		if items[0] != want {
			t.Fatalf("alpha item = %#v, want %#v", items[0], want)
		}
	})

	t.Run("Should keep requested item order after metadata events", func(t *testing.T) {
		t.Parallel()

		builder := newTaskMultiSnapshotBuilder()
		mustApplyTaskMultiEvent(t, builder, eventspkg.EventKindTaskRunMultipleStarted, kinds.TaskRunMultiplePayload{
			Mode:  "parallel",
			Slugs: []string{"alpha", "beta", "gamma"},
			Total: 3,
		})
		for _, slug := range []string{"gamma", "alpha", "beta"} {
			mustApplyTaskMultiEvent(
				t,
				builder,
				eventspkg.EventKindTaskRunMultipleChildStarted,
				kinds.TaskRunMultiplePayload{
					Slug:         slug,
					Status:       taskMultiItemStatusRunning,
					ChildRunID:   "child-" + slug,
					WorktreePath: "/wt/" + slug,
				},
			)
		}

		items := builder.snapshotItems()
		wantOrder := []string{"alpha", "beta", "gamma"}
		if len(items) != len(wantOrder) {
			t.Fatalf("item count = %d, want %d", len(items), len(wantOrder))
		}
		for idx, slug := range wantOrder {
			if items[idx].Slug != slug {
				t.Fatalf("item[%d].Slug = %q, want %q", idx, items[idx].Slug, slug)
			}
			if items[idx].WorktreePath != "/wt/"+slug {
				t.Fatalf("item[%d].WorktreePath = %q, want %q", idx, items[idx].WorktreePath, "/wt/"+slug)
			}
		}
	})

	t.Run("Should leave worktree metadata empty for events without it", func(t *testing.T) {
		t.Parallel()

		builder := newTaskMultiSnapshotBuilder()
		mustApplyTaskMultiEvent(t, builder, eventspkg.EventKindTaskRunMultipleStarted, kinds.TaskRunMultiplePayload{
			Mode:  "enqueued",
			Slugs: []string{"alpha"},
			Total: 1,
		})
		mustApplyTaskMultiEvent(
			t,
			builder,
			eventspkg.EventKindTaskRunMultipleChildCompleted,
			kinds.TaskRunMultiplePayload{
				Slug:       "alpha",
				Status:     taskMultiItemStatusCompleted,
				ChildRunID: "child-alpha",
			},
		)

		items := builder.snapshotItems()
		want := apicore.TaskRunMultipleItem{
			Slug:   "alpha",
			Status: taskMultiItemStatusCompleted,
			RunID:  "child-alpha",
		}
		if items[0] != want {
			t.Fatalf("alpha item = %#v, want %#v (no worktree metadata)", items[0], want)
		}
	})
}

func TestRunManagerTaskRunMultipleSnapshotReconstructsWorktreeMetadataFromEvents(t *testing.T) {
	t.Parallel()

	t.Run("Should reconstruct worktree-aware snapshot from parent events", func(t *testing.T) {
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			buildRunID: taskMultiRunIDBuilder("task-multi-worktree"),
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error {
				return nil
			},
		})
		writeTaskMultiWorkflow(t, env, "alpha", "pending")
		writeTaskMultiWorkflow(t, env, "beta", "pending")

		parent := startTaskMultiRun(t, env, "task-multi-worktree", []string{"alpha", "beta"})
		waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
			return row.Status == runStatusCompleted
		})

		before, err := env.manager.RunMultipleSnapshot(context.Background(), parent.RunID)
		if err != nil {
			t.Fatalf("RunMultipleSnapshot() before metadata error = %v", err)
		}
		assertTaskMultiItems(t, before.Items, []apicore.TaskRunMultipleItem{
			{Slug: "alpha", Status: taskMultiItemStatusCompleted, RunID: "child-alpha"},
			{Slug: "beta", Status: taskMultiItemStatusCompleted, RunID: "child-beta"},
		})
		for idx := range before.Items {
			if before.Items[idx].WorktreePath != "" ||
				before.Items[idx].BaseBranch != "" ||
				before.Items[idx].BaseCommit != "" ||
				before.Items[idx].WorktreeStatus != "" {
				t.Fatalf("pre-metadata item %#v, want empty worktree fields", before.Items[idx])
			}
		}

		appendTaskMultiWorktreeEvent(t, parent.RunID, "alpha", "child-alpha", "/wt/01-alpha", "abc123")
		appendTaskMultiWorktreeEvent(t, parent.RunID, "beta", "child-beta", "/wt/02-beta", "def456")

		after, err := env.manager.RunMultipleSnapshot(context.Background(), parent.RunID)
		if err != nil {
			t.Fatalf("RunMultipleSnapshot() after metadata error = %v", err)
		}
		want := []apicore.TaskRunMultipleItem{
			{
				Slug:           "alpha",
				Status:         taskMultiItemStatusCompleted,
				RunID:          "child-alpha",
				WorktreePath:   "/wt/01-alpha",
				BaseBranch:     "main",
				BaseCommit:     "abc123",
				WorktreeStatus: "preserved",
			},
			{
				Slug:           "beta",
				Status:         taskMultiItemStatusCompleted,
				RunID:          "child-beta",
				WorktreePath:   "/wt/02-beta",
				BaseBranch:     "main",
				BaseCommit:     "def456",
				WorktreeStatus: "preserved",
			},
		}
		if len(after.Items) != len(want) {
			t.Fatalf("after item count = %d, want %d", len(after.Items), len(want))
		}
		for idx := range want {
			if after.Items[idx] != want[idx] {
				t.Fatalf("after item[%d] = %#v, want %#v", idx, after.Items[idx], want[idx])
			}
		}
	})
}

func mustApplyTaskMultiEvent(
	t *testing.T,
	builder *taskMultiSnapshotBuilder,
	kind eventspkg.EventKind,
	payload kinds.TaskRunMultiplePayload,
) {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal %s payload: %v", kind, err)
	}
	if err := builder.applyEvent(eventspkg.Event{Kind: kind, Payload: raw}); err != nil {
		t.Fatalf("applyEvent(%s) error = %v", kind, err)
	}
}

func appendTaskMultiWorktreeEvent(
	t *testing.T,
	parentRunID string,
	slug string,
	childRunID string,
	worktreePath string,
	baseCommit string,
) {
	t.Helper()
	runDB, err := openRunDBForRunID(context.Background(), parentRunID)
	if err != nil {
		t.Fatalf("openRunDBForRunID(%q) error = %v", parentRunID, err)
	}
	defer func() {
		_ = runDB.Close()
	}()
	if _, err := runDB.AppendSyntheticEvent(
		context.Background(),
		eventspkg.EventKindTaskRunMultipleChildCompleted,
		kinds.TaskRunMultiplePayload{
			RunID:          parentRunID,
			Slug:           slug,
			Status:         taskMultiItemStatusCompleted,
			ChildRunID:     childRunID,
			WorktreePath:   worktreePath,
			BaseBranch:     "main",
			BaseCommit:     baseCommit,
			WorktreeStatus: "preserved",
		},
	); err != nil {
		t.Fatalf("AppendSyntheticEvent(%q) error = %v", slug, err)
	}
}

func writeTaskMultiWorkflow(t *testing.T, env *runManagerTestEnv, slug string, status string) {
	t.Helper()
	env.writeWorkflowFile(t, slug, "task_01.md", daemonTaskBody(status, "Task "+slug))
}

func startTaskMultiRun(t *testing.T, env *runManagerTestEnv, runID string, slugs []string) apicore.Run {
	t.Helper()
	run, err := env.manager.StartTaskRunMultiple(
		context.Background(),
		env.workspaceRoot,
		apicore.TaskRunMultipleRequest{
			Workspace:        env.workspaceRoot,
			Slugs:            slugs,
			Mode:             "enqueued",
			PresentationMode: defaultPresentationMode,
			RuntimeOverrides: rawJSON(t, fmt.Sprintf(`{"run_id":%q}`, runID)),
		},
	)
	if err != nil {
		t.Fatalf("StartTaskRunMultiple(%v) error = %v", slugs, err)
	}
	return run
}

func taskMultiRunIDBuilder(parentRunID string) func(*model.RuntimeConfig) (string, error) {
	return func(cfg *model.RuntimeConfig) (string, error) {
		if cfg == nil {
			return "", errors.New("runtime config is required")
		}
		if runID := strings.TrimSpace(cfg.RunID); runID != "" {
			return runID, nil
		}
		if strings.TrimSpace(cfg.ParentRunID) == parentRunID {
			return "child-" + strings.TrimSpace(cfg.Name), nil
		}
		return "generated-" + strings.TrimSpace(cfg.Name), nil
	}
}

func assertNoTaskMultiStart(t *testing.T, ch <-chan string, label string) {
	t.Helper()
	select {
	case got := <-ch:
		t.Fatalf("unexpected child start while checking %s: %s", label, got)
	case <-time.After(100 * time.Millisecond):
	}
}

func requireTaskMultiChildRow(
	t *testing.T,
	env *runManagerTestEnv,
	runID string,
	parentRunID string,
	status string,
) globaldb.Run {
	t.Helper()
	row := waitForRun(t, env.globalDB, runID, func(row globaldb.Run) bool {
		return row.Status == status
	})
	if row.ParentRunID != parentRunID {
		t.Fatalf("%s ParentRunID = %q, want %q", runID, row.ParentRunID, parentRunID)
	}
	return row
}

func hasRunMode(runs []apicore.Run, mode string) bool {
	return slices.ContainsFunc(runs, func(run apicore.Run) bool {
		return run.Mode == mode
	})
}

func assertTaskMultiItems(t *testing.T, got []apicore.TaskRunMultipleItem, want []apicore.TaskRunMultipleItem) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("items = %#v, want %#v", got, want)
	}
	for idx := range want {
		if got[idx].Slug != want[idx].Slug ||
			got[idx].Status != want[idx].Status ||
			got[idx].RunID != want[idx].RunID ||
			!strings.Contains(got[idx].ErrorText, want[idx].ErrorText) {
			t.Fatalf("item[%d] = %#v, want %#v", idx, got[idx], want[idx])
		}
	}
}

func requireGitForTaskMulti(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
}

// commitTaskMultiGitWorkspace turns a prepared workspace into a single-commit git
// repository on branch main so parallel multi-run can resolve a named base branch
// and HEAD commit and create detached worktrees that contain the task files.
func commitTaskMultiGitWorkspace(t *testing.T, root string) {
	t.Helper()
	runGitOutput(t, root, "init", "-q", "-b", "main")
	runGitOutput(t, root, "config", "user.email", "task-multi@example.com")
	runGitOutput(t, root, "config", "user.name", "Task Multi Tester")
	runGitOutput(t, root, "config", "commit.gpgsign", "false")
	runGitOutput(t, root, "add", "-A")
	runGitOutput(t, root, "commit", "-q", "-m", "seed parallel multi-run workspace")
}

func startTaskMultiParallelRun(t *testing.T, env *runManagerTestEnv, runID string, slugs []string) apicore.Run {
	t.Helper()
	run, err := env.manager.StartTaskRunMultiple(
		context.Background(),
		env.workspaceRoot,
		apicore.TaskRunMultipleRequest{
			Workspace:        env.workspaceRoot,
			Slugs:            slugs,
			Mode:             "parallel",
			PresentationMode: defaultPresentationMode,
			RuntimeOverrides: rawJSON(t, fmt.Sprintf(`{"run_id":%q}`, runID)),
		},
	)
	if err != nil {
		t.Fatalf("StartTaskRunMultiple(parallel %v) error = %v", slugs, err)
	}
	return run
}

// assertTaskMultiWorktreeMetadataBeforeChildStart verifies that a worktree-aware
// item-queued event (carrying a worktree path) is recorded before the
// child-started event for the same slug, matching the "emit metadata before child
// launch" recovery invariant.
func assertTaskMultiWorktreeMetadataBeforeChildStart(t *testing.T, parentRunID string, slug string) {
	t.Helper()
	events := allRunEvents(t, parentRunID)
	metadataIdx := -1
	startedIdx := -1
	for idx := range events {
		event := events[idx]
		switch event.Kind {
		case eventspkg.EventKindTaskRunMultipleItemQueued:
			payload, err := decodeTaskMultiPayload(event)
			if err != nil {
				t.Fatalf("decode item-queued event %d: %v", idx, err)
			}
			if payload.Slug == slug && metadataIdx == -1 && strings.TrimSpace(payload.WorktreePath) != "" {
				metadataIdx = idx
			}
		case eventspkg.EventKindTaskRunMultipleChildStarted:
			payload, err := decodeTaskMultiPayload(event)
			if err != nil {
				t.Fatalf("decode child-started event %d: %v", idx, err)
			}
			if payload.Slug == slug && startedIdx == -1 {
				startedIdx = idx
			}
		}
	}
	if metadataIdx == -1 {
		t.Fatalf("no worktree metadata item-queued event for %s", slug)
	}
	if startedIdx == -1 {
		t.Fatalf("no child-started event for %s", slug)
	}
	if metadataIdx >= startedIdx {
		t.Fatalf("worktree metadata event index %d for %s must precede child-started index %d",
			metadataIdx, slug, startedIdx)
	}
}

func startTaskMultiParallelRunWithLimit(
	t *testing.T,
	env *runManagerTestEnv,
	runID string,
	slugs []string,
	limit int,
) apicore.Run {
	t.Helper()
	run, err := env.manager.StartTaskRunMultiple(
		context.Background(),
		env.workspaceRoot,
		apicore.TaskRunMultipleRequest{
			Workspace:        env.workspaceRoot,
			Slugs:            slugs,
			Mode:             "parallel",
			ParallelLimit:    limit,
			PresentationMode: defaultPresentationMode,
			RuntimeOverrides: rawJSON(t, fmt.Sprintf(`{"run_id":%q}`, runID)),
		},
	)
	if err != nil {
		t.Fatalf("StartTaskRunMultiple(parallel limit=%d %v) error = %v", limit, slugs, err)
	}
	return run
}

func TestResolveTaskMultiParallelLimit(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		request    int
		configured int
		want       int
	}{
		{name: "Should prefer a positive request limit over config", request: 3, configured: 2, want: 3},
		{name: "Should use the configured limit when the request is zero", request: 0, configured: 5, want: 5},
		{
			name:       "Should default when both request and config are unset",
			request:    0,
			configured: 0,
			want:       workspacecfg.DefaultRunMultipleParallelLimit,
		},
		{name: "Should use the configured limit when the request is negative", request: -1, configured: 4, want: 4},
		{
			name:       "Should clamp a non-positive configured limit to the default",
			request:    0,
			configured: -2,
			want:       workspacecfg.DefaultRunMultipleParallelLimit,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := resolveTaskMultiParallelLimit(tc.request, tc.configured); got != tc.want {
				t.Fatalf("resolveTaskMultiParallelLimit(%d, %d) = %d, want %d", tc.request, tc.configured, got, tc.want)
			}
		})
	}
}

func TestAggregateTaskMultiParallelResult(t *testing.T) {
	t.Parallel()
	items := []preparedTaskMultiItem{{slug: "alpha"}, {slug: "beta"}, {slug: "gamma"}}
	t.Run("Should return nil when every child succeeds", func(t *testing.T) {
		t.Parallel()
		if err := aggregateTaskMultiParallelResult(items, []error{nil, nil, nil}); err != nil {
			t.Fatalf("aggregateTaskMultiParallelResult() = %v, want nil", err)
		}
	})
	t.Run("Should name the single failed slug and count", func(t *testing.T) {
		t.Parallel()
		err := aggregateTaskMultiParallelResult(items, []error{nil, errors.New("boom"), nil})
		if err == nil {
			t.Fatal("aggregateTaskMultiParallelResult() = nil, want error")
		}
		if !strings.Contains(err.Error(), "beta") {
			t.Fatalf("error %q must name the failed slug beta", err)
		}
		if !strings.Contains(err.Error(), "1 of 3") {
			t.Fatalf("error %q must report 1 of 3 children failed", err)
		}
		if !strings.Contains(err.Error(), "boom") {
			t.Fatalf("error %q must wrap the underlying child error", err)
		}
	})
	t.Run("Should name every failed slug in queue order", func(t *testing.T) {
		t.Parallel()
		err := aggregateTaskMultiParallelResult(items, []error{errors.New("a"), nil, errors.New("c")})
		if err == nil {
			t.Fatal("aggregateTaskMultiParallelResult() = nil, want error")
		}
		alphaIdx := strings.Index(err.Error(), "alpha")
		gammaIdx := strings.Index(err.Error(), "gamma")
		if alphaIdx == -1 || gammaIdx == -1 || alphaIdx > gammaIdx {
			t.Fatalf("error %q must name alpha before gamma", err)
		}
		if !strings.Contains(err.Error(), "2 of 3") {
			t.Fatalf("error %q must report 2 of 3 children failed", err)
		}
	})
}

func TestRunManagerTaskRunMultipleParallelBoundsConcurrency(t *testing.T) {
	t.Parallel()
	t.Run("Should never run more children concurrently than the resolved limit", func(t *testing.T) {
		requireGitForTaskMulti(t)
		var (
			mu          sync.Mutex
			inFlight    int
			maxInFlight int
		)
		entered := make(chan string, 3)
		release := make(chan struct{})
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			buildRunID: taskMultiRunIDBuilder("task-multi-parallel-bound"),
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
				mu.Lock()
				inFlight++
				if inFlight > maxInFlight {
					maxInFlight = inFlight
				}
				mu.Unlock()
				entered <- cfg.Name
				select {
				case <-release:
				case <-ctx.Done():
				}
				mu.Lock()
				inFlight--
				mu.Unlock()
				return nil
			},
		})
		for _, slug := range []string{"alpha", "beta", "gamma"} {
			writeTaskMultiWorkflow(t, env, slug, "pending")
		}
		commitTaskMultiGitWorkspace(t, env.workspaceRoot)

		parent := startTaskMultiParallelRunWithLimit(
			t, env, "task-multi-parallel-bound", []string{"alpha", "beta", "gamma"}, 2,
		)
		// Exactly two children may enter execution; the third must wait for a slot.
		<-entered
		<-entered
		select {
		case third := <-entered:
			close(release)
			t.Fatalf("third child %q entered execution before a slot freed; limit not enforced", third)
		default:
		}
		close(release)
		<-entered // the third child proceeds only after a slot frees
		waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
			return row.Status == runStatusCompleted
		})
		mu.Lock()
		got := maxInFlight
		mu.Unlock()
		if got != 2 {
			t.Fatalf("max concurrent children = %d, want 2", got)
		}
	})
}

func TestRunManagerTaskRunMultipleParallelFailLate(t *testing.T) {
	t.Parallel()
	t.Run("Should keep siblings running after a child fails and fail the parent naming the slug", func(t *testing.T) {
		requireGitForTaskMulti(t)
		var (
			mu       sync.Mutex
			executed []string
		)
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			buildRunID: taskMultiRunIDBuilder("task-multi-parallel-faillate"),
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(_ context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
				mu.Lock()
				executed = append(executed, cfg.Name)
				mu.Unlock()
				if cfg.Name == "alpha" {
					return errors.New("alpha execution boom")
				}
				return nil
			},
		})
		writeTaskMultiWorkflow(t, env, "alpha", "pending")
		writeTaskMultiWorkflow(t, env, "beta", "pending")
		commitTaskMultiGitWorkspace(t, env.workspaceRoot)

		parent := startTaskMultiParallelRunWithLimit(
			t, env, "task-multi-parallel-faillate", []string{"alpha", "beta"}, 2,
		)
		parentRow := waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
			return isTerminalRunStatus(row.Status)
		})
		if parentRow.Status != runStatusFailed {
			t.Fatalf("parent status = %q, want %q", parentRow.Status, runStatusFailed)
		}
		if !strings.Contains(parentRow.ErrorText, "alpha") {
			t.Fatalf("parent error %q must name the failed slug alpha", parentRow.ErrorText)
		}
		requireTaskMultiChildRow(t, env, "child-alpha", parent.RunID, runStatusFailed)
		requireTaskMultiChildRow(t, env, "child-beta", parent.RunID, runStatusCompleted)
		mu.Lock()
		ran := append([]string(nil), executed...)
		mu.Unlock()
		if !slices.Contains(ran, "beta") {
			t.Fatalf("beta did not execute; fail-late must keep siblings running, executed=%v", ran)
		}
		snapshot, err := env.manager.RunMultipleSnapshot(context.Background(), parent.RunID)
		if err != nil {
			t.Fatalf("RunMultipleSnapshot() error = %v", err)
		}
		assertTaskMultiItems(t, snapshot.Items, []apicore.TaskRunMultipleItem{
			{Slug: "alpha", Status: taskMultiItemStatusFailed, RunID: "child-alpha"},
			{Slug: "beta", Status: taskMultiItemStatusCompleted, RunID: "child-beta"},
		})
	})
}

func TestRunManagerTaskRunMultipleParallelCancellation(t *testing.T) {
	t.Parallel()
	t.Run("Should cancel running children and mark not-started items canceled", func(t *testing.T) {
		requireGitForTaskMulti(t)
		entered := make(chan string, 2)
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			buildRunID: taskMultiRunIDBuilder("task-multi-parallel-cancel"),
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
				entered <- cfg.Name
				<-ctx.Done()
				return ctx.Err()
			},
		})
		writeTaskMultiWorkflow(t, env, "alpha", "pending")
		writeTaskMultiWorkflow(t, env, "beta", "pending")
		commitTaskMultiGitWorkspace(t, env.workspaceRoot)

		// Limit 1 keeps beta queued (not started) while alpha runs.
		parent := startTaskMultiParallelRunWithLimit(
			t, env, "task-multi-parallel-cancel", []string{"alpha", "beta"}, 1,
		)
		<-entered // alpha has entered execution; beta is still queued
		if err := env.manager.Cancel(context.Background(), parent.RunID); err != nil {
			t.Fatalf("Cancel() error = %v", err)
		}
		// Repeated cancellation must not corrupt item state.
		_ = env.manager.Cancel(context.Background(), parent.RunID)

		waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
			return row.Status == runStatusCancelled
		})
		// The running child is canceled; the not-started child never creates a run row.
		requireTaskMultiChildRow(t, env, "child-alpha", parent.RunID, runStatusCancelled)
		if _, err := env.globalDB.GetRun(context.Background(), "child-beta"); err == nil {
			t.Fatal("child-beta run row exists; a not-started item must not create a child run")
		}
		snapshot, err := env.manager.RunMultipleSnapshot(context.Background(), parent.RunID)
		if err != nil {
			t.Fatalf("RunMultipleSnapshot() error = %v", err)
		}
		assertTaskMultiItems(t, snapshot.Items, []apicore.TaskRunMultipleItem{
			{Slug: "alpha", Status: taskMultiItemStatusCanceled, RunID: "child-alpha"},
			{Slug: "beta", Status: taskMultiItemStatusCanceled},
		})
	})
}

func TestRunManagerTaskRunMultipleParallelEmitsResolvedLimit(t *testing.T) {
	t.Parallel()
	t.Run("Should emit the resolved parallel limit on the queue-started event", func(t *testing.T) {
		requireGitForTaskMulti(t)
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			buildRunID: taskMultiRunIDBuilder("task-multi-parallel-limitevent"),
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error {
				return nil
			},
		})
		writeTaskMultiWorkflow(t, env, "alpha", "pending")
		writeTaskMultiWorkflow(t, env, "beta", "pending")
		commitTaskMultiGitWorkspace(t, env.workspaceRoot)

		parent := startTaskMultiParallelRunWithLimit(
			t, env, "task-multi-parallel-limitevent", []string{"alpha", "beta"}, 2,
		)
		waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
			return row.Status == runStatusCompleted
		})
		events := allRunEvents(t, parent.RunID)
		found := false
		for idx := range events {
			if events[idx].Kind != eventspkg.EventKindTaskRunMultipleStarted {
				continue
			}
			payload, err := decodeTaskMultiPayload(events[idx])
			if err != nil {
				t.Fatalf("decode started event: %v", err)
			}
			found = true
			if payload.ParallelLimit != 2 {
				t.Fatalf("started event parallel_limit = %d, want 2", payload.ParallelLimit)
			}
		}
		if !found {
			t.Fatal("no task.multi.started event found")
		}
	})
}
