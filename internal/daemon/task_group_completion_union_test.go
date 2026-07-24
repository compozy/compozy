package daemon

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	apicore "github.com/compozy/compozy/internal/api/core"
	corepkg "github.com/compozy/compozy/internal/core"
)

// Suite: daemon task-group dispatch readiness across sibling git worktrees
// (completion-worktree-union feature, task_03).
// Invariant: the daemon write fan-out keeps its home-owned ownership filter after
// the gitenv.ParseWorktreeList refactor, while the preflight readiness inherits the
// unioned read so a prerequisite finished in a sibling no longer spuriously blocks.
// Boundary IN: RunManager hydration/preflight hooks, real SQLite, filesystem, and
// the real Git worktree registry.
// Boundary OUT: CLI rendering and daemon execution lifecycle.
//
// Note: the feature's IT IDs (IT-032, IT-034..IT-037) collide numerically with the
// pre-existing daemon completion-hydration suite in
// task_group_completion_hydration_test.go, whose subtests carry their own IT-032
// label for a different behavior. These are the completion-worktree-union
// definitions from that feature's _tests.md.

// wireUnionHydration points the RunManager's plan-completion hook at the real
// unioned read so the preflight projects sibling completions into the querying
// worktree's plan exactly as production does.
func wireUnionHydration(env *runManagerTestEnv) {
	env.manager.hydratePlanCompletion = func(
		ctx context.Context,
		workspaceRoot, initiative string,
	) ([]string, error) {
		return corepkg.HydratePlanCompletionWithDB(ctx, env.globalDB, workspaceRoot, initiative)
	}
}

// TestTaskGroupHydrationRootsOwnershipFilterAfterRefactor is IT-032: after routing
// the porcelain parse through gitenv.ParseWorktreeList, the daemon write fan-out
// still returns only the canonical root plus home-owned worktrees; a non-owned user
// checkout is excluded (ADR-002 asymmetry preserved; ADR-004 regression guard).
func TestTaskGroupHydrationRootsOwnershipFilterAfterRefactor(t *testing.T) {
	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	initializeHydrationGitRepository(t, env.workspaceRoot)

	ownedRoot := filepath.Join(env.paths.WorktreesDir, "repo", "tg-owned")
	runGitOutput(t, env.workspaceRoot, "worktree", "add", "--detach", ownedRoot, "HEAD")
	userRoot := filepath.Join(t.TempDir(), "user-checkout")
	runGitOutput(t, env.workspaceRoot, "worktree", "add", "--detach", userRoot, "HEAD")

	roots, err := env.manager.taskGroupHydrationRoots(context.Background(), env.workspaceRoot)
	if err != nil {
		t.Fatalf("taskGroupHydrationRoots() error = %v", err)
	}
	if len(roots) != 2 {
		t.Fatalf("hydration roots = %v, want canonical + one owned worktree", roots)
	}
	if got, want := filepath.Clean(roots[0]), filepath.Clean(env.workspaceRoot); got != want {
		t.Fatalf("hydration roots[0] = %q, want canonical %q", got, want)
	}
	resolve := func(path string) string {
		resolved, resolveErr := filepath.EvalSymlinks(path)
		if resolveErr != nil {
			t.Fatalf("EvalSymlinks(%s): %v", path, resolveErr)
		}
		return resolved
	}
	if got, want := resolve(roots[1]), resolve(ownedRoot); got != want {
		t.Fatalf("owned worktree = %q, want %q", got, want)
	}
	for _, root := range roots {
		if resolve(root) == resolve(userRoot) {
			t.Fatalf("non-owned user worktree %q leaked into hydration roots %v", userRoot, roots)
		}
	}
}

// TestRunManagerTaskGroupPreflightUnionSinglePath is IT-034: the single preflight
// path (resolveTaskGroupPreflightEvidence) counts a prerequisite completed in a
// sibling worktree toward readiness, so the dependent group is Eligible and is not
// rejected with task_group_dependencies_unmet. (US-007.AC-1.)
func TestRunManagerTaskGroupPreflightUnionSinglePath(t *testing.T) {
	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	initializeHydrationGitRepository(t, env.workspaceRoot)
	writeHydrationSimpleDependencyPlan(t, env.workspaceRoot, "initiative")
	wireUnionHydration(env)

	sibling := filepath.Join(t.TempDir(), "sibling-b")
	runGitOutput(t, env.workspaceRoot, "worktree", "add", "--detach", sibling, "HEAD")
	writeHydrationSimpleDependencyPlan(t, sibling, "initiative")
	seedDaemonCompletionRows(t, env.globalDB, sibling, "initiative", []string{"TG-001"})

	evidence, err := env.manager.resolveTaskGroupPreflightEvidence(
		context.Background(),
		env.workspaceRoot,
		"initiative/TG-002",
	)
	if err != nil {
		t.Fatalf("resolveTaskGroupPreflightEvidence() error = %v", err)
	}
	if !evidence.readiness.Eligible {
		t.Fatalf("sibling-completed prerequisite readiness = %#v, want eligible", evidence.readiness)
	}
	if _, decisionErr := taskGroupPreflightDecision(evidence, false, nil); decisionErr != nil {
		t.Fatalf("preflight rejected a sibling-satisfied dependency: %v", decisionErr)
	}
	// The union projected TG-001 into the querying worktree's own plan.
	assertDaemonHydrationPlanState(t, env.workspaceRoot, "initiative", []string{"TG-001"})
}

