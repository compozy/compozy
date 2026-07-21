package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveWorkflowReadTargetActiveTaskGroupResolvesNestedRoot(t *testing.T) {
	// INVARIANT: an active (unarchived) child resolves to its active parent root joined
	// with the task group directory, so the filesystem fallback reaches the nested active
	// artifacts under <initiative>/_task_groups/<id> rather than the slug-only
	// <initiative>/<id> directory that structurally cannot exist.
	// OWNING_LAYER: read-model. CONTRACT: nested-workflows/reviews-007/issue_003.
	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	initiative := "customer-management"
	writeDaemonDependentTaskGroupFixture(t, env, initiative, false)
	env.writeWorkflowFile(
		t,
		initiative,
		filepath.Join("_task_groups", "TG-001", "task_01.md"),
		daemonTaskBody("pending", "Foundation child task"),
	)
	syncNamedWorkflowForDaemonTest(t, env, initiative)

	svc := &queryService{globalDB: env.globalDB, runManager: env.manager, documents: newDocumentReader()}
	ref := initiative + "/TG-001"
	target, err := svc.resolveWorkflowReadTarget(context.Background(), env.workspaceRoot, ref)
	if err != nil {
		t.Fatalf("resolveWorkflowReadTarget(active task group) error = %v", err)
	}

	if info, statErr := os.Stat(target.rootDir); statErr != nil || !info.IsDir() {
		t.Fatalf("active task group rootDir %q is not a directory: %v", target.rootDir, statErr)
	}
	// Canonicalize both sides: on macOS t.TempDir() lives under the /var -> /private/var
	// symlink, and production stores the resolved workspace root, so a raw string compare
	// would spuriously differ on the prefix rather than the resolution under test.
	resolvedWorkspaceRoot, err := filepath.EvalSymlinks(env.workspaceRoot)
	if err != nil {
		t.Fatalf("EvalSymlinks(workspaceRoot) error = %v", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(target.rootDir)
	if err != nil {
		t.Fatalf("EvalSymlinks(rootDir) error = %v", err)
	}
	wantRoot := filepath.Join(workflowRootDir(resolvedWorkspaceRoot, initiative), "_task_groups", "TG-001")
	if resolvedRoot != wantRoot {
		t.Fatalf("active task group rootDir = %q, want %q", resolvedRoot, wantRoot)
	}
	// Regression guard: the slug-only root that the task-group-unaware resolver produced
	// (the pre-fix behavior) must never be selected for an active task group.
	wrongRoot := workflowRootDir(resolvedWorkspaceRoot, ref)
	if resolvedRoot == wrongRoot {
		t.Fatalf("active task group rootDir resolved to slug-only root %q", wrongRoot)
	}

	// Clearing the durable snapshots forces the filesystem fallback, proving the
	// resolved root reaches the nested active artifacts and not just the DB rows.
	// Before the fix this fell through to the wrong root and surfaced ErrDocumentMissing.
	target.snapshotsByPath = nil
	doc, err := svc.readRequiredWorkflowDocument(
		context.Background(),
		target,
		"task_01.md",
		markdownDocumentKindTask,
		"task_01",
	)
	if err != nil {
		t.Fatalf("readRequiredWorkflowDocument(active task group via filesystem) error = %v", err)
	}
	if doc.Title != "Foundation child task" {
		t.Fatalf("active task group task document title = %q, want Foundation child task", doc.Title)
	}
}
