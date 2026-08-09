package globaldb

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func gatewayProviderParams(identity gateway.ProviderIdentity, now time.Time) sqlcgen.UpsertGatewayProviderParams {
	confirmedAt := identity.ConfirmedAt
	if confirmedAt.IsZero() && identity.DigestConfirmed != "" {
		confirmedAt = now
	}
	return sqlcgen.UpsertGatewayProviderParams{
		Name: identity.Name, InstallSource: identity.InstallSource,
		DigestConfirmed: store.SQLNullString(identity.DigestConfirmed),
		ConfirmedAt:     nullableGatewayTime(confirmedAt),
	}
}

func gatewayProviderActivationFromGenerated(
	row sqlcgen.GatewayProviderActivation,
) (gateway.ProviderActivation, error) {
	generation, err := gatewayUnsignedValue("provider generation", row.Generation)
	if err != nil {
		return gateway.ProviderActivation{}, err
	}
	lastHealthAt, err := parseNullableGatewayTime(row.LastHealthAt)
	if err != nil {
		return gateway.ProviderActivation{}, fmt.Errorf("store: parse gateway provider health time: %w", err)
	}
	return gateway.ProviderActivation{
		ProviderName: row.ProviderName, Tier: gateway.Tier(row.Tier),
		Desired:    gateway.DesiredState(row.DesiredState),
		Observed:   gateway.ProviderObservedState(row.ObservedState),
		Generation: generation, LastHealthAt: lastHealthAt,
		LastError: row.LastError.String,
	}, nil
}

func gatewaySurfaceExposureFromGenerated(
	row sqlcgen.GatewaySurfaceExposure,
) (gateway.SurfaceExposure, error) {
	generation, err := gatewayUnsignedValue("surface generation", row.Generation)
	if err != nil {
		return gateway.SurfaceExposure{}, err
	}
	consentedAt, err := parseNullableGatewayTime(row.ConsentedAt)
	if err != nil {
		return gateway.SurfaceExposure{}, fmt.Errorf("store: parse gateway consent time: %w", err)
	}
	return gateway.SurfaceExposure{
		Surface: gateway.Surface(row.Surface), Tier: gateway.Tier(row.Tier),
		Desired:    gateway.DesiredState(row.DesiredState),
		Observed:   gateway.SurfaceObservedState(row.ObservedState),
		Generation: generation, ConsentedAt: consentedAt,
	}, nil
}

func gatewayDeviceSessionFromGenerated(
	row sqlcgen.GatewayDeviceSession,
) (gateway.DeviceSession, error) {
	revokeEpoch, err := gatewayUnsignedValue("device revoke epoch", row.RevokeEpoch)
	if err != nil {
		return gateway.DeviceSession{}, err
	}
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return gateway.DeviceSession{}, fmt.Errorf("store: parse gateway device created time: %w", err)
	}
	lastSeenAt, err := parseNullableGatewayTime(row.LastSeenAt)
	if err != nil {
		return gateway.DeviceSession{}, fmt.Errorf("store: parse gateway device last-seen time: %w", err)
	}
	revokedAt, err := parseNullableGatewayTime(row.RevokedAt)
	if err != nil {
		return gateway.DeviceSession{}, fmt.Errorf("store: parse gateway device revoked time: %w", err)
	}
	return gateway.DeviceSession{
		ID: row.ID, Name: row.Name, ActorKind: gateway.ActorKind(row.ActorKind),
		PairingOrigin: row.PairingOrigin, RevokeEpoch: revokeEpoch,
		CreatedAt: createdAt, LastSeenAt: lastSeenAt, RevokedAt: revokedAt,
	}, nil
}

func gatewayStoredDeviceSessionFromGenerated(
	row sqlcgen.GatewayDeviceSession,
) (gateway.StoredDeviceSession, error) {
	device, err := gatewayDeviceSessionFromGenerated(row)
	if err != nil {
		return gateway.StoredDeviceSession{}, err
	}
	return gateway.StoredDeviceSession{Session: device, TokenHash: row.TokenHash}, nil
}

func nullableGatewayTime(value time.Time) sql.NullString {
	if value.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: store.FormatTimestamp(value), Valid: true}
}

func parseNullableGatewayTime(value sql.NullString) (time.Time, error) {
	if !value.Valid {
		return time.Time{}, nil
	}
	return store.ParseTimestamp(value.String)
}

func gatewayUnsignedValue(name string, value int64) (uint64, error) {
	if value < 0 {
		return 0, fmt.Errorf("store: gateway %s must not be negative: %d", name, value)
	}
	return uint64(value), nil
}

func positiveGatewayUnsignedValue(name string, value int64) (uint64, error) {
	if value <= 0 {
		return 0, fmt.Errorf("store: invalid gateway %s", name)
	}
	return uint64(value), nil
}
