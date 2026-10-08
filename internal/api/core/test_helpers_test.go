package core_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/testutil"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
)

type handlerFixture struct {
	Handlers  *core.BaseHandlers
	Engine    *gin.Engine
	HomePaths compozyconfig.HomePaths
}

type noOpAgentDefinitionSync struct{}

func (noOpAgentDefinitionSync) Sync(context.Context) error { return nil }

func testConfigForTest(homePaths compozyconfig.HomePaths) compozyconfig.Config {
	return testutil.ConfigForTest(homePaths)
}

func defaultCoreWorkspaceService(workspaces testutil.StubWorkspaceService) testutil.StubWorkspaceService {
	originalResolve := workspaces.ResolveFn
	workspaces.ResolveFn = func(ctx context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
		if originalResolve != nil {
			resolved, err := originalResolve(ctx, ref)
			if err != nil {
				return workspacepkg.ResolvedWorkspace{}, err
			}
			return normalizeCoreResolvedWorkspace(ref, &resolved), nil
		}
		return normalizeCoreResolvedWorkspace(ref, nil), nil
	}
	return workspaces
}

func normalizeCoreResolvedWorkspace(
	ref string,
	resolved *workspacepkg.ResolvedWorkspace,
) workspacepkg.ResolvedWorkspace {
	if resolved == nil {
		resolved = &workspacepkg.ResolvedWorkspace{}
	}
	workspaceID := strings.TrimSpace(resolved.WorkspaceID)
	if workspaceID == "" {
		workspaceID = strings.TrimSpace(ref)
	}
	if workspaceID == "" {
		workspaceID = strings.TrimSpace(resolved.ID)
	}
	if workspaceID == "" {
		return workspacepkg.ResolvedWorkspace{}
	}
	resolved.WorkspaceID = workspaceID
	if strings.TrimSpace(resolved.ID) == "" {
		resolved.ID = workspaceID
	}
	if strings.TrimSpace(resolved.Name) == "" {
		resolved.Name = workspaceID
	}
	if strings.TrimSpace(resolved.RootDir) == "" {
		if filepath.IsAbs(strings.TrimSpace(ref)) {
			resolved.RootDir = strings.TrimSpace(ref)
		} else {
			resolved.RootDir = "/workspace"
		}
	}
	return *resolved
}

func defaultCoreSessionManager(manager testutil.StubSessionManager) testutil.StubSessionManager {
	if manager.StatusFn != nil || manager.ListAllFn == nil {
		return manager
	}
	manager.StatusFn = func(ctx context.Context, id string) (*session.Info, error) {
		infos, err := manager.ListAllFn(ctx)
		if err != nil {
			return nil, err
		}
		for _, info := range infos {
			if info != nil && strings.TrimSpace(info.ID) == strings.TrimSpace(id) {
				return info, nil
			}
		}
		return nil, session.ErrSessionNotFound
	}
	return manager
}

func newHandlerFixture(
	t *testing.T,
	manager testutil.StubSessionManager,
	observer testutil.StubObserver,
	workspaces testutil.StubWorkspaceService,
) handlerFixture {
	return newHandlerFixtureWithAutomationAndTasks(
		t,
		manager,
		observer,
		testutil.StubAutomationManager{},
		&testutil.StubTaskManager{},
		workspaces,
	)
}

func newHandlerFixtureWithAutomation(
	t *testing.T,
	manager testutil.StubSessionManager,
	observer testutil.StubObserver,
	automation testutil.StubAutomationManager,
	workspaces testutil.StubWorkspaceService,
) handlerFixture {
	return newHandlerFixtureWithAutomationAndTasks(
		t,
		manager,
		observer,
		automation,
		&testutil.StubTaskManager{},
		workspaces,
	)
}

func newHandlerFixtureWithTasks(
	t *testing.T,
	manager testutil.StubSessionManager,
	observer testutil.StubObserver,
	tasks *testutil.StubTaskManager,
	workspaces testutil.StubWorkspaceService,
) handlerFixture {
	return newHandlerFixtureWithAutomationAndTasks(
		t,
		manager,
		observer,
		testutil.StubAutomationManager{},
		tasks,
		workspaces,
	)
}

func newHandlerFixtureWithAutomationAndTasks(
	t *testing.T,
	manager testutil.StubSessionManager,
	observer testutil.StubObserver,
	automation testutil.StubAutomationManager,
	tasks *testutil.StubTaskManager,
	workspaces testutil.StubWorkspaceService,
) handlerFixture {
	return newHandlerFixtureWithRuntime(
		t,
		manager,
		observer,
		automation,
		tasks,
		workspaces,
	)
}

