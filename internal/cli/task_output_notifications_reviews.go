package cli

import (
	"strconv"
	"strings"
)

func taskBridgeNotificationSubscriptionBundle(
	subscription *TaskBridgeNotificationSubscriptionRecord,
) outputBundle {
	return outputBundle{
		jsonValue: *subscription,
		human: func() (string, error) {
			return renderHumanSection(
				"Task Bridge Notification Subscription",
				taskBridgeNotificationRows(subscription),
			), nil
		},
		toon: func() (string, error) {
			return renderJSONPreview(subscription)
		},
	}
}

func taskBridgeNotificationSubscriptionListBundle(
	items []TaskBridgeNotificationSubscriptionRecord,
) outputBundle {
	return listBundle(
		items,
		items,
		"Task Bridge Notification Subscriptions",
		[]string{
			taskSubscriptionValue,
			taskTaskValue,
			taskBridgeValue,
			taskScopeValue,
			taskPeerValue,
			taskGroupValue,
			taskModeValue,
			"Cursor Seq",
			"Cursor Error",
			"Cursor Updated",
			taskUpdatedValue,
		},
		"task_bridge_notification_subscriptions",
		[]string{
			"subscription_id",
			taskTaskIDKey,
			taskBridgeInstanceIDKey,
			taskScopeKey,
			taskPeerIDKey,
			taskGroupIDKey,
			"delivery_mode",
			"cursor_last_sequence",
			"cursor_last_error",
			"cursor_updated_at",
			taskUpdatedAtKey,
		},
		func(item TaskBridgeNotificationSubscriptionRecord) []string {
			return []string{
				stringOrDash(item.SubscriptionID),
				stringOrDash(item.TaskID),
				stringOrDash(item.BridgeInstanceID),
				stringOrDash(string(item.Scope)),
				stringOrDash(item.PeerID),
				stringOrDash(item.GroupID),
				stringOrDash(string(item.DeliveryMode)),
				int64OrDash(item.Cursor.LastSequence),
				stringOrDash(item.Cursor.LastError),
				stringOrDash(formatTimePtr(item.Cursor.UpdatedAt)),
				stringOrDash(formatTime(item.UpdatedAt)),
			}
		},
		func(item TaskBridgeNotificationSubscriptionRecord) []string {
			return []string{
				item.SubscriptionID,
				item.TaskID,
				item.BridgeInstanceID,
				string(item.Scope),
				item.PeerID,
				item.GroupID,
				string(item.DeliveryMode),
				strconv.FormatInt(item.Cursor.LastSequence, 10),
				item.Cursor.LastError,
				formatTimePtr(item.Cursor.UpdatedAt),
				formatTime(item.UpdatedAt),
			}
		},
	)
}

func taskBridgeNotificationRows(subscription *TaskBridgeNotificationSubscriptionRecord) []keyValue {
	return []keyValue{
		{Label: taskSubscriptionValue, Value: stringOrDash(subscription.SubscriptionID)},
		{Label: taskTaskValue, Value: stringOrDash(subscription.TaskID)},
		{Label: taskBridgeValue, Value: stringOrDash(subscription.BridgeInstanceID)},
		{Label: taskScopeValue, Value: stringOrDash(string(subscription.Scope))},
		{Label: taskWorkspaceValue, Value: stringOrDash(subscription.WorkspaceID)},
		{Label: taskPeerValue, Value: stringOrDash(subscription.PeerID)},
		{Label: taskThreadValue, Value: stringOrDash(subscription.ThreadID)},
		{Label: taskGroupValue, Value: stringOrDash(subscription.GroupID)},
		{Label: taskModeValue, Value: stringOrDash(string(subscription.DeliveryMode))},
		{Label: "Cursor Consumer", Value: stringOrDash(subscription.Cursor.ConsumerID)},
		{Label: "Cursor Stream", Value: stringOrDash(subscription.Cursor.StreamName)},
		{Label: "Cursor Subject", Value: stringOrDash(subscription.Cursor.SubjectID)},
		{Label: "Cursor Last Sequence", Value: int64OrDash(subscription.Cursor.LastSequence)},
		{Label: "Cursor Last Delivery", Value: stringOrDash(subscription.Cursor.LastDeliveryID)},
		{
			Label: "Cursor Last Delivered",
			Value: stringOrDash(formatTimePtr(subscription.Cursor.LastDeliveredAt)),
		},
		{Label: "Cursor Last Error", Value: stringOrDash(subscription.Cursor.LastError)},
		{
			Label: "Cursor Updated",
			Value: stringOrDash(formatTimePtr(subscription.Cursor.UpdatedAt)),
		},
		{Label: taskCreatedByValue, Value: stringOrDash(formatTaskActor(subscription.CreatedBy))},
		{Label: taskUpdatedValue, Value: stringOrDash(formatTime(subscription.UpdatedAt))},
	}
}

