package providers

import (
	"crypto/sha256"
	"path/filepath"
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
// verified sign-in is not reported as unverified. A change to the provider's sign-in identity
// invalidates its verdict, and a daemon restart starts empty. Reads never run a status command.
type AuthVerdicts struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[string]authVerdictEntry
}

type authVerdictEntry struct {
	fingerprint [32]byte
	startedAt   time.Time
	verdict     AuthVerdict
}

// NewAuthVerdicts constructs an empty daemon-scoped verdict store.
func NewAuthVerdicts() *AuthVerdicts {
	return &AuthVerdicts{now: time.Now, entries: make(map[string]authVerdictEntry)}
}

// Record keeps a live status verdict from a probe that started at startedAt. Results that say
// nothing about sign-in (transient failures, rate limits, unknown output) leave the previous
// verdict in place, and a probe that started before the stored one cannot overwrite it.
func (v *AuthVerdicts) Record(
	providerName string,
	provider compozyconfig.ProviderConfig,
	result Classification,
	startedAt time.Time,
) {
	if v == nil || provider.EffectiveAuthMode() != compozyconfig.ProviderAuthModeNativeCLI {
		return
	}
	switch result.State {
	case ProviderAuthStateAuthenticated, ProviderAuthStateNeedsLogin, ProviderAuthStatePermissionDenied:
	default:
		return
	}
	name := strings.TrimSpace(providerName)
	if name == "" {
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if previous, ok := v.entries[name]; ok && previous.startedAt.After(startedAt) {
		return
	}
	v.entries[name] = authVerdictEntry{
		fingerprint: authVerdictFingerprint(provider),
		startedAt:   startedAt,
		verdict:     AuthVerdict{Classification: result, CheckedAt: v.now().UTC()},
	}
}

// Forget drops the verdicts of providers whose configuration was written or applied.
func (v *AuthVerdicts) Forget(providerNames ...string) {
	if v == nil {
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	for _, name := range providerNames {
		delete(v.entries, strings.TrimSpace(name))
	}
}

// Lookup returns the provider's last verdict while its sign-in identity is unchanged.
func (v *AuthVerdicts) Lookup(providerName string, provider compozyconfig.ProviderConfig) (AuthVerdict, bool) {
	if v == nil || provider.EffectiveAuthMode() != compozyconfig.ProviderAuthModeNativeCLI {
		return AuthVerdict{}, false
	}
	name := strings.TrimSpace(providerName)
	fingerprint := authVerdictFingerprint(provider)
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

// Known reports whether a verdict exists for the provider's current sign-in identity. Unlike
// Lookup it never drops a mismatched entry, so a caller holding stale settings cannot erase it.
func (v *AuthVerdicts) Known(providerName string, provider compozyconfig.ProviderConfig) bool {
	if v == nil || provider.EffectiveAuthMode() != compozyconfig.ProviderAuthModeNativeCLI {
		return false
	}
	fingerprint := authVerdictFingerprint(provider)
	v.mu.Lock()
	defer v.mu.Unlock()
	entry, found := v.entries[strings.TrimSpace(providerName)]
	return found && entry.fingerprint == fingerprint
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

// authVerdictFingerprint hashes the provider's sign-in identity. A session start prepares the
// launch command with a resolved absolute executable and carries only runtime fields, so the
// identity uses the executable's base name and the auth-relevant fields that both the configured
// and the prepared provider share.
func authVerdictFingerprint(provider compozyconfig.ProviderConfig) [32]byte {
	parts := []string{
		string(provider.EffectiveAuthMode()),
		string(provider.EffectiveEnvPolicy()),
		string(provider.EffectiveHomePolicy()),
		string(provider.EffectiveHarness()),
		strings.TrimSpace(provider.RuntimeProvider),
		strings.TrimSpace(provider.AuthStatusCmd),
		strings.TrimSpace(provider.AuthACPMethod),
	}
	if parsed, err := compozyconfig.ParseLaunchCommand(provider.Command); err == nil {
		parts = append(parts, parsed.Environment...)
		parts = append(parts, "\x01", filepath.Base(parsed.Executable))
		parts = append(parts, parsed.Args...)
	} else {
		parts = append(parts, "\x01", strings.TrimSpace(provider.Command))
	}
	return sha256.Sum256([]byte(strings.Join(parts, "\x00")))
}
