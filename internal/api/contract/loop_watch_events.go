package contract

import "time"

// LoopWatchEventKind is the closed public vocabulary of subscribable watch-events
// hook kinds. It mirrors the shipped rows of the loop engine's watch-events family
// registry (internal/loop.SupportedWatchEvents); a drift test in internal/daemon
// keeps this contract enum and the registry in lockstep. ADR-004 governs the phased
// matrix — phases B/C append rows here as they land.
type LoopWatchEventKind string

const (
	LoopWatchEventTaskStatusChanged LoopWatchEventKind = "task.status_changed"
	LoopWatchEventTaskBlocked       LoopWatchEventKind = "task.blocked"
	LoopWatchEventTaskUnblocked     LoopWatchEventKind = "task.unblocked"
	LoopWatchEventTaskNeedsAttn     LoopWatchEventKind = "task.needs_attention"
	LoopWatchEventTaskRecovered     LoopWatchEventKind = "task.recovered"
	LoopWatchEventTaskRunCompleted  LoopWatchEventKind = "task.run.completed" // #nosec G101 -- event enum.
	LoopWatchEventTaskRunFailed     LoopWatchEventKind = "task.run.failed"
	LoopWatchEventLoopTerminal      LoopWatchEventKind = "loop.terminal"
	LoopWatchEventLoopNodeTerminal  LoopWatchEventKind = "loop.node.terminal"       // #nosec G101 -- event enum.
	LoopWatchEventAutomationDone    LoopWatchEventKind = "automation.run.completed" // #nosec G101 -- event enum.
	LoopWatchEventAutomationFailed  LoopWatchEventKind = "automation.run.failed"    // #nosec G101 -- event enum.
	LoopWatchEventNetworkMessage    LoopWatchEventKind = "network.message.persisted"
	LoopWatchEventNetworkThread     LoopWatchEventKind = "network.thread.opened"
	LoopWatchEventNetworkDirect     LoopWatchEventKind = "network.direct_room.opened"
	LoopWatchEventNetworkWorkOpen   LoopWatchEventKind = "network.work.opened"
	LoopWatchEventNetworkWorkChange LoopWatchEventKind = "network.work.transitioned"
	LoopWatchEventNetworkWorkClosed LoopWatchEventKind = "network.work.closed"
	LoopWatchEventCoordinatorSpawn  LoopWatchEventKind = "coordinator.spawned"
	LoopWatchEventCoordinatorDecide LoopWatchEventKind = "coordinator.decision"
	LoopWatchEventCoordinatorStop   LoopWatchEventKind = "coordinator.stopped"
	LoopWatchEventCoordinatorFail   LoopWatchEventKind = "coordinator.failed"
	LoopWatchEventPostRecord        LoopWatchEventKind = "event.post_record" // #nosec G101 -- event enum.
)

// LoopWatchEventKindValues returns the closed watch-events kind vocabulary in
// ADR-004 order. It is the single source consumed by the OpenAPI enum registration and
// the generated web `LOOP_WATCH_EVENT_KINDS` array, so the authoring kind-select never
// hand-authors the enum in TypeScript.
func LoopWatchEventKindValues() []string {
	return []string{
		string(LoopWatchEventTaskStatusChanged),
		string(LoopWatchEventTaskBlocked),
		string(LoopWatchEventTaskUnblocked),
		string(LoopWatchEventTaskNeedsAttn),
		string(LoopWatchEventTaskRecovered),
		string(LoopWatchEventTaskRunCompleted),
		string(LoopWatchEventTaskRunFailed),
		string(LoopWatchEventLoopTerminal),
		string(LoopWatchEventLoopNodeTerminal),
		string(LoopWatchEventAutomationDone),
		string(LoopWatchEventAutomationFailed),
		string(LoopWatchEventNetworkMessage),
		string(LoopWatchEventNetworkThread),
		string(LoopWatchEventNetworkDirect),
		string(LoopWatchEventNetworkWorkOpen),
		string(LoopWatchEventNetworkWorkChange),
		string(LoopWatchEventNetworkWorkClosed),
		string(LoopWatchEventCoordinatorSpawn),
		string(LoopWatchEventCoordinatorDecide),
		string(LoopWatchEventCoordinatorStop),
		string(LoopWatchEventCoordinatorFail),
		string(LoopWatchEventPostRecord),
	}
}

// LoopWatchEventSubscription is one parked watch-events subscription projected for a
// loop-run detail read-model. Kind names a subscribable hook event; Filter is the CEL
// expression evaluated at wake (empty = match every event of this kind in the run's
// workspace).
type LoopWatchEventSubscription struct {
	Kind   LoopWatchEventKind `json:"kind"`
	Filter string             `json:"filter,omitempty"`
}

// LoopWatchEventsState is the parked watch-events node read-model surfaced on a loop-run
// detail response. It is present only while the run's current generation is dormant at a
// watch-events node; an absent value means the run is not currently parked on events, so
// clients render nothing (truthful UI). Subscriptions and Cursors project the durable
// pending output_ref; LastWakeAt is the run's last coordinator-progress timestamp.
type LoopWatchEventsState struct {
	Subscriptions []LoopWatchEventSubscription `json:"subscriptions"`
	Cursors       map[string]int64             `json:"cursors,omitempty"`
	LastWakeAt    *time.Time                   `json:"last_wake_at,omitempty"`
}
