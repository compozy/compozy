package account

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"net/http"
	"time"
	"uuid"

	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/jonboulle/clockwork"
	"github.com/zalando/go-keyring"
)

type loginAttempt struct {
	id             string
	ctx            context.Context
	cancel         context.CancelFunc
	grant          remoteproto.DeviceFlowResponse
	expires        time.Time
	marketingOptIn bool
	newAccount     bool
}

func (s *Service) StartLogin(ctx context.Context, provider string) (Login, error) {
	return s.StartLoginWithConsent(ctx, provider, false)
}

func (s *Service) StartLoginWithConsent(ctx context.Context, provider string, marketingOptIn bool) (Login, error) {
	if provider != "" && provider != "google" && provider != "github" {
		return Login{}, &cloudclient.Failure{Status: http.StatusBadRequest, Code: "invalid_request",
			Message: "Choose Google or GitHub."}
	}
	if err := ctx.Err(); err != nil {
		return Login{}, err
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return Login{}, context.Canceled
	}
	s.cancelAttemptLocked()
	attemptCtx, cancel := context.WithCancel(s.ctx)
	a := &loginAttempt{
		id:             "att_" + uuid.New().String(),
		ctx:            attemptCtx,
		cancel:         cancel,
		marketingOptIn: marketingOptIn,
	}
	s.attempt = a
	input := remoteproto.DeviceFlowRequest{Provider: provider, ClientName: s.name, ClientPlatform: s.platform,
		InstallationID: s.installation.InstallationID}
	s.mu.Unlock()
	stop := context.AfterFunc(ctx, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.attempt == a {
			s.cancelAttemptLocked()
		}
	})
	defer stop()
	started := s.clock.Now()
	grant, err := s.cloud.StartDeviceFlow(s.ctx, input)
	if err != nil {
		s.finishAttempt(a, "failed", cloudclient.FailureCode(err))
		return Login{}, err
	}
	return s.beginPolling(a, started, grant)
}

func (s *Service) beginPolling(
	a *loginAttempt,
	started time.Time,
	grant remoteproto.DeviceFlowResponse,
) (Login, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a.grant = grant
	a.expires = started.Add(time.Duration(grant.ExpiresIn) * time.Second)
	if s.closed || s.attempt != a {
		s.cancelGrantLocked(a)
		return Login{}, errCancelled
	}
	if !s.loginCurrentLocked(a) {
		return Login{}, errExpired
	}
	s.sendProgressLocked(SignInEvent{AttemptID: a.id, State: "waiting"})
	timer := s.clock.NewTimer(max(0, s.clock.Until(a.expires)))
	s.workers.Go(func() {
		defer timer.Stop()
		select {
		case <-a.ctx.Done():
		case <-timer.Chan():
			s.finishAttempt(a, "expired", errExpired.Code)
		}
	})
	interval := max(time.Second, time.Duration(grant.Interval)*time.Second)
	poll := s.clock.NewTimer(interval)
	s.workers.Go(func() { s.pollLogin(a, poll, interval) })
	return Login{AttemptID: a.id, VerificationURI: grant.VerificationURI,
		VerificationURIComplete: grant.VerificationURIComplete, UserCode: grant.UserCode, ExpiresAt: a.expires}, nil
}

func (s *Service) CancelLogin(ctx context.Context, attemptID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if attemptID == "" || (s.attempt != nil && attemptID == s.attempt.id) {
		s.cancelAttemptLocked()
	}
	return nil
}

func (s *Service) cancelAttemptLocked() {
	if a := s.attempt; a != nil {
		s.endAttemptLocked(a, "cancelled", "") //nolint:misspell // Public protocol state.
		s.cancelGrantLocked(a)
	}
}

func (s *Service) cancelGrantLocked(a *loginAttempt) {
	if s.closed || a.grant.DeviceCode == "" {
		return
	}
	s.workers.Go(func() {
		for s.ctx.Err() == nil {
			err := s.cloud.CancelDeviceFlow(s.ctx, a.grant.DeviceCode)
			if !temporary(err) {
				return
			}
			pause := max(5*time.Second, cloudclient.RetryDelay(s.clock.Now(), err))
			if !s.clock.Now().Add(pause).Before(a.expires) || !s.wait(s.ctx, pause) {
				return
			}
		}
	})
}

