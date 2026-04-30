package daemon

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	corepkg "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/store/globaldb"
)

func TestWorkspaceTransportService_ShouldHandleCRUDAndUnavailableBranches(t *testing.T) {
	newService := func(t *testing.T) (*runManagerTestEnv, *transportWorkspaceService, apicore.WorkspaceRegisterResult) {
		t.Helper()

		env := newRunManagerTestEnv(t, runManagerTestDeps{})
		service := newTransportWorkspaceService(env.globalDB)
		registered, err := service.Register(context.Background(), env.workspaceRoot, "Demo Workspace")
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}
		return env, service, registered
	}

	t.Run("Should register a workspace", func(t *testing.T) {
		_, _, registered := newService(t)
		if !registered.Created {
			t.Fatal("Register() Created = false, want true")
		}
	})

	t.Run("Should report idempotent registration on repeat calls", func(t *testing.T) {
		env, service, _ := newService(t)
		registeredAgain, err := service.Register(context.Background(), env.workspaceRoot, "Demo Workspace")
		if err != nil {
			t.Fatalf("Register(second) error = %v", err)
		}
		if registeredAgain.Created {
			t.Fatal("Register(second) Created = true, want false")
		}
	})

	t.Run("Should list and get the registered workspace", func(t *testing.T) {
		_, service, registered := newService(t)
		list, err := service.List(context.Background())
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(list) != 1 || list[0].ID != registered.Workspace.ID {
			t.Fatalf("unexpected workspace list: %#v", list)
		}

		got, err := service.Get(context.Background(), registered.Workspace.ID)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if got.RootDir != registered.Workspace.RootDir {
			t.Fatalf("Get().RootDir = %q, want %q", got.RootDir, registered.Workspace.RootDir)
		}
	})

	t.Run("Should resolve a workspace by root path", func(t *testing.T) {
		env, service, registered := newService(t)
		resolved, err := service.Resolve(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if resolved.ID != registered.Workspace.ID {
			t.Fatalf("Resolve().ID = %q, want %q", resolved.ID, registered.Workspace.ID)
		}
	})

	t.Run("Should report unavailable workspace updates", func(t *testing.T) {
		_, service, registered := newService(t)
		if _, err := service.Update(
			context.Background(),
			registered.Workspace.ID,
			apicore.WorkspaceUpdateInput{},
		); err == nil || !strings.Contains(err.Error(), "workspace updates is not available") {
			t.Fatalf("Update() error = %v, want unavailable", err)
		}
	})

	t.Run("Should delete a registered workspace", func(t *testing.T) {
		_, service, registered := newService(t)
		if err := service.Delete(context.Background(), registered.Workspace.ID); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		if _, err := service.Get(context.Background(), registered.Workspace.ID); err == nil {
			t.Fatal("Get(after delete) error = nil, want non-nil")
		}
	})

	t.Run("Should report unavailable listing and resolution when the registry is missing", func(t *testing.T) {
		env := newRunManagerTestEnv(t, runManagerTestDeps{})
		nilService := newTransportWorkspaceService(nil)
		if _, err := nilService.List(context.Background()); err == nil ||
			!strings.Contains(err.Error(), "workspace listing is not available") {
			t.Fatalf("nil List() error = %v, want unavailable", err)
		}
		if _, err := nilService.Resolve(context.Background(), env.workspaceRoot); err == nil ||
			!strings.Contains(err.Error(), "workspace resolution is not available") {
			t.Fatalf("nil Resolve() error = %v, want unavailable", err)
		}
	})
}

