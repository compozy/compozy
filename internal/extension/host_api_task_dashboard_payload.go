package extensionpkg

import (
	"strings"

	apicontract "github.com/compozy/agh/internal/api/contract"

	observepkg "github.com/compozy/agh/internal/observe"
	taskpkg "github.com/compozy/agh/internal/task"
)

func taskDashboardPayloadFromView(view *observepkg.TaskDashboardView) apicontract.TaskDashboardPayload {
	if view == nil {
		return apicontract.TaskDashboardPayload{}
	}

	return apicontract.TaskDashboardPayload{
		Totals:          taskDashboardTotalsPayload(view.Totals),
		Cards:           taskDashboardCardsPayload(view.Cards),
		StatusBreakdown: taskDashboardStatusBreakdownPayloads(view.StatusBreakdown),
		Queue:           taskDashboardQueuePayload(view.Queue),
		Health:          taskDashboardHealthPayload(view.Health),
		ActiveRuns:      taskDashboardActiveRunsPayload(view.ActiveRuns),
		Freshness:       taskDashboardFreshnessPayload(view.Freshness),
	}
}

func taskDashboardTotalsPayload(totals observepkg.TaskDashboardTotals) apicontract.TaskDashboardTotalsPayload {
	return apicontract.TaskDashboardTotalsPayload{
		TasksTotal:             totals.TasksTotal,
		RunsTotal:              totals.RunsTotal,
		DraftTasks:             totals.DraftTasks,
		PendingTasks:           totals.PendingTasks,
		ReadyTasks:             totals.ReadyTasks,
		InProgressTasks:        totals.InProgressTasks,
		BlockedTasks:           totals.BlockedTasks,
		CompletedTasks:         totals.CompletedTasks,
		FailedTasks:            totals.FailedTasks,
		CanceledTasks:          totals.CanceledTasks,
		AwaitingApprovalTasks:  totals.AwaitingApprovalTasks,
		DependencyBlockedTasks: totals.DependencyBlockedTasks,
		QueuedRuns:             totals.QueuedRuns,
		ClaimedRuns:            totals.ClaimedRuns,
		StartingRuns:           totals.StartingRuns,
		RunningRuns:            totals.RunningRuns,
		CompletedRuns:          totals.CompletedRuns,
		FailedRuns:             totals.FailedRuns,
		CanceledRuns:           totals.CanceledRuns,
		ActiveRuns:             totals.ActiveRuns,
	}
}

func taskDashboardCardsPayload(cards observepkg.TaskDashboardCards) apicontract.TaskDashboardCardsPayload {
	return apicontract.TaskDashboardCardsPayload{
		InProgress: apicontract.TaskDashboardInProgressCardPayload{
			Tasks:        cards.InProgress.Tasks,
			ActiveRuns:   cards.InProgress.ActiveRuns,
			RunningRuns:  cards.InProgress.RunningRuns,
			StartingRuns: cards.InProgress.StartingRuns,
			ClaimedRuns:  cards.InProgress.ClaimedRuns,
			QueuedRuns:   cards.InProgress.QueuedRuns,
			HealthStatus: cards.InProgress.HealthStatus,
		},
		Blocked: apicontract.TaskDashboardBlockedCardPayload{
			Tasks:                cards.Blocked.Tasks,
			AwaitingApproval:     cards.Blocked.AwaitingApproval,
			AwaitingDependencies: cards.Blocked.AwaitingDependencies,
			HealthStatus:         cards.Blocked.HealthStatus,
		},
		Failed: apicontract.TaskDashboardFailedCardPayload{
			Tasks:        cards.Failed.Tasks,
			FailedRuns:   cards.Failed.FailedRuns,
			ForcedStops:  cards.Failed.ForcedStops,
			HealthStatus: cards.Failed.HealthStatus,
		},
		Latency: apicontract.TaskDashboardLatencyCardPayload{
			ClaimLatencyMillis: taskLatencyMetricPayload(cards.Latency.ClaimLatencyMillis),
			StartLatencyMillis: taskLatencyMetricPayload(cards.Latency.StartLatencyMillis),
		},
	}
}

func taskLatencyMetricPayload(metric observepkg.LatencyMetric) apicontract.TaskLatencyMetricPayload {
	return apicontract.TaskLatencyMetricPayload{
		Samples:       metric.Samples,
		AverageMillis: metric.AverageMillis,
		MaximumMillis: metric.MaximumMillis,
	}
}

