package auth

import (
	"context"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/cloud/plan"
	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

const (
	WebSessionTTL       = 90 * 24 * time.Hour
	WebSessionIdle      = 30 * 24 * time.Hour
	browserCodeTTL      = time.Minute
	browserCallbackPath = "/remote/v1/browser/callback"
)

type BrowserRequest struct {
	Hostname  string
	PublicJWK string
	State     string
	Challenge string
	Provider  string
}

type BrowserResult struct {
	Redirect   string
	Binding    string
	WebSession string
}

func (s *Service) ParseBrowserRequest(form url.Values) (BrowserRequest, error) {
	request := BrowserRequest{
		Hostname: form.Get("hostname"), PublicJWK: form.Get("public_jwk"), State: form.Get(oauthStateField),
		Challenge: form.Get(pkceChallengeField), Provider: form.Get(providerField),
	}
	prefix, ok := strings.CutSuffix(request.Hostname, "."+s.cfg.HostedDomain)
	if !ok || !validDNSLabel(prefix) || !validLabel(request.State, 512) || !ValidSecret(request.Challenge) ||
		(request.Provider != "" && !SupportedProvider(request.Provider)) {
		return BrowserRequest{}, ErrInvalidRequest
	}
	if _, _, err := browserKey(request.PublicJWK); err != nil {
		return BrowserRequest{}, err
	}
	return request, nil
}

func validDomain(domain string) bool {
	if !strings.Contains(domain, ".") || len(domain) > 190 {
		return false
	}
	for label := range strings.SplitSeq(domain, ".") {
		if !validDNSLabel(label) {
			return false
		}
	}
	return true
}

func validDNSLabel(label string) bool {
	return len(label) > 0 && len(label) <= 63 && label[0] != '-' && label[len(label)-1] != '-' &&
		!strings.ContainsFunc(label, func(r rune) bool {
			return r != '-' && (r < 'a' || r > 'z') && (r < '0' || r > '9')
		})
}

func browserKey(encoded string) (remoteproto.JWK, string, error) {
	var jwk remoteproto.JWK
	if len(encoded) > 2048 || strings.ContainsAny(encoded, "\r\n") {
		return jwk, "", ErrInvalidRequest
	}
	data, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil || json.Unmarshal(data, &jwk) != nil || jwk.Kty != "EC" {
		return jwk, "", ErrInvalidRequest
	}
	public, err := jwk.PublicKey()
	if err != nil {
		return jwk, "", ErrInvalidRequest
	}
	jwk, err = remoteproto.NewPublicJWK(public)
	if err != nil {
		return jwk, "", ErrInvalidRequest
	}
	jkt, err := jwk.Thumbprint()
	return jwk, jkt, err
}

func (s *Service) BeginBrowserOAuth(ctx context.Context, request BrowserRequest, cookie string) (string, error) {
	if !SupportedProvider(request.Provider) || !ValidSecret(cookie) {
		return "", ErrInvalidRequest
	}
	data, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	now := s.now()
	return s.beginOAuth(ctx, store.OAuthAttempt{
		BrowserRequest: data, Provider: request.Provider, CookieHash: hashSecret(cookie),
		CreatedAt: now, ExpiresAt: now.Add(oauthAttemptTTL),
	})
}

func (s *Service) WebSession(ctx context.Context, token string) (store.CloudWebSession, error) {
	if !ValidSecret(token) {
		return store.CloudWebSession{}, ErrWebSession
	}
	now := s.now()
	session, err := s.db.TouchWebSession(ctx, hashSecret(token), now, now.Add(-WebSessionIdle))
	if errors.Is(err, store.ErrAuthNotFound) {
		return store.CloudWebSession{}, ErrWebSession
	}
	return session, err
}

func (s *Service) ReuseBrowserSession(
	ctx context.Context, request BrowserRequest, token, binding, agent string,
) (BrowserResult, error) {
	session, err := s.WebSession(ctx, token)
	if err != nil {
		return BrowserResult{}, err
	}
	return s.issueBrowserCode(ctx, request, session, token, binding, agent)
}

func (s *Service) finishBrowserOAuth(
	ctx context.Context, a store.OAuthAttempt, identity store.VerifiedIdentity, cb Callback,
) (CallbackResult, error) {
	var request BrowserRequest
	if err := json.Unmarshal(a.BrowserRequest, &request); err != nil {
		return CallbackResult{}, errOAuthState
	}
	token := Secret()
	account, err := s.signIn(ctx, identity, func(q *store.Queries, now time.Time) (signInCommit, error) {
		return func(account store.AuthAccount, _ bool) error {
			return q.CreateWebSession(ctx, hashSecret(token), account.ID, now, now.Add(WebSessionTTL))
		}, nil
	})
	if err != nil {
		return CallbackResult{}, s.failOAuth(ctx, a, err)
	}
	session := store.CloudWebSession{AccountID: account.ID, Email: account.Email}
	result, err := s.issueBrowserCode(ctx, request, session, token, cb.BrowserBinding, cb.UserAgent)
	return CallbackResult{Account: account, Browser: result, WebSession: token}, err
}

func (s *Service) issueBrowserCode(
	ctx context.Context, req BrowserRequest, session store.CloudWebSession, token, binding, agent string,
) (BrowserResult, error) {
	jwk, jkt, err := browserKey(req.PublicJWK)
	if err != nil {
		return BrowserResult{}, err
	}
	if !ValidSecret(binding) {
		binding = Secret()
	}
	var result BrowserResult
	issued := func(q *store.Queries) error {
		now := s.now()
		computerID, err := s.admitBrowserComputer(ctx, q, req.Hostname, session.AccountID, now)
		if err != nil {
			return err
		}
		pending := store.AuthDeviceKey{
			JKT: jkt, JWK: jwk, Origin: req.Hostname, State: store.KeyPending, PendingUntil: now.Add(browserCodeTTL),
		}
		bound, err := resolveBrowserDevice(ctx, q, session.AccountID, binding)
		if err != nil {
			return err
		}
		pending.DeviceID, err = prepareBrowserKey(ctx, q, session.AccountID, bound, agent, pending, now)
		if errors.Is(err, errKeyRebind) {
			result.Redirect = browserRedirect(req, "error", errKeyRebind.Code)
		}
		if err != nil {
			return err
		}
		code := Secret()
		if err := q.CreateBrowserCode(ctx, store.BrowserCode{
			Hash: hashSecret(code), AccountID: session.AccountID, DeviceID: pending.DeviceID, ComputerID: computerID,
			JKT: jkt, Challenge: req.Challenge, ExpiresAt: pending.PendingUntil,
		}); err != nil {
			return err
		}
		result, err = s.bindBrowserSession(ctx, q, session, token, pending.DeviceID, now)
		if err != nil {
			return err
		}
		result.Binding, result.Redirect = bound.cookie, browserRedirect(req, oauthCodeField, code)
		return q.AppendSecurityEvent(ctx, store.SecurityEvent{
			AccountID: session.AccountID,
			Type:      "browser_authorized",
			Actor:     accountActorPrefix + pending.DeviceID,
			At:        now,
			Details:   map[string]any{"computer_id": computerID},
		})
	}
	err = s.db.Transaction(ctx, issued)
	if err != nil && !errors.Is(err, errKeyRebind) {
		return BrowserResult{}, err
	}
	return result, err
}

func (s *Service) admitBrowserComputer(
	ctx context.Context, q *store.Queries, hostname, accountID string, now time.Time,
) (string, error) {
	a, err := q.LockAuthAccount(ctx, accountID)
	if err != nil {
		return "", err
	}
	if err := requireActive(a); err != nil {
		return "", err
	}
	computerID, owner, err := q.FindBrowserComputer(ctx, hostname)
	if errors.Is(err, store.ErrAuthNotFound) || (err == nil && owner != accountID) {
		return "", errWrongComputer
	}
	if err != nil {
		return "", err
	}
	subscriptions, err := q.Subscriptions(ctx, accountID)
	if err != nil {
		return "", err
	}
	if plan.Derive(subscriptions, s.cfg.MonthlyPriceID, s.cfg.AnnualPriceID, now) != plan.Pro {
		return "", errPlan
	}
	return computerID, s.writeLimit(accountID)
}

type browserDevice struct {
	id, cookie string
	hash       []byte
}

func resolveBrowserDevice(ctx context.Context, q *store.Queries, accountID, cookie string) (browserDevice, error) {
	hash := hashSecret(accountID + ":" + cookie)
	id, revoked, err := q.FindBrowserDevice(ctx, hash, accountID)
	if err != nil && !errors.Is(err, store.ErrAuthNotFound) {
		return browserDevice{}, err
	}
	if revoked {
		cookie = Secret()
		return browserDevice{cookie: cookie, hash: hashSecret(accountID + ":" + cookie)}, nil
	}
	return browserDevice{id: id, cookie: cookie, hash: hash}, nil
}

func prepareBrowserKey(
	ctx context.Context, q *store.Queries, accountID string, bound browserDevice,
	agent string, key store.AuthDeviceKey, now time.Time,
) (string, error) {
	deviceID := bound.id
	owner, err := q.FindDeviceKey(ctx, key.JKT)
	switch {
	case err == nil:
		if deviceID == "" || owner.DeviceID != deviceID || owner.Origin != key.Origin ||
			owner.DeviceRevoked || owner.State == store.KeyRevoked {
			return "", errKeyRebind
		}
		if owner.State == store.KeyPending {
			return deviceID, q.ExtendPendingKey(ctx, key.JKT, key.PendingUntil)
		}
		return deviceID, nil
	case !errors.Is(err, store.ErrAuthNotFound):
		return "", err
	}
	if deviceID == "" {
		deviceID = "dev_" + Secret()
		d := store.AuthDevice{
			ID: deviceID, AccountID: accountID, Kind: store.BrowserDevice,
			Name: browserName(agent), BrowserBinding: bound.hash,
		}
		if err := q.CreateAuthDevice(ctx, d, now); err != nil {
			return "", err
		}
	}
	key.DeviceID = deviceID
	if err := q.ExpirePendingKey(ctx, deviceID, key.Origin, now); err != nil {
		return "", err
	}
	err = q.CreateDeviceKey(ctx, key, now)
	if errors.Is(err, store.ErrAuthNotFound) {
		return "", errKeyRebind
	}
	return deviceID, err
}

func (s *Service) bindBrowserSession(
	ctx context.Context, q *store.Queries, session store.CloudWebSession, token, deviceID string, now time.Time,
) (BrowserResult, error) {
	var result BrowserResult
	if session.BrowserDeviceID != "" && session.BrowserDeviceID != deviceID {
		token = Secret()
		if err := q.CreateWebSession(
			ctx,
			hashSecret(token),
			session.AccountID,
			now,
			now.Add(WebSessionTTL),
		); err != nil {
			return result, err
		}
		result.WebSession = token
	}
	err := q.BindWebSession(ctx, hashSecret(token), session.AccountID, deviceID)
	return result, err
}

func browserRedirect(req BrowserRequest, key, value string) string {
	query := url.Values{key: {value}, oauthStateField: {req.State}}
	return "https://" + req.Hostname + browserCallbackPath + "?" + query.Encode()
}

func browserName(agent string) string {
	switch {
	case strings.Contains(agent, "iPhone"):
		return "iPhone"
	case strings.Contains(agent, "iPad"):
		return "iPad"
	case strings.Contains(agent, "Android"):
		return "Android browser"
	case strings.Contains(agent, "Firefox"):
		return "Firefox"
	case strings.Contains(agent, "Edg/"):
		return "Microsoft Edge"
	case strings.Contains(agent, "Chrome/"):
		return "Chrome"
	case strings.Contains(agent, "Safari/"):
		return "Safari"
	default:
		return "Browser"
	}
}
