package daemon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/plan"
	"github.com/compozy/compozy/internal/core/run/journal"
	"github.com/compozy/compozy/internal/core/taskgroups"
	workspacecfg "github.com/compozy/compozy/internal/core/workspace"
	"github.com/compozy/compozy/internal/store/globaldb"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
)

func TestTaskGroupPreflightDecisionPlanDriftUT080(t *testing.T) {
	t.Parallel()

	previous := &taskGroupPreflightEvidence{
		initiativeSlug: "initiative",
		taskGroupID:    "TG-002",
		planChecksum:   "before",
		readiness:      taskgroups.Readiness{Eligible: true},
	}
	current := &taskGroupPreflightEvidence{
		initiativeSlug: "initiative",
		taskGroupID:    "TG-002",
		planChecksum:   "after",
		readiness: taskgroups.Readiness{
			Eligible: false,
			DirectUnmet: []taskgroups.Dependency{{
				From: "TG-001",
				To:   "TG-002",
			}},
		},
	}

	_, err := taskGroupPreflightDecision(current, false, previous)
	var problem *apicore.Problem
	if !errors.As(err, &problem) {
		t.Fatalf("UT-080 error = %v, want API problem", err)
	}
	if problem.Status != http.StatusConflict || problem.Code != "task_group_dependencies_unmet" {
		t.Fatalf("UT-080 problem = %#v, want 409 task_group_dependencies_unmet", problem)
	}
	if changed, ok := problem.Details["plan_changed"].(bool); !ok || !changed {
		t.Fatalf("UT-080 details = %#v, want plan_changed=true", problem.Details)
	}
}

func TestTaskMultiGroupChildPreflightDecisionRejectsEligiblePlanDrift(t *testing.T) {
	t.Parallel()

	previous := &taskGroupPreflightEvidence{
		initiativeSlug: "initiative",
		taskGroupID:    "TG-001",
		planChecksum:   "before",
		readiness:      taskgroups.Readiness{Eligible: true},
	}
	current := &taskGroupPreflightEvidence{
		initiativeSlug: "initiative",
		taskGroupID:    "TG-001",
		planChecksum:   "after",
		readiness:      taskgroups.Readiness{Eligible: true},
	}

	_, err := taskMultiGroupChildPreflightDecision(current, false, previous)
	var problem *apicore.Problem
	if !errors.As(err, &problem) {
		t.Fatalf("eligible plan drift error = %v, want API problem", err)
	}
	if problem.Status != http.StatusConflict || problem.Code != "task_group_dependencies_unmet" {
		t.Fatalf("eligible plan drift problem = %#v, want 409 task_group_dependencies_unmet", problem)
	}
	if changed, ok := problem.Details["plan_changed"].(bool); !ok || !changed {
		t.Fatalf("eligible plan drift details = %#v, want plan_changed=true", problem.Details)
	}
}

func TestRunManagerTaskMultiGroupParallelRejectsCompletionBeforeChildStart(t *testing.T) {
	requireGitForTaskMulti(t)

	const (
		initiative = "group-completed-before-child-start"
		parentID   = "group-completed-before-child-start-parent"
	)
	firstStarted := make(chan struct{}, 1)
	releaseFirst := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() {
		releaseOnce.Do(func() {
			close(releaseFirst)
		})
	})

	var executed atomic.Int32
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		buildRunID: taskMultiGroupRunIDBuilder(parentID),
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
			executed.Add(1)
			groupID := taskMultiTaskGroupID(cfg.Name)
			if groupID == "TG-001" {
				firstStarted <- struct{}{}
				select {
				case <-releaseFirst:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return commitTaskMultiGroupAgentChange(ctx, cfg.WorkspaceRoot, groupID, groupID+" result\n")
		},
	})
	writeIndependentTaskGroupFixture(t, env, initiative, 2)
	commitTaskMultiGitWorkspace(t, env.workspaceRoot)

	realGit := env.manager.worktreeAllocator.run
	var worktreeAdds atomic.Int32
	env.manager.worktreeAllocator.run = func(
		ctx context.Context,
		dir string,
		args ...string,
	) (string, error) {
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "add" {
			worktreeAdds.Add(1)
		}
		return realGit(ctx, dir, args...)
	}

	parent := startTaskMultiGroupParallelRun(
		t,
		env,
		parentID,
		initiative,
		[]string{"TG-001", "TG-002"},
		1,
	)
	select {
	case <-firstStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("first task-group child did not start")
	}

	completedSecond := independentTaskGroupSpec("TG-002")
	completedSecond.Completed = true
	writeTaskGroupPlanFile(
		t,
		env,
		initiative,
		[]taskgroups.TaskGroup{
			independentTaskGroupSpec("TG-001"),
			completedSecond,
		},
		nil,
	)
	revalidationErr := rejectCompletedTaskMultiGroupChildStart(
		context.Background(),
		env.workspaceRoot,
		initiative+"/TG-002",
	)
	var problem *apicore.Problem
	if !errors.As(revalidationErr, &problem) {
		t.Fatalf("completed child revalidation error = %v, want API problem", revalidationErr)
	}
	rejected, ok := problem.Details["rejected"].(map[string]any)
	if !ok {
		t.Fatalf("completed child revalidation details = %#v, want rejected map", problem.Details)
	}
	rejection, ok := rejected["TG-002"].(map[string]any)
	if !ok || rejection["reason"] != "already_completed" {
		t.Fatalf("completed child rejection = %#v, want already_completed", rejected["TG-002"])
	}
	releaseOnce.Do(func() {
		close(releaseFirst)
	})

	row := waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
		return isTerminalRunStatus(row.Status)
	})
	if row.Status != runStatusFailed {
		t.Fatalf("parent status = %q error=%q, want failed", row.Status, row.ErrorText)
	}
	items := taskMultiItemsByGroupID(requireTaskMultiGroupSnapshot(t, env, parent.RunID, 2).Items)
	if first := items["TG-001"]; first.Status != taskMultiItemStatusCompleted {
		t.Fatalf("first item = %#v, want completed", first)
	}
	second := items["TG-002"]
	if second.Status != taskMultiItemStatusFailed {
		t.Fatalf("completed-before-start item = %#v, want failed", second)
	}
	if second.WorktreePath != "" {
		t.Fatalf("completed-before-start worktree = %q, want no allocation", second.WorktreePath)
	}
	if got := worktreeAdds.Load(); got != 1 {
		t.Fatalf("worktree allocations = %d, want only the first child", got)
	}
	if got := executed.Load(); got != 1 {
		t.Fatalf("executed children = %d, want only the first child", got)
	}
}

func TestRemapTaskMultiChildRuntimeClonesTaskGroupExecutionScope(t *testing.T) {
	t.Parallel()

	sourceRoot := filepath.Join(string(filepath.Separator), "workspace")
	worktreeRoot := filepath.Join(string(filepath.Separator), "worktree")
	base := &model.RuntimeConfig{
		WorkspaceRoot: sourceRoot,
		ExecutionScope: &model.ExecutionScope{
			SpecDir:        filepath.Join(sourceRoot, ".compozy", "tasks", "initiative"),
			OperationalDir: filepath.Join(sourceRoot, ".compozy", "tasks", "initiative", "_task_groups", "TG-001"),
			WorkflowRef:    "initiative/TG-001",
			TasksDir:       filepath.Join(sourceRoot, ".compozy", "tasks", "initiative", "_task_groups", "TG-001"),
			ReviewsDir:     filepath.Join(sourceRoot, ".compozy", "tasks", "initiative", "_task_groups", "TG-001"),
			MemoryDir:      filepath.Join(sourceRoot, ".compozy", "tasks", "initiative", "memory", "TG-001"),
		},
	}

	got, err := remapTaskMultiChildRuntime(base, worktreeRoot, "initiative/TG-001", "parent")
	if err != nil {
		t.Fatalf("remapTaskMultiChildRuntime() error = %v", err)
	}
	if got.ExecutionScope == base.ExecutionScope {
		t.Fatal("ExecutionScope pointer was reused, want independent clone")
	}
	if got.ExecutionScope.WorkflowRef != base.ExecutionScope.WorkflowRef {
		t.Fatalf("WorkflowRef = %q, want preserved %q",
			got.ExecutionScope.WorkflowRef, base.ExecutionScope.WorkflowRef)
	}
	if got.TasksDir != got.ExecutionScope.TasksDir {
		t.Fatalf("TasksDir = %q, want canonical execution scope path %q",
			got.TasksDir, got.ExecutionScope.TasksDir)
	}
	for label, path := range map[string]string{
		"SpecDir":        got.ExecutionScope.SpecDir,
		"OperationalDir": got.ExecutionScope.OperationalDir,
		"TasksDir":       got.ExecutionScope.TasksDir,
		"ReviewsDir":     got.ExecutionScope.ReviewsDir,
		"MemoryDir":      got.ExecutionScope.MemoryDir,
	} {
		if !strings.HasPrefix(path, worktreeRoot+string(filepath.Separator)) {
			t.Fatalf("%s = %q, want remapped below %q", label, path, worktreeRoot)
		}
	}
	if !strings.HasPrefix(base.ExecutionScope.SpecDir, sourceRoot+string(filepath.Separator)) {
		t.Fatalf("base execution scope mutated: %#v", base.ExecutionScope)
	}
}

func TestRunManagerTaskMultiGroupChildRejectsEscapedExecutionScopeBeforeLaunch(t *testing.T) {
	requireGitForTaskMulti(t)

	const (
		initiative = "escaped-execution-scope"
		parentID   = "escaped-execution-scope-parent"
		groupID    = "TG-001"
	)
	var launched atomic.Int32
	var executed atomic.Int32
	buildRunID := taskMultiGroupRunIDBuilder(parentID)
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		buildRunID: func(cfg *model.RuntimeConfig) (string, error) {
			if cfg != nil && cfg.ParentRunID == parentID {
				launched.Add(1)
			}
			return buildRunID(cfg)
		},
		prepare: plan.Prepare,
		execute: func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error {
			executed.Add(1)
			return nil
		},
	})
	writeIndependentTaskGroupFixture(t, env, initiative, 1)
	commitTaskMultiGitWorkspace(t, env.workspaceRoot)

	request := taskMultiGroupRequest(t, env, parentID, initiative, []string{groupID}, 1, true)
	slugs, err := normalizeTaskMultiRequest(request)
	if err != nil {
		t.Fatalf("normalizeTaskMultiRequest() error = %v", err)
	}
	childOverrides, err := taskMultiChildRuntimeOverrides(request.RuntimeOverrides)
	if err != nil {
		t.Fatalf("taskMultiChildRuntimeOverrides() error = %v", err)
	}
	prepared, err := env.manager.prepareTaskMultiStart(
		context.Background(),
		env.workspaceRoot,
		slugs,
		workspacecfg.TaskRunMultipleModeParallel,
		request,
		childOverrides,
	)
	if err != nil {
		t.Fatalf("prepareTaskMultiStart() error = %v", err)
	}
	prepared.executionKind = apicore.ExecutionKindTaskMultiGroupParallel
	if len(prepared.items) != 1 || prepared.items[0].runtimeCfg == nil ||
		prepared.items[0].runtimeCfg.ExecutionScope == nil {
		t.Fatalf("prepared items = %#v, want one task-group runtime", prepared.items)
	}
	item := prepared.items[0]
	item.runtimeCfg.ExecutionScope.MemoryDir = filepath.Join(
		filepath.Dir(env.workspaceRoot),
		"foreign-memory",
	)

	base, err := env.manager.worktreeAllocator.ResolveBase(context.Background(), env.workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveBase() error = %v", err)
	}
	child, err := env.manager.startTaskMultiWorktreeChild(
		&activeRun{
			runID:     parentID,
			ctx:       context.Background(),
			mode:      runModeTaskMulti,
			taskMulti: prepared,
		},
		prepared,
		item,
		0,
		1,
		base,
	)
	if child.Run.RunID != "" {
		waitForRun(t, env.globalDB, child.Run.RunID, func(row globaldb.Run) bool {
			return isTerminalRunStatus(row.Status)
		})
	}
	if child.Allocation.Path != "" {
		if removeErr := env.manager.worktreeAllocator.Remove(
			context.Background(),
			env.workspaceRoot,
			child.Allocation.Path,
		); removeErr != nil {
			t.Fatalf("remove rejected child worktree: %v", removeErr)
		}
	}
	if err == nil || !strings.Contains(err.Error(), "execution scope memory directory") {
		t.Fatalf("startTaskMultiWorktreeChild() error = %v, want escaped memory directory rejection", err)
	}
	if got := launched.Load(); got != 0 {
		t.Fatalf("launched children = %d, want 0", got)
	}
	if got := executed.Load(); got != 0 {
		t.Fatalf("executed children = %d, want 0", got)
	}
}

