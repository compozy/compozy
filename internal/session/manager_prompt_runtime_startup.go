package session

import (
	"context"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

// preparePromptRuntimeStartupDefinition keeps the accepted startup prompt for a rebind of
// the same runtime, and reassembles it when the prompt switches provider or model: the
// startup prompt names the runtime it was built for (runtime envelope, situation self),
// and providers without system-prompt delivery receive it inline on their first turn.
func (m *Manager) preparePromptRuntimeStartupDefinition(
	ctx context.Context,
	session *Session,
	spec *sessionStartSpec,
	runtime *sessionStartRuntime,
) error {
	model := strings.TrimSpace(spec.model)
	if model == "" {
		model = strings.TrimSpace(runtime.agent.Model)
	}
	if !startupRuntimeIdentityChanged(session.Info(), runtime.agent.Provider, model) {
		existingDefinition, manifest := session.startupDefinition()
		runtime.startupManifest = manifest
		runtime.agentDef.Prompt = existingDefinition.Prompt
		runtime.agent.Prompt = existingDefinition.Prompt
		return nil
	}
	if err := m.prepareResumeSoul(ctx, spec); err != nil {
		return fmt.Errorf("session: prepare soul for runtime switch of %q: %w", spec.sessionID, err)
	}
	if err := m.assembleAcceptedStartupDefinition(ctx, spec, runtime, m.now()); err != nil {
		return err
	}
	runtime.startupRebuilt = true
	return nil
}

func startupRuntimeIdentityChanged(current *Info, provider string, model string) bool {
	if current == nil {
		return false
	}
	return compozyconfig.CanonicalProviderName(strings.TrimSpace(current.Provider)) !=
		compozyconfig.CanonicalProviderName(strings.TrimSpace(provider)) ||
		strings.TrimSpace(current.Model) != strings.TrimSpace(model)
}
