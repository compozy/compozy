package hooks

import "fmt"

// HookEventFamily groups hook events into the documented taxonomy families.
type HookEventFamily string

const (
	HookEventFamilySession       HookEventFamily = "session"
	HookEventFamilySandbox       HookEventFamily = "sandbox"
	HookEventFamilyInput         HookEventFamily = "input"
	HookEventFamilyPrompt        HookEventFamily = "prompt"
	HookEventFamilyEvent         HookEventFamily = "event"
	HookEventFamilyAutomation    HookEventFamily = "automation"
	HookEventFamilyAgent         HookEventFamily = "agent"
	HookEventFamilyTurn          HookEventFamily = "turn"
	HookEventFamilyMessage       HookEventFamily = "message"
	HookEventFamilyTool          HookEventFamily = "tool"
	HookEventFamilyPermission    HookEventFamily = "permission"
	HookEventFamilyContext       HookEventFamily = "context"
	HookEventFamilyCoordinator   HookEventFamily = "coordinator"
	HookEventFamilyTask          HookEventFamily = "task"
	HookEventFamilyTaskRun       HookEventFamily = "task.run"
	HookEventFamilyLoop          HookEventFamily = "loop"
	HookEventFamilySpawn         HookEventFamily = "spawn"
	HookEventFamilyNetwork       HookEventFamily = "network"
	HookEventFamilyWindowManager HookEventFamily = "window_manager"
)

// Validate ensures the event family is part of the supported taxonomy.
func (f HookEventFamily) Validate() error {
	switch f {
	case HookEventFamilySession,
		HookEventFamilySandbox,
		HookEventFamilyInput,
		HookEventFamilyPrompt,
		HookEventFamilyEvent,
		HookEventFamilyAutomation,
		HookEventFamilyAgent,
		HookEventFamilyTurn,
		HookEventFamilyMessage,
		HookEventFamilyTool,
		HookEventFamilyPermission,
		HookEventFamilyContext,
		HookEventFamilyCoordinator,
		HookEventFamilyTask,
		HookEventFamilyTaskRun,
		HookEventFamilyLoop,
		HookEventFamilySpawn,
		HookEventFamilyNetwork,
		HookEventFamilyWindowManager:
		return nil
	default:
		return fmt.Errorf("hooks: invalid hook event family %q", f)
	}
}

// HookEvent identifies when a hook fires.
type HookEvent string