func TestRunManagerTaskMultiGroupParallelPersistsCanonicalTaskArtifacts(t *testing.T) {
	requireGitForTaskMulti(t)

	const (
		initiative = "group-artifact-persistence"
		parentID   = "group-artifact-persistence-parent"
		groupID    = "TG-001"
	)
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		buildRunID: taskMultiGroupRunIDBuilder(parentID),
		prepare:    plan.Prepare,
		execute: func(ctx context.Context, prep *model.SolvePreparation, cfg *model.RuntimeConfig) error {
			if cfg.ExecutionScope == nil {
				return errors.New("task-group execution scope was not preserved")
			}
			if cfg.TasksDir != cfg.ExecutionScope.TasksDir {
				return fmt.Errorf(
					"runtime task directory %s differs from canonical scope %s",
					cfg.TasksDir,
					cfg.ExecutionScope.TasksDir,
				)
			}
			if len(prep.Jobs) != 1 {
				return fmt.Errorf("prepared jobs = %d, want 1", len(prep.Jobs))
			}
			taskPath, err := taskMultiPromptTaskPath(string(prep.Jobs[0].Prompt))
			if err != nil {
				return err
			}
			wantTaskPath := filepath.Join(cfg.ExecutionScope.TasksDir, "task_01.md")
			if filepath.Clean(taskPath) != filepath.Clean(wantTaskPath) {
				return fmt.Errorf("prompt task path = %s, want %s", taskPath, wantTaskPath)
			}
			taskContent, err := os.ReadFile(taskPath)
			if err != nil {
				return fmt.Errorf("read prompted task path: %w", err)
			}
			completed := strings.Replace(string(taskContent), "status: pending", "status: completed", 1)
			if completed == string(taskContent) {
				return errors.New("prompted task did not contain pending status")
			}
			completed += "- [x] Canonical artifact persisted\n"
			if err := os.WriteFile(taskPath, []byte(completed), 0o600); err != nil {
				return fmt.Errorf("complete prompted task: %w", err)
			}
			if err := os.MkdirAll(cfg.ExecutionScope.MemoryDir, 0o755); err != nil {
				return fmt.Errorf("create task-group memory directory: %w", err)
			}
			if err := os.WriteFile(
				filepath.Join(cfg.ExecutionScope.MemoryDir, "agent-note.md"),
				[]byte("canonical memory persisted\n"),
				0o600,
			); err != nil {
				return fmt.Errorf("write task-group memory: %w", err)
			}
			return commitTaskMultiGroupAgentChange(ctx, cfg.WorkspaceRoot, groupID, "artifact persistence\n")
		},
	})
	writeIndependentTaskGroupFixture(t, env, initiative, 1)
	parentTaskPath := filepath.Join(
		env.workflowDir(initiative),
		"_task_groups",
		groupID,
		"task_01.md",
	)
	commitTaskMultiGitWorkspace(t, env.workspaceRoot)

	parent := startTaskMultiGroupParallelRun(t, env, parentID, initiative, []string{groupID}, 1)
	row := waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
		return isTerminalRunStatus(row.Status)
	})
	if row.Status != runStatusCompleted {
		t.Fatalf("parent status = %q error=%q, want completed", row.Status, row.ErrorText)
	}
	taskContent, err := os.ReadFile(parentTaskPath)
	if err != nil {
		t.Fatalf("read canonical parent task: %v", err)
	}
	if !strings.Contains(string(taskContent), "status: completed") {
		t.Fatalf("canonical parent task = %q, want completed status", taskContent)
	}
	if !strings.Contains(string(taskContent), "- [x] Canonical artifact persisted") {
		t.Fatalf("canonical parent task = %q, want persisted checklist update", taskContent)
	}
	memoryContent, err := os.ReadFile(filepath.Join(filepath.Dir(parentTaskPath), "memory", "agent-note.md"))
	if err != nil {
		t.Fatalf("read canonical parent memory: %v", err)
	}
	if string(memoryContent) != "canonical memory persisted\n" {
		t.Fatalf("canonical parent memory = %q", memoryContent)
	}
	item := taskMultiItemsByGroupID(requireTaskMultiGroupSnapshot(t, env, parent.RunID, 1).Items)[groupID]
	if item.WorktreeStatus != taskMultiWorktreeStatusRemoved {
		t.Fatalf("worktree status = %q, want removed after artifact persistence", item.WorktreeStatus)
	}
}

// Invariant: a parallel Task Group child watches the same canonical
// _task_groups directory exposed to its runtime, so edits are observed before
// end-of-child reconciliation without creating a legacy slug-shaped copy.
func TestRunManagerTaskMultiGroupParallelWatchesCanonicalTaskGroupDirectory(t *testing.T) {
	requireGitForTaskMulti(t)

	const (
		initiative = "group-canonical-watcher"
		parentID   = "group-canonical-watcher-parent"
		groupID    = "TG-001"
	)
	type childObservation struct {
		runID         string
		tasksDir      string
		workspaceRoot string
		workflowRef   string
	}
	started := make(chan childObservation, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseChild := func() {
		releaseOnce.Do(func() {
			close(release)
		})
	}
	t.Cleanup(releaseChild)

	env := newRunManagerTestEnv(t, runManagerTestDeps{
		buildRunID:      taskMultiGroupRunIDBuilder(parentID),
		prepare:         plan.Prepare,
		watcherDebounce: 40 * time.Millisecond,
		execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
			if cfg.ExecutionScope == nil {
				return errors.New("task-group execution scope was not preserved")
			}
			taskPath := filepath.Join(cfg.ExecutionScope.TasksDir, "task_01.md")
			taskContent, err := os.ReadFile(taskPath)
			if err != nil {
				return fmt.Errorf("read canonical child task: %w", err)
			}
			taskContent = append(taskContent, []byte("\nWatcher observed canonical edit.\n")...)
			if err := os.WriteFile(taskPath, taskContent, 0o600); err != nil {
				return fmt.Errorf("write canonical child task: %w", err)
			}
			started <- childObservation{
				runID:         cfg.RunID,
				tasksDir:      cfg.ExecutionScope.TasksDir,
				workspaceRoot: cfg.WorkspaceRoot,
				workflowRef:   cfg.ExecutionScope.WorkflowRef,
			}
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})
	writeIndependentTaskGroupFixture(t, env, initiative, 1)
	commitTaskMultiGitWorkspace(t, env.workspaceRoot)

	parent := startTaskMultiGroupParallelRun(t, env, parentID, initiative, []string{groupID}, 1)
	var observation childObservation
	select {
	case observation = <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("parallel task-group child did not start")
	}

	active := env.manager.getActive(observation.runID)
	if active == nil {
		t.Fatalf("child run %q is not active", observation.runID)
	}
	if active.workflowRoot != observation.tasksDir {
		t.Fatalf("watcher root = %q, want execution scope tasks dir %q",
			active.workflowRoot, observation.tasksDir)
	}
	if active.watcher == nil || active.watcher.workflowRoot != observation.tasksDir {
		t.Fatalf("active watcher = %#v, want root %q", active.watcher, observation.tasksDir)
	}
	decoyDir := model.TaskDirectoryForWorkspace(observation.workspaceRoot, observation.workflowRef)
	if _, err := os.Lstat(decoyDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("legacy slug task directory stat error = %v, want os.ErrNotExist", err)
	}

	waitForCondition(t, 5*time.Second, "canonical task-group watcher event", func() bool {
		return runArtifactSyncCount(t, env.manager, observation.runID) > 0
	})
	requireRunEvent(t, env.manager, observation.runID, eventspkg.EventKindArtifactUpdated)

	releaseChild()
	row := waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
		return isTerminalRunStatus(row.Status)
	})
	if row.Status != runStatusCompleted {
		t.Fatalf("parent status = %q error=%q, want completed", row.Status, row.ErrorText)
	}
}

// Invariant: failure to remove the displaced tree after CAS commit is observable
// as a warning but cannot be returned as an install failure.
func TestRemoveDisplacedTaskMultiArtifactTreeWarnsOnCleanupFailure(t *testing.T) {
	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() {
		slog.SetDefault(previousLogger)
	})

	called := false
	removeDisplacedTaskMultiArtifactTree(
		func(relativePath string) error {
			called = true
			if relativePath != "staged.canonical" {
				t.Fatalf("remove relative path = %q, want staged.canonical", relativePath)
			}
			return errors.New("cleanup denied")
		},
		"staged.canonical",
		"/workspace/.artifacts.reconcile-1.canonical",
	)
	if !called {
		t.Fatal("displaced artifact cleanup was not attempted")
	}
	if !strings.Contains(logs.String(), "remove previous canonical task-group artifacts") ||
		!strings.Contains(logs.String(), "cleanup denied") {
		t.Fatalf("cleanup warning missing from logs: %q", logs.String())
	}
}

// Invariant: a completed group-parallel child keeps its worktree when the
// terminal parent-journal event cannot be committed, even after artifact sync
// has installed the child's canonical updates.
func TestFinishTaskMultiWorktreeChildEmitsCompletionBeforeCleanup(t *testing.T) {
	requireGitForTaskMulti(t)

	const (
		parentRunID = "group-settlement-before-cleanup"
		initiative  = "group-settlement"
		groupID     = "TG-001"
	)
	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	writeFileForTest(t, filepath.Join(env.workspaceRoot, "README.md"), "seed\n")
	writeCompozyTasksGitignore(t, env.workspaceRoot)
	commitTaskMultiGitWorkspace(t, env.workspaceRoot)

	base, err := env.manager.worktreeAllocator.ResolveBase(context.Background(), env.workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveBase() error = %v", err)
	}
	allocation, err := env.manager.worktreeAllocator.Allocate(context.Background(), taskMultiWorktreeSpec{
		WorkspaceRoot: env.workspaceRoot,
		ParentRunID:   parentRunID,
		Slug:          initiative + "/" + groupID,
		Index:         0,
		TaskNumber:    1,
		Base:          base,
	})
	if err != nil {
		t.Fatalf("Allocate() error = %v", err)
	}
	t.Cleanup(func() {
		if _, statErr := os.Stat(allocation.Path); statErr == nil {
			if removeErr := env.manager.worktreeAllocator.Remove(
				context.Background(),
				env.workspaceRoot,
				allocation.Path,
			); removeErr != nil {
				t.Errorf("remove preserved test worktree: %v", removeErr)
			}
		}
	})

	parentArtifacts := filepath.Join(
		env.workspaceRoot,
		".compozy",
		"tasks",
		initiative,
		"_task_groups",
		groupID,
	)
	childArtifacts, err := remapTaskMultiExecutionScopePath(
		parentArtifacts,
		env.workspaceRoot,
		allocation.Path,
	)
	if err != nil {
		t.Fatalf("remapTaskMultiExecutionScopePath() error = %v", err)
	}
	writeFileForTest(t, filepath.Join(parentArtifacts, "task.md"), "before\n")
	writeFileForTest(t, filepath.Join(childArtifacts, "task.md"), "before\n")
	baseline, err := captureTaskMultiArtifactTreeAt(taskMultiArtifactLocation{
		workspaceRoot: allocation.Path,
		path:          childArtifacts,
	})
	if err != nil {
		t.Fatalf("capture artifact baseline: %v", err)
	}
	if err := os.WriteFile(filepath.Join(childArtifacts, "task.md"), []byte("after\n"), 0o600); err != nil {
		t.Fatalf("write child artifact update: %v", err)
	}

	scope, err := model.OpenBaseRunScope(context.Background(), &model.RuntimeConfig{
		RunID:   parentRunID,
		RunsDir: env.paths.RunsDir,
	})
	if err != nil {
		t.Fatalf("OpenBaseRunScope() error = %v", err)
	}
	if scope.RunEventBus() != nil {
		t.Cleanup(func() {
			_ = scope.RunEventBus().Close(context.Background())
		})
	}
	if err := scope.RunJournal().Close(context.Background()); err != nil {
		t.Fatalf("close parent run journal: %v", err)
	}
	active := &activeRun{
		runID: parentRunID,
		mode:  runModeTaskMulti,
		scope: scope,
		ctx:   context.Background(),
	}
	parentRuntime := &model.RuntimeConfig{
		WorkspaceRoot: env.workspaceRoot,
		ExecutionScope: &model.ExecutionScope{
			OperationalDir: parentArtifacts,
		},
	}
	childRuntime := &model.RuntimeConfig{
		WorkspaceRoot: allocation.Path,
		ExecutionScope: &model.ExecutionScope{
			OperationalDir: childArtifacts,
		},
	}
	prepared := &preparedTaskMulti{
		mode:          workspacecfg.TaskRunMultipleModeParallel,
		executionKind: apicore.ExecutionKindTaskMultiGroupParallel,
		workspace:     globaldb.Workspace{RootDir: env.workspaceRoot},
	}
	item := preparedTaskMultiItem{
		slug:       initiative + "/" + groupID,
		runtimeCfg: parentRuntime,
	}

	err = env.manager.finishTaskMultiWorktreeChild(
		active,
		prepared,
		item,
		0,
		1,
		globaldb.Run{RunID: "completed-child", Status: runStatusCompleted},
		allocation,
		childRuntime,
		baseline,
	)
	if !errors.Is(err, journal.ErrClosed) {
		t.Fatalf("finishTaskMultiWorktreeChild() error = %v, want journal.ErrClosed", err)
	}
	canonical, err := os.ReadFile(filepath.Join(parentArtifacts, "task.md"))
	if err != nil {
		t.Fatalf("read synced canonical artifact: %v", err)
	}
	if string(canonical) != "after\n" {
		t.Fatalf("canonical artifact = %q, want synced child update", canonical)
	}
	if _, err := os.Stat(allocation.Path); err != nil {
		t.Fatalf("worktree removed before terminal settlement: %v", err)
	}
}

