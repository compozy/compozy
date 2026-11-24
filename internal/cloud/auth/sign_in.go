// SPDX-License-Identifier: FSL-1.1-MIT

package auth

import (
	"context"
	"encoding/json/v2"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/compozy/compozy/internal/cloud/observe"
	"github.com/compozy/compozy/internal/cloud/store"
)

const oauthAttemptTTL = 10 * time.Minute

func (s *Service) BeginDeviceOAuth(
	ctx context.Context, grant store.DeviceGrant, provider, cookie string, fresh bool,
) (string, error) {
	if !SupportedProvider(provider) || !ValidSecret(cookie) || (grant.Provider != "" && grant.Provider != provider) {
		return "", ErrInvalidRequest
	}
	if fresh && provider != Google {
		return "", errFreshAuth
	}
	now := s.now()
	if err := grantAvailable(grant, now); err != nil {
		return "", err
	}
	expires := now.Add(oauthAttemptTTL)
	if grant.ExpiresAt.Before(expires) {
		expires = grant.ExpiresAt
	}
	return s.beginOAuth(ctx, store.OAuthAttempt{
		GrantHash: grant.CodeHash, Provider: provider, CookieHash: hashSecret(cookie),
		FreshSignIn: fresh, CreatedAt: now, ExpiresAt: expires,
	})
}

func (s *Service) beginOAuth(ctx context.Context, attempt store.OAuthAttempt) (string, error) {
	state, verifier, nonce := Secret(), Secret(), Secret()
	attempt.StateHash = hashSecret(state)
	attempt.NonceHash = hashSecret(nonce)
	// #nosec G407 -- NewGCMWithRandomNonce generates and prepends the nonce.
	attempt.EncryptedVerifier = s.seal.Seal(nil, nil, []byte(verifier), attempt.StateHash)
	err := s.db.Transaction(ctx, func(q *store.Queries) error { return q.CreateOAuthAttempt(ctx, attempt) })
	if errors.Is(err, store.ErrAuthNotFound) {
		return "", errCodeUsed
	}
	if err != nil {
		return "", err
	}
	return s.oauth.authorize(s.issuer, attempt.Provider, state, verifier, nonce, attempt.FreshSignIn), nil
}

type Callback struct {
	Provider, State, Cookie, Code string
	Denied                        bool
	BrowserBinding                string
	UserAgent                     string
	Location                      string
}

type CallbackResult struct {
	Grant          store.DeviceGrant
	Account        store.AuthAccount
	Browser        BrowserResult
	WebSession     string
	PurchaseReturn string
	NoticeEventID  int64
}

func (s *Service) CompleteOAuth(ctx context.Context, cb Callback) (result CallbackResult, retErr error) {
	if SupportedProvider(cb.Provider) {
		defer func() {
			if retErr != nil {
				s.cfg.Metrics.Count("cloud_sign_ins_total", observe.Labels{
					providerField: cb.Provider, "result": "error", "new_account": "false",
				})
			}
		}()
	}
	if !SupportedProvider(cb.Provider) || !ValidSecret(cb.State) || !ValidSecret(cb.Cookie) ||
		(!cb.Denied && !validLabel(cb.Code, 2048)) {
		return CallbackResult{}, errOAuthState
	}
	attempt, grant, err := s.claimOAuthAttempt(ctx, cb)
	result = CallbackResult{Grant: grant}
	if err != nil {
		return result, err
	}
	var request webOAuthRequest
	if len(attempt.GrantHash) == 0 {
		if err := json.Unmarshal(attempt.BrowserRequest, &request); err != nil {
			return result, errOAuthState
		}
		result.PurchaseReturn, result.NoticeEventID = request.PurchaseReturn, request.NoticeEventID
	}
	if cb.Denied {
		return result, s.failOAuth(ctx, attempt, errDenied)
	}
	verifier, err := s.seal.Open(nil, nil, attempt.EncryptedVerifier, attempt.StateHash)
	if err != nil || !ValidSecret(string(verifier)) {
		return result, errOAuthState
	}
	profile, err := s.oauth.exchange(ctx, s.issuer, attempt, cb.Code, string(verifier), s.now())
	if err != nil {
		return result, s.failOAuth(ctx, attempt, err)
	}
	if len(attempt.GrantHash) == 0 {
		if request.NoticeEventID != 0 {
			result.WebSession, err = s.finishNoticeOAuth(ctx, attempt, request.NoticeEventID, profile)
			return result, err
		}
		return s.finishBrowserOAuth(ctx, attempt, request, profile, cb)
	}
	result.Account, err = s.authorizeDeviceGrant(ctx, attempt, profile)
	if err != nil {
		err = s.failOAuth(ctx, attempt, err)
	}
	return result, err
}

