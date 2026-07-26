package cli

import (
	"strconv"
	"strings"

	taskpkg "github.com/compozy/agh/internal/task"
)

func taskRunBundle(item TaskRunRecord) outputBundle {
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			return renderHumanSection("Task Run", []keyValue{
				{Label: "ID", Value: stringOrDash(item.ID)},
				{Label: taskTaskValue, Value: stringOrDash(item.TaskID)},
				{Label: taskStatusValue, Value: stringOrDash(item.Status.String())},
				{Label: taskAttemptValue, Value: intOrDash(item.Attempt)},
				{Label: "Previous Run", Value: stringOrDash(item.PreviousRunID)},
				{Label: "Failure Kind", Value: stringOrDash(item.FailureKind)},
				{
					Label: taskClaimedByValue,
					Value: stringOrDash(formatTaskActorPtr(item.ClaimedBy)),
				},
				{Label: taskSessionValue, Value: stringOrDash(item.SessionID)},
				{Label: taskOriginValue, Value: stringOrDash(formatTaskOrigin(item.Origin))},
				{Label: "Idempotency Key", Value: stringOrDash(item.IdempotencyKey)},
				{
					Label: taskParticipationChannelValue,
					Value: resolvedParticipationChannel(item.ResolvedNetworkParticipation),
				},
				{Label: taskQueuedValue, Value: stringOrDash(formatTime(item.QueuedAt))},
				{Label: "Claimed", Value: stringOrDash(formatTimePtr(item.ClaimedAt))},
				{Label: taskStartedValue, Value: stringOrDash(formatTimePtr(item.StartedAt))},
				{Label: taskEndedValue, Value: stringOrDash(formatTimePtr(item.EndedAt))},
				{Label: taskErrorValue, Value: stringOrDash(item.Error)},
				{Label: taskResultValue, Value: stringOrDash(compactJSON(item.Result))},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject("task_run", []string{
				"id",
				taskTaskIDKey,
				taskStatusKey,
				taskAttemptKey,
				"previous_run_id",
				taskFailureKindKey,
				taskClaimedByKey,
				taskSessionIDKey,
				taskOriginKey,
				"idempotency_key",
				taskParticipationChannelKey,
				taskQueuedAtKey,
				"claimed_at",
				taskStartedAtKey,
				taskEndedAtKey,
				taskErrorKey,
				"result",
			}, []string{
				item.ID,
				item.TaskID,
				item.Status.String(),
				strconv.Itoa(item.Attempt),
				item.PreviousRunID,
				item.FailureKind,
				formatTaskActorPtr(item.ClaimedBy),
				item.SessionID,
				formatTaskOrigin(item.Origin),
				item.IdempotencyKey,
				resolvedParticipationChannelRaw(item.ResolvedNetworkParticipation),
				formatTime(item.QueuedAt),
				formatTimePtr(item.ClaimedAt),
				formatTimePtr(item.StartedAt),
				formatTimePtr(item.EndedAt),
				item.Error,
				compactJSON(item.Result),
			}), nil
		},
	}
}

