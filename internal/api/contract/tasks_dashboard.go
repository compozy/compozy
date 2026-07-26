package contract

import (
	"time"

	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
)

// TaskDashboardPayload is the observer-backed task dashboard response payload.
type TaskDashboardPayload struct {
	Totals          TaskDashboardTotalsPayload            `json:"totals"`
	Cards           TaskDashboardCardsPayload             `json:"cards"`
	StatusBreakdown []TaskDashboardStatusBreakdownPayload `json:"status_breakdown,omitempty"`
	Queue           TaskDashboardQueuePayload             `json:"queue"`
	Health          TaskDashboardHealthPayload            `json:"health"`
	ActiveRuns      TaskDashboardActiveRunsPayload        `json:"active_runs"`
	Freshness       TaskDashboardFreshnessPayload         `json:"freshness"`
}

// TaskDashboardTotalsPayload collapses current task and run totals into operator-facing counters.
type TaskDashboardTotalsPayload struct {
	TasksTotal             int `json:"tasks_total"`
	RunsTotal              int `json:"runs_total"`
	DraftTasks             int `json:"draft_tasks"`
	PendingTasks           int `json:"pending_tasks"`
	ReadyTasks             int `json:"ready_tasks"`
	InProgressTasks        int `json:"in_progress_tasks"`
	BlockedTasks           int `json:"blocked_tasks"`
	CompletedTasks         int `json:"completed_tasks"`
	FailedTasks            int `json:"failed_tasks"`
	CanceledTasks          int `json:"canceled_tasks"`
	AwaitingApprovalTasks  int `json:"awaiting_approval_tasks"`
	DependencyBlockedTasks int `json:"dependency_blocked_tasks"`
	QueuedRuns             int `json:"queued_runs"`
	ClaimedRuns            int `json:"claimed_runs"`
	StartingRuns           int `json:"starting_runs"`
	RunningRuns            int `json:"running_runs"`
	CompletedRuns          int `json:"completed_runs"`
	FailedRuns             int `json:"failed_runs"`
	CanceledRuns           int `json:"canceled_runs"`
	ActiveRuns             int `json:"active_runs"`
}

// TaskDashboardCardsPayload exposes dashboard-ready card values.
type TaskDashboardCardsPayload struct {
	InProgress TaskDashboardInProgressCardPayload `json:"in_progress"`
	Blocked    TaskDashboardBlockedCardPayload    `json:"blocked"`
	Failed     TaskDashboardFailedCardPayload     `json:"failed"`
	Latency    TaskDashboardLatencyCardPayload    `json:"latency"`
}

// TaskDashboardInProgressCardPayload summarizes active work and live-run pressure.
type TaskDashboardInProgressCardPayload struct {
	Tasks        int    `json:"tasks"`
	ActiveRuns   int    `json:"active_runs"`
	RunningRuns  int    `json:"running_runs"`
	StartingRuns int    `json:"starting_runs"`
	ClaimedRuns  int    `json:"claimed_runs"`
	QueuedRuns   int    `json:"queued_runs"`
	HealthStatus string `json:"health_status"`
}

// TaskDashboardBlockedCardPayload summarizes approval and dependency pressure.
type TaskDashboardBlockedCardPayload struct {
	Tasks                int    `json:"tasks"`
	AwaitingApproval     int    `json:"awaiting_approval"`
	AwaitingDependencies int    `json:"awaiting_dependencies"`
	HealthStatus         string `json:"health_status"`
}

// TaskDashboardFailedCardPayload summarizes failed work and disruptive run outcomes.
type TaskDashboardFailedCardPayload struct {
	Tasks        int    `json:"tasks"`
	FailedRuns   int    `json:"failed_runs"`
	ForcedStops  int    `json:"forced_stops"`
	HealthStatus string `json:"health_status"`
}

// TaskLatencyMetricPayload exposes one task latency metric family.
type TaskLatencyMetricPayload struct {
	Samples       int   `json:"samples"`
	AverageMillis int64 `json:"average_ms"`
	MaximumMillis int64 `json:"maximum_ms"`
}

// TaskDashboardLatencyCardPayload exposes queue and start latency summaries.
type TaskDashboardLatencyCardPayload struct {
	ClaimLatencyMillis TaskLatencyMetricPayload `json:"claim_latency_ms"`
	StartLatencyMillis TaskLatencyMetricPayload `json:"start_latency_ms"`
}

