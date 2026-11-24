package remotehost

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/remoteproto"
)

type Device struct {
	ID              remoteproto.DeviceID `json:"id"`
	Name            string               `json:"name"`
	Scopes          []remoteproto.Scope  `json:"scopes"`
	Admin           bool                 `json:"admin"`
	LastSeenAt      *time.Time           `json:"last_seen_at"`
	PendingApproval bool                 `json:"pending_approval"`
	Approvals       []Approval           `json:"approvals,omitempty"`
}

type DevicePatch struct {
	Scopes     *[]remoteproto.Scope `json:"scopes,omitempty"`
	Approved   *bool                `json:"approved,omitempty"`
	ApprovalID string               `json:"approval_id,omitempty"`
}

func (h *Host) Devices(ctx context.Context) ([]Device, error) {
	return h.devices(ctx, "")
}

func (h *Host) devices(ctx context.Context, includeUnseen remoteproto.DeviceID) ([]Device, error) {
	identity, sessions, policy := h.deviceState()
	if identity.ComputerID == "" {
		return []Device{}, nil
	}
	snapshot, err := h.deviceSnapshot(ctx, identity, sessions)
	if err != nil {
		return nil, err
	}
	connections, err := h.store.RemoteDeviceConnections(ctx)
	if err != nil {
		return nil, err
	}
	approvals, err := h.store.RemoteApprovals(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[remoteproto.DeviceID]remoteproto.AccountDevice)
	if h.account.Status().AccountID == identity.AccountID {
		list, err := h.account.Devices(ctx)
		if err != nil {
			h.opts.Logger.DebugContext(ctx, "Could not read account device names", "code", hostFailureCode(err))
		} else if h.account.Status().AccountID == identity.AccountID {
			for _, item := range list {
				byID[item.ID] = item
			}
		}
	}
	out := make([]Device, 0, len(snapshot.Devices))
	now := h.opts.Now()
	for _, device := range snapshot.Devices {
		entry := Device{ID: device.DeviceID, Name: string(device.DeviceID), Admin: device.Admin,
			Scopes: effectiveScopes(device.Scopes, device, policy)}
		lastSeenAt, connected := connections[device.DeviceID]
		if connected {
			entry.LastSeenAt = new(lastSeenAt)
		}
		if found, ok := byID[device.DeviceID]; ok {
			entry.Name = found.Name
		}
		entry.Approvals, entry.PendingApproval = approvalsForDevice(approvals, device, entry.Name, now)
		if !connected && len(entry.Approvals) == 0 && device.DeviceID != includeUnseen {
			continue
		}
		out = append(out, entry)
	}
	return out, nil
}

func approvalsForDevice(
	approvals []Approval, device remoteproto.SnapshotDevice, name string, now time.Time,
) ([]Approval, bool) {
	var matches []Approval
	var pending bool
	for _, approval := range approvals {
		if approval.DeviceID != device.DeviceID || !slices.Contains(device.JKTs, approval.JKT) {
			continue
		}
		approval.Name = name
		if approval.State == approvalPending && !now.Before(approval.ExpiresAt) {
			approval.State = approvalExpired
		}
		pending = pending || approval.State == approvalPending
		matches = append(matches, approval)
	}
	return matches, pending
}

func (h *Host) deviceSnapshot(
	ctx context.Context,
	identity Identity,
	sessions *Sessions,
) (remoteproto.SecuritySnapshot, error) {
	if sessions != nil {
		return sessions.Snapshot(), nil
	}
	snapshot, err := h.store.RemoteSnapshot(ctx, identity)
	if errors.Is(err, ErrMissing) {
		return remoteproto.SecuritySnapshot{}, nil
	}
	return snapshot, err
}

func (h *Host) deviceState() (Identity, *Sessions, []remoteproto.Scope) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.forgetUnlinkedLocked()
	var sessions *Sessions
	if h.run != nil {
		sessions = h.run.sessions
	}
	return h.identity, sessions, slices.Clone(h.config.MaxScopes)
}

func (h *Host) PatchDevice(ctx context.Context, id remoteproto.DeviceID, patch DevicePatch) (Device, error) {
	identity, sessions, _ := h.deviceState()
	if identity.ComputerID == "" {
		return Device{}, ErrNotLinked
	}
	if patch.Approved == nil && patch.Scopes == nil {
		return Device{}, &Failure{
			http.StatusBadRequest,
			invalidRemoteRequestCode,
			"Specify scopes or an approval decision.",
			nil,
		}
	}
	var requested []remoteproto.Scope
	if patch.Scopes != nil {
		requested = append([]remoteproto.Scope{}, (*patch.Scopes)...)
		if slices.ContainsFunc(requested, func(scope remoteproto.Scope) bool { return !scope.Valid() }) {
			return Device{}, &Failure{
				http.StatusBadRequest,
				invalidRemoteRequestCode,
				"An unknown scope was supplied.",
				nil,
			}
		}
		if h.account.Status().AccountID != identity.AccountID {
			return Device{}, account.ErrSignInRequired
		}
		_, err := h.account.PatchDevice(ctx, string(id), remoteproto.AccountDevicePatch{
			Scopes: map[remoteproto.ComputerID][]remoteproto.Scope{identity.ComputerID: requested},
		})
		if err != nil {
			return Device{}, err
		}
	}
	if patch.Approved != nil {
		if sessions == nil {
			return Device{}, ErrNotLinked
		}
		if err := sessions.DecideLocal(ctx, id, patch.ApprovalID, *patch.Approved); err != nil {
			return Device{}, err
		}
	}
	return h.updatedDevice(ctx, id, requested)
}

func (h *Host) updatedDevice(
	ctx context.Context,
	id remoteproto.DeviceID,
	requested []remoteproto.Scope,
) (Device, error) {
	devices, err := h.devices(ctx, id)
	if err != nil {
		return Device{}, err
	}
	for _, device := range devices {
		if device.ID == id {
			if requested != nil {
				device.Scopes = requested
			}
			return device, nil
		}
	}
	return Device{}, ErrDeviceNotFound
}

func (h *Host) RevokeDevice(ctx context.Context, id remoteproto.DeviceID) error {
	identity, sessions, _ := h.deviceState()
	if identity.ComputerID == "" {
		return ErrNotLinked
	}
	if h.account.Status().AccountID != identity.AccountID {
		return account.ErrSignInRequired
	}
	if err := h.account.RevokeDevice(ctx, string(id)); err != nil {
		return err
	}
	if sessions == nil {
		return nil
	}
	_, err := sessions.RevokeDevice(ctx, id)
	return err
}
