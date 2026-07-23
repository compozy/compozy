package daemon

import (
	"context"
	"errors"
	"fmt"
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

func startTaskMultiGroupParallelRun(
	t *testing.T,
	env *runManagerTestEnv,
	runID string,
	initiative string,
	groupIDs []string,
	limit int,
) apicore.Run {
	t.Helper()
	targets := make([]apicore.TaskRunTarget, 0, len(groupIDs))
	for _, groupID := range groupIDs {
		targets = append(targets, apicore.TaskRunTarget{
			InitiativeSlug: initiative,
			TaskGroupID:    groupID,
		})
	}
	request := apicore.TaskRunMultipleRequest{
		Workspace:        env.workspaceRoot,
		Targets:          targets,
		Mode:             workspacecfg.TaskRunMultipleModeParallel,
		ParallelLimit:    limit,
		PresentationMode: defaultPresentationMode,
		RuntimeOverrides: rawJSON(t, fmt.Sprintf(`{"run_id":%q}`, runID)),
		Execution: &apicore.TaskExecutionDescriptor{
			Kind:          apicore.ExecutionKindTaskMultiGroupParallel,
			Label:         "Parallel task groups",
			UsesWorktrees: true,
			Source:        "test",
		},
	}
	run, err := env.manager.StartTaskRunMultiple(context.Background(), env.workspaceRoot, request)
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
