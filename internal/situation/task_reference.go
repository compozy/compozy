package situation

import (
	"strings"

	taskpkg "github.com/compozy/agh/internal/task"
)

func taskReference(taskRecord taskpkg.Task) taskpkg.Reference {
	return taskpkg.Reference{
		ID:             strings.TrimSpace(taskRecord.ID),
		Identifier:     safeTaskContextText(taskRecord.Identifier, maxReviewReasonFallback),
		Title:          safeTaskContextText(taskRecord.Title, maxReviewReasonFallback),
		Status:         taskRecord.Status,
		Priority:       taskRecord.Priority,
		Owner:          cloneOwnership(taskRecord.Owner),
		Scope:          taskRecord.Scope,
		WorkspaceID:    strings.TrimSpace(taskRecord.WorkspaceID),
		LatestEventSeq: taskRecord.LatestEventSeq,
	}
}
