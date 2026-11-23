package store

import (
	"context"
	"encoding/json/v2"
	"time"

	"github.com/compozy/compozy/internal/cloud/store/sqlcgen"
	"github.com/compozy/compozy/internal/remoteproto"
)

const (
	KeyPending    = "pending"
	KeyActive     = "active"
	KeyRevoked    = "revoked"
	BrowserDevice = "browser"
)

type DeviceKeyOwner struct {
	AccountID, DeviceID, Kind, Origin, State string
	Admin, DeviceRevoked                     bool
}

func (q *Queries) FindDeviceKey(ctx context.Context, jkt string) (DeviceKeyOwner, error) {
	k, err := q.generated.FindAuthDeviceKey(ctx, jkt)
	return DeviceKeyOwner{
		AccountID: k.AccountID, DeviceID: k.DeviceID, Kind: k.Kind, Origin: k.Origin.String,
		State: k.State, Admin: k.Admin, DeviceRevoked: k.DeviceRevokedAt.Valid,
	}, authLookupError(err)
}

func (q *Queries) DevicePopulation(ctx context.Context, accountID string) (enrolled, admins int64, err error) {
	p, err := q.generated.AuthDevicePopulation(ctx, accountID)
	return p.Enrolled, p.Admins, err
}

type AuthDevice struct {
	ID, AccountID, Name, Kind, InstallationID string
	BrowserBinding                            []byte
	Admin                                     bool
}

func (q *Queries) CreateAuthDevice(ctx context.Context, d AuthDevice, now time.Time) error {
	return q.generated.InsertAuthDevice(ctx, sqlcgen.InsertAuthDeviceParams{
		ID: d.ID, AccountID: d.AccountID, Name: d.Name, Kind: d.Kind,
		InstallationID: authText(d.InstallationID), BrowserBindingSha256: d.BrowserBinding,
		Admin: d.Admin, LastSeenAt: timestamp(now),
	})
}

type AuthDeviceKey struct {
	JKT, DeviceID, Origin, State string
	JWK                          remoteproto.JWK
	PendingUntil                 time.Time
}

func (q *Queries) CreateDeviceKey(ctx context.Context, k AuthDeviceKey, now time.Time) error {
	data, err := json.Marshal(k.JWK)
	if err != nil {
		return err
	}
	var activated time.Time
	if k.State == KeyActive {
		activated = now
	}
	return authChanged(q.generated.InsertAuthDeviceKey(ctx, sqlcgen.InsertAuthDeviceKeyParams{
		Jkt: k.JKT, DeviceID: k.DeviceID, PublicJwk: string(data), Origin: authText(k.Origin), State: k.State,
		PendingExpiresAt: timestamp(k.PendingUntil), CreatedAt: timestamp(now), ActivatedAt: timestamp(activated),
	}))
}

func (q *Queries) PromoteRecoveredAdmin(ctx context.Context, deviceID string) error {
	return q.generated.PromoteRecoveredAdmin(ctx, deviceID)
}

func (q *Queries) RecordAuthDeviceChange(ctx context.Context, accountID string) error {
	_, err := q.AdvanceSecurity(ctx, ControlEvent{
		Type: remoteproto.ControlSnapshot, AccountID: accountID,
	})
	return err
}

func (q *Queries) AdvanceDeviceActivity(ctx context.Context, deviceID string, now time.Time) error {
	return q.generated.AdvanceAuthDeviceActivity(ctx, sqlcgen.AdvanceAuthDeviceActivityParams{
		ID: deviceID, AsOf: timestamp(now),
	})
}

type AuthDeviceSession struct {
	ID, AccountID, DeviceID, JKT, Provider string
	AccountState, KeyState                 string
	RefreshHash                            []byte
	Revoked, DeviceRevoked, Admin          bool
}

func (q *Queries) ReadDeviceSession(ctx context.Context, id, accountID string) (AuthDeviceSession, error) {
	r, err := q.generated.ReadAuthDeviceSession(ctx, sqlcgen.ReadAuthDeviceSessionParams{ID: id, AccountID: accountID})
	return AuthDeviceSession{
		ID: r.ID, AccountID: r.AccountID, DeviceID: r.DeviceID, JKT: r.Jkt, Provider: r.Provider,
		AccountState: r.AccountState, KeyState: r.KeyState, RefreshHash: r.RefreshTokenSha256,
		Revoked: r.RevokedAt.Valid, DeviceRevoked: r.DeviceRevokedAt.Valid, Admin: r.Admin,
	}, authLookupError(err)
}

func (q *Queries) CreateAuthDeviceSession(ctx context.Context, s AuthDeviceSession, now time.Time) error {
	if err := q.RevokeDeviceSessions(ctx, s.DeviceID, now); err != nil {
		return err
	}
	return q.generated.InsertAuthDeviceSession(ctx, sqlcgen.InsertAuthDeviceSessionParams{
		ID: s.ID, AccountID: s.AccountID, DeviceID: s.DeviceID, Jkt: s.JKT,
		RefreshTokenSha256: s.RefreshHash, Provider: s.Provider, CreatedAt: timestamp(now),
	})
}

func (q *Queries) RevokeDeviceSessions(ctx context.Context, deviceID string, now time.Time) error {
	return q.generated.RevokeAuthDeviceSession(ctx, sqlcgen.RevokeAuthDeviceSessionParams{
		DeviceID: deviceID, AsOf: timestamp(now),
	})
}

func (q *Queries) RotateAuthDeviceSession(ctx context.Context, s AuthDeviceSession, next []byte, now time.Time) error {
	return authChanged(q.generated.RotateAuthDeviceSession(ctx, sqlcgen.RotateAuthDeviceSessionParams{
		ID: s.ID, NextSha256: next, AsOf: timestamp(now), PreviousSha256: s.RefreshHash,
	}))
}

func (q *Queries) RememberDeviceProof(ctx context.Context, jkt, jti string, now time.Time) error {
	return authChanged(q.generated.RememberCloudProof(ctx, sqlcgen.RememberCloudProofParams{
		Jkt: jkt, Jti: jti, AsOf: timestamp(now), ExpiresAt: timestamp(now.Add(remoteproto.DPoPReplayWindow)),
	}))
}
