package session

import (
	"strings"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
)

// acceptedRouteRecord describes the binding ACP accepted without its command text.
func acceptedRouteRecord(attempt int, resolved compozyconfig.ResolvedAgent, model string) *store.SessionAcceptedRoute {
	if strings.TrimSpace(model) == "" {
		model = resolved.Model
	}
	return store.CloneSessionAcceptedRoute(&store.SessionAcceptedRoute{
		Attempt:            attempt,
		Provider:           resolved.Provider,
		Model:              model,
		AuthMode:           string(resolved.AuthMode),
		HomePolicy:         string(resolved.HomePolicy),
		CommandFingerprint: providerCommandFingerprint(resolved.Command),
	})
}

// commitAcceptedRoute records the accepted binding; the caller persists it in the same
// meta write as the ACP session id it belongs to.
func (s *Session) commitAcceptedRoute(route *store.SessionAcceptedRoute, command string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.acceptedRoute = store.CloneSessionAcceptedRoute(route)
	s.acceptedCommand = strings.TrimSpace(command)
	s.mu.Unlock()
}

// acceptedRouteCommand returns the explicit command of the accepted route ("" = inherit).
func (s *Session) acceptedRouteCommand() string {
	if s == nil {
		return ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.acceptedCommand
}

// applyStartAttemptRoute points a starting session at one eager fallback attempt so the
// launch, its logs, and a later failed start describe the attempted route.
func (s *Session) applyStartAttemptRoute(spec *sessionStartSpec, runtime sessionStartRuntime) {
	if s == nil || spec == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Provider = strings.TrimSpace(runtime.agent.Provider)
	s.Model = strings.TrimSpace(runtime.agent.Model)
	s.ReasoningEffort = strings.TrimSpace(spec.reasoningEffort)
	s.Speed = spec.speed
	s.ACPOptions = acp.CloneSessionConfigOptionSelections(spec.acpOptions)
	s.effectiveProviderAuthMode = runtime.agent.AuthMode
	s.providerHomePolicy = runtime.agent.HomePolicy
	s.providerRoute = runtime.agent
	s.agentDef = compozyconfig.CloneAgentDef(runtime.agentDef)
	s.startupManifest = acp.CloneStartupManifest(runtime.startupManifest)
}
