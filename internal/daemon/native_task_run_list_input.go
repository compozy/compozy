package daemon

import (
	"strings"

	taskpkg "github.com/compozy/agh/internal/task"
)

type taskRunListInput struct {
	TaskID               string `json:"task_id"`
	Status               string `json:"status,omitempty"`
	SessionID            string `json:"session_id,omitempty"`
	ParticipationChannel string `json:"participation_channel,omitempty"`
	Limit                int    `json:"limit,omitempty"`
}

func (i taskRunListInput) query() taskpkg.RunQuery {
	return taskpkg.RunQuery{
		TaskID:               strings.TrimSpace(i.TaskID),
		Status:               taskpkg.ParseRunStatus(i.Status),
		SessionID:            strings.TrimSpace(i.SessionID),
		ParticipationChannel: strings.TrimSpace(i.ParticipationChannel),
		Limit:                i.Limit,
	}
}