func retryTaskRunBundle(record *RetryTaskRunRecord) outputBundle {
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			return renderHumanSection("Task Run Retry", []keyValue{
				{Label: "Source Run", Value: stringOrDash(record.PreviousRun.ID)},
				{Label: "Source Status", Value: stringOrDash(record.PreviousRun.Status.String())},
				{Label: "New Run", Value: stringOrDash(record.Run.ID)},
				{Label: taskTaskValue, Value: stringOrDash(record.Run.TaskID)},
				{Label: taskStatusValue, Value: stringOrDash(record.Run.Status.String())},
				{Label: taskAttemptValue, Value: intOrDash(record.Run.Attempt)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject("task_run_retry", []string{
				"source_run_id",
				"source_status",
				"new_run_id",
				taskTaskIDKey,
				taskStatusKey,
				taskAttemptKey,
			}, []string{
				record.PreviousRun.ID,
				record.PreviousRun.Status.String(),
				record.Run.ID,
				record.Run.TaskID,
				record.Run.Status.String(),
				strconv.Itoa(record.Run.Attempt),
			}), nil
		},
	}
}

func recoverTaskRunBundle(record *RetryTaskRunRecord) outputBundle {
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			return renderHumanSection("Task Run Recovery", []keyValue{
				{Label: "Recovered Run", Value: stringOrDash(record.PreviousRun.ID)},
				{Label: "Recovered Status", Value: stringOrDash(record.PreviousRun.Status.String())},
				{Label: "New Run", Value: stringOrDash(record.Run.ID)},
				{Label: taskTaskValue, Value: stringOrDash(record.Run.TaskID)},
				{Label: taskStatusValue, Value: stringOrDash(record.Run.Status.String())},
				{Label: taskAttemptValue, Value: intOrDash(record.Run.Attempt)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject("task_run_recovery", []string{
				"recovered_run_id",
				"recovered_status",
				"new_run_id",
				taskTaskIDKey,
				taskStatusKey,
				taskAttemptKey,
			}, []string{
				record.PreviousRun.ID,
				record.PreviousRun.Status.String(),
				record.Run.ID,
				record.Run.TaskID,
				record.Run.Status.String(),
				strconv.Itoa(record.Run.Attempt),
			}), nil
		},
	}
}

func bulkForceTaskRunBundle(record BulkForceTaskRunRecord) outputBundle {
	return listBundle(
		record,
		record.Results,
		"Task Run Force Operations",
		[]string{"Run ID", "OK", taskStatusValue, taskErrorValue},
		"task_run_force_operations",
		[]string{taskRunIDKey, "ok", taskStatusKey, taskErrorKey},
		func(item BulkForceTaskRunItemRecord) []string {
			return []string{
				item.RunID,
				strconv.FormatBool(item.OK),
				bulkForceTaskRunStatus(item),
				bulkForceTaskRunError(item),
			}
		},
		func(item BulkForceTaskRunItemRecord) []string {
			return []string{
				item.RunID,
				strconv.FormatBool(item.OK),
				bulkForceTaskRunStatus(item),
				bulkForceTaskRunError(item),
			}
		},
	)
}

func bulkForceTaskRunStatus(item BulkForceTaskRunItemRecord) string {
	if item.Run == nil {
		return ""
	}
	return item.Run.Status.String()
}

func bulkForceTaskRunError(item BulkForceTaskRunItemRecord) string {
	if item.Error == nil {
		return ""
	}
	if strings.TrimSpace(item.Error.Error) != "" {
		return item.Error.Error
	}
	if item.Error.Diagnostic != nil {
		return item.Error.Diagnostic.Code
	}
	return ""
}

func taskRunListBundle(items []TaskRunRecord) outputBundle {
	return listBundle(
		items,
		items,
		"Task Runs",
		[]string{
			"ID",
			taskStatusValue,
			taskAttemptValue,
			taskSessionValue,
			taskClaimedByValue,
			taskParticipationChannelValue,
			taskQueuedValue,
			taskStartedValue,
			taskEndedValue,
			taskErrorValue,
		},
		"task_runs",
		[]string{
			"id",
			taskStatusKey,
			taskAttemptKey,
			taskSessionIDKey,
			taskClaimedByKey,
			taskParticipationChannelKey,
			taskQueuedAtKey,
			taskStartedAtKey,
			taskEndedAtKey,
			taskErrorKey,
		},
		func(item TaskRunRecord) []string {
			return []string{
				stringOrDash(item.ID),
				stringOrDash(item.Status.String()),
				intOrDash(item.Attempt),
				stringOrDash(item.SessionID),
				stringOrDash(formatTaskActorPtr(item.ClaimedBy)),
				resolvedParticipationChannel(item.ResolvedNetworkParticipation),
				stringOrDash(formatTime(item.QueuedAt)),
				stringOrDash(formatTimePtr(item.StartedAt)),
				stringOrDash(formatTimePtr(item.EndedAt)),
				stringOrDash(item.Error),
			}
		},
		func(item TaskRunRecord) []string {
			return []string{
				item.ID,
				item.Status.String(),
				strconv.Itoa(item.Attempt),
				item.SessionID,
				formatTaskActorPtr(item.ClaimedBy),
				resolvedParticipationChannelRaw(item.ResolvedNetworkParticipation),
				formatTime(item.QueuedAt),
				formatTimePtr(item.StartedAt),
				formatTimePtr(item.EndedAt),
				item.Error,
			}
		},
	)
}

func taskChildRows(items []TaskSummaryRecord) [][]string {
	rows := make([][]string, 0, len(items))
	for idx := range items {
		item := &items[idx]
		rows = append(rows, []string{
			stringOrDash(item.ID),
			stringOrDash(item.Identifier),
			stringOrDash(string(item.Scope)),
			stringOrDash(item.WorkspaceID),
			stringOrDash(string(item.Status)),
			stringOrDash(formatTaskOwnership(item.Owner)),
			stringOrDash(item.Title),
		})
	}
	return rows
}

func taskChildToonRows(items []TaskSummaryRecord) [][]string {
	rows := make([][]string, 0, len(items))
	for idx := range items {
		item := &items[idx]
		rows = append(rows, []string{
			item.ID,
			item.Identifier,
			string(item.Scope),
			item.WorkspaceID,
			string(item.Status),
			formatTaskOwnership(item.Owner),
			item.Title,
		})
	}
	return rows
}

func taskDependencyRows(items []TaskDependencyRecord) [][]string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			stringOrDash(item.TaskID),
			stringOrDash(item.DependsOnTaskID),
			stringOrDash(string(item.Kind)),
			stringOrDash(formatTime(item.CreatedAt)),
		})
	}
	return rows
}

