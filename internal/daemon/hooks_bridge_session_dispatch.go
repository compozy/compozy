package daemon

import (
	"context"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/session"
)

// OnSessionCreated forwards the full runtime session to the downstream
// observer after hook dispatch has already run. The native hook payload keeps
// lifecycle ordering, while this pass preserves catalog fields that are not
// exposed on public hook payloads.
func (n *hooksNotifier) OnSessionCreated(ctx context.Context, sess *session.Session) {
	if sess == nil {
		return
	}
	_, agentEventNotify := n.runtime()
	if agentEventNotify != nil {
		agentEventNotify.OnSessionCreated(ctx, sess)
	}
}

// OnSessionStopped forwards the full runtime session to the downstream
// observer after hook dispatch has already run.
func (n *hooksNotifier) OnSessionStopped(ctx context.Context, sess *session.Session) {
	if sess == nil {
		return
	}
	_, agentEventNotify := n.runtime()
	if agentEventNotify != nil {
		agentEventNotify.OnSessionStopped(ctx, sess)
	}
	if runtime := n.subprocessHealthNotifier(); runtime != nil {
		runtime.OnSessionStopped(ctx, sess)
	}
}

// OnSessionFinalizing forwards the recorder-open lifecycle phase to observers that must append
// terminal evidence before the session ledger is materialized.
func (n *hooksNotifier) OnSessionFinalizing(ctx context.Context, sess *session.Session) {
	if sess == nil {
		return
	}
	_, agentEventNotify := n.runtime()
	if finalizer, ok := agentEventNotify.(sessionFinalizationObserver); ok {
		finalizer.OnSessionFinalizing(ctx, sess)
	}
}

func (n *hooksNotifier) DispatchSessionPreCreate(
	ctx context.Context,
	payload hookspkg.SessionPreCreatePayload,
) (hookspkg.SessionPreCreatePayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookSessionPreCreate,
		payload,
		hookRuntime.DispatchSessionPreCreate,
	)
}

func (n *hooksNotifier) DispatchSessionPostCreate(
	ctx context.Context,
	payload hookspkg.SessionPostCreatePayload,
) (hookspkg.SessionPostCreatePayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookSessionPostCreate,
		payload,
		hookRuntime.DispatchSessionPostCreate,
	)
}

func (n *hooksNotifier) DispatchSessionPreResume(
	ctx context.Context,
	payload hookspkg.SessionPreResumePayload,
) (hookspkg.SessionPreResumePayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookSessionPreResume,
		payload,
		hookRuntime.DispatchSessionPreResume,
	)
}

func (n *hooksNotifier) DispatchSessionPostResume(
	ctx context.Context,
	payload hookspkg.SessionPostResumePayload,
) (hookspkg.SessionPostResumePayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookSessionPostResume,
		payload,
		hookRuntime.DispatchSessionPostResume,
	)
}

func (n *hooksNotifier) DispatchSessionPreStop(
	ctx context.Context,
	payload hookspkg.SessionPreStopPayload,
) (hookspkg.SessionPreStopPayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookSessionPreStop,
		payload,
		hookRuntime.DispatchSessionPreStop,
	)
}

func (n *hooksNotifier) DispatchSessionPostStop(
	ctx context.Context,
	payload hookspkg.SessionPostStopPayload,
) (hookspkg.SessionPostStopPayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookSessionPostStop,
		payload,
		hookRuntime.DispatchSessionPostStop,
	)
}

func (n *hooksNotifier) DispatchSandboxPrepare(
	ctx context.Context,
	payload hookspkg.SandboxPreparePayload,
) (hookspkg.SandboxPreparePayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookSandboxPrepare,
		payload,
		hookRuntime.DispatchSandboxPrepare,
	)
}

func (n *hooksNotifier) DispatchSandboxReady(
	ctx context.Context,
	payload hookspkg.SandboxReadyPayload,
) (hookspkg.SandboxReadyPayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookSandboxReady,
		payload,
		hookRuntime.DispatchSandboxReady,
	)
}

func (n *hooksNotifier) DispatchSandboxSyncBefore(
	ctx context.Context,
	payload hookspkg.SandboxSyncBeforePayload,
) (hookspkg.SandboxSyncBeforePayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookSandboxSyncBefore,
		payload,
		hookRuntime.DispatchSandboxSyncBefore,
	)
}

func (n *hooksNotifier) DispatchSandboxSyncAfter(
	ctx context.Context,
	payload hookspkg.SandboxSyncAfterPayload,
) (hookspkg.SandboxSyncAfterPayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookSandboxSyncAfter,
		payload,
		hookRuntime.DispatchSandboxSyncAfter,
	)
}

func (n *hooksNotifier) DispatchSandboxStop(
	ctx context.Context,
	payload hookspkg.SandboxStopPayload,
) (hookspkg.SandboxStopPayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookSandboxStop,
		payload,
		hookRuntime.DispatchSandboxStop,
	)
}

func (n *hooksNotifier) DispatchInputPreSubmit(
	ctx context.Context,
	payload hookspkg.InputPreSubmitPayload,
) (hookspkg.InputPreSubmitPayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookInputPreSubmit,
		payload,
		hookRuntime.DispatchInputPreSubmit,
	)
}

func (n *hooksNotifier) DispatchPromptPostAssemble(
	ctx context.Context,
	payload hookspkg.PromptPayload,
) (hookspkg.PromptPayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookPromptPostAssemble,
		payload,
		hookRuntime.DispatchPromptPostAssemble,
	)
}

func (n *hooksNotifier) DispatchEventPreRecord(
	ctx context.Context,
	payload hookspkg.EventPreRecordPayload,
) (hookspkg.EventPreRecordPayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookEventPreRecord,
		payload,
		hookRuntime.DispatchEventPreRecord,
	)
}

func (n *hooksNotifier) DispatchEventPostRecord(
	ctx context.Context,
	payload hookspkg.EventPostRecordPayload,
) (hookspkg.EventPostRecordPayload, error) {
	return dispatchEventPostRecordWithWatchObservers(ctx, n, payload)
}
