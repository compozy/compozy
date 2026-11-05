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

// accountState is the rate-limit state of one credential identity, shared by every profile
// target that collects through it. Guarded by Manager.mu.
type accountState struct {
	strikes  int
	cooldown *accountCooldown
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

// collectAccount runs one real collection, unless reading the identity failed, and feeds its
// outcome into the identity's backoff.
func (s *Manager) collectAccount(
	ctx context.Context,
	profile, id string,
	state *targetState,
	collector Collector,
	target Target,
	intent Intent,
	identity CredentialIdentity,
	identityErr error,
) (Snapshot, *UsageError) {
	if identityErr != nil {
		return Snapshot{}, collectionOutcome(ctx, target.ProviderName, identityErr)
	}
	snapshot, err := collector.Collect(ctx, target, intent, identity)
	outcome := collectionOutcome(ctx, target.ProviderName, err)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.currentLocked(profile, id, state) {
		s.recordCollectionLocked(newAccountKey(profile, target, identity), id, outcome)
	}
	return snapshot, outcome
}

// activeCooldownLocked returns the identity's cooldown when it is still running at now.
func (s *Manager) activeCooldownLocked(key accountKey, now time.Time) *accountCooldown {
	if a := s.accounts[key]; a != nil && a.cooldown != nil && now.Before(a.cooldown.until) {
		return a.cooldown
	}
	return nil
}

// recordCollectionLocked updates an identity's backoff from one real collection response.
// A 429 adds a strike and rewrites the outcome's RetryAfter to the effective cooldown end;
// success clears the streak and its explicit bypass. Other outcomes leave the streak as is.
func (s *Manager) recordCollectionLocked(key accountKey, providerID string, outcome *UsageError) {
	switch {
	case outcome == nil:
		delete(s.accounts, key)
	case outcome.Code == ErrorCodeProviderRateLimited:
		a := s.accounts[key]
		if a == nil {
			a = &accountState{}
			s.accounts[key] = a
		}
		a.strikes++
		now := s.options.Clock.Now()
		until := rateLimitUntil(now, outcome.RetryAfter, a.strikes)
		a.cooldown = &accountCooldown{until: until, bypassed: a.cooldown != nil && a.cooldown.bypassed}
		outcome.RetryAfter = &until
		s.options.Logger.Info("provider_usage.cooldown", "provider", providerID, "until", until, "strikes", a.strikes)
	}
}

// pruneAccountsLocked forgets identities whose last cooldown ended over an hour ago; an account
// still in use re-enters its streak on the next 429 within that hour.
func (s *Manager) pruneAccountsLocked(now time.Time) {
	for key, a := range s.accounts {
		if a.cooldown == nil || now.Sub(a.cooldown.until) > rateLimitCap {
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
