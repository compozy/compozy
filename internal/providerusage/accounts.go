package providerusage

import (
	"context"
	"encoding/hex"
	"strings"
	"time"
)

const (
	rateLimitFloor = 5 * time.Minute // shortest cooldown a 429 earns, whatever Retry-After says
	rateLimitCap   = time.Hour       // bound on server-requested and backoff cooldowns
)

// accountKey is one credential identity within a collector family; an unknown identity is never shared.
type accountKey string

// accountState is one identity's shared collection state, guarded by Manager.mu.
type accountState struct {
	strikes  int
	cooldown *accountCooldown
	call     *accountCall
	success  *accountSuccess
}

// accountCall is one in-flight collection that other targets of the identity join.
type accountCall struct {
	done           chan struct{}
	route          string // results only serve targets on the same route
	leaderCanceled bool   // the leader's flight ended first; joiners must collect themselves
	joiners        int    // targets waiting on this call
	snapshot       Snapshot
	outcome        *UsageError
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

// rateLimitUntil applies the floor, doubling per consecutive strike, a longer Retry-After, and the cap.
func rateLimitUntil(now time.Time, retryAfter *time.Time, strikes int) time.Time {
	delay := rateLimitFloor << min(max(strikes-1, 0), 4)
	if retryAfter != nil {
		delay = max(delay, retryAfter.Sub(now))
	}
	return now.Add(min(delay, rateLimitCap))
}

// collectAccount returns a fresh shared success, joins a running collection, or collects once for all.
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
		a, running, ready := s.joinAccountLocked(key, target, intent, maxAge)
		if ready != nil {
			s.mu.Unlock()
			return ready.snapshot, ready.outcome
		}
		if running != nil {
			running.joiners++
			s.mu.Unlock()
			select {
			case <-running.done:
			case <-ctx.Done():
				return Snapshot{}, collectionOutcome(ctx, target.ProviderName, ctx.Err())
			}
			if running.route != target.RouteFingerprint || running.leaderCanceled {
				continue
			}
			return cloneSnapshot(running.snapshot), clonePointer(running.outcome)
		}
		call := &accountCall{done: make(chan struct{}), route: target.RouteFingerprint}
		a.call = call
		s.mu.Unlock()
		snapshot, err := collector.Collect(ctx, target, intent, identity)
		outcome := collectionOutcome(ctx, target.ProviderName, err)
		s.mu.Lock()
		// A drop (disable, shutdown, consent change) replaced the entry, so the result is not kept.
		if s.accounts[key] == a {
			a.call = nil
			s.recordCollectionLocked(a, target, snapshot, outcome)
		}
		call.snapshot, call.outcome, call.leaderCanceled = snapshot, outcome, ctx.Err() != nil
		close(call.done)
		s.mu.Unlock()
		return cloneSnapshot(snapshot), clonePointer(outcome)
	}
}

// joinAccountLocked returns the entry plus either a running call to join or a ready result.
func (s *Manager) joinAccountLocked(
	key accountKey,
	target Target,
	intent Intent,
	maxAge time.Duration,
) (entry *accountState, running, ready *accountCall) {
	entry = s.accounts[key]
	if entry == nil {
		entry = &accountState{}
		s.accounts[key] = entry
	}
	now := s.options.Clock.Now()
	if entry.success != nil && entry.success.route == target.RouteFingerprint && now.Sub(entry.success.at) < maxAge {
		return entry, nil, &accountCall{snapshot: cloneSnapshot(entry.success.snapshot)}
	}
	if entry.call != nil {
		return entry, entry.call, nil
	}
	// One explicit flight per cooldown streak spends the bypass, in the same step as its request.
	if c := s.activeCooldownLocked(key, now); c != nil {
		if intent != IntentExplicit || c.bypassed {
			return entry, nil, &accountCall{outcome: RateLimited(target.ProviderName, c.until)}
		}
		c.bypassed = true
	}
	return entry, nil, nil
}

func (s *Manager) activeCooldownLocked(key accountKey, now time.Time) *accountCooldown {
	if a := s.accounts[key]; a != nil && a.cooldown != nil && now.Before(a.cooldown.until) {
		return a.cooldown
	}
	return nil
}

// recordCollectionLocked adds a strike on 429 and clears the streak on success.
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
		// The message and retry_after both name the effective cooldown end.
		*outcome = *RateLimited(target.ProviderName, until)
		s.options.Logger.Info(
			"provider_usage.cooldown", "provider", target.ProviderID, "until", until, "strikes", a.strikes,
		)
	}
}

// pruneAccountsLocked keeps an identity for an hour after its last cooldown or success so streaks grow.
func (s *Manager) pruneAccountsLocked(now time.Time) {
	for key, a := range s.accounts {
		if a.call == nil &&
			(a.cooldown == nil || now.Sub(a.cooldown.until) > rateLimitCap) &&
			(a.success == nil || now.Sub(a.success.at) > rateLimitCap) {
			delete(s.accounts, key)
		}
	}
}

// forgetAccountsLocked drops one collector family's identities, or all when family is empty.
func (s *Manager) forgetAccountsLocked(family string) {
	for key := range s.accounts {
		if family == "" || key.family() == family {
			delete(s.accounts, key)
		}
	}
}
