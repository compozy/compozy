package globaldb

import (
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

func taskBlockFromGenerated(row sqlcgen.TaskBlock) (taskpkg.TaskBlock, error) {
	block := taskpkg.TaskBlock{
		ID:          row.ID,
		WorkspaceID: taskNullStringValue(row.WorkspaceID),
		TaskID:      row.TaskID,
		Kind:        taskpkg.BlockKind(strings.TrimSpace(row.Kind)),
		Reason:      row.Reason,
		CreatedBy: taskpkg.ActorIdentity{
			Kind: taskpkg.ActorKind(strings.TrimSpace(row.CreatedByKind)),
			Ref:  row.CreatedByRef,
		},
		ClearNote: taskNullStringValue(row.ClearNote),
	}
	if row.ClearedByKind.Valid || row.ClearedByRef.Valid {
		block.ClearedBy = taskpkg.ActorIdentity{
			Kind: taskpkg.ActorKind(strings.TrimSpace(row.ClearedByKind.String)),
			Ref:  strings.TrimSpace(row.ClearedByRef.String),
		}
	}
	details, err := decodeTaskJSON(row.DetailsJson, "task_block.details_json")
	if err != nil {
		return taskpkg.TaskBlock{}, err
	}
	block.Details = details
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return taskpkg.TaskBlock{}, err
	}
	block.CreatedAt = createdAt
	if err := assignNullableTaskTimestamp(&block.ExpiresAt, row.ExpiresAt); err != nil {
		return taskpkg.TaskBlock{}, err
	}
	if err := assignNullableTaskTimestamp(&block.ClearedAt, row.ClearedAt); err != nil {
		return taskpkg.TaskBlock{}, err
	}
	return block, nil
}

func taskBlocksFromGenerated(rows []sqlcgen.TaskBlock) ([]taskpkg.TaskBlock, error) {
	blocks := make([]taskpkg.TaskBlock, 0, len(rows))
	for _, row := range rows {
		block, err := taskBlockFromGenerated(row)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}
	return blocks, nil
}

func taskBlockRecurrenceFromGenerated(row sqlcgen.TaskBlockRecurrence) (taskpkg.BlockRecurrence, error) {
	if row.Count < 0 || uint64(row.Count) > uint64(^uint(0)>>1) {
		return taskpkg.BlockRecurrence{}, fmt.Errorf(
			"%w: task block recurrence count out of range: %d",
			taskpkg.ErrValidation,
			row.Count,
		)
	}
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return taskpkg.BlockRecurrence{}, err
	}
	return taskpkg.BlockRecurrence{
		TaskID:    row.TaskID,
		Kind:      taskpkg.BlockKind(strings.TrimSpace(row.Kind)),
		Count:     int(row.Count),
		UpdatedAt: updatedAt,
	}, nil
}
