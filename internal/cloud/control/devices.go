package control

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

func viewDevice(device store.ControlDevice, caller remoteproto.DeviceID) remoteproto.AccountDevice {
	return remoteproto.AccountDevice{
		ID: device.ID, Name: device.Name, Kind: device.Kind, Admin: device.Admin,
		LastSeenAt: device.LastSeenAt, ThisDevice: device.ID == caller,
	}
}

func (s *Service) ListDevices(ctx context.Context, principal DevicePrincipal) ([]remoteproto.AccountDevice, error) {
	var devices []remoteproto.AccountDevice
	err := s.deviceTransaction(
		ctx,
		principal,
		func(q *store.Queries, account store.ControlAccount, _ store.ControlDevice) error {
			found, err := q.ListControlDevices(ctx, account.ID)
			if err != nil {
				return err
			}
			devices = make([]remoteproto.AccountDevice, 0, len(found))
			for _, device := range found {
				devices = append(devices, viewDevice(device, principal.DeviceID))
			}
			return nil
		},
	)
	return devices, err
}

func (s *Service) GetDevice(
	ctx context.Context,
	principal DevicePrincipal,
	id remoteproto.DeviceID,
) (remoteproto.AccountDevice, error) {
	var result remoteproto.AccountDevice
	err := s.deviceTransaction(
		ctx,
		principal,
		func(q *store.Queries, account store.ControlAccount, _ store.ControlDevice) error {
			device, err := findDevice(ctx, q, account.ID, id)
			result = viewDevice(device, principal.DeviceID)
			return err
		},
	)
	return result, err
}

func findDevice(
	ctx context.Context,
	q *store.Queries,
	account string,
	id remoteproto.DeviceID,
) (store.ControlDevice, error) {
	device, err := q.ControlDevice(ctx, account, id)
	if errors.Is(err, store.ErrNotFound) {
		return store.ControlDevice{}, errDeviceNotFound
	}
	return device, err
}

func (s *Service) PatchDevice(
	ctx context.Context, principal DevicePrincipal, id remoteproto.DeviceID, patch remoteproto.AccountDevicePatch,
) (remoteproto.AccountDevice, error) {
	var result remoteproto.AccountDevice
	err := s.deviceTransaction(
		ctx,
		principal,
		func(q *store.Queries, account store.ControlAccount, caller store.ControlDevice) error {
			if !caller.Admin && (patch.Admin != nil || len(patch.Scopes) > 0 || id != caller.ID) {
				return errAdminRequired
			}
			device, err := findDevice(ctx, q, account.ID, id)
			if err != nil {
				return err
			}
			event := store.ControlEvent{AccountID: account.ID, ResourceID: string(id)}
			if err := s.applyDevicePatch(ctx, q, &device, patch, caller.ID, &event); err != nil {
				return err
			}
			result = viewDevice(device, caller.ID)
			if event.Type == "" {
				return nil
			}
			if event.Type == remoteproto.ControlDevicesChanged {
				return q.NotifyControlEvent(ctx, event)
			}
			_, err = q.AdvanceSecurity(ctx, event)
			return err
		},
	)
	return result, err
}

func (s *Service) applyDevicePatch(
	ctx context.Context,
	q *store.Queries,
	device *store.ControlDevice,
	patch remoteproto.AccountDevicePatch,
	caller remoteproto.DeviceID,
	event *store.ControlEvent,
) error {
	if patch.Name != nil {
		name := strings.TrimSpace(*patch.Name)
		if name == "" || len(name) > 128 {
			return ErrInvalidInput
		}
		if name != device.Name {
			device.Name = name
			event.Type = remoteproto.ControlDevicesChanged
		}
	}
	if patch.Admin != nil && *patch.Admin != device.Admin {
		device.Admin = *patch.Admin
		event.Type = remoteproto.ControlSnapshot
		if err := s.securityEvent(ctx, q, event.AccountID, caller, "device_admin_changed",
			map[string]any{auditDeviceID: device.ID, "admin": device.Admin}); err != nil {
			return err
		}
	}
	if event.Type != "" {
		if err := q.UpdateControlDevice(ctx, event.AccountID, *device); err != nil {
			return err
		}
	}
	changed, err := s.patchDeviceScopes(ctx, q, event.AccountID, device.ID, patch.Scopes)
	if err != nil || !changed {
		return err
	}
	if event.Type != remoteproto.ControlSnapshot {
		event.Type = remoteproto.ControlScopesChanged
	}
	return s.securityEvent(
		ctx,
		q,
		event.AccountID,
		caller,
		"device_scopes_changed",
		map[string]any{auditDeviceID: device.ID},
	)
}

func (s *Service) patchDeviceScopes(
	ctx context.Context, q *store.Queries, account string, device remoteproto.DeviceID,
	byComputer map[remoteproto.ComputerID][]remoteproto.Scope,
) (bool, error) {
	var changed bool
	for id, scopes := range byComputer {
		if _, err := ownedComputer(ctx, q, account, id); err != nil {
			return false, err
		}
		if scopes == nil || slices.ContainsFunc(scopes, func(scope remoteproto.Scope) bool { return !scope.Valid() }) {
			return false, ErrInvalidInput
		}
		scopes = slices.Compact(slices.Sorted(slices.Values(scopes)))
		updated, err := q.SetControlDeviceScopes(ctx, device, id, scopes)
		if err != nil {
			return false, err
		}
		changed = changed || updated
	}
	return changed, nil
}

func (s *Service) RevokeDevice(ctx context.Context, principal DevicePrincipal, id remoteproto.DeviceID) error {
	return s.deviceTransaction(
		ctx,
		principal,
		func(q *store.Queries, account store.ControlAccount, caller store.ControlDevice) error {
			if id != caller.ID && !caller.Admin {
				return errAdminRequired
			}
			if _, err := findDevice(ctx, q, account.ID, id); err != nil {
				return err
			}
			if err := q.RevokeControlDevice(ctx, account.ID, id, s.config.Now()); err != nil {
				return err
			}
			if err := s.securityEvent(
				ctx,
				q,
				account.ID,
				caller.ID,
				"device_revoked",
				map[string]any{auditDeviceID: id},
			); err != nil {
				return err
			}
			_, err := q.AdvanceSecurity(ctx, store.ControlEvent{
				Type: remoteproto.ControlRevokeDevice, AccountID: account.ID, ResourceID: string(id),
			})
			return err
		},
	)
}