// Invariant: a failed cleanup-metadata write never leaves projected state as
// durable history when the actual worktree or branch cleanup diverges.
func TestCleanupSettledTaskWorktreeRecordsOnlySettledMetadata(t *testing.T) {
	requireGitForTaskMulti(t)

	tests := []struct {
		name               string
		failGit            func([]string) bool
		wantWorktreeExists bool
		wantStatus         string
		wantReason         string
	}{
		{
			name: "worktree removal fails",
			failGit: func(args []string) bool {
				return len(args) >= 2 && args[0] == "worktree" && args[1] == "remove"
			},
			wantWorktreeExists: true,
			wantStatus:         taskMultiWorktreeStatusPreserved,
			wantReason:         "remove safe worktree",
		},
		{
			name: "empty result branch deletion fails",
			failGit: func(args []string) bool {
				return len(args) >= 3 && args[0] == "branch" && args[1] == "-d"
			},
			wantWorktreeExists: false,
			wantStatus:         taskMultiWorktreeStatusRemoved,
			wantReason:         "empty result branch cleanup failed",
		},
	}

	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parentRunID := fmt.Sprintf("group-cleanup-settled-metadata-%d", index)
			slug := fmt.Sprintf("group-cleanup/TG-%03d", index+1)
			env := newRunManagerTestEnv(t, runManagerTestDeps{})
			writeFileForTest(t, filepath.Join(env.workspaceRoot, "README.md"), "seed\n")
			commitTaskMultiGitWorkspace(t, env.workspaceRoot)

			base, err := env.manager.worktreeAllocator.ResolveBase(context.Background(), env.workspaceRoot)
			if err != nil {
				t.Fatalf("ResolveBase() error = %v", err)
			}
			resultBranch, err := taskMultiResultBranch(parentRunID, 0, slug)
			if err != nil {
				t.Fatalf("taskMultiResultBranch() error = %v", err)
			}
			allocation, err := env.manager.worktreeAllocator.Allocate(
				context.Background(),
				taskMultiWorktreeSpec{
					WorkspaceRoot: env.workspaceRoot,
					ParentRunID:   parentRunID,
					Slug:          slug,
					Index:         0,
					TaskNumber:    1,
					ResultBranch:  resultBranch,
					Base:          base,
				},
			)
			if err != nil {
				t.Fatalf("Allocate() error = %v", err)
			}

			realGit := env.manager.worktreeAllocator.run
			env.manager.worktreeAllocator.run = func(
				ctx context.Context,
				dir string,
				args ...string,
			) (string, error) {
				if tt.failGit(args) {
					return "", errors.New("simulated cleanup divergence")
				}
				return realGit(ctx, dir, args...)
			}
			t.Cleanup(func() {
				env.manager.worktreeAllocator.run = realGit
				if _, statErr := os.Stat(allocation.Path); statErr == nil {
					if removeErr := env.manager.worktreeAllocator.Remove(
						context.Background(),
						env.workspaceRoot,
						allocation.Path,
					); removeErr != nil {
						t.Errorf("remove preserved test worktree: %v", removeErr)
					}
				}
				if _, deleteErr := env.manager.worktreeAllocator.DeleteBranchIfAt(
					context.Background(),
					env.workspaceRoot,
					resultBranch,
					base.Commit,
				); deleteErr != nil {
					t.Errorf("delete preserved test result branch: %v", deleteErr)
				}
			})

			recordErr := errors.New("record settled cleanup metadata")
			var attempted []taskMultiWorktreeAllocation
			var durable []taskMultiWorktreeAllocation
			err = env.manager.cleanupSettledTaskWorktreeAndRecord(
				context.Background(),
				env.workspaceRoot,
				allocation,
				taskMultiWorktreeCleanupPolicy{
					reportNoChanges: true,
				},
				func(settled taskMultiWorktreeAllocation) error {
					attempted = append(attempted, settled)
					if settled.WorktreeStatus == tt.wantStatus &&
						settled.ResultBranch == resultBranch &&
						!settled.NoChanges &&
						strings.Contains(settled.WorktreeReason, tt.wantReason) {
						return recordErr
					}
					durable = append(durable, settled)
					return nil
				},
			)
			if !errors.Is(err, recordErr) {
				t.Fatalf(
					"cleanupSettledTaskWorktreeAndRecord() error = %v, want record error",
					err,
				)
			}
			if len(attempted) != 1 {
				t.Fatalf("cleanup metadata attempts = %d, want 1: %#v", len(attempted), attempted)
			}
			if len(durable) != 0 {
				t.Fatalf("durable cleanup metadata = %#v, want none", durable)
			}
			_, statErr := os.Stat(allocation.Path)
			if tt.wantWorktreeExists && statErr != nil {
				t.Fatalf("preserved worktree stat error = %v", statErr)
			}
			if !tt.wantWorktreeExists && !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("removed worktree stat error = %v, want os.ErrNotExist", statErr)
			}
			if got := runGitOutput(
				t,
				env.workspaceRoot,
				"branch",
				"--list",
				resultBranch,
				"--format=%(refname:short)",
			); got != resultBranch {
				t.Fatalf("result branch after cleanup divergence = %q, want %q", got, resultBranch)
			}
		})
	}
}

// Invariant: canonical task-group edits made after child launch survive beside
// non-conflicting child artifact updates.
func TestRunManagerTaskMultiGroupParallelMergesConcurrentCanonicalArtifactEdits(t *testing.T) {
	requireGitForTaskMulti(t)

	const (
		initiative = "group-artifact-concurrent-edit"
		parentID   = "group-artifact-concurrent-edit-parent"
		groupID    = "TG-001"
	)
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseChild := func() {
		releaseOnce.Do(func() {
			close(release)
		})
	}
	t.Cleanup(releaseChild)
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		buildRunID: taskMultiGroupRunIDBuilder(parentID),
		prepare:    plan.Prepare,
		execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
			if cfg.ExecutionScope == nil {
				return errors.New("task-group execution scope was not preserved")
			}
			started <- struct{}{}
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
			taskPath := filepath.Join(cfg.ExecutionScope.TasksDir, "task_01.md")
			taskContent, err := os.ReadFile(taskPath)
			if err != nil {
				return fmt.Errorf("read child task artifact: %w", err)
			}
			completed := strings.Replace(string(taskContent), "status: pending", "status: completed", 1)
			if completed == string(taskContent) {
				return errors.New("child task artifact did not contain pending status")
			}
			if err := os.WriteFile(taskPath, []byte(completed), 0o600); err != nil {
				return fmt.Errorf("complete child task artifact: %w", err)
			}
			return commitTaskMultiGroupAgentChange(
				ctx,
				cfg.WorkspaceRoot,
				groupID,
				"concurrent artifact edit\n",
			)
		},
	})
	writeIndependentTaskGroupFixture(t, env, initiative, 1)
	parentTaskGroupDir := filepath.Join(
		env.workflowDir(initiative),
		"_task_groups",
		groupID,
	)
	parentTaskPath := filepath.Join(parentTaskGroupDir, "task_01.md")
	parentMemoryPath := filepath.Join(parentTaskGroupDir, "memory", "operator-note.md")
	commitTaskMultiGitWorkspace(t, env.workspaceRoot)

	parent := startTaskMultiGroupParallelRun(t, env, parentID, initiative, []string{groupID}, 1)
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("parallel task-group child did not start")
	}
	if err := os.MkdirAll(filepath.Dir(parentMemoryPath), 0o755); err != nil {
		t.Fatalf("create canonical memory directory: %v", err)
	}
	if err := os.WriteFile(parentMemoryPath, []byte("operator edit after launch\n"), 0o600); err != nil {
		t.Fatalf("write concurrent canonical memory: %v", err)
	}
	releaseChild()

	row := waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
		return isTerminalRunStatus(row.Status)
	})
	if row.Status != runStatusCompleted {
		t.Fatalf("parent status = %q error=%q, want completed", row.Status, row.ErrorText)
	}
	taskContent, err := os.ReadFile(parentTaskPath)
	if err != nil {
		t.Fatalf("read canonical parent task: %v", err)
	}
	if !strings.Contains(string(taskContent), "status: completed") {
		t.Fatalf("canonical parent task = %q, want child completion", taskContent)
	}
	memoryContent, err := os.ReadFile(parentMemoryPath)
	if err != nil {
		t.Fatalf("read concurrent canonical memory: %v", err)
	}
	if string(memoryContent) != "operator edit after launch\n" {
		t.Fatalf("canonical memory = %q, want operator edit", memoryContent)
	}
}

// Invariant: divergent edits to one artifact never overwrite the canonical
// version and leave the child worktree available for manual reconciliation.
func TestRunManagerTaskMultiGroupParallelPreservesWorktreeOnArtifactConflict(t *testing.T) {
	requireGitForTaskMulti(t)

	const (
		initiative = "group-artifact-conflict"
		parentID   = "group-artifact-conflict-parent"
		groupID    = "TG-001"
	)
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseChild := func() {
		releaseOnce.Do(func() {
			close(release)
		})
	}
	t.Cleanup(releaseChild)
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		buildRunID: taskMultiGroupRunIDBuilder(parentID),
		prepare:    plan.Prepare,
		execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
			if cfg.ExecutionScope == nil {
				return errors.New("task-group execution scope was not preserved")
			}
			started <- struct{}{}
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
			taskPath := filepath.Join(cfg.ExecutionScope.TasksDir, "task_01.md")
			taskContent, err := os.ReadFile(taskPath)
			if err != nil {
				return fmt.Errorf("read child task artifact: %w", err)
			}
			taskContent = append(taskContent, "\nchild completion edit\n"...)
			if err := os.WriteFile(taskPath, taskContent, 0o600); err != nil {
				return fmt.Errorf("write child task artifact: %w", err)
			}
			return commitTaskMultiGroupAgentChange(
				ctx,
				cfg.WorkspaceRoot,
				groupID,
				"conflicting artifact edit\n",
			)
		},
	})
	writeIndependentTaskGroupFixture(t, env, initiative, 1)
	parentTaskPath := filepath.Join(
		env.workflowDir(initiative),
		"_task_groups",
		groupID,
		"task_01.md",
	)
	commitTaskMultiGitWorkspace(t, env.workspaceRoot)

	parent := startTaskMultiGroupParallelRun(t, env, parentID, initiative, []string{groupID}, 1)
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("parallel task-group child did not start")
	}
	parentTaskContent, err := os.ReadFile(parentTaskPath)
	if err != nil {
		t.Fatalf("read canonical parent task: %v", err)
	}
	parentTaskContent = append(parentTaskContent, "\nparent review edit\n"...)
	if err := os.WriteFile(parentTaskPath, parentTaskContent, 0o600); err != nil {
		t.Fatalf("write conflicting canonical task edit: %v", err)
	}
	releaseChild()

	row := waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
		return isTerminalRunStatus(row.Status)
	})
	if row.Status != runStatusFailed || !strings.Contains(row.ErrorText, "artifact conflict") {
		t.Fatalf("parent status = %q error=%q, want artifact conflict failure", row.Status, row.ErrorText)
	}
	canonicalContent, err := os.ReadFile(parentTaskPath)
	if err != nil {
		t.Fatalf("read canonical task after conflict: %v", err)
	}
	if !strings.Contains(string(canonicalContent), "parent review edit") ||
		strings.Contains(string(canonicalContent), "child completion edit") {
		t.Fatalf("canonical task after conflict = %q, want only parent edit", canonicalContent)
	}
	item := taskMultiItemsByGroupID(requireTaskMultiGroupSnapshot(t, env, parent.RunID, 1).Items)[groupID]
	if item.Status != taskMultiItemStatusFailed ||
		item.WorktreeStatus != taskMultiWorktreeStatusPreserved {
		t.Fatalf("conflicted artifact item = %#v, want failed with preserved worktree", item)
	}
	if _, err := os.Stat(item.WorktreePath); err != nil {
		t.Fatalf("preserved conflict worktree %s is unavailable: %v", item.WorktreePath, err)
	}
}