func TestTaskTransportService_ShouldHandleWorkflowReadsAndUnavailableBranches(t *testing.T) {
	newService := func(t *testing.T) (*runManagerTestEnv, *transportTaskService) {
		t.Helper()

		env := newRunManagerTestEnv(t, runManagerTestDeps{})
		env.writeWorkflowFile(t, env.workflowSlug, "task_01.md", daemonTaskBody("pending", "Transport task"))
		initialRun := env.startTaskRun(t, "task-transport-seed-001", nil)
		waitForRun(t, env.globalDB, initialRun.RunID, func(row globaldb.Run) bool {
			return row.Status == runStatusCompleted
		})
		return env, newTransportTaskService(env.globalDB, env.manager)
	}

	t.Run("Should list and get workflows", func(t *testing.T) {
		env, service := newService(t)
		workflows, err := service.ListWorkflows(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("ListWorkflows() error = %v", err)
		}
		if len(workflows) != 1 || workflows[0].Slug != env.workflowSlug {
			t.Fatalf("unexpected workflows: %#v", workflows)
		}
		if workflows[0].TaskCounts == nil || workflows[0].TaskCounts.Total != 1 ||
			workflows[0].TaskCounts.Pending != 1 {
			t.Fatalf("unexpected workflow task counts: %#v", workflows[0].TaskCounts)
		}
		if workflows[0].CanStartRun == nil || !*workflows[0].CanStartRun ||
			workflows[0].StartBlockReason != "" {
			t.Fatalf("unexpected workflow start action: %#v", workflows[0])
		}
		if workflows[0].ArchiveEligible == nil || *workflows[0].ArchiveEligible ||
			workflows[0].ArchiveReason != "task workflow not fully completed" {
			t.Fatalf("unexpected workflow archive action: %#v", workflows[0])
		}

		workflow, err := service.GetWorkflow(context.Background(), env.workspaceRoot, env.workflowSlug)
		if err != nil {
			t.Fatalf("GetWorkflow() error = %v", err)
		}
		if workflow.Slug != env.workflowSlug {
			t.Fatalf("GetWorkflow().Slug = %q, want %q", workflow.Slug, env.workflowSlug)
		}
		if workflow.TaskCounts == nil || workflow.TaskCounts.Total != 1 || workflow.TaskCounts.Pending != 1 {
			t.Fatalf("unexpected GetWorkflow() task counts: %#v", workflow.TaskCounts)
		}
		if workflow.CanStartRun == nil || !*workflow.CanStartRun || workflow.StartBlockReason != "" {
			t.Fatalf("unexpected GetWorkflow() start action: %#v", workflow)
		}
		if workflow.ArchiveEligible == nil || *workflow.ArchiveEligible ||
			workflow.ArchiveReason != "task workflow not fully completed" {
			t.Fatalf("unexpected GetWorkflow() archive action: %#v", workflow)
		}
	})

	t.Run("Should mark completed workflows as not startable", func(t *testing.T) {
		env := newRunManagerTestEnv(t, runManagerTestDeps{})
		env.writeWorkflowFile(t, env.workflowSlug, "task_01.md", daemonTaskBody("completed", "Transport task"))
		syncWorkflowForDaemonTest(t, env)

		service := newTransportTaskService(env.globalDB, env.manager)
		workflows, err := service.ListWorkflows(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("ListWorkflows() error = %v", err)
		}
		if len(workflows) != 1 || workflows[0].TaskCounts == nil {
			t.Fatalf("unexpected workflows: %#v", workflows)
		}
		if workflows[0].TaskCounts.Total != 1 || workflows[0].TaskCounts.Completed != 1 ||
			workflows[0].TaskCounts.Pending != 0 {
			t.Fatalf("unexpected completed counts: %#v", workflows[0].TaskCounts)
		}
		if workflows[0].CanStartRun == nil || *workflows[0].CanStartRun {
			t.Fatalf("CanStartRun = %#v, want false", workflows[0].CanStartRun)
		}
		if workflows[0].StartBlockReason != "no pending tasks" {
			t.Fatalf("StartBlockReason = %q, want no pending tasks", workflows[0].StartBlockReason)
		}
		if workflows[0].ArchiveEligible == nil || !*workflows[0].ArchiveEligible ||
			workflows[0].ArchiveReason != "" {
			t.Fatalf("unexpected completed archive action: %#v", workflows[0])
		}
	})

	t.Run("Should expose archive eligibility for review-only workflows", func(t *testing.T) {
		env := newRunManagerTestEnv(t, runManagerTestDeps{})
		resolvedSlug := "review-only-resolved"
		pendingSlug := "review-only-pending"
		env.writeWorkflowFile(
			t,
			resolvedSlug,
			filepath.Join("reviews-001", "issue_001.md"),
			daemonReviewIssueBody("resolved", "medium"),
		)
		env.writeWorkflowFile(
			t,
			pendingSlug,
			filepath.Join("reviews-001", "issue_001.md"),
			daemonReviewIssueBody("pending", "high"),
		)
		syncNamedWorkflowForDaemonTest(t, env, resolvedSlug)
		syncNamedWorkflowForDaemonTest(t, env, pendingSlug)

		service := newTransportTaskService(env.globalDB, env.manager)
		workflows, err := service.ListWorkflows(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("ListWorkflows() error = %v", err)
		}
		bySlug := make(map[string]apicore.WorkflowSummary, len(workflows))
		for _, workflow := range workflows {
			bySlug[workflow.Slug] = workflow
		}
		resolved := bySlug[resolvedSlug]
		if resolved.ArchiveEligible == nil || !*resolved.ArchiveEligible ||
			resolved.ArchiveReason != "" {
			t.Fatalf("unexpected resolved review-only archive action: %#v", resolved)
		}
		pending := bySlug[pendingSlug]
		if pending.ArchiveEligible == nil || *pending.ArchiveEligible ||
			pending.ArchiveReason != "review rounds not fully resolved" {
			t.Fatalf("unexpected pending review-only archive action: %#v", pending)
		}
	})

	t.Run("Should start a task run", func(t *testing.T) {
		env, service := newService(t)
		run, err := service.StartRun(context.Background(), env.workspaceRoot, env.workflowSlug, apicore.TaskRunRequest{
			Workspace:        env.workspaceRoot,
			PresentationMode: defaultPresentationMode,
			RuntimeOverrides: rawJSON(t, `{"run_id":"task-transport-run-002","dry_run":true}`),
		})
		if err != nil {
			t.Fatalf("StartRun() error = %v", err)
		}
		waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
			return row.Status == runStatusCompleted
		})
	})

	t.Run("Should report unavailable item listing and validation", func(t *testing.T) {
		env, service := newService(t)
		if _, err := service.ListItems(context.Background(), env.workspaceRoot, env.workflowSlug); err == nil ||
			!strings.Contains(err.Error(), "task item listing is not available") {
			t.Fatalf("ListItems() error = %v, want unavailable", err)
		}
		if _, err := service.Validate(context.Background(), env.workspaceRoot, env.workflowSlug); err == nil ||
			!strings.Contains(err.Error(), "task validation is not available") {
			t.Fatalf("Validate() error = %v, want unavailable", err)
		}
	})

	t.Run("Should archive workflows and surface archived reads", func(t *testing.T) {
		env, service := newService(t)
		env.writeWorkflowFile(t, env.workflowSlug, "task_01.md", daemonTaskBody("completed", "Transport task"))
		syncWorkflowForDaemonTest(t, env)
		archiveResult, err := service.Archive(context.Background(), env.workspaceRoot, env.workflowSlug)
		if err != nil {
			t.Fatalf("Archive() error = %v", err)
		}
		if !archiveResult.Archived {
			t.Fatalf("Archive().Archived = %v, want true", archiveResult.Archived)
		}
		workflowsAfterArchive, err := service.ListWorkflows(context.Background(), env.workspaceRoot)
		if err != nil {
			t.Fatalf("ListWorkflows(after archive) error = %v", err)
		}
		if len(workflowsAfterArchive) != 1 || workflowsAfterArchive[0].ArchivedAt == nil {
			t.Fatalf("unexpected workflows after archive: %#v", workflowsAfterArchive)
		}

		detail, err := service.TaskDetail(context.Background(), env.workspaceRoot, env.workflowSlug, "task_01")
		if err != nil {
			t.Fatalf("TaskDetail(archived workflow) error = %v", err)
		}
		if detail.Task.Title != "Transport task" || detail.Document.Title != "Transport task" {
			t.Fatalf("unexpected archived task detail: %#v", detail)
		}
	})

	t.Run("Should report unavailable workflow listing and archiving without a database", func(t *testing.T) {
		env, _ := newService(t)
		nilDBService := newTransportTaskService(nil, env.manager)
		if _, err := nilDBService.ListWorkflows(context.Background(), env.workspaceRoot); err == nil ||
			!strings.Contains(err.Error(), "workflow listing is not available") {
			t.Fatalf("nil ListWorkflows() error = %v, want unavailable", err)
		}
		if _, err := nilDBService.Archive(context.Background(), env.workspaceRoot, env.workflowSlug); err == nil ||
			!strings.Contains(err.Error(), "task archiving is not available") {
			t.Fatalf("nil Archive() error = %v, want unavailable", err)
		}
	})

	t.Run("Should report unavailable task runs without a run manager", func(t *testing.T) {
		env, _ := newService(t)
		nilRunManagerService := newTransportTaskService(env.globalDB, nil)
		if _, err := nilRunManagerService.StartRun(
			context.Background(),
			env.workspaceRoot,
			env.workflowSlug,
			apicore.TaskRunRequest{},
		); err == nil || !strings.Contains(err.Error(), "task runs is not available") {
			t.Fatalf("nil StartRun() error = %v, want unavailable", err)
		}
	})
}

