// SPDX-License-Identifier: FSL-1.1-MIT

package auth

import (
	"cmp"
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

const (
	grantTTL             = 15 * time.Minute
	enrollmentTTL        = 5 * time.Minute
	pollInterval   int32 = 5
	activationPath       = "/activate"
	codeAlphabet         = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
)

func (s *Service) StartDeviceGrant(
	ctx context.Context, input remoteproto.DeviceFlowRequest, location string,
) (remoteproto.DeviceFlowResponse, error) {
	if (input.Provider != "" && !SupportedProvider(input.Provider)) ||
		!validLabel(input.ClientName, 160) || !validLabel(input.ClientPlatform, 80) ||
		!validLabel(input.InstallationID, 128) {
		return remoteproto.DeviceFlowResponse{}, ErrInvalidRequest
	}
	deviceCode := Secret()
	grant := store.DeviceGrant{
		CodeHash: hashSecret(deviceCode), Provider: input.Provider, ClientName: input.ClientName,
		ClientPlatform: input.ClientPlatform, InstallationID: input.InstallationID,
		Location: location, ExpiresAt: s.now().Add(grantTTL),
	}
	for range 4 {
		grant.UserCode = deviceUserCode()
		err := s.db.CreateDeviceGrant(ctx, grant)
		if errors.Is(err, store.ErrAuthNotFound) {
			continue
		}
		if err != nil {
			return remoteproto.DeviceFlowResponse{}, err
		}
		page := s.issuer + activationPath
		return remoteproto.DeviceFlowResponse{
			DeviceCode: deviceCode, UserCode: grant.UserCode, VerificationURI: page,
			VerificationURIComplete: page + "?" + ActivationQuery(grant.UserCode, input.Provider, false),
			Interval:                pollInterval, ExpiresIn: int(grantTTL.Seconds()),
		}, nil
	}
	return remoteproto.DeviceFlowResponse{}, errors.New("could not allocate a device user code")
}

func deviceUserCode() string {
	var code [9]byte
	random := randBytes()
	for i := range code {
		code[i] = codeAlphabet[int(random[i])%len(codeAlphabet)]
	}
	code[4] = '-'
	return string(code[:])
}

func validLabel(value string, limit int) bool {
	return strings.TrimSpace(value) != "" && len(value) <= limit && utf8.ValidString(value) &&
		!strings.ContainsFunc(value, unicode.IsControl)
}

func ActivationQuery(code, provider string, fresh bool) string {
	query := url.Values{"user_code": {code}}
	if provider != "" {
		query.Set(providerField, provider)
	}
	if fresh {
		query.Set("max_age", "0")
	}
	return query.Encode()
}

func normalizeCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) == 8 {
		code = code[:4] + "-" + code[4:]
	}
	if len(code) != 9 || code[4] != '-' {
		return ""
	}
	for i := range len(code) {
		if i != 4 && !strings.ContainsRune(codeAlphabet, rune(code[i])) {
			return ""
		}
	}
	return code
}

func (s *Service) FindActivation(ctx context.Context, browser, code string) (store.DeviceGrant, error) {
	now := s.now()
	if err := s.codes.check(browser, now, false); err != nil {
		return store.DeviceGrant{}, err
	}
	normalized := normalizeCode(code)
	grant, err := store.DeviceGrant{}, store.ErrAuthNotFound
	if normalized != "" {
		grant, err = s.db.FindDeviceGrant(ctx, normalized)
	}
	if errors.Is(err, store.ErrAuthNotFound) {
		if err := s.codes.check(browser, now, true); err != nil {
			return store.DeviceGrant{}, err
		}
		return store.DeviceGrant{}, errWrongCode
	}
	if err != nil {
		return store.DeviceGrant{}, err
	}
	return grant, grantAvailable(grant, now)
}

func grantAvailable(g store.DeviceGrant, now time.Time) error {
	switch {
	case g.State == stateAuthorized || g.State == stateRedeemed:
		return errCodeUsed
	case g.State == stateCancelled:
		return errDenied
	case g.State == stateExpired || !now.Before(g.ExpiresAt):
		return errExpired
	default:
		return nil
	}
}

type codeWindow struct {
	next     *codeWindow
	browser  string
	failures int
	until    time.Time
}

type codeEntries struct {
	mu             sync.Mutex
	windows        map[string]*codeWindow
	oldest, newest *codeWindow
}

