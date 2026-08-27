package task

import (
	"context"
	"fmt"
	"strings"
)

func (m *Service) publishCoordinatorEnqueuedRuns(
	ctx context.Context,
	runs []Run,
	eventIDs []string,
	actor ActorContext,
) error {
	if len(runs) > len(eventIDs) {
		return fmt.Errorf(
			"%w: coordinator publication has %d runs but only %d reserved event IDs",
			ErrValidation,
			len(runs),
			len(eventIDs),
		)
	}
	for index := range runs {
		if strings.TrimSpace(eventIDs[index]) == "" {
			return fmt.Errorf(
				"%w: coordinator publication event ID %d is required",
				ErrValidation,
				index,
			)
		}
	}
	var publicationErrs []error
	for index, run := range runs {
		idempotencyKey := run.IdempotencyKeyValue()
		task, err := m.reconcileTaskCascade(ctx, run.TaskID, actor)
		if err != nil {
			publicationErrs = append(publicationErrs, err)
			task, err = m.store.GetTask(ctx, run.TaskID)
			if err != nil {
				publicationErrs = append(publicationErrs, fmt.Errorf(
					"task: load committed enqueued task %q for publication: %w",
					run.TaskID,
					err,
				))
			}
		}
		if err := m.recordTaskEventWithID(
			ctx,
			eventIDs[index],
			run.TaskID,
			run.ID,
			taskEventRunEnqueued,
			actor,
			runEnqueuedPayload{
				Attempt:        int(run.Attempt),
				Status:         run.Status,
				TaskStatus:     task.Status,
				IdempotencyKey: idempotencyKey,
			},
		); err != nil {
			publicationErrs = append(publicationErrs, err)
		}
		m.dispatchTaskRunEnqueued(ctx, run, task, actor, idempotencyKey)
	}
	return errorsJoin(publicationErrs...)
}
