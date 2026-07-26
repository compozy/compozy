package core_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/api/testutil"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

func testLiveParticipation(workspaceID string, channelID string) participation.Spec {
	return participation.Spec{
		Version:         participation.SpecVersion,
		Mode:            participation.ModeLive,
		WorkspaceID:     workspaceID,
		ChannelStrategy: participation.StrategyNamed,
		ChannelID:       channelID,
		Source:          participation.SourceExplicitRequest,
		Bounds: participation.Bounds{
			MaxWakes:         4,
			MaxWakeWallTime:  "30s",
			MaxTotalWallTime: "2m",
			MaxInputTokens:   4096,
			MaxOutputTokens:  4096,
			MaxWakeDepth:     4,
			CoalesceWindow:   "250ms",
		},
	}
}

func testNamedParticipationRequest(channelID string) *participation.Request {
	mode := participation.ModeLive
	strategy := participation.StrategyNamed
	return &participation.Request{
		Mode:            &mode,
		ChannelStrategy: &strategy,
		ChannelID:       &channelID,
	}
}

func testParticipationRequestChannel(request *participation.Request) string {
	if request == nil || request.ChannelID == nil {
		return ""
	}
	return strings.TrimSpace(*request.ChannelID)
}

type stubDreamTrigger struct {
	Triggered bool
	Reason    string
	Err       error
	Last      time.Time
	LastErr   error
	EnabledFn bool
	Calls     int
	Workspace string
}

func (s *stubDreamTrigger) Trigger(_ context.Context, workspace string) (bool, string, error) {
	s.Calls++
	s.Workspace = workspace
	return s.Triggered, s.Reason, s.Err
}

func (s *stubDreamTrigger) LastConsolidatedAt() (time.Time, error) {
	return s.Last, s.LastErr
}

func (s *stubDreamTrigger) Enabled() bool {
	return s.EnabledFn
}

type handlerFixture struct {
	Handlers  *core.BaseHandlers
	Engine    *gin.Engine
	HomePaths aghconfig.HomePaths
}

type noOpAgentDefinitionSync struct{}

func (noOpAgentDefinitionSync) Sync(context.Context) error { return nil }

func testConfigWithDisabledNetwork(homePaths aghconfig.HomePaths) aghconfig.Config {
	return testutil.ConfigWithDisabledNetwork(homePaths)
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
	store *memory.Store,
	dream core.DreamTrigger,
) handlerFixture {
	return newHandlerFixtureWithAutomationAndTasks(
		t,
		manager,
		observer,
		testutil.StubAutomationManager{},
		&testutil.StubTaskManager{},
		workspaces,
		store,
		dream,
	)
}

func newHandlerFixtureWithAutomation(
	t *testing.T,
	manager testutil.StubSessionManager,
	observer testutil.StubObserver,
	automation testutil.StubAutomationManager,
	workspaces testutil.StubWorkspaceService,
	store *memory.Store,
	dream core.DreamTrigger,
) handlerFixture {
	return newHandlerFixtureWithAutomationAndTasks(
		t,
		manager,
		observer,
		automation,
		&testutil.StubTaskManager{},
		workspaces,
		store,
		dream,
	)
}

func newHandlerFixtureWithTasks(
	t *testing.T,
	manager testutil.StubSessionManager,
	observer testutil.StubObserver,
	tasks *testutil.StubTaskManager,
	workspaces testutil.StubWorkspaceService,
	store *memory.Store,
	dream core.DreamTrigger,
) handlerFixture {
	return newHandlerFixtureWithAutomationAndTasks(
		t,
		manager,
		observer,
		testutil.StubAutomationManager{},
		tasks,
		workspaces,
		store,
		dream,
	)
}

func newHandlerFixtureWithTasksAndBridges(
	t *testing.T,
	manager testutil.StubSessionManager,
	observer testutil.StubObserver,
	tasks *testutil.StubTaskManager,
	bridges testutil.StubBridgeService,
	workspaces testutil.StubWorkspaceService,
	store *memory.Store,
	dream core.DreamTrigger,
) handlerFixture {
	return newHandlerFixtureWithAutomationTasksAndBridges(
		t,
		manager,
		observer,
		testutil.StubAutomationManager{},
		tasks,
		bridges,
		workspaces,
		store,
		dream,
	)
}

func newHandlerFixtureWithAutomationAndTasks(
	t *testing.T,
	manager testutil.StubSessionManager,
	observer testutil.StubObserver,
	automation testutil.StubAutomationManager,
	tasks *testutil.StubTaskManager,
	workspaces testutil.StubWorkspaceService,
	store *memory.Store,
	dream core.DreamTrigger,
) handlerFixture {
	return newHandlerFixtureWithAutomationTasksAndBridges(
		t,
		manager,
		observer,
		automation,
		tasks,
		testutil.StubBridgeService{},
		workspaces,
		store,
		dream,
	)
}

