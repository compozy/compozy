package cli

import (
	"strconv"
)

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
		{Label: sessionProfileValue, Value: stringOrDash(review.ProfileName)},
		{Label: taskTaskValue, Value: stringOrDash(review.TaskID)},
		{Label: taskRunValue, Value: stringOrDash(review.RunID)},
		{Label: taskStatusValue, Value: stringOrDash(string(review.Status))},
		{Label: taskOutcomeValue, Value: stringOrDash(string(review.Outcome))},
		{Label: taskReasonValue, Value: stringOrDash(review.Reason)},
		{Label: cliDeliveryValue, Value: stringOrDash(review.DeliveryID)},
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
			sessionProfileValue,
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
			profileNameOutputKey,
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
				stringOrDash(item.ProfileName),
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
				item.ProfileName,
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
		[]string{"Run ID", taskTaskValue, taskStatusValue, taskAttemptValue},
		"task_fanout_runs",
		[]string{agentKernelRunIDKey, taskTaskIDKey, taskStatusKey, taskAttemptKey},
		func(item TaskRunRecord) []string {
			return []string{
				stringOrDash(item.ID),
				stringOrDash(item.TaskID),
				stringOrDash(string(item.Status)),
				intOrDash(item.Attempt),
			}
		},
		func(item TaskRunRecord) []string {
			return []string{
				item.ID,
				item.TaskID,
				string(item.Status),
				strconv.Itoa(item.Attempt),
			}
		},
	)
}
