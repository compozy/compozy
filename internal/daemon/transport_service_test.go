package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	apicore "github.com/compozy/compozy/internal/api/core"
	corepkg "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/taskgroups"
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

	t.Run("Should report unavailable item listing and validate task metadata", func(t *testing.T) {
		env, service := newService(t)
		if _, err := service.ListItems(context.Background(), env.workspaceRoot, env.workflowSlug); err == nil ||
			!strings.Contains(err.Error(), "task item listing is not available") {
			t.Fatalf("ListItems() error = %v, want unavailable", err)
		}
		result, err := service.Validate(context.Background(), env.workspaceRoot, env.workflowSlug)
		if err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
		if !result.Valid || result.CheckedAt.IsZero() {
			t.Fatalf("Validate() result = %#v, want successful validation", result)
		}
	})

	t.Run("Should validate a task group against its logical reference", func(t *testing.T) {
		env := newRunManagerTestEnv(t, runManagerTestDeps{})
		initiative := "watcher"
		writeDaemonDependentTaskGroupFixture(t, env, initiative, true)
		env.writeWorkflowFile(
			t,
			initiative,
			filepath.Join("_task_groups", "TG-002", "_tasks.md"),
			taskGroupTaskGraphManifest("watcher/TG-002"),
		)

		service := newTransportTaskService(env.globalDB, env.manager)
		result, err := service.Validate(context.Background(), env.workspaceRoot, "watcher/TG-002")
		if err != nil {
			t.Fatalf("Validate(task group) error = %v", err)
		}
		if !result.Valid {
			t.Fatalf("Validate(task group) result = %#v, want valid", result)
		}

		env.writeWorkflowFile(
			t,
			initiative,
			filepath.Join("_task_groups", "TG-002", "_tasks.md"),
			taskGroupTaskGraphManifest("watcher/TG-001"),
		)
		_, err = service.Validate(context.Background(), env.workspaceRoot, "watcher/TG-002")
		var problem *apicore.Problem
		if !errors.As(err, &problem) {
			t.Fatalf("Validate(wrong task group manifest) error = %v, want transport problem", err)
		}
		if problem.Status != 422 || problem.Code != "task_validation_failed" {
			t.Fatalf("Validate(wrong task group manifest) problem = %#v", problem)
		}

		env.writeWorkflowFile(
			t,
			initiative,
			filepath.Join("_task_groups", "TG-001", "task_01.md"),
			daemonTaskBody("pending", "Sibling task group task"),
		)
		env.writeWorkflowFile(
			t,
			initiative,
			filepath.Join("_task_groups", "TG-002", "_tasks.md"),
			strings.Replace(
				taskGroupTaskGraphManifest("watcher/TG-002"),
				"file: task_01.md",
				"file: ../TG-001/task_01.md",
				1,
			),
		)
		_, err = service.Validate(context.Background(), env.workspaceRoot, "watcher/TG-002")
		var taskGroupErr *taskgroups.Error
		if !errors.As(err, &taskGroupErr) || !errors.Is(err, taskgroups.ErrInvalidPlan) {
			t.Fatalf("Validate(escaped task group manifest) error = %v, want invalid task group manifest", err)
		}
		if len(taskGroupErr.Issues) != 1 || !strings.Contains(taskGroupErr.Issues[0].Message, "sibling-ownership") {
			t.Fatalf("Validate(escaped task group manifest) issues = %#v", taskGroupErr.Issues)
		}
	})

	t.Run("Should archive workflows and surface archived reads", func(t *testing.T) {
		env, service := newService(t)
		env.writeWorkflowFile(t, env.workflowSlug, "task_01.md", daemonTaskBody("completed", "Transport task"))
		syncWorkflowForDaemonTest(t, env)
		archiveResult, err := service.Archive(
			context.Background(),
			env.workspaceRoot,
			env.workflowSlug,
			apicore.ArchiveRequest{},
		)
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

	t.Run("Should refresh stale empty workflow state before archiving review-only workflows", func(t *testing.T) {
		testCases := []struct {
			name         string
			slug         string
			reviewStatus string
			wantArchived bool
			wantProblem  bool
		}{
			{
				name:         "Should archive resolved review-only workflow",
				slug:         "review-only-resolved-stale",
				reviewStatus: "resolved",
				wantArchived: true,
			},
			{
				name:         "Should require force for unresolved review-only workflow",
				slug:         "review-only-pending-stale",
				reviewStatus: "pending",
				wantProblem:  true,
			},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				env := newRunManagerTestEnv(t, runManagerTestDeps{})
				if err := os.MkdirAll(env.workflowDir(tc.slug), 0o755); err != nil {
					t.Fatalf("mkdir workflow dir: %v", err)
				}
				syncNamedWorkflowForDaemonTest(t, env, tc.slug)
				env.writeWorkflowFile(
					t,
					tc.slug,
					filepath.Join("reviews-001", "issue_001.md"),
					daemonReviewIssueBody(tc.reviewStatus, "medium"),
				)

				service := newTransportTaskService(env.globalDB, env.manager)
				result, err := service.Archive(
					context.Background(),
					env.workspaceRoot,
					tc.slug,
					apicore.ArchiveRequest{},
				)
				if tc.wantProblem {
					var problem *apicore.Problem
					if !errors.As(err, &problem) {
						t.Fatalf("Archive() error = %v, want transport problem", err)
					}
					if problem.Code != string(contract.CodeWorkflowForceRequired) {
						t.Fatalf(
							"Archive() problem code = %q, want %q",
							problem.Code,
							contract.CodeWorkflowForceRequired,
						)
					}
					if got := problem.Details["review_unresolved"]; got != 1 {
						t.Fatalf("review_unresolved = %#v, want 1", got)
					}
					return
				}

				if err != nil {
					t.Fatalf("Archive() error = %v", err)
				}
				if result.Archived != tc.wantArchived {
					t.Fatalf("Archive().Archived = %v, want %v", result.Archived, tc.wantArchived)
				}
			})
		}
	})

	t.Run("Should surface force-required archive conflicts and map forced success counts", func(t *testing.T) {
		env, service := newService(t)
		env.writeWorkflowFile(t, env.workflowSlug, "task_01.md", daemonTaskBody("pending", "Transport task"))
		env.writeWorkflowFile(
			t,
			env.workflowSlug,
			filepath.Join("reviews-001", "issue_001.md"),
			daemonReviewIssueBody("pending", "high"),
		)
		syncWorkflowForDaemonTest(t, env)

		_, err := service.Archive(context.Background(), env.workspaceRoot, env.workflowSlug, apicore.ArchiveRequest{})
		var problem *apicore.Problem
		if !errors.As(err, &problem) {
			t.Fatalf("Archive() error = %v, want transport problem", err)
		}
		if problem.Status != 409 || problem.Code != string(contract.CodeWorkflowForceRequired) {
			t.Fatalf("unexpected archive problem: %#v", problem)
		}
		if got := problem.Details["task_non_terminal"]; got != 1 {
			t.Fatalf("task_non_terminal = %#v, want 1", got)
		}
		if got := problem.Details["review_unresolved"]; got != 1 {
			t.Fatalf("review_unresolved = %#v, want 1", got)
		}

		result, err := service.Archive(
			context.Background(),
			env.workspaceRoot,
			env.workflowSlug,
			apicore.ArchiveRequest{Force: true},
		)
		if err != nil {
			t.Fatalf("Archive(force) error = %v", err)
		}
		if !result.Archived || !result.Forced {
			t.Fatalf("unexpected forced archive result: %#v", result)
		}
		if result.CompletedTasks != 1 || result.ResolvedReviewIssues != 1 {
			t.Fatalf("unexpected forced archive counts: %#v", result)
		}
	})

	t.Run("Should report unavailable workflow listing and archiving without a database", func(t *testing.T) {
		env, _ := newService(t)
		nilDBService := newTransportTaskService(nil, env.manager)
		if _, err := nilDBService.ListWorkflows(context.Background(), env.workspaceRoot); err == nil ||
			!strings.Contains(err.Error(), "workflow listing is not available") {
			t.Fatalf("nil ListWorkflows() error = %v, want unavailable", err)
		}
		if _, err := nilDBService.Archive(
			context.Background(),
			env.workspaceRoot,
			env.workflowSlug,
			apicore.ArchiveRequest{},
		); err == nil ||
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

func TestTaskTransportService_ShouldProjectAndArchiveTaskGroupInitiativesAsRoots(t *testing.T) {
	// Suite boundary
	// IN: sync and archive transport calls backed by the real global catalog
	// OUT: task group execution and picker behavior
	// Invariant: API reads nest hidden children, and task-group-only archive targets are rejected.
	// CONTRACT: IT-055.
	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	t.Setenv("HOME", env.homeDir)
	initiative := "watcher"
	env.writeWorkflowFile(t, initiative, "_task_groups.md", daemonTaskGroupPlan("x"))
	env.writeWorkflowFile(
		t,
		initiative,
		filepath.Join("_task_groups", "TG-001", "task_01.md"),
		daemonTaskBody("completed", "task group task"),
	)

	syncService := newTransportSyncService(env.globalDB)
	syncResult, err := syncService.Sync(context.Background(), apicore.SyncRequest{
		Workspace:    env.workspaceRoot,
		WorkflowSlug: initiative,
	})
	if err != nil {
		t.Fatalf("Sync(initiative): %v", err)
	}
	if syncResult.WorkflowsScanned != 2 || len(syncResult.TaskGroupChildIDs) != 1 || syncResult.Partial {
		t.Fatalf("Sync(initiative) result = %#v, want root plus one complete child", syncResult)
	}
	_, err = syncService.Sync(context.Background(), apicore.SyncRequest{
		Workspace:    env.workspaceRoot,
		WorkflowSlug: "watcher/TG-001",
	})
	if !errors.Is(err, corepkg.ErrTaskGroupRootOnly) {
		t.Fatalf("Sync(task group target) error = %v, want root-only error", err)
	}

	service := newTransportTaskService(env.globalDB, env.manager)
	workflows, err := service.ListWorkflows(context.Background(), env.workspaceRoot)
	if err != nil {
		t.Fatalf("ListWorkflows(initiative): %v", err)
	}
	if len(workflows) != 1 || workflows[0].Slug != initiative || workflows[0].Kind != "initiative" {
		t.Fatalf("initiative workflow list = %#v", workflows)
	}
	if workflows[0].CanStartRun == nil || *workflows[0].CanStartRun ||
		workflows[0].StartBlockReason != "select a task group" {
		t.Fatalf("initiative start action = %#v, want task group selection required", workflows[0])
	}
	if workflows[0].ArchiveEligible == nil || !*workflows[0].ArchiveEligible || workflows[0].ArchiveReason != "" {
		t.Fatalf("initiative aggregate archive action = %#v, want eligible", workflows[0])
	}
	if len(workflows[0].TaskGroups) != 1 {
		t.Fatalf("initiative task group summary = %#v, want one hidden child", workflows[0].TaskGroups)
	}
	child := workflows[0].TaskGroups[0]
	if child.TaskGroupID != "TG-001" || child.Reference != "watcher/TG-001" || !child.LifecycleComplete ||
		child.TaskCounts == nil || child.TaskCounts.Completed != 1 || child.UnmetDependencyCount != 0 ||
		child.IndependentlyEligible || child.CanStartRun == nil || *child.CanStartRun {
		t.Fatalf("nested task group summary = %#v", child)
	}

	_, err = service.Archive(
		context.Background(),
		env.workspaceRoot,
		"watcher/TG-001",
		apicore.ArchiveRequest{},
	)
	if !errors.Is(err, corepkg.ErrTaskGroupRootOnly) {
		t.Fatalf("Archive(task group target) error = %v, want root-only error", err)
	}
	archiveResult, err := service.Archive(
		context.Background(),
		env.workspaceRoot,
		initiative,
		apicore.ArchiveRequest{},
	)
	if err != nil {
		t.Fatalf("Archive(initiative): %v", err)
	}
	if !archiveResult.Archived || len(archiveResult.TaskGroupChildIDs) != 1 {
		t.Fatalf("Archive(initiative) result = %#v, want one root archive and one child id", archiveResult)
	}
}

func TestTaskTransportService_ShouldRequireConfirmationForTransitiveDependencyAfterPrerequisiteReopens(t *testing.T) {
	// Suite boundary
	// IN: workflow sync, global catalog, and nested transport projection
	// OUT: execution authorization, which owns detailed override handling
	// Invariant: a task group with an unmet transitive prerequisite requires explicit authorization.
	// CONTRACT: IT-055.
	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	t.Setenv("HOME", env.homeDir)
	initiative := "reopened-dependencies"
	plan, err := taskgroups.RenderPlan(taskgroups.Plan{
		SchemaVersion: taskgroups.SchemaVersion,
		Initiative:    initiative,
		TaskGroups: []taskgroups.TaskGroup{
			{
				ID:         "TG-001",
				Title:      "Foundation",
				Outcome:    "Provide the prerequisite",
				Directory:  "_task_groups/TG-001",
				OwnedScope: []string{"foundation"},
			},
			{
				ID:         "TG-002",
				Title:      "Delivery",
				Outcome:    "Use the prerequisite",
				Directory:  "_task_groups/TG-002",
				Completed:  true,
				OwnedScope: []string{"delivery"},
			},
			{
				ID:         "TG-003",
				Title:      "Notifications",
				Outcome:    "Use the completed direct prerequisite",
				Directory:  "_task_groups/TG-003",
				OwnedScope: []string{"notifications"},
			},
		},
		Edges: []taskgroups.Dependency{
			{From: "TG-001", To: "TG-002", Rationale: "Foundation must be complete first"},
			{From: "TG-002", To: "TG-003", Rationale: "Delivery must be complete first"},
		},
	})
	if err != nil {
		t.Fatalf("RenderPlan() error = %v", err)
	}
	env.writeWorkflowFile(t, initiative, "_prd.md", "# Canonical PRD\n")
	env.writeWorkflowFile(t, initiative, "_techspec.md", "# Canonical TechSpec\n")
	env.writeWorkflowFile(t, initiative, "_task_groups.md", string(plan))
	env.writeWorkflowFile(
		t,
		initiative,
		filepath.Join("_task_groups", "TG-001", "task_01.md"),
		daemonTaskBody("pending", "Task Group foundation task"),
	)
	env.writeWorkflowFile(
		t,
		initiative,
		filepath.Join("_task_groups", "TG-002", "task_01.md"),
		daemonTaskBody("completed", "Task Group delivery task"),
	)
	env.writeWorkflowFile(
		t,
		initiative,
		filepath.Join("_task_groups", "TG-003", "task_01.md"),
		daemonTaskBody("pending", "Task Group notification task"),
	)
	syncNamedWorkflowForDaemonTest(t, env, initiative)

	workflows, err := newTransportTaskService(env.globalDB, env.manager).ListWorkflows(
		context.Background(),
		env.workspaceRoot,
	)
	if err != nil {
		t.Fatalf("ListWorkflows() error = %v", err)
	}
	if len(workflows) != 1 || len(workflows[0].TaskGroups) != 3 {
		t.Fatalf("ListWorkflows() = %#v, want one initiative with three task groups", workflows)
	}

	var notification apicore.TaskGroupSummary
	for _, taskGroup := range workflows[0].TaskGroups {
		if taskGroup.TaskGroupID == "TG-003" {
			notification = taskGroup
			break
		}
	}
	if notification.TaskGroupID == "" {
		t.Fatalf("TG-003 summary missing from %#v", workflows[0].TaskGroups)
	}
	if notification.UnmetDependencyCount != 1 || notification.CanStartRun == nil ||
		!*notification.CanStartRun || !notification.RequiresStartConfirmation ||
		notification.StartBlockReason != "" || len(notification.UnmetDependencies) != 0 ||
		len(notification.UnmetDependencyPaths) != 1 ||
		!slices.Equal(notification.UnmetDependencyPaths[0].TaskGroupIDs, []string{"TG-001", "TG-002"}) ||
		len(notification.UnmetDependencyPaths[0].Dependencies) != 1 ||
		notification.UnmetDependencyPaths[0].Dependencies[0].TaskGroupID != "TG-001" ||
		notification.UnmetDependencyPaths[0].Dependencies[0].Title != "Foundation" ||
		notification.UnmetDependencyPaths[0].Dependencies[0].Rationale != "Foundation must be complete first" {
		t.Fatalf("TG-003 readiness = %#v, want one transitive blocker and start confirmation", notification)
	}
}

func syncWorkflowForDaemonTest(t *testing.T, env *runManagerTestEnv) {
	t.Helper()

	syncNamedWorkflowForDaemonTest(t, env, env.workflowSlug)
}

func taskGroupTaskGraphManifest(workflow string) string {
	return strings.Join([]string{
		"---",
		"schema_version: \"compozy.tasks/v2\"",
		"workflow: " + workflow,
		"graph:",
		"  nodes:",
		"    - id: task_01",
		"      file: task_01.md",
		"  edges: []",
		"---",
		"# Task Graph",
	}, "\n")
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
