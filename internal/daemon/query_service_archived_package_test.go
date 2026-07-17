package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apicore "github.com/compozy/compozy/internal/api/core"
	corepkg "github.com/compozy/compozy/internal/core"
)

// archiveDependentPackageInitiativeForTest builds a two-package initiative with
// per-package tasks, memory, and a review round, syncs it into the durable
// catalog, then force-archives the whole hierarchy as one root. It returns the
// archive result so callers can locate the moved archive directory.
func archiveDependentPackageInitiativeForTest(
	t *testing.T,
	env *runManagerTestEnv,
	initiative string,
) *corepkg.ArchiveResult {
	t.Helper()

	writeDaemonDependentPackageFixture(t, env, initiative, false)
	for _, fixture := range []struct {
		packageID string
		title     string
		severity  string
	}{
		{packageID: "WP-001", title: "Foundation child task", severity: "high"},
		{packageID: "WP-002", title: "Delivery child task", severity: "low"},
	} {
		packageRoot := filepath.Join("_packages", fixture.packageID)
		env.writeWorkflowFile(
			t,
			initiative,
			filepath.Join(packageRoot, "task_01.md"),
			daemonTaskBody("pending", fixture.title),
		)
		env.writeWorkflowFile(
			t,
			initiative,
			filepath.Join(packageRoot, "memory", "MEMORY.md"),
			"# "+fixture.packageID+" Memory\n",
		)
		env.writeWorkflowFile(
			t,
			initiative,
			filepath.Join(packageRoot, "reviews-001", "_meta.md"),
			daemonReviewRoundMetaBody("manual", fixture.packageID, 1),
		)
		env.writeWorkflowFile(
			t,
			initiative,
			filepath.Join(packageRoot, "reviews-001", "issue_001.md"),
			daemonReviewIssueBody("pending", fixture.severity),
		)
	}
	syncNamedWorkflowForDaemonTest(t, env, initiative)

	workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveOrRegister() error = %v", err)
	}
	result, err := corepkg.ArchiveWithDB(context.Background(), env.globalDB, workspace, corepkg.ArchiveConfig{
		WorkspaceRoot: workspace.RootDir,
		Name:          initiative,
		Force:         true,
	})
	if err != nil {
		t.Fatalf("ArchiveWithDB(%s) error = %v", initiative, err)
	}
	if result == nil || result.Archived != 1 || len(result.ArchivedPaths) != 1 {
		t.Fatalf("ArchiveWithDB(%s) result = %#v, want one archived root", initiative, result)
	}
	return result
}

func taskBoardContainsTitle(lanes []apicore.TaskLane, title string) bool {
	for laneIndex := range lanes {
		for itemIndex := range lanes[laneIndex].Items {
			if lanes[laneIndex].Items[itemIndex].Title == title {
				return true
			}
		}
	}
	return false
}