// TestRunManagerTaskGroupPreflightUnionParallelPath is IT-035: the parallel start
// path (prepareTaskMultiGroupLaunch → hydrateTaskGroupPlanBestEffort before
// ValidateIndependentSet) includes sibling-completed prerequisites in the readiness
// set, so the dependent group validates as eligible. Discriminating: without the
// union, TG-002's dependency on TG-001 is unmet and the launch is rejected.
// (US-007.AC-2.)
func TestRunManagerTaskGroupPreflightUnionParallelPath(t *testing.T) {
	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	initializeHydrationGitRepository(t, env.workspaceRoot)
	writeHydrationSimpleDependencyPlan(t, env.workspaceRoot, "initiative")
	wireUnionHydration(env)

	sibling := filepath.Join(t.TempDir(), "sibling-b")
	runGitOutput(t, env.workspaceRoot, "worktree", "add", "--detach", sibling, "HEAD")
	writeHydrationSimpleDependencyPlan(t, sibling, "initiative")
	seedDaemonCompletionRows(t, env.globalDB, sibling, "initiative", []string{"TG-001"})

	launch, err := env.manager.prepareTaskMultiGroupLaunch(
		context.Background(),
		env.workspaceRoot,
		[]string{"initiative/TG-002"},
	)
	if err != nil {
		t.Fatalf("prepareTaskMultiGroupLaunch() error = %v", err)
	}
	if launch == nil || len(launch.groups) != 1 || launch.groups[0].ID != "TG-002" {
		t.Fatalf("prepared launch = %#v, want a single eligible TG-002", launch)
	}
	assertDaemonHydrationPlanState(t, env.workspaceRoot, "initiative", []string{"TG-001"})
}

// TestRunManagerTaskGroupPreflightUnionEnumerationFallback is IT-036: a non-git
// workspace makes sibling enumeration fail; the preflight degrades to the
// single-workspace completion set with no error, exactly as tasks sync does.
// (US-007.EC-2.)
func TestRunManagerTaskGroupPreflightUnionEnumerationFallback(t *testing.T) {
	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	// Intentionally NOT a git repository, so sibling enumeration errors.
	writeHydrationSimpleDependencyPlan(t, env.workspaceRoot, "initiative")
	wireUnionHydration(env)
	// The prerequisite is completed in the querying workspace's own rows.
	seedDaemonCompletionRows(t, env.globalDB, env.workspaceRoot, "initiative", []string{"TG-001"})

	evidence, err := env.manager.resolveTaskGroupPreflightEvidence(
		context.Background(),
		env.workspaceRoot,
		"initiative/TG-002",
	)
	if err != nil {
		t.Fatalf("resolveTaskGroupPreflightEvidence() on non-git workspace error = %v", err)
	}
	if !evidence.readiness.Eligible {
		t.Fatalf("single-workspace fallback readiness = %#v, want eligible", evidence.readiness)
	}
	assertDaemonHydrationPlanState(t, env.workspaceRoot, "initiative", []string{"TG-001"})
}

// TestRunManagerTaskGroupPreflightUnmetDependencyOverride is IT-037: a prerequisite
// completed in no worktree still blocks with task_group_dependencies_unmet; setting
// allowOutOfOrder authorizes the run — no readiness decision is terminal (ADR-003).
// (US-007.EC-1, US-008.AC-2.)
func TestRunManagerTaskGroupPreflightUnmetDependencyOverride(t *testing.T) {
	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	initializeHydrationGitRepository(t, env.workspaceRoot)
	writeHydrationSimpleDependencyPlan(t, env.workspaceRoot, "initiative")
	wireUnionHydration(env)
	// Register the workspace with no completed task groups anywhere.
	seedDaemonCompletionRows(t, env.globalDB, env.workspaceRoot, "initiative", nil)

	evidence, err := env.manager.resolveTaskGroupPreflightEvidence(
		context.Background(),
		env.workspaceRoot,
		"initiative/TG-002",
	)
	if err != nil {
		t.Fatalf("resolveTaskGroupPreflightEvidence() error = %v", err)
	}
	if evidence.readiness.Eligible {
		t.Fatal("readiness eligible without any TG-001 completion")
	}
	_, blockErr := taskGroupPreflightDecision(evidence, false, nil)
	var problem *apicore.Problem
	if !errors.As(blockErr, &problem) || problem.Code != "task_group_dependencies_unmet" {
		t.Fatalf("preflight decision = %#v (%v), want task_group_dependencies_unmet", problem, blockErr)
	}
	authorized, err := taskGroupPreflightDecision(evidence, true, nil)
	if err != nil {
		t.Fatalf("allowOutOfOrder decision error = %v", err)
	}
	if !authorized {
		t.Fatal("allowOutOfOrder did not authorize the run")
	}
}
