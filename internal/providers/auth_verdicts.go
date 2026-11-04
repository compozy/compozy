package providers

import (
	"crypto/sha256"
	"encoding/json/v2"
	"strings"
	"sync"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

// AuthVerdict is the last live native-CLI sign-in verdict for one provider.
type AuthVerdict struct {
	Classification Classification
	CheckedAt      time.Time
}

// AuthVerdicts keeps the last live sign-in verdict per native_cli provider for one daemon process.
// Explicit auth probes and session pre-start probes feed it; settings and provider list read it so a
// verified sign-in is not reported as unverified. A provider config change invalidates its verdict,
// and a daemon restart starts empty. Reads never run a status command.
type AuthVerdicts struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[string]authVerdictEntry
}

type authVerdictEntry struct {
	fingerprint [32]byte
	verdict     AuthVerdict
}

// NewAuthVerdicts constructs an empty daemon-scoped verdict store.
func NewAuthVerdicts() *AuthVerdicts {
	return &AuthVerdicts{now: time.Now, entries: make(map[string]authVerdictEntry)}
}

// Record keeps a live status-command verdict. Results that say nothing about sign-in
// (transient failures, rate limits, unknown output) leave the previous verdict in place.
func (v *AuthVerdicts) Record(providerName string, provider compozyconfig.ProviderConfig, result Classification) {
	if v == nil || provider.EffectiveAuthMode() != compozyconfig.ProviderAuthModeNativeCLI {
		return
	}
	switch result.State {
	case ProviderAuthStateAuthenticated, ProviderAuthStateNeedsLogin, ProviderAuthStatePermissionDenied:
	default:
		return
	}
	name := strings.TrimSpace(providerName)
	fingerprint, ok := authVerdictFingerprint(provider)
	if name == "" || !ok {
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	v.entries[name] = authVerdictEntry{
		fingerprint: fingerprint,
		verdict:     AuthVerdict{Classification: result, CheckedAt: v.now().UTC()},
	}
}

// Lookup returns the provider's last verdict while its configuration is unchanged.
func (v *AuthVerdicts) Lookup(providerName string, provider compozyconfig.ProviderConfig) (AuthVerdict, bool) {
	if v == nil || provider.EffectiveAuthMode() != compozyconfig.ProviderAuthModeNativeCLI {
		return AuthVerdict{}, false
	}
	fingerprint, ok := authVerdictFingerprint(provider)
	if !ok {
		return AuthVerdict{}, false
	}
	name := strings.TrimSpace(providerName)
	v.mu.Lock()
	defer v.mu.Unlock()
	entry, found := v.entries[name]
	if !found {
		return AuthVerdict{}, false
	}
	if entry.fingerprint != fingerprint {
		delete(v.entries, name)
		return AuthVerdict{}, false
	}
	return entry.verdict, true
}

// Apply replaces a declared native_cli "unknown" classification with the last live verdict.
// Any other declared state (for example a missing CLI) is current and wins. The returned time is
// zero when no verdict applies.
func (v *AuthVerdicts) Apply(
	providerName string,
	provider compozyconfig.ProviderConfig,
	declared Classification,
) (Classification, time.Time) {
	if declared.State != ProviderAuthStateUnknown {
		return declared, time.Time{}
	}
	verdict, ok := v.Lookup(providerName, provider)
	if !ok {
		return declared, time.Time{}
	}
	return verdict.Classification, verdict.CheckedAt
}

func authVerdictFingerprint(provider compozyconfig.ProviderConfig) ([32]byte, bool) {
	data, err := json.Marshal(provider, json.Deterministic(true))
	if err != nil {
		return [32]byte{}, false
	}
	return sha256.Sum256(data), true
}
