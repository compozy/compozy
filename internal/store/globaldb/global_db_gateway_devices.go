package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ gateway.DeviceStore = (*GatewayRepo)(nil)

func (g *GatewayRepo) CreateDevice(ctx context.Context, record gateway.StoredDeviceSession) error {
	if err := g.checkReady(ctx, "create gateway device"); err != nil {
		return err
	}
	epoch, err := gatewayDeviceEpoch(record.Session.RevokeEpoch)
	if err != nil {
		return err
	}
	err = g.queries.CreateGatewayDevice(ctx, sqlcgen.CreateGatewayDeviceParams{
		ID: record.Session.ID, Name: record.Session.Name, TokenHash: record.TokenHash,
		ActorKind: string(record.Session.ActorKind), PairingOrigin: record.Session.PairingOrigin,
		RevokeEpoch: epoch, CreatedAt: store.FormatTimestamp(record.Session.CreatedAt),
		LastSeenAt: nullableGatewayTime(record.Session.LastSeenAt),
		RevokedAt:  nullableGatewayTime(record.Session.RevokedAt),
	})
	if err != nil {
		return fmt.Errorf("store: create gateway device: %w", err)
	}
	return nil
}

func (g *GatewayRepo) FindDeviceByTokenHash(
	ctx context.Context,
	tokenHash string,
	now time.Time,
) (gateway.StoredDeviceSession, error) {
	if err := g.checkReady(ctx, "find gateway device credential"); err != nil {
		return gateway.StoredDeviceSession{}, err
	}
	row, err := g.queries.FindActiveGatewayDeviceByTokenHash(
		ctx,
		sqlcgen.FindActiveGatewayDeviceByTokenHashParams{
			LastSeenAt: nullableGatewayTime(now), TokenHash: tokenHash,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return gateway.StoredDeviceSession{}, gateway.ErrDeviceNotFound
	}
	if err != nil {
		return gateway.StoredDeviceSession{}, fmt.Errorf("store: find gateway device credential: %w", err)
	}
	return gatewayStoredDeviceSessionFromGenerated(row)
}

func (g *GatewayRepo) ListDevices(ctx context.Context) ([]gateway.DeviceSession, error) {
	if err := g.checkReady(ctx, "list gateway devices"); err != nil {
		return nil, err
	}
	rows, err := g.queries.ListGatewayDevices(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list gateway devices: %w", err)
	}
	devices := make([]gateway.DeviceSession, 0, len(rows))
	for _, row := range rows {
		device, mapErr := gatewayDeviceSessionFromGenerated(row)
		if mapErr != nil {
			return nil, mapErr
		}
		devices = append(devices, device)
	}
	return devices, nil
}

func (g *GatewayRepo) GetDevice(ctx context.Context, deviceID string) (gateway.DeviceSession, error) {
	if err := g.checkReady(ctx, "get gateway device"); err != nil {
		return gateway.DeviceSession{}, err
	}
	row, err := g.queries.GetGatewayDevice(ctx, deviceID)
	if errors.Is(err, sql.ErrNoRows) {
		return gateway.DeviceSession{}, gateway.ErrDeviceNotFound
	}
	if err != nil {
		return gateway.DeviceSession{}, fmt.Errorf("store: get gateway device: %w", err)
	}
	return gatewayDeviceSessionFromGenerated(row)
}

func (g *GatewayRepo) RenameDevice(
	ctx context.Context,
	deviceID string,
	name string,
) (gateway.DeviceSession, error) {
	if err := g.checkReady(ctx, "rename gateway device"); err != nil {
		return gateway.DeviceSession{}, err
	}
	row, err := g.queries.RenameGatewayDevice(ctx, sqlcgen.RenameGatewayDeviceParams{
		Name: name, ID: deviceID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return gateway.DeviceSession{}, gateway.ErrDeviceNotFound
	}
	if err != nil {
		return gateway.DeviceSession{}, fmt.Errorf("store: rename gateway device: %w", err)
	}
	return gatewayDeviceSessionFromGenerated(row)
}

func (g *GatewayRepo) RenameDeviceForActor(
	ctx context.Context,
	actorID string,
	actorEpoch uint64,
	deviceID string,
	name string,
) (gateway.DeviceSession, error) {
	if err := g.checkReady(ctx, "rename gateway device with actor fence"); err != nil {
		return gateway.DeviceSession{}, err
	}
	epoch, err := gatewayDeviceEpoch(actorEpoch)
	if err != nil {
		return gateway.DeviceSession{}, err
	}
	row, err := g.queries.RenameGatewayDeviceForActor(ctx, sqlcgen.RenameGatewayDeviceForActorParams{
		Name: name, TargetID: deviceID, ActorID: actorID, ActorEpoch: epoch,
	})
	if err == nil {
		return gatewayDeviceSessionFromGenerated(row)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return gateway.DeviceSession{}, fmt.Errorf("store: rename gateway device with actor fence: %w", err)
	}
	return gateway.DeviceSession{}, g.classifyDeviceMutationMiss(ctx, actorID, epoch, deviceID)
}

func (g *GatewayRepo) RevokeDevice(
	ctx context.Context,
	deviceID string,
	now time.Time,
) (gateway.DeviceSession, bool, error) {
	if err := g.checkReady(ctx, "revoke gateway device"); err != nil {
		return gateway.DeviceSession{}, false, err
	}
	row, err := g.queries.RevokeGatewayDevice(ctx, sqlcgen.RevokeGatewayDeviceParams{
		RevokedAt: nullableGatewayTime(now), ID: deviceID,
	})
	if err == nil {
		device, mapErr := gatewayDeviceSessionFromGenerated(row)
		return device, true, mapErr
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return gateway.DeviceSession{}, false, fmt.Errorf("store: revoke gateway device: %w", err)
	}
	device, getErr := g.GetDevice(ctx, deviceID)
	if getErr != nil {
		return gateway.DeviceSession{}, false, getErr
	}
	return device, false, nil
}

func (g *GatewayRepo) RevokeDeviceForActor(
	ctx context.Context,
	actorID string,
	actorEpoch uint64,
	deviceID string,
	now time.Time,
) (gateway.DeviceSession, bool, error) {
	if err := g.checkReady(ctx, "revoke gateway device with actor fence"); err != nil {
		return gateway.DeviceSession{}, false, err
	}
	epoch, err := gatewayDeviceEpoch(actorEpoch)
	if err != nil {
		return gateway.DeviceSession{}, false, err
	}
	row, err := g.queries.RevokeGatewayDeviceForActor(ctx, sqlcgen.RevokeGatewayDeviceForActorParams{
		RevokedAt: nullableGatewayTime(now), TargetID: deviceID,
		ActorID: actorID, ActorEpoch: epoch,
	})
	if err == nil {
		device, mapErr := gatewayDeviceSessionFromGenerated(row)
		return device, true, mapErr
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return gateway.DeviceSession{}, false, fmt.Errorf("store: revoke gateway device with actor fence: %w", err)
	}
	if classifyErr := g.classifyDeviceMutationMiss(ctx, actorID, epoch, deviceID); classifyErr != nil {
		return gateway.DeviceSession{}, false, classifyErr
	}
	device, getErr := g.GetDevice(ctx, deviceID)
	if getErr != nil {
		return gateway.DeviceSession{}, false, getErr
	}
	return device, false, nil
}

func (g *GatewayRepo) RevalidateDeviceEpoch(ctx context.Context, deviceID string, epoch uint64) error {
	if err := g.checkReady(ctx, "revalidate gateway device epoch"); err != nil {
		return err
	}
	value, err := gatewayDeviceEpoch(epoch)
	if err != nil {
		return err
	}
	matches, err := g.queries.GatewayDeviceEpochMatches(ctx, sqlcgen.GatewayDeviceEpochMatchesParams{
		ID: deviceID, RevokeEpoch: value,
	})
	if err != nil {
		return fmt.Errorf("store: revalidate gateway device epoch: %w", err)
	}
	if !matches {
		return gateway.ErrDeviceRevoked
	}
	return nil
}

func (g *GatewayRepo) classifyDeviceMutationMiss(
	ctx context.Context,
	actorID string,
	actorEpoch int64,
	targetID string,
) error {
	matches, err := g.queries.GatewayDeviceEpochMatches(ctx, sqlcgen.GatewayDeviceEpochMatchesParams{
		ID: actorID, RevokeEpoch: actorEpoch,
	})
	if err != nil {
		return fmt.Errorf("store: classify gateway actor epoch: %w", err)
	}
	if !matches {
		return gateway.ErrDeviceRevoked
	}
	if _, err := g.queries.GetGatewayDevice(ctx, targetID); errors.Is(err, sql.ErrNoRows) {
		return gateway.ErrDeviceNotFound
	} else if err != nil {
		return fmt.Errorf("store: classify gateway device target: %w", err)
	}
	return nil
}

func gatewayDeviceEpoch(value uint64) (int64, error) {
	return gatewaySQLiteInteger("device epoch", value)
}