func taskBridgeNotificationSubscriptionDeleteBundle(
	taskID string,
	subscriptionID string,
) outputBundle {
	item := struct {
		TaskID         string `json:"task_id"`
		SubscriptionID string `json:"subscription_id"`
		Status         string `json:"status"`
	}{
		TaskID:         strings.TrimSpace(taskID),
		SubscriptionID: strings.TrimSpace(subscriptionID),
		Status:         taskDeletedKey,
	}
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			return renderHumanSection("Task Bridge Notification Subscription", []keyValue{
				{Label: taskTaskIDValue, Value: stringOrDash(item.TaskID)},
				{Label: taskSubscriptionValue, Value: stringOrDash(item.SubscriptionID)},
				{Label: taskStatusValue, Value: item.Status},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"task_bridge_notification_subscription",
				[]string{taskTaskIDKey, "subscription_id", taskStatusKey},
				[]string{item.TaskID, item.SubscriptionID, item.Status},
			), nil
		},
	}
}

func taskRunReviewRequestBundle(record *TaskRunReviewRequestRecord) outputBundle {
	return outputBundle{
		jsonValue: *record,
		human: func() (string, error) {
			return renderHumanSection("Task Run Review Request", []keyValue{
				{Label: taskReviewValue, Value: stringOrDash(record.Review.ReviewID)},
				{Label: taskRunValue, Value: stringOrDash(record.Review.RunID)},
				{Label: taskTaskValue, Value: stringOrDash(record.Review.TaskID)},
				{Label: taskStatusValue, Value: stringOrDash(string(record.Review.Status))},
				{Label: "Policy", Value: stringOrDash(string(record.Review.Policy))},
				{Label: taskCreatedValue, Value: strconv.FormatBool(record.Created)},
			}), nil
		},
		toon: func() (string, error) {
			return renderJSONPreview(record)
		},
	}
}

func taskRunReviewBundle(review *TaskRunReviewRecord) outputBundle {
	return outputBundle{
		jsonValue: *review,
		human: func() (string, error) {
			return renderHumanSection("Task Run Review", taskRunReviewRows(review)), nil
		},
		toon: func() (string, error) {
			return renderJSONPreview(review)
		},
	}
}

func taskRunReviewVerdictBundle(record *TaskRunReviewVerdictRecord) outputBundle {
	return outputBundle{
		jsonValue: *record,
		human: func() (string, error) {
			rows := taskRunReviewRows(&record.Review)
			if record.ContinuationRun != nil {
				rows = append(
					rows,
					keyValue{
						Label: "Continuation Run",
						Value: stringOrDash(record.ContinuationRun.ID),
					},
				)
			}
			rows = append(
				rows,
				keyValue{Label: "Circuit Opened", Value: strconv.FormatBool(record.CircuitOpened)},
			)
			return renderHumanSection("Task Run Review Verdict", rows), nil
		},
		toon: func() (string, error) {
			return renderJSONPreview(record)
		},
	}
}

