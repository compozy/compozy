package daemon

import (
	"log/slog"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

// skipDisabledRoleRoutes applies BR-6 to a resolved role: fallbacks on disabled providers are
// dropped, a disabled primary is skipped in favor of the first enabled fallback, and a role with
// no enabled route is unavailable with provider_disabled. Each skip logs fallback.route_skipped
// with the route's declared index (0 = primary).
func skipDisabledRoleRoutes(cfg *compozyconfig.Config, resolved *ResolvedRole) error {
	if cfg == nil || resolved == nil {
		return nil
	}
	kept := make([]compozyconfig.RoleFallback, 0, len(resolved.Fallbacks))
	for index, fallback := range resolved.Fallbacks {
		if cfg.IsProviderDisabled(fallback.Provider) {
			logRoleRouteSkipped(resolved.Role, fallback.Provider, index+1)
			continue
		}
		kept = append(kept, fallback)
	}
	resolved.Fallbacks = kept
	if !cfg.IsProviderDisabled(resolved.Provider) {
		return nil
	}
	logRoleRouteSkipped(resolved.Role, resolved.Provider, 0)
	if len(kept) == 0 {
		return &RoleResolutionError{
			Code:  contract.CodeProviderDisabled,
			Role:  resolved.Role,
			Agent: resolved.AgentName,
			Err: &compozyconfig.ProviderDisabledError{
				Provider: compozyconfig.CanonicalProviderName(resolved.Provider),
			},
		}
	}
	resolved.PrimaryDisabled = true
	return nil
}

func logRoleRouteSkipped(role compozyconfig.RoleName, provider string, routeIndex int) {
	slog.Info(
		"fallback.route_skipped",
		"reason", "provider_disabled",
		"provider", compozyconfig.CanonicalProviderName(provider),
		"route_index", routeIndex,
		"owner", "role",
		"role", string(role),
	)
}

// appendRoleProviderDisabledDiagnostic reports an enabled role whose explicit provider and every
// fallback are disabled, so the role is unavailable (BR-6).
// ponytail: a primary inherited from the role agent's provider is not projected here; the
// invocation-time resolver still refuses it with provider_disabled.
func appendRoleProviderDisabledDiagnostic(
	diagnostics []contract.RoleDiagnostic,
	cfg *compozyconfig.Config,
	common compozyconfig.RoleConfig,
) []contract.RoleDiagnostic {
	if !common.Enabled || !cfg.IsProviderDisabled(common.Provider) {
		return diagnostics
	}
	for _, fallback := range common.FallbackChain {
		if !cfg.IsProviderDisabled(fallback.Provider) {
			return diagnostics
		}
	}
	err := compozyconfig.ProviderDisabledError{Provider: compozyconfig.CanonicalProviderName(common.Provider)}
	return append(diagnostics, contract.RoleDiagnostic{Code: contract.CodeProviderDisabled, Message: err.Error()})
}
