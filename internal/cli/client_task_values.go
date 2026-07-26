package cli

import (
	"errors"

	"net/url"

	"strconv"
	"strings"
)

func taskValues(query TaskListQuery) url.Values {
	values := url.Values{}
	if trimmed := strings.TrimSpace(string(query.Scope)); trimmed != "" {
		values.Set("scope", trimmed)
	}
	if trimmed := strings.TrimSpace(query.Workspace); trimmed != "" {
		values.Set("workspace", trimmed)
	}
	if trimmed := strings.TrimSpace(string(query.Status)); trimmed != "" {
		values.Set("status", trimmed)
	}
	if trimmed := strings.TrimSpace(string(query.Priority)); trimmed != "" {
		values.Set("priority", trimmed)
	}
	if query.IncludeDrafts {
		values.Set("include_drafts", "true")
	}
	if trimmed := strings.TrimSpace(string(query.OwnerKind)); trimmed != "" {
		values.Set("owner_kind", trimmed)
	}
	if trimmed := strings.TrimSpace(query.OwnerRef); trimmed != "" {
		values.Set("owner_ref", trimmed)
	}
	if trimmed := strings.TrimSpace(query.ParentTaskID); trimmed != "" {
		values.Set("parent_task_id", trimmed)
	}
	if trimmed := strings.TrimSpace(query.ParticipationChannel); trimmed != "" {
		values.Set("participation_channel", trimmed)
	}
	if trimmed := strings.TrimSpace(query.Query); trimmed != "" {
		values.Set("query", trimmed)
	}
	if trimmed := strings.TrimSpace(string(query.Sort)); trimmed != "" {
		values.Set("sort", trimmed)
	}
	if trimmed := strings.TrimSpace(query.Cursor); trimmed != "" {
		values.Set("cursor", trimmed)
	}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	return values
}

func taskRunValues(query TaskRunListQuery) url.Values {
	values := url.Values{}
	if trimmed := strings.TrimSpace(query.Status.String()); trimmed != "" {
		values.Set("status", trimmed)
	}
	if trimmed := strings.TrimSpace(query.SessionID); trimmed != "" {
		values.Set("session_id", trimmed)
	}
	if trimmed := strings.TrimSpace(query.ParticipationChannel); trimmed != "" {
		values.Set("participation_channel", trimmed)
	}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	return values
}

func schedulerBacklogValues(query SchedulerBacklogQuery) url.Values {
	values := url.Values{}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	if trimmed := strings.TrimSpace(query.WorkspaceID); trimmed != "" {
		values.Set("workspace", trimmed)
	}
	if query.IncludePaused {
		values.Set("include_paused", "true")
	}
	return values
}

func taskRunReviewListPathAndValues(query TaskRunReviewListQuery) (string, url.Values, error) {
	taskID := strings.TrimSpace(query.TaskID)
	runID := strings.TrimSpace(query.RunID)
	switch {
	case taskID != "" && runID != "":
		return "", nil, errors.New("cli: choose either --task or --run when listing task reviews")
	case taskID != "":
		return "/api/tasks/" + url.PathEscape(taskID) + "/reviews", taskRunReviewValues(query), nil
	case runID != "":
		return "/api/task-runs/" + url.PathEscape(runID) + "/reviews", taskRunReviewValues(query), nil
	default:
		return "", nil, errors.New("cli: task review list requires --task or --run")
	}
}

func taskRunReviewValues(query TaskRunReviewListQuery) url.Values {
	values := url.Values{}
	if trimmed := strings.TrimSpace(string(query.Status)); trimmed != "" {
		values.Set("status", trimmed)
	}
	if trimmed := strings.TrimSpace(query.ReviewerSessionID); trimmed != "" {
		values.Set("reviewer_session_id", trimmed)
	}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	return values
}

func taskBridgeNotificationSubscriptionValues(query TaskBridgeNotificationSubscriptionQuery) url.Values {
	values := url.Values{}
	if trimmed := strings.TrimSpace(query.BridgeInstanceID); trimmed != "" {
		values.Set("bridge_instance_id", trimmed)
	}
	if trimmed := strings.TrimSpace(string(query.Scope)); trimmed != "" {
		values.Set("scope", trimmed)
	}
	if trimmed := strings.TrimSpace(query.WorkspaceID); trimmed != "" {
		values.Set("workspace_id", trimmed)
	}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	return values
}
