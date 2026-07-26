package core

import (
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	observepkg "github.com/compozy/agh/internal/observe"
	taskpkg "github.com/compozy/agh/internal/task"
)

// TaskReferencePayloadFromReference converts one task reference into the shared payload.
func TaskReferencePayloadFromReference(record taskpkg.Reference) contract.TaskReferencePayload {
	return contract.TaskReferencePayload{
		ID:              record.ID,
		Identifier:      record.Identifier,
		Title:           taskpkg.RedactClaimTokens(record.Title),
		Status:          record.Status,
		Priority:        record.Priority,
		Owner:           cloneOwnership(record.Owner),
		Scope:           record.Scope,
		WorkspaceID:     record.WorkspaceID,
		LatestEventSeq:  record.LatestEventSeq,
		Paused:          record.Paused,
		EffectivePaused: record.EffectivePaused,
		PausedByTaskID:  record.PausedByTaskID,
	}
}

func cloneRunDesignationSummary(
	summary *taskpkg.RunDesignationSummary,
) *taskpkg.RunDesignationSummary {
	if summary == nil {
		return nil
	}
	return &taskpkg.RunDesignationSummary{
		Index: summary.Index,
		Brief: taskpkg.RedactClaimTokens(strings.TrimSpace(summary.Brief)),
	}
}

// TaskDependencyReferencePayloadsFromReferences converts enriched dependency references into shared payloads.
func TaskDependencyReferencePayloadsFromReferences(
	dependencies []taskpkg.DependencyReference,
) []contract.TaskDependencyReferencePayload {
	payloads := make([]contract.TaskDependencyReferencePayload, 0, len(dependencies))
	for _, dependency := range dependencies {
		payloads = append(payloads, contract.TaskDependencyReferencePayload{
			TaskID:          dependency.TaskID,
			DependsOnTaskID: dependency.DependsOnTaskID,
			Kind:            dependency.Kind,
			CreatedAt:       dependency.CreatedAt,
			DependsOn:       TaskReferencePayloadFromReference(dependency.DependsOn),
		})
	}
	return payloads
}

// TaskTimelineItemPayloadsFromItems converts task timeline items into shared payloads.
func TaskTimelineItemPayloadsFromItems(items []taskpkg.TimelineItem) []contract.TaskTimelineItemPayload {
	payloads := make([]contract.TaskTimelineItemPayload, 0, len(items))
	for _, item := range items {
		payloads = append(payloads, TaskTimelineItemPayloadFromItem(item))
	}
	return payloads
}

// TaskTimelineItemPayloadFromItem converts one task timeline item into the shared payload.
func TaskTimelineItemPayloadFromItem(item taskpkg.TimelineItem) contract.TaskTimelineItemPayload {
	return contract.TaskTimelineItemPayload{
		Sequence:  item.Sequence,
		EventID:   item.EventID,
		Task:      TaskReferencePayloadFromReference(item.Task),
		Run:       TaskRunSummaryPayloadFromSummary(item.Run),
		EventType: item.EventType,
		Actor:     item.Actor,
		Origin:    item.Origin,
		Payload:   taskpkg.RedactClaimTokenJSON(item.Payload),
		Timestamp: item.Timestamp,
	}
}

// TaskStreamEventPayloadFromEvent converts one task live-stream event into the shared payload.
func TaskStreamEventPayloadFromEvent(event taskpkg.StreamEvent) contract.TaskStreamEventPayload {
	return contract.TaskStreamEventPayload{
		Sequence: event.Sequence,
		Type:     event.Type,
		Timeline: TaskTimelineItemPayloadFromItem(event.Timeline),
	}
}

// TaskTreePayloadFromView converts one task tree snapshot into the shared payload.
func TaskTreePayloadFromView(view *taskpkg.TreeView) contract.TaskTreePayload {
	if view == nil {
		return contract.TaskTreePayload{}
	}

	payload := contract.TaskTreePayload{
		Root: TaskTreeNodePayloadFromNode(view.Root),
	}
	if len(view.Descendants) > 0 {
		payload.Descendants = make([]contract.TaskTreeNodePayload, 0, len(view.Descendants))
		for _, node := range view.Descendants {
			payload.Descendants = append(payload.Descendants, TaskTreeNodePayloadFromNode(node))
		}
	}
	return payload
}

