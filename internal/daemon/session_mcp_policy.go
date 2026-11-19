package daemon

import (
	"context"
	"encoding/json/v2"

	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (r *nativeToolPolicyResolver) applyEffectiveSessionToolPolicy(
	ctx context.Context,
	inputs *toolspkg.PolicyInputs,
	info *session.Info,
) error {
	if info == nil || info.Type == session.SessionTypeTriage {
		return applySessionToolPolicy(inputs, info)
	}
	inputs.AlwaysOnLineage = inputs.AlwaysOnLineage || info.InstanceID != ""
	ctx = session.WithMCPAuthoritySession(ctx, info)
	if toolspkg.IsHostedCatalogRead(ctx) && !mcpAuthorityRead(ctx) {
		return r.applyHostedSessionToolPolicy(ctx, inputs, info.ID)
	}
	policy, err := r.EffectiveMCPAuthority(ctx, info.ID)
	if err != nil {
		return err
	}
	return applyConcreteSessionToolPolicy(inputs, info.ID, policy)
}

func (r *nativeToolPolicyResolver) applyHostedSessionToolPolicy(
	ctx context.Context, inputs *toolspkg.PolicyInputs, id string,
) error {
	plan, err := r.planMCPAuthority(ctx, id)
	if err != nil {
		return err
	}
	ceiling, err := json.Marshal(struct {
		Tools   []string
		Servers []string
		Intent  string
		Root    bool
	}{plan.policy.Tools, plan.policy.MCPServers, string(plan.policy.ToolsMode), plan.root})
	if err != nil {
		return err
	}
	// Authority enumeration depends on the profile, workspace, and resolved policy, not session identity.
	scope := plan.scope
	scope.SessionID, scope.AgentName = "", ""
	inputs.Session, err = toolspkg.CachedHostedSessionPolicy(ctx, scope, plan.inputs, string(ceiling),
		func() (toolspkg.SessionToolPolicy, error) {
			policy, err := r.materializeMCPAuthority(ctx, plan)
			if err != nil {
				return toolspkg.SessionToolPolicy{}, err
			}
			var concrete toolspkg.PolicyInputs
			err = applyConcreteSessionToolPolicy(&concrete, id, policy)
			return concrete.Session, err
		})
	return err
}
