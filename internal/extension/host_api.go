package extensionpkg

import (
	"context"
	"encoding/json"

	"sync"
	"time"

	"github.com/compozy/agh/internal/acp"

	automationpkg "github.com/compozy/agh/internal/automation"

	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/network"

	observepkg "github.com/compozy/agh/internal/observe"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/session"
	skillspkg "github.com/compozy/agh/internal/skills"
	"github.com/compozy/agh/internal/store"

	taskpkg "github.com/compozy/agh/internal/task"

	workspacepkg "github.com/compozy/agh/internal/workspace"
)

const (
	hostAPIAgentNameKey                 = "agent_name"
	hostAPIAutomationJobsDeletePath     = "automation/jobs/delete"
	hostAPIAutomationJobsGetPath        = "automation/jobs/get"
	hostAPIAutomationJobsRunsPath       = "automation/jobs/runs"
	hostAPIAutomationJobsTriggerPath    = "automation/jobs/trigger"
	hostAPIAutomationJobsUpdatePath     = "automation/jobs/update"
	hostAPIAutomationRunsPath           = "automation/runs"
	hostAPIAutomationTriggersPath       = "automation/triggers"
	hostAPIAutomationTriggersCreatePath = "automation/triggers/create"
	hostAPIAutomationTriggersDeletePath = "automation/triggers/delete"
	hostAPIAutomationTriggersFirePath   = "automation/triggers/fire"
	hostAPIAutomationTriggersGetPath    = "automation/triggers/get"
	hostAPIAutomationTriggersRunsPath   = "automation/triggers/runs"
	hostAPIAutomationTriggersUpdatePath = "automation/triggers/update"
	hostAPIBridgesInstancesGetPath      = "bridges/instances/get"
	hostAPIBridgesMessagesIngestPath    = "bridges/messages/ingest"
	hostAPILimitKey                     = "limit"
	hostAPIMemoryStorePath              = "memory/store"
	hostAPIMethodKey                    = "method"
	hostAPIObserveHealthPath            = "observe/health"
	hostAPIResourceKey                  = "resource"
	hostAPIResourcesGetPath             = "resources/get"
	hostAPISandboxInfoPath              = "sandbox/info"
	hostAPISandboxListPath              = "sandbox/list"
	hostAPIScopeKey                     = "scope"
	hostAPISessionIDKey                 = "session_id"
	hostAPISessionsListPath             = "sessions/list"
	hostAPISessionsPromptPath           = "sessions/prompt"
	hostAPISessionsStatusPath           = "sessions/status"
	hostAPISessionsStopPath             = "sessions/stop"
	hostAPIWorkspaceIDKey               = "workspace_id"
)

const (
	// HostAPIRateLimitedCode is the protocol code for per-extension backpressure.
	HostAPIRateLimitedCode = -32002
	// HostAPIUnavailableCode reports a temporarily unavailable Host API resource.
	HostAPIUnavailableCode = -32005
	// HostAPINotFoundCode reports a missing Host API resource.
	HostAPINotFoundCode = -32006
	// HostAPIInvalidParamsCode is the JSON-RPC invalid params code used for bad request payloads.
	HostAPIInvalidParamsCode = -32602
	// HostAPIMethodNotFoundCode is the JSON-RPC method-not-found code for unknown Host API methods.
	HostAPIMethodNotFoundCode = -32601

	defaultHostAPIRateLimit             = 10
	defaultHostAPIBurst                 = 20
	defaultHostAPIDefaultLimit          = 100
	defaultHostAPIRecallLimit           = 10
	defaultHostAPIBridgeIngestDedupTTL  = 24 * time.Hour
	defaultHostAPIBridgeCleanupInterval = time.Hour
	maxMemoryDescriptionLength          = 160
	tagCommentPrefix                    = "<!-- agh-tags:"
	hostAPIUnknownExtensionName         = "unknown"
	hostAPISandboxStateSynced           = "synced"
	hostAPISandboxStatePending          = "pending"
)

type hostAPIContextKey string

const hostAPIExtensionNameContextKey hostAPIContextKey = "extension.host_api.extension_name"
const hostAPIBridgeRuntimeContextKey hostAPIContextKey = "extension.host_api.bridge_runtime"
const hostAPIResourceSessionContextKey hostAPIContextKey = "extension.host_api.resource_session"

// HostAPIOption customizes a HostAPIHandler.
type HostAPIOption func(*HostAPIHandler)

