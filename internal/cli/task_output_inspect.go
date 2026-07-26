package cli

import (
	"strconv"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
)

func taskDeleteBundle(id string) outputBundle {
	item := struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}{
		ID:     strings.TrimSpace(id),
		Status: taskDeletedKey,
	}
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			return renderHumanSection(taskTaskValue, []keyValue{
				{Label: "ID", Value: stringOrDash(item.ID)},
				{Label: taskStatusValue, Value: item.Status},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				taskTaskKey,
				[]string{"id", taskStatusKey},
				[]string{item.ID, item.Status},
			), nil
		},
	}
}

func taskExecutionProfileDeleteBundle(id string) outputBundle {
	item := struct {
		TaskID string `json:"task_id"`
		Status string `json:"status"`
	}{
		TaskID: strings.TrimSpace(id),
		Status: taskDeletedKey,
	}
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			return renderHumanSection("Task Execution Profile", []keyValue{
				{Label: taskTaskIDValue, Value: stringOrDash(item.TaskID)},
				{Label: taskStatusValue, Value: item.Status},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"task_execution_profile",
				[]string{taskTaskIDKey, taskStatusKey},
				[]string{item.TaskID, item.Status},
			), nil
		},
	}
}

func taskDetailBundle(detail *TaskDetailRecord) outputBundle {
	return outputBundle{
		jsonValue: detail,
		human:     func() (string, error) { return renderTaskDetailHuman(detail) },
		toon:      func() (string, error) { return renderTaskDetailToon(detail) },
	}
}

func taskInspectBundle(record *TaskInspectRecord) outputBundle {
	return outputBundle{
		jsonValue: record,
		human:     func() (string, error) { return renderTaskInspectHuman(record) },
		toon:      func() (string, error) { return renderTaskInspectToon(record) },
	}
}

func renderTaskInspectHuman(record *TaskInspectRecord) (string, error) {
	blocks := []string{
		renderHumanSection("Task Inspect", []keyValue{
			{Label: "Target", Value: stringOrDash(record.Target)},
			{Label: taskTaskValue, Value: stringOrDash(record.Task.ID)},
			{Label: taskTitleValue, Value: stringOrDash(record.Task.Title)},
			{Label: taskStatusValue, Value: stringOrDash(string(record.Task.Status))},
			{Label: "Current Run", Value: stringOrDash(taskInspectCurrentRunID(record.CurrentRun))},
			{Label: cliNextActionValue, Value: stringOrDash(record.NextAction)},
			{Label: "As Of", Value: stringOrDash(formatTime(record.AsOf))},
		}),
	}
	if record.CurrentRun != nil {
		blocks = append(
			blocks,
			renderHumanSection("Current Run", taskInspectRunSectionRows(record.CurrentRun)),
		)
	}
	if record.BoundSession != nil {
		blocks = append(
			blocks,
			renderHumanSection("Bound Session", taskInspectSessionRows(record.BoundSession)),
		)
	}
	blocks = append(
		blocks,
		renderHumanTable(
			"Diagnostics",
			[]string{cliCodeValue, cliSeverityValue, "Message", cliCommandValue},
			taskInspectDiagnosticRows(record.Diagnostics),
		),
		renderHumanTable(
			"Recent Runs",
			[]string{
				taskRunValue,
				taskStatusValue,
				taskAttemptValue,
				taskSessionValue,
				cliHashValue,
				"Heartbeat Age",
				taskErrorValue,
			},
			taskInspectRunRows(record.RecentRuns),
		),
		renderHumanTable(
			"Recent Events",
			[]string{
				"ID",
				taskTypeValue,
				taskRunValue,
				taskOutcomeValue,
				authoredContextSummaryValue,
				taskTimeValue,
			},
			taskInspectEventRows(record.RecentEvents),
		),
	)
	return renderHumanBlocks(blocks...), nil
}

func renderTaskInspectToon(record *TaskInspectRecord) (string, error) {
	blocks := []string{
		renderToonObject("task_inspect", []string{
			automationTargetKey,
			taskTaskIDKey,
			taskTitleKey,
			taskStatusKey,
			"current_run_id",
			cliNextActionKey,
			"as_of",
		}, []string{
			record.Target,
			record.Task.ID,
			record.Task.Title,
			string(record.Task.Status),
			taskInspectCurrentRunID(record.CurrentRun),
			record.NextAction,
			formatTime(record.AsOf),
		}),
		renderToonArray(
			"diagnostics",
			[]string{cliCodeKey, "severity", clientMessageKey, "command"},
			taskInspectDiagnosticToonRows(record.Diagnostics),
		),
		renderToonArray(
			"recent_runs",
			[]string{
				taskRunIDKey,
				taskStatusKey,
				taskAttemptKey,
				taskSessionIDKey,
				"hash",
				"heartbeat_age_seconds",
				taskErrorKey,
			},
			taskInspectRunToonRows(record.RecentRuns),
		),
		renderToonArray(
			"recent_events",
			[]string{
				"id",
				extensionTypeKey,
				taskRunIDKey,
				taskOutcomeKey,
				memorySummaryKey,
				taskTimestampKey,
			},
			taskInspectEventToonRows(record.RecentEvents),
		),
	}
	return renderHumanBlocks(blocks...), nil
}

