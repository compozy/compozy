package account

import (
	"context"
	"fmt"
	"net/http"

	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/remoteproto"
)

func WithLinkedAccount(id string) Option {
	return func(s *Service) { s.linkedAccountID = id }
}

func (s *Service) restoreHostLink(ctx context.Context) error {
	if s.linkedAccountID == "" {
		if s.installation.Source == SourceLink {
			return s.signedOutLocked(ctx, ReauthRequired, "computer_unlinked")
		}
		return nil
	}
	if s.installation.AccountID == s.linkedAccountID {
		s.linkedEmail = s.installation.Profile.Email
		if s.installation.State == SignedIn {
			return nil
		}
	}
	switch s.installation.SignedOutReason {
	case reasonAccountSuspended, reasonAccountDeleted, "computer_unlinked", "signed_out":
		return nil
	default:
		return s.useHostLinkLocked(ctx)
	}
}

// CommitHostLink keeps installation identity changes serialized with enrollment,
// refresh and sign-out. The host owns the durable computer-key record.
func (s *Service) CommitHostLink(ctx context.Context, id, email string, commit func() error) error {
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	current := s.installation
	if current.Source == SourceToken {
		return ErrTokenLocalOnly
	}
	if current.State == SignedIn && current.AccountID != id {
		return linkedAccountConflict(email)
	}
	if s.linkedAccountID != "" && s.linkedAccountID != id {
		return linkedAccountConflict(s.linkedEmail)
	}
	if commit != nil {
		if err := commit(); err != nil {
			return err
		}
	}
	s.linkedAccountID = id
	if email != "" {
		s.linkedEmail = email
	}
	s.token = ""
	if current.State == SignedIn {
		if current.Source == SourceSession && s.linkedEmail == "" {
			s.linkedEmail = current.Profile.Email
		}
		return nil
	}
	if current.State == ReauthRequired && current.SignedOutReason == reasonAccountSuspended {
		return nil
	}
	return s.useHostLinkLocked(ctx)
}

func (s *Service) useHostLinkLocked(ctx context.Context) error {
	next := s.installation
	next.State, next.Source, next.SignedOutReason = SignedIn, SourceLink, ""
	next.AccountID, next.TokenName = s.linkedAccountID, ""
	next.Profile = remoteproto.AccountSummary{Email: s.linkedEmail}
	next.LastValidatedAt = s.clock.Now()
	if err := s.transitionLocked(ctx, next, deviceSession{}); err != nil {
		return err
	}
	s.sendProgressLocked(SignInEvent{State: string(SignedIn)})
	return nil
}

func (s *Service) ObserveHostLink(ctx context.Context, id, reason string) error {
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if id != s.linkedAccountID {
		return nil
	}
	switch reason {
	case "":
		if (s.installation.State == SignedOut && s.installation.SignedOutReason != reasonAccountDeleted) ||
			(s.installation.State == ReauthRequired && s.installation.SignedOutReason == reasonAccountSuspended) {
			return s.useHostLinkLocked(ctx)
		}
		if s.installation.Source == SourceLink {
			next := s.installation
			next.LastValidatedAt = s.clock.Now()
			if err := s.store.SaveInstallationAccount(ctx, next.AuthGeneration, next); err != nil {
				return err
			}
			s.installation = next
		}
	case reasonAccountSuspended:
		if s.installation.State != ReauthRequired || s.installation.SignedOutReason != reason {
			return s.signedOutLocked(ctx, ReauthRequired, reason)
		}
	case "computer_unlinked", reasonAccountDeleted, "signed_out":
		if s.installation.Source == SourceLink || s.installation.State == ReauthRequired ||
			reason == reasonAccountDeleted {
			state := ReauthRequired
			if reason == "signed_out" {
				state = SignedOut
			}
			if err := s.signedOutLocked(ctx, state, reason); err != nil {
				return err
			}
		}
		s.linkedAccountID, s.linkedEmail = "", ""
	}
	return nil
}

func (s *Service) rejectDeviceLocked(reason string) error {
	if s.linkedAccountID == s.installation.AccountID && s.linkedAccountID != "" &&
		(reason == reasonDeviceRevoked || reason == "session_invalid") {
		id := s.installation.AccountID
		if err := s.useHostLinkLocked(s.ctx); err != nil {
			return err
		}
		return s.credentials.remove(refreshEntry, id)
	}
	return s.signedOutLocked(s.ctx, ReauthRequired, reason)
}

func linkedAccountConflict(email string) error {
	message := "This computer is linked for remote access. Unlink it before signing in with another account."
	if email != "" {
		message = fmt.Sprintf(
			"This computer is linked to %s for remote access. Unlink it before signing in with another account.", email,
		)
	}
	return &cloudclient.Failure{
		Status:  http.StatusConflict,
		Code:    "installation_linked_to_other_account",
		Message: message,
	}
}
