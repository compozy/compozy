package session

import (
	"context"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

type noopSessionLifecycleHooks struct{}

func (noopSessionLifecycleHooks) DispatchSessionPreCreate(
	_ context.Context,
	payload hookspkg.SessionPreCreatePayload,
) (hookspkg.SessionPreCreatePayload, error) {
	return payload, nil
}

func (noopSessionLifecycleHooks) DispatchSessionPostCreate(
	_ context.Context,
	payload hookspkg.SessionPostCreatePayload,
) (hookspkg.SessionPostCreatePayload, error) {
	return payload, nil
}

func (noopSessionLifecycleHooks) DispatchSessionPreResume(
	_ context.Context,
	payload hookspkg.SessionPreResumePayload,
) (hookspkg.SessionPreResumePayload, error) {
	return payload, nil
}

func (noopSessionLifecycleHooks) DispatchSessionPostResume(
	_ context.Context,
	payload hookspkg.SessionPostResumePayload,
) (hookspkg.SessionPostResumePayload, error) {
	return payload, nil
}

func (noopSessionLifecycleHooks) DispatchSessionPreStop(
	_ context.Context,
	payload hookspkg.SessionPreStopPayload,
) (hookspkg.SessionPreStopPayload, error) {
	return payload, nil
}

func (noopSessionLifecycleHooks) DispatchSessionPostStop(
	_ context.Context,
	payload hookspkg.SessionPostStopPayload,
) (hookspkg.SessionPostStopPayload, error) {
	return payload, nil
}

type noopSandboxHooks struct{}

func (noopSandboxHooks) DispatchSandboxPrepare(
	_ context.Context,
	payload hookspkg.SandboxPreparePayload,
) (hookspkg.SandboxPreparePayload, error) {
	return payload, nil
}

func (noopSandboxHooks) DispatchSandboxReady(
	_ context.Context,
	payload hookspkg.SandboxReadyPayload,
) (hookspkg.SandboxReadyPayload, error) {
	return payload, nil
}

func (noopSandboxHooks) DispatchSandboxSyncBefore(
	_ context.Context,
	payload hookspkg.SandboxSyncBeforePayload,
) (hookspkg.SandboxSyncBeforePayload, error) {
	return payload, nil
}

func (noopSandboxHooks) DispatchSandboxSyncAfter(
	_ context.Context,
	payload hookspkg.SandboxSyncAfterPayload,
) (hookspkg.SandboxSyncAfterPayload, error) {
	return payload, nil
}

func (noopSandboxHooks) DispatchSandboxStop(
	_ context.Context,
	payload hookspkg.SandboxStopPayload,
) (hookspkg.SandboxStopPayload, error) {
	return payload, nil
}

type noopPromptHooks struct{}

func (noopPromptHooks) DispatchInputPreSubmit(
	_ context.Context,
	payload hookspkg.InputPreSubmitPayload,
) (hookspkg.InputPreSubmitPayload, error) {
	return payload, nil
}

func (noopPromptHooks) DispatchPromptPostAssemble(
	_ context.Context,
	payload hookspkg.PromptPayload,
) (hookspkg.PromptPayload, error) {
	return payload, nil
}

type noopEventHooks struct{}

func (noopEventHooks) DispatchEventPreRecord(
	_ context.Context,
	payload hookspkg.EventPreRecordPayload,
) (hookspkg.EventPreRecordPayload, error) {
	return payload, nil
}

func (noopEventHooks) DispatchEventPostRecord(
	_ context.Context,
	payload hookspkg.EventPostRecordPayload,
) (hookspkg.EventPostRecordPayload, error) {
	return payload, nil
}

type noopAgentHooks struct{}

func (noopAgentHooks) DispatchAgentPreStart(
	_ context.Context,
	payload hookspkg.AgentPreStartPayload,
) (hookspkg.AgentPreStartPayload, error) {
	return payload, nil
}

func (noopAgentHooks) DispatchAgentSpawned(
	_ context.Context,
	payload hookspkg.AgentSpawnedPayload,
) (hookspkg.AgentSpawnedPayload, error) {
	return payload, nil
}

func (noopAgentHooks) DispatchAgentCrashed(
	_ context.Context,
	payload hookspkg.AgentCrashedPayload,
) (hookspkg.AgentCrashedPayload, error) {
	return payload, nil
}

func (noopAgentHooks) DispatchAgentStopped(
	_ context.Context,
	payload hookspkg.AgentStoppedPayload,
) (hookspkg.AgentStoppedPayload, error) {
	return payload, nil
}
