package bridges

import (
	"context"
	"encoding/json"

	"fmt"

	"strings"

	"github.com/compozy/agh/internal/notifications"
	taskpkg "github.com/compozy/agh/internal/task"
)

func (n *TerminalTaskNotifier) resolveTerminalTaskNotification(
	ctx context.Context,
	subscription BridgeTaskSubscription,
	record taskpkg.EventRecord,
) (terminalTaskNotificationResolution, error) {
	taskRecord, err := n.taskEvents.GetTask(ctx, subscription.TaskID)
	if err != nil {
		return terminalTaskNotificationResolution{}, fmt.Errorf(
			"bridges: load task %q for terminal notification: %w",
			subscription.TaskID,
			err,
		)
	}
	taskStatus := taskRecord.Status.Normalize()
	eventStatus, ok := taskStatusForTerminalEvent(record.Event.EventType)
	if !ok {
		return terminalTaskNotificationResolution{decision: terminalTaskNotificationDecisionIgnore}, nil
	}
	if !isTaskTerminalStatus(taskStatus) {
		return terminalTaskNotificationResolution{decision: terminalTaskNotificationDecisionDefer}, nil
	}
	if taskStatus != eventStatus {
		return terminalTaskNotificationResolution{
			decision: terminalTaskNotificationDecisionMismatch,
			diagnostic: terminalTaskNotificationStatusMismatchError(
				subscription,
				record,
				taskStatus,
				eventStatus,
			),
		}, nil
	}

	var run taskpkg.Run
	if strings.TrimSpace(record.Event.RunID) != "" {
		run, err = n.taskEvents.GetTaskRun(ctx, record.Event.RunID)
		if err != nil {
			return terminalTaskNotificationResolution{}, fmt.Errorf(
				"bridges: load task run %q for terminal notification: %w",
				record.Event.RunID,
				err,
			)
		}
		if strings.TrimSpace(run.TaskID) != subscription.TaskID {
			return terminalTaskNotificationResolution{}, fmt.Errorf(
				"bridges: terminal event run %q belongs to task %q, want %q",
				run.ID,
				run.TaskID,
				subscription.TaskID,
			)
		}
		if statusFromRun, runOK := taskStatusForRunStatus(run.Status); !runOK || statusFromRun != eventStatus {
			return terminalTaskNotificationResolution{
				decision: terminalTaskNotificationDecisionMismatch,
				diagnostic: terminalTaskNotificationRunMismatchError(
					subscription,
					record,
					run.Status.Normalize(),
					eventStatus,
				),
			}, nil
		}
	}

	notification, err := terminalTaskNotificationEnvelope(subscription, record, eventStatus, run)
	if err != nil {
		return terminalTaskNotificationResolution{}, err
	}
	return terminalTaskNotificationResolution{
		decision:     terminalTaskNotificationDecisionDeliver,
		notification: notification,
	}, nil
}

func terminalTaskNotificationStatusMismatchError(
	subscription BridgeTaskSubscription,
	record taskpkg.EventRecord,
	current taskpkg.Status,
	expected taskpkg.Status,
) error {
	return fmt.Errorf(
		"%w: subscription %q task %q event %q sequence %d expects task status %q but current task status is %q",
		ErrTerminalTaskNotificationMismatch,
		subscription.SubscriptionID,
		subscription.TaskID,
		strings.TrimSpace(record.Event.EventType),
		record.Sequence,
		expected.Normalize(),
		current.Normalize(),
	)
}

func terminalTaskNotificationRunMismatchError(
	subscription BridgeTaskSubscription,
	record taskpkg.EventRecord,
	current taskpkg.RunStatus,
	expected taskpkg.Status,
) error {
	return fmt.Errorf(
		"%w: subscription %q task %q run %q event %q sequence %d "+
			"expects run-derived task status %q but current run status is %q",
		ErrTerminalTaskNotificationMismatch,
		subscription.SubscriptionID,
		subscription.TaskID,
		strings.TrimSpace(record.Event.RunID),
		strings.TrimSpace(record.Event.EventType),
		record.Sequence,
		expected.Normalize(),
		current.Normalize(),
	)
}

func (n *TerminalTaskNotifier) deliverNotification(
	ctx context.Context,
	subscription BridgeTaskSubscription,
	notification TerminalTaskNotification,
) error {
	instance, err := n.instances.GetBridgeInstance(ctx, subscription.BridgeInstanceID)
	if err != nil {
		return fmt.Errorf(
			"bridges: load bridge instance %q for task notification: %w",
			subscription.BridgeInstanceID,
			err,
		)
	}
	if !instance.Enabled || instance.Status.Normalize() != BridgeStatusReady {
		return fmt.Errorf(
			"%w: bridge instance %q status %q enabled=%t",
			ErrBridgeInstanceUnavailable,
			instance.ID,
			instance.Status.Normalize(),
			instance.Enabled,
		)
	}
	if instance.NotificationSuppress {
		return fmt.Errorf("%w: bridge instance %q", ErrBridgeNotificationSuppressed, instance.ID)
	}

	metadata, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("bridges: encode terminal task notification: %w", err)
	}
	event := DeliveryEvent{
		DeliveryID:       notification.DeliveryID,
		BridgeInstanceID: subscription.BridgeInstanceID,
		RoutingKey:       subscription.RoutingKey(),
		DeliveryTarget:   subscription.DeliveryTarget(),
		Seq:              notification.Seq,
		EventType:        DeliveryEventTypeFinal,
		Content:          MessageContent{Text: terminalTaskNotificationText(notification)},
		Final:            true,
		Operation:        DeliveryOperationPost,
		ProviderMetadata: metadata,
	}
	if err := event.Validate(); err != nil {
		return err
	}

	ack, err := n.transport.DeliverBridge(ctx, instance.ExtensionName, DeliveryRequest{Event: event})
	if err != nil {
		return fmt.Errorf(
			"bridges: deliver terminal task notification %q: %w",
			notification.DeliveryID,
			err,
		)
	}
	if err := ack.ValidateFor(event); err != nil {
		return fmt.Errorf(
			"bridges: validate terminal task notification ack %q: %w",
			notification.DeliveryID,
			err,
		)
	}
	return nil
}

func (n *TerminalTaskNotifier) recordCursorError(
	ctx context.Context,
	key notifications.CursorKey,
	cause error,
) error {
	lastError := redactTerminalTaskNotificationText(cause.Error())
	if _, err := n.cursors.RecordError(ctx, notifications.CursorError{
		Key:       key,
		LastError: truncateTerminalTaskCursorError(lastError),
		Now:       n.now(),
	}); err != nil {
		return fmt.Errorf("bridges: record terminal task notification cursor error: %w", err)
	}
	return nil
}
