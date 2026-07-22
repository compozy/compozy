package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apicore "github.com/compozy/compozy/internal/api/core"
	corepkg "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/taskgroups"
	"github.com/compozy/compozy/internal/store/globaldb"
)

// archiveDependentTaskGroupInitiativeForTest builds a two-task-group initiative with
// per-task-group tasks, memory, and a review round, syncs it into the durable
// catalog, then force-archives the whole hierarchy as one root. It returns the
// archive result so callers can locate the moved archive directory.
func archiveDependentTaskGroupInitiativeForTest(
	t *testing.T,
	env *runManagerTestEnv,
	initiative string,
) *corepkg.ArchiveResult {
	t.Helper()

	writeDaemonDependentTaskGroupFixture(t, env, initiative, false)
	for _, fixture := range []struct {
		taskGroupID string
		title       string
		severity    string
	}{
		{taskGroupID: "TG-001", title: "Foundation child task", severity: "high"},
		{taskGroupID: "TG-002", title: "Delivery child task", severity: "low"},
	} {
		taskGroupRoot := filepath.Join("_task_groups", fixture.taskGroupID)
		env.writeWorkflowFile(
			t,
			initiative,
			filepath.Join(taskGroupRoot, "task_01.md"),
			daemonTaskBody("pending", fixture.title),
		)
		env.writeWorkflowFile(
			t,
			initiative,
			filepath.Join(taskGroupRoot, "memory", "MEMORY.md"),
			"# "+fixture.taskGroupID+" Memory\n",
		)
		env.writeWorkflowFile(
			t,
			initiative,
			filepath.Join(taskGroupRoot, "reviews-001", "_meta.md"),
			daemonReviewRoundMetaBody("manual", fixture.taskGroupID, 1),
		)
		env.writeWorkflowFile(
			t,
			initiative,
			filepath.Join(taskGroupRoot, "reviews-001", "issue_001.md"),
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

// recreateInitiativeWithoutFoundationForTest writes a fresh single-task-group plan
// for an already-archived initiative that drops TG-001, then re-syncs it so a new
// active generation exists whose current plan no longer contains the foundation
// task group. The archived TG-001 row remains in the durable catalog.
func recreateInitiativeWithoutFoundationForTest(
	t *testing.T,
	env *runManagerTestEnv,
	initiative string,
) {
	t.Helper()

	plan, err := taskgroups.RenderPlan(taskgroups.Plan{
		SchemaVersion: taskgroups.SchemaVersion,
		Initiative:    initiative,
		TaskGroups: []taskgroups.TaskGroup{
			{
				ID:         "TG-002",
				Title:      "Delivery",
				Outcome:    "Deliver the recreated scope",
				Directory:  "_task_groups/TG-002",
				OwnedScope: []string{"delivery"},
			},
		},
	})
	if err != nil {
		t.Fatalf("RenderPlan() error = %v", err)
	}
	env.writeWorkflowFile(t, initiative, "_prd.md", "# Recreated PRD\n")
	env.writeWorkflowFile(t, initiative, "_techspec.md", "# Recreated TechSpec\n")
	env.writeWorkflowFile(t, initiative, "_task_groups.md", string(plan))
	env.writeWorkflowFile(
		t,
		initiative,
		filepath.Join("_task_groups", "TG-002", "task_01.md"),
		daemonTaskBody("pending", "Recreated delivery task"),
	)
	syncNamedWorkflowForDaemonTest(t, env, initiative)
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

func archivedTaskGroupPlanSnapshotForTest(
	t *testing.T,
	env *runManagerTestEnv,
	taskGroupRef string,
) globaldb.ArtifactSnapshotRow {
	t.Helper()

	workspaceID := mustWorkspaceID(t, env.globalDB, env.workspaceRoot)
	taskGroupWorkflow, err := env.globalDB.GetLatestArchivedWorkflowBySlug(
		context.Background(),
		workspaceID,
		taskGroupRef,
	)
	if err != nil {
		t.Fatalf("GetLatestArchivedWorkflowBySlug(%s) error = %v", taskGroupRef, err)
	}
	snapshots, err := env.globalDB.ListArtifactSnapshots(
		context.Background(),
		taskGroupWorkflow.ParentWorkflowID,
	)
	if err != nil {
		t.Fatalf("ListArtifactSnapshots(parent of %s) error = %v", taskGroupRef, err)
	}
	for idx := range snapshots {
		if filepath.ToSlash(snapshots[idx].RelativePath) == taskgroups.ManifestFileName {
			return snapshots[idx]
		}
	}
	t.Fatalf("parent of %s has no %s snapshot", taskGroupRef, taskgroups.ManifestFileName)
	return globaldb.ArtifactSnapshotRow{}
}

func TestQueryServiceReadsArchivedTaskGroupPlanExcerptAfterArchiveDirectoryDisappears(t *testing.T) {
	// INVARIANT: an archived task group's plan excerpt remains readable from the
	// selected parent generation's durable snapshot when its archive directory is gone.
	// OWNING_LAYER: service-integration. CONTRACT: nested-workflows/reviews-004/issue_005.
	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	initiative := "customer-management"
	result := archiveDependentTaskGroupInitiativeForTest(t, env, initiative)
	taskGroupRef := initiative + "/TG-001"
	planSnapshot := archivedTaskGroupPlanSnapshotForTest(t, env, taskGroupRef)

	if err := os.RemoveAll(result.ArchivedPaths[0]); err != nil {
		t.Fatalf("RemoveAll(archived initiative) error = %v", err)
	}

	query := NewQueryService(QueryServiceConfig{GlobalDB: env.globalDB, RunManager: env.manager})
	spec, err := query.WorkflowSpec(context.Background(), env.workspaceRoot, taskGroupRef)
	if err != nil {
		t.Fatalf("WorkflowSpec(%s) error = %v, want snapshot-backed success", taskGroupRef, err)
	}
	if spec.PlanExcerpt == nil || !strings.Contains(spec.PlanExcerpt.Markdown, "TG-001 — Foundation") {
		t.Fatalf("WorkflowSpec(%s).PlanExcerpt = %#v", taskGroupRef, spec.PlanExcerpt)
	}
	if !spec.PlanExcerpt.UpdatedAt.Equal(planSnapshot.SourceMTime) {
		t.Fatalf(
			"WorkflowSpec(%s).PlanExcerpt.UpdatedAt = %s, want snapshot time %s",
			taskGroupRef,
			spec.PlanExcerpt.UpdatedAt,
			planSnapshot.SourceMTime,
		)
	}
}

func TestQueryServicePinsArchivedTaskGroupPlanExcerptToDurableGeneration(t *testing.T) {
	// INVARIANT: an archived task group's plan excerpt uses the selected parent
	// generation's durable content and timestamp even when its filesystem plan diverges.
	// OWNING_LAYER: service-integration. CONTRACT: nested-workflows/reviews-004/issue_005.
	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	initiative := "customer-management"
	result := archiveDependentTaskGroupInitiativeForTest(t, env, initiative)
	taskGroupRef := initiative + "/TG-001"
	planSnapshot := archivedTaskGroupPlanSnapshotForTest(t, env, taskGroupRef)
	planPath := filepath.Join(result.ArchivedPaths[0], taskgroups.ManifestFileName)

	content, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("ReadFile(archived task group plan) error = %v", err)
	}
	divergentContent := strings.Replace(string(content), "TG-001 — Foundation", "TG-001 — Filesystem Drift", 1)
	if err := os.WriteFile(planPath, []byte(divergentContent), 0o600); err != nil {
		t.Fatalf("WriteFile(divergent archived task group plan) error = %v", err)
	}
	filesystemTime := planSnapshot.SourceMTime.AddDate(0, 0, 1)
	if err := os.Chtimes(planPath, filesystemTime, filesystemTime); err != nil {
		t.Fatalf("Chtimes(divergent archived task group plan) error = %v", err)
	}

	query := NewQueryService(QueryServiceConfig{GlobalDB: env.globalDB, RunManager: env.manager})
	spec, err := query.WorkflowSpec(context.Background(), env.workspaceRoot, taskGroupRef)
	if err != nil {
		t.Fatalf("WorkflowSpec(%s) error = %v", taskGroupRef, err)
	}
	if spec.PlanExcerpt == nil || !strings.Contains(spec.PlanExcerpt.Markdown, "TG-001 — Foundation") {
		t.Fatalf("WorkflowSpec(%s).PlanExcerpt = %#v, want durable Foundation excerpt", taskGroupRef, spec.PlanExcerpt)
	}
	if strings.Contains(spec.PlanExcerpt.Markdown, "Filesystem Drift") {
		t.Fatalf("WorkflowSpec(%s) used divergent filesystem plan: %q", taskGroupRef, spec.PlanExcerpt.Markdown)
	}
	if !spec.PlanExcerpt.UpdatedAt.Equal(planSnapshot.SourceMTime) {
		t.Fatalf(
			"WorkflowSpec(%s).PlanExcerpt.UpdatedAt = %s, want snapshot time %s",
			taskGroupRef,
			spec.PlanExcerpt.UpdatedAt,
			planSnapshot.SourceMTime,
		)
	}
}

func TestTransportServicesReadArchivedTaskGroupThroughPublicSurface(t *testing.T) {
	// INVARIANT: archived task groups stay readable through the public transport surface
	// even though their active plan directory has moved into the archive hierarchy.
	// OWNING_LAYER: service-integration. CONTRACT: nested-workflows/reviews-002/issue_004.
	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	initiative := "customer-management"
	archiveDependentTaskGroupInitiativeForTest(t, env, initiative)

	query := NewQueryService(QueryServiceConfig{GlobalDB: env.globalDB, RunManager: env.manager})
	taskService := newTransportTaskService(env.globalDB, env.manager, query)
	reviewService := newTransportReviewService(env.globalDB, env.manager, query)
	ctx := context.Background()
	ref := initiative + "/TG-001"

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
	if spec.PlanExcerpt == nil || !strings.Contains(spec.PlanExcerpt.Markdown, "TG-001 — Foundation") {
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

func TestResolveWorkflowReadTargetArchivedTaskGroupResolvesNestedRoot(t *testing.T) {
	// INVARIANT: an archived child resolves to its archived parent root joined with the
	// task group directory, so filesystem reads reach the nested archived artifacts rather
	// than a non-existent top-level archive directory derived from the child slug.
	// OWNING_LAYER: read-model. CONTRACT: nested-workflows/reviews-002/issue_004.
	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	initiative := "customer-management"
	result := archiveDependentTaskGroupInitiativeForTest(t, env, initiative)

	svc := &queryService{globalDB: env.globalDB, runManager: env.manager, documents: newDocumentReader()}
	target, err := svc.resolveWorkflowReadTarget(context.Background(), env.workspaceRoot, initiative+"/TG-001")
	if err != nil {
		t.Fatalf("resolveWorkflowReadTarget(archived task group) error = %v", err)
	}

	wantRoot := filepath.Join(result.ArchivedPaths[0], "_task_groups", "TG-001")
	if target.rootDir != wantRoot {
		t.Fatalf("archived task group rootDir = %q, want %q", target.rootDir, wantRoot)
	}
	if info, statErr := os.Stat(target.rootDir); statErr != nil || !info.IsDir() {
		t.Fatalf("archived task group rootDir %q is not a directory: %v", target.rootDir, statErr)
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
		t.Fatalf("readRequiredWorkflowDocument(archived task group via filesystem) error = %v", err)
	}
	if doc.Title != "Foundation child task" {
		t.Fatalf("archived task group task document title = %q, want Foundation child task", doc.Title)
	}
}

func TestTransportGateRejectsArchivedTaskGroupDroppedByRecreatedParent(t *testing.T) {
	// INVARIANT: once an initiative is archived and recreated with a plan that drops a
	// child task group, public reads of that child resolve relative to the current parent
	// generation and return a typed task-group-not-found instead of shadowing the stale
	// archived generation's tasks, spec, reviews, and memory.
	// OWNING_LAYER: service-integration. CONTRACT: nested-workflows/reviews-003/issue_002.
	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	initiative := "customer-management"
	archiveDependentTaskGroupInitiativeForTest(t, env, initiative)
	recreateInitiativeWithoutFoundationForTest(t, env, initiative)

	query := NewQueryService(QueryServiceConfig{GlobalDB: env.globalDB, RunManager: env.manager})
	taskService := newTransportTaskService(env.globalDB, env.manager, query)
	reviewService := newTransportReviewService(env.globalDB, env.manager, query)
	ctx := context.Background()
	droppedRef := initiative + "/TG-001"

	surfaces := []struct {
		name string
		read func() error
	}{
		{"WorkflowOverview", func() error {
			_, err := taskService.WorkflowOverview(ctx, env.workspaceRoot, droppedRef)
			return err
		}},
		{"TaskBoard", func() error {
			_, err := taskService.TaskBoard(ctx, env.workspaceRoot, droppedRef)
			return err
		}},
		{"TaskDetail", func() error {
			_, err := taskService.TaskDetail(ctx, env.workspaceRoot, droppedRef, "task_01")
			return err
		}},
		{"WorkflowSpec", func() error {
			_, err := taskService.WorkflowSpec(ctx, env.workspaceRoot, droppedRef)
			return err
		}},
		{"WorkflowMemoryIndex", func() error {
			_, err := taskService.WorkflowMemoryIndex(ctx, env.workspaceRoot, droppedRef)
			return err
		}},
		{"ReviewGetLatest", func() error {
			_, err := reviewService.GetLatest(ctx, env.workspaceRoot, droppedRef)
			return err
		}},
		{"ReviewListIssues", func() error {
			_, err := reviewService.ListIssues(ctx, env.workspaceRoot, droppedRef, 1)
			return err
		}},
	}
	for _, surface := range surfaces {
		t.Run("Should reject dropped task group on "+surface.name, func(t *testing.T) {
			err := surface.read()
			if !errors.Is(err, taskgroups.ErrTaskGroupNotFound) {
				t.Fatalf("%s(%s) error = %v, want ErrTaskGroupNotFound", surface.name, droppedRef, err)
			}
		})
	}

	// Sanity: the task group the recreated plan still contains reads the current
	// generation rather than being over-blocked, and does not return archived data.
	survivingRef := initiative + "/TG-002"
	board, err := taskService.TaskBoard(ctx, env.workspaceRoot, survivingRef)
	if err != nil {
		t.Fatalf("TaskBoard(%s) error = %v", survivingRef, err)
	}
	if !taskBoardContainsTitle(board.Lanes, "Recreated delivery task") {
		t.Fatalf("TaskBoard(%s) missing recreated task: %#v", survivingRef, board.Lanes)
	}
	if taskBoardContainsTitle(board.Lanes, "Delivery child task") {
		t.Fatalf("TaskBoard(%s) served stale archived task: %#v", survivingRef, board.Lanes)
	}
}