func newHandlerFixtureWithAutomationTasksAndBridges(
	t *testing.T,
	manager testutil.StubSessionManager,
	observer testutil.StubObserver,
	automation testutil.StubAutomationManager,
	tasks *testutil.StubTaskManager,
	bridges testutil.StubBridgeService,
	workspaces testutil.StubWorkspaceService,
	store *memory.Store,
	dream core.DreamTrigger,
) handlerFixture {
	t.Helper()
	manager = defaultCoreSessionManager(manager)
	workspaces = defaultCoreWorkspaceService(workspaces)

	gin.SetMode(gin.TestMode)
	homePaths := testutil.NewTestHomePaths(t)
	cfg := testConfigWithDisabledNetwork(homePaths)
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
		Bridges:                      bridges,
		Workspaces:                   workspaces,
		AgentDefinitionSync:          noOpAgentDefinitionSync{},
		MemoryStore:                  store,
		DreamTrigger:                 dream,
		HomePaths:                    homePaths,
		Config:                       cfg,
		Logger:                       testutil.DiscardLogger(),
		StartedAt:                    time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
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
	engine.GET("/sessions/:session_id", handlers.GetSessionByID)
	engine.POST("/sessions", handlers.CreateSession)
	engine.GET("/workspaces/:workspace_id/sessions/:session_id", handlers.GetSession)
	engine.DELETE("/workspaces/:workspace_id/sessions/:session_id", handlers.DeleteSession)
	engine.POST("/workspaces/:workspace_id/sessions/:session_id/stop", handlers.StopSession)
	engine.POST("/workspaces/:workspace_id/sessions/:session_id/attach", handlers.AttachSession)
	engine.GET("/workspaces/:workspace_id/sessions/:session_id/recap", handlers.SessionRecap)
	engine.GET("/workspaces/:workspace_id/sessions/:session_id/usage", handlers.SessionUsage)
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
	engine.GET("/network/status", handlers.NetworkStatus)
	engine.GET("/workspaces/:workspace_id/network/usage", handlers.GetNetworkUsage)
	engine.GET("/workspaces/:workspace_id/network-coordination", handlers.GetNetworkCoordination)
	engine.PUT("/workspaces/:workspace_id/network-coordination", handlers.PutNetworkCoordination)
	engine.PUT(
		"/workspaces/:workspace_id/network-coordination/invitation",
		handlers.PutNetworkCoordinationInvitation,
	)
	engine.GET("/workspaces/:workspace_id/network/peers", handlers.NetworkPeers)
	engine.GET("/workspaces/:workspace_id/network/peers/:peer_id/messages", handlers.NetworkPeerMessages)
	engine.GET("/workspaces/:workspace_id/network/peers/:peer_id", handlers.NetworkPeer)
	engine.GET("/workspaces/:workspace_id/network/channels", handlers.NetworkChannels)
	engine.POST("/workspaces/:workspace_id/network/channels", handlers.CreateNetworkChannel)
	engine.GET("/workspaces/:workspace_id/network/channels/:channel", handlers.NetworkChannel)
	engine.PATCH("/workspaces/:workspace_id/network/channels/:channel", handlers.UpdateNetworkChannel)
	engine.GET("/workspaces/:workspace_id/network/channels/:channel/subscriptions", handlers.NetworkSubscriptions)
	engine.PUT("/workspaces/:workspace_id/network/channels/:channel/subscriptions", handlers.UpsertNetworkSubscription)
	engine.DELETE(
		"/workspaces/:workspace_id/network/channels/:channel/subscriptions/:session_id",
		handlers.DeleteNetworkSubscription,
	)
	engine.GET("/workspaces/:workspace_id/network/channels/:channel/messages", handlers.NetworkChannelMessages)
	engine.GET("/workspaces/:workspace_id/network/channels/:channel/threads", handlers.NetworkThreads)
	engine.GET("/workspaces/:workspace_id/network/channels/:channel/threads/:thread_id", handlers.NetworkThread)
	engine.POST(
		"/workspaces/:workspace_id/network/channels/:channel/threads/:thread_id/promote-task",
		handlers.PromoteNetworkThreadTask,
	)
	engine.GET(
		"/workspaces/:workspace_id/network/channels/:channel/threads/:thread_id/messages",
		handlers.NetworkThreadMessages,
	)
	engine.GET("/workspaces/:workspace_id/network/channels/:channel/directs", handlers.NetworkDirectRooms)
	engine.POST(
		"/workspaces/:workspace_id/network/channels/:channel/directs/resolve",
		handlers.ResolveNetworkDirectRoom,
	)
	engine.GET("/workspaces/:workspace_id/network/channels/:channel/directs/:direct_id", handlers.NetworkDirectRoom)
	engine.GET(
		"/workspaces/:workspace_id/network/channels/:channel/directs/:direct_id/messages",
		handlers.NetworkDirectRoomMessages,
	)
	engine.GET("/workspaces/:workspace_id/network/work/:work_id", handlers.NetworkWork)
	engine.POST("/workspaces/:workspace_id/network/send", handlers.NetworkSend)
	engine.GET("/workspaces/:workspace_id/network/inbox", handlers.NetworkInbox)
	engine.GET("/tasks", handlers.ListTasks)
	engine.POST("/tasks", handlers.CreateTask)
	engine.GET("/tasks/:id", handlers.GetTask)
	engine.DELETE("/tasks/:id", handlers.DeleteTask)
	engine.PATCH("/tasks/:id", handlers.UpdateTask)
	engine.GET("/tasks/:id/execution-profile", handlers.GetTaskExecutionProfile)
	engine.PUT("/tasks/:id/execution-profile", handlers.SetTaskExecutionProfile)
	engine.DELETE("/tasks/:id/execution-profile", handlers.DeleteTaskExecutionProfile)
	engine.POST("/tasks/:id/notifications/bridges", handlers.CreateTaskBridgeNotificationSubscription)
	engine.GET("/tasks/:id/notifications/bridges", handlers.ListTaskBridgeNotificationSubscriptions)
	engine.GET("/tasks/:id/notifications/bridges/:subscription_id", handlers.GetTaskBridgeNotificationSubscription)
	engine.DELETE(
		"/tasks/:id/notifications/bridges/:subscription_id",
		handlers.DeleteTaskBridgeNotificationSubscription,
	)
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
	engine.GET("/task-runs/:id/conversation/stream", handlers.StreamTaskRunConversation)
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
	engine.GET("/memory", handlers.ListMemory)
	engine.GET("/memory/health", handlers.MemoryHealth)
	engine.GET("/memory/config", handlers.MemoryConfigMetadata)
	engine.GET("/memory/history", handlers.MemoryHistory)
	engine.GET("/memory/scope-show", handlers.MemoryScopeShow)
	engine.POST("/memory", handlers.WriteMemory)
	engine.POST("/memory/search", handlers.SearchMemory)
	engine.POST("/memory/reindex", handlers.ReindexMemory)
	engine.POST("/memory/promote", handlers.PromoteMemory)
	engine.POST("/memory/reset", handlers.ResetMemory)
	engine.POST("/memory/reload", handlers.ReloadMemory)
	engine.GET("/memory/decisions", handlers.ListMemoryDecisions)
	engine.GET("/memory/decisions/:decision_id", handlers.GetMemoryDecision)
	engine.POST("/memory/decisions/:decision_id/revert", handlers.RevertMemoryDecision)
	engine.GET("/memory/recall-traces/:session_id/:turn_seq", handlers.GetMemoryRecallTrace)
	engine.GET("/memory/dreams/status", handlers.GetMemoryDreamStatus)
	engine.GET("/memory/dreams", handlers.ListMemoryDreams)
	engine.POST("/memory/dreams/trigger", handlers.TriggerMemoryDream)
	engine.GET("/memory/dreams/:dream_id", handlers.GetMemoryDream)
	engine.POST("/memory/dreams/:dream_id/retry", handlers.RetryMemoryDream)
	engine.GET("/memory/daily", handlers.ListMemoryDailyLogs)
	engine.GET("/memory/extractor/status", handlers.GetMemoryExtractorStatus)
	engine.GET("/memory/extractor/failures", handlers.ListMemoryExtractorFailures)
	engine.POST("/memory/extractor/retry", handlers.RetryMemoryExtractor)
	engine.POST("/memory/extractor/drain", handlers.DrainMemoryExtractor)
	engine.GET("/memory/providers", handlers.ListMemoryProviders)
	engine.POST("/memory/providers/select", handlers.SelectMemoryProvider)
	engine.GET("/memory/providers/:provider_name", handlers.GetMemoryProvider)
	engine.POST("/memory/providers/:provider_name/enable", handlers.EnableMemoryProvider)
	engine.POST("/memory/providers/:provider_name/disable", handlers.DisableMemoryProvider)
	engine.POST("/memory/ad-hoc", handlers.CreateMemoryAdhocNote)
	engine.GET("/workspaces/:workspace_id/memory/sessions/:session_id/ledger", handlers.GetMemorySessionLedger)
	engine.POST("/workspaces/:workspace_id/memory/sessions/:session_id/replay", handlers.ReplayMemorySession)
	engine.POST("/memory/sessions/prune", handlers.PruneMemorySessions)
	engine.POST("/memory/sessions/repair", handlers.RepairMemorySessions)
	engine.GET("/memory/:filename", handlers.ReadMemory)
	engine.PATCH("/memory/:filename", handlers.EditMemory)
	engine.DELETE("/memory/:filename", handlers.DeleteMemory)
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
