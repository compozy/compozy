// SPDX-License-Identifier: FSL-1.1-MIT

package store

import (
	"context"
	"encoding/json/jsontext"
	"time"

	"github.com/compozy/compozy/internal/cloud/store/sqlcgen"
	"github.com/compozy/compozy/internal/remoteproto"
)

type AdminAccount struct {
	ID, Email, Name, State string
	CloudRoutesMax         int32
}

func (q *Queries) SearchAdminAccounts(ctx context.Context, term string) ([]AdminAccount, error) {
	rows, err := q.generated.SearchAdminAccounts(ctx, term)
	if err != nil {
		return nil, err
	}
	accounts := make([]AdminAccount, 0, len(rows))
	for _, row := range rows {
		accounts = append(accounts, AdminAccount{ID: row.ID, Email: row.Email, Name: row.Name, State: row.State})
	}
	return accounts, nil
}

func (q *Queries) AdminAccount(ctx context.Context, id string) (AdminAccount, error) {
	row, err := q.generated.GetAdminAccount(ctx, id)
	return AdminAccount(row), err
}

func (q *Queries) AdminAccountData(ctx context.Context, id string, eventLimit int32) ([]byte, error) {
	return q.generated.AdminAccountData(ctx, sqlcgen.AdminAccountDataParams{AccountID: id, EventLimit: eventLimit})
}

type AdminTunnel struct {
	AccountID, Email, Hostname, State string
	TunnelRef
}

func (q *Queries) AdminTunnel(ctx context.Context, hostname string) (AdminTunnel, error) {
	row, err := q.generated.AdminTunnel(ctx, hostname)
	return AdminTunnel{
		AccountID: row.AccountID, Email: row.Email, Hostname: row.Hostname, State: row.State,
		TunnelRef: TunnelRef{ComputerID: remoteproto.ComputerID(row.ComputerID), Generation: row.Generation},
	}, err
}

func (q *Queries) TakeDownAdminTunnel(ctx context.Context, ref TunnelRef, now time.Time) error {
	return q.generated.TakeDownAdminTunnel(ctx, sqlcgen.TakeDownAdminTunnelParams{
		ComputerID: string(ref.ComputerID), Generation: ref.Generation, At: timestamp(now),
	})
}

func (q *Queries) SetAdminHostedRouteCap(ctx context.Context, id string, cap int32) (bool, error) {
	n, err := q.generated.SetAdminHostedRouteCap(ctx, sqlcgen.SetAdminHostedRouteCapParams{AccountID: id, Cap: cap})
	return n != 0, err
}

func (q *Queries) RevokeAdminDeviceSession(
	ctx context.Context,
	accountID, sessionID string,
	now time.Time,
) (bool, error) {
	n, err := q.generated.RevokeAdminDeviceSession(ctx, sqlcgen.RevokeAdminDeviceSessionParams{
		AccountID: accountID, SessionID: sessionID, At: timestamp(now),
	})
	return n != 0, err
}

func (q *Queries) DetachAdminIdentity(ctx context.Context, accountID, provider, subject string) (bool, error) {
	n, err := q.generated.DetachAdminIdentity(ctx, sqlcgen.DetachAdminIdentityParams{
		AccountID: accountID, Provider: provider, Subject: subject,
	})
	return n != 0, err
}

func (q *Queries) InsertComplimentaryGrant(ctx context.Context, id, accountID, actor string, now, end time.Time) error {
	return q.generated.InsertComplimentaryGrant(ctx, sqlcgen.InsertComplimentaryGrantParams{
		ID: id, AccountID: accountID, Actor: dbText(actor), AsOf: timestamp(now), EndsAt: timestamp(end),
	})
}

type AdminContact struct {
	ID, Email, Name, Source string
	ConsentedAt             time.Time
}

func (q *Queries) AdminOptedInContacts(ctx context.Context) ([]AdminContact, error) {
	rows, err := q.generated.AdminOptedInContacts(ctx)
	if err != nil {
		return nil, err
	}
	contacts := make([]AdminContact, 0, len(rows))
	for _, row := range rows {
		contacts = append(contacts, AdminContact{
			ID: row.ID, Email: row.Email, Name: row.Name, Source: row.MarketingConsentSource.String,
			ConsentedAt: row.MarketingConsentAt.Time,
		})
	}
	return contacts, nil
}

type AdminEvent struct {
	ID        int64          `json:"id"`
	AccountID string         `json:"account_id"`
	Type      string         `json:"type"`
	Actor     string         `json:"actor"`
	At        time.Time      `json:"at"`
	Details   jsontext.Value `json:"details"`
}

func (q *Queries) AdminSecurityEvent(ctx context.Context, id int64) (AdminEvent, error) {
	row, err := q.generated.AdminSecurityEvent(ctx, id)
	return AdminEvent{
		ID: row.ID, AccountID: row.AccountID.String, Type: row.Type, Actor: row.Actor,
		At: row.At.Time, Details: row.DetailsJson,
	}, err
}
