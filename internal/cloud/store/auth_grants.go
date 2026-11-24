// SPDX-License-Identifier: FSL-1.1-MIT

package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/compozy/compozy/internal/cloud/store/sqlcgen"
)

var ErrAuthNotFound = errors.New("cloud auth record not found")

func authLookupError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAuthNotFound
	}
	return err
}

func authChanged(rows int64, err error) error {
	if err == nil && rows == 0 {
		return ErrAuthNotFound
	}
	return err
}

func authText(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}

type DeviceGrant struct {
	CodeHash       []byte
	UserCode       string
	Provider       string
	ClientName     string
	ClientPlatform string
	InstallationID string
	Location       string
	State          string
	AccountID      string
	FailureCode    string
	ExpiresAt      time.Time
	LastPoll       time.Time
	Interval       int32
	NewAccount     bool
	FreshSignIn    bool
}

func (q *Queries) CreateDeviceGrant(ctx context.Context, g DeviceGrant) error {
	return authChanged(q.generated.InsertDeviceGrant(ctx, sqlcgen.InsertDeviceGrantParams{
		DeviceCodeSha256: g.CodeHash, UserCode: g.UserCode, Provider: authText(g.Provider),
		ClientName: g.ClientName, ClientPlatform: g.ClientPlatform, InstallationID: g.InstallationID,
		RequestLocation: authText(g.Location), ExpiresAt: timestamp(g.ExpiresAt),
	}))
}

func (q *Queries) FindDeviceGrant(ctx context.Context, code string) (DeviceGrant, error) {
	g, err := q.generated.FindDeviceGrantByCode(ctx, code)
	return mapDeviceGrant(g), authLookupError(err)
}

func (q *Queries) LockDeviceGrant(ctx context.Context, hash []byte) (DeviceGrant, error) {
	g, err := q.generated.LockDeviceGrant(ctx, hash)
	return mapDeviceGrant(g), authLookupError(err)
}

func mapDeviceGrant(g sqlcgen.DeviceGrant) DeviceGrant {
	return DeviceGrant{
		CodeHash: g.DeviceCodeSha256, UserCode: g.UserCode, Provider: g.Provider.String,
		ClientName: g.ClientName, ClientPlatform: g.ClientPlatform, InstallationID: g.InstallationID,
		Location: g.RequestLocation.String, State: g.State, AccountID: g.AccountID.String,
		FailureCode: g.FailureCode.String, ExpiresAt: g.ExpiresAt.Time, LastPoll: g.LastPollAt.Time,
		Interval: g.PollIntervalSeconds, NewAccount: g.NewAccount, FreshSignIn: g.MaxAgeZero,
	}
}

type OAuthAttempt struct {
	StateHash, GrantHash, BrowserRequest, CookieHash, EncryptedVerifier, NonceHash []byte
	Provider                                                                       string
	FreshSignIn                                                                    bool
	CreatedAt, ExpiresAt                                                           time.Time
}

func (q *Queries) CreateOAuthAttempt(ctx context.Context, a OAuthAttempt) error {
	if len(a.GrantHash) > 0 {
		err := authChanged(q.generated.BindDeviceGrantOAuth(ctx, sqlcgen.BindDeviceGrantOAuthParams{
			DeviceCodeSha256: a.GrantHash, OauthStateSha256: a.StateHash, PkceVerifierEnc: a.EncryptedVerifier,
			ActivationNonceSha256: a.CookieHash, AsOf: timestamp(a.CreatedAt),
		}))
		if err != nil {
			return err
		}
	}
	return q.generated.InsertCloudOAuthAttempt(ctx, sqlcgen.InsertCloudOAuthAttemptParams{
		StateSha256:      a.StateHash,
		DeviceCodeSha256: a.GrantHash,
		BrowserRequest:   authText(string(a.BrowserRequest)),
		Provider:         a.Provider,
		CookieSha256:     a.CookieHash,
		PkceVerifierEnc:  a.EncryptedVerifier,
		NonceSha256:      a.NonceHash,
		MaxAgeZero:       a.FreshSignIn,
		CreatedAt:        timestamp(a.CreatedAt),
		ExpiresAt:        timestamp(a.ExpiresAt),
	})
}

