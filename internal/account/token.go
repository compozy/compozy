package account

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/zalando/go-keyring"
)

type tokenCloud interface {
	ExchangeAccountToken(context.Context, string, string) (remoteproto.AccountTokenIdentity, error)
}

func WithAccountToken(token string) Option {
	return func(s *Service) { s.token = token }
}

func (s *Service) startToken(ctx context.Context) error {
	if _, ok := s.cloud.(tokenCloud); !ok {
		s.token = ""
	}
	if s.installation.State == SignedIn && s.installation.Source != "token" {
		s.token = ""
		return nil
	}
	if s.token == "" && s.installation.Source == "token" {
		token, err := s.credentials.read(tokenEntry, s.installation.InstallationID)
		if errors.Is(err, keyring.ErrNotFound) {
			return s.signedOutLocked(ctx, ReauthRequired, "session_invalid")
		}
		if err != nil {
			return errKeyring
		}
		s.token = token
	}
	s.exchangeToken(ctx)
	return ctx.Err()
}

func (s *Service) exchangeToken(ctx context.Context) {
	cloud, ok := s.cloud.(tokenCloud)
	if !ok {
		return
	}
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	s.mu.Lock()
	current, token := s.installation, s.token
	s.mu.Unlock()
	if token == "" || (current.State == SignedIn && current.Source != "token") {
		return
	}
	result, err := cloud.ExchangeAccountToken(ctx, token, current.InstallationID)
	if ctx.Err() != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if failure, ok := errors.AsType[*cloudclient.Failure](err); ok && failure.Code == "reauth_required" {
		switch failure.Reason {
		case "token_revoked", "account_suspended", "account_deleted":
			if saveErr := s.signedOutLocked(ctx, ReauthRequired, failure.Reason); saveErr != nil {
				s.postponeRefreshLocked(saveErr)
				return
			}
			s.token = ""
			return
		}
	}
	if err == nil {
		err = s.saveTokenIdentityLocked(ctx, result)
	}
	if err != nil {
		s.postponeRefreshLocked(err)
	}
}

func (s *Service) saveTokenIdentityLocked(ctx context.Context, result remoteproto.AccountTokenIdentity) error {
	if result.AccountID == "" || result.TokenID == "" || result.Email == "" {
		return errors.New("cloud returned an incomplete account token identity")
	}
	if s.linkedAccountID != "" {
		return ErrTokenLocalOnly
	}
	next := s.installation
	if err := s.credentials.write(tokenEntry, next.InstallationID, s.token); err != nil {
		return err
	}
	next.LastValidatedAt = s.clock.Now()
	if next.State == SignedIn && next.Source == "token" && next.AccountID == result.AccountID &&
		next.TokenName == result.TokenName && next.Profile.Email == result.Email {
		if err := s.store.SaveInstallationAccount(ctx, next.AuthGeneration, next); err != nil {
			return err
		}
		s.installation = next
		s.retryAt, s.backoff, s.retryErr = time.Time{}, 0, nil
		return nil
	}
	next.State, next.Source, next.SignedOutReason = SignedIn, "token", ""
	next.AccountID, next.TokenName = result.AccountID, result.TokenName
	next.Profile = remoteproto.AccountSummary{Email: result.Email}
	if err := s.transitionLocked(ctx, next, deviceSession{}); err != nil {
		return err
	}
	s.sendProgressLocked(SignInEvent{State: string(SignedIn)})
	return nil
}
