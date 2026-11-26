// SPDX-License-Identifier: FSL-1.1-MIT

package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/compozy/compozy/internal/cloud/store/sqlcgen"
)

type CloudWebSession struct {
	AccountID, Email, BrowserDeviceID, State string
	FreshNoticeEventID                       int64
}

func (q *Queries) CreateWebSession(ctx context.Context, hash []byte, accountID string, now, expires time.Time) error {
	return q.generated.InsertCloudWebSession(ctx, sqlcgen.InsertCloudWebSessionParams{
		IDSha256: hash, AccountID: accountID, CreatedAt: timestamp(now), ExpiresAt: timestamp(expires),
	})
}

func (q *Queries) CreateNoticeWebSession(
	ctx context.Context, hash []byte, accountID string, eventID int64, now, expires time.Time,
) error {
	return q.generated.InsertNoticeWebSession(ctx, sqlcgen.InsertNoticeWebSessionParams{
		IDSha256: hash, AccountID: accountID, CreatedAt: timestamp(now), ExpiresAt: timestamp(expires),
		FreshNoticeEventID: pgtype.Int8{Int64: eventID, Valid: true},
	})
}

func (q *Queries) TouchWebSession(ctx context.Context, hash []byte, now, idleLimit time.Time) (CloudWebSession, error) {
	row, err := q.generated.TouchCloudWebSession(ctx, sqlcgen.TouchCloudWebSessionParams{
		IDSha256: hash, AsOf: timestamp(now), IdleLimit: timestamp(idleLimit),
	})
	return CloudWebSession{
		AccountID: row.AccountID, Email: row.Email, BrowserDeviceID: row.BrowserDeviceID.String,
		State: row.State, FreshNoticeEventID: row.FreshNoticeEventID.Int64,
	}, authLookupError(err)
}

func (q *Queries) RevokePurchaseWebSession(ctx context.Context, hash []byte, now time.Time) error {
	return q.generated.RevokePurchaseWebSession(ctx, sqlcgen.RevokePurchaseWebSessionParams{
		IDSha256: hash, RevokedAt: timestamp(now),
	})
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
	Hash                                                      []byte
	AccountID, DeviceID, ComputerID, JKT, Challenge, Location string
	ExpiresAt, UsedAt                                         time.Time
}

func (q *Queries) CreateBrowserCode(ctx context.Context, c BrowserCode) error {
	return q.generated.InsertCloudBrowserCode(ctx, sqlcgen.InsertCloudBrowserCodeParams{
		CodeSha256: c.Hash, AccountID: c.AccountID, DeviceID: c.DeviceID, ComputerID: c.ComputerID,
		Jkt: c.JKT, CodeChallenge: c.Challenge,
		ExpiresAt: timestamp(c.ExpiresAt), RequestLocation: authText(c.Location),
	})
}

func (q *Queries) BrowserCode(ctx context.Context, hash []byte) (BrowserCode, error) {
	row, err := q.generated.CloudBrowserCode(ctx, hash)
	return BrowserCode{
		Hash: hash, AccountID: row.AccountID, DeviceID: row.DeviceID, ComputerID: row.ComputerID,
		JKT: row.Jkt, Challenge: row.CodeChallenge, Location: row.RequestLocation.String,
		ExpiresAt: row.ExpiresAt.Time, UsedAt: row.UsedAt.Time,
	}, authLookupError(err)
}

func (q *Queries) UseBrowserCode(ctx context.Context, hash []byte, now time.Time) error {
	return authChanged(q.generated.UseCloudBrowserCode(ctx, sqlcgen.UseCloudBrowserCodeParams{
		CodeSha256: hash, AsOf: timestamp(now),
	}))
}

func (q *Queries) ActivateBrowserKey(ctx context.Context, id, jkt, origin string, now time.Time) error {
	if err := q.generated.RetireBrowserOriginKey(ctx, sqlcgen.RetireBrowserOriginKeyParams{
		DeviceID: id, Origin: authText(origin), AsOf: timestamp(now),
	}); err != nil {
		return err
	}
	return authChanged(q.generated.ActivateBrowserKey(ctx, sqlcgen.ActivateBrowserKeyParams{
		Jkt: jkt, DeviceID: id, AsOf: timestamp(now),
	}))
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
	const replayReceiptTTL = 24 * time.Hour
	if err := q.generated.SweepBrowserCodes(ctx, timestamp(now.Add(-replayReceiptTTL))); err != nil {
		return err
	}
	const abandonedDeviceTTL = 10 * time.Minute
	return q.generated.SweepUnactivatedBrowserDevices(ctx, timestamp(now.Add(-abandonedDeviceTTL)))
}
