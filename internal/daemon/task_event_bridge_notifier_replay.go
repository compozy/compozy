package daemon

import (
	"context"
	"fmt"

	bridgepkg "github.com/compozy/compozy/internal/bridges"
	"github.com/compozy/compozy/internal/notifications"
	presetspkg "github.com/compozy/compozy/internal/notifications/presets"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type notificationPermitEventReader interface {
	GetTaskEventRecord(ctx context.Context, eventID string) (taskpkg.EventRecord, error)
}

func (o *bridgeTerminalTaskNotificationObserver) replayDeliveryPermits() {
	if o == nil || o.permits == nil {
		return
	}
	ctx := o.ctx
	cancel := func() {}
	if o.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, o.timeout)
	}
	defer cancel()
	permits, err := o.permits.ListDeliveryPermits(ctx)
	if err != nil {
		if ctx.Err() == nil {
			o.log().Warn("daemon: list notification delivery permits for replay failed", "error", err)
		}
		return
	}
	for _, permit := range permits {
		if err := o.enqueueDeliveryPermit(ctx, permit); err != nil {
			o.log().Warn(
				"daemon: reconstruct notification delivery permit failed",
				"error", err,
				"profile_id", permit.Key.ProfileID,
				"stream_name", permit.Key.StreamName,
				"subject_id", permit.Key.SubjectID,
				"delivery_id", permit.DeliveryID,
			)
		}
	}
}

func (o *bridgeTerminalTaskNotificationObserver) enqueueDeliveryPermit(
	ctx context.Context,
	permit notifications.DeliveryPermit,
) error {
	normalized, err := permit.Normalize(o.now())
	if err != nil {
		return err
	}
	identity, err := notifications.DecodeDeliveryID(normalized.DeliveryID)
	if err != nil {
		return err
	}
	if identity.Cursor != normalized.Key || identity.Kind != notifications.DeliveryKindDeliver {
		return fmt.Errorf("notification delivery permit identity does not match its cursor")
	}
	if normalized.Key.StreamName == bridgepkg.BridgeTaskNotificationStream {
		return o.enqueueTerminalDeliveryPermit(ctx, normalized)
	}
	return o.enqueuePresetDeliveryPermit(ctx, normalized, identity)
}

func (o *bridgeTerminalTaskNotificationObserver) enqueueTerminalDeliveryPermit(
	ctx context.Context,
	permit notifications.DeliveryPermit,
) error {
	if o.notifier == nil {
		return fmt.Errorf("terminal task notifier is unavailable")
	}
	taskRecord, err := o.tasks.GetTask(ctx, permit.Key.SubjectID)
	if err != nil {
		return fmt.Errorf("load task %q for terminal permit: %w", permit.Key.SubjectID, err)
	}
	if taskRecord.ProfileID != permit.Key.ProfileID {
		return fmt.Errorf(
			"task %q profile %q does not match permit profile %q",
			taskRecord.ID,
			taskRecord.ProfileID,
			permit.Key.ProfileID,
		)
	}
	o.enqueueWake(bridgeTerminalTaskNotificationWake{
		taskID:           permit.Key.SubjectID,
		terminalDispatch: true,
		pendingKey:       bridgeTerminalTaskNotificationPendingKey{taskID: permit.Key.SubjectID},
	})
	return nil
}

func (o *bridgeTerminalTaskNotificationObserver) enqueuePresetDeliveryPermit(
	ctx context.Context,
	permit notifications.DeliveryPermit,
	identity notifications.DeliveryIdentity,
) error {
	if o.presets == nil {
		return fmt.Errorf("notification preset dispatcher is unavailable")
	}
	reader, ok := o.tasks.(notificationPermitEventReader)
	if !ok {
		return fmt.Errorf("task event reader cannot reconstruct notification permits")
	}
	record, err := reader.GetTaskEventRecord(ctx, permit.Key.SubjectID)
	if err != nil {
		return fmt.Errorf("load task event %q: %w", permit.Key.SubjectID, err)
	}
	if record.Event.EventType != permit.Key.StreamName || record.Sequence != identity.Sequence {
		return fmt.Errorf(
			"task event %q does not match permit stream %q sequence %d",
			record.Event.ID,
			permit.Key.StreamName,
			identity.Sequence,
		)
	}
	event := presetspkg.EventFromTaskRecord(ctx, o.tasks, record, o.log())
	if event.ProfileID != permit.Key.ProfileID || event.Scope != permit.Key.Scope {
		return fmt.Errorf(
			"task event %q owner does not match permit profile %q scope %q",
			record.Event.ID,
			permit.Key.ProfileID,
			permit.Key.Scope.Kind,
		)
	}
	o.enqueueRecord(record, false, true)
	return nil
}