// TaskTreeNodePayloadFromNode converts one task tree node into the shared payload.
func TaskTreeNodePayloadFromNode(node taskpkg.TreeNode) contract.TaskTreeNodePayload {
	return contract.TaskTreeNodePayload{
		Task:           TaskReferencePayloadFromReference(node.Task),
		ParentTaskID:   node.ParentTaskID,
		Depth:          node.Depth,
		ChildCount:     node.ChildCount,
		ActiveRun:      TaskRunSummaryPayloadFromSummary(node.ActiveRun),
		LastActivityAt: node.LastActivityAt,
	}
}

// TaskRunSessionPayloadFromSession converts one task run session link into the shared payload.
func TaskRunSessionPayloadFromSession(session *taskpkg.RunSessionRef) *contract.TaskRunSessionPayload {
	if session == nil {
		return nil
	}

	return &contract.TaskRunSessionPayload{
		SessionID:   session.SessionID,
		WorkspaceID: session.WorkspaceID,
		AgentName:   session.AgentName,
		Name:        session.Name,
		State:       session.State,
		CreatedAt:   session.CreatedAt,
		UpdatedAt:   session.UpdatedAt,
	}
}

// TaskTriageStatePayloadFromState converts one triage-state record into the shared payload.
func TaskTriageStatePayloadFromState(state taskpkg.TriageState) contract.TaskTriageStatePayload {
	return contract.TaskTriageStatePayload{
		TaskID:             state.TaskID,
		Actor:              state.Actor,
		Read:               state.Read,
		Archived:           state.Archived,
		Dismissed:          state.Dismissed,
		LastSeenActivityAt: optionalTime(state.LastSeenActivityAt),
		UpdatedAt:          state.UpdatedAt,
	}
}

// TaskDashboardPayloadFromView converts one observer-backed dashboard view into the shared payload.
func TaskDashboardPayloadFromView(view *observepkg.TaskDashboardView) contract.TaskDashboardPayload {
	if view == nil {
		return contract.TaskDashboardPayload{}
	}

	return contract.TaskDashboardPayload{
		Totals:          taskDashboardTotalsPayload(view.Totals),
		Cards:           taskDashboardCardsPayload(view.Cards),
		Queue:           taskDashboardQueuePayload(view.Queue),
		Health:          taskDashboardHealthPayload(view.Health),
		StatusBreakdown: taskDashboardStatusBreakdownPayloads(view.StatusBreakdown),
		ActiveRuns:      taskDashboardActiveRunsPayload(view.ActiveRuns),
		Freshness:       taskDashboardFreshnessPayload(view.Freshness),
	}
}

