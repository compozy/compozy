package cli

import "strconv"

func taskRunDetailBundle(detail *TaskRunDetailRecord) outputBundle {
	return outputBundle{
		jsonValue: detail,
		human: func() (string, error) {
			runBlock, err := taskRunBundle(detail.Run).human()
			if err != nil {
				return "", err
			}

			blocks := []string{runBlock}
			if detail.Task != nil {
				blocks = append(blocks, taskRunDetailTaskBlock(detail))
			}
			if detail.Session != nil {
				blocks = append(blocks, taskRunDetailSessionBlock(detail))
			}
			blocks = append(blocks, renderHumanSection(
				"Operational Summary",
				taskRunOperationalSummaryRows(detail.Summary),
			))
			return renderHumanBlocks(blocks...), nil
		},
		toon: func() (string, error) {
			taskTitle, taskStatus := "", ""
			if detail.Task != nil {
				taskTitle = detail.Task.Title
				taskStatus = string(detail.Task.Status)
			}
			return renderToonObject("task_run_detail", []string{
				"id",
				taskTaskIDKey,
				taskStatusKey,
				taskAttemptKey,
				taskSessionIDKey,
				taskTitleKey,
				"task_status",
				"last_event_type",
				"last_activity_at",
				"total_cost",
				"cost_currency",
				"cost_status",
				"cost_source",
			}, []string{
				detail.Run.ID,
				detail.Run.TaskID,
				detail.Run.Status.String(),
				strconv.Itoa(detail.Run.Attempt),
				detail.Run.SessionID,
				taskTitle,
				taskStatus,
				detail.Summary.LastEventType,
				formatTime(detail.Summary.LastActivityAt),
				formatCostProvenance(
					detail.Summary.TotalCost,
					formatStringPtr(detail.Summary.CostCurrency),
					detail.Summary.CostStatus,
				),
				formatStringPtr(detail.Summary.CostCurrency),
				string(detail.Summary.CostStatus),
				string(detail.Summary.CostSource),
			}), nil
		},
	}
}

func taskRunDetailTaskBlock(detail *TaskRunDetailRecord) string {
	return renderHumanSection(taskTaskValue, []keyValue{
		{Label: "ID", Value: stringOrDash(detail.Task.ID)},
		{Label: taskIdentifierValue, Value: stringOrDash(detail.Task.Identifier)},
		{Label: taskTitleValue, Value: stringOrDash(detail.Task.Title)},
		{Label: taskStatusValue, Value: stringOrDash(string(detail.Task.Status))},
		{Label: cliPriorityValue, Value: stringOrDash(string(detail.Task.Priority))},
		{Label: taskOwnerValue, Value: stringOrDash(formatTaskOwnership(detail.Task.Owner))},
		{Label: taskScopeValue, Value: stringOrDash(string(detail.Task.Scope))},
		{Label: taskWorkspaceValue, Value: stringOrDash(detail.Task.WorkspaceID)},
		{Label: "Latest Event", Value: int64OrDash(detail.Task.LatestEventSeq)},
	})
}

func taskRunDetailSessionBlock(detail *TaskRunDetailRecord) string {
	return renderHumanSection(taskSessionValue, []keyValue{
		{Label: "ID", Value: stringOrDash(detail.Session.SessionID)},
		{Label: taskWorkspaceValue, Value: stringOrDash(detail.Session.WorkspaceID)},
		{Label: bundleAgentValue, Value: stringOrDash(detail.Session.AgentName)},
		{Label: providerNameValue, Value: stringOrDash(detail.Session.Name)},
		{Label: taskStatusValue, Value: stringOrDash(detail.Session.State)},
		{Label: taskCreatedValue, Value: stringOrDash(formatTime(detail.Session.CreatedAt))},
		{Label: taskUpdatedValue, Value: stringOrDash(formatTime(detail.Session.UpdatedAt))},
	})
}