// HostAPIHandler handles extension -> AGH Host API JSON-RPC requests.
type HostAPIHandler struct {
	sessions         hostAPISessionManager
	automation       HostAPIAutomationManager
	tasks            hostAPITaskManager
	network          hostAPINetworkService
	networkStore     store.NetworkConversationStore
	networkUsage     store.NetworkUsageStore
	memory           *memory.Store
	observer         hostAPIObserver
	skills           hostAPISkillsRegistry
	modelCatalog     hostAPIModelCatalogService
	workspaces       workspacepkg.RuntimeResolver
	bridges          hostAPIBridgeRegistry
	dedupStore       hostAPIBridgeDedupStore
	deliveryBroker   hostAPIDeliveryBroker
	resourceStore    resources.RawStore
	resourceCodecs   *resources.CodecRegistry
	soulAuthoring    hostAPISoulAuthoringService
	soulRefresher    hostAPISoulRefresher
	heartbeatAuthor  hostAPIHeartbeatAuthoringService
	heartbeatStatus  hostAPIHeartbeatStatusService
	heartbeatWake    hostAPIHeartbeatWakeService
	sessionHealth    hostAPISessionHealthReader
	wakeEvents       hostAPIHeartbeatWakeEventReader
	memoryProviders  *MemoryProviderRegistry
	capChecker       *CapabilityChecker
	limiter          *hostAPIRateLimiter
	automationGetter func() HostAPIAutomationManager
	resourceTrigger  func(context.Context, resources.ResourceKind, resources.ReconcileReason) error
	now              func() time.Time
	rateLimit        int
	rateBurst        int
	clarify          *hostAPIClarifyRuntime

	bridgeIngestDedupTTL  time.Duration
	bridgeCleanupInterval time.Duration
	bridgeLocks           *hostAPIKeyLocker
	bridgeCleanupMu       sync.Mutex
	bridgeLastCleanup     time.Time

	methods map[string]hostAPIMethodFunc
}

type hostAPIMethodFunc func(context.Context, json.RawMessage) (any, error)

type hostAPISessionManager interface {
	Create(ctx context.Context, opts session.CreateOpts) (*session.Session, error)
	ListAll(ctx context.Context) ([]*session.Info, error)
	Status(ctx context.Context, id string) (*session.Info, error)
	Events(ctx context.Context, id string, query store.EventQuery) ([]store.SessionEvent, error)
	Stop(ctx context.Context, id string) error
	Prompt(ctx context.Context, id string, msg string) (<-chan acp.AgentEvent, error)
	ExecSandbox(ctx context.Context, req session.SandboxExecRequest) (session.SandboxExecResult, error)
}

type hostAPISessionAcceptanceManager interface {
	CreateAccepted(ctx context.Context, opts session.CreateAcceptedOpts) (*session.Info, error)
}

type hostAPIBridgePromptSessionManager interface {
	PromptWithOpts(
		ctx context.Context,
		id string,
		opts session.PromptOpts,
	) (<-chan acp.AgentEvent, error)
}

type hostAPIPromptingSessionManager interface {
	IsPrompting(id string) bool
}

type hostAPINetworkService interface {
	Send(ctx context.Context, req network.SendRequest) (string, error)
	ListPeers(ctx context.Context, workspaceID string, channel string) ([]network.PeerInfo, error)
	ListChannels(ctx context.Context, workspaceID string) ([]network.ChannelInfo, error)
	Status(ctx context.Context) (*network.Status, error)
}

type hostAPIObserver interface {
	Health(ctx context.Context) (observepkg.Health, error)
	QueryEvents(ctx context.Context, query store.EventSummaryQuery) ([]store.EventSummary, error)
	QueryTaskDashboard(ctx context.Context, query observepkg.TaskDashboardQuery) (observepkg.TaskDashboardView, error)
	QueryTaskInbox(
		ctx context.Context,
		query observepkg.TaskInboxQuery,
		actor taskpkg.ActorIdentity,
	) (observepkg.TaskInboxView, error)
}

