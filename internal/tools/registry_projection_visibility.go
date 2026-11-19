package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// ProjectionScopeProvider declares every scope field used by provider listing and availability.
type ProjectionScopeProvider interface {
	ProjectionScope(Scope) Scope
}

// ProjectionVisibility identifies equivalent session views within one registry and refresh epoch.
func (r *RuntimeRegistry) ProjectionVisibility(ctx context.Context, scope Scope) (string, bool) {
	ctx = r.ReadContext(ctx)
	cache, ok := ctx.Value(policyResolutionKey{}).(*policyResolutionCache)
	if !ok {
		return "", false
	}
	key := policyResolutionScope{r, scope}
	cache.visibilityMu.Lock()
	class, found := cache.visibilities[key]
	cache.visibilityMu.Unlock()
	if found {
		return class, true
	}
	if scope.Operator || (!r.usePolicyInput && r.evaluator != nil) {
		return "", false
	}
	inputs, err := r.policyInputsFor(ctx, scope)
	if err != nil {
		return "", false
	}
	if inputs.CheckIn != nil {
		return "", false
	}
	material := struct {
		Profile   string
		Workspace string
		Policy    visibilityPolicy
		Providers []Scope
		Native    []Availability
	}{Profile: scope.ProfileID, Workspace: scope.WorkspaceID, Policy: projectionPolicy(inputs)}
	for _, provider := range r.providers {
		if native, ok := provider.(*NativeProvider); ok {
			for _, id := range native.ids {
				if resolve := native.tools[id].Availability; resolve != nil {
					material.Native = append(material.Native, r.availabilityFor(ctx, scope, &registryEntry{
						descriptor: native.tools[id].Descriptor, provider: native,
					}))
				}
			}
			continue
		}
		providerScope, ok := provider.(ProjectionScopeProvider)
		if !ok {
			return "", false
		}
		material.Providers = append(material.Providers, providerScope.ProjectionScope(scope))
	}
	encoded, err := json.Marshal(material)
	if err != nil || ctx.Err() != nil {
		return "", false
	}
	digest := sha256.Sum256(encoded)
	class = hex.EncodeToString(digest[:])
	cache.visibilityMu.Lock()
	if cache.visibilities == nil {
		cache.visibilities = make(map[policyResolutionScope]string)
	}
	cache.visibilities[key] = class
	cache.visibilityMu.Unlock()
	return class, true
}

// ToolPattern and agent enforcement are absent from PolicyInputs' JSON shape.
type visibilityPolicy struct {
	Inputs        PolicyInputs
	Allow         []string
	Deny          []string
	AgentEnforced bool
	AgentAllow    []string
	AgentDeny     []string
	SessionDigest [sha256.Size]byte
}

func projectionPolicy(inputs PolicyInputs) visibilityPolicy {
	digest := inputs.Session.projectionDigest
	if digest != [sha256.Size]byte{} {
		inputs.Session.Tools = nil
	}
	return visibilityPolicy{inputs, projectionPatterns(inputs.AllowTools), projectionPatterns(inputs.DenyTools),
		inputs.Agent.Enforced, projectionPatterns(inputs.Agent.Tools), projectionPatterns(inputs.Agent.DenyTools), digest}
}

func projectionPatterns(patterns []ToolPattern) []string {
	result := make([]string, len(patterns))
	for i, pattern := range patterns {
		result[i] = pattern.String()
	}
	return result
}

// ProjectionScope excludes execution identity because MCP catalogs are profile/workspace scoped.
func (p *MCPProvider) ProjectionScope(scope Scope) Scope {
	return Scope{ProfileID: scope.ProfileID, WorkspaceID: scope.WorkspaceID}
}

var _ ProjectionScopeProvider = (*MCPProvider)(nil)