func TestRunManagerTaskMultiGroupParallelPreservesWorktreeWhenArtifactSyncFails(t *testing.T) {
	requireGitForTaskMulti(t)

	const (
		initiative = "group-artifact-sync-failure"
		parentID   = "group-artifact-sync-failure-parent"
		groupID    = "TG-001"
	)
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		buildRunID: taskMultiGroupRunIDBuilder(parentID),
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
			if cfg.ExecutionScope == nil {
				return errors.New("task-group execution scope was not preserved")
			}
			if err := os.Symlink(
				"task_01.md",
				filepath.Join(cfg.ExecutionScope.OperationalDir, "artifact-link"),
			); err != nil {
				return fmt.Errorf("create invalid artifact symlink: %w", err)
			}
			return commitTaskMultiGroupAgentChange(ctx, cfg.WorkspaceRoot, groupID, "artifact sync failure\n")
		},
	})
	writeIndependentTaskGroupFixture(t, env, initiative, 1)
	commitTaskMultiGitWorkspace(t, env.workspaceRoot)

	parent := startTaskMultiGroupParallelRun(t, env, parentID, initiative, []string{groupID}, 1)
	row := waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
		return isTerminalRunStatus(row.Status)
	})
	if row.Status != runStatusFailed {
		t.Fatalf("parent status = %q error=%q, want failed", row.Status, row.ErrorText)
	}
	item := taskMultiItemsByGroupID(requireTaskMultiGroupSnapshot(t, env, parent.RunID, 1).Items)[groupID]
	if item.Status != taskMultiItemStatusFailed ||
		item.WorktreeStatus != taskMultiWorktreeStatusPreserved ||
		!strings.Contains(item.ErrorText, "symlink") {
		t.Fatalf("failed artifact item = %#v, want failed with preserved worktree", item)
	}
	if _, err := os.Stat(item.WorktreePath); err != nil {
		t.Fatalf("preserved worktree %s is unavailable: %v", item.WorktreePath, err)
	}
}

// Invariant: a child cannot replace its operational directory with a sibling
// symlink and reconcile that sibling's artifacts into the canonical group.
func TestRunManagerTaskMultiGroupParallelRejectsSymlinkedOperationalRoot(t *testing.T) {
	requireGitForTaskMulti(t)

	const (
		initiative = "group-artifact-root-symlink"
		parentID   = "group-artifact-root-symlink-parent"
		groupID    = "TG-001"
		siblingID  = "TG-002"
	)
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		buildRunID: taskMultiGroupRunIDBuilder(parentID),
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
			if cfg.ExecutionScope == nil {
				return errors.New("task-group execution scope was not preserved")
			}
			operationalRoot := cfg.ExecutionScope.OperationalDir
			siblingRoot := filepath.Join(filepath.Dir(operationalRoot), siblingID)
			if err := os.RemoveAll(operationalRoot); err != nil {
				return fmt.Errorf("remove child operational root: %w", err)
			}
			if err := os.Symlink(filepath.Base(siblingRoot), operationalRoot); err != nil {
				return fmt.Errorf("replace child operational root with sibling symlink: %w", err)
			}
			return commitTaskMultiGroupAgentChange(
				ctx,
				cfg.WorkspaceRoot,
				groupID,
				"symlinked operational root\n",
			)
		},
	})
	writeIndependentTaskGroupFixture(t, env, initiative, 2)
	artifactPaths := make([]string, 0, 6)
	for _, id := range []string{groupID, siblingID} {
		reviewPath := filepath.Join("_task_groups", id, "reviews", "review.md")
		memoryPath := filepath.Join("_task_groups", id, "memory", "MEMORY.md")
		env.writeWorkflowFile(t, initiative, reviewPath, id+" canonical review\n")
		env.writeWorkflowFile(t, initiative, memoryPath, id+" canonical memory\n")
		artifactPaths = append(
			artifactPaths,
			filepath.Join("_task_groups", id, "task_01.md"),
			reviewPath,
			memoryPath,
		)
	}
	commitTaskMultiGitWorkspace(t, env.workspaceRoot)
	canonicalBefore := readTaskMultiGroupArtifactFiles(
		t,
		env.workflowDir(initiative),
		artifactPaths,
	)

	parent := startTaskMultiGroupParallelRun(t, env, parentID, initiative, []string{groupID}, 1)
	row := waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
		return isTerminalRunStatus(row.Status)
	})
	if row.Status != runStatusFailed || !strings.Contains(row.ErrorText, "symlink") {
		t.Fatalf("parent status = %q error=%q, want symlink failure", row.Status, row.ErrorText)
	}
	item := taskMultiItemsByGroupID(requireTaskMultiGroupSnapshot(t, env, parent.RunID, 1).Items)[groupID]
	if item.Status != taskMultiItemStatusFailed ||
		item.WorktreeStatus != taskMultiWorktreeStatusPreserved {
		t.Fatalf("symlinked artifact item = %#v, want failed with preserved worktree", item)
	}
	if _, err := os.Stat(item.WorktreePath); err != nil {
		t.Fatalf("preserved worktree %s is unavailable: %v", item.WorktreePath, err)
	}
	canonicalAfter := readTaskMultiGroupArtifactFiles(
		t,
		env.workflowDir(initiative),
		artifactPaths,
	)
	if !reflect.DeepEqual(canonicalAfter, canonicalBefore) {
		t.Fatalf(
			"canonical task-group artifacts changed through child root symlink:\nbefore=%q\nafter=%q",
			canonicalBefore,
			canonicalAfter,
		)
	}
}

// Invariant: every component leading to a child artifact root is a real
// directory, so an in-workspace parent symlink cannot redirect reconciliation.
func TestRunManagerTaskMultiGroupParallelRejectsSymlinkedOperationalParent(t *testing.T) {
	requireGitForTaskMulti(t)

	const (
		initiative = "group-artifact-parent-symlink"
		parentID   = "group-artifact-parent-symlink-parent"
		groupID    = "TG-001"
	)
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		buildRunID: taskMultiGroupRunIDBuilder(parentID),
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
			if cfg.ExecutionScope == nil {
				return errors.New("task-group execution scope was not preserved")
			}
			operationalRoot := cfg.ExecutionScope.OperationalDir
			groupsRoot := filepath.Dir(operationalRoot)
			foreignRoot := filepath.Join(filepath.Dir(groupsRoot), "foreign-task-groups")
			if err := os.Rename(groupsRoot, foreignRoot); err != nil {
				return fmt.Errorf("move child task-group tree: %w", err)
			}
			if err := os.Symlink(filepath.Base(foreignRoot), groupsRoot); err != nil {
				return fmt.Errorf("replace child task-group parent with symlink: %w", err)
			}
			foreignTask := filepath.Join(foreignRoot, groupID, "task_01.md")
			if err := os.WriteFile(foreignTask, []byte("foreign task-group content\n"), 0o600); err != nil {
				return fmt.Errorf("write foreign task-group artifact: %w", err)
			}
			return commitTaskMultiGroupAgentChange(
				ctx,
				cfg.WorkspaceRoot,
				groupID,
				"symlinked operational parent\n",
			)
		},
	})
	writeIndependentTaskGroupFixture(t, env, initiative, 1)
	commitTaskMultiGitWorkspace(t, env.workspaceRoot)
	taskPath := filepath.Join("_task_groups", groupID, "task_01.md")
	canonicalBefore := readTaskMultiGroupArtifactFiles(
		t,
		env.workflowDir(initiative),
		[]string{taskPath},
	)

	parent := startTaskMultiGroupParallelRun(t, env, parentID, initiative, []string{groupID}, 1)
	row := waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
		return isTerminalRunStatus(row.Status)
	})
	if row.Status != runStatusFailed || !strings.Contains(row.ErrorText, "symlink") {
		t.Fatalf("parent status = %q error=%q, want symlink failure", row.Status, row.ErrorText)
	}
	item := taskMultiItemsByGroupID(requireTaskMultiGroupSnapshot(t, env, parent.RunID, 1).Items)[groupID]
	if item.Status != taskMultiItemStatusFailed ||
		item.WorktreeStatus != taskMultiWorktreeStatusPreserved {
		t.Fatalf("symlinked parent item = %#v, want failed with preserved worktree", item)
	}
	if _, err := os.Stat(item.WorktreePath); err != nil {
		t.Fatalf("preserved worktree %s is unavailable: %v", item.WorktreePath, err)
	}
	canonicalAfter := readTaskMultiGroupArtifactFiles(
		t,
		env.workflowDir(initiative),
		[]string{taskPath},
	)
	if !reflect.DeepEqual(canonicalAfter, canonicalBefore) {
		t.Fatalf(
			"canonical task-group artifacts changed through child parent symlink:\nbefore=%q\nafter=%q",
			canonicalBefore,
			canonicalAfter,
		)
	}
}

func TestRunManagerTaskMultiGroupParallelIsolationAndAgentCommits(t *testing.T) {
	// IT-001, IT-002, IT-003: real git worktrees prove isolation, agent-owned
	// commits, an untouched checkout, and no-change branch cleanup.
	requireGitForTaskMulti(t)

	const (
		initiative = "group-isolation"
		parentID   = "group-isolation-parent"
	)
	var (
		mu            sync.Mutex
		executionRefs []string
	)
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		buildRunID: taskMultiGroupRunIDBuilder(parentID),
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
			if cfg.ExecutionScope == nil {
				return errors.New("task-group execution scope was not preserved")
			}
			mu.Lock()
			executionRefs = append(executionRefs, cfg.ExecutionScope.WorkflowRef)
			mu.Unlock()
			groupID := taskMultiTaskGroupID(cfg.Name)
			if groupID == "TG-003" {
				return nil
			}
			return commitTaskMultiGroupAgentChange(ctx, cfg.WorkspaceRoot, groupID, groupID+" result\n")
		},
	})
	writeIndependentTaskGroupFixture(t, env, initiative, 3)
	commitTaskMultiGitWorkspace(t, env.workspaceRoot)
	baseCommit := runGitOutput(t, env.workspaceRoot, "rev-parse", "HEAD")
	baseBranch := runGitOutput(t, env.workspaceRoot, "branch", "--show-current")
	baseStatus := runGitOutput(t, env.workspaceRoot, "status", "--porcelain")

	parent := startTaskMultiGroupParallelRun(
		t,
		env,
		parentID,
		initiative,
		[]string{"TG-001", "TG-002", "TG-003"},
		0,
	)
	row := waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
		return isTerminalRunStatus(row.Status)
	})
	if row.Status != runStatusCompleted {
		t.Fatalf("IT-001 parent status = %q error=%q, want completed", row.Status, row.ErrorText)
	}

	snapshot := requireTaskMultiGroupSnapshot(t, env, parent.RunID, 3)
	if snapshot.ExecutionKind != apicore.ExecutionKindTaskMultiGroupParallel {
		t.Fatalf("execution kind = %q, want task_multi_group_parallel", snapshot.ExecutionKind)
	}
	items := taskMultiItemsByGroupID(snapshot.Items)
	for _, groupID := range []string{"TG-001", "TG-002"} {
		item := items[groupID]
		if item.Status != taskMultiItemStatusCompleted || strings.TrimSpace(item.ResultBranch) == "" {
			t.Fatalf("%s item = %#v, want completed with branch", groupID, item)
		}
		if item.BaseCommit != baseCommit || item.BaseBranch != baseBranch {
			t.Fatalf("%s base = %s/%s, want %s/%s",
				groupID, item.BaseBranch, item.BaseCommit, baseBranch, baseCommit)
		}
		if item.WorktreeStatus != taskMultiWorktreeStatusRemoved {
			t.Fatalf("%s worktree status = %q, want removed", groupID, item.WorktreeStatus)
		}
		if got := runGitOutput(
			t,
			env.workspaceRoot,
			"rev-list",
			"--count",
			baseCommit+".."+item.ResultBranch,
		); got != "1" {
			t.Fatalf("%s commits after base = %q, want 1 agent commit", groupID, got)
		}
		if got := runGitOutput(
			t,
			env.workspaceRoot,
			"log",
			"-1",
			"--format=%an",
			item.ResultBranch,
		); got != "Task Group Agent" {
			t.Fatalf("%s branch author = %q, want Task Group Agent", groupID, got)
		}
		if got := runGitOutput(
			t,
			env.workspaceRoot,
			"show",
			item.ResultBranch+":"+strings.ToLower(groupID)+".txt",
		); got != groupID+" result" {
			t.Fatalf("%s branch output = %q", groupID, got)
		}
		otherID := "TG-001"
		if groupID == otherID {
			otherID = "TG-002"
		}
		if got := runGitOutputAllowFailure(
			t,
			env.workspaceRoot,
			"show",
			item.ResultBranch+":"+strings.ToLower(otherID)+".txt",
		); got == nil {
			t.Fatalf("%s branch unexpectedly contains sibling %s output", groupID, otherID)
		}
	}
	noChanges := items["TG-003"]
	if noChanges.Status != taskMultiItemStatusNoChanges ||
		noChanges.ResultBranch != "" ||
		noChanges.WorktreeStatus != taskMultiWorktreeStatusRemoved {
		t.Fatalf("IT-003 no-change item = %#v", noChanges)
	}

	if got := runGitOutput(t, env.workspaceRoot, "rev-parse", "HEAD"); got != baseCommit {
		t.Fatalf("checkout HEAD = %q, want unchanged %q", got, baseCommit)
	}
	if got := runGitOutput(t, env.workspaceRoot, "branch", "--show-current"); got != baseBranch {
		t.Fatalf("checkout branch = %q, want unchanged %q", got, baseBranch)
	}
	if got := runGitOutput(t, env.workspaceRoot, "status", "--porcelain"); got != baseStatus {
		t.Fatalf("checkout status = %q, want unchanged %q", got, baseStatus)
	}

	mu.Lock()
	gotRefs := append([]string(nil), executionRefs...)
	mu.Unlock()
	wantRefs := []string{
		initiative + "/TG-001",
		initiative + "/TG-002",
		initiative + "/TG-003",
	}
	slices.Sort(gotRefs)
	if !reflect.DeepEqual(gotRefs, wantRefs) {
		t.Fatalf("execution scopes = %#v, want %#v", gotRefs, wantRefs)
	}
	for _, groupID := range []string{"TG-001", "TG-002", "TG-003"} {
		assertTaskMultiWorktreeMetadataBeforeChildStart(t, env.manager, parent.RunID, initiative+"/"+groupID)
		assertTaskMultiGroupEventsCarryID(t, env.manager, parent.RunID, groupID)
	}
}

