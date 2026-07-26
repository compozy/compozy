package daemon

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
)

type hooksNotifier struct {
	mu sync.RWMutex

	logger                    *slog.Logger
	now                       func() time.Time
	hooks                     hookRuntime
	agentEventNotify          session.Notifier
	subprocessHealthRuntime   subprocessHealthRuntimeNotifier
	eventSummaries            hookEventSummaryWriter
	taskRunEnqueuedHooks      []taskRunEnqueuedObserver
	taskRunTerminalHooks      []taskRunTerminalObserver
	loopStartedHooks          []loopStartedObserver
	loopTerminalHooks         []loopTerminalObserver
	taskStatusChangedHooks    []taskStatusChangedObserver
	taskStatusProjectionHooks []taskStatusProjectionConsumer
	taskLifecycleWatchHooks   []taskLifecycleWatchObserver
	loopNodeTerminalHooks     []loopNodeTerminalObserver
	automationRunWatchHooks   []automationRunWatchObserver
	networkWatchHooks         []networkWatchObserver
	coordinatorWatchHooks     []coordinatorWatchObserver
	eventRecordWatchHooks     []eventRecordWatchObserver
}

type subprocessHealthRuntimeNotifier interface {
	session.SubprocessHealthNotifier
	OnSessionStopped(context.Context, *session.Session)
}

var _ session.Notifier = (*hooksNotifier)(nil)
var _ session.FinalizationNotifier = (*hooksNotifier)(nil)
var _ session.LifecycleHooks = (*hooksNotifier)(nil)
var _ session.SandboxHooks = (*hooksNotifier)(nil)
var _ session.PromptHooks = (*hooksNotifier)(nil)
var _ session.EventHooks = (*hooksNotifier)(nil)
var _ session.AgentHooks = (*hooksNotifier)(nil)
var _ session.ConversationHooks = (*hooksNotifier)(nil)
var _ session.ToolHooks = (*hooksNotifier)(nil)
var _ session.CompactionHooks = (*hooksNotifier)(nil)
var _ session.SpawnHooks = (*hooksNotifier)(nil)
var _ session.AuthoredContextHooks = (*hooksNotifier)(nil)
var _ taskpkg.RunHookDispatcher = (*hooksNotifier)(nil)
var _ network.HookDispatcher = (*hooksNotifier)(nil)
var _ session.AgentEventNotifier = (*hooksNotifier)(nil)
var _ session.SandboxLifecycleNotifier = (*hooksNotifier)(nil)
var _ session.SubprocessHealthNotifier = (*hooksNotifier)(nil)

func newHooksNotifier(logger *slog.Logger, now func() time.Time) *hooksNotifier {
	if logger == nil {
		logger = slog.Default()
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &hooksNotifier{logger: logger, now: now}
}

func (n *hooksNotifier) setRuntime(
	hooks hookRuntime,
	agentEventNotify session.Notifier,
	eventSummaries ...hookEventSummaryWriter,
) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.hooks = hooks
	n.agentEventNotify = agentEventNotify
	if len(eventSummaries) > 0 {
		n.eventSummaries = eventSummaries[0]
	}
}

func (n *hooksNotifier) setSubprocessHealthRuntime(runtime subprocessHealthRuntimeNotifier) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.subprocessHealthRuntime = runtime
}

func (n *hooksNotifier) subprocessHealthNotifier() subprocessHealthRuntimeNotifier {
	if n == nil {
		return nil
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.subprocessHealthRuntime
}

func (n *hooksNotifier) AddTaskRunEnqueuedObserver(observer taskRunEnqueuedObserver) {
	if n == nil || observer == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.taskRunEnqueuedHooks = append(n.taskRunEnqueuedHooks, observer)
}

func (n *hooksNotifier) taskRunEnqueuedObservers() []taskRunEnqueuedObserver {
	if n == nil {
		return nil
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	return append([]taskRunEnqueuedObserver(nil), n.taskRunEnqueuedHooks...)
}

func (n *hooksNotifier) AddTaskRunTerminalObserver(observer taskRunTerminalObserver) {
	if n == nil || observer == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.taskRunTerminalHooks = append(n.taskRunTerminalHooks, observer)
}

func (n *hooksNotifier) taskRunTerminalObservers() []taskRunTerminalObserver {
	if n == nil {
		return nil
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	return append([]taskRunTerminalObserver(nil), n.taskRunTerminalHooks...)
}