const (
	HookSessionPreCreate        HookEvent = "session.pre_create"
	HookSessionPostCreate       HookEvent = "session.post_create"
	HookSessionPreResume        HookEvent = "session.pre_resume"
	HookSessionPostResume       HookEvent = "session.post_resume"
	HookSessionPreStop          HookEvent = "session.pre_stop"
	HookSessionPostStop         HookEvent = "session.post_stop"
	HookSessionMessagePersisted HookEvent = "session.message_persisted"

	HookSandboxPrepare    HookEvent = "sandbox.prepare"
	HookSandboxReady      HookEvent = "sandbox.ready"
	HookSandboxSyncBefore HookEvent = "sandbox.sync.before"
	HookSandboxSyncAfter  HookEvent = "sandbox.sync.after"
	HookSandboxStop       HookEvent = "sandbox.stop"

	HookInputPreSubmit HookEvent = "input.pre_submit"

	HookPromptPostAssemble HookEvent = "prompt.post_assemble"

	HookEventPreRecord  HookEvent = "event.pre_record"
	HookEventPostRecord HookEvent = "event.post_record"

	HookAutomationJobPreFire      HookEvent = "automation.job.pre_fire"
	HookAutomationJobPostFire     HookEvent = "automation.job.post_fire"
	HookAutomationTriggerPreFire  HookEvent = "automation.trigger.pre_fire"
	HookAutomationTriggerPostFire HookEvent = "automation.trigger.post_fire"
	HookAutomationRunCompleted    HookEvent = "automation.run.completed"
	HookAutomationRunFailed       HookEvent = "automation.run.failed"

	HookAgentPreStart                HookEvent = "agent.pre_start"
	HookAgentSpawned                 HookEvent = "agent.spawned"
	HookAgentCrashed                 HookEvent = "agent.crashed"
	HookAgentStopped                 HookEvent = "agent.stopped"
	HookAgentSoulSnapshotResolved    HookEvent = "agent.soul.snapshot.resolved"
	HookAgentSoulMutationAfter       HookEvent = "agent.soul.mutation.after"
	HookAgentHeartbeatPolicyResolved HookEvent = "agent.heartbeat.policy.resolved"
	HookAgentHeartbeatWakeBefore     HookEvent = "agent.heartbeat.wake.before"
	HookAgentHeartbeatWakeAfter      HookEvent = "agent.heartbeat.wake.after"
	HookSessionHealthUpdateAfter     HookEvent = "session.health.update.after"

	HookTurnStart HookEvent = "turn.start"
	HookTurnEnd   HookEvent = "turn.end"

	HookMessageStart HookEvent = "message.start"
	HookMessageDelta HookEvent = "message.delta"
	HookMessageEnd   HookEvent = "message.end"

	HookToolPreCall   HookEvent = "tool.pre_call"
	HookToolPostCall  HookEvent = "tool.post_call"
	HookToolPostError HookEvent = "tool.post_error"

	HookPermissionRequest  HookEvent = "permission.request"
	HookPermissionResolved HookEvent = "permission.resolved"
	HookPermissionDenied   HookEvent = "permission.denied"

	HookContextPreCompact  HookEvent = "context.pre_compact"
	HookContextPostCompact HookEvent = "context.post_compact"

	HookCoordinatorPreSpawn HookEvent = "coordinator.pre_spawn"
	HookCoordinatorSpawned  HookEvent = "coordinator.spawned"
	HookCoordinatorDecision HookEvent = "coordinator.decision"
	HookCoordinatorStopped  HookEvent = "coordinator.stopped"
	HookCoordinatorFailed   HookEvent = "coordinator.failed"

	HookTaskBlocked        HookEvent = "task.blocked"
	HookTaskUnblocked      HookEvent = "task.unblocked"
	HookTaskNeedsAttention HookEvent = "task.needs_attention"
	HookTaskRecovered      HookEvent = "task.recovered"
	HookTaskStatusChanged  HookEvent = "task.status_changed"

	HookTaskRunEnqueued       HookEvent = "task.run.enqueued"
	HookTaskRunPreClaim       HookEvent = "task.run.pre_claim"
	HookTaskRunPostClaim      HookEvent = "task.run.post_claim"
	HookTaskRunLeaseExtended  HookEvent = "task.run.lease_extended"
	HookTaskRunLeaseExpired   HookEvent = "task.run.lease_expired"
	HookTaskRunLeaseRecovered HookEvent = "task.run.lease_recovered"
	HookTaskRunReleased       HookEvent = "task.run.released"
	HookTaskRunCompleted      HookEvent = "task.run.completed"
	HookTaskRunFailed         HookEvent = "task.run.failed"

	HookLoopStarted        HookEvent = "loop.started"
	HookLoopGenerationPre  HookEvent = "loop.generation.pre"
	HookLoopGenerationPost HookEvent = "loop.generation.post"
	HookLoopGatePre        HookEvent = "loop.gate.pre"
	HookLoopGatePost       HookEvent = "loop.gate.post"
	HookLoopNodeTerminal   HookEvent = "loop.node.terminal"
	HookLoopTerminal       HookEvent = "loop.terminal"

	HookSpawnPreCreate     HookEvent = "spawn.pre_create"
	HookSpawnCreated       HookEvent = "spawn.created"
	HookSpawnParentStopped HookEvent = "spawn.parent_stopped"
	HookSpawnTTLExpired    HookEvent = "spawn.ttl_expired"
	HookSpawnReaped        HookEvent = "spawn.reaped"
)

type hookEventSpec struct {
	family       HookEventFamily
	syncEligible bool
}

