package observe

import (
	"context"

	"errors"

	"math"

	"strings"
	"time"

	taskpkg "github.com/compozy/agh/internal/task"
)

// QueryTaskSummary returns the current task summary buckets filtered by the supplied view.
func (o *Observer) QueryTaskSummary(ctx context.Context, query TaskSummaryQuery) (Summary, error) {
	snapshot, err := o.loadTaskSnapshot(ctx, query)
	if err != nil {
		return Summary{}, err
	}
	return taskSummaryFromSnapshot(snapshot, o.now), nil
}

// QueryTaskMetrics returns task-domain counters and latency summaries derived from durable state and audit rows.
func (o *Observer) QueryTaskMetrics(ctx context.Context, query TaskMetricsQuery) (TaskMetrics, error) {
	if ctx == nil {
		return TaskMetrics{}, errors.New("observe: task metrics context is required")
	}
	if err := query.Validate(); err != nil {
		return TaskMetrics{}, err
	}

	snapshot, err := o.loadTaskSnapshot(ctx, TaskSummaryQuery{
		ParticipationChannel: query.ParticipationChannel,
	})
	if err != nil {
		return TaskMetrics{}, err
	}
	return taskMetricsFromSnapshot(snapshot, query, o.now), nil
}

// QueryTaskDashboard returns the observer-backed aggregate task dashboard view.
func (o *Observer) QueryTaskDashboard(ctx context.Context, query TaskDashboardQuery) (TaskDashboardView, error) {
	if ctx == nil {
		return TaskDashboardView{}, errors.New("observe: task dashboard context is required")
	}
	if err := query.Validate(); err != nil {
		return TaskDashboardView{}, err
	}

	snapshot, err := o.loadTaskSnapshot(ctx, query.summaryQuery())
	if err != nil {
		return TaskDashboardView{}, err
	}

	summary := taskSummaryFromSnapshot(snapshot, o.now)
	metrics := taskMetricsFromSnapshot(snapshot, query.metricsQuery(o.startedAt), o.now)
	health, err := o.taskHealthFromSnapshot(ctx, snapshot, summary, metrics)
	if err != nil {
		return TaskDashboardView{}, err
	}

	return o.taskDashboardFromSnapshot(snapshot, summary, metrics, health), nil
}

func (o *Observer) collectTaskHealth(ctx context.Context) (TaskHealth, error) {
	if ctx == nil {
		return TaskHealth{}, errors.New("observe: task health context is required")
	}

	snapshot, err := o.loadTaskSnapshot(ctx, TaskSummaryQuery{})
	if err != nil {
		return TaskHealth{}, err
	}
	summary := taskSummaryFromSnapshot(snapshot, o.now)
	metrics := taskMetricsFromSnapshot(snapshot, TaskMetricsQuery{Since: o.startedAt}, o.now)
	return o.taskHealthFromSnapshot(ctx, snapshot, summary, metrics)
}

func (o *Observer) taskHealthFromSnapshot(
	ctx context.Context,
	snapshot taskSnapshot,
	summary Summary,
	metrics TaskMetrics,
) (TaskHealth, error) {
	stuckRuns := findStuckRuns(snapshot.runs, o.now(), o.taskHealthConfig)
	sortStuckRuns(stuckRuns)
	activeOrphans, err := o.countActiveOrphanRuns(ctx, snapshot.runs)
	if err != nil {
		return TaskHealth{}, err
	}

	queueDepthTotal := 0
	var oldestQueuedAt time.Time
	var oldestQueuedAge int64
	for _, item := range summary.QueueDepth {
		queueDepthTotal += item.Count
		if item.OldestQueuedAt.IsZero() {
			continue
		}
		if oldestQueuedAt.IsZero() || item.OldestQueuedAt.Before(oldestQueuedAt) {
			oldestQueuedAt = item.OldestQueuedAt
			oldestQueuedAge = item.OldestQueueAgeMilli
		}
	}

	status := taskHealthStatusOK
	if len(stuckRuns) > 0 || activeOrphans > 0 {
		status = taskHealthStatusWarn
	}

	return TaskHealth{
		Status:                     status,
		QueueDepthTotal:            queueDepthTotal,
		OldestQueuedAt:             oldestQueuedAt,
		OldestQueueAgeMilli:        oldestQueuedAge,
		QueueDepth:                 summary.QueueDepth,
		StuckRuns:                  stuckRuns,
		ActiveOrphanRuns:           activeOrphans,
		TaskTotals:                 summary.TaskTotals,
		RunTotals:                  summary.RunTotals,
		OwnerTotals:                summary.OwnerTotals,
		ForcedStopsSinceStart:      metrics.TaskForcedStopsTotal,
		DuplicateIngressSinceStart: metrics.DuplicateIngressTotal,
		ChannelMismatchSinceStart:  metrics.ChannelMismatchTotal,
		RecoverySinceStart:         metrics.RecoveryTotals,
	}, nil
}