// TaskDashboardStatusBreakdownPayload reports one aggregated task-status bucket.
type TaskDashboardStatusBreakdownPayload struct {
	Status       taskpkg.Status `json:"status"`
	Count        int            `json:"count"`
	SharePercent int            `json:"share_percent"`
}

// TaskDashboardQueuePayload reports current queue backlog state.
type TaskDashboardQueuePayload struct {
	Total                 int                              `json:"total"`
	Depth                 []TaskDashboardQueueDepthPayload `json:"depth,omitempty"`
	OldestQueuedAt        time.Time                        `json:"oldest_queued_at"`
	OldestQueueAgeMilli   int64                            `json:"oldest_queue_age_ms"`
	BacklogWarning        bool                             `json:"backlog_warning"`
	BacklogStatus         string                           `json:"backlog_status"`
	BacklogThresholdMilli int64                            `json:"backlog_threshold_ms"`
}

// TaskDashboardQueueDepthPayload reports queued work by channel.
type TaskDashboardQueueDepthPayload struct {
	ChannelID           string    `json:"channel_id,omitempty"`
	Count               int       `json:"count"`
	OldestQueuedAt      time.Time `json:"oldest_queued_at"`
	OldestQueueAgeMilli int64     `json:"oldest_queue_age_ms"`
}

// TaskDashboardHealthPayload exposes warning-oriented dashboard health indicators.
type TaskDashboardHealthPayload struct {
	Status           string `json:"status"`
	StuckRuns        int    `json:"stuck_runs"`
	ActiveOrphanRuns int    `json:"active_orphan_runs"`
	QueueBacklog     bool   `json:"queue_backlog"`
}

// TaskDashboardActiveRunsPayload summarizes the currently active run set and recent cards.
type TaskDashboardActiveRunsPayload struct {
	Total    int                             `json:"total"`
	Running  int                             `json:"running"`
	Starting int                             `json:"starting"`
	Claimed  int                             `json:"claimed"`
	Queued   int                             `json:"queued"`
	Items    []TaskDashboardActiveRunPayload `json:"items,omitempty"`
}

// TaskDashboardActiveRunPayload exposes one recent active-run card payload.
type TaskDashboardActiveRunPayload struct {
	TaskID                       string              `json:"task_id"`
	TaskIdentifier               string              `json:"task_identifier,omitempty"`
	TaskTitle                    string              `json:"task_title"`
	TaskStatus                   taskpkg.Status      `json:"task_status"`
	TaskPriority                 taskpkg.Priority    `json:"task_priority,omitempty"`
	TaskOwner                    *taskpkg.Ownership  `json:"task_owner,omitempty"`
	Scope                        taskpkg.Scope       `json:"scope"`
	WorkspaceID                  string              `json:"workspace_id,omitempty"`
	LatestEventSeq               int64               `json:"latest_event_seq"`
	RunID                        string              `json:"run_id"`
	RunStatus                    taskpkg.RunStatus   `json:"run_status"`
	Attempt                      int                 `json:"attempt"`
	MaxAttempts                  int                 `json:"max_attempts"`
	SessionID                    string              `json:"session_id,omitempty"`
	ResolvedNetworkParticipation *participation.Spec `json:"resolved_network_participation,omitempty"`
	LastActivityAt               time.Time           `json:"last_activity_at"`
	AgeMilli                     int64               `json:"age_ms"`
	HealthStatus                 string              `json:"health_status"`
	Stuck                        bool                `json:"stuck"`
	Error                        string              `json:"error,omitempty"`
}

// TaskDashboardFreshnessPayload exposes recency and stale-warning state for the dashboard snapshot.
type TaskDashboardFreshnessPayload struct {
	ObservedAt       time.Time `json:"observed_at"`
	LatestActivityAt time.Time `json:"latest_activity_at"`
	AgeMilli         int64     `json:"age_ms"`
	StaleAfterMilli  int64     `json:"stale_after_ms"`
	HasLiveWork      bool      `json:"has_live_work"`
	Status           string    `json:"status"`
	Stale            bool      `json:"stale"`
}

// TaskTriageStatePayload is the shared actor-scoped task triage state.
type TaskTriageStatePayload struct {
	TaskID             string                `json:"task_id"`
	Actor              taskpkg.ActorIdentity `json:"actor"`
	Read               bool                  `json:"read"`
	Archived           bool                  `json:"archived"`
	Dismissed          bool                  `json:"dismissed"`
	LastSeenActivityAt *time.Time            `json:"last_seen_activity_at,omitempty"`
	UpdatedAt          time.Time             `json:"updated_at"`
}
