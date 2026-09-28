package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type taskRunIdempotencyReader interface {
	GetTaskRunByIdempotencyKey(context.Context, string, Origin) (Run, error)
}

func existingQueuedRunWithStore(
	ctx context.Context,
	store taskRunIdempotencyReader,
	taskID string,
	idempotencyKey string,
	origin Origin,
) (*Run, bool, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return nil, false, nil
	}
	run, err := store.GetTaskRunByIdempotencyKey(ctx, idempotencyKey, origin)
	switch {
	case errors.Is(err, ErrTaskRunIdempotencyNotFound):
		return nil, false, nil
	case err != nil:
		return nil, false, err
	case strings.TrimSpace(run.TaskID) != strings.TrimSpace(taskID):
		return nil, false, fmt.Errorf(
			"%w: idempotency key %q is already bound to task %q",
			ErrValidation,
			idempotencyKey,
			run.TaskID,
		)
	default:
		return &run, true, nil
	}
}