func TestRunManagerTaskMultiGroupParallelFaultIsolation(t *testing.T) {
	requireGitForTaskMulti(t)

	t.Run("IT-006 partial success preserves failed group and completes siblings", func(t *testing.T) {
		const (
			initiative = "partial-groups"
			parentID   = "partial-groups-parent"
		)
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			buildRunID: taskMultiGroupRunIDBuilder(parentID),
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
				groupID := taskMultiTaskGroupID(cfg.Name)
				if groupID == "TG-002" {
					return errors.New("simulated TG-002 failure")
				}
				return commitTaskMultiGroupAgentChange(ctx, cfg.WorkspaceRoot, groupID, groupID+" success\n")
			},
		})
		writeIndependentTaskGroupFixture(t, env, initiative, 3)
		commitTaskMultiGitWorkspace(t, env.workspaceRoot)

		parent := startTaskMultiGroupParallelRun(
			t, env, parentID, initiative, []string{"TG-001", "TG-002", "TG-003"}, 3,
		)
		row := waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
			return isTerminalRunStatus(row.Status)
		})
		if row.Status != runStatusFailed ||
			!strings.Contains(row.ErrorText, "partial success") ||
			!strings.Contains(row.ErrorText, "TG-002") ||
			!strings.Contains(row.ErrorText, "worktree preserved at") {
			t.Fatalf("IT-006 parent = status:%q error:%q", row.Status, row.ErrorText)
		}
		snapshot := requireTaskMultiGroupSnapshot(t, env, parent.RunID, 3)
		items := taskMultiItemsByGroupID(snapshot.Items)
		for _, groupID := range []string{"TG-001", "TG-003"} {
			if item := items[groupID]; item.Status != taskMultiItemStatusCompleted ||
				item.ResultBranch == "" ||
				item.WorktreeStatus != taskMultiWorktreeStatusRemoved {
				t.Fatalf("IT-006 successful %s = %#v", groupID, item)
			}
		}
		failed := items["TG-002"]
		if failed.Status != taskMultiItemStatusFailed ||
			failed.WorktreeStatus != taskMultiWorktreeStatusPreserved ||
			failed.WorktreePath == "" {
			t.Fatalf("IT-006 failed TG-002 = %#v", failed)
		}
		if _, err := os.Stat(failed.WorktreePath); err != nil {
			t.Fatalf("IT-006 preserved worktree stat = %v", err)
		}
		if !snapshot.Incomplete || !containsStringFragment(snapshot.IncompleteReasons, "TG-002") {
			t.Fatalf("IT-006 incomplete = %v reasons=%#v", snapshot.Incomplete, snapshot.IncompleteReasons)
		}
	})

	t.Run("IT-007 all failures preserve every group without sibling cancellation", func(t *testing.T) {
		const (
			initiative = "failed-groups"
			parentID   = "failed-groups-parent"
		)
		var executed atomic.Int32
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			buildRunID: taskMultiGroupRunIDBuilder(parentID),
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error {
				executed.Add(1)
				return errors.New("simulated group failure")
			},
		})
		writeIndependentTaskGroupFixture(t, env, initiative, 3)
		commitTaskMultiGitWorkspace(t, env.workspaceRoot)

		parent := startTaskMultiGroupParallelRun(
			t, env, parentID, initiative, []string{"TG-001", "TG-002", "TG-003"}, 3,
		)
		row := waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
			return isTerminalRunStatus(row.Status)
		})
		if row.Status != runStatusFailed || strings.Contains(row.ErrorText, "partial success") {
			t.Fatalf("IT-007 parent = status:%q error:%q", row.Status, row.ErrorText)
		}
		if got := executed.Load(); got != 3 {
			t.Fatalf("IT-007 executed groups = %d, want 3", got)
		}
		snapshot := requireTaskMultiGroupSnapshot(t, env, parent.RunID, 3)
		for groupID, item := range taskMultiItemsByGroupID(snapshot.Items) {
			if item.Status != taskMultiItemStatusFailed ||
				item.WorktreeStatus != taskMultiWorktreeStatusPreserved {
				t.Fatalf("IT-007 %s = %#v", groupID, item)
			}
		}
	})

	t.Run("IT-009 parent cancellation is distinct from child failure", func(t *testing.T) {
		const (
			initiative = "cancel-groups"
			parentID   = "cancel-groups-parent"
		)
		started := make(chan string, 3)
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			buildRunID: taskMultiGroupRunIDBuilder(parentID),
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
				started <- taskMultiTaskGroupID(cfg.Name)
				<-ctx.Done()
				return ctx.Err()
			},
		})
		writeIndependentTaskGroupFixture(t, env, initiative, 3)
		commitTaskMultiGitWorkspace(t, env.workspaceRoot)
		parent := startTaskMultiGroupParallelRun(
			t, env, parentID, initiative, []string{"TG-001", "TG-002", "TG-003"}, 2,
		)
		waitForTaskMultiGroupStarts(t, started, 2)
		if err := env.manager.Cancel(context.Background(), parent.RunID); err != nil {
			t.Fatalf("IT-009 Cancel() error = %v", err)
		}
		row := waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
			return isTerminalRunStatus(row.Status)
		})
		if row.Status != runStatusCancelled {
			t.Fatalf("IT-009 parent status = %q error=%q, want canceled", row.Status, row.ErrorText)
		}
		snapshot := requireTaskMultiGroupSnapshot(t, env, parent.RunID, 3)
		for _, item := range snapshot.Items {
			if item.Status == taskMultiItemStatusFailed {
				t.Fatalf("IT-009 item reported failed instead of canceled: %#v", item)
			}
			if item.Status != taskMultiItemStatusCanceled {
				t.Fatalf("IT-009 item status = %q, want canceled: %#v", item.Status, item)
			}
			if item.WorktreePath != "" && item.WorktreeStatus != taskMultiWorktreeStatusPreserved {
				t.Fatalf("IT-009 launched item worktree = %#v, want preserved", item)
			}
		}
	})
}

func TestRunManagerTaskMultiGroupParallelBoundedConcurrency(t *testing.T) {
	requireGitForTaskMulti(t)

	tests := []struct {
		name     string
		count    int
		limit    int
		wantPeak int32
	}{
		{name: "IT-013 limit one serializes three groups", count: 3, limit: 1, wantPeak: 1},
		{name: "IT-014 limit above selection runs all groups", count: 3, limit: 10, wantPeak: 3},
		{name: "IT-015 default limit caps four groups at two", count: 4, limit: 0, wantPeak: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			initiative := strings.ReplaceAll(strings.ToLower(test.name[:6]), "_", "-")
			parentID := initiative + "-parent"
			observer := &taskMultiGroupConcurrencyObserver{}
			release := make(chan struct{})
			env := newRunManagerTestEnv(t, runManagerTestDeps{
				buildRunID: taskMultiGroupRunIDBuilder(parentID),
				prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
					return &model.SolvePreparation{}, nil
				},
				execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
					observer.enter()
					defer observer.leave()
					select {
					case <-release:
					case <-ctx.Done():
						return ctx.Err()
					}
					groupID := taskMultiTaskGroupID(cfg.Name)
					return commitTaskMultiGroupAgentChange(ctx, cfg.WorkspaceRoot, groupID, groupID+" concurrent\n")
				},
			})
			writeIndependentTaskGroupFixture(t, env, initiative, test.count)
			commitTaskMultiGitWorkspace(t, env.workspaceRoot)
			groupIDs := make([]string, 0, test.count)
			for index := 1; index <= test.count; index++ {
				groupIDs = append(groupIDs, fmt.Sprintf("TG-%03d", index))
			}
			parent := startTaskMultiGroupParallelRun(
				t, env, parentID, initiative, groupIDs, test.limit,
			)
			observer.waitForPeak(t, test.wantPeak)
			close(release)
			row := waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
				return isTerminalRunStatus(row.Status)
			})
			if row.Status != runStatusCompleted {
				t.Fatalf("%s parent status = %q error=%q", test.name, row.Status, row.ErrorText)
			}
			if got := observer.peak.Load(); got != test.wantPeak {
				t.Fatalf("%s peak = %d, want %d", test.name, got, test.wantPeak)
			}
			snapshot := requireTaskMultiGroupSnapshot(t, env, parent.RunID, test.count)
			for _, item := range snapshot.Items {
				if item.Status != taskMultiItemStatusCompleted || item.ResultBranch == "" {
					t.Fatalf("%s item = %#v, want committed branch", test.name, item)
				}
			}
		})
	}
}

func TestRunManagerTaskMultiGroupParallelLaunchFailuresAreIsolated(t *testing.T) {
	requireGitForTaskMulti(t)

	t.Run("IT-016 existing rendered branch fails one group without overwrite", func(t *testing.T) {
		const (
			initiative = "branch-collision"
			parentID   = "branch-collision-parent"
		)
		template := "collision/{group}"
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			buildRunID: taskMultiGroupRunIDBuilder(parentID),
			loadProjectConfig: func(context.Context, string) (workspacecfg.ProjectConfig, error) {
				return workspacecfg.ProjectConfig{
					Tasks: workspacecfg.TasksConfig{
						Run: workspacecfg.TaskRunConfig{
							ParallelTaskGroups: workspacecfg.ParallelTaskGroupsConfig{
								BranchTemplate: &template,
							},
						},
					},
				}, nil
			},
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
				groupID := taskMultiTaskGroupID(cfg.Name)
				return commitTaskMultiGroupAgentChange(ctx, cfg.WorkspaceRoot, groupID, groupID+" success\n")
			},
		})
		writeIndependentTaskGroupFixture(t, env, initiative, 2)
		commitTaskMultiGitWorkspace(t, env.workspaceRoot)
		base := runGitOutput(t, env.workspaceRoot, "rev-parse", "HEAD")
		runGitOutput(t, env.workspaceRoot, "branch", "collision/tg-001", base)

		parent := startTaskMultiGroupParallelRun(
			t, env, parentID, initiative, []string{"TG-001", "TG-002"}, 2,
		)
		waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
			return isTerminalRunStatus(row.Status)
		})
		items := taskMultiItemsByGroupID(requireTaskMultiGroupSnapshot(t, env, parent.RunID, 2).Items)
		if items["TG-001"].Status != taskMultiItemStatusFailed {
			t.Fatalf("IT-016 collision item = %#v, want failed", items["TG-001"])
		}
		if got := runGitOutput(t, env.workspaceRoot, "rev-parse", "collision/tg-001"); got != base {
			t.Fatalf("IT-016 existing branch moved to %q, want %q", got, base)
		}
		if items["TG-002"].Status != taskMultiItemStatusCompleted || items["TG-002"].ResultBranch == "" {
			t.Fatalf("IT-016 sibling = %#v, want completed", items["TG-002"])
		}
	})

	t.Run("IT-021 allocator failure leaves already started sibling running", func(t *testing.T) {
		const (
			initiative = "allocate-failure"
			parentID   = "allocate-failure-parent"
		)
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			buildRunID: taskMultiGroupRunIDBuilder(parentID),
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
				groupID := taskMultiTaskGroupID(cfg.Name)
				return commitTaskMultiGroupAgentChange(ctx, cfg.WorkspaceRoot, groupID, groupID+" success\n")
			},
		})
		writeIndependentTaskGroupFixture(t, env, initiative, 2)
		commitTaskMultiGitWorkspace(t, env.workspaceRoot)
		realGit := env.manager.worktreeAllocator.run
		env.manager.worktreeAllocator.run = func(
			ctx context.Context,
			dir string,
			args ...string,
		) (string, error) {
			if len(args) >= 4 &&
				args[0] == "worktree" &&
				args[1] == "add" &&
				strings.Contains(strings.Join(args, " "), "tg-001") {
				return "", errors.New("simulated allocator permission failure")
			}
			return realGit(ctx, dir, args...)
		}

		parent := startTaskMultiGroupParallelRun(
			t, env, parentID, initiative, []string{"TG-001", "TG-002"}, 2,
		)
		waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
			return isTerminalRunStatus(row.Status)
		})
		items := taskMultiItemsByGroupID(requireTaskMultiGroupSnapshot(t, env, parent.RunID, 2).Items)
		if items["TG-001"].Status != taskMultiItemStatusFailed ||
			!strings.Contains(items["TG-001"].ErrorText, "allocator permission failure") {
			t.Fatalf("IT-021 failed-to-start item = %#v", items["TG-001"])
		}
		if items["TG-002"].Status != taskMultiItemStatusCompleted || items["TG-002"].ResultBranch == "" {
			t.Fatalf("IT-021 sibling = %#v, want completed", items["TG-002"])
		}
	})
}

