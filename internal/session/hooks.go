package session

import (
	"context"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

// LifecycleHooks groups create/resume/stop session lifecycle hook dispatch.
type LifecycleHooks interface {
	DispatchSessionPreCreate(
		context.Context,
		hookspkg.SessionPreCreatePayload,
	) (hookspkg.SessionPreCreatePayload, error)
	DispatchSessionPostCreate(
		context.Context,
		hookspkg.SessionPostCreatePayload,
	) (hookspkg.SessionPostCreatePayload, error)
	DispatchSessionPreResume(
		context.Context,
		hookspkg.SessionPreResumePayload,
	) (hookspkg.SessionPreResumePayload, error)
	DispatchSessionPostResume(
		context.Context,
		hookspkg.SessionPostResumePayload,
	) (hookspkg.SessionPostResumePayload, error)
	DispatchSessionPreStop(context.Context, hookspkg.SessionPreStopPayload) (hookspkg.SessionPreStopPayload, error)
	DispatchSessionPostStop(context.Context, hookspkg.SessionPostStopPayload) (hookspkg.SessionPostStopPayload, error)
}

// SandboxHooks groups execution-sandbox lifecycle hook dispatch.
type SandboxHooks interface {
	DispatchSandboxPrepare(
		context.Context,
		hookspkg.SandboxPreparePayload,
	) (hookspkg.SandboxPreparePayload, error)
	DispatchSandboxReady(
		context.Context,
		hookspkg.SandboxReadyPayload,
	) (hookspkg.SandboxReadyPayload, error)
	DispatchSandboxSyncBefore(
		context.Context,
		hookspkg.SandboxSyncBeforePayload,
	) (hookspkg.SandboxSyncBeforePayload, error)
	DispatchSandboxSyncAfter(
		context.Context,
		hookspkg.SandboxSyncAfterPayload,
	) (hookspkg.SandboxSyncAfterPayload, error)
	DispatchSandboxStop(
		context.Context,
		hookspkg.SandboxStopPayload,
	) (hookspkg.SandboxStopPayload, error)
}

// PromptHooks groups prompt assembly and user-input hook dispatch.
type PromptHooks interface {
	DispatchInputPreSubmit(context.Context, hookspkg.InputPreSubmitPayload) (hookspkg.InputPreSubmitPayload, error)
	DispatchPromptPostAssemble(context.Context, hookspkg.PromptPayload) (hookspkg.PromptPayload, error)
}

// EventHooks groups event-record persistence hook dispatch.
type EventHooks interface {
	DispatchEventPreRecord(context.Context, hookspkg.EventPreRecordPayload) (hookspkg.EventPreRecordPayload, error)
	DispatchEventPostRecord(context.Context, hookspkg.EventPostRecordPayload) (hookspkg.EventPostRecordPayload, error)
}

// AgentHooks groups agent start and stop lifecycle hook dispatch.
type AgentHooks interface {
	DispatchAgentPreStart(context.Context, hookspkg.AgentPreStartPayload) (hookspkg.AgentPreStartPayload, error)
	DispatchAgentSpawned(context.Context, hookspkg.AgentSpawnedPayload) (hookspkg.AgentSpawnedPayload, error)
	DispatchAgentCrashed(context.Context, hookspkg.AgentCrashedPayload) (hookspkg.AgentCrashedPayload, error)
	DispatchAgentStopped(context.Context, hookspkg.AgentStoppedPayload) (hookspkg.AgentStoppedPayload, error)
}

// ConversationHooks groups turn/message hook dispatch used during prompt streaming.
type ConversationHooks interface {
	DispatchTurnStart(context.Context, hookspkg.TurnStartPayload) (hookspkg.TurnStartPayload, error)
	DispatchTurnEnd(context.Context, hookspkg.TurnEndPayload) (hookspkg.TurnEndPayload, error)
	DispatchMessageStart(context.Context, hookspkg.MessageStartPayload) (hookspkg.MessageStartPayload, error)
	DispatchMessageDelta(context.Context, hookspkg.MessageDeltaPayload) (hookspkg.MessageDeltaPayload, error)
	DispatchMessageEnd(context.Context, hookspkg.MessageEndPayload) (hookspkg.MessageEndPayload, error)
	DispatchSessionMessagePersisted(
		context.Context,
		hookspkg.SessionMessagePersistedPayload,
	) (hookspkg.SessionMessagePersistedPayload, error)
}

// ToolHooks groups provider-native tool execution hook dispatch.
type ToolHooks interface {
	DispatchToolPreCall(context.Context, hookspkg.ToolPreCallPayload) (hookspkg.ToolPreCallPayload, error)
	DispatchToolPostCall(context.Context, hookspkg.ToolPostCallPayload) (hookspkg.ToolPostCallPayload, error)
	DispatchToolPostError(context.Context, hookspkg.ToolPostErrorPayload) (hookspkg.ToolPostErrorPayload, error)
}

// CompactionHooks groups context compaction hook dispatch.
type CompactionHooks interface {
	DispatchContextPreCompact(
		context.Context,
		hookspkg.ContextPreCompactPayload,
	) (hookspkg.ContextPreCompactPayload, error)
	DispatchContextPostCompact(
		context.Context,
		hookspkg.ContextPostCompactPayload,
	) (hookspkg.ContextPostCompactPayload, error)
}

// SpawnHooks groups safe child-session spawn hook dispatch.
type SpawnHooks interface {
	DispatchSpawnPreCreate(context.Context, hookspkg.SpawnPreCreatePayload) (hookspkg.SpawnPreCreatePayload, error)
	DispatchSpawnCreated(context.Context, hookspkg.SpawnCreatedPayload) (hookspkg.SpawnCreatedPayload, error)
	DispatchSpawnParentStopped(
		context.Context,
		hookspkg.SpawnParentStoppedPayload,
	) (hookspkg.SpawnParentStoppedPayload, error)
	DispatchSpawnTTLExpired(
		context.Context,
		hookspkg.SpawnTTLExpiredPayload,
	) (hookspkg.SpawnTTLExpiredPayload, error)
	DispatchSpawnReaped(
		context.Context,
		hookspkg.SpawnReapedPayload,
	) (hookspkg.SpawnReapedPayload, error)
}

// AuthoredContextHooks groups Soul, Heartbeat, and session-health observation hook dispatch.
type AuthoredContextHooks interface {
	DispatchSessionHealthUpdateAfter(
		context.Context,
		hookspkg.SessionHealthUpdateAfterPayload,
	) (hookspkg.SessionHealthUpdateAfterPayload, error)
}

// HookSet collects the grouped session hook domains. Nil groups are treated as
// no-op implementations so callers only provide the domains they exercise.
type HookSet struct {
	Session         LifecycleHooks
	Sandbox         SandboxHooks
	Prompt          PromptHooks
	Events          EventHooks
	Agent           AgentHooks
	Conversation    ConversationHooks
	Tools           ToolHooks
	Compaction      CompactionHooks
	Spawn           SpawnHooks
	AuthoredContext AuthoredContextHooks
}

var _ LifecycleHooks = noopSessionLifecycleHooks{}
var _ SandboxHooks = noopSandboxHooks{}
var _ PromptHooks = noopPromptHooks{}
var _ EventHooks = noopEventHooks{}
var _ AgentHooks = noopAgentHooks{}
var _ ConversationHooks = noopConversationHooks{}
var _ ToolHooks = noopToolHooks{}
var _ CompactionHooks = noopCompactionHooks{}
var _ SpawnHooks = noopSpawnHooks{}
var _ AuthoredContextHooks = noopAuthoredContextHooks{}

func (h HookSet) session() LifecycleHooks {
	if h.Session != nil {
		return h.Session
	}
	return noopSessionLifecycleHooks{}
}

func (h HookSet) sandbox() SandboxHooks {
	if h.Sandbox != nil {
		return h.Sandbox
	}
	return noopSandboxHooks{}
}

func (h HookSet) prompt() PromptHooks {
	if h.Prompt != nil {
		return h.Prompt
	}
	return noopPromptHooks{}
}

func (h HookSet) events() EventHooks {
	if h.Events != nil {
		return h.Events
	}
	return noopEventHooks{}
}

func (h HookSet) agent() AgentHooks {
	if h.Agent != nil {
		return h.Agent
	}
	return noopAgentHooks{}
}

func (h HookSet) conversation() ConversationHooks {
	if h.Conversation != nil {
		return h.Conversation
	}
	return noopConversationHooks{}
}

func (h HookSet) tools() ToolHooks {
	if h.Tools != nil {
		return h.Tools
	}
	return noopToolHooks{}
}

func (h HookSet) compaction() CompactionHooks {
	if h.Compaction != nil {
		return h.Compaction
	}
	return noopCompactionHooks{}
}

func (h HookSet) spawn() SpawnHooks {
	if h.Spawn != nil {
		return h.Spawn
	}
	return noopSpawnHooks{}
}

func (h HookSet) authoredContext() AuthoredContextHooks {
	if h.AuthoredContext != nil {
		return h.AuthoredContext
	}
	return noopAuthoredContextHooks{}
}
