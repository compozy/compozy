package providerusage

import (
	"context"
	"encoding/hex"
	"strings"
	"time"
)

const (
	// rateLimitFloor is the shortest cooldown a 429 earns, whatever Retry-After says.
	rateLimitFloor = 5 * time.Minute
	// rateLimitCap bounds both server-requested and backoff cooldowns.
	rateLimitCap = time.Hour
)

// accountKey identifies one credential identity inside one collector family. Targets without a
// known identity get a key of their own, so they never share state with another target.
type accountKey string

// accountState is the collection state of one credential identity, shared by every profile
// target that collects through it: the rate-limit streak, the in-flight request, and the last
// success. Guarded by Manager.mu.
type accountState struct {
	strikes  int
	cooldown *accountCooldown
	call     *accountCall
	success  *accountSuccess
}

// accountCall is one in-flight collection that other targets of the identity join.
type accountCall struct {
	done     chan struct{}
	ctx      context.Context // leader flight; joiners retry when it was canceled
	route    string          // results only serve targets on the same route
	snapshot Snapshot
	outcome  *UsageError
}

type accountSuccess struct {
	snapshot Snapshot
	route    string
	at       time.Time
}

type accountCooldown struct {
	until    time.Time
	bypassed bool
}

func newAccountKey(profile string, target Target, identity CredentialIdentity) accountKey {
	family := target.Collector()
	switch {
	case identity.AccountID != "":
		return accountKey(family + "\x00a\x00" + identity.AccountID)
	case identity != (CredentialIdentity{}):
		return accountKey(family + "\x00d\x00" + hex.EncodeToString(identity.Digest[:]))
	default:
		return accountKey(family + "\x00t\x00" + profile + "\x00" + target.ProviderID)
	}
}

func (k accountKey) family() string {
	family, _, _ := strings.Cut(string(k), "\x00")
	return family
}

// rateLimitUntil applies the floor, the per-identity exponential backoff, a longer server
// Retry-After, and the cap. strikes counts consecutive 429s including the current one.
func rateLimitUntil(now time.Time, retryAfter *time.Time, strikes int) time.Time {
	delay := rateLimitFloor << min(max(strikes-1, 0), 4)
	if retryAfter != nil {
		delay = max(delay, retryAfter.Sub(now))
	}
	return now.Add(min(delay, rateLimitCap))
}

// collectAccount returns the identity's collection for one target: a success younger than
// maxAge, the result of a collection another target already started, or a new collection that
// later targets join. A cooldown that began after admission answers without a request.
func (s *Manager) collectAccount(
	ctx context.Context,
	profile string,
	collector Collector,
	target Target,
	intent Intent,
	identity CredentialIdentity,
	identityErr error,
	maxAge time.Duration,
) (Snapshot, *UsageError) {
	if identityErr != nil {
		return Snapshot{}, collectionOutcome(ctx, target.ProviderName, identityErr)
	}
	key := newAccountKey(profile, target, identity)
	for {
		s.mu.Lock()
		a, call, done := s.joinAccountLocked(key, target, intent, maxAge)
		if done != nil {
			s.mu.Unlock()
			return done.snapshot, done.outcome
		}
		if call != nil {
			s.mu.Unlock()
			select {
			case <-call.done:
			case <-ctx.Done():
				return Snapshot{}, collectionOutcome(ctx, target.ProviderName, ctx.Err())
			}
			if call.route != target.RouteFingerprint || (call.ctx.Err() != nil && ctx.Err() == nil) {
				continue // another route's or a canceled leader's result does not serve this target
			}
			return cloneSnapshot(call.snapshot), clonePointer(call.outcome)
		}
		call = &accountCall{done: make(chan struct{}), ctx: ctx, route: target.RouteFingerprint}
		a.call = call
		s.mu.Unlock()
		snapshot, err := collector.Collect(ctx, target, intent, identity)
		outcome := collectionOutcome(ctx, target.ProviderName, err)
		s.mu.Lock()
		// A drop (disable, shutdown, consent change) replaced the entry; its result is not stored.
		if s.accounts[key] == a {
			a.call = nil
			s.recordCollectionLocked(a, target, snapshot, outcome)
		}
		call.snapshot, call.outcome = snapshot, outcome
		close(call.done)
		s.mu.Unlock()
		return cloneSnapshot(snapshot), clonePointer(outcome)
	}
}

// joinAccountLocked returns a ready result, an in-flight call to join, or the entry this target
// should start a call on.
func (s *Manager) joinAccountLocked(
	key accountKey,
	target Target,
	intent Intent,
	maxAge time.Duration,
) (*accountState, *accountCall, *accountCall) {
	a := s.accounts[key]
	if a == nil {
		a = &accountState{}
		s.accounts[key] = a
	}
	now := s.options.Clock.Now()
	if a.success != nil && a.success.route == target.RouteFingerprint && now.Sub(a.success.at) < maxAge {
		return a, nil, &accountCall{snapshot: cloneSnapshot(a.success.snapshot)}
	}
	if a.call != nil {
		return a, a.call, nil
	}
	// Admission saw no cooldown, or granted this explicit flight the identity's bypass.
	if c := s.activeCooldownLocked(key, now); c != nil && (intent != IntentExplicit || !c.bypassed) {
		return a, nil, &accountCall{outcome: RateLimited(target.ProviderName, c.until)}
	}
	return a, nil, nil
}

// activeCooldownLocked returns the identity's cooldown when it is still running at now.
func (s *Manager) activeCooldownLocked(key accountKey, now time.Time) *accountCooldown {
	if a := s.accounts[key]; a != nil && a.cooldown != nil && now.Before(a.cooldown.until) {
		return a.cooldown
	}
	return nil
}

// recordCollectionLocked updates an identity from one real collection response. A 429 adds a
// strike and rewrites the outcome's RetryAfter to the effective cooldown end; success clears the
// streak and its explicit bypass and keeps the data for other targets. Other outcomes leave the
// streak as is.
func (s *Manager) recordCollectionLocked(a *accountState, target Target, snapshot Snapshot, outcome *UsageError) {
	now := s.options.Clock.Now()
	switch {
	case outcome == nil:
		a.strikes, a.cooldown = 0, nil
		a.success = &accountSuccess{snapshot: cloneSnapshot(snapshot), route: target.RouteFingerprint, at: now}
	case outcome.Code == ErrorCodeProviderRateLimited:
		a.strikes++
		a.success = nil
		until := rateLimitUntil(now, outcome.RetryAfter, a.strikes)
		a.cooldown = &accountCooldown{until: until, bypassed: a.cooldown != nil && a.cooldown.bypassed}
		outcome.RetryAfter = &until
		s.options.Logger.Info(
			"provider_usage.cooldown", "provider", target.ProviderID, "until", until, "strikes", a.strikes,
		)
	}
}

// pruneAccountsLocked forgets idle identities whose last cooldown and success are over an hour
// old; an account still in use re-enters its streak on the next 429 within that hour.
func (s *Manager) pruneAccountsLocked(now time.Time) {
	for key, a := range s.accounts {
		if a.call == nil &&
			(a.cooldown == nil || now.Sub(a.cooldown.until) > rateLimitCap) &&
			(a.success == nil || now.Sub(a.success.at) > rateLimitCap) {
			delete(s.accounts, key)
		}
	}
}

// forgetAccountsLocked drops every identity of one collector family, or all when family is empty.
func (s *Manager) forgetAccountsLocked(family string) {
	for key := range s.accounts {
		if family == "" || key.family() == family {
			delete(s.accounts, key)
		}
	}
}