func (c *codeEntries) check(browser string, now time.Time, failed bool) error {
	const window = 15 * time.Minute
	const maxBrowsers = 16_384
	c.mu.Lock()
	defer c.mu.Unlock()

	for c.oldest != nil && !now.Before(c.oldest.until) {
		c.evictOldest()
	}
	if entry := c.windows[browser]; entry != nil {
		if entry.failures >= 5 {
			return &Failure{Status: http.StatusTooManyRequests, Code: "rate_limited", Message: "Too many attempts.",
				RetryAfter: int((entry.until.Sub(now) + time.Second - 1) / time.Second)}
		}
		if failed {
			entry.failures++
		}
		return nil
	}
	if failed {
		if len(c.windows) >= maxBrowsers {
			c.evictOldest()
		}
		if c.windows == nil {
			c.windows = make(map[string]*codeWindow)
		}
		entry := &codeWindow{browser: browser, failures: 1, until: now.Add(window)}
		if c.newest == nil {
			c.oldest = entry
		} else {
			c.newest.next = entry
		}
		c.newest = entry
		c.windows[browser] = entry
	}
	return nil
}

func (c *codeEntries) evictOldest() {
	delete(c.windows, c.oldest.browser)
	c.oldest = c.oldest.next
	if c.oldest == nil {
		c.newest = nil
	}
}

func (s *Service) CancelDeviceGrant(ctx context.Context, code string) error {
	if !ValidSecret(code) {
		return ErrInvalidRequest
	}
	return s.db.Transaction(ctx, func(q *store.Queries) error {
		g, err := q.LockDeviceGrant(ctx, hashSecret(code))
		if errors.Is(err, store.ErrAuthNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		now := s.now()
		if !now.Before(g.ExpiresAt) || (g.State != statePending && g.State != stateAuthorized) {
			return nil
		}
		if err := q.CancelGrant(ctx, g.CodeHash, errDenied.Code, true, now); err != nil {
			return err
		}
		return q.AppendSecurityEvent(ctx, store.SecurityEvent{
			AccountID: g.AccountID, Type: "sign_in_canceled", Actor: systemActor, At: now,
		})
	})
}

func (s *Service) PollDeviceGrant(ctx context.Context, code string) (remoteproto.DeviceFlowTokenResponse, error) {
	var result remoteproto.DeviceFlowTokenResponse
	if !ValidSecret(code) {
		return result, errExpired
	}
	var outcome error
	err := s.db.Transaction(ctx, func(q *store.Queries) error {
		var err error
		result, outcome, err = s.pollDeviceGrant(ctx, q, hashSecret(code), s.now())
		return err
	})
	return result, cmp.Or(err, outcome)
}

// The protocol outcome is distinct from a storage error so poll and expiry writes commit.
func (s *Service) pollDeviceGrant(
	ctx context.Context, q *store.Queries, hash []byte, now time.Time,
) (remoteproto.DeviceFlowTokenResponse, error, error) {
	var empty remoteproto.DeviceFlowTokenResponse
	g, err := q.LockDeviceGrant(ctx, hash)
	switch {
	case errors.Is(err, store.ErrAuthNotFound):
		return empty, errExpired, nil
	case err != nil:
		return empty, nil, err
	case g.State == stateCancelled:
		return empty, deniedGrant(g.FailureCode), nil
	case g.State == stateExpired || g.State == stateRedeemed:
		return empty, errExpired, nil
	case !now.Before(g.ExpiresAt):
		return empty, errExpired, q.ExpireGrant(ctx, hash)
	}
	last := cmp.Or(g.LastPoll, g.ExpiresAt.Add(-grantTTL))
	if now.Before(last.Add(time.Duration(g.Interval) * time.Second)) {
		interval := min(g.Interval+pollInterval, int32(grantTTL/time.Second))
		return empty, errSlowDown, q.RecordGrantPoll(ctx, hash, interval, now)
	}
	if err := q.RecordGrantPoll(ctx, hash, g.Interval, now); err != nil {
		return empty, nil, err
	}
	if g.State != stateAuthorized {
		return empty, errPending, nil
	}
	a, err := q.LockAuthAccount(ctx, g.AccountID)
	if err != nil {
		return empty, nil, err
	}
	if err := requireActive(a); err != nil {
		return empty, err, nil
	}
	token := "enr_" + Secret()
	err = q.RedeemGrant(ctx, g, hashSecret(token), now, now.Add(enrollmentTTL))
	return remoteproto.DeviceFlowTokenResponse{
		EnrollmentToken: token, AccountID: a.ID, Email: a.Email, NewAccount: g.NewAccount,
	}, nil, err
}

func deniedGrant(code string) error {
	switch code {
	case errAccountSuspended.Code:
		return errAccountSuspended
	case errAccountDeleting.Code:
		return errAccountDeleting
	case errIdentityConflict.Code:
		return errIdentityConflict
	default:
		return errDenied
	}
}
