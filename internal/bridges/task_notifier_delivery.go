package bridges

import (
	"context"

	"errors"
	"fmt"

	"github.com/compozy/agh/internal/notifications"
	taskpkg "github.com/compozy/agh/internal/task"
)

func (n *TerminalTaskNotifier) deliverSubscription(
	ctx context.Context,
	subscription BridgeTaskSubscription,
) (terminalTaskNotificationOutcome, error) {
	normalized := subscription.Normalize()
	if err := normalized.Validate(); err != nil {
		return terminalTaskNotificationFailed, err
	}

	cursorKey, cursor, err := n.loadTaskNotificationCursor(ctx, normalized)
	if err != nil {
		return terminalTaskNotificationFailed, err
	}
	records, err := n.listTaskNotificationRecords(ctx, normalized, cursor)
	if err != nil {
		return terminalTaskNotificationFailed, err
	}

	deferred := false
	var mismatchErr error
	safeSequence := cursor.LastSequence
	for _, record := range records {
		result, err := n.processTaskNotificationRecord(
			ctx,
			cursorKey,
			normalized,
			record,
		)
		if err != nil {
			return terminalTaskNotificationFailed, err
		}
		outcome := result.decision
		delivered := result.diagnostic
		updatedSequence := result.sequence
		switch outcome {
		case terminalTaskNotificationDecisionDeliver:
			return terminalTaskNotificationDelivered, nil
		case terminalTaskNotificationDecisionSuppress:
			return terminalTaskNotificationSuppressed, nil
		case terminalTaskNotificationDecisionMismatch:
			safeSequence = updatedSequence
			mismatchErr = errors.Join(mismatchErr, delivered)
		case terminalTaskNotificationDecisionDefer:
			deferred = true
			goto finalize
		default:
			safeSequence = updatedSequence
		}
	}

finalize:
	if safeSequence > cursor.LastSequence {
		if _, err := n.advanceTaskNotificationCursor(ctx, cursorKey, safeSequence, ""); err != nil {
			return terminalTaskNotificationFailed, fmt.Errorf(
				"bridges: advance task notification cursor for subscription %q: %w",
				normalized.SubscriptionID,
				err,
			)
		}
	}
	if mismatchErr != nil {
		if recordErr := n.recordCursorError(ctx, cursorKey, mismatchErr); recordErr != nil {
			mismatchErr = errors.Join(mismatchErr, recordErr)
		}
		return terminalTaskNotificationFailed, mismatchErr
	}
	if deferred {
		return terminalTaskNotificationDeferred, nil
	}
	return terminalTaskNotificationNoop, nil
}

func (n *TerminalTaskNotifier) processTaskNotificationRecord(
	ctx context.Context,
	cursorKey notifications.CursorKey,
	subscription BridgeTaskSubscription,
	record taskpkg.EventRecord,
) (processTaskNotificationResult, error) {
	if !isTerminalTaskNotificationCandidate(record.Event.EventType) {
		return processTaskNotificationResult{
			decision: terminalTaskNotificationDecisionIgnore,
			sequence: record.Sequence,
		}, nil
	}

	resolution, err := n.resolveTerminalTaskNotification(ctx, subscription, record)
	if err != nil {
		if recordErr := n.recordCursorError(ctx, cursorKey, err); recordErr != nil {
			err = errors.Join(err, recordErr)
		}
		return processTaskNotificationResult{}, err
	}
	switch resolution.decision {
	case terminalTaskNotificationDecisionMismatch:
		return processTaskNotificationResult{
			decision:   resolution.decision,
			diagnostic: resolution.diagnostic,
			sequence:   record.Sequence,
		}, nil
	case terminalTaskNotificationDecisionDefer:
		return processTaskNotificationResult{decision: resolution.decision}, nil
	case terminalTaskNotificationDecisionDeliver:
	default:
		return processTaskNotificationResult{
			decision: terminalTaskNotificationDecisionIgnore,
			sequence: record.Sequence,
		}, nil
	}

	if err := n.deliverNotification(ctx, subscription, resolution.notification); err != nil {
		if errors.Is(err, ErrBridgeNotificationSuppressed) {
			if _, advanceErr := n.advanceTaskNotificationCursor(
				ctx,
				cursorKey,
				record.Sequence,
				resolution.notification.DeliveryID,
			); advanceErr != nil {
				return processTaskNotificationResult{}, fmt.Errorf(
					"bridges: advance suppressed task notification cursor for subscription %q: %w",
					subscription.SubscriptionID,
					advanceErr,
				)
			}
			return processTaskNotificationResult{
				decision: terminalTaskNotificationDecisionSuppress,
				sequence: record.Sequence,
			}, nil
		}
		if recordErr := n.recordCursorError(ctx, cursorKey, err); recordErr != nil {
			err = errors.Join(err, recordErr)
		}
		return processTaskNotificationResult{}, err
	}
	if _, err := n.advanceTaskNotificationCursor(
		ctx,
		cursorKey,
		record.Sequence,
		resolution.notification.DeliveryID,
	); err != nil {
		return processTaskNotificationResult{}, fmt.Errorf(
			"bridges: advance task notification cursor for subscription %q: %w",
			subscription.SubscriptionID,
			err,
		)
	}
	return processTaskNotificationResult{
		decision: terminalTaskNotificationDecisionDeliver,
		sequence: record.Sequence,
	}, nil
}

func (n *TerminalTaskNotifier) advanceTaskNotificationCursor(
	ctx context.Context,
	key notifications.CursorKey,
	sequence int64,
	deliveryID string,
) (notifications.Cursor, error) {
	return n.cursors.Advance(ctx, notifications.AdvanceCursor{
		Key:          key,
		LastSequence: sequence,
		DeliveryID:   deliveryID,
		Now:          n.now(),
	})
}

func (n *TerminalTaskNotifier) loadTaskNotificationCursor(
	ctx context.Context,
	subscription BridgeTaskSubscription,
) (notifications.CursorKey, notifications.Cursor, error) {
	cursorKey := subscription.CursorKey()
	cursor, err := n.cursors.Get(ctx, cursorKey)
	if err != nil && !errors.Is(err, notifications.ErrCursorNotFound) {
		return notifications.CursorKey{}, notifications.Cursor{}, fmt.Errorf(
			"bridges: load task notification cursor for subscription %q: %w",
			subscription.SubscriptionID,
			err,
		)
	}
	return cursorKey, cursor, nil
}

func (n *TerminalTaskNotifier) listTaskNotificationRecords(
	ctx context.Context,
	subscription BridgeTaskSubscription,
	cursor notifications.Cursor,
) ([]taskpkg.EventRecord, error) {
	records, err := n.taskEvents.ListTaskEventRecords(ctx, taskpkg.EventRecordQuery{
		TaskID:        subscription.TaskID,
		AfterSequence: cursor.LastSequence,
		Limit:         n.eventLimit,
	})
	if err != nil {
		return nil, fmt.Errorf(
			"bridges: list task events for subscription %q: %w",
			subscription.SubscriptionID,
			err,
		)
	}
	return records, nil
}