var hookEventSpecs = mergeHookEventSpecs(map[HookEvent]hookEventSpec{
	HookSessionPreCreate:  {family: HookEventFamilySession, syncEligible: true},
	HookSessionPostCreate: {family: HookEventFamilySession, syncEligible: true},
	HookSessionPreResume:  {family: HookEventFamilySession, syncEligible: true},
	HookSessionPostResume: {family: HookEventFamilySession, syncEligible: true},
	HookSessionPreStop:    {family: HookEventFamilySession, syncEligible: true},
	HookSessionPostStop:   {family: HookEventFamilySession, syncEligible: true},
	HookSessionMessagePersisted: {
		family:       HookEventFamilySession,
		syncEligible: false,
	},
	HookSandboxPrepare: {
		family:       HookEventFamilySandbox,
		syncEligible: true,
	},
	HookSandboxReady: {
		family:       HookEventFamilySandbox,
		syncEligible: false,
	},
	HookSandboxSyncBefore: {
		family:       HookEventFamilySandbox,
		syncEligible: true,
	},
	HookSandboxSyncAfter: {
		family:       HookEventFamilySandbox,
		syncEligible: false,
	},
	HookSandboxStop: {
		family:       HookEventFamilySandbox,
		syncEligible: true,
	},
	HookInputPreSubmit: {family: HookEventFamilyInput, syncEligible: true},
	HookPromptPostAssemble: {
		family:       HookEventFamilyPrompt,
		syncEligible: true,
	},
	HookEventPreRecord:  {family: HookEventFamilyEvent, syncEligible: false},
	HookEventPostRecord: {family: HookEventFamilyEvent, syncEligible: false},
	HookAutomationJobPreFire: {
		family:       HookEventFamilyAutomation,
		syncEligible: true,
	},
	HookAutomationJobPostFire: {
		family:       HookEventFamilyAutomation,
		syncEligible: false,
	},
	HookAutomationTriggerPreFire: {
		family:       HookEventFamilyAutomation,
		syncEligible: true,
	},
	HookAutomationTriggerPostFire: {
		family:       HookEventFamilyAutomation,
		syncEligible: false,
	},
	HookAutomationRunCompleted: {
		family:       HookEventFamilyAutomation,
		syncEligible: false,
	},
	HookAutomationRunFailed: {
		family:       HookEventFamilyAutomation,
		syncEligible: false,
	},
	HookAgentPreStart: {family: HookEventFamilyAgent, syncEligible: true},
	HookAgentSpawned:  {family: HookEventFamilyAgent, syncEligible: true},
	HookAgentCrashed:  {family: HookEventFamilyAgent, syncEligible: true},
	HookAgentStopped:  {family: HookEventFamilyAgent, syncEligible: true},
	HookAgentSoulSnapshotResolved: {
		family:       HookEventFamilyAgent,
		syncEligible: false,
	},
	HookAgentSoulMutationAfter: {
		family:       HookEventFamilyAgent,
		syncEligible: false,
	},
	HookAgentHeartbeatPolicyResolved: {
		family:       HookEventFamilyAgent,
		syncEligible: false,
	},
	HookAgentHeartbeatWakeBefore: {
		family:       HookEventFamilyAgent,
		syncEligible: true,
	},
	HookAgentHeartbeatWakeAfter: {
		family:       HookEventFamilyAgent,
		syncEligible: false,
	},
	HookSessionHealthUpdateAfter: {
		family:       HookEventFamilySession,
		syncEligible: false,
	},
	HookTurnStart:     {family: HookEventFamilyTurn, syncEligible: true},
	HookTurnEnd:       {family: HookEventFamilyTurn, syncEligible: true},
	HookMessageStart:  {family: HookEventFamilyMessage, syncEligible: true},
	HookMessageDelta:  {family: HookEventFamilyMessage, syncEligible: false},
	HookMessageEnd:    {family: HookEventFamilyMessage, syncEligible: true},
	HookToolPreCall:   {family: HookEventFamilyTool, syncEligible: true},
	HookToolPostCall:  {family: HookEventFamilyTool, syncEligible: true},
	HookToolPostError: {family: HookEventFamilyTool, syncEligible: true},
	HookPermissionRequest: {
		family:       HookEventFamilyPermission,
		syncEligible: true,
	},
	HookPermissionResolved: {
		family:       HookEventFamilyPermission,
		syncEligible: false,
	},
	HookPermissionDenied: {
		family:       HookEventFamilyPermission,
		syncEligible: false,
	},
	HookContextPreCompact:  {family: HookEventFamilyContext, syncEligible: true},
	HookContextPostCompact: {family: HookEventFamilyContext, syncEligible: true},
	HookCoordinatorPreSpawn: {
		family:       HookEventFamilyCoordinator,
		syncEligible: true,
	},
	HookCoordinatorSpawned: {
		family:       HookEventFamilyCoordinator,
		syncEligible: true,
	},
	HookCoordinatorDecision: {
		family:       HookEventFamilyCoordinator,
		syncEligible: true,
	},
	HookCoordinatorStopped: {
		family:       HookEventFamilyCoordinator,
		syncEligible: true,
	},
	HookCoordinatorFailed: {
		family:       HookEventFamilyCoordinator,
		syncEligible: true,
	},
	HookTaskBlocked:        {family: HookEventFamilyTask, syncEligible: true},
	HookTaskUnblocked:      {family: HookEventFamilyTask, syncEligible: true},
	HookTaskNeedsAttention: {family: HookEventFamilyTask, syncEligible: true},
	HookTaskRecovered:      {family: HookEventFamilyTask, syncEligible: true},
	HookTaskStatusChanged:  {family: HookEventFamilyTask, syncEligible: false},
	HookTaskRunEnqueued: {
		family:       HookEventFamilyTaskRun,
		syncEligible: true,
	},
	HookTaskRunPreClaim: {
		family:       HookEventFamilyTaskRun,
		syncEligible: true,
	},
	HookTaskRunPostClaim: {
		family:       HookEventFamilyTaskRun,
		syncEligible: true,
	},
	HookTaskRunLeaseExtended: {
		family:       HookEventFamilyTaskRun,
		syncEligible: true,
	},
	HookTaskRunLeaseExpired: {
		family:       HookEventFamilyTaskRun,
		syncEligible: true,
	},
	HookTaskRunLeaseRecovered: {
		family:       HookEventFamilyTaskRun,
		syncEligible: true,
	},
	HookTaskRunReleased: {
		family:       HookEventFamilyTaskRun,
		syncEligible: true,
	},
	HookTaskRunCompleted: {
		family:       HookEventFamilyTaskRun,
		syncEligible: true,
	},
	HookTaskRunFailed: {
		family:       HookEventFamilyTaskRun,
		syncEligible: true,
	},
	HookLoopStarted: {
		family:       HookEventFamilyLoop,
		syncEligible: false,
	},
	HookLoopGenerationPre: {
		family:       HookEventFamilyLoop,
		syncEligible: true,
	},
	HookLoopGenerationPost: {
		family:       HookEventFamilyLoop,
		syncEligible: false,
	},
	HookLoopGatePre: {
		family:       HookEventFamilyLoop,
		syncEligible: true,
	},
	HookLoopGatePost: {
		family:       HookEventFamilyLoop,
		syncEligible: false,
	},
	HookLoopNodeTerminal: {
		family:       HookEventFamilyLoop,
		syncEligible: false,
	},
	HookLoopTerminal: {
		family:       HookEventFamilyLoop,
		syncEligible: false,
	},
	HookSpawnPreCreate: {
		family:       HookEventFamilySpawn,
		syncEligible: true,
	},
	HookSpawnCreated: {
		family:       HookEventFamilySpawn,
		syncEligible: true,
	},
	HookSpawnParentStopped: {
		family:       HookEventFamilySpawn,
		syncEligible: true,
	},
	HookSpawnTTLExpired: {
		family:       HookEventFamilySpawn,
		syncEligible: true,
	},
	HookSpawnReaped: {
		family:       HookEventFamilySpawn,
		syncEligible: true,
	},
}, networkHookEventSpecs(), windowManagerHookEventSpecs())
