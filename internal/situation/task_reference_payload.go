package situation

import (
	"github.com/compozy/compozy/internal/api/contract"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func taskReferencePayload(taskRecord taskpkg.Task) *contract.TaskReferencePayload {
	reference := taskReference(taskRecord)
	return &contract.TaskReferencePayload{
		ID:             reference.ID,
		Identifier:     reference.Identifier,
		Title:          reference.Title,
		Status:         reference.Status,
		Priority:       reference.Priority,
		Owner:          reference.Owner,
		Scope:          reference.Scope,
		WorkspaceID:    reference.WorkspaceID,
		LatestEventSeq: reference.LatestEventSeq,
	}
}