func newHandlerFixtureWithRuntime(
	t *testing.T,
	manager testutil.StubSessionManager,
	observer testutil.StubObserver,
	automation testutil.StubAutomationManager,
	tasks *testutil.StubTaskManager,

	workspaces testutil.StubWorkspaceService,
) handlerFixture {
	t.Helper()
	manager = defaultCoreSessionManager(manager)
	workspaces = defaultCoreWorkspaceService(workspaces)

	gin.SetMode(gin.TestMode)
	homePaths := testutil.NewTestHomePaths(t)
	cfg := testConfigForTest(homePaths)
	cfg.HTTP.Host = "127.0.0.1"
	cfg.HTTP.Port = 2123
	cfg.Daemon.Socket = "/tmp/api-core-test.sock"

	handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
		TransportName:                "api-core-test",
		MaskInternalErrors:           false,
		IncludeSessionWorkspaceInSSE: true,
		Sessions:                     manager,
		SessionAcceptance:            manager,
		SessionCatalog:               manager,
		Observer:                     observer,
		Automation:                   automation,
		Tasks:                        tasks,

		Workspaces:          workspaces,
		AgentDefinitionSync: noOpAgentDefinitionSync{},
		HomePaths:           homePaths,
		Config:              cfg,
		Logger:              testutil.DiscardLogger(),
		StartedAt:           time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
		Now: func() time.Time {
			return time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC)
		},
		PollInterval: 5 * time.Millisecond,
		HTTPPort:     cfg.HTTP.Port,
	})

	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.GET("/sessions", handlers.ListSessions)
	engine.GET("/sessions/catalog-stream", handlers.StreamSessionCatalog)
	engine.GET("/sessions/attention-summary", handlers.SessionAttentionSummary)
	engine.GET("/sessions/:session_id", handlers.GetSessionByID)
	engine.GET("/sessions/:session_id/owner", handlers.GetSessionOwner)
	engine.GET("/sessions/:session_id/status", handlers.GetSessionStatus)
	engine.GET("/sessions/:session_id/events", handlers.SessionEvents)
	engine.GET("/sessions/:session_id/history", handlers.SessionHistory)
	engine.GET("/sessions/:session_id/stream", handlers.StreamSession)
	engine.GET("/sessions/:session_id/transcript", handlers.SessionTranscript)
	engine.GET("/sessions/:session_id/transcript/search", handlers.SessionTranscriptSearch)
	engine.GET("/sessions/:session_id/transcript/outline", handlers.SessionTranscriptOutline)
	engine.POST("/sessions", handlers.CreateSession)
	engine.GET("/workspaces/:workspace_id/sessions/:session_id", handlers.GetSession)
	engine.PATCH("/workspaces/:workspace_id/sessions/:session_id", handlers.RenameSession)
	engine.GET("/workspaces/:workspace_id/sessions/:session_id/commands", handlers.GetSessionCommands)
	engine.POST("/workspaces/:workspace_id/sessions/:session_id/presence", handlers.SessionPresence)
	engine.POST("/workspaces/:workspace_id/sessions/:session_id/wait", handlers.SessionWait)
	engine.POST("/workspaces/:workspace_id/sessions/:session_id/prompt/cancel", handlers.CancelSessionPrompt)
	engine.GET("/workspaces/:workspace_id/sessions/:session_id/interactions", handlers.ListSessionInteractions)
	engine.DELETE("/workspaces/:workspace_id/sessions/:session_id", handlers.DeleteSession)
	engine.POST("/workspaces/:workspace_id/sessions/:session_id/stop", handlers.StopSession)
	engine.POST("/workspaces/:workspace_id/sessions/:session_id/archive", handlers.ArchiveSession)
	engine.POST("/workspaces/:workspace_id/sessions/:session_id/unarchive", handlers.UnarchiveSession)
	engine.POST("/workspaces/:workspace_id/sessions/:session_id/attach", handlers.AttachSession)
	engine.PUT("/workspaces/:workspace_id/sessions/:session_id/runtime", handlers.SetSessionRuntime)
	engine.DELETE("/workspaces/:workspace_id/sessions/:session_id/runtime", handlers.ClearSessionRuntime)
	engine.GET("/workspaces/:workspace_id/sessions/:session_id/recap", handlers.SessionRecap)
	engine.GET("/workspaces/:workspace_id/sessions/:session_id/usage", handlers.SessionUsage)
	engine.GET("/workspaces/:workspace_id/sessions/:session_id/usage/turns", handlers.SessionUsageTurns)
	engine.POST("/workspaces/:workspace_id/sessions/:session_id/repair", handlers.RepairSession)
	engine.GET("/workspaces/:workspace_id/sessions/:session_id/events", handlers.SessionEvents)
	engine.GET("/workspaces/:workspace_id/sessions/:session_id/history", handlers.SessionHistory)
	engine.GET("/workspaces/:workspace_id/sessions/:session_id/transcript", handlers.SessionTranscript)
	engine.GET("/workspaces/:workspace_id/sessions/:session_id/stream", handlers.StreamSession)
	engine.GET("/agents", handlers.ListAgents)
	engine.GET("/agents/catalog", handlers.ListAgentCatalog)
	engine.POST("/agents", handlers.CreateAgent)
	engine.PUT("/agents/:name", handlers.UpdateAgent)
	engine.DELETE("/agents/:name", handlers.DeleteAgent)
	engine.POST("/agents/:name/duplicate", handlers.DuplicateAgent)
	engine.GET("/agents/:name", handlers.GetAgent)
	engine.GET("/hooks/catalog", handlers.HookCatalog)
	engine.GET("/workspaces/:workspace_id/hooks/runs", handlers.HookRuns)
	engine.GET("/hooks/events", handlers.HookEvents)
	engine.GET("/logs", handlers.ListLogs)
	engine.GET("/logs/stream", handlers.StreamLogs)
	engine.GET("/status", handlers.GetStatus)
	engine.GET("/doctor", handlers.GetDoctor)
	engine.POST("/drain", handlers.DrainDaemon)
	engine.POST("/undrain", handlers.UndrainDaemon)
	engine.GET("/automation/jobs", handlers.ListAutomationJobs)
	engine.POST("/automation/jobs", handlers.CreateAutomationJob)
	engine.GET("/automation/jobs/:id", handlers.GetAutomationJob)
	engine.PATCH("/automation/jobs/:id", handlers.UpdateAutomationJob)
	engine.DELETE("/automation/jobs/:id", handlers.DeleteAutomationJob)
	engine.POST("/automation/jobs/:id/trigger", handlers.TriggerAutomationJob)
	engine.GET("/automation/jobs/:id/runs", handlers.AutomationJobRuns)
	engine.GET("/automation/triggers", handlers.ListAutomationTriggers)
	engine.POST("/automation/triggers", handlers.CreateAutomationTrigger)
	engine.GET("/automation/triggers/:id", handlers.GetAutomationTrigger)
	engine.PATCH("/automation/triggers/:id", handlers.UpdateAutomationTrigger)
	engine.DELETE("/automation/triggers/:id", handlers.DeleteAutomationTrigger)
	engine.GET("/automation/triggers/:id/runs", handlers.AutomationTriggerRuns)
	engine.GET("/automation/runs", handlers.ListAutomationRuns)
	engine.GET("/automation/runs/:id", handlers.GetAutomationRun)
	engine.POST("/webhooks/global/:endpoint", handlers.DeliverGlobalWebhook)
	engine.POST("/webhooks/workspaces/:workspace_id/:endpoint", handlers.DeliverWorkspaceWebhook)

	engine.GET("/tasks", handlers.ListTasks)
	engine.POST("/tasks", handlers.CreateTask)
	engine.GET("/tasks/:id", handlers.GetTask)
	engine.DELETE("/tasks/:id", handlers.DeleteTask)
	engine.PATCH("/tasks/:id", handlers.UpdateTask)
	engine.GET("/tasks/:id/execution-profile", handlers.GetTaskExecutionProfile)
	engine.PUT("/tasks/:id/execution-profile", handlers.SetTaskExecutionProfile)
	engine.PATCH("/tasks/:id/execution-profile/worktree", handlers.SetTaskWorktreePolicy)
	engine.DELETE("/tasks/:id/execution-profile", handlers.DeleteTaskExecutionProfile)

	engine.GET("/tasks/:id/reviews", handlers.ListTaskReviews)
	engine.POST("/tasks/:id/publish", handlers.PublishTask)
	engine.POST("/tasks/:id/start", handlers.StartTask)
	engine.POST("/tasks/:id/cancel", handlers.CancelTask)
	engine.POST("/tasks/:id/pause", handlers.PauseTask)
	engine.POST("/tasks/:id/resume", handlers.ResumeTask)
	engine.POST("/tasks/:id/children", handlers.CreateChildTask)
	engine.POST("/tasks/:id/dependencies", handlers.AddTaskDependency)
	engine.DELETE("/tasks/:id/dependencies/:depends_on_id", handlers.RemoveTaskDependency)
	engine.GET("/tasks/:id/runs", handlers.ListTaskRuns)
	engine.GET("/tasks/:id/inspect", handlers.InspectTask)
	engine.GET("/tasks/:id/timeline", handlers.TaskTimeline)
	engine.GET("/tasks/:id/stream", handlers.StreamTask)
	engine.GET("/tasks/:id/tree", handlers.TaskTree)
	engine.POST("/tasks/:id/approve", handlers.ApproveTask)
	engine.POST("/tasks/:id/reject", handlers.RejectTask)
	engine.POST("/tasks/:id/triage/read", handlers.MarkTaskRead)
	engine.POST("/tasks/:id/triage/archive", handlers.ArchiveTask)
	engine.POST("/tasks/:id/triage/dismiss", handlers.DismissTask)
	engine.POST("/tasks/:id/runs", handlers.EnqueueTaskRun)
	engine.POST("/tasks/:id/runs/fan-out", handlers.FanOutTaskRuns)
	engine.GET("/task-runs/:id", handlers.GetTaskRun)
	engine.GET("/task-runs/:id/result", handlers.ReadTaskRunResult)

	engine.GET("/runs/:id/inspect", handlers.InspectRun)
	engine.POST("/runs/:id/release", handlers.ForceReleaseTaskRun)
	engine.POST("/runs/:id/fail", handlers.ForceFailTaskRun)
	engine.POST("/runs/:id/retry", handlers.RetryTaskRun)
	engine.POST("/runs/bulk/release", handlers.BulkForceReleaseTaskRuns)
	engine.POST("/runs/bulk/fail", handlers.BulkForceFailTaskRuns)
	engine.GET("/scheduler", handlers.GetScheduler)
	engine.POST("/scheduler/pause", handlers.PauseScheduler)
	engine.POST("/scheduler/resume", handlers.ResumeScheduler)
	engine.POST("/scheduler/drain", handlers.DrainScheduler)
	engine.GET("/scheduler/backlog", handlers.GetSchedulerBacklog)
	engine.POST("/task-runs/:id/start", handlers.StartTaskRun)
	engine.POST("/task-runs/:id/attach-session", handlers.AttachTaskRunSession)
	engine.POST("/task-runs/:id/complete", handlers.CompleteTaskRun)
	engine.POST("/task-runs/:id/fail", handlers.FailTaskRun)
	engine.POST("/task-runs/:id/cancel", handlers.CancelTaskRun)
	engine.POST("/task-runs/:id/reviews", handlers.RequestTaskRunReview)
	engine.GET("/task-runs/:id/reviews", handlers.ListTaskRunReviews)
	engine.GET("/task-reviews/:id", handlers.GetTaskRunReview)
	engine.POST("/task-reviews/:id/verdict", handlers.SubmitTaskRunReviewVerdict)
	engine.GET("/observe/overview", handlers.ObserveOverview)
	engine.GET("/observe/tasks/dashboard", handlers.TaskDashboard)
	engine.GET("/observe/tasks/inbox", handlers.TaskInbox)
	engine.POST("/workspaces", handlers.CreateWorkspace)
	engine.GET("/workspaces", handlers.ListWorkspaces)
	engine.GET("/workspaces/:workspace_id", handlers.GetWorkspace)
	engine.PATCH("/workspaces/:workspace_id", handlers.UpdateWorkspace)
	engine.DELETE("/workspaces/:workspace_id", handlers.DeleteWorkspace)
	engine.POST("/workspaces/resolve", handlers.ResolveWorkspace)

	return handlerFixture{
		Handlers:  handlers,
		Engine:    engine,
		HomePaths: homePaths,
	}
}

func performRequest(t *testing.T, engine http.Handler, method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	return testutil.PerformRequest(t, engine, method, path, body)
}

func testSessionManager(
	workspaceID string,
	sessionIDs ...string,
) testutil.StubSessionManager {
	allowed := make(map[string]struct{}, len(sessionIDs))
	for _, id := range sessionIDs {
		allowed[strings.TrimSpace(id)] = struct{}{}
	}
	return testutil.StubSessionManager{
		StatusFn: func(_ context.Context, id string) (*session.Info, error) {
			trimmedID := strings.TrimSpace(id)
			if _, ok := allowed[trimmedID]; !ok {
				return nil, session.ErrSessionNotFound
			}
			return &session.Info{
				ID:          trimmedID,
				ProfileID:   store.DefaultProfileID,
				WorkspaceID: strings.TrimSpace(workspaceID),
				State:       session.StateActive,
			}, nil
		},
	}
}
