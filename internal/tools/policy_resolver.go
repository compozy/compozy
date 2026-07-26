package tools

import (
	"context"
)

// PolicyInputResolver resolves effective policy inputs for the current caller scope.
type PolicyInputResolver interface {
	Resolve(ctx context.Context, scope Scope) (PolicyInputs, error)
}

type staticPolicyInputResolver struct {
	inputs PolicyInputs
}

var _ PolicyInputResolver = (*staticPolicyInputResolver)(nil)

// NewStaticPolicyInputResolver returns a resolver for callers with fixed policy inputs.
func NewStaticPolicyInputResolver(inputs PolicyInputs) PolicyInputResolver {
	return &staticPolicyInputResolver{
		inputs: clonePolicyInputs(inputs),
	}
}

func (r *staticPolicyInputResolver) Resolve(_ context.Context, _ Scope) (PolicyInputs, error) {
	if r == nil {
		return DefaultPolicyInputs(), nil
	}
	return clonePolicyInputs(r.inputs), nil
}

func clonePolicyInputs(src PolicyInputs) PolicyInputs {
	cloned := src
	cloned.TrustedSources = append([]SourceGrant(nil), src.TrustedSources...)
	cloned.AllowSources = append([]SourceGrant(nil), src.AllowSources...)
	cloned.AllowTools = append([]ToolPattern(nil), src.AllowTools...)
	cloned.AllowToolsets = append([]ToolsetID(nil), src.AllowToolsets...)
	cloned.DenyTools = append([]ToolPattern(nil), src.DenyTools...)
	cloned.Agent.Tools = append([]ToolPattern(nil), src.Agent.Tools...)
	cloned.Agent.Toolsets = append([]ToolsetID(nil), src.Agent.Toolsets...)
	cloned.Agent.DenyTools = append([]ToolPattern(nil), src.Agent.DenyTools...)
	cloned.Session.Tools = append([]ToolID(nil), src.Session.Tools...)
	return cloned
}
