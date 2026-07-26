// Package surfaces defines the static extension resource surface policy.
package surfaces

import (
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/agh/internal/resources"
)

// ManifestFamily identifies one family-oriented manifest request name.
type ManifestFamily string

const (
	FamilyHooks              ManifestFamily = "hooks"
	FamilyTools              ManifestFamily = "tools"
	FamilyAgents             ManifestFamily = "agents"
	FamilyMCPServers         ManifestFamily = "mcp_servers"
	FamilySkills             ManifestFamily = "skills"
	FamilyAutomationJobs     ManifestFamily = "automation_jobs"
	FamilyAutomationTriggers ManifestFamily = "automation_triggers"
	FamilyBundles            ManifestFamily = "bundles"
	FamilyWindowLayouts      ManifestFamily = "window_layouts"
	FamilyBridgeInstances    ManifestFamily = "bridge_instances"
	FamilyBundleActivations  ManifestFamily = "bundle_activations"
)

// Surface declares the daemon-authoritative resource publication metadata for one kind.
type Surface struct {
	Kind             resources.ResourceKind
	ManifestFamily   ManifestFamily
	ExtensionPublish bool
	LegalScopes      []resources.ResourceScopeKind
}

// GrantRequest is the validated manifest request shape resolved from family names.
type GrantRequest struct {
	Kinds    []resources.ResourceKind
	Scopes   []resources.ResourceScopeKind
	MaxScope resources.ResourceScopeKind
}

var (
	globalAndWorkspaceScopes = []resources.ResourceScopeKind{
		resources.ResourceScopeKindGlobal,
		resources.ResourceScopeKindWorkspace,
	}
	registry = []Surface{
		{
			Kind:             resources.ResourceKind("hook.binding"),
			ManifestFamily:   FamilyHooks,
			ExtensionPublish: true,
			LegalScopes:      cloneScopes(globalAndWorkspaceScopes),
		},
		{
			Kind:             resources.ResourceKind("tool"),
			ManifestFamily:   FamilyTools,
			ExtensionPublish: true,
			LegalScopes:      cloneScopes(globalAndWorkspaceScopes),
		},
		{
			Kind:             resources.ResourceKind("agent"),
			ManifestFamily:   FamilyAgents,
			ExtensionPublish: true,
			LegalScopes:      cloneScopes(globalAndWorkspaceScopes),
		},
		{
			Kind:             resources.ResourceKind("mcp_server"),
			ManifestFamily:   FamilyMCPServers,
			ExtensionPublish: true,
			LegalScopes:      cloneScopes(globalAndWorkspaceScopes),
		},
		{
			Kind:             resources.ResourceKind("skill"),
			ManifestFamily:   FamilySkills,
			ExtensionPublish: true,
			LegalScopes:      cloneScopes(globalAndWorkspaceScopes),
		},
		{
			Kind:             resources.ResourceKind("automation.job"),
			ManifestFamily:   FamilyAutomationJobs,
			ExtensionPublish: true,
			LegalScopes:      cloneScopes(globalAndWorkspaceScopes),
		},
		{
			Kind:             resources.ResourceKind("automation.trigger"),
			ManifestFamily:   FamilyAutomationTriggers,
			ExtensionPublish: true,
			LegalScopes:      cloneScopes(globalAndWorkspaceScopes),
		},
		{
			Kind:             resources.ResourceKind("bundle"),
			ManifestFamily:   FamilyBundles,
			ExtensionPublish: true,
			LegalScopes:      cloneScopes(globalAndWorkspaceScopes),
		},
		{
			Kind:             resources.ResourceKind("window_layout"),
			ManifestFamily:   FamilyWindowLayouts,
			ExtensionPublish: true,
			LegalScopes:      cloneScopes(globalAndWorkspaceScopes),
		},
		{
			Kind:             resources.ResourceKind("bridge.instance"),
			ManifestFamily:   FamilyBridgeInstances,
			ExtensionPublish: false,
			LegalScopes:      cloneScopes(globalAndWorkspaceScopes),
		},
		{
			Kind:             resources.ResourceKind("bundle.activation"),
			ManifestFamily:   FamilyBundleActivations,
			ExtensionPublish: false,
			LegalScopes:      cloneScopes(globalAndWorkspaceScopes),
		},
	}
	registryByKind         = buildRegistryByKind(registry)
	registryByManifestName = buildRegistryByManifestName(registry)
)

// All returns the full static surface registry.
func All() []Surface {
	cloned := make([]Surface, 0, len(registry))
	for _, surface := range registry {
		cloned = append(cloned, cloneSurface(surface))
	}
	return cloned
}

// Lookup resolves one resource kind from the static registry.
func Lookup(kind resources.ResourceKind) (Surface, bool) {
	surface, ok := registryByKind[kind.Normalize()]
	if !ok {
		return Surface{}, false
	}
	return cloneSurface(surface), true
}