func taskSummaryFromSnapshot(snapshot taskSnapshot, now func() time.Time) Summary {
	return Summary{
		TotalTasks:  len(snapshot.tasks),
		TotalRuns:   len(snapshot.runs),
		TaskTotals:  summarizeTasks(snapshot.tasks, snapshot.taskChannels),
		TaskOrigins: summarizeTaskOrigins(snapshot.tasks, snapshot.taskChannels),
		RunTotals:   summarizeRuns(snapshot.runs),
		OwnerTotals: summarizeOwners(snapshot.tasks),
		QueueDepth:  summarizeQueueDepth(snapshot.runs, now),
	}
}

func taskMetricsFromSnapshot(snapshot taskSnapshot, query TaskMetricsQuery, now func() time.Time) TaskMetrics {
	runs := filterRunsByOrigin(snapshot.runs, query.OriginKind)
	events := filterTaskEvents(snapshot.events, snapshot.taskChannels, snapshot.runsByID, query)
	audits := filterTaskIngressAudits(snapshot.audits, query)
	duplicateIngress := max(countAcceptedEnqueueAudits(audits)-countNetworkEnqueueEvents(events), 0)

	return TaskMetrics{
		TasksTotal: summarizeTasks(
			filterTasksByOrigin(snapshot.tasks, query.OriginKind),
			snapshot.taskChannels,
		),
		TaskRunsTotal:           summarizeRuns(runs),
		TaskQueueDepth:          summarizeQueueDepth(runs, now),
		TaskCancelRequestsTotal: summarizeCancelRequests(events),
		TaskForcedStopsTotal:    countEventsByType(events, taskEventRunForceStopped),
		TaskClaimLatencyMillis:  summarizeClaimLatency(runs),
		TaskStartLatencyMillis:  summarizeStartLatency(runs),
		DuplicateIngressTotal:   duplicateIngress,
		ChannelMismatchTotal:    countChannelMismatchAudits(audits),
		RecoveryTotals:          summarizeRecovery(events),
	}
}

func (o *Observer) taskDashboardFromSnapshot(
	snapshot taskSnapshot,
	summary Summary,
	metrics TaskMetrics,
	health TaskHealth,
) TaskDashboardView {
	totals := taskDashboardTotalsFromSnapshot(snapshot, summary, metrics)
	queue := taskDashboardQueueFromRows(summary.QueueDepth, o.taskDashboardConfig, o.now)
	healthSummary := taskDashboardHealthFromHealth(health, queue.BacklogWarning)

	return TaskDashboardView{
		Totals:          totals,
		Cards:           taskDashboardCardsFromTotals(totals, metrics, healthSummary, health.ForcedStopsSinceStart),
		StatusBreakdown: taskDashboardStatusBreakdownFromTotals(totals),
		Queue:           queue,
		Health:          healthSummary,
		ActiveRuns: taskDashboardActiveRunsFromSnapshot(
			snapshot,
			o.now,
			o.taskDashboardConfig,
			o.taskHealthConfig,
		),
		Freshness: taskDashboardFreshnessFromSnapshot(snapshot, o.now, o.taskDashboardConfig),
	}
}