func TestRunManagerTaskMultiGroupParallelGitEdgeCases(t *testing.T) {
	requireGitForTaskMulti(t)

	t.Run("IT-004 colliding edits remain isolated and conflict only at merge", func(t *testing.T) {
		const (
			initiative = "merge-conflict"
			parentID   = "merge-conflict-parent"
		)
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			buildRunID: taskMultiGroupRunIDBuilder(parentID),
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
				groupID := taskMultiTaskGroupID(cfg.Name)
				path := filepath.Join(cfg.WorkspaceRoot, "config.toml")
				if err := os.WriteFile(path, []byte("owner = \""+groupID+"\"\n"), 0o600); err != nil {
					return err
				}
				return commitTaskMultiGroupPaths(ctx, cfg.WorkspaceRoot, groupID+" config", "config.toml")
			},
		})
		writeIndependentTaskGroupFixture(t, env, initiative, 2)
		writeFileForTest(t, filepath.Join(env.workspaceRoot, "config.toml"), "owner = \"base\"\n")
		commitTaskMultiGitWorkspace(t, env.workspaceRoot)
		parent := startTaskMultiGroupParallelRun(
			t, env, parentID, initiative, []string{"TG-001", "TG-002"}, 2,
		)
		row := waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
			return isTerminalRunStatus(row.Status)
		})
		if row.Status != runStatusCompleted {
			t.Fatalf("IT-004 parent status = %q error=%q", row.Status, row.ErrorText)
		}
		items := taskMultiItemsByGroupID(requireTaskMultiGroupSnapshot(t, env, parent.RunID, 2).Items)
		mergeRoot := filepath.Join(t.TempDir(), "merge")
		runGitOutput(t, env.workspaceRoot, "worktree", "add", "-q", "--detach", mergeRoot, items["TG-001"].ResultBranch)
		t.Cleanup(func() {
			_, _ = runTaskMultiWorktreeGitCommand(
				context.Background(), env.workspaceRoot, "worktree", "remove", "--force", mergeRoot,
			)
		})
		if _, err := runTaskMultiWorktreeGitCommand(
			context.Background(), mergeRoot, "merge", "--no-commit", items["TG-002"].ResultBranch,
		); err == nil {
			t.Fatal("IT-004 merge error = nil, want shared-file conflict")
		}
		if got := runGitOutput(t, mergeRoot, "diff", "--name-only", "--diff-filter=U"); got != "config.toml" {
			t.Fatalf("IT-004 unmerged files = %q, want config.toml", got)
		}
	})

	t.Run("IT-005 captured base remains stable when checkout advances", func(t *testing.T) {
		const (
			initiative = "captured-base"
			parentID   = "captured-base-parent"
		)
		started := make(chan struct{}, 1)
		release := make(chan struct{})
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			buildRunID: taskMultiGroupRunIDBuilder(parentID),
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
				started <- struct{}{}
				select {
				case <-release:
				case <-ctx.Done():
					return ctx.Err()
				}
				return commitTaskMultiGroupAgentChange(ctx, cfg.WorkspaceRoot, "TG-001", "captured base\n")
			},
		})
		writeIndependentTaskGroupFixture(t, env, initiative, 1)
		commitTaskMultiGitWorkspace(t, env.workspaceRoot)
		capturedBase := runGitOutput(t, env.workspaceRoot, "rev-parse", "HEAD")
		parent := startTaskMultiGroupParallelRun(
			t, env, parentID, initiative, []string{"TG-001"}, 1,
		)
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("IT-005 child did not start")
		}
		writeFileForTest(t, filepath.Join(env.workspaceRoot, "checkout-advance.txt"), "advanced\n")
		runGitOutput(t, env.workspaceRoot, "add", "--", "checkout-advance.txt")
		runGitOutput(t, env.workspaceRoot, "commit", "-q", "-m", "advance checkout during group run")
		advancedHead := runGitOutput(t, env.workspaceRoot, "rev-parse", "HEAD")
		close(release)
		waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
			return isTerminalRunStatus(row.Status)
		})
		item := requireTaskMultiGroupSnapshot(t, env, parent.RunID, 1).Items[0]
		if item.BaseCommit != capturedBase {
			t.Fatalf("IT-005 item base = %q, want captured %q", item.BaseCommit, capturedBase)
		}
		if got := runGitOutput(
			t, env.workspaceRoot, "merge-base", item.ResultBranch, advancedHead,
		); got != capturedBase {
			t.Fatalf("IT-005 merge base = %q, want %q", got, capturedBase)
		}
		if err := runGitOutputAllowFailure(
			t, env.workspaceRoot, "show", item.ResultBranch+":checkout-advance.txt",
		); err == nil {
			t.Fatal("IT-005 result branch contains checkout commit made after launch")
		}
	})

	t.Run("IT-028 one group retains internal task commits in order", func(t *testing.T) {
		const (
			initiative = "ordered-tasks"
			parentID   = "ordered-tasks-parent"
		)
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			buildRunID: taskMultiGroupRunIDBuilder(parentID),
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
				for index := 1; index <= 2; index++ {
					name := fmt.Sprintf("internal-task-%02d.txt", index)
					if err := os.WriteFile(
						filepath.Join(cfg.WorkspaceRoot, name),
						[]byte(fmt.Sprintf("task %d\n", index)),
						0o600,
					); err != nil {
						return err
					}
					if err := commitTaskMultiGroupPaths(
						ctx,
						cfg.WorkspaceRoot,
						fmt.Sprintf("internal task %02d", index),
						name,
					); err != nil {
						return err
					}
				}
				return nil
			},
		})
		writeIndependentTaskGroupFixture(t, env, initiative, 1)
		env.writeWorkflowFile(
			t,
			initiative,
			filepath.Join("_task_groups", "TG-001", "task_02.md"),
			daemonTaskBody("pending", "Second internal task"),
		)
		commitTaskMultiGitWorkspace(t, env.workspaceRoot)
		base := runGitOutput(t, env.workspaceRoot, "rev-parse", "HEAD")
		parent := startTaskMultiGroupParallelRun(
			t, env, parentID, initiative, []string{"TG-001"}, 1,
		)
		waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
			return isTerminalRunStatus(row.Status)
		})
		item := requireTaskMultiGroupSnapshot(t, env, parent.RunID, 1).Items[0]
		subjects := strings.Split(
			runGitOutput(t, env.workspaceRoot, "log", "--reverse", "--format=%s", base+".."+item.ResultBranch),
			"\n",
		)
		if want := []string{"internal task 01", "internal task 02"}; !reflect.DeepEqual(subjects, want) {
			t.Fatalf("IT-028 branch commits = %#v, want %#v", subjects, want)
		}
	})
}

func TestRunManagerTaskMultiGroupParallelDeletedWorktreeFailsCleanlyIT012(t *testing.T) {
	requireGitForTaskMulti(t)

	const (
		initiative = "deleted-worktree"
		parentID   = "deleted-worktree-parent"
	)
	targetPath := make(chan string, 1)
	release := make(chan struct{})
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		buildRunID: taskMultiGroupRunIDBuilder(parentID),
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
			groupID := taskMultiTaskGroupID(cfg.Name)
			if groupID == "TG-001" {
				targetPath <- cfg.WorkspaceRoot
				select {
				case <-release:
				case <-ctx.Done():
					return ctx.Err()
				}
				return os.WriteFile(filepath.Join(cfg.WorkspaceRoot, "after-delete.txt"), []byte("missing\n"), 0o600)
			}
			return commitTaskMultiGroupAgentChange(ctx, cfg.WorkspaceRoot, groupID, groupID+" sibling\n")
		},
	})
	writeIndependentTaskGroupFixture(t, env, initiative, 2)
	commitTaskMultiGitWorkspace(t, env.workspaceRoot)
	checkoutHead := runGitOutput(t, env.workspaceRoot, "rev-parse", "HEAD")
	parent := startTaskMultiGroupParallelRun(
		t, env, parentID, initiative, []string{"TG-001", "TG-002"}, 2,
	)
	var deletedPath string
	select {
	case deletedPath = <-targetPath:
	case <-time.After(5 * time.Second):
		t.Fatal("IT-012 target child did not start")
	}
	if err := os.RemoveAll(deletedPath); err != nil {
		t.Fatalf("IT-012 remove worktree directory: %v", err)
	}
	close(release)
	waitForRun(t, env.globalDB, parent.RunID, func(row globaldb.Run) bool {
		return isTerminalRunStatus(row.Status)
	})
	items := taskMultiItemsByGroupID(requireTaskMultiGroupSnapshot(t, env, parent.RunID, 2).Items)
	if items["TG-001"].Status != taskMultiItemStatusFailed {
		t.Fatalf("IT-012 deleted group = %#v, want failed", items["TG-001"])
	}
	if items["TG-002"].Status != taskMultiItemStatusCompleted {
		t.Fatalf("IT-012 sibling = %#v, want completed", items["TG-002"])
	}
	if got := runGitOutput(t, env.workspaceRoot, "rev-parse", "HEAD"); got != checkoutHead {
		t.Fatalf("IT-012 checkout HEAD = %q, want %q", got, checkoutHead)
	}
}

func TestTaskMultiGroupParkedSettlementIT008(t *testing.T) {
	t.Parallel()

	allocation := taskMultiWorktreeAllocation{
		Path:           "/managed/group-worktree",
		WorktreeStatus: taskMultiWorktreeStatusPreserved,
	}
	kind, status, reason := taskMultiChildSettlement(
		globaldb.Run{
			RunID:     "parked-child",
			Status:    runStatusParked,
			ErrorText: "stalled twice and parked",
		},
		allocation,
		true,
	)
	if kind != eventspkg.EventKindTaskRunMultipleChildFailed ||
		status != taskMultiItemStatusFailed ||
		!strings.Contains(reason, "parked") {
		t.Fatalf("IT-008 parked settlement = %s/%s/%q", kind, status, reason)
	}
	prepared := &preparedTaskMulti{executionKind: apicore.ExecutionKindTaskMultiGroupParallel}
	err := taskMultiChildTerminalError(
		globaldb.Run{
			RunID:     "parked-child",
			Status:    runStatusParked,
			ErrorText: reason,
		},
		"initiative/TG-001",
		prepared,
		allocation,
	)
	if err == nil || !strings.Contains(err.Error(), "worktree preserved at /managed/group-worktree") {
		t.Fatalf("IT-008 terminal error = %v, want preserved worktree", err)
	}
}