func TestTransportServicesReadArchivedWorkPackageThroughPublicSurface(t *testing.T) {
	// INVARIANT: archived work packages stay readable through the public transport surface
	// even though their active plan directory has moved into the archive hierarchy.
	// OWNING_LAYER: service-integration. CONTRACT: nested-workflows/reviews-002/issue_004.
	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	initiative := "customer-management"
	archiveDependentPackageInitiativeForTest(t, env, initiative)

	query := NewQueryService(QueryServiceConfig{GlobalDB: env.globalDB, RunManager: env.manager})
	taskService := newTransportTaskService(env.globalDB, env.manager, query)
	reviewService := newTransportReviewService(env.globalDB, env.manager, query)
	ctx := context.Background()
	ref := initiative + "/WP-001"

	overview, err := taskService.WorkflowOverview(ctx, env.workspaceRoot, ref)
	if err != nil {
		t.Fatalf("WorkflowOverview(%s) error = %v", ref, err)
	}
	if overview.Workflow.Slug != ref {
		t.Fatalf("WorkflowOverview(%s).Workflow.Slug = %q, want %q", ref, overview.Workflow.Slug, ref)
	}

	board, err := taskService.TaskBoard(ctx, env.workspaceRoot, ref)
	if err != nil {
		t.Fatalf("TaskBoard(%s) error = %v", ref, err)
	}
	if !taskBoardContainsTitle(board.Lanes, "Foundation child task") {
		t.Fatalf("TaskBoard(%s) missing archived task: %#v", ref, board.Lanes)
	}

	detail, err := taskService.TaskDetail(ctx, env.workspaceRoot, ref, "task_01")
	if err != nil {
		t.Fatalf("TaskDetail(%s) error = %v", ref, err)
	}
	if detail.Task.Title != "Foundation child task" {
		t.Fatalf("TaskDetail(%s).Task.Title = %q, want Foundation child task", ref, detail.Task.Title)
	}

	spec, err := taskService.WorkflowSpec(ctx, env.workspaceRoot, ref)
	if err != nil {
		t.Fatalf("WorkflowSpec(%s) error = %v", ref, err)
	}
	if spec.PRD == nil || spec.PRD.Title != "Canonical PRD" ||
		spec.TechSpec == nil || spec.TechSpec.Title != "Canonical TechSpec" {
		t.Fatalf("WorkflowSpec(%s) canonical documents = %#v", ref, spec)
	}
	if spec.PlanExcerpt == nil || !strings.Contains(spec.PlanExcerpt.Markdown, "WP-001 — Foundation") {
		t.Fatalf("WorkflowSpec(%s).PlanExcerpt = %#v", ref, spec.PlanExcerpt)
	}

	memoryIndex, err := taskService.WorkflowMemoryIndex(ctx, env.workspaceRoot, ref)
	if err != nil {
		t.Fatalf("WorkflowMemoryIndex(%s) error = %v", ref, err)
	}
	if len(memoryIndex.Entries) != 1 {
		t.Fatalf("WorkflowMemoryIndex(%s).Entries = %#v, want one memory entry", ref, memoryIndex.Entries)
	}

	summary, err := reviewService.GetLatest(ctx, env.workspaceRoot, ref)
	if err != nil {
		t.Fatalf("GetLatest(%s) error = %v", ref, err)
	}
	if summary.WorkflowSlug != ref {
		t.Fatalf("GetLatest(%s).WorkflowSlug = %q, want %q", ref, summary.WorkflowSlug, ref)
	}
	issues, err := reviewService.ListIssues(ctx, env.workspaceRoot, ref, 1)
	if err != nil {
		t.Fatalf("ListIssues(%s) error = %v", ref, err)
	}
	if len(issues) != 1 || issues[0].Severity != "high" {
		t.Fatalf("ListIssues(%s) = %#v, want one high-severity issue", ref, issues)
	}
	issueDetail, err := reviewService.ReviewDetail(ctx, env.workspaceRoot, ref, 1, "1")
	if err != nil {
		t.Fatalf("ReviewDetail(%s) error = %v", ref, err)
	}
	if issueDetail.Issue.Severity != "high" {
		t.Fatalf("ReviewDetail(%s).Issue.Severity = %q, want high", ref, issueDetail.Issue.Severity)
	}
}

func TestResolveWorkflowReadTargetArchivedPackageResolvesNestedRoot(t *testing.T) {
	// INVARIANT: an archived child resolves to its archived parent root joined with the
	// package directory, so filesystem reads reach the nested archived artifacts rather
	// than a non-existent top-level archive directory derived from the child slug.
	// OWNING_LAYER: read-model. CONTRACT: nested-workflows/reviews-002/issue_004.
	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	initiative := "customer-management"
	result := archiveDependentPackageInitiativeForTest(t, env, initiative)

	svc := &queryService{globalDB: env.globalDB, runManager: env.manager, documents: newDocumentReader()}
	target, err := svc.resolveWorkflowReadTarget(context.Background(), env.workspaceRoot, initiative+"/WP-001")
	if err != nil {
		t.Fatalf("resolveWorkflowReadTarget(archived package) error = %v", err)
	}

	wantRoot := filepath.Join(result.ArchivedPaths[0], "_packages", "WP-001")
	if target.rootDir != wantRoot {
		t.Fatalf("archived package rootDir = %q, want %q", target.rootDir, wantRoot)
	}
	if info, statErr := os.Stat(target.rootDir); statErr != nil || !info.IsDir() {
		t.Fatalf("archived package rootDir %q is not a directory: %v", target.rootDir, statErr)
	}

	// Clearing the durable snapshots forces the filesystem fallback, proving the
	// resolved root reaches the nested archived artifacts and not just the DB rows.
	target.snapshotsByPath = nil
	doc, err := svc.readRequiredWorkflowDocument(
		context.Background(),
		target,
		"task_01.md",
		markdownDocumentKindTask,
		"task_01",
	)
	if err != nil {
		t.Fatalf("readRequiredWorkflowDocument(archived package via filesystem) error = %v", err)
	}
	if doc.Title != "Foundation child task" {
		t.Fatalf("archived package task document title = %q, want Foundation child task", doc.Title)
	}
}
