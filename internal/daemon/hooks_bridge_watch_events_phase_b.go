package daemon

import (
	"context"

	hookspkg "github.com/compozy/compozy/internal/hooks"
)

func (n *hooksNotifier) DispatchAutomationJobPreFire(
	ctx context.Context,
	payload hookspkg.AutomationJobPreFirePayload,
) (hookspkg.AutomationJobPreFirePayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookAutomationJobPreFire,
		payload,
		hookRuntime.DispatchAutomationJobPreFire,
	)
}

func (n *hooksNotifier) DispatchAutomationJobPostFire(
	ctx context.Context,
	payload hookspkg.AutomationJobPostFirePayload,
) (hookspkg.AutomationJobPostFirePayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookAutomationJobPostFire,
		payload,
		hookRuntime.DispatchAutomationJobPostFire,
	)
}

func (n *hooksNotifier) DispatchAutomationTriggerPreFire(
	ctx context.Context,
	payload hookspkg.AutomationTriggerPreFirePayload,
) (hookspkg.AutomationTriggerPreFirePayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookAutomationTriggerPreFire,
		payload,
		hookRuntime.DispatchAutomationTriggerPreFire,
	)
}

func (n *hooksNotifier) DispatchAutomationTriggerPostFire(
	ctx context.Context,
	payload hookspkg.AutomationTriggerPostFirePayload,
) (hookspkg.AutomationTriggerPostFirePayload, error) {
	return n.dispatchRuntime(ctx,
		hookspkg.HookAutomationTriggerPostFire,
		payload,
		hookRuntime.DispatchAutomationTriggerPostFire,
	)
}

func (n *hooksNotifier) DispatchAutomationRunCompleted(
	ctx context.Context,
	payload hookspkg.AutomationRunCompletedPayload,
) (hookspkg.AutomationRunCompletedPayload, error) {
	result, err := n.dispatchRuntime(ctx,
		hookspkg.HookAutomationRunCompleted,
		payload,
		hookRuntime.DispatchAutomationRunCompleted,
	)
	n.notifyAutomationRunCompletedObservers(ctx, result)
	return result, err
}

func (n *hooksNotifier) DispatchAutomationRunFailed(
	ctx context.Context,
	payload hookspkg.AutomationRunFailedPayload,
) (hookspkg.AutomationRunFailedPayload, error) {
	result, err := n.dispatchRuntime(ctx,
		hookspkg.HookAutomationRunFailed,
		payload,
		hookRuntime.DispatchAutomationRunFailed,
	)
	n.notifyAutomationRunFailedObservers(ctx, result)
	return result, err
}

func (n *hooksNotifier) notifyAutomationRunCompletedObservers(
	ctx context.Context,
	payload hookspkg.AutomationRunCompletedPayload,
) {
	for _, observer := range n.automationRunWatchObservers() {
		notifyObserver(
			ctx,
			n,
			observer,
			payload,
			"automation run",
			[]any{
				daemonHookEventKey, hookspkg.HookAutomationRunCompleted,
				daemonLogRunIDKey, payload.RunID,
				daemonWorkspaceIDKey, payload.WorkspaceID,
			},
			func(
				ctx context.Context,
				observer automationRunWatchObserver,
				payload hookspkg.AutomationRunCompletedPayload,
			) error {
				return observer.OnAutomationRunCompleted(ctx, payload)
			},
		)
	}
}

func (n *hooksNotifier) notifyAutomationRunFailedObservers(
	ctx context.Context,
	payload hookspkg.AutomationRunFailedPayload,
) {
	for _, observer := range n.automationRunWatchObservers() {
		notifyObserver(
			ctx,
			n,
			observer,
			payload,
			"automation run",
			[]any{
				daemonHookEventKey, hookspkg.HookAutomationRunFailed,
				daemonLogRunIDKey, payload.RunID,
				daemonWorkspaceIDKey, payload.WorkspaceID,
			},
			func(
				ctx context.Context,
				observer automationRunWatchObserver,
				payload hookspkg.AutomationRunFailedPayload,
			) error {
				return observer.OnAutomationRunFailed(ctx, payload)
			},
		)
	}
}
