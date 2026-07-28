package cli

import "strconv"

func agentTaskNextBundle(record AgentTaskNextRecord) outputBundle {
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			return renderJSONPreview(record)
		},
		toon: func() (string, error) {
			return renderJSONPreview(record)
		},
	}
}

func agentTaskLeaseBundle(record AgentTaskLeaseRecord) outputBundle {
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			return renderJSONPreview(record)
		},
		toon: func() (string, error) {
			return renderJSONPreview(record)
		},
	}
}

func taskBundle(item *TaskRecord) outputBundle {
	return outputBundle{
		jsonValue: *item,
		human: func() (string, error) {
			return renderHumanSection(taskTaskValue, []keyValue{
				{Label: "ID", Value: stringOrDash(item.ID)},
				{Label: taskIdentifierValue, Value: stringOrDash(item.Identifier)},
				{Label: taskScopeValue, Value: stringOrDash(string(item.Scope))},
				{Label: taskWorkspaceValue, Value: stringOrDash(item.WorkspaceID)},
				{Label: taskParentValue, Value: stringOrDash(item.ParentTaskID)},
				{Label: taskTitleValue, Value: stringOrDash(item.Title)},
				{Label: taskDescriptionValue, Value: stringOrDash(item.Description)},
				{Label: taskStatusValue, Value: stringOrDash(string(item.Status))},
				{Label: "Paused", Value: strconv.FormatBool(item.Paused)},
				{Label: "Paused By", Value: stringOrDash(item.PausedBy)},
				{Label: taskReasonValue, Value: stringOrDash(item.PausedReason)},
				{Label: taskOwnerValue, Value: stringOrDash(formatTaskOwnership(item.Owner))},
				{Label: taskCreatedByValue, Value: stringOrDash(formatTaskActor(item.CreatedBy))},
				{Label: taskOriginValue, Value: stringOrDash(formatTaskOrigin(item.Origin))},
				{
					Label: taskParticipationChannelValue,
					Value: resolvedParticipationChannel(item.ResolvedNetworkParticipation),
				},
				{Label: taskCreatedValue, Value: stringOrDash(formatTime(item.CreatedAt))},
				{Label: taskUpdatedValue, Value: stringOrDash(formatTime(item.UpdatedAt))},
				{Label: "Closed", Value: stringOrDash(formatTimePtr(item.ClosedAt))},
				{Label: "Metadata", Value: stringOrDash(compactJSON(item.Metadata))},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(taskTaskKey, []string{
				"id",
				taskIdentifierKey,
				taskScopeKey,
				taskWorkspaceIDKey,
				"parent_task_id",
				taskTitleKey,
				taskDescriptionKey,
				taskStatusKey,
				"paused",
				"paused_by",
				"paused_reason",
				taskOwnerKey,
				createdByKey,
				taskOriginKey,
				taskParticipationChannelKey,
				taskCreatedAtKey,
				taskUpdatedAtKey,
				"closed_at",
				"metadata",
			}, []string{
				item.ID,
				item.Identifier,
				string(item.Scope),
				item.WorkspaceID,
				item.ParentTaskID,
				item.Title,
				item.Description,
				string(item.Status),
				strconv.FormatBool(item.Paused),
				item.PausedBy,
				item.PausedReason,
				formatTaskOwnership(item.Owner),
				formatTaskActor(item.CreatedBy),
				formatTaskOrigin(item.Origin),
				resolvedParticipationChannelRaw(item.ResolvedNetworkParticipation),
				formatTime(item.CreatedAt),
				formatTime(item.UpdatedAt),
				formatTimePtr(item.ClosedAt),
				compactJSON(item.Metadata),
			}), nil
		},
	}
}

func taskBlockBundle(item TaskBlockRecord) outputBundle {
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			return renderHumanSection("Task Block", taskBlockKeyValues(item)), nil
		},
		toon: func() (string, error) {
			return renderToonObject("task_block", taskBlockToonFields(), taskBlockToonValues(item)), nil
		},
	}
}

