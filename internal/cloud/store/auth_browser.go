package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/compozy/compozy/internal/cloud/store/sqlcgen"
)

type CloudWebSession struct {
	AccountID, Email, BrowserDeviceID string
}

func (q *Queries) CreateWebSession(ctx context.Context, hash []byte, accountID string, now, expires time.Time) error {
	return q.generated.InsertCloudWebSession(ctx, sqlcgen.InsertCloudWebSessionParams{
		IDSha256: hash, AccountID: accountID, CreatedAt: timestamp(now), ExpiresAt: timestamp(expires),
	})
}

func (q *Queries) TouchWebSession(ctx context.Context, hash []byte, now, idleLimit time.Time) (CloudWebSession, error) {
	row, err := q.generated.TouchCloudWebSession(ctx, sqlcgen.TouchCloudWebSessionParams{
		IDSha256: hash, AsOf: timestamp(now), IdleLimit: timestamp(idleLimit),
	})
	return CloudWebSession{
		AccountID: row.AccountID, Email: row.Email, BrowserDeviceID: row.BrowserDeviceID.String,
	}, authLookupError(err)
}

func (q *Queries) BindWebSession(ctx context.Context, hash []byte, accountID, deviceID string) error {
	return authChanged(q.generated.BindCloudWebSession(ctx, sqlcgen.BindCloudWebSessionParams{
		IDSha256: hash, AccountID: accountID, BrowserDeviceID: authText(deviceID),
	}))
}

func (q *Queries) FindBrowserComputer(ctx context.Context, hostname string) (computerID, accountID string, err error) {
	row, err := q.generated.FindBrowserComputer(ctx, hostname)
	return row.ID, row.AccountID, authLookupError(err)
}

func (q *Queries) FindBrowserDevice(
	ctx context.Context,
	binding []byte,
	accountID string,
) (deviceID string, revoked bool, err error) {
	d, err := q.generated.FindBoundBrowserDevice(ctx, sqlcgen.FindBoundBrowserDeviceParams{
		BrowserBindingSha256: binding, AccountID: accountID,
	})
	return d.ID, d.RevokedAt.Valid, authLookupError(err)
}

func (q *Queries) ExtendPendingKey(ctx context.Context, jkt string, expires time.Time) error {
	return authChanged(q.generated.ExtendPendingBrowserKey(ctx, sqlcgen.ExtendPendingBrowserKeyParams{
		Jkt: jkt, PendingExpiresAt: timestamp(expires),
	}))
}

func (q *Queries) ExpirePendingKey(ctx context.Context, deviceID, hostname string, now time.Time) error {
	return q.generated.ExpirePendingBrowserKey(ctx, sqlcgen.ExpirePendingBrowserKeyParams{
		DeviceID: deviceID, Origin: authText(hostname), AsOf: timestamp(now),
	})
}

type BrowserCode struct {
	Hash                                            []byte
	AccountID, DeviceID, ComputerID, JKT, Challenge string
	ExpiresAt                                       time.Time
}

func (q *Queries) CreateBrowserCode(ctx context.Context, c BrowserCode) error {
	return q.generated.InsertCloudBrowserCode(ctx, sqlcgen.InsertCloudBrowserCodeParams{
		CodeSha256: c.Hash, AccountID: c.AccountID, DeviceID: c.DeviceID, ComputerID: c.ComputerID,
		Jkt: c.JKT, CodeChallenge: c.Challenge, ExpiresAt: timestamp(c.ExpiresAt),
	})
}

func (q *Queries) SweepAuth(ctx context.Context, now time.Time) error {
	for _, sweep := range []func(context.Context, pgtype.Timestamptz) error{
		q.generated.SweepAuthAttempts, q.generated.SweepCloudProofs, q.generated.SweepExpiredDeviceGrants,
		q.generated.SweepPendingBrowserKeys,
	} {
		if err := sweep(ctx, timestamp(now)); err != nil {
			return err
		}
	}
	const abandonedDeviceTTL = 10 * time.Minute
	return q.generated.SweepUnactivatedBrowserDevices(ctx, timestamp(now.Add(-abandonedDeviceTTL)))
}
