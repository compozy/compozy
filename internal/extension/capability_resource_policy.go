package extensionpkg

import (
	"slices"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/extension/surfaces"
	"github.com/compozy/agh/internal/resources"
)

type sourceTierPolicy struct {
	allowAllActions  bool
	allowAllSecurity bool
	allowedActions   []string
	allowedSecurity  []string
	maxResourceScope resources.ResourceScopeKind
}

func sourcePolicy(source ExtensionSource) sourceTierPolicy {
	switch source {
	case SourceBundled, SourceUser, SourceWorkspace:
		return sourceTierPolicy{
			allowAllActions:  true,
			allowAllSecurity: true,
			maxResourceScope: sourceTierMaxScope(source),
		}
	case SourceMarketplace:
		return sourceTierPolicy{
			allowedActions:   marketplaceActionCeiling(),
			allowedSecurity:  slices.Clone(marketplaceSecurityCeiling),
			maxResourceScope: sourceTierMaxScope(source),
		}
	default:
		return sourceTierPolicy{}
	}
}

func marketplaceActionCeiling() []string {
	actions := make([]string, 0, len(hostAPIMethodSecurityCapability))
	for method, capability := range hostAPIMethodSecurityCapability {
		if capabilityGranted(marketplaceSecurityCeiling, capability) {
			actions = append(actions, method)
		}
	}
	slices.Sort(actions)
	return actions
}

func effectiveResourceGrants(
	source ExtensionSource,
	operatorPolicy aghconfig.ExtensionsResourcesConfig,
	requested surfaces.GrantRequest,
	sessionMaxScope resources.ResourceScopeKind,
) ([]resources.ResourceKind, []resources.ResourceScopeKind, error) {
	if len(requested.Kinds) == 0 {
		return nil, nil, nil
	}

	grantedKinds := slices.Clone(requested.Kinds)
	if len(operatorPolicy.AllowedKinds) > 0 {
		allowedKinds, err := surfaces.NormalizeAllowedKinds(operatorPolicy.AllowedKinds)
		if err != nil {
			return nil, nil, err
		}
		grantedKinds = intersectKinds(grantedKinds, allowedKinds)
	}
	if len(grantedKinds) == 0 {
		return nil, nil, nil
	}

	finalMaxScope, err := narrowScopeCeiling(
		requested.MaxScope,
		sourceTierMaxScope(source),
		operatorPolicy.MaxScope,
		sessionMaxScope,
	)
	if err != nil {
		return nil, nil, err
	}
	grantedScopes := intersectScopes(requested.Scopes, scopesThrough(finalMaxScope))
	if len(grantedScopes) == 0 {
		return nil, nil, nil
	}
	return grantedKinds, grantedScopes, nil
}

func sourceTierMaxScope(source ExtensionSource) resources.ResourceScopeKind {
	switch source {
	case SourceWorkspace, SourceMarketplace:
		return resources.ResourceScopeKindWorkspace
	case SourceBundled, SourceUser:
		return resources.ResourceScopeKindGlobal
	default:
		return ""
	}
}

func narrowScopeCeiling(
	requested resources.ResourceScopeKind,
	sourceTier resources.ResourceScopeKind,
	operator resources.ResourceScopeKind,
	session resources.ResourceScopeKind,
) (resources.ResourceScopeKind, error) {
	candidates := []resources.ResourceScopeKind{
		requested.Normalize(),
		sourceTier.Normalize(),
		operator.Normalize(),
		session.Normalize(),
	}
	result := resources.ResourceScopeKindGlobal
	seen := false
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if err := candidate.Validate("resource scope"); err != nil {
			return "", err
		}
		if !seen || scopeRank(candidate) < scopeRank(result) {
			result = candidate
			seen = true
		}
	}
	if !seen {
		return "", nil
	}
	return result, nil
}

func scopeRank(scope resources.ResourceScopeKind) int {
	switch scope.Normalize() {
	case resources.ResourceScopeKindWorkspace:
		return 0
	case resources.ResourceScopeKindGlobal:
		return 1
	default:
		return 2
	}
}

func scopesThrough(maxScope resources.ResourceScopeKind) []resources.ResourceScopeKind {
	switch maxScope.Normalize() {
	case resources.ResourceScopeKindGlobal:
		return []resources.ResourceScopeKind{
			resources.ResourceScopeKindGlobal,
			resources.ResourceScopeKindWorkspace,
		}
	case resources.ResourceScopeKindWorkspace:
		return []resources.ResourceScopeKind{resources.ResourceScopeKindWorkspace}
	default:
		return nil
	}
}

func intersectKinds(
	left []resources.ResourceKind,
	right []resources.ResourceKind,
) []resources.ResourceKind {
	if len(left) == 0 || len(right) == 0 {
		return nil
	}
	index := make(map[resources.ResourceKind]struct{}, len(right))
	for _, kind := range right {
		index[kind.Normalize()] = struct{}{}
	}
	var kinds []resources.ResourceKind
	for _, kind := range left {
		normalized := kind.Normalize()
		if _, ok := index[normalized]; ok {
			kinds = append(kinds, normalized)
		}
	}
	if len(kinds) == 0 {
		return nil
	}
	slices.Sort(kinds)
	return kinds
}

func intersectScopes(
	left []resources.ResourceScopeKind,
	right []resources.ResourceScopeKind,
) []resources.ResourceScopeKind {
	if len(left) == 0 || len(right) == 0 {
		return nil
	}
	index := make(map[resources.ResourceScopeKind]struct{}, len(right))
	for _, scope := range right {
		index[scope.Normalize()] = struct{}{}
	}
	var scopes []resources.ResourceScopeKind
	for _, scope := range left {
		normalized := scope.Normalize()
		if _, ok := index[normalized]; ok {
			scopes = append(scopes, normalized)
		}
	}
	if len(scopes) == 0 {
		return nil
	}
	slices.Sort(scopes)
	return scopes
}

func cloneResourcePolicy(policy aghconfig.ExtensionsResourcesConfig) aghconfig.ExtensionsResourcesConfig {
	return aghconfig.ExtensionsResourcesConfig{
		AllowedKinds: append([]resources.ResourceKind(nil), policy.AllowedKinds...),
		MaxScope:     policy.MaxScope,
		SnapshotRateLimit: aghconfig.ExtensionsResourceRateLimitConfig{
			Requests: policy.SnapshotRateLimit.Requests,
			Window:   policy.SnapshotRateLimit.Window,
			Queue:    policy.SnapshotRateLimit.Queue,
		},
		OperatorWriteRateLimit: aghconfig.ExtensionsResourceRateLimitConfig{
			Requests: policy.OperatorWriteRateLimit.Requests,
			Window:   policy.OperatorWriteRateLimit.Window,
			Queue:    policy.OperatorWriteRateLimit.Queue,
		},
	}
}
