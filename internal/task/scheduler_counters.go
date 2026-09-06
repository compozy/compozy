package task

import "time"

// SchedulerCounters reports cumulative mechanical scheduler decisions since boot.
// CapacityWaitingRuns counts observations across cycles, not unique queued runs.
type SchedulerCounters struct {
	Cycles              int       `json:"cycles"`
	WakeAttempts        int       `json:"wake_attempts"`
	WakeSkipped         int       `json:"wake_skipped"`
	WakeSucceeded       int       `json:"wake_succeeded"`
	WakeFailed          int       `json:"wake_failed"`
	CapacityWaitingRuns int       `json:"capacity_waiting_runs"`
	SpawnRequested      int       `json:"spawn_requested"`
	NeedsAttention      int       `json:"needs_attention"`
	LastCycleAt         time.Time `json:"last_cycle_at,omitzero"`
}

// SetSchedulerCountersReader binds the mechanical scheduler's ephemeral counters.
func (m *Service) SetSchedulerCountersReader(read func() SchedulerCounters) {
	m.schedulerCountersMu.Lock()
	m.schedulerCountersReader = read
	m.schedulerCountersMu.Unlock()
}

func (m *Service) schedulerCounters() *SchedulerCounters {
	m.schedulerCountersMu.RLock()
	read := m.schedulerCountersReader
	m.schedulerCountersMu.RUnlock()
	if read == nil {
		return nil
	}
	counters := read()
	return &counters
}
