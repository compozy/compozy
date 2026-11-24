package store

import (
	"cmp"
	"context"
	"encoding/json/v2"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/cloud/store/sqlcgen"
	"github.com/compozy/compozy/internal/remoteproto"
)

const (
	EventDeviceEnrolled   = "device_enrolled"
	EventAccountSuspended = "account_suspended"
)

func (q *Queries) UpdateMarketingConsent(ctx context.Context, account AuthAccount, optIn bool, source string, now time.Time) error {
	if err := q.generated.TrackMarketingAddress(ctx, sqlcgen.TrackMarketingAddressParams{
		AccountID: account.ID, Email: account.Email,
	}); err != nil {
		return err
	}
	return q.generated.UpdateMarketingConsent(ctx, sqlcgen.UpdateMarketingConsentParams{
		AccountID: account.ID, OptIn: optIn, Source: authText(source), AsOf: timestamp(now),
	})
}

func (q *Queries) DirtyMarketingAccounts(ctx context.Context, now time.Time) ([]string, error) {
	return q.generated.DirtyMarketingAccounts(ctx, timestamp(now))
}

func (q *Queries) ClaimMarketingAccount(ctx context.Context, id string, now time.Time) (AuthAccount, error) {
	account, err := q.generated.ClaimMarketingAccount(ctx, sqlcgen.ClaimMarketingAccountParams{
		AccountID: id, AsOf: timestamp(now),
	})
	return mapAuthAccount(account), authLookupError(err)
}

func (q *Queries) MarketingAddresses(ctx context.Context, id string) ([]string, error) {
	return q.generated.MarketingAddresses(ctx, id)
}

func (q *Queries) ForgetMarketingAddress(ctx context.Context, id, address string) error {
	return q.generated.ForgetMarketingAddress(ctx, sqlcgen.ForgetMarketingAddressParams{AccountID: id, Email: address})
}

func (q *Queries) RetryMarketingAccount(ctx context.Context, id string, retry time.Time) error {
	return q.generated.RetryMarketingAccount(ctx, sqlcgen.RetryMarketingAccountParams{
		AccountID: id, RetryAt: timestamp(retry),
	})
}

func (q *Queries) RecordMarketingSync(ctx context.Context, id string, now time.Time) error {
	return q.generated.RecordMarketingSync(ctx, sqlcgen.RecordMarketingSyncParams{AccountID: id, AsOf: timestamp(now)})
}

func (q *Queries) QueueSecurityEmail(ctx context.Context, event SecurityEvent) error {
	id, err := q.appendSecurityEvent(ctx, event)
	if err != nil {
		return err
	}
	return q.generated.QueueAccountEmail(ctx, sqlcgen.QueueAccountEmailParams{EventID: id, RetryAt: timestamp(event.At)})
}

func (q *Queries) RecordDeviceEnrollment(
	ctx context.Context, accountID string, id remoteproto.DeviceID, name, kind, location string, admin bool, at time.Time,
) error {
	if _, err := netip.ParseAddr(strings.TrimSpace(name)); err == nil {
		name = ""
	}
	return q.QueueSecurityEmail(ctx, SecurityEvent{
		Type: EventDeviceEnrolled, AccountID: accountID, Actor: "account:" + string(id), At: at,
		Details: map[string]any{"device_id": id, "device_name": name, "kind": kind, "location": location, "admin": admin},
	})
}

type AccountEmail struct {
	AccountID, Type string
	Notice          remoteproto.AccountSignInNotice
}

type AccountEmailEntry struct {
	ID        int64
	AccountID string
}

func (q *Queries) DueAccountEmails(ctx context.Context, now time.Time) ([]AccountEmailEntry, error) {
	rows, err := q.generated.DueAccountEmails(ctx, timestamp(now))
	if err != nil {
		return nil, err
	}
	entries := make([]AccountEmailEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, AccountEmailEntry{ID: row.EventID, AccountID: row.AccountID.String})
	}
	return entries, nil
}

func (q *Queries) LockAccountEmail(ctx context.Context, id int64, now time.Time) (AccountEmail, error) {
	row, err := q.generated.LockAccountEmail(ctx, sqlcgen.LockAccountEmailParams{EventID: id, AsOf: timestamp(now)})
	if err != nil {
		return AccountEmail{}, authLookupError(err)
	}
	result := AccountEmail{AccountID: row.AccountID.String, Type: row.Type}
	if row.Type == EventDeviceEnrolled {
		result.Notice, err = readSignInEvent(row.DetailsJson, row.At.Time)
	}
	return result, err
}

func (q *Queries) RetryAccountEmail(ctx context.Context, id int64, now, retry time.Time) error {
	return q.generated.RetryAccountEmail(ctx, sqlcgen.RetryAccountEmailParams{
		EventID: id, AsOf: timestamp(now), RetryAt: timestamp(retry),
	})
}

func (q *Queries) RemoveAccountEmail(ctx context.Context, id int64) error {
	return q.generated.RemoveAccountEmail(ctx, id)
}

func (q *Queries) RecentSignIns(ctx context.Context, id string, now time.Time) ([]remoteproto.AccountSignInNotice, error) {
	rows, err := q.generated.RecentSignInEvents(ctx, sqlcgen.RecentSignInEventsParams{
		AccountID: authText(id), Since: timestamp(now.Add(-remoteproto.SignInNoticeTTL)),
	})
	if err != nil {
		return nil, err
	}
	notices := make([]remoteproto.AccountSignInNotice, 0, len(rows))
	for _, row := range rows {
		notice, err := readSignInEvent(row.DetailsJson, row.At.Time)
		if err != nil {
			return nil, err
		}
		notices = append(notices, notice)
	}
	slices.SortFunc(notices, func(a, b remoteproto.AccountSignInNotice) int {
		return cmp.Or(b.At.Compare(a.At), cmp.Compare(a.DeviceID, b.DeviceID))
	})
	return notices, nil
}

func (q *Queries) AccountSignInNotice(ctx context.Context, accountID string, eventID int64) (remoteproto.AccountSignInNotice, error) {
	row, err := q.generated.AccountSignInEvent(ctx, sqlcgen.AccountSignInEventParams{
		ID: eventID, AccountID: authText(accountID),
	})
	if err != nil {
		return remoteproto.AccountSignInNotice{}, authLookupError(err)
	}
	return readSignInEvent(row.DetailsJson, row.At.Time)
}

func (q *Queries) HasActivatedBrowserKey(ctx context.Context, deviceID string) (bool, error) {
	return q.generated.HasActivatedBrowserKey(ctx, deviceID)
}

func readSignInEvent(details []byte, at time.Time) (remoteproto.AccountSignInNotice, error) {
	var payload struct {
		DeviceID remoteproto.DeviceID `json:"device_id"`
		Name     string               `json:"device_name"`
		Kind     string               `json:"kind"`
		Location string               `json:"location"`
	}
	if err := json.Unmarshal(details, &payload); err != nil {
		return remoteproto.AccountSignInNotice{}, fmt.Errorf("decode sign-in notice: %w", err)
	}
	return remoteproto.AccountSignInNotice{
		DeviceID: payload.DeviceID, Name: cmp.Or(payload.Name, "Device"), Kind: payload.Kind, Location: payload.Location, At: at,
	}, nil
}