func taskBlockListBundle(items []TaskBlockRecord) outputBundle {
	return listBundle(
		items,
		items,
		"Task Blocks",
		[]string{"ID", taskTaskIDValue, taskKindValue, taskReasonValue, taskCreatedValue, "Cleared"},
		"task_blocks",
		[]string{"id", taskTaskIDKey, taskKindKey, taskReasonKey, taskCreatedAtKey, "cleared_at"},
		func(item TaskBlockRecord) []string {
			return []string{
				stringOrDash(item.ID),
				stringOrDash(item.TaskID),
				stringOrDash(string(item.Kind)),
				stringOrDash(item.Reason),
				stringOrDash(formatTime(item.CreatedAt)),
				stringOrDash(formatTimePtr(item.ClearedAt)),
			}
		},
		func(item TaskBlockRecord) []string {
			return []string{
				item.ID,
				item.TaskID,
				string(item.Kind),
				item.Reason,
				formatTime(item.CreatedAt),
				formatTimePtr(item.ClearedAt),
			}
		},
	)
}

func taskBlockKeyValues(item TaskBlockRecord) []keyValue {
	return []keyValue{
		{Label: "ID", Value: stringOrDash(item.ID)},
		{Label: taskTaskIDValue, Value: stringOrDash(item.TaskID)},
		{Label: taskWorkspaceValue, Value: stringOrDash(item.WorkspaceID)},
		{Label: taskKindValue, Value: stringOrDash(string(item.Kind))},
		{Label: taskReasonValue, Value: stringOrDash(item.Reason)},
		{Label: "Details", Value: stringOrDash(compactJSON(item.Details))},
		{Label: taskCreatedByValue, Value: stringOrDash(formatTaskActor(item.CreatedBy))},
		{Label: taskCreatedValue, Value: stringOrDash(formatTime(item.CreatedAt))},
		{Label: "Expires", Value: stringOrDash(formatTimePtr(item.ExpiresAt))},
		{Label: "Cleared", Value: stringOrDash(formatTimePtr(item.ClearedAt))},
		{Label: "Cleared By", Value: stringOrDash(formatTaskActorPtr(item.ClearedBy))},
		{Label: "Clear Note", Value: stringOrDash(item.ClearNote)},
	}
}

func taskBlockToonFields() []string {
	return []string{
		"id",
		taskTaskIDKey,
		taskWorkspaceIDKey,
		taskKindKey,
		taskReasonKey,
		createdByKey,
		taskCreatedAtKey,
		"expires_at",
		"cleared_at",
		"cleared_by",
		"clear_note",
	}
}

func taskBlockToonValues(item TaskBlockRecord) []string {
	return []string{
		item.ID,
		item.TaskID,
		item.WorkspaceID,
		string(item.Kind),
		item.Reason,
		formatTaskActor(item.CreatedBy),
		formatTime(item.CreatedAt),
		formatTimePtr(item.ExpiresAt),
		formatTimePtr(item.ClearedAt),
		formatTaskActorPtr(item.ClearedBy),
		item.ClearNote,
	}
}

func taskExecutionProfileBundle(profile *TaskExecutionProfileRecord) outputBundle {
	return outputBundle{
		jsonValue: *profile,
		human: func() (string, error) {
			return renderHumanSection("Task Execution Profile", []keyValue{
				{Label: taskTaskIDValue, Value: stringOrDash(profile.TaskID)},
				{Label: "Coordinator", Value: stringOrDash(string(profile.Coordinator.Mode))},
				{Label: "Worker", Value: stringOrDash(string(profile.Worker.Mode))},
				{Label: "Worker Agent", Value: stringOrDash(profile.Worker.AgentName)},
				{Label: "Worker Provider", Value: stringOrDash(profile.Worker.Provider)},
				{Label: "Worker Model", Value: stringOrDash(profile.Worker.Model)},
				{Label: "Review Agent", Value: stringOrDash(profile.Review.AgentName)},
				{Label: taskSandboxValue, Value: stringOrDash(string(profile.Sandbox.Mode))},
				{Label: "Sandbox Ref", Value: stringOrDash(profile.Sandbox.SandboxRef)},
				{Label: "Runtime", Value: stringOrDash(string(profile.Runtime.Mode))},
				{Label: taskUpdatedValue, Value: stringOrDash(formatTime(profile.UpdatedAt))},
			}), nil
		},
		toon: func() (string, error) {
			return renderJSONPreview(profile)
		},
	}
}
