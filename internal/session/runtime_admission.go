package session

import (
	"log/slog"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

// runtimeAdmission names why a runtime is being resolved, so the disabled-provider gate can
// exempt recovery of a session's already-accepted route (BR-5) while refusing new work.
type runtimeAdmission int

const (
	// admissionNewWork covers create, spawn, derive and every other new session start.
	admissionNewWork runtimeAdmission = iota
	// admissionStoppedResume covers resuming a stopped session.
	admissionStoppedResume
	// admissionRuntimeChange covers an explicit runtime selection or a fallback to another route.
	admissionRuntimeChange
	// admissionAcceptedRecovery covers prompt-time recovery of the already-accepted route; never gated.
	admissionAcceptedRecovery
)

func (a runtimeAdmission) surface() string {
	switch a {
	case admissionStoppedResume:
		return "resume"
	case admissionRuntimeChange:
		return "runtime_selection"
	case admissionAcceptedRecovery:
		return "accepted_recovery"
	default:
		return "session_start"
	}
}

// specAdmission resolves the admission purpose of a start spec for the resolved provider. A
// runtime plan for a session already bound to that provider recovers its accepted route.
func specAdmission(spec *sessionStartSpec, resolvedProvider string) runtimeAdmission {
	if spec.admission == admissionRuntimeChange {
		return runtimeChangeAdmission(spec.boundProvider, resolvedProvider)
	}
	if spec.admission == admissionNewWork && spec.startAction == sessionStartActionResume {
		return admissionStoppedResume
	}
	return spec.admission
}

// runtimeChangeAdmission exempts a route on the provider the session is already bound to.
func runtimeChangeAdmission(boundProvider string, selectedProvider string) runtimeAdmission {
	bound := compozyconfig.CanonicalProviderName(strings.TrimSpace(boundProvider))
	if bound != "" && bound == compozyconfig.CanonicalProviderName(strings.TrimSpace(selectedProvider)) {
		return admissionAcceptedRecovery
	}
	return admissionRuntimeChange
}

// requireEnabledRuntimeProvider refuses a disabled provider unless the admission recovers the
// session's accepted route. The predicate reads the config snapshot of this decision.
func requireEnabledRuntimeProvider(
	cfg *compozyconfig.Config,
	provider string,
	admission runtimeAdmission,
	logger *slog.Logger,
) error {
	if admission == admissionAcceptedRecovery {
		return nil
	}
	err := cfg.RequireEnabledProvider(provider)
	if err != nil {
		if logger == nil {
			logger = slog.Default()
		}
		logger.Info(
			"provider.disabled_refused",
			"provider", compozyconfig.CanonicalProviderName(provider),
			"surface", admission.surface(),
		)
	}
	return err
}

// enabledFallbackRoutes drops agent-chain routes on disabled providers (BR-6) so a chain only
// attempts enabled routes. Each skip logs the route's declared 1-based index.
func enabledFallbackRoutes(cfg *compozyconfig.Config, routes []FallbackRoute, logger *slog.Logger) []FallbackRoute {
	if len(routes) == 0 {
		return routes
	}
	kept := make([]FallbackRoute, 0, len(routes))
	for index, route := range routes {
		if !cfg.IsProviderDisabled(route.Provider) {
			kept = append(kept, route)
			continue
		}
		if logger == nil {
			logger = slog.Default()
		}
		logger.Info(
			"fallback.route_skipped",
			"reason", "provider_disabled",
			"provider", compozyconfig.CanonicalProviderName(route.Provider),
			"route_index", index+1,
			"owner", "agent_chain",
		)
	}
	return kept
}