// PublishableKinds returns the kinds that extensions may publish.
func PublishableKinds() []resources.ResourceKind {
	kinds := make([]resources.ResourceKind, 0, len(registry))
	for _, surface := range registry {
		if !surface.ExtensionPublish {
			continue
		}
		kinds = append(kinds, surface.Kind)
	}
	slices.Sort(kinds)
	return kinds
}

// ResolveManifestRequest validates one family-oriented manifest request against the surface table.
func ResolveManifestRequest(
	families []string,
	maxScope resources.ResourceScopeKind,
) (GrantRequest, error) {
	normalizedFamilies := normalizeFamilies(families)
	normalizedMaxScope := maxScope.Normalize()
	if len(normalizedFamilies) == 0 {
		if normalizedMaxScope != "" {
			return GrantRequest{}, fmt.Errorf("resources.publish.max_scope requires at least one family")
		}
		return GrantRequest{}, nil
	}
	if normalizedMaxScope == "" {
		normalizedMaxScope = resources.ResourceScopeKindGlobal
	}
	if err := normalizedMaxScope.Validate("resources.publish.max_scope"); err != nil {
		return GrantRequest{}, err
	}

	requestedKinds := make([]resources.ResourceKind, 0, len(normalizedFamilies))
	legalScopes := cloneScopes(globalAndWorkspaceScopes)
	for _, family := range normalizedFamilies {
		surface, ok := registryByManifestName[family]
		if !ok {
			return GrantRequest{}, fmt.Errorf("unknown manifest resource family %q", family)
		}
		if !surface.ExtensionPublish {
			return GrantRequest{}, fmt.Errorf("manifest resource family %q is daemon-only", family)
		}
		requestedKinds = append(requestedKinds, surface.Kind)
		legalScopes = intersectScopes(legalScopes, surface.LegalScopes)
	}
	requestedKinds = normalizeKinds(requestedKinds)
	allowedScopes := intersectScopes(legalScopes, scopesThrough(normalizedMaxScope))
	if len(allowedScopes) == 0 {
		return GrantRequest{}, fmt.Errorf(
			"resources.publish.max_scope %q is not legal for requested kinds",
			normalizedMaxScope,
		)
	}
	return GrantRequest{
		Kinds:    requestedKinds,
		Scopes:   allowedScopes,
		MaxScope: normalizedMaxScope,
	}, nil
}

// NormalizeAllowedKinds validates and normalizes operator-config allowlisted kinds.
func NormalizeAllowedKinds(kinds []resources.ResourceKind) ([]resources.ResourceKind, error) {
	normalized := normalizeKinds(kinds)
	for _, kind := range normalized {
		surface, ok := registryByKind[kind]
		if !ok {
			return nil, fmt.Errorf("unknown extension resource kind %q", kind)
		}
		if !surface.ExtensionPublish {
			return nil, fmt.Errorf("resource kind %q is daemon-only", kind)
		}
	}
	return normalized, nil
}

func buildRegistryByKind(values []Surface) map[resources.ResourceKind]Surface {
	index := make(map[resources.ResourceKind]Surface, len(values))
	for _, surface := range values {
		index[surface.Kind.Normalize()] = cloneSurface(surface)
	}
	return index
}

func buildRegistryByManifestName(values []Surface) map[string]Surface {
	index := make(map[string]Surface, len(values))
	for _, surface := range values {
		index[strings.TrimSpace(string(surface.ManifestFamily))] = cloneSurface(surface)
	}
	return index
}

func cloneSurface(surface Surface) Surface {
	return Surface{
		Kind:             surface.Kind,
		ManifestFamily:   surface.ManifestFamily,
		ExtensionPublish: surface.ExtensionPublish,
		LegalScopes:      cloneScopes(surface.LegalScopes),
	}
}

func normalizeFamilies(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	families := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		families = append(families, trimmed)
	}
	if len(families) == 0 {
		return nil
	}
	slices.Sort(families)
	return families
}

func normalizeKinds(values []resources.ResourceKind) []resources.ResourceKind {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[resources.ResourceKind]struct{}, len(values))
	kinds := make([]resources.ResourceKind, 0, len(values))
	for _, value := range values {
		normalized := value.Normalize()
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		kinds = append(kinds, normalized)
	}
	if len(kinds) == 0 {
		return nil
	}
	slices.Sort(kinds)
	return kinds
}

func scopesThrough(maxScope resources.ResourceScopeKind) []resources.ResourceScopeKind {
	switch maxScope.Normalize() {
	case resources.ResourceScopeKindGlobal:
		return cloneScopes(globalAndWorkspaceScopes)
	case resources.ResourceScopeKindWorkspace:
		return []resources.ResourceScopeKind{resources.ResourceScopeKindWorkspace}
	default:
		return nil
	}
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

func cloneScopes(values []resources.ResourceScopeKind) []resources.ResourceScopeKind {
	if len(values) == 0 {
		return nil
	}
	return append([]resources.ResourceScopeKind(nil), values...)
}
