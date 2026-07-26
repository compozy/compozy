package daemon

import (
	"context"
	"errors"
	"fmt"

	aghconfig "github.com/compozy/agh/internal/config"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

// CoordinatorRoleResolver resolves coordinator routing and safety policy without starting behavior.
type CoordinatorRoleResolver interface {
	ResolveCoordinatorRole(ctx context.Context, workspaceID string) (aghconfig.ResolvedCoordinatorRole, error)
}

type coordinatorAgentResolver interface {
	ResolveAgent(name string, resolved *workspacepkg.ResolvedWorkspace) (aghconfig.AgentDef, error)
}

type defaultCoordinatorRoleResolver struct {
	roles *roleResolver
}

var _ CoordinatorRoleResolver = (*defaultCoordinatorRoleResolver)(nil)

func coordinatorRoleResolverFor(roles *roleResolver) CoordinatorRoleResolver {
	return &defaultCoordinatorRoleResolver{roles: roles}
}

func (r *defaultCoordinatorRoleResolver) ResolveCoordinatorRole(
	ctx context.Context,
	workspaceID string,
) (aghconfig.ResolvedCoordinatorRole, error) {
	if ctx == nil {
		return aghconfig.ResolvedCoordinatorRole{}, errors.New("daemon: coordinator role context is required")
	}
	if r == nil || r.roles == nil {
		return aghconfig.ResolvedCoordinatorRole{}, errors.New("daemon: coordinator role resolver is required")
	}

	resolvedRole, effectiveConfig, err := r.roles.resolveEffective(ctx, workspaceID, aghconfig.RoleCoordinator)
	if err != nil {
		recordErr := r.roles.recordRoleResolveError(ctx, workspaceID, aghconfig.RoleCoordinator, err)
		return aghconfig.ResolvedCoordinatorRole{}, fmt.Errorf(
			"daemon: resolve coordinator role: %w",
			errors.Join(err, recordErr),
		)
	}
	effective := effectiveConfig.Roles.Coordinator
	return aghconfig.ResolvedCoordinatorRole{
		Enabled:                       resolvedRole.Enabled,
		AgentName:                     resolvedRole.AgentName,
		Provider:                      resolvedRole.Provider,
		Model:                         resolvedRole.Model,
		ReasoningEffort:               resolvedRole.ReasoningEffort,
		Fallbacks:                     append([]aghconfig.RoleFallback(nil), resolvedRole.Fallbacks...),
		TTL:                           effective.TTL,
		MaxChildren:                   effective.MaxChildren,
		MaxActiveSessionsPerWorkspace: effective.MaxActiveSessionsPerWorkspace,
	}, nil
}