func (s *Service) pollLogin(a *loginAttempt, poll clockwork.Timer, interval time.Duration) {
	defer poll.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-poll.Chan():
		}
		if !s.loginCurrent(a) {
			return
		}
		result, err := s.cloud.PollDeviceFlow(a.ctx, a.grant.DeviceCode)
		if err == nil {
			s.enrollLogin(a, result)
			return
		}
		code := cloudclient.FailureCode(err)
		var pause time.Duration
		switch {
		case code == "slow_down":
			interval += 5 * time.Second
			pause = interval
		case code == "authorization_pending":
			pause = interval
		case code == "expired_token":
			s.finishAttempt(a, "expired", errExpired.Code)
			return
		case temporary(err):
			pause = max(interval, cloudclient.RetryDelay(s.clock.Now(), err))
		default:
			s.finishAttempt(a, "failed", code)
			return
		}
		poll.Reset(pause)
	}
}

func (s *Service) enrollLogin(a *loginAttempt, result remoteproto.DeviceFlowTokenResponse) {
	s.mu.Lock()
	if !s.loginCurrentLocked(a) {
		s.mu.Unlock()
		return
	}
	s.sendProgressLocked(SignInEvent{AttemptID: a.id, State: "authorized"})
	a.newAccount = result.NewAccount
	installationID := s.installation.InstallationID
	s.mu.Unlock()
	var replaced bool
	for s.loginCurrent(a) {
		err := s.enrollOnce(a, result, installationID, &replaced)
		if err == nil {
			return
		}
		if temporary(err) {
			if s.wait(a.ctx, max(5*time.Second, cloudclient.RetryDelay(s.clock.Now(), err))) {
				continue
			}
			return
		}
		s.finishAttempt(a, "failed", cloudclient.FailureCode(err))
		return
	}
}

func (s *Service) enrollOnce(
	a *loginAttempt, result remoteproto.DeviceFlowTokenResponse, installationID string, replaced *bool,
) error {
	// Cloud enrollment replaces any previous session for this device. Resolve and
	// clean up an in-flight enrollment before another attempt can enroll its key.
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	key, err := s.loginKey(a, result.AccountID, false)
	if err != nil {
		return err
	}
	now := s.clock.Now()
	device, err := s.enroll(result.EnrollmentToken, installationID, key)
	if cloudclient.FailureCode(err) == "device_key_revoked" && !*replaced {
		*replaced = true
		key, err = s.loginKey(a, result.AccountID, true)
		if err != nil {
			return err
		}
		device, err = s.enroll(result.EnrollmentToken, installationID, key)
	}
	if err != nil {
		return err
	}
	s.finishEnrollment(a, result, device, key, now)
	return nil
}

func (s *Service) loginKey(a *loginAttempt, id string, replace bool) (*ecdsa.PrivateKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.loginCurrentLocked(a) {
		return nil, errCancelled
	}
	if id == "" {
		return nil, errKeyring
	}
	if s.linkedAccountID != "" && s.linkedAccountID != id {
		return nil, linkedAccountConflict(s.linkedEmail)
	}
	if replace {
		if err := s.credentials.remove(deviceKeyEntry, id); err != nil {
			return nil, err
		}
	}
	return s.credentials.deviceKey(id, true)
}

func (s *Service) enroll(credential, installationID string, key *ecdsa.PrivateKey) (
	remoteproto.DeviceEnrollResponse, error,
) {
	jwk, err := remoteproto.NewPublicJWK(key.Public())
	if err != nil {
		return remoteproto.DeviceEnrollResponse{}, err
	}
	return s.cloud.Enroll(s.ctx, credential, key, remoteproto.DeviceEnrollRequest{
		PublicJWK: jwk, Name: s.name, Kind: "desktop", InstallationID: installationID,
	})
}

