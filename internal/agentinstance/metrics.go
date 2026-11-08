package agentinstance

// Metrics counts live paused instances and held inputs within one profile and workspace.
type Metrics struct {
	CheckInToolCalls map[string]map[string]int64 `json:"heartbeat_check_in_tool_calls_total"`
	Picks            map[string]int64            `json:"agent_instance_picks_total"`
	PauseCounts      map[string]int64            `json:"pause_counts"`
	HeldInputCount   int64                       `json:"held_input_count"`
}
