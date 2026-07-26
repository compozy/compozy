package daemon

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/agh/internal/notifications"
	taskpkg "github.com/compozy/agh/internal/task"
)

const (
	taskStatusProjectionCursorConsumer = "task-status-projection"
	taskStatusProjectionCursorStream   = "task-events"
	taskStatusProjectionCursorSubject  = "installation"
)

var taskStatusProjectionCursorKey = notifications.CursorKey{
	ConsumerID: taskStatusProjectionCursorConsumer,
	StreamName: taskStatusProjectionCursorStream,
	SubjectID:  taskStatusProjectionCursorSubject,
}

func (o *taskStatusProjectionObserver) signalTaskStatusProjection() {
	if o == nil {
		return
	}
	select {
	case <-o.ctx.Done():
		return
	case o.queue <- struct{}{}:
	default:
	}
}

func (o *taskStatusProjectionObserver) start() {
	o.wg.Go(func() {
		for {
			select {
			case <-o.ctx.Done():
				return
			case <-o.queue:
				if !o.replayTaskStatusProjections() {
					return
				}
			}
		}
	})
	o.signalTaskStatusProjection()
}

func (o *taskStatusProjectionObserver) replayTaskStatusProjections() bool {
	for {
		err := o.catchUpTaskStatusProjections(o.ctx)
		if err == nil {
			return true
		}
		o.recordTaskStatusProjectionError(err)
		o.logger.Warn("daemon: task status projection replay failed", "error", err)
		timer := time.NewTimer(o.retryDelay)
		select {
		case <-o.ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return false
		case <-timer.C:
		}
	}
}

func (o *taskStatusProjectionObserver) catchUpTaskStatusProjections(ctx context.Context) error {
	after, err := o.taskStatusProjectionCursor(ctx)
	if err != nil {
		return err
	}
	for {
		records, listErr := o.events.ListTaskEventRecords(ctx, taskpkg.EventRecordQuery{
			AfterSequence: after,
			Limit:         o.batchSize,
			AllTasks:      true,
		})
		if listErr != nil {
			return fmt.Errorf("list task status projection events after %d: %w", after, listErr)
		}
		if len(records) == 0 {
			return nil
		}
		for _, record := range records {
			if taskStatusProjectionEvent(record.Event.EventType) {
				projectionCtx, cancel := context.WithTimeout(ctx, o.timeout)
				processErr := o.processWithContext(projectionCtx, record)
				cancel()
				if processErr != nil {
					return fmt.Errorf(
						"project task event %q at sequence %d: %w",
						record.Event.ID,
						record.Sequence,
						processErr,
					)
				}
			}
			if advanceErr := o.advanceTaskStatusProjectionCursor(ctx, record); advanceErr != nil {
				return advanceErr
			}
			after = record.Sequence
		}
		if len(records) < o.batchSize {
			return nil
		}
	}
}

func (o *taskStatusProjectionObserver) taskStatusProjectionCursor(ctx context.Context) (int64, error) {
	cursor, err := o.cursors.GetCursor(ctx, taskStatusProjectionCursorKey)
	switch {
	case errors.Is(err, notifications.ErrCursorNotFound):
		return 0, nil
	case err != nil:
		return 0, fmt.Errorf("load task status projection cursor: %w", err)
	default:
		return cursor.LastSequence, nil
	}
}

func (o *taskStatusProjectionObserver) advanceTaskStatusProjectionCursor(
	ctx context.Context,
	record taskpkg.EventRecord,
) error {
	_, err := o.cursors.AdvanceCursor(ctx, notifications.AdvanceCursor{
		Key:             taskStatusProjectionCursorKey,
		LastSequence:    record.Sequence,
		LastDeliveredAt: record.Event.Timestamp,
		DeliveryID:      record.Event.ID,
		Now:             o.now().UTC(),
	})
	if err != nil {
		return fmt.Errorf(
			"advance task status projection cursor to %d for event %q: %w",
			record.Sequence,
			record.Event.ID,
			err,
		)
	}
	return nil
}

func (o *taskStatusProjectionObserver) recordTaskStatusProjectionError(replayErr error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(o.ctx), o.timeout)
	defer cancel()
	if _, err := o.cursors.RecordCursorError(ctx, notifications.CursorError{
		Key:       taskStatusProjectionCursorKey,
		LastError: replayErr.Error(),
		Now:       o.now().UTC(),
	}); err != nil {
		o.logger.Warn(
			"daemon: record task status projection cursor error failed",
			"error", err,
			"projection_error", replayErr,
		)
	}
}
