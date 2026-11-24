package account

import (
	"context"
	"net/http"
	"time"

	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/remoteproto"
)

func (s *Service) SetMarketingConsent(ctx context.Context, optIn bool, source string) (Installation, error) {
	if source != "settings" && source != "cli" {
		return Installation{}, &cloudclient.Failure{Status: http.StatusBadRequest, Code: "invalid_request",
			Message: "Check the email preference and try again."}
	}
	var result Installation
	err := s.withDevice(ctx, func(ctx context.Context, g *generation, creds cloudclient.Credentials) error {
		var err error
		result, err = s.patchMarketingConsent(ctx, g, creds, optIn, source)
		return err
	})
	return result, err
}

func (s *Service) patchMarketingConsent(
	ctx context.Context, g *generation, creds cloudclient.Credentials, optIn bool, source string,
) (Installation, error) {
	summary, err := s.cloud.PatchAccountEmail(ctx, creds, remoteproto.AccountEmailPatch{
		MarketingOptIn: new(optIn), Source: source,
	})
	if err != nil {
		return Installation{}, err
	}
	return s.applyCloudSummary(ctx, g, summary, true)
}

func (s *Service) DeleteAccount(ctx context.Context, confirmEmail string) (remoteproto.AccountDeleteResponse, error) {
	var result remoteproto.AccountDeleteResponse
	var deleted *generation
	err := s.withDevice(ctx, func(ctx context.Context, g *generation, creds cloudclient.Credentials) error {
		var err error
		result, err = s.cloud.DeleteAccount(ctx, creds, remoteproto.AccountDeleteRequest{ConfirmEmail: confirmEmail})
		if err == nil {
			deleted = g
		}
		return err
	})
	if deleted == nil {
		return result, err
	}
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if deleted != s.generation {
		return result, nil
	}
	s.cancelAttemptLocked()
	if err := s.signedOutLocked(s.ctx, SignedOut, "account_deleted"); err != nil {
		if deleted == s.generation {
			return result, err
		}
		s.logger.Warn("Deleted account credential could not be removed")
	}
	return result, nil
}

func (s *Service) DismissSignInNotices(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.installation.State != SignedIn {
		return ErrSignInRequired
	}
	next := s.installation
	next.SignInNoticesDismissedAt = s.clock.Now()
	if err := s.store.SaveInstallationAccount(ctx, next.AuthGeneration, next); err != nil {
		return err
	}
	s.installation = next
	s.emitLocked(s.generation, Change{Kind: "profile"})
	return nil
}

func withVisibleNotices(installation Installation, now time.Time) Installation {
	cutoff := now.Add(-remoteproto.SignInNoticeTTL)
	if installation.SignInNoticesDismissedAt.After(cutoff) {
		cutoff = installation.SignInNoticesDismissedAt
	}
	notices := make([]remoteproto.AccountSignInNotice, 0, len(installation.Profile.RecentSignIns))
	for _, notice := range installation.Profile.RecentSignIns {
		if notice.At.After(cutoff) {
			notices = append(notices, notice)
		}
	}
	installation.Profile.RecentSignIns = notices
	return installation
}
