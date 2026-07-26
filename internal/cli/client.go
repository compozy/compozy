package cli

import (
	"context"
	"io"
	"time"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
)

const (
	clientErrorKey   = "error"
	clientMessageKey = "message"
	cliOutcomeKey    = "outcome"
)

const (
	baseURL                        = "http://unix"
	defaultUnixSocketClientTimeout = 30 * time.Second
	defaultUserAgentName           = "agh-cli"
)

// DaemonClient is the CLI transport surface for talking to the AGH daemon over UDS.
type DaemonClient interface {
	Status(ctx context.Context) (StatusRecord, error)
	ObserveOverview(ctx context.Context, query ObserveOverviewQuery) (contract.ObserveOverviewResponse, error)
	Doctor(ctx context.Context, query DoctorQuery) (DoctorRecord, error)
	DaemonStatus(ctx context.Context) (DaemonStatus, error)
	Drain(ctx context.Context) (DrainStatusRecord, error)
	Undrain(ctx context.Context) (DrainStatusRecord, error)
	TriggerSettingsRestart(ctx context.Context) (SettingsRestartActionRecord, error)
	GetSettingsRestartStatus(ctx context.Context, operationID string) (SettingsRestartStatusRecord, error)
	CreateSupportBundle(ctx context.Context, request CreateSupportBundleRequest) (SupportBundleOperationRecord, error)
	GetSupportBundle(ctx context.Context, operationID string) (SupportBundleOperationRecord, error)
	DownloadSupportBundle(ctx context.Context, operationID string, dst io.Writer) error
	GetSettingsUpdate(ctx context.Context) (SettingsUpdateRecord, error)
	UpdateSettingsSkills(ctx context.Context, request UpdateSettingsSkillsRequest) (SettingsMutationRecord, error)
	ReloadSettings(ctx context.Context) (SettingsMutationRecord, error)
	ListSettingsApplyRecords(ctx context.Context, query SettingsApplyHistoryQuery) (SettingsApplyHistoryRecord, error)
	GetOnboardingStatus(ctx context.Context) (contract.OnboardingStatusResponse, error)
	CompleteOnboarding(ctx context.Context) (contract.OnboardingStatusResponse, error)
	ResetOnboarding(ctx context.Context) (contract.OnboardingStatusResponse, error)
	ListProviders(ctx context.Context) (contract.ProviderListResponse, error)
	ProbeProviderAuth(ctx context.Context, providerID string) (contract.ProviderAuthProbeResponse, error)
	ProviderModelClient
	NetworkCoordinationClient
	ListVaultSecrets(ctx context.Context, query VaultListQuery) ([]VaultRecord, error)
	GetVaultSecret(ctx context.Context, ref string) (VaultRecord, error)
	PutVaultSecret(ctx context.Context, request PutVaultSecretRequest) (VaultRecord, error)
	DeleteVaultSecret(ctx context.Context, ref string) error
	NetworkStatus(ctx context.Context) (NetworkStatusRecord, error)
	NetworkPeers(ctx context.Context, query NetworkPeersQuery) ([]NetworkPeerRecord, error)
	NetworkChannels(ctx context.Context, workspaceRef string) ([]NetworkChannelRecord, error)
	CreateNetworkChannel(
		ctx context.Context,
		workspaceRef string,
		request CreateNetworkChannelRequest,
	) (NetworkChannelDetailRecord, error)
	UpdateNetworkChannel(
		ctx context.Context,
		workspaceRef string,
		channel string,
		request UpdateNetworkChannelRequest,
	) (NetworkChannelDetailRecord, error)
	ListNetworkSubscriptions(
		ctx context.Context,
		query NetworkSubscriptionsQuery,
	) ([]NetworkSubscriptionRecord, error)
	SetNetworkSubscription(
		ctx context.Context,
		workspaceRef string,
		channel string,
		request NetworkSubscriptionRequest,
	) (NetworkSubscriptionRecord, error)
	DeleteNetworkSubscription(
		ctx context.Context,
		workspaceRef string,
		channel string,
		sessionID string,
		threadID string,
	) error
	NetworkThreads(ctx context.Context, query NetworkThreadsQuery) (contract.NetworkThreadsResponse, error)
	NetworkThread(
		ctx context.Context,
		workspaceRef string,
		channel string,
		threadID string,
	) (NetworkThreadRecord, error)
	NetworkThreadMessages(
		ctx context.Context,
		query NetworkConversationMessagesQuery,
	) (contract.NetworkThreadMessagesResponse, error)
	NetworkDirects(ctx context.Context, query NetworkDirectsQuery) (contract.NetworkDirectRoomsResponse, error)
	NetworkDirectResolve(
		ctx context.Context,
		workspaceRef string,
		channel string,
		request NetworkDirectResolveRequest,
	) (NetworkDirectRoomRecord, error)
	NetworkDirect(
		ctx context.Context,
		workspaceRef string,
		channel string,
		directID string,
	) (NetworkDirectRoomRecord, error)
	NetworkDirectMessages(
		ctx context.Context,
		query NetworkConversationMessagesQuery,
	) (contract.NetworkDirectRoomMessagesResponse, error)
	NetworkWork(ctx context.Context, workspaceRef string, workID string) (NetworkWorkRecord, error)
	NetworkSend(ctx context.Context, request NetworkSendRequest) (NetworkSendRecord, error)
	NetworkInbox(ctx context.Context, workspaceRef string, sessionID string) ([]NetworkEnvelopeRecord, error)
	PromoteNetworkThreadTask(
		ctx context.Context,
		workspaceRef string,
		channel string,
		threadID string,
		request PromoteNetworkThreadTaskRequest,
	) (PromoteNetworkThreadTaskRecord, error)
	ListExtensions(ctx context.Context) ([]ExtensionRecord, error)
	MarketplaceClient
	MCPSettingsClient
	InstallExtension(ctx context.Context, request InstallExtensionRequest) (ExtensionRecord, error)
	UpdateExtension(ctx context.Context, name string, request UpdateExtensionRequest) (ExtensionUpdateRecord, error)
	RemoveExtension(ctx context.Context, name string) (ManagedExtensionRemoveRecord, error)
	EnableExtension(ctx context.Context, name string) (ExtensionRecord, error)
	DisableExtension(ctx context.Context, name string) (ExtensionRecord, error)
	ExtensionStatus(ctx context.Context, name string) (ExtensionRecord, error)
	ExtensionProvenance(ctx context.Context, name string) (ExtensionProvenanceRecord, error)
	ListBundleCatalog(ctx context.Context) ([]BundleCatalogRecord, error)
	PreviewBundleActivation(ctx context.Context, request ActivateBundleRequest) (BundleActivationRecord, error)
	ActivateBundle(ctx context.Context, request ActivateBundleRequest) (BundleActivationRecord, error)
	ListBundleActivations(ctx context.Context) ([]BundleActivationRecord, error)
	GetBundleActivation(ctx context.Context, id string) (BundleActivationRecord, error)
	UpdateBundleActivation(
		ctx context.Context,
		id string,
		request UpdateBundleActivationRequest,
	) (BundleActivationRecord, error)
	DeactivateBundle(ctx context.Context, id string) error
	BundleNetworkSettings(ctx context.Context) (BundleNetworkSettingsRecord, error)
	bridgeClientAPI
	ListNotificationPresets(ctx context.Context, query NotificationPresetQuery) (NotificationPresetListRecord, error)
	GetNotificationPreset(ctx context.Context, name string) (NotificationPresetRecord, error)
	CreateNotificationPreset(
		ctx context.Context,
		request CreateNotificationPresetRequest,
	) (NotificationPresetRecord, error)
	UpdateNotificationPreset(
		ctx context.Context,
		name string,
		request UpdateNotificationPresetRequest,
	) (NotificationPresetRecord, error)
	DeleteNotificationPreset(ctx context.Context, name string) error
	sessionClientAPI
	CreateWorkspace(ctx context.Context, request WorkspaceCreateRequest) (WorkspaceRecord, error)
	ListWorkspaces(ctx context.Context) ([]WorkspaceRecord, error)
	GetWorkspace(ctx context.Context, ref string) (WorkspaceDetailRecord, error)
	UpdateWorkspace(ctx context.Context, ref string, request WorkspaceUpdateRequest) (WorkspaceRecord, error)
	DeleteWorkspace(ctx context.Context, ref string) error
	ListAgents(ctx context.Context, query AgentQuery) ([]AgentRecord, error)
	GetAgent(ctx context.Context, name string, query AgentQuery) (AgentRecord, error)
	CreateAgent(ctx context.Context, request contract.CreateAgentRequest) (AgentRecord, error)
	UpdateAgent(ctx context.Context, name string, request contract.UpdateAgentRequest) (AgentRecord, error)
	DeleteAgent(ctx context.Context, name string, workspace string) (contract.DeleteAgentResponse, error)
	DuplicateAgent(ctx context.Context, name string, request contract.DuplicateAgentRequest) (AgentRecord, error)
	RolesClient
	GetAgentSoul(ctx context.Context, name string, query AgentQuery) (AgentSoulRecord, error)
	ValidateAgentSoul(ctx context.Context, name string, request AgentSoulValidateRequest) (AgentSoulRecord, error)
	PutAgentSoul(ctx context.Context, name string, request AgentSoulPutRequest) (AgentSoulMutationRecord, error)
	DeleteAgentSoul(ctx context.Context, name string, request AgentSoulDeleteRequest) (AgentSoulMutationRecord, error)
	ListAgentSoulHistory(
		ctx context.Context,
		name string,
		request AgentSoulHistoryRequest,
	) (AgentSoulHistoryRecord, error)
	RollbackAgentSoul(
		ctx context.Context,
		name string,
		request AgentSoulRollbackRequest,
	) (AgentSoulMutationRecord, error)
	GetAgentHeartbeat(
		ctx context.Context,
		name string,
		query AgentQuery,
	) (AgentHeartbeatRecord, error)
	ValidateAgentHeartbeat(
		ctx context.Context,
		name string,
		request AgentHeartbeatValidateRequest,
	) (AgentHeartbeatRecord, error)
	PutAgentHeartbeat(
		ctx context.Context,
		name string,
		request AgentHeartbeatPutRequest,
	) (AgentHeartbeatMutationRecord, error)
	DeleteAgentHeartbeat(
		ctx context.Context,
		name string,
		request AgentHeartbeatDeleteRequest,
	) (AgentHeartbeatMutationRecord, error)
	ListAgentHeartbeatHistory(
		ctx context.Context,
		name string,
		request AgentHeartbeatHistoryRequest,
	) (AgentHeartbeatHistoryRecord, error)
	RollbackAgentHeartbeat(
		ctx context.Context,
		name string,
		request AgentHeartbeatRollbackRequest,
	) (AgentHeartbeatMutationRecord, error)
	GetAgentHeartbeatStatus(
		ctx context.Context,
		name string,
		request AgentHeartbeatStatusRequest,
	) (AgentHeartbeatStatusRecord, error)
	WakeAgentHeartbeat(
		ctx context.Context,
		name string,
		request AgentHeartbeatWakeRequest,
	) (AgentHeartbeatWakeDecisionRecord, error)
	ListResources(ctx context.Context, query ResourceListQuery) ([]ResourceRecord, error)
	GetResource(ctx context.Context, kind string, id string) (ResourceRecord, error)
	PutResource(ctx context.Context, kind string, id string, request ResourcePutRequest) (ResourceRecord, error)
	DeleteResource(ctx context.Context, kind string, id string, request ResourceDeleteRequest) error
	ListSkills(ctx context.Context, query SkillQuery) ([]SkillRecord, error)
	GetSkill(ctx context.Context, name string, query SkillQuery) (SkillRecord, error)
	GetSkillContent(ctx context.Context, name string, query SkillQuery) (string, error)
	GetSkillShadows(ctx context.Context, name string, query SkillQuery) (SkillShadowsRecord, error)
	EnableSkill(ctx context.Context, name string, query SkillQuery) (SkillActionRecord, error)
	DisableSkill(ctx context.Context, name string, query SkillQuery) (SkillActionRecord, error)
	InstallSkillMarketplace(
		ctx context.Context,
		request SkillMarketplaceInstallRequest,
	) (SkillMarketplaceInstallRecord, error)
	UpdateSkillMarketplace(
		ctx context.Context,
		request SkillMarketplaceUpdateRequest,
	) ([]SkillMarketplaceUpdateRecord, error)
	RemoveSkillMarketplace(ctx context.Context, name string) (SkillMarketplaceRemoveRecord, error)
	ToolClient
	HookCatalog(ctx context.Context, query HookCatalogQuery) ([]HookCatalogRecord, error)
	HookRuns(ctx context.Context, workspaceRef string, query HookRunsQuery) ([]HookRunRecord, error)
	HookEvents(ctx context.Context, query HookEventsQuery) ([]HookEventRecord, error)
	ListLogs(ctx context.Context, query LogsListQuery) ([]LogEventRecord, error)
	StreamLogs(ctx context.Context, query LogsListQuery, lastEventID string, handler SSEHandler) error
	MemoryHealth(ctx context.Context, workspace string) (MemoryHealthRecord, error)
	MemoryHistory(ctx context.Context, query MemoryHistoryQuery) ([]MemoryHistoryRecord, error)
	ListMemory(ctx context.Context, query MemoryListQuery) (MemoryListRecord, error)
	ShowMemory(ctx context.Context, filename string, query MemorySelectorQuery) (MemoryEntryRecord, error)
	CreateMemory(ctx context.Context, request MemoryCreateRequest) (MemoryMutationRecord, error)
	EditMemory(ctx context.Context, filename string, request MemoryEditRequest) (MemoryMutationRecord, error)
	DeleteMemory(ctx context.Context, filename string, query MemorySelectorQuery) (MemoryDeleteRecord, error)
	SearchMemory(ctx context.Context, request MemorySearchRequest) (MemorySearchRecord, error)
	ReindexMemory(ctx context.Context, request MemoryReindexRequest) (MemoryReindexRecord, error)
	PromoteMemory(ctx context.Context, request MemoryPromoteRequest) (MemoryPromoteRecord, error)
	ResetMemory(ctx context.Context, request MemoryResetRequest) (MemoryResetRecord, error)
	ReloadMemory(ctx context.Context, request MemorySelectorQuery) (MemoryReloadRecord, error)
	MemoryScopeShow(ctx context.Context, query MemorySelectorQuery) (MemoryScopeShowRecord, error)
	ListMemoryDecisions(ctx context.Context, query MemoryDecisionListQuery) (MemoryDecisionListRecord, error)
	GetMemoryDecision(ctx context.Context, id string) (MemoryDecisionRecord, error)
	RevertMemoryDecision(
		ctx context.Context,
		id string,
		request MemoryDecisionRevertRequest,
	) (MemoryDecisionRevertRecord, error)
	GetMemoryRecallTrace(ctx context.Context, sessionID string, turnSeq int64) (MemoryRecallTraceRecord, error)
	ListMemoryDreams(ctx context.Context) (MemoryDreamListRecord, error)
	GetMemoryDream(ctx context.Context, id string) (MemoryDreamRecord, error)
	TriggerMemoryDream(ctx context.Context, request MemoryDreamTriggerRequest) (MemoryDreamTriggerRecord, error)
	RetryMemoryDream(ctx context.Context, id string, request MemoryDreamRetryRequest) (MemoryDreamRetryRecord, error)
	GetMemoryDreamStatus(ctx context.Context) (MemoryDreamListRecord, error)
	ListMemoryDailyLogs(ctx context.Context, query MemorySelectorQuery) (MemoryDailyLogListRecord, error)
	GetMemoryExtractorStatus(ctx context.Context, sessionID string) (MemoryExtractorStatusRecord, error)
	ListMemoryExtractorFailures(ctx context.Context) (MemoryExtractorFailuresRecord, error)
	RetryMemoryExtractor(ctx context.Context, request MemoryExtractorRetryRequest) (MemoryExtractorRetryRecord, error)
	DrainMemoryExtractor(ctx context.Context) (MemoryExtractorDrainRecord, error)
	ListMemoryProviders(ctx context.Context) (MemoryProviderListRecord, error)
	GetMemoryProvider(ctx context.Context, name string) (MemoryProviderRecord, error)
	SelectMemoryProvider(
		ctx context.Context,
		request MemoryProviderSelectRequest,
	) (MemoryProviderLifecycleRecord, error)
	EnableMemoryProvider(
		ctx context.Context,
		name string,
		request MemoryProviderLifecycleRequest,
	) (MemoryProviderLifecycleRecord, error)
	DisableMemoryProvider(
		ctx context.Context,
		name string,
		request MemoryProviderLifecycleRequest,
	) (MemoryProviderLifecycleRecord, error)
	CreateMemoryAdhocNote(ctx context.Context, request MemoryAdhocNoteRequest) (MemoryAdhocNoteRecord, error)
	automationClientAPI
	ListTasks(ctx context.Context, query TaskListQuery) (TaskListRecord, error)
	CreateTask(ctx context.Context, request CreateTaskRequest) (TaskRecord, error)
	CreateTaskAsAgent(
		ctx context.Context,
		request CreateTaskRequest,
		credentials agentidentity.Credentials,
	) (TaskRecord, error)
	GetTask(ctx context.Context, id string) (TaskDetailRecord, error)
	InspectTask(ctx context.Context, id string) (TaskInspectRecord, error)
	InspectRun(ctx context.Context, id string) (TaskInspectRecord, error)
	UpdateTask(ctx context.Context, id string, request UpdateTaskRequest) (TaskRecord, error)
	BlockTask(ctx context.Context, id string, request CreateTaskBlockRequest) (TaskBlockRecord, error)
	BlockTaskAsAgent(
		ctx context.Context,
		id string,
		request CreateTaskBlockRequest,
		credentials agentidentity.Credentials,
	) (TaskBlockRecord, error)
	ListTaskBlocks(ctx context.Context, id string, includeCleared bool) ([]TaskBlockRecord, error)
	ClearTaskBlock(
		ctx context.Context,
		id string,
		blockID string,
		request ClearTaskBlockRequest,
	) (TaskBlockRecord, error)
	ClearTaskBlockAsAgent(
		ctx context.Context,
		id string,
		blockID string,
		request ClearTaskBlockRequest,
		credentials agentidentity.Credentials,
	) (TaskBlockRecord, error)
	RecoverTask(ctx context.Context, id string, request RecoverTaskRequest) (TaskRecord, error)
	DeleteTask(ctx context.Context, id string) error
	GetTaskExecutionProfile(ctx context.Context, id string) (TaskExecutionProfileRecord, error)
	SetTaskExecutionProfile(
		ctx context.Context,
		id string,
		request *TaskExecutionProfileRequest,
	) (TaskExecutionProfileRecord, error)
	DeleteTaskExecutionProfile(ctx context.Context, id string) error
	CreateTaskBridgeNotificationSubscription(
		ctx context.Context,
		taskID string,
		request *TaskBridgeNotificationSubscriptionRequest,
	) (TaskBridgeNotificationSubscriptionRecord, error)
	ListTaskBridgeNotificationSubscriptions(
		ctx context.Context,
		taskID string,
		query TaskBridgeNotificationSubscriptionQuery,
	) ([]TaskBridgeNotificationSubscriptionRecord, error)
	GetTaskBridgeNotificationSubscription(
		ctx context.Context,
		taskID string,
		subscriptionID string,
	) (TaskBridgeNotificationSubscriptionRecord, error)
	DeleteTaskBridgeNotificationSubscription(ctx context.Context, taskID string, subscriptionID string) error
	RequestTaskRunReview(
		ctx context.Context,
		runID string,
		request *TaskRunReviewRequest,
	) (TaskRunReviewRequestRecord, error)
	RequestTaskRunReviewAsAgent(
		ctx context.Context,
		runID string,
		request *TaskRunReviewRequest,
		credentials agentidentity.Credentials,
	) (TaskRunReviewRequestRecord, error)
	ListTaskRunReviews(ctx context.Context, query TaskRunReviewListQuery) ([]TaskRunReviewRecord, error)
	GetTaskRunReview(ctx context.Context, reviewID string) (TaskRunReviewRecord, error)
	SubmitTaskRunReviewVerdict(
		ctx context.Context,
		reviewID string,
		request *TaskRunReviewVerdictRequest,
	) (TaskRunReviewVerdictRecord, error)
	SubmitTaskRunReviewVerdictAsAgent(
		ctx context.Context,
		reviewID string,
		request *TaskRunReviewVerdictRequest,
		credentials agentidentity.Credentials,
	) (TaskRunReviewVerdictRecord, error)
	PublishTask(ctx context.Context, id string, request TaskExecutionRequest) (TaskExecutionRecord, error)
	StartTask(ctx context.Context, id string, request TaskExecutionRequest) (TaskExecutionRecord, error)
	ApproveTask(ctx context.Context, id string, request TaskExecutionRequest) (TaskExecutionRecord, error)
	RejectTask(ctx context.Context, id string) (TaskRecord, error)
	CancelTask(ctx context.Context, id string, request CancelTaskRequest) (TaskRecord, error)
	PauseTask(ctx context.Context, id string, request PauseTaskRequest) (TaskRecord, error)
	ResumeTask(ctx context.Context, id string, request ResumeTaskRequest) (TaskRecord, error)
	CreateChildTask(ctx context.Context, id string, request CreateTaskChildRequest) (TaskRecord, error)
	AddTaskDependency(ctx context.Context, id string, request AddTaskDependencyRequest) (TaskDetailRecord, error)
	RemoveTaskDependency(ctx context.Context, id string, dependsOnID string) (TaskDetailRecord, error)
	EnqueueTaskRun(ctx context.Context, id string, request EnqueueTaskRunRequest) (TaskRunRecord, error)
	FanOutTaskRuns(ctx context.Context, id string, request FanOutTaskRunsRequest) (FanOutTaskRunsRecord, error)
	ListTaskRuns(ctx context.Context, id string, query TaskRunListQuery) ([]TaskRunRecord, error)
	GetTaskRun(ctx context.Context, id string) (TaskRunDetailRecord, error)
	StartTaskRun(ctx context.Context, id string, request StartTaskRunRequest) (TaskRunRecord, error)
	AttachTaskRunSession(ctx context.Context, id string, request AttachTaskRunSessionRequest) (TaskRunRecord, error)
	CompleteTaskRun(ctx context.Context, id string, request CompleteTaskRunRequest) (TaskRunRecord, error)
	FailTaskRun(ctx context.Context, id string, request FailTaskRunRequest) (TaskRunRecord, error)
	CancelTaskRun(ctx context.Context, id string, request CancelTaskRunRequest) (TaskRunRecord, error)
	ForceReleaseTaskRun(ctx context.Context, id string, request ForceReleaseTaskRunRequest) (TaskRunRecord, error)
	ForceFailTaskRun(ctx context.Context, id string, request ForceFailTaskRunRequest) (TaskRunRecord, error)
	RetryTaskRun(ctx context.Context, id string, request RetryTaskRunRequest) (RetryTaskRunRecord, error)
	RecoverTaskRun(ctx context.Context, id string, request RecoverTaskRunRequest) (RetryTaskRunRecord, error)
	BulkForceReleaseTaskRuns(ctx context.Context, request BulkForceTaskRunRequest) (BulkForceTaskRunRecord, error)
	BulkForceFailTaskRuns(ctx context.Context, request BulkForceTaskRunRequest) (BulkForceTaskRunRecord, error)
	SchedulerStatus(ctx context.Context) (SchedulerStatusRecord, error)
	PauseScheduler(ctx context.Context, request SchedulerPauseRequest) (SchedulerStatusRecord, error)
	ResumeScheduler(ctx context.Context, request SchedulerResumeRequest) (SchedulerStatusRecord, error)
	DrainScheduler(ctx context.Context, request SchedulerDrainRequest) (SchedulerDrainRecord, error)
	SchedulerBacklog(ctx context.Context, query SchedulerBacklogQuery) (SchedulerBacklogRecord, error)
	AgentMe(ctx context.Context, credentials agentidentity.Credentials) (AgentMeRecord, error)
	AgentContext(ctx context.Context, credentials agentidentity.Credentials) (AgentContextRecord, error)
	AgentSpawn(
		ctx context.Context,
		request AgentSpawnRequest,
		credentials agentidentity.Credentials,
	) (AgentSpawnRecord, error)
	AgentChannels(ctx context.Context, credentials agentidentity.Credentials) ([]AgentChannelRecord, error)
	AgentChannelRecv(
		ctx context.Context,
		channel string,
		query AgentChannelRecvQuery,
		credentials agentidentity.Credentials,
	) ([]AgentChannelMessageRecord, error)
	AgentChannelSend(
		ctx context.Context,
		channel string,
		request AgentChannelSendRequest,
		credentials agentidentity.Credentials,
	) (AgentChannelMessageRecord, error)
	AgentChannelReply(
		ctx context.Context,
		request AgentChannelReplyRequest,
		credentials agentidentity.Credentials,
	) (AgentChannelMessageRecord, error)
	AgentTaskClaimNext(
		ctx context.Context,
		request AgentTaskClaimNextRequest,
		credentials agentidentity.Credentials,
	) (AgentTaskNextRecord, error)
	AgentTaskHeartbeat(
		ctx context.Context,
		runID string,
		request AgentTaskHeartbeatRequest,
		credentials agentidentity.Credentials,
	) (AgentTaskLeaseRecord, error)
	AgentTaskComplete(
		ctx context.Context,
		runID string,
		request AgentTaskCompleteRequest,
		credentials agentidentity.Credentials,
	) (AgentTaskLeaseRecord, error)
	AgentTaskFail(
		ctx context.Context,
		runID string,
		request AgentTaskFailRequest,
		credentials agentidentity.Credentials,
	) (AgentTaskLeaseRecord, error)
	AgentTaskRelease(
		ctx context.Context,
		runID string,
		request AgentTaskReleaseRequest,
		credentials agentidentity.Credentials,
	) (AgentTaskLeaseRecord, error)
}