func (q *Queries) ConsumeOAuthAttempt(
	ctx context.Context, state, cookie []byte, provider string, now time.Time,
) (OAuthAttempt, error) {
	a, err := q.generated.ConsumeCloudOAuthAttempt(ctx, sqlcgen.ConsumeCloudOAuthAttemptParams{
		StateSha256: state, CookieSha256: cookie, Provider: provider, AsOf: timestamp(now),
	})
	return OAuthAttempt{
		StateHash: a.StateSha256, GrantHash: a.DeviceCodeSha256, BrowserRequest: a.BrowserRequest,
		CookieHash: a.CookieSha256, EncryptedVerifier: a.PkceVerifierEnc, NonceHash: a.NonceSha256,
		Provider: a.Provider, FreshSignIn: a.MaxAgeZero, CreatedAt: a.CreatedAt.Time, ExpiresAt: a.ExpiresAt.Time,
	}, authLookupError(err)
}

func (q *Queries) AuthorizeGrant(ctx context.Context, g DeviceGrant, now time.Time) error {
	return authChanged(q.generated.AuthorizeDeviceGrant(ctx, sqlcgen.AuthorizeDeviceGrantParams{
		DeviceCodeSha256: g.CodeHash, AccountID: authText(g.AccountID), Provider: authText(g.Provider),
		NewAccount: g.NewAccount, MaxAgeZero: g.FreshSignIn, AsOf: timestamp(now),
	}))
}

func (q *Queries) CancelGrant(
	ctx context.Context,
	codeHash []byte,
	reason string,
	allowAuthorized bool,
	now time.Time,
) error {
	return q.generated.CancelDeviceGrant(ctx, sqlcgen.CancelDeviceGrantParams{
		DeviceCodeSha256: codeHash,
		FailureCode:      authText(reason),
		AllowAuthorized:  allowAuthorized,
		AsOf:             timestamp(now),
	})
}

func (q *Queries) ExpireGrant(ctx context.Context, codeHash []byte) error {
	return q.generated.ExpireDeviceGrant(ctx, codeHash)
}

func (q *Queries) RecordGrantPoll(ctx context.Context, codeHash []byte, interval int32, now time.Time) error {
	return q.generated.RecordDeviceGrantPoll(ctx, sqlcgen.RecordDeviceGrantPollParams{
		DeviceCodeSha256: codeHash, IntervalSeconds: interval, AsOf: timestamp(now),
	})
}

func (q *Queries) RedeemGrant(ctx context.Context, g DeviceGrant, tokenHash []byte, now, expires time.Time) error {
	if err := authChanged(q.generated.RedeemDeviceGrant(ctx, sqlcgen.RedeemDeviceGrantParams{
		DeviceCodeSha256: g.CodeHash, AsOf: timestamp(now),
	})); err != nil {
		return err
	}
	return q.generated.InsertEnrollmentToken(ctx, sqlcgen.InsertEnrollmentTokenParams{
		TokenSha256: tokenHash, AccountID: g.AccountID, DeviceCodeSha256: g.CodeHash,
		Provider: g.Provider, MaxAgeZero: g.FreshSignIn, ExpiresAt: timestamp(expires),
	})
}

type EnrollmentToken struct {
	AccountID, Provider, InstallationID, Location string
	FreshSignIn                                   bool
}

func (q *Queries) FindEnrollment(ctx context.Context, hash []byte, now time.Time) (EnrollmentToken, error) {
	row, err := q.generated.FindEnrollmentToken(ctx, sqlcgen.FindEnrollmentTokenParams{
		TokenSha256: hash, AsOf: timestamp(now),
	})
	return EnrollmentToken{
		AccountID: row.AccountID, Provider: row.Provider, InstallationID: row.InstallationID,
		Location: row.RequestLocation.String, FreshSignIn: row.MaxAgeZero,
	}, authLookupError(err)
}

func (q *Queries) ConsumeEnrollment(ctx context.Context, hash []byte, now time.Time) error {
	return authChanged(q.generated.ConsumeEnrollmentToken(ctx, sqlcgen.ConsumeEnrollmentTokenParams{
		TokenSha256: hash, AsOf: timestamp(now),
	}))
}
