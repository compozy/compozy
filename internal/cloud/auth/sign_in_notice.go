package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

const NoticeSessionTTL = 10 * time.Minute

var ErrSignInNotice = failure(http.StatusNotFound, "sign_in_notice_not_found", "This sign-in notice is unavailable.")

func (s *Service) BeginSignInNoticeOAuth(ctx context.Context, eventID int64, cookie string) (string, error) {
	if eventID <= 0 {
		return "", ErrInvalidRequest
	}
	return s.beginWebOAuth(ctx, webOAuthRequest{NoticeEventID: eventID}, Google, cookie)
}

func (s *Service) finishNoticeOAuth(
	ctx context.Context, attempt store.OAuthAttempt, eventID int64, identity store.VerifiedIdentity,
) (string, error) {
	if !attempt.FreshSignIn || attempt.Provider != Google {
		return "", errFreshAuth
	}
	token := Secret()
	_, err := s.signIn(ctx, identity, func(q *store.Queries, now time.Time) (signInCommit, error) {
		return func(a store.AuthAccount, created bool) error {
			if created {
				return ErrSignInNotice
			}
			if _, err := s.lookupNotice(ctx, q, a.ID, eventID); err != nil {
				return err
			}
			return q.CreateNoticeWebSession(ctx, hashSecret(token), a.ID, eventID, now, now.Add(NoticeSessionTTL))
		}, nil
	})
	if err != nil {
		return "", s.failOAuth(ctx, attempt, err)
	}
	return token, nil
}

func (s *Service) SignInNotice(
	ctx context.Context, token string, eventID int64,
) (string, remoteproto.AccountSignInNotice, error) {
	session, err := s.readWebSession(ctx, s.db.Queries, token, eventID)
	if err != nil {
		return "", remoteproto.AccountSignInNotice{}, err
	}
	notice, err := s.lookupNotice(ctx, s.db.Queries, session.AccountID, eventID)
	return session.AccountID, notice, err
}

func (s *Service) CheckNoticeSession(
	ctx context.Context, q *store.Queries, token, accountID string, eventID int64,
) (remoteproto.AccountSignInNotice, error) {
	session, err := s.readWebSession(ctx, q, token, eventID)
	if err != nil {
		return remoteproto.AccountSignInNotice{}, err
	}
	if session.AccountID != accountID {
		return remoteproto.AccountSignInNotice{}, ErrSignInNotice
	}
	return s.lookupNotice(ctx, q, accountID, eventID)
}

func (s *Service) lookupNotice(
	ctx context.Context, q *store.Queries, accountID string, eventID int64,
) (remoteproto.AccountSignInNotice, error) {
	notice, err := q.AccountSignInNotice(ctx, accountID, eventID)
	if errors.Is(err, store.ErrAuthNotFound) {
		return remoteproto.AccountSignInNotice{}, ErrSignInNotice
	}
	return notice, err
}
