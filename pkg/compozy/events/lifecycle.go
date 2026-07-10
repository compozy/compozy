package events

// SettlementScope identifies which lifecycle boundary an event settles.
type SettlementScope string

const (
	// SettlementScopeNone means the event reports progress without settling a lifecycle.
	SettlementScopeNone SettlementScope = ""
	// SettlementScopeRun means the event terminates the complete run.
	SettlementScopeRun SettlementScope = "run"
	// SettlementScopeQueue means the event settles a task-multi queue.
	SettlementScopeQueue SettlementScope = "queue"
	// SettlementScopeParallel means the event settles a parallel-task orchestrator.
	SettlementScopeParallel SettlementScope = "parallel"
)

// SettlementScopeForKind classifies public run, queue, and parallel settlement events.
func SettlementScopeForKind(kind EventKind) SettlementScope {
	switch kind {
	case EventKindRunCrashed,
		EventKindRunCompleted,
		EventKindRunFailed,
		EventKindRunCancelled,
		EventKindRunRecoveryExhausted:
		return SettlementScopeRun
	case EventKindTaskRunMultipleQueueCompleted,
		EventKindTaskRunMultipleQueueFailed,
		EventKindTaskRunMultipleQueueCanceled:
		return SettlementScopeQueue
	case EventKindTaskParallelCompleted,
		EventKindTaskParallelFailed,
		EventKindTaskParallelRolledBack,
		EventKindTaskParallelCanceled:
		return SettlementScopeParallel
	default:
		return SettlementScopeNone
	}
}

// IsRunTerminalKind reports whether kind terminates the complete run stream.
func IsRunTerminalKind(kind EventKind) bool {
	return SettlementScopeForKind(kind) == SettlementScopeRun
}

// RequiresDurablePublish reports whether kind must be fsynced before live publication.
func RequiresDurablePublish(kind EventKind) bool {
	return SettlementScopeForKind(kind) != SettlementScopeNone || kind == EventKindTaskParallelWaveCompleted
}
