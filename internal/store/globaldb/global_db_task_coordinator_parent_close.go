package globaldb

import (
	"context"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func validateCoordinatorParentClosesWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	parentRunID looppkg.RunID,
	plan taskpkg.CoordinatorCompletionPlan,
) error {
	if len(plan.ParentCloses) == 0 {
		return nil
	}
	if plan.Terminal == nil {
		return fmt.Errorf("%w: parent-close actions require a terminal parent", taskpkg.ErrValidation)
	}
	for _, raw := range plan.ParentCloses {
		spec := raw.Normalize()
		child, err := getLoopRunByIDWithExecutor(ctx, exec, looppkg.RunID(spec.ChildLoopRunID))
		if err != nil {
			return err
		}
		if child.ParentLoopRunID != parentRunID || spec.ParentLoopRunID != string(parentRunID) {
			return fmt.Errorf(
				"%w: child Loop run %q is not owned by parent %q",
				looppkg.ErrTransitionConflict,
				child.ID,
				parentRunID,
			)
		}
		if spec.ParentStatus != strings.TrimSpace(plan.Terminal.Status) {
			return fmt.Errorf(
				"%w: parent-close status %q does not match terminal %q",
				taskpkg.ErrValidation,
				spec.ParentStatus,
				plan.Terminal.Status,
			)
		}
	}
	return nil
}
