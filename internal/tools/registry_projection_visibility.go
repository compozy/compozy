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
	if scope.Operator || (!r.usePolicyInput && r.evaluator != nil) {
		return "", false
	}
	inputs, err := r.policyInputsFor(ctx, scope)
	if err != nil {
		return "", false
	}
	// ToolPattern and agent enforcement are intentionally absent from PolicyInputs' JSON shape.
	type visibilityPolicy struct {
		Inputs        PolicyInputs
		Allow         []string
		Deny          []string
		AgentEnforced bool
		AgentAllow    []string
		AgentDeny     []string
	}
	policy := visibilityPolicy{inputs, projectionPatterns(inputs.AllowTools), projectionPatterns(inputs.DenyTools),
		inputs.Agent.Enforced, projectionPatterns(inputs.Agent.Tools), projectionPatterns(inputs.Agent.DenyTools)}
	material := struct {
		Profile   string
		Workspace string
		Policy    visibilityPolicy
		Providers []Scope
		Native    []Availability
	}{Profile: scope.ProfileID, Workspace: scope.WorkspaceID, Policy: policy}
	for _, provider := range r.providers {
		if native, ok := provider.(*NativeProvider); ok {
			for _, id := range native.ids {
				if resolve := native.tools[id].Availability; resolve != nil {
					material.Native = append(material.Native, resolve(ctx, scope))
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
	return hex.EncodeToString(digest[:]), true
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