// TestRunManagerTaskMultiGroupParallelRelaunchRecovery exercises the
// US-001.EC-6 / ADR-008 re-launch recovery decision path end-to-end through
// StartTaskRunMultiple: the selection-fingerprint gate, the completed/terminal
// relaunch problems, and the --new bypass. E2E-012 is the CLI wrapper over these
// same daemon behaviors (re-attach while active, refuse + --new after completion)
// and is covered at the CLI layer outside this daemon file.
func TestRunManagerTaskMultiGroupParallelRelaunchRecovery(t *testing.T) {
	requireGitForTaskMulti(t)

	t.Run("IT-022 active selection re-attaches without a second launch", func(t *testing.T) {
		const (
			initiative = "relaunch-active"
			parentID   = "relaunch-active-parent"
		)
		var executed atomic.Int32
		started := make(chan string, 2)
		release := make(chan struct{})
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			buildRunID: taskMultiGroupRunIDBuilder(parentID),
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
				executed.Add(1)
				started <- taskMultiTaskGroupID(cfg.Name)
				select {
				case <-release:
				case <-ctx.Done():
					return ctx.Err()
				}
				groupID := taskMultiTaskGroupID(cfg.Name)
				return commitTaskMultiGroupAgentChange(ctx, cfg.WorkspaceRoot, groupID, groupID+" active\n")
			},
		})
		writeIndependentTaskGroupFixture(t, env, initiative, 2)
		commitTaskMultiGitWorkspace(t, env.workspaceRoot)

		first := startTaskMultiGroupParallelRun(
			t, env, parentID, initiative, []string{"TG-001", "TG-002"}, 2,
		)
		// Both children are actively running (blocked below), so the parent run is
		// not settled when the equivalent selection is re-issued.
		waitForTaskMultiGroupStarts(t, started, 2)

		existing, err := attemptTaskMultiGroupParallelRun(
			t, env, parentID+"-again", initiative, []string{"TG-001", "TG-002"}, 2, false,
		)
		if err != nil {
			t.Fatalf("IT-022 re-attach error = %v, want existing run", err)
		}
		if existing.RunID != first.RunID {
			t.Fatalf("IT-022 re-attach run = %q, want existing %q", existing.RunID, first.RunID)
		}

		close(release)
		row := waitForRun(t, env.globalDB, first.RunID, func(row globaldb.Run) bool {
			return isTerminalRunStatus(row.Status)
		})
		if row.Status != runStatusCompleted {
			t.Fatalf("IT-022 parent status = %q error=%q, want completed", row.Status, row.ErrorText)
		}
		if got := executed.Load(); got != 2 {
			t.Fatalf("IT-022 executed children = %d, want 2 (no second launch)", got)
		}
		requireTaskMultiGroupSnapshot(t, env, first.RunID, 2)
	})

	t.Run("IT-029 completion projection preserves active selection re-attachment", func(t *testing.T) {
		const (
			initiative = "relaunch-hydrated-peer"
			parentID   = "relaunch-hydrated-peer-parent"
		)
		started := make(chan string, 2)
		release := make(chan struct{})
		t.Cleanup(func() {
			select {
			case <-release:
			default:
				close(release)
			}
		})
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			buildRunID: taskMultiGroupRunIDBuilder(parentID),
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
				started <- taskMultiTaskGroupID(cfg.Name)
				select {
				case <-release:
				case <-ctx.Done():
					return ctx.Err()
				}
				groupID := taskMultiTaskGroupID(cfg.Name)
				return commitTaskMultiGroupAgentChange(ctx, cfg.WorkspaceRoot, groupID, groupID+" hydrated peer\n")
			},
		})
		writeIndependentTaskGroupFixture(t, env, initiative, 3)
		commitTaskMultiGitWorkspace(t, env.workspaceRoot)

		first := startTaskMultiGroupParallelRun(
			t, env, parentID, initiative, []string{"TG-001", "TG-002"}, 2,
		)
		waitForTaskMultiGroupStarts(t, started, 2)

		initiativeDir := filepath.Join(env.workspaceRoot, ".compozy", "tasks", initiative)
		completed, err := taskgroups.NewStore().MarkComplete(
			context.Background(),
			initiativeDir,
			"TG-003",
		)
		if err != nil {
			t.Fatalf("IT-029 MarkComplete(TG-003) error = %v", err)
		}
		if !completed.CompletionRecorded {
			t.Fatalf("IT-029 completion result = %#v, want TG-003 projection recorded", completed)
		}

		existing, err := attemptTaskMultiGroupParallelRun(
			t, env, parentID+"-again", initiative, []string{"TG-001", "TG-002"}, 2, false,
		)
		if err != nil {
			t.Fatalf("IT-029 re-attach error = %v, want existing run", err)
		}
		if existing.RunID != first.RunID {
			t.Fatalf("IT-029 re-attach run = %q, want existing %q", existing.RunID, first.RunID)
		}

		close(release)
		row := waitForRun(t, env.globalDB, first.RunID, func(row globaldb.Run) bool {
			return isTerminalRunStatus(row.Status)
		})
		if row.Status != runStatusCompleted {
			t.Fatalf("IT-029 parent status = %q error=%q, want completed", row.Status, row.ErrorText)
		}
	})

	t.Run("IT-023/IT-026 completed selection refuses re-attach; --new starts fresh", func(t *testing.T) {
		const (
			initiative = "relaunch-done"
			parentID   = "relaunch-done-alpha"
			freshID    = "relaunch-done-omega"
		)
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			buildRunID: taskMultiGroupRunIDBuilder(parentID),
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
				groupID := taskMultiTaskGroupID(cfg.Name)
				return commitTaskMultiGroupAgentChange(ctx, cfg.WorkspaceRoot, groupID, groupID+" done\n")
			},
		})
		writeIndependentTaskGroupFixture(t, env, initiative, 2)
		commitTaskMultiGitWorkspace(t, env.workspaceRoot)

		first := startTaskMultiGroupParallelRun(
			t, env, parentID, initiative, []string{"TG-001", "TG-002"}, 2,
		)
		if row := waitForRun(t, env.globalDB, first.RunID, func(row globaldb.Run) bool {
			return isTerminalRunStatus(row.Status)
		}); row.Status != runStatusCompleted {
			t.Fatalf("IT-023 first parent status = %q error=%q, want completed", row.Status, row.ErrorText)
		}
		firstItems := taskMultiItemsByGroupID(requireTaskMultiGroupSnapshot(t, env, first.RunID, 2).Items)
		firstBranches := make(map[string]string, 2)
		firstSHAs := make(map[string]string, 2)
		for _, groupID := range []string{"TG-001", "TG-002"} {
			branch := strings.TrimSpace(firstItems[groupID].ResultBranch)
			if branch == "" {
				t.Fatalf("IT-023 %s completed without a branch: %#v", groupID, firstItems[groupID])
			}
			firstBranches[groupID] = branch
			firstSHAs[branch] = runGitOutput(t, env.workspaceRoot, "rev-parse", branch)
		}

		// IT-023: re-issue without --new is refused with the completed record.
		_, err := attemptTaskMultiGroupParallelRun(
			t, env, parentID+"-dup", initiative, []string{"TG-001", "TG-002"}, 2, false,
		)
		var problem *apicore.Problem
		if !errors.As(err, &problem) {
			t.Fatalf("IT-023 re-issue error = %v, want API problem", err)
		}
		if problem.Status != http.StatusConflict ||
			problem.Code != "parallel_task_groups_selection_completed" {
			t.Fatalf("IT-023 problem = %#v, want 409 parallel_task_groups_selection_completed", problem)
		}
		if required, ok := problem.Details["new_required"].(bool); !ok || !required {
			t.Fatalf("IT-023 details = %#v, want new_required=true", problem.Details)
		}
		if branches, ok := problem.Details["result_branches"].([]string); !ok || len(branches) == 0 {
			t.Fatalf("IT-023 details = %#v, want reported result_branches", problem.Details)
		}

		// IT-026: --new mints a fresh run and namespace without touching prior branches.
		fresh, err := attemptTaskMultiGroupParallelRun(
			t, env, freshID, initiative, []string{"TG-001", "TG-002"}, 2, true,
		)
		if err != nil {
			t.Fatalf("IT-026 --new launch error = %v", err)
		}
		if fresh.RunID == first.RunID {
			t.Fatalf("IT-026 --new run = %q, want a fresh run id", fresh.RunID)
		}
		if row := waitForRun(t, env.globalDB, fresh.RunID, func(row globaldb.Run) bool {
			return isTerminalRunStatus(row.Status)
		}); row.Status != runStatusCompleted {
			t.Fatalf("IT-026 fresh parent status = %q error=%q, want completed", row.Status, row.ErrorText)
		}
		freshItems := taskMultiItemsByGroupID(requireTaskMultiGroupSnapshot(t, env, fresh.RunID, 2).Items)
		for _, groupID := range []string{"TG-001", "TG-002"} {
			freshBranch := strings.TrimSpace(freshItems[groupID].ResultBranch)
			if freshBranch == "" {
				t.Fatalf("IT-026 %s fresh branch missing: %#v", groupID, freshItems[groupID])
			}
			if freshBranch == firstBranches[groupID] {
				t.Fatalf("IT-026 %s reused prior branch %q, want fresh namespace", groupID, freshBranch)
			}
			if got := runGitOutput(
				t, env.workspaceRoot, "rev-parse", firstBranches[groupID],
			); got != firstSHAs[firstBranches[groupID]] {
				t.Fatalf("IT-026 prior branch %q moved to %q", firstBranches[groupID], got)
			}
		}
	})

	t.Run("IT-024 partial-terminal selection reports the terminal record", func(t *testing.T) {
		const (
			initiative = "relaunch-partial"
			parentID   = "relaunch-partial-parent"
		)
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			buildRunID: taskMultiGroupRunIDBuilder(parentID),
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
				groupID := taskMultiTaskGroupID(cfg.Name)
				if groupID == "TG-002" {
					return errors.New("simulated TG-002 relaunch failure")
				}
				return commitTaskMultiGroupAgentChange(ctx, cfg.WorkspaceRoot, groupID, groupID+" ok\n")
			},
		})
		writeIndependentTaskGroupFixture(t, env, initiative, 3)
		commitTaskMultiGitWorkspace(t, env.workspaceRoot)

		first := startTaskMultiGroupParallelRun(
			t, env, parentID, initiative, []string{"TG-001", "TG-002", "TG-003"}, 3,
		)
		if row := waitForRun(t, env.globalDB, first.RunID, func(row globaldb.Run) bool {
			return isTerminalRunStatus(row.Status)
		}); row.Status != runStatusFailed {
			t.Fatalf("IT-024 first parent status = %q, want failed (partial)", row.Status)
		}

		_, err := attemptTaskMultiGroupParallelRun(
			t, env, parentID+"-dup", initiative, []string{"TG-001", "TG-002", "TG-003"}, 3, false,
		)
		var problem *apicore.Problem
		if !errors.As(err, &problem) {
			t.Fatalf("IT-024 re-issue error = %v, want API problem", err)
		}
		if problem.Status != http.StatusConflict ||
			problem.Code != "parallel_task_groups_selection_terminal" {
			t.Fatalf("IT-024 problem = %#v, want 409 parallel_task_groups_selection_terminal", problem)
		}
		if failed, ok := problem.Details["failed"].([]string); !ok || !containsStringFragment(failed, "TG-002") {
			t.Fatalf("IT-024 failed detail = %#v, want TG-002", problem.Details["failed"])
		}
		if succeeded, ok := problem.Details["succeeded"].([]string); !ok ||
			!containsStringFragment(succeeded, "TG-001") {
			t.Fatalf("IT-024 succeeded detail = %#v, want TG-001", problem.Details["succeeded"])
		}
		if preserved, ok := problem.Details["preserved_paths"].([]string); !ok ||
			!containsStringFragment(preserved, "TG-002") {
			t.Fatalf("IT-024 preserved_paths detail = %#v, want TG-002", problem.Details["preserved_paths"])
		}
	})

	t.Run("IT-025 plan drift rejects the re-launch on the launch path", func(t *testing.T) {
		const initiative = "relaunch-drift"
		env := newRunManagerTestEnv(t, runManagerTestDeps{
			buildRunID: taskMultiGroupRunIDBuilder(initiative + "-parent"),
			prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
				return &model.SolvePreparation{}, nil
			},
			execute: func(context.Context, *model.SolvePreparation, *model.RuntimeConfig) error {
				return errors.New("IT-025 must reject before executing any child")
			},
		})
		writeIndependentTaskGroupFixture(t, env, initiative, 3)
		// Plan drift: TG-002 now depends on the unselected, incomplete TG-003, so the
		// {TG-001, TG-002} selection is no longer a mutually independent runnable set.
		writeTaskGroupPlanFile(
			t,
			env,
			initiative,
			[]taskgroups.TaskGroup{
				independentTaskGroupSpec("TG-001"),
				independentTaskGroupSpec("TG-002"),
				independentTaskGroupSpec("TG-003"),
			},
			[]taskgroups.Dependency{{
				From:      "TG-003",
				To:        "TG-002",
				Rationale: "TG-002 now consumes TG-003 output after the plan changed",
			}},
		)
		commitTaskMultiGitWorkspace(t, env.workspaceRoot)

		_, err := attemptTaskMultiGroupParallelRun(
			t, env, initiative+"-parent", initiative, []string{"TG-001", "TG-002"}, 2, false,
		)
		var problem *apicore.Problem
		if !errors.As(err, &problem) {
			t.Fatalf("IT-025 launch error = %v, want API problem", err)
		}
		if problem.Status != http.StatusConflict || problem.Code != "task_group_dependencies_unmet" {
			t.Fatalf("IT-025 problem = %#v, want 409 task_group_dependencies_unmet", problem)
		}
		rejected, ok := problem.Details["rejected"].(map[string]any)
		if !ok {
			t.Fatalf("IT-025 details = %#v, want rejected map", problem.Details)
		}
		if _, found := rejected["TG-002"]; !found {
			t.Fatalf("IT-025 rejected = %#v, want TG-002 rejected for the added dependency", rejected)
		}
	})
}

