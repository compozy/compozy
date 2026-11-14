package daemon

import (
	"context"

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
	policy, err := r.EffectiveMCPAuthority(session.WithMCPAuthoritySession(ctx, info), info.ID)
	if err != nil {
		return err
	}
	return applyConcreteSessionToolPolicy(inputs, info.ID, policy)
}
