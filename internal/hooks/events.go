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
	HookEventFamilyWorktree      HookEventFamily = "worktree"
	HookEventFamilyTerminal      HookEventFamily = "terminal"
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
		HookEventFamilyWindowManager,
		HookEventFamilyWorktree,
		HookEventFamilyTerminal:
		return nil
	default:
		return fmt.Errorf("hooks: invalid hook event family %q", f)
	}
}

// HookEvent identifies when a hook fires.
type HookEvent string

const (
	HookSessionPreCreate                HookEvent = "session.pre_create"
	HookSessionPostCreate               HookEvent = "session.post_create"
	HookSessionPreResume                HookEvent = "session.pre_resume"
	HookSessionPostResume               HookEvent = "session.post_resume"
	HookSessionPreStop                  HookEvent = "session.pre_stop"
	HookSessionPostStop                 HookEvent = "session.post_stop"
	HookSessionMessagePersisted         HookEvent = "session.message_persisted"
	HookSessionRuntimeRecoveryStarted   HookEvent = "session.runtime_recovery.started"
	HookSessionRuntimeRecoverySucceeded HookEvent = "session.runtime_recovery.succeeded"
	HookSessionRuntimeRecoveryExhausted HookEvent = "session.runtime_recovery.exhausted"

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

type hookEventDefinition struct {
	event        HookEvent
	family       HookEventFamily
	syncEligible bool
}

var baseHookEventDefinitions = []hookEventDefinition{
	{event: HookSessionPreCreate, family: HookEventFamilySession, syncEligible: true},
	{event: HookSessionPostCreate, family: HookEventFamilySession, syncEligible: true},
	{event: HookSessionPreResume, family: HookEventFamilySession, syncEligible: true},
	{event: HookSessionPostResume, family: HookEventFamilySession, syncEligible: true},
	{event: HookSessionPreStop, family: HookEventFamilySession, syncEligible: true},
	{event: HookSessionPostStop, family: HookEventFamilySession, syncEligible: true},
	{event: HookSessionMessagePersisted,
		family:       HookEventFamilySession,
		syncEligible: false,
	},
	{event: HookSessionRuntimeRecoveryStarted, family: HookEventFamilySession, syncEligible: false},
	{event: HookSessionRuntimeRecoverySucceeded, family: HookEventFamilySession, syncEligible: false},
	{event: HookSessionRuntimeRecoveryExhausted, family: HookEventFamilySession, syncEligible: false},
	{event: HookSandboxPrepare,
		family:       HookEventFamilySandbox,
		syncEligible: true,
	},
	{event: HookSandboxReady,
		family:       HookEventFamilySandbox,
		syncEligible: false,
	},
	{event: HookSandboxSyncBefore,
		family:       HookEventFamilySandbox,
		syncEligible: true,
	},
	{event: HookSandboxSyncAfter,
		family:       HookEventFamilySandbox,
		syncEligible: false,
	},
	{event: HookSandboxStop,
		family:       HookEventFamilySandbox,
		syncEligible: true,
	},
	{event: HookInputPreSubmit, family: HookEventFamilyInput, syncEligible: true},
	{event: HookPromptPostAssemble,
		family:       HookEventFamilyPrompt,
		syncEligible: true,
	},
	{event: HookEventPreRecord, family: HookEventFamilyEvent, syncEligible: false},
	{event: HookEventPostRecord, family: HookEventFamilyEvent, syncEligible: false},
	{event: HookAutomationJobPreFire,
		family:       HookEventFamilyAutomation,
		syncEligible: true,
	},
	{event: HookAutomationJobPostFire,
		family:       HookEventFamilyAutomation,
		syncEligible: false,
	},
	{event: HookAutomationTriggerPreFire,
		family:       HookEventFamilyAutomation,
		syncEligible: true,
	},
	{event: HookAutomationTriggerPostFire,
		family:       HookEventFamilyAutomation,
		syncEligible: false,
	},
	{event: HookAutomationRunCompleted,
		family:       HookEventFamilyAutomation,
		syncEligible: false,
	},
	{event: HookAutomationRunFailed,
		family:       HookEventFamilyAutomation,
		syncEligible: false,
	},
	{event: HookAgentPreStart, family: HookEventFamilyAgent, syncEligible: true},
	{event: HookAgentSpawned, family: HookEventFamilyAgent, syncEligible: true},
	{event: HookAgentCrashed, family: HookEventFamilyAgent, syncEligible: true},
	{event: HookAgentStopped, family: HookEventFamilyAgent, syncEligible: true},
	{event: HookAgentSoulSnapshotResolved,
		family:       HookEventFamilyAgent,
		syncEligible: false,
	},
	{event: HookAgentSoulMutationAfter,
		family:       HookEventFamilyAgent,
		syncEligible: false,
	},
	{event: HookAgentHeartbeatPolicyResolved,
		family:       HookEventFamilyAgent,
		syncEligible: false,
	},
	{event: HookAgentHeartbeatWakeBefore,
		family:       HookEventFamilyAgent,
		syncEligible: true,
	},
	{event: HookAgentHeartbeatWakeAfter,
		family:       HookEventFamilyAgent,
		syncEligible: false,
	},
	{event: HookSessionHealthUpdateAfter,
		family:       HookEventFamilySession,
		syncEligible: false,
	},
	{event: HookTurnStart, family: HookEventFamilyTurn, syncEligible: true},
	{event: HookTurnEnd, family: HookEventFamilyTurn, syncEligible: true},
	{event: HookMessageStart, family: HookEventFamilyMessage, syncEligible: true},
	{event: HookMessageDelta, family: HookEventFamilyMessage, syncEligible: false},
	{event: HookMessageEnd, family: HookEventFamilyMessage, syncEligible: true},
	{event: HookToolPreCall, family: HookEventFamilyTool, syncEligible: true},
	{event: HookToolPostCall, family: HookEventFamilyTool, syncEligible: true},
	{event: HookToolPostError, family: HookEventFamilyTool, syncEligible: true},
	{event: HookPermissionRequest,
		family:       HookEventFamilyPermission,
		syncEligible: true,
	},
	{event: HookPermissionResolved,
		family:       HookEventFamilyPermission,
		syncEligible: false,
	},
	{event: HookPermissionDenied,
		family:       HookEventFamilyPermission,
		syncEligible: false,
	},
	{event: HookContextPreCompact, family: HookEventFamilyContext, syncEligible: true},
	{event: HookContextPostCompact, family: HookEventFamilyContext, syncEligible: true},
	{event: HookCoordinatorPreSpawn,
		family:       HookEventFamilyCoordinator,
		syncEligible: true,
	},
	{event: HookCoordinatorSpawned,
		family:       HookEventFamilyCoordinator,
		syncEligible: true,
	},
	{event: HookCoordinatorDecision,
		family:       HookEventFamilyCoordinator,
		syncEligible: true,
	},
	{event: HookCoordinatorStopped,
		family:       HookEventFamilyCoordinator,
		syncEligible: true,
	},
	{event: HookCoordinatorFailed,
		family:       HookEventFamilyCoordinator,
		syncEligible: true,
	},
	{event: HookTaskBlocked, family: HookEventFamilyTask, syncEligible: true},
	{event: HookTaskUnblocked, family: HookEventFamilyTask, syncEligible: true},
	{event: HookTaskNeedsAttention, family: HookEventFamilyTask, syncEligible: true},
	{event: HookTaskRecovered, family: HookEventFamilyTask, syncEligible: true},
	{event: HookTaskStatusChanged, family: HookEventFamilyTask, syncEligible: false},
	{event: HookTaskRunEnqueued,
		family:       HookEventFamilyTaskRun,
		syncEligible: true,
	},
	{event: HookTaskRunPreClaim,
		family:       HookEventFamilyTaskRun,
		syncEligible: true,
	},
	{event: HookTaskRunPostClaim,
		family:       HookEventFamilyTaskRun,
		syncEligible: true,
	},
	{event: HookTaskRunLeaseExtended,
		family:       HookEventFamilyTaskRun,
		syncEligible: true,
	},
	{event: HookTaskRunLeaseExpired,
		family:       HookEventFamilyTaskRun,
		syncEligible: true,
	},
	{event: HookTaskRunLeaseRecovered,
		family:       HookEventFamilyTaskRun,
		syncEligible: true,
	},
	{event: HookTaskRunReleased,
		family:       HookEventFamilyTaskRun,
		syncEligible: true,
	},
	{event: HookTaskRunCompleted,
		family:       HookEventFamilyTaskRun,
		syncEligible: true,
	},
	{event: HookTaskRunFailed,
		family:       HookEventFamilyTaskRun,
		syncEligible: true,
	},
	{event: HookLoopStarted,
		family:       HookEventFamilyLoop,
		syncEligible: false,
	},
	{event: HookLoopGenerationPre,
		family:       HookEventFamilyLoop,
		syncEligible: true,
	},
	{event: HookLoopGenerationPost,
		family:       HookEventFamilyLoop,
		syncEligible: false,
	},
	{event: HookLoopGatePre,
		family:       HookEventFamilyLoop,
		syncEligible: true,
	},
	{event: HookLoopGatePost,
		family:       HookEventFamilyLoop,
		syncEligible: false,
	},
	{event: HookLoopNodeTerminal,
		family:       HookEventFamilyLoop,
		syncEligible: false,
	},
	{event: HookLoopTerminal,
		family:       HookEventFamilyLoop,
		syncEligible: false,
	},
	{event: HookSpawnPreCreate,
		family:       HookEventFamilySpawn,
		syncEligible: true,
	},
	{event: HookSpawnCreated,
		family:       HookEventFamilySpawn,
		syncEligible: true,
	},
	{event: HookSpawnParentStopped,
		family:       HookEventFamilySpawn,
		syncEligible: true,
	},
	{event: HookSpawnTTLExpired,
		family:       HookEventFamilySpawn,
		syncEligible: true,
	},
	{event: HookSpawnReaped,
		family:       HookEventFamilySpawn,
		syncEligible: true,
	},
}