func taskDashboardStatusBreakdownPayloads(
	items []observepkg.TaskDashboardStatusBreakdown,
) []apicontract.TaskDashboardStatusBreakdownPayload {
	if len(items) == 0 {
		return nil
	}

	payloads := make([]apicontract.TaskDashboardStatusBreakdownPayload, len(items))
	for i := range items {
		item := &items[i]
		payloads[i] = apicontract.TaskDashboardStatusBreakdownPayload{
			Status:       item.Status,
			Count:        item.Count,
			SharePercent: item.SharePercent,
		}
	}
	return payloads
}

func taskDashboardQueuePayload(queue observepkg.TaskDashboardQueue) apicontract.TaskDashboardQueuePayload {
	payload := apicontract.TaskDashboardQueuePayload{
		Total:                 queue.Total,
		OldestQueuedAt:        queue.OldestQueuedAt,
		OldestQueueAgeMilli:   queue.OldestQueueAgeMilli,
		BacklogWarning:        queue.BacklogWarning,
		BacklogStatus:         queue.BacklogStatus,
		BacklogThresholdMilli: queue.BacklogThresholdMilli,
	}
	if len(queue.Depth) == 0 {
		return payload
	}

	payload.Depth = make([]apicontract.TaskDashboardQueueDepthPayload, 0, len(queue.Depth))
	for _, item := range queue.Depth {
		payload.Depth = append(payload.Depth, apicontract.TaskDashboardQueueDepthPayload{
			ChannelID:           item.ChannelID,
			Count:               item.Count,
			OldestQueuedAt:      item.OldestQueuedAt,
			OldestQueueAgeMilli: item.OldestQueueAgeMilli,
		})
	}
	return payload
}

func taskDashboardHealthPayload(health observepkg.TaskDashboardHealth) apicontract.TaskDashboardHealthPayload {
	return apicontract.TaskDashboardHealthPayload{
		Status:           health.Status,
		StuckRuns:        health.StuckRuns,
		ActiveOrphanRuns: health.ActiveOrphanRuns,
		QueueBacklog:     health.QueueBacklog,
	}
}

func taskDashboardActiveRunsPayload(
	activeRuns observepkg.TaskDashboardActiveRuns,
) apicontract.TaskDashboardActiveRunsPayload {
	payload := apicontract.TaskDashboardActiveRunsPayload{
		Total:    activeRuns.Total,
		Running:  activeRuns.Running,
		Starting: activeRuns.Starting,
		Claimed:  activeRuns.Claimed,
		Queued:   activeRuns.Queued,
	}
	if len(activeRuns.Items) == 0 {
		return payload
	}

	payload.Items = make([]apicontract.TaskDashboardActiveRunPayload, 0, len(activeRuns.Items))
	for _, item := range activeRuns.Items {
		payload.Items = append(payload.Items, apicontract.TaskDashboardActiveRunPayload{
			TaskID:                       item.TaskID,
			TaskIdentifier:               item.TaskIdentifier,
			TaskTitle:                    taskpkg.RedactClaimTokens(strings.TrimSpace(item.TaskTitle)),
			TaskStatus:                   item.TaskStatus,
			TaskPriority:                 item.TaskPriority,
			TaskOwner:                    cloneOwnership(item.TaskOwner),
			Scope:                        item.Scope,
			WorkspaceID:                  item.WorkspaceID,
			LatestEventSeq:               item.LatestEventSeq,
			RunID:                        item.RunID,
			RunStatus:                    item.RunStatus,
			Attempt:                      item.Attempt,
			MaxAttempts:                  item.MaxAttempts,
			SessionID:                    item.SessionID,
			ResolvedNetworkParticipation: cloneResolvedParticipation(item.ResolvedNetworkParticipation),
			LastActivityAt:               item.LastActivityAt,
			AgeMilli:                     item.AgeMilli,
			HealthStatus:                 item.HealthStatus,
			Stuck:                        item.Stuck,
			Error:                        taskpkg.RedactClaimTokens(strings.TrimSpace(item.Error)),
		})
	}
	return payload
}

func taskDashboardFreshnessPayload(
	freshness observepkg.TaskDashboardFreshness,
) apicontract.TaskDashboardFreshnessPayload {
	return apicontract.TaskDashboardFreshnessPayload{
		ObservedAt:       freshness.ObservedAt,
		LatestActivityAt: freshness.LatestActivityAt,
		AgeMilli:         freshness.AgeMilli,
		StaleAfterMilli:  freshness.StaleAfterMilli,
		HasLiveWork:      freshness.HasLiveWork,
		Status:           freshness.Status,
		Stale:            freshness.Stale,
	}
}
