package session

import (
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func (s *sessionStartSpec) applyResolvedReasoningEffort(resolved compozyconfig.ResolvedAgent) error {
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

func preferredACPModel(resolved compozyconfig.ResolvedAgent, explicitOverride bool) string {
	model := strings.TrimSpace(resolved.Model)
	if model == "" {
		return ""
	}
	runtimeProvider := strings.TrimSpace(resolved.RuntimeProvider)
	if runtimeProvider == "" {
		runtimeProvider = strings.TrimSpace(resolved.Provider)
	}
	if resolved.Harness == compozyconfig.ProviderHarnessACP {
		if explicitOverride || runtimeProvider == runtimeProviderCodex || runtimeProvider == runtimeProviderClaude {
			return compozyconfig.ACPModelTransportValue(runtimeProvider, model)
		}
		return ""
	}
	if resolved.Harness != compozyconfig.ProviderHarnessPiACP ||
		(!explicitOverride && resolved.AuthMode != compozyconfig.ProviderAuthModeNativeCLI) {
		return ""
	}
	if runtimeProvider == "" || strings.HasPrefix(model, runtimeProvider+"/") {
		return model
	}
	return runtimeProvider + "/" + model
}
