package agentinstance

// Metrics counts live paused instances and held inputs within one profile and workspace.
type Metrics struct {
	PauseCounts    map[string]int64 `json:"pause_counts"`
	HeldInputCount int64            `json:"held_input_count"`
}