func taskDashboardTotalsFromSnapshot(
	snapshot taskSnapshot,
	summary Summary,
	metrics TaskMetrics,
) TaskDashboardTotals {
	awaitingApproval := countAwaitingApprovalTasks(snapshot.tasks)
	dependencyBlocked := countDependencyBlockedTasks(snapshot.tasks)

	totals := TaskDashboardTotals{
		TasksTotal:            summary.TotalTasks,
		RunsTotal:             summary.TotalRuns,
		DraftTasks:            countTaskStatus(summary.TaskTotals, taskpkg.TaskStatusDraft),
		PendingTasks:          countTaskStatus(summary.TaskTotals, taskpkg.TaskStatusPending),
		ReadyTasks:            countTaskStatus(summary.TaskTotals, taskpkg.TaskStatusReady),
		InProgressTasks:       countTaskStatus(summary.TaskTotals, taskpkg.TaskStatusInProgress),
		BlockedTasks:          countTaskStatus(summary.TaskTotals, taskpkg.TaskStatusBlocked),
		CompletedTasks:        countTaskStatus(summary.TaskTotals, taskpkg.TaskStatusCompleted),
		FailedTasks:           countTaskStatus(summary.TaskTotals, taskpkg.TaskStatusFailed),
		CanceledTasks:         countTaskStatus(summary.TaskTotals, taskpkg.TaskStatusCanceled),
		AwaitingApprovalTasks: awaitingApproval,
		QueuedRuns:            countRunStatus(metrics.TaskRunsTotal, taskpkg.TaskRunStatusQueued),
		ClaimedRuns:           countRunStatus(metrics.TaskRunsTotal, taskpkg.TaskRunStatusClaimed),
		StartingRuns:          countRunStatus(metrics.TaskRunsTotal, taskpkg.TaskRunStatusStarting),
		RunningRuns:           countRunStatus(metrics.TaskRunsTotal, taskpkg.TaskRunStatusRunning),
		CompletedRuns:         countRunStatus(metrics.TaskRunsTotal, taskpkg.TaskRunStatusCompleted),
		FailedRuns:            countRunStatus(metrics.TaskRunsTotal, taskpkg.TaskRunStatusFailed),
		CanceledRuns:          countRunStatus(metrics.TaskRunsTotal, taskpkg.TaskRunStatusCanceled),
	}
	totals.DependencyBlockedTasks = dependencyBlocked
	totals.ActiveRuns = totals.QueuedRuns + totals.ClaimedRuns + totals.StartingRuns + totals.RunningRuns
	return totals
}

func taskDashboardCardsFromTotals(
	totals TaskDashboardTotals,
	metrics TaskMetrics,
	health TaskDashboardHealth,
	forcedStops int,
) TaskDashboardCards {
	return TaskDashboardCards{
		InProgress: TaskDashboardInProgressCard{
			Tasks:        totals.InProgressTasks,
			ActiveRuns:   totals.ActiveRuns,
			RunningRuns:  totals.RunningRuns,
			StartingRuns: totals.StartingRuns,
			ClaimedRuns:  totals.ClaimedRuns,
			QueuedRuns:   totals.QueuedRuns,
			HealthStatus: health.Status,
		},
		Blocked: TaskDashboardBlockedCard{
			Tasks:                totals.BlockedTasks,
			AwaitingApproval:     totals.AwaitingApprovalTasks,
			AwaitingDependencies: totals.DependencyBlockedTasks,
			HealthStatus:         dashboardStatusForCount(totals.BlockedTasks),
		},
		Failed: TaskDashboardFailedCard{
			Tasks:        totals.FailedTasks,
			FailedRuns:   totals.FailedRuns,
			ForcedStops:  forcedStops,
			HealthStatus: dashboardStatusForAny(totals.FailedTasks > 0 || totals.FailedRuns > 0 || forcedStops > 0),
		},
		Latency: TaskDashboardLatencyCard{
			ClaimLatencyMillis: metrics.TaskClaimLatencyMillis,
			StartLatencyMillis: metrics.TaskStartLatencyMillis,
		},
	}
}

func taskDashboardStatusBreakdownFromTotals(totals TaskDashboardTotals) []TaskDashboardStatusBreakdown {
	type statusCount struct {
		status taskpkg.Status
		count  int
	}

	rows := []statusCount{
		{status: taskpkg.TaskStatusCompleted, count: totals.CompletedTasks},
		{status: taskpkg.TaskStatusPending, count: totals.PendingTasks},
		{status: taskpkg.TaskStatusInProgress, count: totals.InProgressTasks},
		{status: taskpkg.TaskStatusReady, count: totals.ReadyTasks},
		{status: taskpkg.TaskStatusBlocked, count: totals.BlockedTasks},
		{status: taskpkg.TaskStatusFailed, count: totals.FailedTasks},
		{status: taskpkg.TaskStatusCanceled, count: totals.CanceledTasks},
		{status: taskpkg.TaskStatusDraft, count: totals.DraftTasks},
	}

	breakdown := make([]TaskDashboardStatusBreakdown, 0, len(rows))
	for _, row := range rows {
		if row.count <= 0 || totals.TasksTotal <= 0 {
			continue
		}
		breakdown = append(breakdown, TaskDashboardStatusBreakdown{
			Status:       row.status,
			Count:        row.count,
			SharePercent: int(math.Round(float64(row.count) * 100 / float64(totals.TasksTotal))),
		})
	}
	return breakdown
}

