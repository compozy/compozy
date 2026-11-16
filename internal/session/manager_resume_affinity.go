package session

import (
	"context"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
)

const (
	acceptedRouteMissingReason = "accepted_route_missing"
	// agyProviderName scopes the unverified-native-id repair to the Antigravity route.
	agyProviderName = "agy"
)

// applyResumeRouteAffinity keeps a resume on the route ACP accepted. The persisted record
// is matched against the primary route and every chain route by provider, auth mode,
// home policy, and command fingerprint (by record, not index). A match relaunches that
// route's command and keeps the native id; no match clears the native id and resumes on
// the primary route through context replay, so a native session is never loaded on a
// foreign account. An AGY record without any accepted route uses the same replay instead
// of assuming its native id belongs to the current CLI store.
func (m *Manager) applyResumeRouteAffinity(
	ctx context.Context,
	spec *sessionStartSpec,
	agentDef compozyconfig.AgentDef,
) {
	record := store.CloneSessionAcceptedRoute(spec.acceptedRoute)
	if spec.startAction != sessionStartActionResume {
		return
	}
	if record == nil {
		m.applyUnverifiedNativeRoute(spec)
		return
	}
	primary := FallbackRoute{Provider: spec.provider, Model: spec.model, ReasoningEffort: spec.reasoningEffort}
	candidates := append([]FallbackRoute{primary}, fallbackRoutesForAgent(agentDef)...)
	if spec.sessionType == SessionTypeCoordinator {
		role := spec.workspace.Config.Roles.Coordinator
		primary = FallbackRoute{Provider: role.Provider, Model: role.Model, ReasoningEffort: role.ReasoningEffort,
			Speed: role.Speed, ACPOptions: ACPOptionSelectionsFromConfig(role.ACPOptions)}
		candidates = append(
			[]FallbackRoute{primary},
			fallbackRoutesForChain(spec.workspace.Config.Roles.Coordinator.FallbackChain)...)
	}
	for index, route := range candidates {
		if m.fallbackRouteMatchesAcceptedRoute(ctx, spec, agentDef, route, record) {
			spec.command = strings.TrimSpace(route.Command)
			spec.sessionEnv = compozyconfig.MergeSessionEnv(nil, route.SessionEnv)
			spec.fallbackAttempt = index
			if spec.sessionType == SessionTypeCoordinator {
				spec.fallbackAttempt = record.Attempt
			}
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
	spec.sessionEnv = nil
	spec.fallbackAttempt = 0
	switch {
	case spec.sessionType == SessionTypeCoordinator:
		spec.provider, spec.model, spec.reasoningEffort = primary.Provider, primary.Model, primary.ReasoningEffort
		spec.speed, spec.acpOptions = primary.Speed, primary.ACPOptions
	case record.Attempt > 0:
		// The recorded binding came from a chain route: return to the primary route.
		resetSpecToPrimaryRoute(spec)
	default:
		applyResumeRuntimeOptions(spec)
	}
	clearResumeNativeRoute(spec)
}

// applyUnverifiedNativeRoute keeps a pre-upgrade AGY record without accepted-route metadata on
// replay: its native id proves no membership in the CLI's store. Other providers keep the
// existing restore path, so this repair stays scoped to the affected route.
func (m *Manager) applyUnverifiedNativeRoute(spec *sessionStartSpec) {
	if strings.TrimSpace(spec.acpSessionID) == "" ||
		compozyconfig.CanonicalProviderName(spec.provider) != agyProviderName {
		return
	}
	spec.startLogger(m).Info(
		"session.resume.native_route_unverified",
		"provider", agyProviderName,
	)
	applyResumeRuntimeOptions(spec)
	clearResumeNativeRoute(spec)
}

// applyResumeRuntimeOptions reconstructs ACP options only on the AGY route and only from a
// genuine selected-runtime witness. A record without one is retained verbatim: storage cannot
// tell the old approve-all mode from an explicit selection of the same option, so discarding
// either would lose operator intent. The native bind validates the retained controls and
// refuses an incompatible value instead of guessing provenance. Other providers keep their
// existing restore path.
func applyResumeRuntimeOptions(spec *sessionStartSpec) {
	if spec == nil || compozyconfig.CanonicalProviderName(spec.provider) != agyProviderName {
		return
	}
	if selected := spec.selectedRuntime; selected != nil &&
		resumeSelectionAppliesToProvider(selected.Provider, spec.provider) {
		spec.acpOptions = acp.CloneSessionConfigOptionSelections(selected.ACPOptions)
	}
}

// resumeUsesRetainedOptions reports that the resumed AGY route replays recorded ACP options
// with no selected-runtime witness to reconstruct them from.
func resumeUsesRetainedOptions(spec *sessionStartSpec) bool {
	if spec == nil || compozyconfig.CanonicalProviderName(spec.provider) != agyProviderName ||
		len(spec.acpOptions) == 0 {
		return false
	}
	selected := spec.selectedRuntime
	return selected == nil || !resumeSelectionAppliesToProvider(selected.Provider, spec.provider)
}

// resumeSelectionAppliesToProvider keeps another provider's selected options off this route.
func resumeSelectionAppliesToProvider(selectedProvider, provider string) bool {
	selected := compozyconfig.CanonicalProviderName(selectedProvider)
	return selected == "" || selected == compozyconfig.CanonicalProviderName(provider)
}

func clearResumeNativeRoute(spec *sessionStartSpec) {
	if strings.TrimSpace(spec.acpSessionID) == "" {
		return
	}
	spec.acpSessionID = ""
	spec.resumeReplay = true
	spec.resumeReplayReason = acceptedRouteMissingReason
}

func (m *Manager) fallbackRouteMatchesAcceptedRoute(
	ctx context.Context,
	base *sessionStartSpec,
	agentDef compozyconfig.AgentDef,
	route FallbackRoute,
	record *store.SessionAcceptedRoute,
) bool {
	if base == nil {
		return false
	}
	spec := *base
	spec.provider = strings.TrimSpace(route.Provider)
	spec.model = strings.TrimSpace(route.Model)
	spec.command = strings.TrimSpace(route.Command)
	resolved, err := spec.workspace.Config.ResolveSessionAgentWithRuntime(agentDef, compozyconfig.RuntimeOverrides{
		Provider: spec.provider, Model: spec.model, Command: spec.command,
		SessionEnv: route.SessionEnv,
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
		resolved.LaunchFingerprint() == record.CommandFingerprint
}

// resetSpecToPrimaryRoute replaces a chain route's complete runtime selection (provider,
// model, reasoning, speed, and ACP options) with the session's primary route: the
// operator-selected runtime when one exists, else the agent defaults. No setting of the
// removed route survives to be merged over the primary defaults.
func resetSpecToPrimaryRoute(spec *sessionStartSpec) {
	if selected := spec.selectedRuntime; selected != nil {
		spec.provider = strings.TrimSpace(selected.Provider)
		spec.model = strings.TrimSpace(selected.Model)
		spec.reasoningEffort = strings.TrimSpace(selected.ReasoningEffort)
		spec.speed = selected.Speed
		spec.acpOptions = acp.CloneSessionConfigOptionSelections(selected.ACPOptions)
		return
	}
	spec.provider = ""
	spec.model = ""
	spec.reasoningEffort = ""
	spec.speed = ""
	spec.acpOptions = nil
}