func (s *Service) claimOAuthAttempt(ctx context.Context, cb Callback) (store.OAuthAttempt, store.DeviceGrant, error) {
	var attempt store.OAuthAttempt
	var grant store.DeviceGrant
	var outcome error
	now := s.now()
	err := s.db.Transaction(ctx, func(q *store.Queries) error {
		var err error
		attempt, err = q.ConsumeOAuthAttempt(ctx, hashSecret(cb.State), hashSecret(cb.Cookie), cb.Provider, now)
		if errors.Is(err, store.ErrAuthNotFound) {
			return errOAuthState
		}
		if err != nil || len(attempt.GrantHash) == 0 {
			return err
		}
		grant, err = q.LockDeviceGrant(ctx, attempt.GrantHash)
		if err == nil {
			outcome = grantAvailable(grant, now)
		}
		return err
	})
	if err != nil {
		return attempt, grant, err
	}
	return attempt, grant, outcome
}

func (s *Service) authorizeDeviceGrant(
	ctx context.Context, a store.OAuthAttempt, identity store.VerifiedIdentity,
) (store.AuthAccount, error) {
	return s.signIn(ctx, identity, func(q *store.Queries, now time.Time) (signInCommit, error) {
		grant, err := q.LockDeviceGrant(ctx, a.GrantHash)
		if err != nil {
			return nil, err
		}
		if err := grantAvailable(grant, now); err != nil {
			return nil, err
		}
		return func(account store.AuthAccount, created bool) error {
			grant.AccountID, grant.Provider, grant.NewAccount = account.ID, a.Provider, created
			grant.FreshSignIn = a.FreshSignIn
			return q.AuthorizeGrant(ctx, grant, now)
		}, nil
	})
}

type signInCommit func(store.AuthAccount, bool) error
type prepareSignIn func(*store.Queries, time.Time) (signInCommit, error)

// prepare locks the flow before the account, and returns the write that finalizes it.
func (s *Service) signIn(
	ctx context.Context, identity store.VerifiedIdentity, prepare prepareSignIn,
) (store.AuthAccount, error) {
	var account store.AuthAccount
	for range 4 {
		var created bool
		err := s.db.Transaction(ctx, func(q *store.Queries) error {
			now := s.now()
			commit, err := prepare(q, now)
			if err != nil {
				return err
			}
			account, created, err = q.LockIdentityAccount(ctx, identity, "acc_"+Secret(), now)
			if err != nil {
				return err
			}
			if err := requireActive(account); err != nil {
				return err
			}
			if err := q.SaveIdentity(ctx, account, identity, now); err != nil {
				return err
			}
			account.Email = identity.Email
			if err := commit(account, created); err != nil {
				return err
			}
			return q.AppendSecurityEvent(ctx, store.SecurityEvent{
				AccountID: account.ID, Type: "sign_in", Actor: systemActor, At: now,
				Details: map[string]any{providerField: identity.Provider},
			})
		})
		if errors.Is(err, store.ErrIdentityConflict) {
			return store.AuthAccount{}, errIdentityConflict
		}
		if !retryIdentity(err) {
			if err == nil {
				s.cfg.Metrics.Count("cloud_sign_ins_total", observe.Labels{
					providerField: identity.Provider, "result": "ok", "new_account": strconv.FormatBool(created),
				})
				if created {
					s.cfg.Metrics.Count("cloud_new_accounts_total", observe.Labels{providerField: identity.Provider})
				}
			}
			return account, err
		}
	}
	return store.AuthAccount{}, store.ErrIdentityRace
}

func retryIdentity(err error) bool {
	if errors.Is(err, store.ErrIdentityRace) {
		return true
	}
	dbError, ok := errors.AsType[*pgconn.PgError](err)
	return ok && (dbError.Code == "40P01" || dbError.Code == "40001")
}

func (s *Service) failOAuth(ctx context.Context, attempt store.OAuthAttempt, cause error) error {
	f, ok := errors.AsType[*Failure](cause)
	if !ok || f == errCodeUsed || f == errExpired {
		return cause
	}
	err := s.db.Transaction(ctx, func(q *store.Queries) error {
		now := s.now()
		switch f {
		case errDenied, errAccountSuspended, errAccountDeleting, errIdentityConflict:
			if len(attempt.GrantHash) > 0 {
				if err := q.CancelGrant(ctx, attempt.GrantHash, f.Code, false, now); err != nil {
					return err
				}
			}
		}
		return q.AppendSecurityEvent(ctx, store.SecurityEvent{
			Type: "sign_in_failed", Actor: systemActor, At: now,
			Details: map[string]any{providerField: attempt.Provider, "reason": f.Code},
		})
	})
	if err != nil {
		return err
	}
	return cause
}
