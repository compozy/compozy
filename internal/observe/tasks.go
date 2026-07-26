package observe

import (
	"time"

	"github.com/compozy/agh/internal/network/participation"

	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

const (
	tasksReceivedKey = "received"
	tasksRejectedKey = "rejected"
)

const (
	taskIngressAuditEnqueueAction = "task.run.enqueue"
	taskIngressChannelMismatch    = "channel_mismatch"
	taskEventCanceled             = "task.canceled"
	taskEventRunEnqueued          = "task.run_enqueued"
	taskEventRunForceStopped      = "task.run_force_stopped"
	taskEventRunRecovered         = "task.run_recovered"
	taskHealthStatusOK            = "ok"
	taskHealthStatusWarn          = "warn"
)

// TaskSummaryQuery filters the current task summary view.
type TaskSummaryQuery struct {
	Scope                taskpkg.Scope      `json:"scope,omitempty"`
	WorkspaceID          string             `json:"workspace_id,omitempty"`
	OwnerKind            taskpkg.OwnerKind  `json:"owner_kind,omitempty"`
	OwnerRef             string             `json:"owner_ref,omitempty"`
	ParticipationChannel string             `json:"participation_channel,omitempty"`
	OriginKind           taskpkg.OriginKind `json:"origin_kind,omitempty"`
	Search               string             `json:"search,omitempty"`
}

// Validate ensures the summary query uses supported filters.
func (q TaskSummaryQuery) Validate() error {
	if q.Scope.Normalize() != "" {
		if err := q.Scope.Validate("task_summary_query.scope"); err != nil {
			return err
		}
	}
	if q.OwnerKind.Normalize() != "" {
		if err := q.OwnerKind.Validate("task_summary_query.owner_kind"); err != nil {
			return err
		}
	}
	if q.OriginKind.Normalize() != "" {
		if err := q.OriginKind.Validate("task_summary_query.origin_kind"); err != nil {
			return err
		}
	}
	return nil
}

// TaskMetricsQuery filters audit-derived metrics and current queue metrics.
type TaskMetricsQuery struct {
	Since                time.Time          `json:"since"`
	ParticipationChannel string             `json:"participation_channel,omitempty"`
	OriginKind           taskpkg.OriginKind `json:"origin_kind,omitempty"`
}

// Validate ensures the metrics query uses supported filters.
func (q TaskMetricsQuery) Validate() error {
	if q.OriginKind.Normalize() != "" {
		if err := q.OriginKind.Validate("task_metrics_query.origin_kind"); err != nil {
			return err
		}
	}
	return nil
}

// TaskDashboardQuery filters the observer-backed task dashboard read model.
type TaskDashboardQuery struct {
	Scope                taskpkg.Scope      `json:"scope,omitempty"`
	WorkspaceID          string             `json:"workspace_id,omitempty"`
	OwnerKind            taskpkg.OwnerKind  `json:"owner_kind,omitempty"`
	OwnerRef             string             `json:"owner_ref,omitempty"`
	ParticipationChannel string             `json:"participation_channel,omitempty"`
	OriginKind           taskpkg.OriginKind `json:"origin_kind,omitempty"`
}

// Validate ensures the dashboard query uses supported filters.
func (q TaskDashboardQuery) Validate() error {
	if q.Scope.Normalize() != "" {
		if err := taskpkg.ValidateScopeBinding(
			q.Scope,
			q.WorkspaceID,
			"task_dashboard_query",
			"workspace_id",
		); err != nil {
			return err
		}
	}
	return q.summaryQuery().Validate()
}

func (q TaskDashboardQuery) summaryQuery() TaskSummaryQuery {
	return TaskSummaryQuery{
		Scope:                q.Scope,
		WorkspaceID:          q.WorkspaceID,
		OwnerKind:            q.OwnerKind,
		OwnerRef:             q.OwnerRef,
		ParticipationChannel: q.ParticipationChannel,
		OriginKind:           q.OriginKind,
	}
}

func (q TaskDashboardQuery) metricsQuery(since time.Time) TaskMetricsQuery {
	return TaskMetricsQuery{
		Since:                since,
		ParticipationChannel: q.ParticipationChannel,
		OriginKind:           q.OriginKind,
	}
}

// TaskStatusTotal reports one current task-count bucket.
type TaskStatusTotal struct {
	Scope     taskpkg.Scope  `json:"scope"`
	Status    taskpkg.Status `json:"status"`
	ChannelID string         `json:"channel_id,omitempty"`
	Count     int            `json:"count"`
}

// TaskOriginTotal reports one current task-origin bucket.
type TaskOriginTotal struct {
	OriginKind taskpkg.OriginKind `json:"origin_kind"`
	ChannelID  string             `json:"channel_id,omitempty"`
	Count      int                `json:"count"`
}

// TaskRunTotal reports one current task-run bucket.
type TaskRunTotal struct {
	Status     taskpkg.RunStatus  `json:"status"`
	OriginKind taskpkg.OriginKind `json:"origin_kind"`
	ChannelID  string             `json:"channel_id,omitempty"`
	Count      int                `json:"count"`
}

// TaskOwnerTotal reports one current ownership bucket.
type TaskOwnerTotal struct {
	OwnerKind taskpkg.OwnerKind `json:"owner_kind"`
	OwnerRef  string            `json:"owner_ref"`
	Count     int               `json:"count"`
}

// TaskQueueDepth reports queued work by channel.
type TaskQueueDepth struct {
	ChannelID           string    `json:"channel_id,omitempty"`
	Count               int       `json:"count"`
	OldestQueuedAt      time.Time `json:"oldest_queued_at"`
	OldestQueueAgeMilli int64     `json:"oldest_queue_age_ms"`
}

// Summary exposes the current read-side task summary buckets.
type Summary struct {
	TotalTasks  int               `json:"total_tasks"`
	TotalRuns   int               `json:"total_runs"`
	TaskTotals  []TaskStatusTotal `json:"task_totals,omitempty"`
	TaskOrigins []TaskOriginTotal `json:"task_origins,omitempty"`
	RunTotals   []TaskRunTotal    `json:"run_totals,omitempty"`
	OwnerTotals []TaskOwnerTotal  `json:"owner_totals,omitempty"`
	QueueDepth  []TaskQueueDepth  `json:"queue_depth,omitempty"`
}

// LatencyMetric summarizes one task-run latency family in milliseconds.
type LatencyMetric struct {
	Samples       int   `json:"samples"`
	AverageMillis int64 `json:"average_ms"`
	MaximumMillis int64 `json:"maximum_ms"`
}

// TaskCancelRequestTotal reports cancellation requests grouped by origin.
type TaskCancelRequestTotal struct {
	OriginKind taskpkg.OriginKind `json:"origin_kind"`
	Count      int                `json:"count"`
}

// TaskRecoveryTotals reports boot-recovery outcomes grouped by manager action.
type TaskRecoveryTotals struct {
	Requeued      int `json:"requeued"`
	MarkedRunning int `json:"marked_running"`
	Failed        int `json:"failed"`
}

// TaskMetrics exposes current counters and latency summaries for the task domain.
type TaskMetrics struct {
	TasksTotal              []TaskStatusTotal        `json:"tasks_total,omitempty"`
	TaskRunsTotal           []TaskRunTotal           `json:"task_runs_total,omitempty"`
	TaskQueueDepth          []TaskQueueDepth         `json:"task_queue_depth,omitempty"`
	TaskCancelRequestsTotal []TaskCancelRequestTotal `json:"task_cancel_requests_total,omitempty"`
	TaskForcedStopsTotal    int                      `json:"task_forced_stops_total"`
	TaskClaimLatencyMillis  LatencyMetric            `json:"task_claim_latency_ms"`
	TaskStartLatencyMillis  LatencyMetric            `json:"task_start_latency_ms"`
	DuplicateIngressTotal   int                      `json:"duplicate_ingress_total"`
	ChannelMismatchTotal    int                      `json:"channel_mismatch_total"`
	RecoveryTotals          TaskRecoveryTotals       `json:"recovery_totals"`
}

// StuckTaskRun reports one run that exceeded the configured claimed/starting/running threshold.
type StuckTaskRun struct {
	TaskID     string             `json:"task_id"`
	RunID      string             `json:"run_id"`
	Status     taskpkg.RunStatus  `json:"status"`
	OriginKind taskpkg.OriginKind `json:"origin_kind"`
	ChannelID  string             `json:"channel_id,omitempty"`
	SessionID  string             `json:"session_id,omitempty"`
	AgeMillis  int64              `json:"age_ms"`
}

// TaskHealth exposes the current operational task-health view.
type TaskHealth struct {
	Status                     string             `json:"status"`
	QueueDepthTotal            int                `json:"queue_depth_total"`
	OldestQueuedAt             time.Time          `json:"oldest_queued_at"`
	OldestQueueAgeMilli        int64              `json:"oldest_queue_age_ms"`
	QueueDepth                 []TaskQueueDepth   `json:"queue_depth,omitempty"`
	StuckRuns                  []StuckTaskRun     `json:"stuck_runs,omitempty"`
	ActiveOrphanRuns           int                `json:"active_orphan_runs"`
	TaskTotals                 []TaskStatusTotal  `json:"task_totals,omitempty"`
	RunTotals                  []TaskRunTotal     `json:"run_totals,omitempty"`
	OwnerTotals                []TaskOwnerTotal   `json:"owner_totals,omitempty"`
	ForcedStopsSinceStart      int                `json:"forced_stops_since_start"`
	DuplicateIngressSinceStart int                `json:"duplicate_ingress_since_start"`
	ChannelMismatchSinceStart  int                `json:"channel_mismatch_since_start"`
	RecoverySinceStart         TaskRecoveryTotals `json:"recovery_since_start"`
}

// TaskDashboardView exposes the observer-owned aggregate payload for the Paper task dashboard.
type TaskDashboardView struct {
	Totals          TaskDashboardTotals            `json:"totals"`
	Cards           TaskDashboardCards             `json:"cards"`
	StatusBreakdown []TaskDashboardStatusBreakdown `json:"status_breakdown,omitempty"`
	Queue           TaskDashboardQueue             `json:"queue"`
	Health          TaskDashboardHealth            `json:"health"`
	ActiveRuns      TaskDashboardActiveRuns        `json:"active_runs"`
	Freshness       TaskDashboardFreshness         `json:"freshness"`
}

// TaskDashboardTotals collapses the current task and run totals into chart-friendly counters.
type TaskDashboardTotals struct {
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

// TaskDashboardCards exposes dashboard-ready card values without leaking raw summary buckets.
type TaskDashboardCards struct {
	InProgress TaskDashboardInProgressCard `json:"in_progress"`
	Blocked    TaskDashboardBlockedCard    `json:"blocked"`
	Failed     TaskDashboardFailedCard     `json:"failed"`
	Latency    TaskDashboardLatencyCard    `json:"latency"`
}

// TaskDashboardInProgressCard summarizes active work and live run pressure.
type TaskDashboardInProgressCard struct {
	Tasks        int    `json:"tasks"`
	ActiveRuns   int    `json:"active_runs"`
	RunningRuns  int    `json:"running_runs"`
	StartingRuns int    `json:"starting_runs"`
	ClaimedRuns  int    `json:"claimed_runs"`
	QueuedRuns   int    `json:"queued_runs"`
	HealthStatus string `json:"health_status"`
}

// TaskDashboardBlockedCard summarizes blocked work and approval/dependency pressure.
type TaskDashboardBlockedCard struct {
	Tasks                int    `json:"tasks"`
	AwaitingApproval     int    `json:"awaiting_approval"`
	AwaitingDependencies int    `json:"awaiting_dependencies"`
	HealthStatus         string `json:"health_status"`
}

// TaskDashboardFailedCard summarizes failed work and disruptive run outcomes.
type TaskDashboardFailedCard struct {
	Tasks        int    `json:"tasks"`
	FailedRuns   int    `json:"failed_runs"`
	ForcedStops  int    `json:"forced_stops"`
	HealthStatus string `json:"health_status"`
}

// TaskDashboardLatencyCard exposes current run-queue latency summaries for operator cards.
type TaskDashboardLatencyCard struct {
	ClaimLatencyMillis LatencyMetric `json:"claim_latency_ms"`
	StartLatencyMillis LatencyMetric `json:"start_latency_ms"`
}

// TaskDashboardStatusBreakdown reports one aggregated task status bucket for chart rendering.
type TaskDashboardStatusBreakdown struct {
	Status       taskpkg.Status `json:"status"`
	Count        int            `json:"count"`
	SharePercent int            `json:"share_percent"`
}

// TaskDashboardQueue reports backlog state for queued task work.
type TaskDashboardQueue struct {
	Total                 int              `json:"total"`
	Depth                 []TaskQueueDepth `json:"depth,omitempty"`
	OldestQueuedAt        time.Time        `json:"oldest_queued_at"`
	OldestQueueAgeMilli   int64            `json:"oldest_queue_age_ms"`
	BacklogWarning        bool             `json:"backlog_warning"`
	BacklogStatus         string           `json:"backlog_status"`
	BacklogThresholdMilli int64            `json:"backlog_threshold_ms"`
}

// TaskDashboardHealth reports warning-oriented dashboard health indicators.
type TaskDashboardHealth struct {
	Status           string `json:"status"`
	StuckRuns        int    `json:"stuck_runs"`
	ActiveOrphanRuns int    `json:"active_orphan_runs"`
	QueueBacklog     bool   `json:"queue_backlog"`
}

// TaskDashboardActiveRuns summarizes the currently active run set and exposes recent cards.
type TaskDashboardActiveRuns struct {
	Total    int                      `json:"total"`
	Running  int                      `json:"running"`
	Starting int                      `json:"starting"`
	Claimed  int                      `json:"claimed"`
	Queued   int                      `json:"queued"`
	Items    []TaskDashboardActiveRun `json:"items,omitempty"`
}

// TaskDashboardActiveRun exposes one recent active-run card payload.
type TaskDashboardActiveRun struct {
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

// TaskDashboardFreshness exposes the recency and stale-warning state of the dashboard snapshot.
type TaskDashboardFreshness struct {
	ObservedAt       time.Time `json:"observed_at"`
	LatestActivityAt time.Time `json:"latest_activity_at"`
	AgeMilli         int64     `json:"age_ms"`
	StaleAfterMilli  int64     `json:"stale_after_ms"`
	HasLiveWork      bool      `json:"has_live_work"`
	Status           string    `json:"status"`
	Stale            bool      `json:"stale"`
}

type taskSnapshot struct {
	tasks        []taskpkg.Summary
	runs         []taskpkg.Run
	events       []taskpkg.Event
	audits       []store.NetworkAuditEntry
	tasksByID    map[string]taskpkg.Summary
	runsByID     map[string]taskpkg.Run
	taskChannels map[string]string
}

type taskRecoveryPayload struct {
	Action taskpkg.RunBootRecoveryAction `json:"action"`
}