func taskDashboardTotalsPayload(totals observepkg.TaskDashboardTotals) contract.TaskDashboardTotalsPayload {
	return contract.TaskDashboardTotalsPayload{
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

func taskDashboardCardsPayload(cards observepkg.TaskDashboardCards) contract.TaskDashboardCardsPayload {
	return contract.TaskDashboardCardsPayload{
		InProgress: contract.TaskDashboardInProgressCardPayload{
			Tasks:        cards.InProgress.Tasks,
			ActiveRuns:   cards.InProgress.ActiveRuns,
			RunningRuns:  cards.InProgress.RunningRuns,
			StartingRuns: cards.InProgress.StartingRuns,
			ClaimedRuns:  cards.InProgress.ClaimedRuns,
			QueuedRuns:   cards.InProgress.QueuedRuns,
			HealthStatus: cards.InProgress.HealthStatus,
		},
		Blocked: contract.TaskDashboardBlockedCardPayload{
			Tasks:                cards.Blocked.Tasks,
			AwaitingApproval:     cards.Blocked.AwaitingApproval,
			AwaitingDependencies: cards.Blocked.AwaitingDependencies,
			HealthStatus:         cards.Blocked.HealthStatus,
		},
		Failed: contract.TaskDashboardFailedCardPayload{
			Tasks:        cards.Failed.Tasks,
			FailedRuns:   cards.Failed.FailedRuns,
			ForcedStops:  cards.Failed.ForcedStops,
			HealthStatus: cards.Failed.HealthStatus,
		},
		Latency: contract.TaskDashboardLatencyCardPayload{
			ClaimLatencyMillis: taskLatencyMetricPayload(cards.Latency.ClaimLatencyMillis),
			StartLatencyMillis: taskLatencyMetricPayload(cards.Latency.StartLatencyMillis),
		},
	}
}

func taskLatencyMetricPayload(metric observepkg.LatencyMetric) contract.TaskLatencyMetricPayload {
	return contract.TaskLatencyMetricPayload{
		Samples:       metric.Samples,
		AverageMillis: metric.AverageMillis,
		MaximumMillis: metric.MaximumMillis,
	}
}

func taskDashboardQueuePayload(queue observepkg.TaskDashboardQueue) contract.TaskDashboardQueuePayload {
	payload := contract.TaskDashboardQueuePayload{
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

	payload.Depth = make([]contract.TaskDashboardQueueDepthPayload, 0, len(queue.Depth))
	for _, item := range queue.Depth {
		payload.Depth = append(payload.Depth, contract.TaskDashboardQueueDepthPayload{
			ChannelID:           item.ChannelID,
			Count:               item.Count,
			OldestQueuedAt:      item.OldestQueuedAt,
			OldestQueueAgeMilli: item.OldestQueueAgeMilli,
		})
	}
	return payload
}

func taskDashboardHealthPayload(health observepkg.TaskDashboardHealth) contract.TaskDashboardHealthPayload {
	return contract.TaskDashboardHealthPayload{
		Status:           health.Status,
		StuckRuns:        health.StuckRuns,
		ActiveOrphanRuns: health.ActiveOrphanRuns,
		QueueBacklog:     health.QueueBacklog,
	}
}

func taskDashboardStatusBreakdownPayloads(
	items []observepkg.TaskDashboardStatusBreakdown,
) []contract.TaskDashboardStatusBreakdownPayload {
	if len(items) == 0 {
		return nil
	}

	payloads := make([]contract.TaskDashboardStatusBreakdownPayload, 0, len(items))
	for _, item := range items {
		payloads = append(payloads, contract.TaskDashboardStatusBreakdownPayload{
			Status:       item.Status,
			Count:        item.Count,
			SharePercent: item.SharePercent,
		})
	}
	return payloads
}

func taskDashboardActiveRunsPayload(
	activeRuns observepkg.TaskDashboardActiveRuns,
) contract.TaskDashboardActiveRunsPayload {
	payload := contract.TaskDashboardActiveRunsPayload{
		Total:    activeRuns.Total,
		Running:  activeRuns.Running,
		Starting: activeRuns.Starting,
		Claimed:  activeRuns.Claimed,
		Queued:   activeRuns.Queued,
	}
	if len(activeRuns.Items) == 0 {
		return payload
	}

	payload.Items = make([]contract.TaskDashboardActiveRunPayload, 0, len(activeRuns.Items))
	for _, item := range activeRuns.Items {
		payload.Items = append(payload.Items, contract.TaskDashboardActiveRunPayload{
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
) contract.TaskDashboardFreshnessPayload {
	return contract.TaskDashboardFreshnessPayload{
		ObservedAt:       freshness.ObservedAt,
		LatestActivityAt: freshness.LatestActivityAt,
		AgeMilli:         freshness.AgeMilli,
		StaleAfterMilli:  freshness.StaleAfterMilli,
		HasLiveWork:      freshness.HasLiveWork,
		Status:           freshness.Status,
		Stale:            freshness.Stale,
	}
}
