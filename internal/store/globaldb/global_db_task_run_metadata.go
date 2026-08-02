package globaldb

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// UpdateTaskRunMetadata applies a metadata-only compare-and-set. The SQL shape
// cannot alter lifecycle, identity, session, lease, result, or error fields.
func (g *TaskRepo) UpdateTaskRunMetadata(
	ctx context.Context,
	mutation taskpkg.RunMetadataMutation,
) (taskpkg.Run, error) {
	if err := g.checkReady(ctx, "update task run metadata"); err != nil {
		return taskpkg.Run{}, err
	}
	id, err := requireTaskValue(mutation.RunID, "task run id")
	if err != nil {
		return taskpkg.Run{}, err
	}

	var updated taskpkg.Run
	err = g.withTaskImmediateTransaction(ctx, "update task run metadata", func(exec taskSQLExecutor) error {
		current, loadErr := g.getTaskRunWithExecutor(ctx, exec, id)
		if loadErr != nil {
			return loadErr
		}
		candidate := current
		candidate.Metadata = cloneTaskRawJSON(mutation.Metadata)
		if validationErr := candidate.Validate(); validationErr != nil {
			return validationErr
		}
		affected, updateErr := sqlcgen.New(exec).UpdateTaskRunMetadata(
			ctx,
			sqlcgen.UpdateTaskRunMetadataParams{
				MetadataJson:         nullableTaskRawJSON(candidate.Metadata),
				ID:                   id,
				ExpectedMetadataJson: nullableTaskRawJSON(mutation.ExpectedMetadata),
			},
		)
		if updateErr != nil {
			return fmt.Errorf("store: update task run %q metadata: %w", id, updateErr)
		}
		if affected == 0 {
			return fmt.Errorf(
				"%w: task run %q metadata changed before update",
				taskpkg.ErrInvalidStatusTransition,
				id,
			)
		}
		updated, loadErr = g.getTaskRunWithExecutor(ctx, exec, id)
		return loadErr
	})
	if err != nil {
		return taskpkg.Run{}, err
	}
	return updated, nil
}
