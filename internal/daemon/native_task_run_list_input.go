package daemon

import (
	"strings"

	taskpkg "github.com/compozy/compozy/internal/task"
)

type taskRunListInput struct {
	TaskID    string `json:"task_id"`
	Status    string `json:"status,omitempty"`
	SessionID string `json:"session_id,omitempty"`

	Limit int `json:"limit,omitzero"`
}

func (i taskRunListInput) query() taskpkg.RunQuery {
	return taskpkg.RunQuery{
		TaskID:    strings.TrimSpace(i.TaskID),
		Status:    taskpkg.ParseRunStatus(i.Status),
		SessionID: strings.TrimSpace(i.SessionID),

		Limit: i.Limit,
	}
}
