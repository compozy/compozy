package account

import (
	"context"
	"crypto/rand"
	"net/http"
	"time"

	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/remoteproto"
)

// UseDevice binds the operation, including response delivery, to the current account.
func (s *Service) UseDevice(ctx context.Context, call func(context.Context, cloudclient.Credentials) error) error {
	return s.withDevice(ctx, func(ctx context.Context, _ *generation, creds cloudclient.Credentials) error {
		return call(ctx, creds)
	})
}

func (s *Service) withDevice(
	ctx context.Context, call func(context.Context, *generation, cloudclient.Credentials) error,
) error {
	s.mu.Lock()
	g, state := s.generation, s.installation.State
	s.mu.Unlock()
	if state != SignedIn {
		return ErrSignInRequired
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(g.ctx, cancel)
	defer stop()
	if err := ctx.Err(); err != nil {
		return err
	}
	creds, err := s.refresh(g, true)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	err = call(ctx, g, creds)
	if g.ctx.Err() != nil {
		return ErrSignInRequired
	}
	if shouldRevalidate(err) {
		s.expireAccess(g, creds.AccessToken)
		if _, refreshErr := s.refresh(g, true); reauthReason(refreshErr) != "" {
			return refreshErr
		}
	}
	return err
}

func shouldRevalidate(err error) bool {
	switch cloudclient.FailureCode(err) {
	case "device_revoked", "account_suspended", "account_deleting", "reauth_required":
		return true
	default:
		return false
	}
}

func (s *Service) expireAccess(g *generation, token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if g == s.generation && token == s.session.access {
		s.session.accessExpires = time.Time{}
	}
}

func (s *Service) Devices(ctx context.Context) (devices []remoteproto.AccountDevice, err error) {
	err = s.UseDevice(ctx, func(ctx context.Context, creds cloudclient.Credentials) error {
		var callErr error
		devices, callErr = s.cloud.Devices(ctx, creds)
		return callErr
	})
	return devices, err
}

func (s *Service) PatchDevice(ctx context.Context, id string, patch remoteproto.AccountDevicePatch) (
	result remoteproto.AccountDevice, err error,
) {
	err = s.UseDevice(ctx, func(ctx context.Context, creds cloudclient.Credentials) error {
		var callErr error
		result, callErr = s.cloud.PatchDevice(ctx, creds, id, patch)
		return callErr
	})
	return result, err
}

func (s *Service) RevokeDevice(ctx context.Context, id string) error {
	var revoked *generation
	err := s.withDevice(ctx, func(ctx context.Context, g *generation, creds cloudclient.Credentials) error {
		s.mu.Lock()
		self := g == s.generation && id == string(s.installation.Profile.Device.ID)
		s.mu.Unlock()
		if err := s.cloud.RevokeDevice(ctx, creds, id); err != nil {
			return err
		}
		if self {
			revoked = g
		}
		return nil
	})
	if revoked == nil {
		return err
	}
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if revoked != s.generation {
		return nil
	}
	return s.signedOutLocked(s.ctx, ReauthRequired, "device_revoked")
}

func (s *Service) Approvals(ctx context.Context) (approvals []remoteproto.AccountApproval, err error) {
	err = s.UseDevice(ctx, func(ctx context.Context, creds cloudclient.Credentials) error {
		var callErr error
		approvals, callErr = s.cloud.Approvals(ctx, creds)
		return callErr
	})
	return approvals, err
}

func (s *Service) Approval(ctx context.Context, id string) (result remoteproto.AccountApproval, err error) {
	err = s.UseDevice(ctx, func(ctx context.Context, creds cloudclient.Credentials) error {
		details, callErr := s.cloud.Approval(ctx, creds, id)
		result = details.AccountApproval
		return callErr
	})
	return result, err
}

func (s *Service) DecideApproval(
	ctx context.Context,
	id string,
	approve bool,
) (out remoteproto.ApprovalDelivery, err error) {
	err = s.withDevice(ctx, func(ctx context.Context, g *generation, creds cloudclient.Credentials) error {
		approval, err := s.cloud.Approval(ctx, creds, id)
		if err != nil {
			return err
		}
		decision := "deny"
		if approve {
			decision = "approve"
		}
		jws, err := remoteproto.SignApprovalDecision(creds.Key, remoteproto.ApprovalDecision{
			ApprovalID: approval.ID, ComputerID: approval.ComputerID, DeviceID: approval.DeviceID,
			JKT: approval.KeyFingerprint, Decision: decision, JTI: rand.Text(), IssuedAt: s.clock.Now().Unix(),
		})
		if err != nil {
			return err
		}
		out, err = s.cloud.DecideApproval(ctx, creds, id, jws)
		if err == nil {
			s.holdFeed(g, id, remoteproto.ApprovalTTL)
		}
		return err
	})
	return out, err
}

func (s *Service) Checkout(ctx context.Context, interval string) (target string, err error) {
	if interval != "month" && interval != "year" {
		return "", &cloudclient.Failure{Status: http.StatusBadRequest, Code: "invalid_request",
			Message: "Choose a monthly or annual checkout."}
	}
	err = s.withDevice(ctx, func(ctx context.Context, g *generation, creds cloudclient.Credentials) error {
		var callErr error
		target, callErr = s.cloud.Checkout(ctx, creds, interval)
		if callErr == nil {
			s.holdFeed(g, checkoutInterest, 30*time.Minute)
		}
		return callErr
	})
	return target, err
}

func (s *Service) BillingPortal(ctx context.Context) (target string, err error) {
	err = s.UseDevice(ctx, func(ctx context.Context, creds cloudclient.Credentials) error {
		var callErr error
		target, callErr = s.cloud.BillingPortal(ctx, creds)
		return callErr
	})
	return target, err
}