func syncWorkflowForDaemonTest(t *testing.T, env *runManagerTestEnv) {
	t.Helper()

	syncNamedWorkflowForDaemonTest(t, env, env.workflowSlug)
}

func syncNamedWorkflowForDaemonTest(t *testing.T, env *runManagerTestEnv, slug string) {
	t.Helper()

	workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveOrRegister() error = %v", err)
	}
	if _, err := corepkg.SyncWithDB(context.Background(), env.globalDB, workspace, corepkg.SyncConfig{
		WorkspaceRoot: workspace.RootDir,
		Name:          slug,
	}); err != nil {
		t.Fatalf("SyncWithDB() error = %v", err)
	}
}

func TestTransportSyncResult_ShouldMapStructuredFields(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve identity counts and slices", func(t *testing.T) {
		t.Parallel()

		syncedAt := time.Date(2026, 4, 20, 22, 0, 0, 0, time.UTC)
		result := transportSyncResult("ws-123", "demo", &syncedAt, &corepkg.SyncResult{
			Target:                 "/tmp/demo",
			WorkflowsScanned:       2,
			WorkflowsPruned:        3,
			SnapshotsUpserted:      4,
			TaskItemsUpserted:      5,
			ReviewRoundsUpserted:   6,
			ReviewIssuesUpserted:   7,
			CheckpointsUpdated:     8,
			LegacyArtifactsRemoved: 9,
			SyncedPaths:            []string{"a", "b"},
			PrunedWorkflows:        []string{"stale"},
			Warnings:               []string{"warn"},
		})

		if result.WorkspaceID != "ws-123" || result.WorkflowSlug != "demo" {
			t.Fatalf("unexpected sync identity payload: %#v", result)
		}
		if result.WorkflowsPruned != 3 || result.TaskItemsUpserted != 5 ||
			result.ReviewIssuesUpserted != 7 || result.LegacyArtifactsRemoved != 9 {
			t.Fatalf("unexpected sync counts: %#v", result)
		}
		if len(result.SyncedPaths) != 2 || len(result.PrunedWorkflows) != 1 ||
			result.PrunedWorkflows[0] != "stale" || len(result.Warnings) != 1 {
			t.Fatalf("unexpected sync slices: %#v", result)
		}
	})
}
