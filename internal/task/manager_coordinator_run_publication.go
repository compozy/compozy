package task

import (
	"context"
	"fmt"
)

func (m *Service) publishCoordinatorEnqueuedRuns(
	ctx context.Context,
	runs []Run,
	actor ActorContext,
) error {
	var publicationErrs []error
	for _, run := range runs {
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
		if err := m.recordTaskEvent(
			ctx,
			run.TaskID,
			run.ID,
			taskEventRunEnqueued,
			actor,
			runEnqueuedPayload{
				Attempt:        int(run.Attempt),
				Status:         run.Status,
				TaskStatus:     task.Status,
				IdempotencyKey: run.IdempotencyKey,
			},
		); err != nil {
			publicationErrs = append(publicationErrs, err)
		}
		m.dispatchTaskRunEnqueued(ctx, run, task, actor, run.IdempotencyKey)
	}
	return errorsJoin(publicationErrs...)
}
