package contract

import "time"

// SchedulerCountersPayload reports cumulative decisions for this daemon lifetime.
type SchedulerCountersPayload struct {
	Cycles              int        `json:"cycles"`
	WakeAttempts        int        `json:"wake_attempts"`
	WakeSkipped         int        `json:"wake_skipped"`
	WakeSucceeded       int        `json:"wake_succeeded"`
	WakeFailed          int        `json:"wake_failed"`
	CapacityWaitingRuns int        `json:"capacity_waiting_runs"`
	SpawnRequested      int        `json:"spawn_requested"`
	NeedsAttention      int        `json:"needs_attention"`
	LastCycleAt         *time.Time `json:"last_cycle_at,omitempty"`
}