func taskDashboardQueueFromRows(
	rows []TaskQueueDepth,
	cfg taskDashboardConfig,
	now func() time.Time,
) TaskDashboardQueue {
	queue := TaskDashboardQueue{
		Depth: rows,
	}
	threshold := max(cfg.backlogWarnAfter, 0)
	queue.BacklogThresholdMilli = threshold.Milliseconds()

	for _, item := range rows {
		queue.Total += item.Count
		if item.OldestQueuedAt.IsZero() {
			continue
		}
		if queue.OldestQueuedAt.IsZero() || item.OldestQueuedAt.Before(queue.OldestQueuedAt) {
			queue.OldestQueuedAt = item.OldestQueuedAt
			queue.OldestQueueAgeMilli = item.OldestQueueAgeMilli
		}
	}

	if queue.Total > 0 && threshold > 0 && time.Duration(queue.OldestQueueAgeMilli)*time.Millisecond >= threshold {
		queue.BacklogWarning = true
		queue.BacklogStatus = taskHealthStatusWarn
	} else {
		queue.BacklogStatus = taskHealthStatusOK
	}
	if now != nil && !queue.OldestQueuedAt.IsZero() {
		queue.OldestQueueAgeMilli = safeSince(now(), queue.OldestQueuedAt).Milliseconds()
		if queue.Total > 0 && threshold > 0 && time.Duration(queue.OldestQueueAgeMilli)*time.Millisecond >= threshold {
			queue.BacklogWarning = true
			queue.BacklogStatus = taskHealthStatusWarn
		}
	}

	return queue
}

func taskDashboardHealthFromHealth(health TaskHealth, queueBacklog bool) TaskDashboardHealth {
	status := health.Status
	if queueBacklog {
		status = taskHealthStatusWarn
	}
	if strings.TrimSpace(status) == "" {
		status = taskHealthStatusOK
	}

	return TaskDashboardHealth{
		Status:           status,
		StuckRuns:        len(health.StuckRuns),
		ActiveOrphanRuns: health.ActiveOrphanRuns,
		QueueBacklog:     queueBacklog,
	}
}

func taskDashboardActiveRunsFromSnapshot(
	snapshot taskSnapshot,
	now func() time.Time,
	cfg taskDashboardConfig,
	healthCfg TaskHealthConfig,
) TaskDashboardActiveRuns {
	currentTime := dashboardNow(now)
	items := dashboardActiveRuns(snapshot.runs)
	stuckByID := dashboardStuckRunSet(snapshot.runs, currentTime, healthCfg)

	activeRuns := taskDashboardActiveRunCounts(items)
	activeRuns.Items = taskDashboardActiveRunItems(
		items,
		snapshot.tasksByID,
		currentTime,
		cfg.activeRunLimit,
		stuckByID,
	)
	return activeRuns
}

func taskDashboardFreshnessFromSnapshot(
	snapshot taskSnapshot,
	now func() time.Time,
	cfg taskDashboardConfig,
) TaskDashboardFreshness {
	observedAt := time.Now().UTC()
	if now != nil {
		observedAt = now().UTC()
	}

	latestActivity := latestTaskSnapshotActivityAt(snapshot)
	staleAfter := max(cfg.staleAfter, 0)
	hasLiveWork := snapshotHasLiveWork(snapshot)
	age := safeSince(observedAt, latestActivity)

	status, stale := classifyTaskDashboardFreshness(latestActivity, hasLiveWork, age, staleAfter)
	return TaskDashboardFreshness{
		ObservedAt:       observedAt,
		LatestActivityAt: latestActivity,
		AgeMilli:         age.Milliseconds(),
		StaleAfterMilli:  staleAfter.Milliseconds(),
		HasLiveWork:      hasLiveWork,
		Status:           status,
		Stale:            stale,
	}
}
