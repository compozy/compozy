package session

import (
	"fmt"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
)

func (s *sessionStartSpec) applyResolvedReasoningEffort(resolved aghconfig.ResolvedAgent) error {
	if s == nil || strings.TrimSpace(s.reasoningEffort) != "" {
		return nil
	}
	effort := strings.TrimSpace(resolved.ReasoningEffort)
	if effort == "" {
		return nil
	}
	if err := ValidateReasoningEffort(effort); err != nil {
		return fmt.Errorf("session: agent reasoning_effort: %w", err)
	}
	s.reasoningEffort = effort
	return nil
}

func preferredACPModel(resolved aghconfig.ResolvedAgent, explicitOverride bool) string {
	model := strings.TrimSpace(resolved.Model)
	if model == "" {
		return ""
	}
	runtimeProvider := strings.TrimSpace(resolved.RuntimeProvider)
	if runtimeProvider == "" {
		runtimeProvider = strings.TrimSpace(resolved.Provider)
	}
	if resolved.Harness == aghconfig.ProviderHarnessACP {
		if explicitOverride || runtimeProvider == runtimeProviderCodex || runtimeProvider == runtimeProviderClaude {
			return aghconfig.ACPModelTransportValue(runtimeProvider, model)
		}
		return ""
	}
	if resolved.Harness != aghconfig.ProviderHarnessPiACP ||
		(!explicitOverride && resolved.AuthMode != aghconfig.ProviderAuthModeNativeCLI) {
		return ""
	}
	if runtimeProvider == "" || strings.HasPrefix(model, runtimeProvider+"/") {
		return model
	}
	return runtimeProvider + "/" + model
}
