package daemon

import (
	"context"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

type taskStatusChangedObserver interface {
	OnTaskStatusChanged(context.Context, hookspkg.TaskStatusChangedPayload) error
}

type taskStatusProjectionConsumer interface {
	OnTaskStatusProjection(context.Context, taskStatusProjection) error
}

type taskLifecycleWatchObserver interface {
	OnTaskBlocked(context.Context, hookspkg.TaskBlockedPayload) error
	OnTaskUnblocked(context.Context, hookspkg.TaskUnblockedPayload) error
	OnTaskNeedsAttention(context.Context, hookspkg.TaskNeedsAttentionPayload) error
	OnTaskRecovered(context.Context, hookspkg.TaskRecoveredPayload) error
}

type loopNodeTerminalObserver interface {
	OnLoopNodeTerminal(context.Context, hookspkg.LoopNodeTerminalPayload) error
}

type automationRunWatchObserver interface {
	OnAutomationRunCompleted(context.Context, hookspkg.AutomationRunCompletedPayload) error
	OnAutomationRunFailed(context.Context, hookspkg.AutomationRunFailedPayload) error
}

type networkWatchObserver interface {
	OnNetworkThreadOpened(context.Context, hookspkg.NetworkThreadOpenedPayload) error
	OnNetworkDirectRoomOpened(context.Context, hookspkg.NetworkDirectRoomOpenedPayload) error
	OnNetworkMessagePersisted(context.Context, hookspkg.NetworkMessagePersistedPayload) error
	OnNetworkWorkOpened(context.Context, hookspkg.NetworkWorkOpenedPayload) error
	OnNetworkWorkTransitioned(context.Context, hookspkg.NetworkWorkTransitionedPayload) error
	OnNetworkWorkClosed(context.Context, hookspkg.NetworkWorkClosedPayload) error
}

func (n *hooksNotifier) AddTaskStatusChangedObserver(observer taskStatusChangedObserver) {
	if n == nil || observer == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.taskStatusChangedHooks = append(n.taskStatusChangedHooks, observer)
}

func (n *hooksNotifier) AddTaskStatusProjectionObserver(observer taskStatusProjectionConsumer) {
	if n == nil || observer == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.taskStatusProjectionHooks = append(n.taskStatusProjectionHooks, observer)
}

func (n *hooksNotifier) PublishTaskStatusProjection(ctx context.Context, projection taskStatusProjection) {
	if n == nil {
		return
	}
	n.mu.RLock()
	observers := append([]taskStatusProjectionConsumer(nil), n.taskStatusProjectionHooks...)
	n.mu.RUnlock()
	for _, observer := range observers {
		if err := observer.OnTaskStatusProjection(ctx, projection); err != nil {
			n.logger.Warn(
				"daemon: task status projection observer failed",
				"error", err,
				"event_id", projection.EventID,
				"task_id", projection.TaskID,
				"run_id", projection.RunID,
			)
		}
	}
}

func (n *hooksNotifier) AddTaskLifecycleWatchObserver(observer taskLifecycleWatchObserver) {
	if n == nil || observer == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.taskLifecycleWatchHooks = append(n.taskLifecycleWatchHooks, observer)
}

func (n *hooksNotifier) AddLoopNodeTerminalObserver(observer loopNodeTerminalObserver) {
	if n == nil || observer == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.loopNodeTerminalHooks = append(n.loopNodeTerminalHooks, observer)
}

func (n *hooksNotifier) AddAutomationRunWatchObserver(observer automationRunWatchObserver) {
	if n == nil || observer == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.automationRunWatchHooks = append(n.automationRunWatchHooks, observer)
}

func (n *hooksNotifier) AddNetworkWatchObserver(observer networkWatchObserver) {
	if n == nil || observer == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.networkWatchHooks = append(n.networkWatchHooks, observer)
}

func (n *hooksNotifier) taskStatusChangedObservers() []taskStatusChangedObserver {
	if n == nil {
		return nil
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	return append([]taskStatusChangedObserver(nil), n.taskStatusChangedHooks...)
}

func (n *hooksNotifier) taskLifecycleWatchObservers() []taskLifecycleWatchObserver {
	if n == nil {
		return nil
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	return append([]taskLifecycleWatchObserver(nil), n.taskLifecycleWatchHooks...)
}

func (n *hooksNotifier) loopNodeTerminalObservers() []loopNodeTerminalObserver {
	if n == nil {
		return nil
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	return append([]loopNodeTerminalObserver(nil), n.loopNodeTerminalHooks...)
}

func (n *hooksNotifier) automationRunWatchObservers() []automationRunWatchObserver {
	if n == nil {
		return nil
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	return append([]automationRunWatchObserver(nil), n.automationRunWatchHooks...)
}

func (n *hooksNotifier) networkWatchObservers() []networkWatchObserver {
	if n == nil {
		return nil
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	return append([]networkWatchObserver(nil), n.networkWatchHooks...)
}

func dispatchTaskStatusChangedWithWatchObservers(
	ctx context.Context,
	notifier *hooksNotifier,
	payload hookspkg.TaskStatusChangedPayload,
) (hookspkg.TaskStatusChangedPayload, error) {
	result, err := dispatchRuntime(
		ctx,
		notifier,
		hookspkg.HookTaskStatusChanged,
		payload,
		hookRuntime.DispatchTaskStatusChanged,
	)
	notifier.notifyTaskStatusChangedObservers(ctx, result)
	return result, err
}

func (n *hooksNotifier) notifyTaskStatusChangedObservers(
	ctx context.Context,
	payload hookspkg.TaskStatusChangedPayload,
) {
	for _, observer := range n.taskStatusChangedObservers() {
		notifyObserver(
			ctx,
			n,
			observer,
			payload,
			"task status",
			[]any{daemonTaskIDKey, payload.TaskID, daemonWorkspaceIDKey, payload.WorkspaceID},
			func(ctx context.Context, observer taskStatusChangedObserver, payload hookspkg.TaskStatusChangedPayload) error {
				return observer.OnTaskStatusChanged(ctx, payload)
			},
		)
	}
}

func dispatchTaskBlockedWithWatchObservers(
	ctx context.Context,
	notifier *hooksNotifier,
	payload hookspkg.TaskBlockedPayload,
) (hookspkg.TaskBlockedPayload, error) {
	result, err := dispatchRuntime(ctx, notifier, hookspkg.HookTaskBlocked, payload, hookRuntime.DispatchTaskBlocked)
	notifier.notifyTaskBlockedObservers(ctx, result)
	return result, err
}

func dispatchTaskUnblockedWithWatchObservers(
	ctx context.Context,
	notifier *hooksNotifier,
	payload hookspkg.TaskUnblockedPayload,
) (hookspkg.TaskUnblockedPayload, error) {
	result, err := dispatchRuntime(
		ctx,
		notifier,
		hookspkg.HookTaskUnblocked,
		payload,
		hookRuntime.DispatchTaskUnblocked,
	)
	notifier.notifyTaskUnblockedObservers(ctx, result)
	return result, err
}

func dispatchTaskNeedsAttentionWithWatchObservers(
	ctx context.Context,
	notifier *hooksNotifier,
	payload hookspkg.TaskNeedsAttentionPayload,
) (hookspkg.TaskNeedsAttentionPayload, error) {
	result, err := dispatchRuntime(
		ctx,
		notifier,
		hookspkg.HookTaskNeedsAttention,
		payload,
		hookRuntime.DispatchTaskNeedsAttention,
	)
	notifier.notifyTaskNeedsAttentionObservers(ctx, result)
	return result, err
}

func dispatchTaskRecoveredWithWatchObservers(
	ctx context.Context,
	notifier *hooksNotifier,
	payload hookspkg.TaskRecoveredPayload,
) (hookspkg.TaskRecoveredPayload, error) {
	result, err := dispatchRuntime(
		ctx,
		notifier,
		hookspkg.HookTaskRecovered,
		payload,
		hookRuntime.DispatchTaskRecovered,
	)
	notifier.notifyTaskRecoveredObservers(ctx, result)
	return result, err
}

func (n *hooksNotifier) notifyTaskBlockedObservers(ctx context.Context, payload hookspkg.TaskBlockedPayload) {
	for _, observer := range n.taskLifecycleWatchObservers() {
		notifyObserver(
			ctx,
			n,
			observer,
			payload,
			"task lifecycle",
			taskLifecycleObserverAttributes(hookspkg.HookTaskBlocked, payload.TaskID, payload.WorkspaceID),
			func(ctx context.Context, observer taskLifecycleWatchObserver, payload hookspkg.TaskBlockedPayload) error {
				return observer.OnTaskBlocked(ctx, payload)
			},
		)
	}
}

func (n *hooksNotifier) notifyTaskUnblockedObservers(ctx context.Context, payload hookspkg.TaskUnblockedPayload) {
	for _, observer := range n.taskLifecycleWatchObservers() {
		notifyObserver(
			ctx,
			n,
			observer,
			payload,
			"task lifecycle",
			taskLifecycleObserverAttributes(hookspkg.HookTaskUnblocked, payload.TaskID, payload.WorkspaceID),
			func(ctx context.Context, observer taskLifecycleWatchObserver, payload hookspkg.TaskUnblockedPayload) error {
				return observer.OnTaskUnblocked(ctx, payload)
			},
		)
	}
}

func (n *hooksNotifier) notifyTaskNeedsAttentionObservers(
	ctx context.Context,
	payload hookspkg.TaskNeedsAttentionPayload,
) {
	for _, observer := range n.taskLifecycleWatchObservers() {
		notifyObserver(
			ctx,
			n,
			observer,
			payload,
			"task lifecycle",
			taskLifecycleObserverAttributes(hookspkg.HookTaskNeedsAttention, payload.TaskID, payload.WorkspaceID),
			func(
				ctx context.Context,
				observer taskLifecycleWatchObserver,
				payload hookspkg.TaskNeedsAttentionPayload,
			) error {
				return observer.OnTaskNeedsAttention(ctx, payload)
			},
		)
	}
}

func (n *hooksNotifier) notifyTaskRecoveredObservers(ctx context.Context, payload hookspkg.TaskRecoveredPayload) {
	for _, observer := range n.taskLifecycleWatchObservers() {
		notifyObserver(
			ctx,
			n,
			observer,
			payload,
			"task lifecycle",
			taskLifecycleObserverAttributes(hookspkg.HookTaskRecovered, payload.TaskID, payload.WorkspaceID),
			func(ctx context.Context, observer taskLifecycleWatchObserver, payload hookspkg.TaskRecoveredPayload) error {
				return observer.OnTaskRecovered(ctx, payload)
			},
		)
	}
}

func taskLifecycleObserverAttributes(
	event hookspkg.HookEvent,
	taskID string,
	workspaceID string,
) []any {
	return []any{
		daemonHookEventKey, event,
		daemonTaskIDKey, taskID,
		daemonWorkspaceIDKey, workspaceID,
	}
}

func dispatchLoopNodeTerminalWithWatchObservers(
	ctx context.Context,
	notifier *hooksNotifier,
	payload hookspkg.LoopNodeTerminalPayload,
) (hookspkg.LoopNodeTerminalPayload, error) {
	result, err := dispatchRuntime(
		ctx,
		notifier,
		hookspkg.HookLoopNodeTerminal,
		payload,
		hookRuntime.DispatchLoopNodeTerminal,
	)
	notifier.notifyLoopNodeTerminalObservers(ctx, result)
	return result, err
}

func (n *hooksNotifier) notifyLoopNodeTerminalObservers(
	ctx context.Context,
	payload hookspkg.LoopNodeTerminalPayload,
) {
	for _, observer := range n.loopNodeTerminalObservers() {
		notifyObserver(
			ctx,
			n,
			observer,
			payload,
			"loop node terminal",
			[]any{daemonLoopRunIDKey, payload.LoopRunID, "node_id", payload.NodeID},
			func(ctx context.Context, observer loopNodeTerminalObserver, payload hookspkg.LoopNodeTerminalPayload) error {
				return observer.OnLoopNodeTerminal(ctx, payload)
			},
		)
	}
}
