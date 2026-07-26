package daemon

import (
	"context"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

func (n *hooksNotifier) DispatchAutomationJobPreFire(
	ctx context.Context,
	payload hookspkg.AutomationJobPreFirePayload,
) (hookspkg.AutomationJobPreFirePayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookAutomationJobPreFire,
		payload,
		hookRuntime.DispatchAutomationJobPreFire,
	)
}

func (n *hooksNotifier) DispatchAutomationJobPostFire(
	ctx context.Context,
	payload hookspkg.AutomationJobPostFirePayload,
) (hookspkg.AutomationJobPostFirePayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookAutomationJobPostFire,
		payload,
		hookRuntime.DispatchAutomationJobPostFire,
	)
}

func (n *hooksNotifier) DispatchAutomationTriggerPreFire(
	ctx context.Context,
	payload hookspkg.AutomationTriggerPreFirePayload,
) (hookspkg.AutomationTriggerPreFirePayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookAutomationTriggerPreFire,
		payload,
		hookRuntime.DispatchAutomationTriggerPreFire,
	)
}

func (n *hooksNotifier) DispatchAutomationTriggerPostFire(
	ctx context.Context,
	payload hookspkg.AutomationTriggerPostFirePayload,
) (hookspkg.AutomationTriggerPostFirePayload, error) {
	return dispatchRuntime(
		ctx,
		n,
		hookspkg.HookAutomationTriggerPostFire,
		payload,
		hookRuntime.DispatchAutomationTriggerPostFire,
	)
}

func (n *hooksNotifier) DispatchAutomationRunCompleted(
	ctx context.Context,
	payload hookspkg.AutomationRunCompletedPayload,
) (hookspkg.AutomationRunCompletedPayload, error) {
	result, err := dispatchRuntime(
		ctx,
		n,
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
	result, err := dispatchRuntime(
		ctx,
		n,
		hookspkg.HookAutomationRunFailed,
		payload,
		hookRuntime.DispatchAutomationRunFailed,
	)
	n.notifyAutomationRunFailedObservers(ctx, result)
	return result, err
}

func dispatchNetworkThreadOpenedWithWatchObservers(
	ctx context.Context,
	notifier *hooksNotifier,
	payload hookspkg.NetworkThreadOpenedPayload,
) (hookspkg.NetworkThreadOpenedPayload, error) {
	result, err := dispatchRuntime(
		ctx,
		notifier,
		hookspkg.HookNetworkThreadOpened,
		payload,
		hookRuntime.DispatchNetworkThreadOpened,
	)
	notifier.notifyNetworkWatchObservers(
		ctx,
		hookspkg.HookNetworkThreadOpened,
		result,
		func(ctx context.Context, observer networkWatchObserver, payload hookspkg.NetworkPayload) error {
			return observer.OnNetworkThreadOpened(ctx, payload)
		},
	)
	return result, err
}

func dispatchNetworkDirectRoomOpenedWithWatchObservers(
	ctx context.Context,
	notifier *hooksNotifier,
	payload hookspkg.NetworkDirectRoomOpenedPayload,
) (hookspkg.NetworkDirectRoomOpenedPayload, error) {
	result, err := dispatchRuntime(
		ctx,
		notifier,
		hookspkg.HookNetworkDirectRoomOpened,
		payload,
		hookRuntime.DispatchNetworkDirectRoomOpened,
	)
	notifier.notifyNetworkWatchObservers(
		ctx,
		hookspkg.HookNetworkDirectRoomOpened,
		result,
		func(ctx context.Context, observer networkWatchObserver, payload hookspkg.NetworkPayload) error {
			return observer.OnNetworkDirectRoomOpened(ctx, payload)
		},
	)
	return result, err
}

func dispatchNetworkMessagePersistedWithWatchObservers(
	ctx context.Context,
	notifier *hooksNotifier,
	payload hookspkg.NetworkMessagePersistedPayload,
) (hookspkg.NetworkMessagePersistedPayload, error) {
	result, err := dispatchRuntime(
		ctx,
		notifier,
		hookspkg.HookNetworkMessagePersisted,
		payload,
		hookRuntime.DispatchNetworkMessagePersisted,
	)
	notifier.notifyNetworkWatchObservers(
		ctx,
		hookspkg.HookNetworkMessagePersisted,
		result,
		func(ctx context.Context, observer networkWatchObserver, payload hookspkg.NetworkPayload) error {
			return observer.OnNetworkMessagePersisted(ctx, payload)
		},
	)
	return result, err
}

func dispatchNetworkWorkOpenedWithWatchObservers(
	ctx context.Context,
	notifier *hooksNotifier,
	payload hookspkg.NetworkWorkOpenedPayload,
) (hookspkg.NetworkWorkOpenedPayload, error) {
	result, err := dispatchRuntime(
		ctx,
		notifier,
		hookspkg.HookNetworkWorkOpened,
		payload,
		hookRuntime.DispatchNetworkWorkOpened,
	)
	notifier.notifyNetworkWatchObservers(
		ctx,
		hookspkg.HookNetworkWorkOpened,
		result,
		func(ctx context.Context, observer networkWatchObserver, payload hookspkg.NetworkPayload) error {
			return observer.OnNetworkWorkOpened(ctx, payload)
		},
	)
	return result, err
}

func dispatchNetworkWorkTransitionedWithWatchObservers(
	ctx context.Context,
	notifier *hooksNotifier,
	payload hookspkg.NetworkWorkTransitionedPayload,
) (hookspkg.NetworkWorkTransitionedPayload, error) {
	result, err := dispatchRuntime(
		ctx,
		notifier,
		hookspkg.HookNetworkWorkTransitioned,
		payload,
		hookRuntime.DispatchNetworkWorkTransitioned,
	)
	notifier.notifyNetworkWatchObservers(
		ctx,
		hookspkg.HookNetworkWorkTransitioned,
		result,
		func(ctx context.Context, observer networkWatchObserver, payload hookspkg.NetworkPayload) error {
			return observer.OnNetworkWorkTransitioned(ctx, payload)
		},
	)
	return result, err
}

func dispatchNetworkWorkClosedWithWatchObservers(
	ctx context.Context,
	notifier *hooksNotifier,
	payload hookspkg.NetworkWorkClosedPayload,
) (hookspkg.NetworkWorkClosedPayload, error) {
	result, err := dispatchRuntime(
		ctx,
		notifier,
		hookspkg.HookNetworkWorkClosed,
		payload,
		hookRuntime.DispatchNetworkWorkClosed,
	)
	notifier.notifyNetworkWatchObservers(
		ctx,
		hookspkg.HookNetworkWorkClosed,
		result,
		func(ctx context.Context, observer networkWatchObserver, payload hookspkg.NetworkPayload) error {
			return observer.OnNetworkWorkClosed(ctx, payload)
		},
	)
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

func (n *hooksNotifier) notifyNetworkWatchObservers(
	ctx context.Context,
	event hookspkg.HookEvent,
	payload hookspkg.NetworkPayload,
	call func(context.Context, networkWatchObserver, hookspkg.NetworkPayload) error,
) {
	for _, observer := range n.networkWatchObservers() {
		notifyObserver(
			ctx,
			n,
			observer,
			payload,
			"network",
			[]any{
				daemonHookEventKey, event,
				"message_id", payload.MessageID,
				"work_id", payload.WorkID,
				daemonWorkspaceIDKey, payload.WorkspaceID,
			},
			call,
		)
	}
}