// TestRunManagerTaskMultiGroupParallelParkedSelectionReAttaches guards Issue 006:
// a parked group-parallel run is a resumable stall (globaldb's active-run predicate
// treats parked as active), so the relaunch gate must re-attach to it rather than
// route it through the terminal-report / --new path.
func TestRunManagerTaskMultiGroupParallelParkedSelectionReAttaches(t *testing.T) {
	requireGitForTaskMulti(t)

	const (
		initiative = "relaunch-parked"
		parentID   = "relaunch-parked-parent"
	)
	env := newRunManagerTestEnv(t, runManagerTestDeps{
		buildRunID: taskMultiGroupRunIDBuilder(parentID),
		prepare: func(context.Context, *model.RuntimeConfig, model.RunScope) (*model.SolvePreparation, error) {
			return &model.SolvePreparation{}, nil
		},
		execute: func(ctx context.Context, _ *model.SolvePreparation, cfg *model.RuntimeConfig) error {
			groupID := taskMultiTaskGroupID(cfg.Name)
			return commitTaskMultiGroupAgentChange(ctx, cfg.WorkspaceRoot, groupID, groupID+" parked\n")
		},
	})
	writeIndependentTaskGroupFixture(t, env, initiative, 2)
	commitTaskMultiGitWorkspace(t, env.workspaceRoot)

	first := startTaskMultiGroupParallelRun(
		t, env, parentID, initiative, []string{"TG-001", "TG-002"}, 2,
	)
	waitForRun(t, env.globalDB, first.RunID, func(row globaldb.Run) bool {
		return isTerminalRunStatus(row.Status)
	})

	// Drive the persisted parent into the resumable parked state.
	ctx := context.Background()
	row, err := env.globalDB.GetRun(ctx, first.RunID)
	if err != nil {
		t.Fatalf("GetRun(%q) error = %v", first.RunID, err)
	}
	if strings.TrimSpace(row.SelectionFingerprint) == "" {
		t.Fatalf("parked run is missing its selection fingerprint: %#v", row)
	}
	row.Status = runStatusParked
	if _, err := env.globalDB.UpdateRun(ctx, row); err != nil {
		t.Fatalf("UpdateRun(parked) error = %v", err)
	}

	// Re-issuing the same selection must re-attach to the parked run, not refuse it.
	existing, err := attemptTaskMultiGroupParallelRun(
		t, env, parentID+"-reattach", initiative, []string{"TG-001", "TG-002"}, 2, false,
	)
	if err != nil {
		var problem *apicore.Problem
		if errors.As(err, &problem) {
			t.Fatalf("parked re-issue returned terminal problem %q, want re-attach", problem.Code)
		}
		t.Fatalf("parked re-issue error = %v, want re-attach", err)
	}
	if existing.RunID != first.RunID {
		t.Fatalf("parked re-issue run = %q, want re-attach to %q", existing.RunID, first.RunID)
	}
}

func writeIndependentTaskGroupFixture(
	t *testing.T,
	env *runManagerTestEnv,
	initiative string,
	count int,
) {
	t.Helper()
	groups := make([]taskgroups.TaskGroup, 0, count)
	for index := 1; index <= count; index++ {
		groupID := fmt.Sprintf("TG-%03d", index)
		groups = append(groups, taskgroups.TaskGroup{
			ID:         groupID,
			Title:      "Parallel group " + groupID,
			Outcome:    "Produce isolated output for " + groupID,
			Directory:  "_task_groups/" + groupID,
			OwnedScope: []string{strings.ToLower(groupID) + ".txt"},
		})
		env.writeWorkflowFile(
			t,
			initiative,
			filepath.Join("_task_groups", groupID, "task_01.md"),
			daemonTaskBody("pending", "Execute "+groupID),
		)
	}
	plan, err := taskgroups.RenderPlan(taskgroups.Plan{
		SchemaVersion: taskgroups.SchemaVersion,
		Initiative:    initiative,
		TaskGroups:    groups,
	})
	if err != nil {
		t.Fatalf("RenderPlan() error = %v", err)
	}
	env.writeWorkflowFile(t, initiative, "_prd.md", "# Parallel groups\n")
	env.writeWorkflowFile(t, initiative, "_techspec.md", "# Parallel groups techspec\n")
	env.writeWorkflowFile(t, initiative, "_task_groups.md", string(plan))
	writeCompozyTasksGitignore(t, env.workspaceRoot)
}

// independentTaskGroupSpec mirrors the group shape written by
// writeIndependentTaskGroupFixture so callers can rewrite the plan with a single
// group mutated (e.g. an added dependency) while keeping the rest identical.
func independentTaskGroupSpec(groupID string) taskgroups.TaskGroup {
	return taskgroups.TaskGroup{
		ID:         groupID,
		Title:      "Parallel group " + groupID,
		Outcome:    "Produce isolated output for " + groupID,
		Directory:  "_task_groups/" + groupID,
		OwnedScope: []string{strings.ToLower(groupID) + ".txt"},
	}
}

// writeTaskGroupPlanFile overwrites _task_groups.md with an explicit group set and
// dependency graph, used to simulate plan drift between an initial launch and a
// later re-launch. Dependency edges are rendered from Plan.Edges (each edge needs
// a rationale), not from TaskGroup.Dependencies.
func writeTaskGroupPlanFile(
	t *testing.T,
	env *runManagerTestEnv,
	initiative string,
	groups []taskgroups.TaskGroup,
	edges []taskgroups.Dependency,
) {
	t.Helper()
	plan, err := taskgroups.RenderPlan(taskgroups.Plan{
		SchemaVersion: taskgroups.SchemaVersion,
		Initiative:    initiative,
		TaskGroups:    groups,
		Edges:         edges,
	})
	if err != nil {
		t.Fatalf("RenderPlan() error = %v", err)
	}
	env.writeWorkflowFile(t, initiative, "_task_groups.md", string(plan))
}

func taskMultiGroupRunIDBuilder(parentRunID string) func(*model.RuntimeConfig) (string, error) {
	return func(cfg *model.RuntimeConfig) (string, error) {
		if cfg == nil {
			return "", errors.New("runtime config is required")
		}
		if runID := strings.TrimSpace(cfg.RunID); runID != "" {
			return runID, nil
		}
		if cfg.ParentRunID == parentRunID {
			return "child-" + strings.ReplaceAll(strings.TrimSpace(cfg.Name), "/", "-"), nil
		}
		return "generated-" + strings.ReplaceAll(strings.TrimSpace(cfg.Name), "/", "-"), nil
	}
}

// taskMultiGroupRequest builds a parallel task-group launch request. newRun sets
// the --new bypass so the relaunch gate is skipped and a fresh namespace is minted.
func taskMultiGroupRequest(
	t *testing.T,
	env *runManagerTestEnv,
	runID string,
	initiative string,
	groupIDs []string,
	limit int,
	newRun bool,
) apicore.TaskRunMultipleRequest {
	t.Helper()
	targets := make([]apicore.TaskRunTarget, 0, len(groupIDs))
	for _, groupID := range groupIDs {
		targets = append(targets, apicore.TaskRunTarget{
			InitiativeSlug: initiative,
			TaskGroupID:    groupID,
		})
	}
	return apicore.TaskRunMultipleRequest{
		Workspace:        env.workspaceRoot,
		Targets:          targets,
		Mode:             workspacecfg.TaskRunMultipleModeParallel,
		ParallelLimit:    limit,
		PresentationMode: defaultPresentationMode,
		NewRun:           newRun,
		RuntimeOverrides: rawJSON(t, fmt.Sprintf(`{"run_id":%q}`, runID)),
		Execution: &apicore.TaskExecutionDescriptor{
			Kind:          apicore.ExecutionKindTaskMultiGroupParallel,
			Label:         "Parallel task groups",
			UsesWorktrees: true,
			Source:        "test",
		},
	}
}

// attemptTaskMultiGroupParallelRun launches a selection and returns the raw
// result so callers can assert re-attach runs and relaunch-gate problems.
func attemptTaskMultiGroupParallelRun(
	t *testing.T,
	env *runManagerTestEnv,
	runID string,
	initiative string,
	groupIDs []string,
	limit int,
	newRun bool,
) (apicore.Run, error) {
	t.Helper()
	return env.manager.StartTaskRunMultiple(
		context.Background(),
		env.workspaceRoot,
		taskMultiGroupRequest(t, env, runID, initiative, groupIDs, limit, newRun),
	)
}

func startTaskMultiGroupParallelRun(
	t *testing.T,
	env *runManagerTestEnv,
	runID string,
	initiative string,
	groupIDs []string,
	limit int,
) apicore.Run {
	t.Helper()
	run, err := env.manager.StartTaskRunMultiple(
		context.Background(),
		env.workspaceRoot,
		taskMultiGroupRequest(t, env, runID, initiative, groupIDs, limit, false),
	)
	if err != nil {
		t.Fatalf("StartTaskRunMultiple(task groups %v) error = %v", groupIDs, err)
	}
	return run
}

func commitTaskMultiGroupAgentChange(
	ctx context.Context,
	worktreeRoot string,
	groupID string,
	content string,
) error {
	name := strings.ToLower(groupID) + ".txt"
	if err := os.WriteFile(filepath.Join(worktreeRoot, name), []byte(content), 0o600); err != nil {
		return fmt.Errorf("write %s output: %w", groupID, err)
	}
	return commitTaskMultiGroupPaths(ctx, worktreeRoot, groupID+" agent commit", name)
}

func commitTaskMultiGroupPaths(
	ctx context.Context,
	worktreeRoot string,
	subject string,
	paths ...string,
) error {
	for _, args := range [][]string{
		append([]string{"add", "--"}, paths...),
		{
			"-c", "user.name=Task Group Agent",
			"-c", "user.email=agent@example.com",
			"commit", "--no-verify", "-m", subject,
		},
	} {
		if _, err := runTaskMultiWorktreeGitCommand(ctx, worktreeRoot, args...); err != nil {
			return fmt.Errorf("git %v: %w", args, err)
		}
	}
	return nil
}

func taskMultiPromptTaskPath(promptText string) (string, error) {
	const prefix = "- Task file: `"
	for line := range strings.SplitSeq(promptText, "\n") {
		if !strings.HasPrefix(line, prefix) || !strings.HasSuffix(line, "`") {
			continue
		}
		path := strings.TrimSuffix(strings.TrimPrefix(line, prefix), "`")
		if strings.TrimSpace(path) == "" {
			break
		}
		return path, nil
	}
	return "", errors.New("prepared prompt did not identify an exact task file")
}

func readTaskMultiGroupArtifactFiles(
	t *testing.T,
	root string,
	relativePaths []string,
) map[string][]byte {
	t.Helper()
	artifacts := make(map[string][]byte, len(relativePaths))
	for _, relativePath := range relativePaths {
		data, err := os.ReadFile(filepath.Join(root, relativePath))
		if err != nil {
			t.Fatalf("read canonical task-group artifact %s: %v", relativePath, err)
		}
		artifacts[relativePath] = data
	}
	return artifacts
}

func requireTaskMultiGroupSnapshot(
	t *testing.T,
	env *runManagerTestEnv,
	runID string,
	wantItems int,
) apicore.TaskRunMultipleSnapshot {
	t.Helper()
	snapshot, err := env.manager.RunMultipleSnapshot(context.Background(), runID)
	if err != nil {
		t.Fatalf("RunMultipleSnapshot(%q) error = %v", runID, err)
	}
	if len(snapshot.Items) != wantItems {
		t.Fatalf("snapshot items = %d, want %d: %#v", len(snapshot.Items), wantItems, snapshot.Items)
	}
	return snapshot
}

func taskMultiItemsByGroupID(items []apicore.TaskRunMultipleItem) map[string]apicore.TaskRunMultipleItem {
	result := make(map[string]apicore.TaskRunMultipleItem, len(items))
	for index := range items {
		item := items[index]
		result[taskMultiTaskGroupID(item.Slug)] = item
	}
	return result
}

func containsStringFragment(values []string, fragment string) bool {
	for _, value := range values {
		if strings.Contains(value, fragment) {
			return true
		}
	}
	return false
}

func waitForTaskMultiGroupStarts(t *testing.T, started <-chan string, count int) {
	t.Helper()
	for index := 0; index < count; index++ {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out after %d of %d task-group starts", index, count)
		}
	}
}

func assertTaskMultiGroupEventsCarryID(
	t *testing.T,
	manager *RunManager,
	runID string,
	groupID string,
) {
	t.Helper()
	seen := false
	for _, event := range allRunEvents(t, manager, runID) {
		switch event.Kind {
		case eventspkg.EventKindTaskRunMultipleItemQueued,
			eventspkg.EventKindTaskRunMultipleChildStarted,
			eventspkg.EventKindTaskRunMultipleChildCompleted,
			eventspkg.EventKindTaskRunMultipleChildFailed,
			eventspkg.EventKindTaskRunMultipleItemCanceled:
			payload, err := decodeTaskMultiPayload(event)
			if err != nil {
				t.Fatalf("decode %s: %v", event.Kind, err)
			}
			if payload.TaskGroupID == groupID {
				seen = true
			}
		}
	}
	if !seen {
		t.Fatalf("no task.multi item event carried task_group_id=%s", groupID)
	}
}

func runGitOutputAllowFailure(t *testing.T, dir string, args ...string) error {
	t.Helper()
	_, err := runTaskMultiWorktreeGitCommand(context.Background(), dir, args...)
	return err
}

type taskMultiGroupConcurrencyObserver struct {
	current atomic.Int32
	peak    atomic.Int32
}

func (o *taskMultiGroupConcurrencyObserver) enter() {
	current := o.current.Add(1)
	for {
		peak := o.peak.Load()
		if current <= peak || o.peak.CompareAndSwap(peak, current) {
			return
		}
	}
}

func (o *taskMultiGroupConcurrencyObserver) leave() {
	o.current.Add(-1)
}

func (o *taskMultiGroupConcurrencyObserver) waitForPeak(t *testing.T, want int32) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if o.peak.Load() >= want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("peak concurrency = %d, want at least %d", o.peak.Load(), want)
}
