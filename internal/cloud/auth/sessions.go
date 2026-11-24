// SPDX-License-Identifier: FSL-1.1-MIT

package auth

import (
	"context"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

const (
	refreshAAD    = "compozy-refresh-v1"
	refreshPrefix = "rfr_"
)

type DeviceProof struct {
	Token, Proof, Method, Path string
}

func RequestDeviceProof(r *http.Request) DeviceProof {
	scheme, token, _ := strings.Cut(r.Header.Get("Authorization"), " ")
	if !strings.EqualFold(scheme, "DPoP") && !strings.EqualFold(scheme, "Bearer") {
		token = ""
	}
	return DeviceProof{
		Token: token, Proof: r.Header.Get("DPoP"), Method: r.Method, Path: r.URL.EscapedPath(),
	}
}

type DevicePrincipal struct {
	AccountID string
	DeviceID  remoteproto.DeviceID
	JKT       string
	Admin     bool
	SessionID string `json:"-"`
}

func (s *Service) AuthenticateDevice(r *http.Request) (DevicePrincipal, error) {
	credentials := RequestDeviceProof(r)
	if strings.HasPrefix(credentials.Token, "cpzat_") {
		return DevicePrincipal{}, errAccountTokenLocalOnly
	}
	if credentials.Token == "" || len(credentials.Token) > 4096 {
		return DevicePrincipal{}, errSessionInvalid
	}
	ctx, now := r.Context(), s.now()
	claims, err := s.access.VerifyDeviceAccess(ctx, credentials.Token, now)
	if err != nil {
		return DevicePrincipal{}, errSessionInvalid
	}
	session, err := s.db.ReadDeviceSession(ctx, claims.SessionID, claims.AccountID)
	if errors.Is(err, store.ErrAuthNotFound) {
		return DevicePrincipal{}, errSessionInvalid
	}
	if err != nil {
		return DevicePrincipal{}, err
	}
	if session.DeviceID != string(claims.DeviceID) || session.JKT != claims.Cnf.JKT {
		return DevicePrincipal{}, errSessionInvalid
	}
	if err := s.verifyDeviceProof(ctx, credentials, session.JKT, credentials.Token, now); err != nil {
		return DevicePrincipal{}, err
	}
	if err := requireActive(store.AuthAccount{State: session.AccountState}); err != nil {
		return DevicePrincipal{}, err
	}
	if session.DeviceRevoked || session.KeyState == store.KeyRevoked {
		return DevicePrincipal{}, errDeviceRevoked
	}
	if err := validSession(session); err != nil {
		return DevicePrincipal{}, err
	}
	return DevicePrincipal{
		AccountID: session.AccountID,
		DeviceID:  claims.DeviceID,
		JKT:       session.JKT,
		Admin:     session.Admin,
		SessionID: session.ID,
	}, nil
}

func (s *Service) verifyDeviceProof(
	ctx context.Context, credentials DeviceProof, jkt, access string, now time.Time,
) error {
	raw, err := s.proofs.VerifyDPoP(credentials.Proof, remoteproto.DPoPBinding{
		Method: credentials.Method, URL: s.issuer + credentials.Path, JKT: jkt, Credential: access,
	}, now)
	if errors.Is(err, remoteproto.ErrDPoPReplayed) {
		return errReplayedProof
	}
	if err != nil {
		return errInvalidProof
	}
	err = s.db.RememberDeviceProof(ctx, jkt, raw.JTI, now)
	if errors.Is(err, store.ErrAuthNotFound) {
		return errReplayedProof
	}
	return err
}

type refreshSession struct {
	SessionID string `json:"sid"`
	AccountID string `json:"account_id"`
	JKT       string `json:"jkt"`
}

func (s *Service) issueTokens(d store.AuthDeviceSession, now time.Time) (remoteproto.DeviceSessionTokens, error) {
	var tokens remoteproto.DeviceSessionTokens
	var err error
	tokens.AccessToken, err = s.signer.SignDeviceAccess(remoteproto.DeviceAccessClaims{
		Issuer: s.issuer, AccountID: d.AccountID, SessionID: d.ID, DeviceID: remoteproto.DeviceID(d.DeviceID),
		Cnf: remoteproto.Confirmation{JKT: d.JKT}, IssuedAt: now.Unix(), JTI: Secret(),
		ExpiresAt: now.Add(remoteproto.DeviceAccessTTL).Unix(),
	})
	if err != nil {
		return tokens, err
	}
	claims, err := json.Marshal(refreshSession{SessionID: d.ID, AccountID: d.AccountID, JKT: d.JKT})
	if err != nil {
		return tokens, err
	}
	// The sealed account and key locator still authenticates after the session's account is deleted.
	// #nosec G407 -- NewGCMWithRandomNonce generates and prepends the nonce.
	sealed := s.seal.Seal(nil, nil, claims, []byte(refreshAAD))
	tokens.RefreshToken = refreshPrefix + base64.RawURLEncoding.EncodeToString(sealed)
	return tokens, nil
}

func (s *Service) parseRefresh(raw string) (refreshSession, error) {
	var claims refreshSession
	value, ok := strings.CutPrefix(raw, refreshPrefix)
	if !ok || len(value) > 2048 || strings.ContainsAny(value, "\r\n") {
		return claims, errSessionInvalid
	}
	sealed, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil {
		return claims, errSessionInvalid
	}
	data, err := s.seal.Open(nil, nil, sealed, []byte(refreshAAD))
	if err != nil || json.Unmarshal(data, &claims) != nil || claims.AccountID == "" || claims.SessionID == "" ||
		!ValidSecret(claims.JKT) {
		return refreshSession{}, errSessionInvalid
	}
	return claims, nil
}

func validCredential(token, prefix string) bool {
	secret, ok := strings.CutPrefix(token, prefix)
	return ok && ValidSecret(secret)
}

func (s *Service) Refresh(ctx context.Context, proof DeviceProof) (remoteproto.DeviceSessionTokens, error) {
	var tokens remoteproto.DeviceSessionTokens
	err := s.withRefresh(ctx, proof, func(q *store.Queries, d store.AuthDeviceSession, now time.Time) error {
		var err error
		tokens, err = s.issueTokens(d, now)
		if err != nil {
			return err
		}
		if err := q.RotateAuthDeviceSession(ctx, d, hashSecret(tokens.RefreshToken), now); err != nil {
			return err
		}
		if err := touchDeviceActivity(ctx, q, d, now); err != nil {
			return err
		}
		return sessionEvent(ctx, q, d, "device_session_refreshed", now)
	})
	return tokens, err
}

func (s *Service) Logout(ctx context.Context, proof DeviceProof) error {
	err := s.withRefresh(ctx, proof, func(q *store.Queries, d store.AuthDeviceSession, now time.Time) error {
		if err := q.RevokeDeviceSessions(ctx, d.DeviceID, now); err != nil {
			return err
		}
		return sessionEvent(ctx, q, d, "device_session_signed_out", now)
	})
	if f, ok := errors.AsType[*Failure](err); ok && f.Reason == errAccountSuspended.Code {
		return errAccountSuspended
	}
	return err
}

type sessionWrite func(*store.Queries, store.AuthDeviceSession, time.Time) error

func (s *Service) withRefresh(ctx context.Context, proof DeviceProof, write sessionWrite) error {
	claims, err := s.parseRefresh(proof.Token)
	if err != nil {
		return err
	}
	if err := s.verifyDeviceProof(ctx, proof, claims.JKT, "", s.now()); err != nil {
		return err
	}
	return s.db.Transaction(ctx, func(q *store.Queries) error {
		if err := lockRefreshAccount(ctx, q, claims.AccountID); err != nil {
			return err
		}
		d, err := q.ReadDeviceSession(ctx, claims.SessionID, claims.AccountID)
		if errors.Is(err, store.ErrAuthNotFound) {
			return errSessionInvalid
		}
		if err != nil {
			return err
		}
		if d.JKT != claims.JKT || !equalHash(d.RefreshHash, hashSecret(proof.Token)) {
			return errSessionInvalid
		}
		if err := validSession(d); err != nil {
			return err
		}
		if err := s.writeLimit(d.AccountID); err != nil {
			return err
		}
		return write(q, d, s.now())
	})
}

func lockRefreshAccount(ctx context.Context, q *store.Queries, accountID string) error {
	account, err := q.LockAuthAccount(ctx, accountID)
	if errors.Is(err, store.ErrAuthNotFound) {
		deleted, err := q.AuthAccountDeleted(ctx, accountID)
		if err != nil {
			return err
		}
		if deleted {
			return reauth("account_deleted")
		}
		return errSessionInvalid
	}
	if err != nil {
		return err
	}
	return validSession(store.AuthDeviceSession{AccountState: account.State, KeyState: store.KeyActive})
}

func validSession(d store.AuthDeviceSession) error {
	switch {
	case d.AccountState == stateSuspended:
		return reauth(errAccountSuspended.Code)
	case d.AccountState == stateDeleting:
		return reauth("account_deleted")
	case d.DeviceRevoked || d.KeyState == store.KeyRevoked:
		return reauth(errDeviceRevoked.Code)
	case d.AccountState != stateActive || d.KeyState != store.KeyActive || d.Revoked:
		return errSessionInvalid
	default:
		return nil
	}
}

func touchDeviceActivity(ctx context.Context, q *store.Queries, d store.AuthDeviceSession, now time.Time) error {
	if err := q.AdvanceDeviceActivity(ctx, d.DeviceID, now); err != nil {
		return err
	}
	return q.AdvanceAccountActivity(ctx, d.AccountID, now)
}

func sessionEvent(ctx context.Context, q *store.Queries, d store.AuthDeviceSession, event string, now time.Time) error {
	return q.AppendSecurityEvent(ctx, store.SecurityEvent{
		AccountID: d.AccountID, Actor: accountActorPrefix + d.DeviceID, At: now, Type: event,
		Details: map[string]any{deviceIDDetail: d.DeviceID},
	})
}
