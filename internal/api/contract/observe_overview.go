package contract

import "time"

// ObserveOverviewSchemaVersion identifies the home overview payload contract revision.
const ObserveOverviewSchemaVersion = "observe-overview/v1"

// ObserveOverviewPayload is the home overview read model (global by default; optional workspace scope).
type ObserveOverviewPayload struct {
	SchemaVersion string                        `json:"schema_version"`
	GeneratedAt   time.Time                     `json:"generated_at"`
	Attention     OverviewAttentionPayload      `json:"attention"`
	Today         OverviewTodayPayload          `json:"today"`
	Outcomes      OverviewOutcomesPayload       `json:"outcomes"`
	Usage         OverviewUsagePayload          `json:"usage"`
	Pulse         OverviewPulsePayload          `json:"pulse"`
	Network       OverviewNetworkPayload        `json:"network"`
	System        OverviewSystemPayload         `json:"system"`
	Freshness     TaskDashboardFreshnessPayload `json:"freshness"`
}

// OverviewAttentionPayload lists everything currently waiting on the user.
type OverviewAttentionPayload struct {
	Total  int                            `json:"total"`
	ByKind map[string]int                 `json:"by_kind"`
	Items  []OverviewAttentionItemPayload `json:"items"`
}

// OverviewAttentionItemPayload is one actionable attention row.
type OverviewAttentionItemPayload struct {
	Kind       string    `json:"kind"`
	Title      string    `json:"title"`
	Detail     string    `json:"detail,omitempty"`
	TaskID     string    `json:"task_id,omitempty"`
	RunID      string    `json:"run_id,omitempty"`
	SessionID  string    `json:"session_id,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
	Actions    []string  `json:"actions"`
}

// OverviewTodayPayload summarizes today's terminal work.
type OverviewTodayPayload struct {
	RunsCompleted int `json:"runs_completed"`
	RunsFailed    int `json:"runs_failed"`
	TasksClosed   int `json:"tasks_closed"`
}

// OverviewOutcomesPayload summarizes terminal run outcomes for the fixed window.
type OverviewOutcomesPayload struct {
	WindowDays int                         `json:"window_days"`
	Days       []OverviewOutcomeDayPayload `json:"days"`
	Completed  int                         `json:"completed"`
	Failed     int                         `json:"failed"`
	Canceled   int                         `json:"canceled"`
	SuccessPct float64                     `json:"success_pct"`
}

// OverviewOutcomeDayPayload is one daemon-local day of run outcomes.
type OverviewOutcomeDayPayload struct {
	Date      string `json:"date"`
	Completed int    `json:"completed"`
	Failed    int    `json:"failed"`
	Canceled  int    `json:"canceled"`
}

// OverviewUsagePayload summarizes token usage and estimated cost for the selected window.
type OverviewUsagePayload struct {
	WindowDays    int                         `json:"window_days"`
	RetentionDays int                         `json:"retention_days"`
	Truncated     bool                        `json:"truncated"`
	TotalTokens   int64                       `json:"total_tokens"`
	EstimatedCost *float64                    `json:"estimated_cost,omitempty"`
	CostCurrency  string                      `json:"cost_currency,omitempty"`
	CostStatus    string                      `json:"cost_status,omitempty"`
	Days          []OverviewUsageDayPayload   `json:"days"`
	AgentShare    []OverviewAgentSharePayload `json:"agent_share"`
}

// OverviewUsageDayPayload is one daemon-local day of token usage.
type OverviewUsageDayPayload struct {
	Date   string `json:"date"`
	Tokens int64  `json:"tokens"`
}

// OverviewAgentSharePayload is one agent's fraction of window token usage.
type OverviewAgentSharePayload struct {
	AgentName string  `json:"agent_name"`
	Tokens    int64   `json:"tokens"`
	Fraction  float64 `json:"fraction"`
}

// OverviewPulsePayload summarizes hour-by-weekday activity plus derived insights.
type OverviewPulsePayload struct {
	WindowDays     int                            `json:"window_days"`
	Buckets        []OverviewPulseBucketPayload   `json:"buckets"`
	Busiest        *OverviewPulseBucketPayload    `json:"busiest,omitempty"`
	LongestSession *OverviewLongestSessionPayload `json:"longest_session,omitempty"`
}

// OverviewPulseBucketPayload is one hour-by-weekday event count; weekday 0 is Sunday.
type OverviewPulseBucketPayload struct {
	Weekday int `json:"weekday"`
	Hour    int `json:"hour"`
	Events  int `json:"events"`
}

// OverviewLongestSessionPayload is the longest user session inside the pulse window.
type OverviewLongestSessionPayload struct {
	SessionID       string `json:"session_id"`
	AgentName       string `json:"agent_name"`
	DurationSeconds int64  `json:"duration_seconds"`
	Date            string `json:"date"`
}

// OverviewNetworkPayload carries today-windowed network counters.
type OverviewNetworkPayload struct {
	MessagesToday int `json:"messages_today"`
}

// OverviewSystemPayload carries today-windowed system counters plus retention truth.
type OverviewSystemPayload struct {
	HookRunsToday     int `json:"hook_runs_today"`
	HookFailuresToday int `json:"hook_failures_today"`
	RetentionDays     int `json:"retention_days"`
}
