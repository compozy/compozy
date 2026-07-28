package daemon

const (
	loopManagedGoalKind            = "goal"
	loopManagedPromptKindCompact   = "compact"
	loopManagedReadySlotPhase      = "ready-slot"
	daemonUnknownValue             = "unknown"
	loopManagedContextStateUnknown = daemonUnknownValue
	daemonStatusField              = "status"
	daemonAgentField               = "agent"
	goalSnapshotStatusComplete     = "complete"
	goalSnapshotStatusBlocked      = "blocked"
	goalSnapshotStatusActive       = "active"
)
