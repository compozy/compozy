package providers

import (
	"crypto/sha256"
	"path/filepath"
	"runtime"
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
// invalidates its verdict, a sign-in verified elsewhere drops a needs_login verdict, and a daemon
// restart starts empty. Reads never run a status command.
type AuthVerdicts struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[string]authVerdictEntry
	// signedIn is the newest sign-in another source verified per provider; a needs_login probe that
	// started before it is outdated and cannot record.
	signedIn map[string]time.Time
}

type authVerdictEntry struct {
	fingerprint [32]byte
	startedAt   time.Time
	verdict     AuthVerdict
	dropped     bool
}

// NewAuthVerdicts constructs an empty daemon-scoped verdict store.
func NewAuthVerdicts() *AuthVerdicts {
	return &AuthVerdicts{
		now:      time.Now,
		entries:  make(map[string]authVerdictEntry),
		signedIn: make(map[string]time.Time),
	}
}

// Record keeps a live status verdict from a probe that ran with env and started at startedAt.
// Results that say nothing about sign-in (transient failures, rate limits, unknown output) leave
// the previous verdict in place, and a probe that started before the stored one cannot overwrite it.
func (v *AuthVerdicts) Record(
	providerName string,
	provider compozyconfig.ProviderConfig,
	env []string,
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
	if result.State == ProviderAuthStateNeedsLogin && v.signedIn[name].After(startedAt) {
		return
	}
	v.entries[name] = authVerdictEntry{
		fingerprint: authVerdictFingerprint(provider, env),
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
		delete(v.signedIn, strings.TrimSpace(name))
	}
}

// DropNeedsLogin records that another source, such as a published usage collection, verified the
// provider's sign-in at observedAt. A needs_login probe that started before observedAt can no longer
// record, even when it is still running, and a stored needs_login verdict from such a probe is
// dropped. A verdict whose probe started after observedAt is newer evidence and stays. It reports
// whether a stored verdict was dropped, so the caller can re-check the provider once.
func (v *AuthVerdicts) DropNeedsLogin(providerName string, observedAt time.Time) bool {
	if v == nil {
		return false
	}
	name := strings.TrimSpace(providerName)
	if name == "" {
		return false
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if observedAt.After(v.signedIn[name]) {
		v.signedIn[name] = observedAt
	}
	entry, found := v.entries[name]
	if !found || entry.dropped || entry.verdict.Classification.State != ProviderAuthStateNeedsLogin ||
		!entry.startedAt.Before(observedAt) {
		return false
	}
	v.entries[name] = authVerdictEntry{startedAt: entry.startedAt, dropped: true}
	return true
}

// Lookup returns the provider's last verdict while its sign-in identity and the auth home it was
// checked against match the reader's env. A mismatched verdict is dropped; its probe start time
// stays so a probe that started earlier cannot record a verdict afterwards.
func (v *AuthVerdicts) Lookup(
	providerName string,
	provider compozyconfig.ProviderConfig,
	env []string,
) (AuthVerdict, bool) {
	if v == nil || provider.EffectiveAuthMode() != compozyconfig.ProviderAuthModeNativeCLI {
		return AuthVerdict{}, false
	}
	name := strings.TrimSpace(providerName)
	fingerprint := authVerdictFingerprint(provider, env)
	v.mu.Lock()
	defer v.mu.Unlock()
	entry, found := v.entries[name]
	if !found || entry.dropped {
		return AuthVerdict{}, false
	}
	if entry.fingerprint != fingerprint {
		v.entries[name] = authVerdictEntry{startedAt: entry.startedAt, dropped: true}
		return AuthVerdict{}, false
	}
	return entry.verdict, true
}

// Known reports whether a verdict exists for the provider's current sign-in identity. Unlike
// Lookup it never drops a mismatched entry, so a caller holding stale settings cannot erase it.
func (v *AuthVerdicts) Known(providerName string, provider compozyconfig.ProviderConfig, env []string) bool {
	if v == nil || provider.EffectiveAuthMode() != compozyconfig.ProviderAuthModeNativeCLI {
		return false
	}
	fingerprint := authVerdictFingerprint(provider, env)
	v.mu.Lock()
	defer v.mu.Unlock()
	entry, found := v.entries[strings.TrimSpace(providerName)]
	return found && !entry.dropped && entry.fingerprint == fingerprint
}

// Apply replaces the declared native_cli unverified fallback with the last live verdict for env.
// Any other declared result (a missing CLI, a resolver error) is current and wins. The returned
// time is zero when no verdict applies.
func (v *AuthVerdicts) Apply(
	providerName string,
	provider compozyconfig.ProviderConfig,
	env []string,
	declared Classification,
) (Classification, time.Time) {
	if !declared.Unverified {
		return declared, time.Time{}
	}
	verdict, ok := v.Lookup(providerName, provider, env)
	if !ok {
		return declared, time.Time{}
	}
	return verdict.Classification, verdict.CheckedAt
}

// AuthenticationRouteFingerprint fences recovery proofs to the configured launch and isolation route.
// Callers must exclude per-session environment overrides, which a provider-level proof cannot verify.
func AuthenticationRouteFingerprint(providerName string, provider compozyconfig.ProviderConfig) string {
	provider.RuntimeProvider = provider.RuntimeProviderName(providerName)
	provider.Command = strings.TrimSpace(provider.Command)
	identity := authVerdictFingerprint(provider, nil)
	return compozyconfig.CommandFingerprint(strings.Join([]string{
		provider.Command,
		string(identity[:]),
		strings.TrimSpace(provider.Transport),
		strings.TrimSpace(provider.BaseURL),
	}, "\x00"))
}

// authVerdictHomeKeys are the environment variables that choose where a native CLI keeps its
// sign-in state.
var authVerdictHomeKeys = []string{
	"HOME", "USERPROFILE", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "PROVIDER_HOME",
	"CLAUDE_CONFIG_DIR", "CLAUDE_SECURESTORAGE_CONFIG_DIR", "CODEX_HOME", "PROVIDER_CODEX_HOME",
	"OPENCODE_CONFIG_DIR", "PI_CODING_AGENT_DIR", "KIMI_CODE_HOME",
}

// authVerdictFingerprint hashes the provider's sign-in identity and the auth home env it was
// checked with. A session start prepares the launch command with a resolved absolute executable
// and carries only runtime fields, so the identity uses the executable's base name and the
// auth-relevant fields that both the configured and the prepared provider share. Only the hash is
// stored.
func authVerdictFingerprint(provider compozyconfig.ProviderConfig, env []string) [32]byte {
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
	parts = append(parts, "\x02")
	parts = append(parts, authVerdictHomeEnv(env, runtime.GOOS == "windows")...)
	return sha256.Sum256([]byte(strings.Join(parts, "\x00")))
}

// authVerdictHomeEnv returns the auth home assignments in authVerdictHomeKeys order. Like
// subprocess.LookupEnv, the last assignment wins and Windows keys match case-insensitively.
func authVerdictHomeEnv(env []string, caseInsensitive bool) []string {
	values := make([]string, 0, len(authVerdictHomeKeys))
	for _, key := range authVerdictHomeKeys {
		value, found := "", false
		for _, entry := range env {
			candidate, candidateValue, ok := strings.Cut(entry, "=")
			if ok && (candidate == key || caseInsensitive && strings.EqualFold(candidate, key)) {
				value, found = candidateValue, true
			}
		}
		if found {
			values = append(values, key+"="+value)
		}
	}
	return values
}