func (s *Service) finishEnrollment(
	a *loginAttempt, result remoteproto.DeviceFlowTokenResponse, device remoteproto.DeviceEnrollResponse,
	key *ecdsa.PrivateKey, now time.Time,
) {
	session := deviceSession{
		key:           key,
		access:        device.AccessToken,
		refresh:       device.RefreshToken,
		accessExpires: now.Add(accessTTL),
	}
	if device.DeviceID == "" || device.AccessToken == "" || device.RefreshToken == "" {
		s.finishAttempt(a, "failed", "account_operation_failed")
		return
	}
	creds := cloudclient.Credentials{Key: key, AccessToken: session.access}
	profile, err := s.cloud.Account(a.ctx, creds)
	if err != nil {
		profile = remoteproto.AccountSummary{Email: result.Email,
			Device: remoteproto.AccountDevice{ID: device.DeviceID, Name: s.name, Admin: device.Admin}}
	}
	generation, err := s.commitLogin(a, result.AccountID, profile, session, now)
	if err != nil {
		s.finishAttempt(a, "failed", cloudclient.FailureCode(err))
		if err := s.cloud.Logout(s.ctx, session.refresh, session.key); err != nil && s.ctx.Err() == nil {
			s.logger.Debug("enrollment cleanup deferred by cloud", "code", cloudclient.FailureCode(err))
		}
		return
	}
	if a.marketingOptIn && generation.ctx.Err() == nil {
		creds := cloudclient.Credentials{Key: key, AccessToken: session.access}
		if _, err := s.patchMarketingConsent(generation.ctx, generation, creds, true, "sign_in"); err != nil &&
			generation.ctx.Err() == nil {
			s.logger.Warn("Sign-in email preference could not be saved", "code", cloudclient.FailureCode(err))
		}
	}
}

func (s *Service) commitLogin(
	a *loginAttempt, id string, profile remoteproto.AccountSummary, session deviceSession, now time.Time,
) (*generation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.loginCurrentLocked(a) {
		return nil, errCancelled
	}
	if s.linkedAccountID != "" && s.linkedAccountID != id {
		return nil, linkedAccountConflict(s.linkedEmail)
	}
	next := s.installation
	previousID := next.AccountID
	previousSource := next.Source
	if profile.Plan == "" && previousID == id {
		profile.AccountPlan = next.Profile.AccountPlan
	}
	oldRefresh, err := s.credentials.read(refreshEntry, id)
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return nil, errKeyring
	}
	if err := s.credentials.write(refreshEntry, id, session.refresh); err != nil {
		return nil, err
	}
	next.State, next.Source, next.AccountID, next.SignedOutReason = SignedIn, "session", id, ""
	next.TokenName = ""
	next.Profile, next.LastValidatedAt = profile, now
	if previousID != id {
		next.SignInNoticesDismissedAt = time.Time{}
	}
	if err := s.transitionLocked(s.ctx, next, session); err != nil {
		return nil, errors.Join(err, s.restoreRefresh(id, oldRefresh))
	}
	s.token = ""
	if previousSource == "token" {
		if err := s.credentials.remove(tokenEntry, next.InstallationID); err != nil {
			s.logger.Warn("previous account token could not be removed", "code", cloudclient.FailureCode(err))
		}
	}
	if s.linkedAccountID == id {
		s.linkedEmail = profile.Email
	}
	if previousSource == "session" && previousID != "" && previousID != id {
		if err := s.credentials.remove(refreshEntry, previousID); err != nil {
			s.logger.Warn("previous account credential could not be removed", "code", cloudclient.FailureCode(err))
		}
	}
	s.endAttemptLocked(a, string(SignedIn), "")
	s.sendProgressLocked(SignInEvent{State: string(SignedIn)})
	s.emitLocked(s.generation, Change{Kind: "sign_in"})
	return s.generation, nil
}

func (s *Service) restoreRefresh(id, oldRefresh string) error {
	if oldRefresh != "" {
		return s.credentials.write(refreshEntry, id, oldRefresh)
	}
	return s.credentials.remove(refreshEntry, id)
}

func (s *Service) loginCurrent(a *loginAttempt) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loginCurrentLocked(a)
}

func (s *Service) loginCurrentLocked(a *loginAttempt) bool {
	if s.closed || s.attempt != a {
		return false
	}
	if !s.clock.Now().Before(a.expires) {
		s.endAttemptLocked(a, "expired", errExpired.Code)
		return false
	}
	return true
}

func (s *Service) finishAttempt(a *loginAttempt, state, code string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.attempt != a || s.closed {
		return
	}
	if !a.expires.IsZero() && !s.clock.Now().Before(a.expires) {
		state, code = "expired", errExpired.Code
	}
	s.endAttemptLocked(a, state, code)
}

func (s *Service) endAttemptLocked(a *loginAttempt, state, code string) {
	s.attempt = nil
	a.cancel()
	s.sendProgressLocked(SignInEvent{AttemptID: a.id, State: state, Code: code,
		NewAccount: state == string(SignedIn) && a.newAccount})
}

func temporary(err error) bool {
	failure, ok := errors.AsType[*cloudclient.Failure](err)
	return ok && failure.Temporary()
}

func (s *Service) wait(ctx context.Context, delay time.Duration) bool {
	timer := s.clock.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.Chan():
		return ctx.Err() == nil
	}
}