func taskInspectRunSectionRows(run *contract.TaskInspectRunPayload) []keyValue {
	return []keyValue{
		{Label: taskRunValue, Value: stringOrDash(run.RunID)},
		{Label: taskTaskValue, Value: stringOrDash(run.TaskID)},
		{Label: taskStatusValue, Value: stringOrDash(run.Status.String())},
		{Label: taskAttemptValue, Value: intOrDash(run.Attempt)},
		{Label: taskSessionValue, Value: stringOrDash(run.BoundSessionID)},
		{Label: "Claim Hash", Value: stringOrDash(run.ClaimTokenHashTruncated)},
		{Label: "Lease Until", Value: stringOrDash(formatTimePtr(run.LeaseUntil))},
		{
			Label: "Heartbeat Age",
			Value: stringOrDash(taskInspectHeartbeatAge(run.HeartbeatAgeSeconds)),
		},
		{Label: taskErrorValue, Value: stringOrDash(run.LastErrorSummary)},
	}
}

func taskInspectSessionRows(session *contract.TaskInspectSessionPayload) []keyValue {
	return []keyValue{
		{Label: taskSessionValue, Value: stringOrDash(session.SessionID)},
		{Label: taskStatusValue, Value: stringOrDash(session.State)},
		{Label: bundleAgentValue, Value: stringOrDash(session.AgentName)},
		{Label: installProviderValue, Value: stringOrDash(session.ProviderName)},
		{Label: taskWorkspaceValue, Value: stringOrDash(session.WorkspaceID)},
		{Label: taskStartedValue, Value: stringOrDash(formatTimePtr(session.StartedAt))},
		{Label: "Last Activity", Value: stringOrDash(formatTimePtr(session.LastActivityAt))},
		{Label: taskReasonValue, Value: stringOrDash(session.StopReason)},
		{Label: "Failure", Value: stringOrDash(session.FailureKind)},
	}
}

func taskInspectDiagnosticRows(items []contract.DiagnosticItem) [][]string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			stringOrDash(item.Code),
			stringOrDash(item.Severity),
			stringOrDash(item.Message),
			stringOrDash(item.SuggestedCommand),
		})
	}
	return rows
}

func taskInspectDiagnosticToonRows(items []contract.DiagnosticItem) [][]string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{item.Code, item.Severity, item.Message, item.SuggestedCommand})
	}
	return rows
}

func taskInspectRunRows(items []contract.TaskInspectRunPayload) [][]string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			stringOrDash(item.RunID),
			stringOrDash(item.Status.String()),
			intOrDash(item.Attempt),
			stringOrDash(item.BoundSessionID),
			stringOrDash(item.ClaimTokenHashTruncated),
			stringOrDash(taskInspectHeartbeatAge(item.HeartbeatAgeSeconds)),
			stringOrDash(item.LastErrorSummary),
		})
	}
	return rows
}

func taskInspectRunToonRows(items []contract.TaskInspectRunPayload) [][]string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			item.RunID,
			item.Status.String(),
			strconv.Itoa(item.Attempt),
			item.BoundSessionID,
			item.ClaimTokenHashTruncated,
			taskInspectHeartbeatAge(item.HeartbeatAgeSeconds),
			item.LastErrorSummary,
		})
	}
	return rows
}

func taskInspectEventRows(items []contract.TaskInspectEventPayload) [][]string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			stringOrDash(item.ID),
			stringOrDash(item.Type),
			stringOrDash(item.RunID),
			stringOrDash(item.Outcome),
			stringOrDash(item.Summary),
			stringOrDash(formatTime(item.Timestamp)),
		})
	}
	return rows
}

func taskInspectEventToonRows(items []contract.TaskInspectEventPayload) [][]string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			item.ID,
			item.Type,
			item.RunID,
			item.Outcome,
			item.Summary,
			formatTime(item.Timestamp),
		})
	}
	return rows
}

func taskInspectCurrentRunID(run *contract.TaskInspectRunPayload) string {
	if run == nil {
		return ""
	}
	return run.RunID
}

func taskInspectHeartbeatAge(value *int64) string {
	if value == nil {
		return ""
	}
	return int64OrDash(*value)
}

func renderTaskDetailHuman(detail *TaskDetailRecord) (string, error) {
	taskBlock, err := taskBundle(&detail.Task).human()
	if err != nil {
		return "", err
	}
	return renderHumanBlocks(
		taskBlock,
		renderHumanTable(
			"Child Tasks",
			[]string{
				"ID",
				taskIdentifierValue,
				taskScopeValue,
				taskWorkspaceValue,
				taskStatusValue,
				taskOwnerValue,
				taskTitleValue,
			},
			taskChildRows(detail.Children),
		),
		renderHumanTable(
			"Dependencies",
			[]string{taskTaskValue, "Depends On", taskKindValue, taskCreatedValue},
			taskDependencyRows(detail.Dependencies),
		),
		renderHumanTable(
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
			taskRunRows(detail.Runs),
		),
		renderHumanTable(
			"Task Events",
			[]string{"ID", taskTypeValue, taskRunValue, "Actor", taskOriginValue, taskTimeValue},
			taskEventRows(detail.Events),
		),
	), nil
}

func renderTaskDetailToon(detail *TaskDetailRecord) (string, error) {
	taskBlock, err := taskBundle(&detail.Task).toon()
	if err != nil {
		return "", err
	}
	return renderHumanBlocks(
		taskBlock,
		renderToonArray(
			"task_children",
			[]string{
				"id",
				taskIdentifierKey,
				taskScopeKey,
				taskWorkspaceIDKey,
				taskStatusKey,
				taskOwnerKey,
				taskTitleKey,
			},
			taskChildToonRows(detail.Children),
		),
		renderToonArray(
			"task_dependencies",
			[]string{taskTaskIDKey, "depends_on_task_id", taskKindKey, taskCreatedAtKey},
			taskDependencyToonRows(detail.Dependencies),
		),
		renderToonArray(
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
			taskRunToonRows(detail.Runs),
		),
		renderToonArray(
			"task_events",
			[]string{"id", "event_type", taskRunIDKey, "actor", taskOriginKey, taskTimestampKey},
			taskEventToonRows(detail.Events),
		),
	), nil
}
