package gateway

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/workspaceaccess"
)

// RemoveDevice revokes when needed, records the durable removal, and deletes the device record.
func (s *DeviceService) RemoveDevice(ctx context.Context, deviceID string) (RemoveResult, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return RemoveResult{}, ErrDeviceNotFound
	}
	operationCtx := ctx
	selfRemoval := releaseMutationForDevice(ctx, deviceID)
	if selfRemoval {
		operationCtx = context.WithValue(context.WithoutCancel(ctx), selfRevocationContextKey{}, deviceID)
	}
	device, err := s.store.GetDevice(operationCtx, deviceID)
	if err != nil {
		return RemoveResult{}, fmt.Errorf("gateway: get device to remove: %w", err)
	}
	result := RemoveResult{Device: device}
	if device.RevokedAt.IsZero() {
		revoked, revokeErr := s.revokeForRemoval(operationCtx, deviceID, selfRemoval)
		if revokeErr != nil {
			return RemoveResult{}, revokeErr
		}
		result.Device = revoked.Device
		result.Revoked = revoked.Changed
		result.Canceled = revoked.Canceled
	}
	if err := s.recordDeviceRemoved(operationCtx, result.Device); err != nil {
		return RemoveResult{}, err
	}
	removed, err := s.deleteDeviceRecord(operationCtx, deviceID, selfRemoval)
	if err != nil {
		return RemoveResult{}, err
	}
	result.Device = removed
	return result, nil
}

func (s *DeviceService) revokeForRemoval(
	ctx context.Context,
	deviceID string,
	selfRemoval bool,
) (RevokeResult, error) {
	return s.revokeSession(ctx, deviceID, func(operationCtx context.Context) (DeviceSession, bool, error) {
		if !selfRemoval {
			if actor, ok := DeviceFromContext(operationCtx); ok {
				return s.store.RevokeDeviceForActor(
					operationCtx,
					actor.ID,
					actor.RevokeEpoch,
					deviceID,
					s.now(),
				)
			}
		}
		return s.store.RevokeDevice(operationCtx, deviceID, s.now())
	})
}

// deleteDeviceRecord keeps the actor fence except when the caller is the removed device.
func (s *DeviceService) deleteDeviceRecord(
	ctx context.Context,
	deviceID string,
	selfRemoval bool,
) (DeviceSession, error) {
	actor, hasActor := DeviceFromContext(ctx)
	var (
		device DeviceSession
		err    error
	)
	if hasActor && !selfRemoval && actor.ID != deviceID {
		device, err = s.store.DeleteDeviceForActor(ctx, actor.ID, actor.RevokeEpoch, deviceID)
	} else {
		device, err = s.store.DeleteDevice(ctx, deviceID)
	}
	if err != nil {
		return DeviceSession{}, fmt.Errorf("gateway: delete device record: %w", err)
	}
	return device, nil
}

func (s *DeviceService) recordDeviceRemoved(ctx context.Context, device DeviceSession) error {
	// Removal is only offered when its audit record is guaranteed: without a
	// sink the device would disappear with no durable trace, so fail before the
	// destructive step instead of silently skipping the event.
	if s.events == nil {
		return errors.New("gateway: device removal requires a configured event sink")
	}
	if err := s.events.RecordDeviceRemoved(
		ctx,
		deviceMutationEvent(ctx, device, s.now()),
	); err != nil {
		return fmt.Errorf("gateway: record device removed event: %w", err)
	}
	return nil
}

func deviceMutationEvent(
	ctx context.Context,
	device DeviceSession,
	removedAt time.Time,
) DeviceMutationEvent {
	event := DeviceMutationEvent{Device: device, RemovedAt: removedAt.UTC()}
	if actor, ok := DeviceFromContext(ctx); ok {
		event.ActorKind = string(actor.ActorKind)
		event.ActorID = strings.TrimSpace(actor.ID)
		return event
	}
	event.ActorKind = string(workspaceaccess.ActorHuman)
	event.ActorID = "local-user"
	return event
}