func taskRunReviewRows(review *TaskRunReviewRecord) []keyValue {
	return []keyValue{
		{Label: taskReviewValue, Value: stringOrDash(review.ReviewID)},
		{Label: taskTaskValue, Value: stringOrDash(review.TaskID)},
		{Label: taskRunValue, Value: stringOrDash(review.RunID)},
		{Label: taskStatusValue, Value: stringOrDash(string(review.Status))},
		{Label: taskOutcomeValue, Value: stringOrDash(string(review.Outcome))},
		{Label: taskReasonValue, Value: stringOrDash(review.Reason)},
		{Label: "Delivery", Value: stringOrDash(review.DeliveryID)},
		{Label: "Missing Work", Value: stringOrDash(compactJSON(review.MissingWork))},
		{Label: "Next Guidance", Value: stringOrDash(review.NextRoundGuidance)},
		{Label: "Reviewer Session", Value: stringOrDash(review.ReviewerSessionID)},
		{Label: "Reviewed By", Value: stringOrDash(formatTaskActorPtr(review.ReviewedBy))},
		{Label: "Requested", Value: stringOrDash(formatTime(review.RequestedAt))},
		{Label: "Reviewed", Value: stringOrDash(formatTime(review.ReviewedAt))},
		{Label: taskUpdatedValue, Value: stringOrDash(formatTime(review.UpdatedAt))},
	}
}

func taskRunReviewListBundle(items []TaskRunReviewRecord) outputBundle {
	return listBundle(
		items,
		items,
		"Task Run Reviews",
		[]string{
			taskReviewValue,
			taskTaskValue,
			taskRunValue,
			taskStatusValue,
			taskOutcomeValue,
			"Reviewer Session",
			taskUpdatedValue,
		},
		"task_run_reviews",
		[]string{
			"review_id",
			taskTaskIDKey,
			taskRunIDKey,
			taskStatusKey,
			cliOutcomeKey,
			"reviewer_session_id",
			taskUpdatedAtKey,
		},
		func(item TaskRunReviewRecord) []string {
			return []string{
				stringOrDash(item.ReviewID),
				stringOrDash(item.TaskID),
				stringOrDash(item.RunID),
				stringOrDash(string(item.Status)),
				stringOrDash(string(item.Outcome)),
				stringOrDash(item.ReviewerSessionID),
				stringOrDash(formatTime(item.UpdatedAt)),
			}
		},
		func(item TaskRunReviewRecord) []string {
			return []string{
				item.ReviewID,
				item.TaskID,
				item.RunID,
				string(item.Status),
				string(item.Outcome),
				item.ReviewerSessionID,
				formatTime(item.UpdatedAt),
			}
		},
	)
}

func taskExecutionBundle(item *TaskExecutionRecord) outputBundle {
	return outputBundle{
		jsonValue: *item,
		human: func() (string, error) {
			taskBlock, err := taskBundle(&item.Task).human()
			if err != nil {
				return "", err
			}
			runBlock, err := taskRunBundle(item.Run).human()
			if err != nil {
				return "", err
			}
			return renderHumanBlocks(taskBlock, runBlock), nil
		},
		toon: func() (string, error) {
			taskBlock, err := taskBundle(&item.Task).toon()
			if err != nil {
				return "", err
			}
			runBlock, err := taskRunBundle(item.Run).toon()
			if err != nil {
				return "", err
			}
			return renderHumanBlocks(taskBlock, runBlock), nil
		},
	}
}

func taskFanOutRunsBundle(record FanOutTaskRunsRecord) outputBundle {
	return listBundle(
		record,
		record.Runs,
		"Task Fan-Out Runs",
		[]string{"Run ID", taskTaskValue, taskStatusValue, taskAttemptValue, taskParticipationChannelValue},
		"task_fanout_runs",
		[]string{agentKernelRunIDKey, taskTaskIDKey, taskStatusKey, taskAttemptKey, taskParticipationChannelKey},
		func(item TaskRunRecord) []string {
			return []string{
				stringOrDash(item.ID),
				stringOrDash(item.TaskID),
				stringOrDash(string(item.Status)),
				intOrDash(item.Attempt),
				resolvedParticipationChannel(item.ResolvedNetworkParticipation),
			}
		},
		func(item TaskRunRecord) []string {
			return []string{
				item.ID,
				item.TaskID,
				string(item.Status),
				strconv.Itoa(item.Attempt),
				resolvedParticipationChannelRaw(item.ResolvedNetworkParticipation),
			}
		},
	)
}