// HostAPIAutomationManager is the automation surface exposed to the extension Host API.
type HostAPIAutomationManager interface {
	ListJobs(ctx context.Context, query automationpkg.JobListQuery) (automationpkg.JobListPage, error)
	GetJob(ctx context.Context, id string) (automationpkg.Job, error)
	CreateJob(ctx context.Context, job automationpkg.Job) (automationpkg.Job, error)
	UpdateJob(ctx context.Context, job automationpkg.Job) (automationpkg.Job, error)
	DeleteJob(ctx context.Context, id string) error
	TriggerJob(ctx context.Context, id string) (automationpkg.Run, error)
	TriggerJobWithPayload(ctx context.Context, id string, payload map[string]any) (automationpkg.Run, error)
	ListTriggers(ctx context.Context, query automationpkg.TriggerListQuery) (automationpkg.TriggerListPage, error)
	GetTrigger(ctx context.Context, id string) (automationpkg.Trigger, error)
	CreateTrigger(
		ctx context.Context,
		trigger automationpkg.Trigger,
		webhookSecret automationpkg.WebhookSecretWrite,
	) (automationpkg.Trigger, error)
	UpdateTrigger(
		ctx context.Context,
		trigger automationpkg.Trigger,
		webhookSecret *automationpkg.WebhookSecretWrite,
	) (automationpkg.Trigger, error)
	DeleteTrigger(ctx context.Context, id string) error
	ListRuns(ctx context.Context, query automationpkg.RunQuery) ([]automationpkg.Run, error)
	SetJobEnabled(ctx context.Context, id string, enabled bool) (automationpkg.Job, error)
	SetTriggerEnabled(ctx context.Context, id string, enabled bool) (automationpkg.Trigger, error)
	FireExtensionTrigger(
		ctx context.Context,
		request automationpkg.ExtensionTriggerRequest,
	) (automationpkg.TriggerResult, error)
}

type hostAPITaskManager interface {
	ListTaskCatalog(
		ctx context.Context,
		query taskpkg.CatalogQuery,
		actor taskpkg.ActorContext,
	) (taskpkg.CatalogPage, error)
	GetTask(ctx context.Context, id string, actor taskpkg.ActorContext) (*taskpkg.View, error)
	Timeline(
		ctx context.Context,
		taskID string,
		query taskpkg.TimelineQuery,
		actor taskpkg.ActorContext,
	) ([]taskpkg.TimelineItem, error)
	Tree(ctx context.Context, taskID string, actor taskpkg.ActorContext) (*taskpkg.TreeView, error)
	RunDetail(ctx context.Context, runID string, actor taskpkg.ActorContext) (*taskpkg.RunDetailView, error)
	ListTaskRuns(
		ctx context.Context,
		taskID string,
		query taskpkg.RunQuery,
		actor taskpkg.ActorContext,
	) ([]taskpkg.Run, error)
	CreateTask(ctx context.Context, spec taskpkg.CreateTask, actor taskpkg.ActorContext) (*taskpkg.Task, error)
	UpdateTask(
		ctx context.Context,
		id string,
		patch taskpkg.Patch,
		actor taskpkg.ActorContext,
	) (*taskpkg.Task, error)
	CancelTask(
		ctx context.Context,
		id string,
		req taskpkg.CancelTask,
		actor taskpkg.ActorContext,
	) (*taskpkg.Task, error)
	EnqueueRun(ctx context.Context, spec taskpkg.EnqueueRun, actor taskpkg.ActorContext) (*taskpkg.Run, error)
	StartRun(
		ctx context.Context,
		runID string,
		req taskpkg.StartRun,
		actor taskpkg.ActorContext,
	) (*taskpkg.Run, error)
	AttachRunSession(
		ctx context.Context,
		runID string,
		sessionID string,
		actor taskpkg.ActorContext,
	) (*taskpkg.Run, error)
	CompleteRun(
		ctx context.Context,
		runID string,
		result taskpkg.RunResult,
		actor taskpkg.ActorContext,
	) (*taskpkg.Run, error)
	FailRun(
		ctx context.Context,
		runID string,
		failure taskpkg.RunFailure,
		actor taskpkg.ActorContext,
	) (*taskpkg.Run, error)
	CancelRun(
		ctx context.Context,
		runID string,
		req taskpkg.CancelRun,
		actor taskpkg.ActorContext,
	) (*taskpkg.Run, error)
}

type hostAPISkillsRegistry interface {
	List() []*skillspkg.Skill
	ForWorkspace(ctx context.Context, resolved *workspacepkg.ResolvedWorkspace) ([]*skillspkg.Skill, error)
	ForAgent(
		ctx context.Context,
		resolved *workspacepkg.ResolvedWorkspace,
		agentName string,
	) ([]*skillspkg.Skill, error)
}
