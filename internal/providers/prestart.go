package providers

import (
	"context"
	"hash/fnv"
	"strconv"
	"strings"
	"sync"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	diagcontract "github.com/compozy/agh/internal/diagnosticcontract"
)

const preStartCacheTTL = 30 * time.Second

// PreStartReport carries a structured diagnostic when the pre-start probe fails.
type PreStartReport struct {
	Item *diagcontract.DiagnosticItem
}

type preStartCacheEntry struct {
	report    PreStartReport
	expiresAt time.Time
}

var (
	preStartCacheMu sync.Mutex
	preStartCache   = map[string]preStartCacheEntry{}
)

// PreStart classifies provider-auth readiness before a provider subprocess is spawned.
func PreStart(
	ctx context.Context,
	provider aghconfig.ProviderConfig,
	env *ProbeEnv,
) PreStartReport {
	normalized := env.Normalize()
	key := preStartCacheKey(provider, normalized)
	now := time.Now()

	preStartCacheMu.Lock()
	cached, ok := preStartCache[key]
	if ok && now.Before(cached.expiresAt) {
		preStartCacheMu.Unlock()
		return cached.report
	}
	preStartCacheMu.Unlock()

	report := runPreStart(ctx, provider, &normalized)

	preStartCacheMu.Lock()
	preStartCache[key] = preStartCacheEntry{report: report, expiresAt: now.Add(preStartCacheTTL)}
	preStartCacheMu.Unlock()
	return report
}

// InvalidatePreStartCache clears all cached pre-start probe reports.
func InvalidatePreStartCache() {
	preStartCacheMu.Lock()
	preStartCache = map[string]preStartCacheEntry{}
	preStartCacheMu.Unlock()
}

func runPreStart(
	ctx context.Context,
	provider aghconfig.ProviderConfig,
	env *ProbeEnv,
) PreStartReport {
	if provider.EffectiveAuthMode() == aghconfig.ProviderAuthModeNone {
		return PreStartReport{}
	}
	launchCLI, err := LaunchCommandStatus(provider, env)
	if err != nil {
		item := DiagnosticItem(env.ProviderName, ClassifyError(err))
		return PreStartReport{Item: &item}
	}
	if launchCLI != nil && launchCLI.Command != "" && !launchCLI.Present {
		classification := Classification{
			State:   ProviderAuthStateMissingCLI,
			Code:    diagcontract.CodeProviderCLIMissing,
			Message: "Provider CLI is not installed or not available on PATH.",
			Kind:    ProviderFailureCLIMissing,
			Action:  ProviderFailureActionInstallCLI,
		}
		item := DiagnosticItem(env.ProviderName, classification)
		return PreStartReport{Item: &item}
	}
	classification, err := ClassifyDeclared(ctx, provider, env)
	if err != nil {
		item := DiagnosticItem(env.ProviderName, ClassifyError(err))
		return PreStartReport{Item: &item}
	}
	if classification.Code != "" && classification.State != ProviderAuthStateUnknown {
		item := DiagnosticItem(env.ProviderName, classification)
		return PreStartReport{Item: &item}
	}
	if strings.TrimSpace(provider.AuthStatusCmd) == "" {
		return PreStartReport{}
	}
	result, err := env.RunCommand(ctx, ProviderAuthCommandSpec{
		Command: strings.TrimSpace(provider.AuthStatusCmd),
		Env:     append([]string(nil), env.CommandEnv...),
		Timeout: DefaultProviderAuthCommandTimeout,
		NoTTY:   true,
	})
	if err != nil {
		item := DiagnosticItem(env.ProviderName, ClassifyError(err))
		return PreStartReport{Item: &item}
	}
	probeClassification := ClassifyProbeResult(provider, ProbeOutcome{
		ExitCode: result.ExitCode,
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
	}, env)
	if probeClassification.Code == "" {
		return PreStartReport{}
	}
	item := DiagnosticItem(env.ProviderName, probeClassification)
	return PreStartReport{Item: &item}
}

func preStartCacheKey(provider aghconfig.ProviderConfig, env ProbeEnv) string {
	hash := fnv.New64a()
	parts := []string{
		strings.TrimSpace(env.ProviderName),
		string(provider.EffectiveAuthMode()),
		strings.TrimSpace(provider.Command),
		strings.TrimSpace(provider.AuthStatusCmd),
		strings.TrimSpace(provider.AuthLoginCmd),
	}
	for _, slot := range provider.EffectiveCredentialSlots() {
		parts = append(parts, strings.TrimSpace(slot.SecretRef), strings.TrimSpace(slot.TargetEnv))
	}
	if _, err := hash.Write([]byte(strings.Join(parts, "\x00"))); err != nil {
		return strings.TrimSpace(env.ProviderName)
	}
	return strings.TrimSpace(env.ProviderName) + ":" + strconv.FormatUint(hash.Sum64(), 16)
}