func taskDependencyToonRows(items []TaskDependencyRecord) [][]string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			item.TaskID,
			item.DependsOnTaskID,
			string(item.Kind),
			formatTime(item.CreatedAt),
		})
	}
	return rows
}

func taskRunRows(items []TaskRunRecord) [][]string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			stringOrDash(item.ID),
			stringOrDash(item.Status.String()),
			intOrDash(item.Attempt),
			stringOrDash(item.SessionID),
			stringOrDash(formatTaskActorPtr(item.ClaimedBy)),
			resolvedParticipationChannel(item.ResolvedNetworkParticipation),
			stringOrDash(formatTime(item.QueuedAt)),
			stringOrDash(formatTimePtr(item.StartedAt)),
			stringOrDash(formatTimePtr(item.EndedAt)),
			stringOrDash(item.Error),
		})
	}
	return rows
}

func taskRunToonRows(items []TaskRunRecord) [][]string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			item.ID,
			item.Status.String(),
			strconv.Itoa(item.Attempt),
			item.SessionID,
			formatTaskActorPtr(item.ClaimedBy),
			resolvedParticipationChannelRaw(item.ResolvedNetworkParticipation),
			formatTime(item.QueuedAt),
			formatTimePtr(item.StartedAt),
			formatTimePtr(item.EndedAt),
			item.Error,
		})
	}
	return rows
}

func taskEventRows(items []TaskEventRecord) [][]string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			stringOrDash(item.ID),
			stringOrDash(item.EventType),
			stringOrDash(item.RunID),
			stringOrDash(formatTaskActor(item.Actor)),
			stringOrDash(formatTaskOrigin(item.Origin)),
			stringOrDash(formatTime(item.Timestamp)),
		})
	}
	return rows
}

func taskEventToonRows(items []TaskEventRecord) [][]string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			item.ID,
			item.EventType,
			item.RunID,
			formatTaskActor(item.Actor),
			formatTaskOrigin(item.Origin),
			formatTime(item.Timestamp),
		})
	}
	return rows
}

func formatTaskOwnership(owner *taskpkg.Ownership) string {
	if owner == nil {
		return ""
	}
	return firstNonEmpty(
		string(owner.Kind)+":"+strings.TrimSpace(owner.Ref),
		strings.TrimSpace(owner.Ref),
	)
}

func formatTaskActor(actor taskpkg.ActorIdentity) string {
	return firstNonEmpty(
		string(actor.Kind)+":"+strings.TrimSpace(actor.Ref),
		strings.TrimSpace(actor.Ref),
	)
}

func formatTaskActorPtr(actor *taskpkg.ActorIdentity) string {
	if actor == nil {
		return ""
	}
	return formatTaskActor(*actor)
}

func formatTaskOrigin(origin taskpkg.Origin) string {
	return firstNonEmpty(
		string(origin.Kind)+":"+strings.TrimSpace(origin.Ref),
		strings.TrimSpace(origin.Ref),
	)
}
