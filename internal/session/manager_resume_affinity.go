package session

import (
	"context"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
)

const acceptedRouteMissingReason = "accepted_route_missing"

// applyResumeRouteAffinity keeps a resume on the route ACP accepted. The persisted record
// is matched against the primary route and every chain route by provider, auth mode,
// home policy, and command fingerprint (by record, not index). A match relaunches that
// route's command and keeps the native id; no match clears the native id and resumes on
// the primary route through context replay, so a native session is never loaded on a
// foreign account.
func (m *Manager) applyResumeRouteAffinity(
	ctx context.Context,
	spec *sessionStartSpec,
	agentDef compozyconfig.AgentDef,
) {
	record := store.CloneSessionAcceptedRoute(spec.acceptedRoute)
	if spec.startAction != sessionStartActionResume || record == nil {
		return
	}
	primary := FallbackRoute{Provider: spec.provider, Model: spec.model, ReasoningEffort: spec.reasoningEffort}
	candidates := append([]FallbackRoute{primary}, fallbackRoutesForAgent(agentDef)...)
	for index, route := range candidates {
		if m.fallbackRouteMatchesAcceptedRoute(ctx, *spec, agentDef, route, record) {
			spec.command = strings.TrimSpace(route.Command)
			spec.fallbackAttempt = index
			return
		}
	}

	spec.startLogger(m).Info(
		"session.fallback.route_missing",
		"accepted_route.provider", record.Provider,
		"accepted_route.attempt", record.Attempt,
		"accepted_route.command_fingerprint", record.CommandFingerprint,
	)
	spec.acceptedRoute = nil
	spec.command = ""
	spec.fallbackAttempt = 0
	if record.Attempt > 0 {
		// The recorded binding came from a chain route: return to the primary route.
		resetSpecToPrimaryRoute(spec)
	}
	if strings.TrimSpace(spec.acpSessionID) != "" {
		spec.acpSessionID = ""
		spec.resumeReplay = true
		spec.resumeReplayReason = acceptedRouteMissingReason
	}
}

func (m *Manager) fallbackRouteMatchesAcceptedRoute(
	ctx context.Context,
	spec sessionStartSpec,
	agentDef compozyconfig.AgentDef,
	route FallbackRoute,
	record *store.SessionAcceptedRoute,
) bool {
	spec.provider = strings.TrimSpace(route.Provider)
	spec.model = strings.TrimSpace(route.Model)
	spec.command = strings.TrimSpace(route.Command)
	resolved, err := spec.workspace.Config.ResolveSessionAgentWithRuntime(agentDef, compozyconfig.RuntimeOverrides{
		Provider: spec.provider, Model: spec.model, Command: spec.command,
	})
	if err != nil {
		return false
	}
	resolved, err = m.resolveSpawnProviderCommand(ctx, &spec, agentDef, resolved, map[string]bool{spec.sessionID: true})
	if err != nil {
		return false
	}
	return strings.TrimSpace(resolved.Provider) == record.Provider &&
		string(resolved.AuthMode) == record.AuthMode &&
		string(resolved.HomePolicy) == record.HomePolicy &&
		providerCommandFingerprint(resolved.Command) == record.CommandFingerprint
}

// resetSpecToPrimaryRoute replaces a chain route's selection with the session's primary
// route: the operator-selected runtime when one exists, else the agent defaults.
func resetSpecToPrimaryRoute(spec *sessionStartSpec) {
	if selected := spec.selectedRuntime; selected != nil {
		spec.provider = strings.TrimSpace(selected.Provider)
		spec.model = strings.TrimSpace(selected.Model)
		spec.reasoningEffort = strings.TrimSpace(selected.ReasoningEffort)
		return
	}
	spec.provider = ""
	spec.model = ""
	spec.reasoningEffort = ""
}
