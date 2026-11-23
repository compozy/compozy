package store

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/compozy/compozy/internal/cloud/store/sqlcgen"
	"github.com/compozy/compozy/internal/remoteproto"
)

var ErrNotFound = pgx.ErrNoRows

type ControlAccount struct {
	ID            string
	State         string
	SecurityEpoch uint64
}

func (q *Queries) ControlAccount(ctx context.Context, id string) (ControlAccount, error) {
	row, err := q.generated.ControlAccount(ctx, id)
	return ControlAccount{ID: row.ID, State: row.State, SecurityEpoch: uint64(row.SecurityEpoch)}, err
}

// AdvanceSecurity makes additions and incremental changes use the same commit fence.
func (q *Queries) AdvanceSecurity(ctx context.Context, event ControlEvent) (uint64, error) {
	if !q.inTx {
		return 0, errors.New("advancing the security epoch requires a transaction")
	}
	epoch, err := q.generated.BumpControlEpoch(ctx, event.AccountID)
	if err != nil {
		return 0, err
	}
	event.SecurityEpoch = uint64(epoch)
	return event.SecurityEpoch, q.NotifyControlEvent(ctx, event)
}

type ControlDevice struct {
	ID         remoteproto.DeviceID
	Name       string
	Kind       string
	Admin      bool
	LastSeenAt time.Time
}

func (q *Queries) ControlDevice(ctx context.Context, account string, id remoteproto.DeviceID) (ControlDevice, error) {
	row, err := q.generated.ControlDevice(ctx, sqlcgen.ControlDeviceParams{AccountID: account, ID: string(id)})
	return controlDevice(row), err
}

func (q *Queries) ListControlDevices(ctx context.Context, account string) ([]ControlDevice, error) {
	rows, err := q.generated.ListControlDevices(ctx, account)
	if err != nil {
		return nil, err
	}
	devices := make([]ControlDevice, 0, len(rows))
	for _, row := range rows {
		devices = append(devices, controlDevice(sqlcgen.ControlDeviceRow(row)))
	}
	return devices, nil
}

func controlDevice(row sqlcgen.ControlDeviceRow) ControlDevice {
	return ControlDevice{
		ID: remoteproto.DeviceID(
			row.ID,
		),
		Name:       row.Name,
		Kind:       row.Kind,
		Admin:      row.Admin,
		LastSeenAt: row.LastSeenAt.Time,
	}
}

func (q *Queries) ControlPrincipalActive(
	ctx context.Context, account string, device remoteproto.DeviceID, jkt, session string,
) (bool, error) {
	return q.generated.ControlPrincipalActive(ctx, sqlcgen.ControlPrincipalActiveParams{
		AccountID: account, DeviceID: string(device), Jkt: jkt, ID: session,
	})
}

func (q *Queries) UpdateControlDevice(ctx context.Context, account string, device ControlDevice) error {
	return q.generated.UpdateControlDevice(ctx, sqlcgen.UpdateControlDeviceParams{
		AccountID: account, ID: string(device.ID), Name: device.Name, Admin: device.Admin,
	})
}

func (q *Queries) RevokeControlDevice(
	ctx context.Context,
	account string,
	id remoteproto.DeviceID,
	at time.Time,
) error {
	device, revokedAt := string(id), timestamp(at)
	if err := q.generated.RevokeControlDevice(ctx, sqlcgen.RevokeControlDeviceParams{
		AccountID: account, ID: device, RevokedAt: revokedAt,
	}); err != nil {
		return err
	}
	if err := q.generated.RevokeControlDeviceKeys(ctx, sqlcgen.RevokeControlDeviceKeysParams{
		DeviceID: device, RevokedAt: revokedAt,
	}); err != nil {
		return err
	}
	if err := q.generated.RevokeControlDeviceSessions(ctx, sqlcgen.RevokeControlDeviceSessionsParams{
		AccountID: account, DeviceID: device, RevokedAt: revokedAt,
	}); err != nil {
		return err
	}
	if err := q.generated.RevokeControlWebSessions(ctx, sqlcgen.RevokeControlWebSessionsParams{
		AccountID: account, BrowserDeviceID: dbText(device), RevokedAt: revokedAt,
	}); err != nil {
		return err
	}
	return q.generated.ReleaseControlDeviceReservations(ctx, sqlcgen.ReleaseControlDeviceReservationsParams{
		AccountID: account, DeviceID: device, ReleasedAt: revokedAt,
	})
}

func (q *Queries) SetControlDeviceScopes(
	ctx context.Context, device remoteproto.DeviceID, computer remoteproto.ComputerID, scopes []remoteproto.Scope,
) (bool, error) {
	values := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		values = append(values, string(scope))
	}
	count, err := q.generated.SetControlDeviceScopes(ctx, sqlcgen.SetControlDeviceScopesParams{
		DeviceID: string(device), ComputerID: dbText(string(computer)), Scopes: values,
	})
	return count > 0, err
}

func (q *Queries) EffectiveControlScopes(
	ctx context.Context, device ControlDevice, computer remoteproto.ComputerID,
) ([]remoteproto.Scope, error) {
	rows, err := q.generated.ControlDeviceScopes(ctx, sqlcgen.ControlDeviceScopesParams{
		DeviceID: string(device.ID), TargetID: string(computer),
	})
	if err != nil {
		return nil, err
	}
	scopes := remoteproto.DefaultScopes(device.Admin)
	for _, row := range rows {
		scopes = restrictScopes(scopes, row.Scopes)
	}
	return scopes, nil
}

func (q *Queries) ControlSnapshotDevices(
	ctx context.Context, account string, computer remoteproto.ComputerID,
) ([]remoteproto.SnapshotDevice, error) {
	rows, err := q.generated.ControlSnapshotDevices(ctx, sqlcgen.ControlSnapshotDevicesParams{
		AccountID: account, TargetID: string(computer),
	})
	if err != nil {
		return nil, err
	}
	devices := make([]remoteproto.SnapshotDevice, 0, len(rows))
	for _, row := range rows {
		scopes := remoteproto.DefaultScopes(row.Admin)
		if row.HasGlobalScopes {
			scopes = restrictScopes(scopes, row.GlobalScopes)
		}
		if row.HasLocalScopes {
			scopes = restrictScopes(scopes, row.LocalScopes)
		}
		devices = append(devices, remoteproto.SnapshotDevice{
			DeviceID: remoteproto.DeviceID(row.ID), JKTs: row.Jkts, Admin: row.Admin, Scopes: scopes,
		})
	}
	return devices, nil
}

func restrictScopes(scopes []remoteproto.Scope, allowed []string) []remoteproto.Scope {
	return slices.DeleteFunc(scopes, func(scope remoteproto.Scope) bool {
		return !slices.Contains(allowed, string(scope))
	})
}

func dbText(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}
