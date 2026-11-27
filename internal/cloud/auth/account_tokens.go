// SPDX-License-Identifier: FSL-1.1-MIT

package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

func IsAccountTokenCredential(value string) bool {
	return strings.HasPrefix(value, remoteproto.AccountTokenPrefix) ||
		strings.HasPrefix(value, remoteproto.AccountTokenIDPrefix)
}

func AccountTokenRouteError() error { return errAccountTokenLocalOnly }

func (s *Service) ExchangeAccountToken(
	ctx context.Context, token string, request remoteproto.AccountTokenExchange,
) (result remoteproto.AccountTokenIdentity, err error) {
	secret, ok := strings.CutPrefix(token, remoteproto.AccountTokenPrefix)
	if !ok || !ValidSecret(secret) {
		return result, reauth("token_revoked")
	}
	if !strings.HasPrefix(request.InstallationID, "ins_") || len(request.InstallationID) > 128 {
		return result, ErrInvalidRequest
	}
	hash := hashSecret(token)
	record, err := s.db.ReadAccountToken(ctx, hash)
	if errors.Is(err, store.ErrAuthNotFound) {
		return result, reauth("token_revoked")
	}
	if err != nil {
		return result, err
	}
	if err := s.writeLimit(record.AccountID); err != nil {
		return result, err
	}
	err = s.db.Transaction(ctx, func(q *store.Queries) error {
		account, err := q.LockAuthAccount(ctx, record.AccountID)
		if errors.Is(err, store.ErrAuthNotFound) {
			return reauth("token_revoked")
		}
		if err != nil {
			return err
		}
		switch account.State {
		case stateSuspended:
			return reauth("account_suspended")
		case stateDeleting:
			return reauth("account_deleted")
		}
		record, err := q.ReadAccountToken(ctx, hash)
		if errors.Is(err, store.ErrAuthNotFound) || record.Revoked {
			return reauth("token_revoked")
		}
		if err != nil {
			return err
		}
		now := s.now()
		if err := q.TouchAccountToken(ctx, record.ID, request.InstallationID, now); err != nil {
			return err
		}
		result = remoteproto.AccountTokenIdentity{
			AccountID: account.ID, Email: account.Email, TokenID: record.ID, TokenName: record.Name,
		}
		if err := q.AdvanceAccountActivity(ctx, account.ID, now); err != nil {
			return err
		}
		return q.RecordInstallationActivity(ctx, account.ID, request.InstallationID, now)
	})
	return result, err
}

func (s *Service) ListAccountTokens(ctx context.Context, p DevicePrincipal) ([]remoteproto.AccountToken, error) {
	return s.db.ListAccountTokens(ctx, p.AccountID)
}

func (s *Service) CreateAccountToken(
	ctx context.Context, p DevicePrincipal, name string,
) (remoteproto.AccountTokenCreated, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 100 || strings.ContainsAny(name, "\x00\r\n") {
		return remoteproto.AccountTokenCreated{}, ErrInvalidRequest
	}
	result := remoteproto.AccountTokenCreated{
		ID:   remoteproto.AccountTokenIDPrefix + strings.ToLower(rand.Text()),
		Name: name, Token: remoteproto.AccountTokenPrefix + Secret(),
	}
	err := s.accountTokenWrite(ctx, p, func(q *store.Queries) error {
		if err := q.InsertAccountToken(
			ctx,
			result.ID,
			p.AccountID,
			name,
			hashSecret(result.Token),
			s.now(),
		); err != nil {
			return err
		}
		return s.recordAccountTokenEvent(ctx, q, p, "account_token_created", result.ID)
	})
	if err != nil {
		return remoteproto.AccountTokenCreated{}, err
	}
	return result, nil
}

func (s *Service) RevokeAccountToken(ctx context.Context, p DevicePrincipal, id string) error {
	return s.accountTokenWrite(ctx, p, func(q *store.Queries) error {
		revoked, err := q.RevokeAccountToken(ctx, id, p.AccountID, s.now())
		if err != nil {
			return err
		}
		if !revoked {
			return failure(http.StatusNotFound, "account_token_not_found", "Account token not found.")
		}
		return s.recordAccountTokenEvent(ctx, q, p, "account_token_revoked", id)
	})
}

func (s *Service) accountTokenWrite(ctx context.Context, p DevicePrincipal, run func(*store.Queries) error) error {
	if err := s.writeLimit(p.AccountID); err != nil {
		return err
	}
	return s.db.Transaction(ctx, func(q *store.Queries) error {
		account, err := q.LockAuthAccount(ctx, p.AccountID)
		if err != nil {
			return err
		}
		if err := requireActive(account); err != nil {
			return err
		}
		active, err := q.ControlPrincipalActive(ctx, p.AccountID, p.DeviceID, p.JKT, p.SessionID)
		if err != nil {
			return err
		}
		if !active {
			return errSessionInvalid
		}
		return run(q)
	})
}

func (s *Service) recordAccountTokenEvent(
	ctx context.Context, q *store.Queries, p DevicePrincipal, kind, id string,
) error {
	return q.AppendSecurityEvent(ctx, store.SecurityEvent{
		AccountID: p.AccountID, Type: kind, Actor: accountActorPrefix + string(p.DeviceID), At: s.now(),
		Details: map[string]any{"token_id": id},
	})
}
