package daemon

import (
	"context"
	"errors"
	"fmt"

	compozyconfig "github.com/compozy/compozy/internal/config"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// CoordinatorRoleResolver resolves coordinator routing and safety policy without starting behavior.
type CoordinatorRoleResolver interface {
	ResolveCoordinatorRole(ctx context.Context, workspaceID string) (compozyconfig.ResolvedCoordinatorRole, error)
}

type coordinatorAgentResolver interface {
	ResolveAgent(name string, resolved *workspacepkg.ResolvedWorkspace) (compozyconfig.AgentDef, error)
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
) (compozyconfig.ResolvedCoordinatorRole, error) {
	if ctx == nil {
		return compozyconfig.ResolvedCoordinatorRole{}, errors.New("daemon: coordinator role context is required")
	}
	if r == nil || r.roles == nil {
		return compozyconfig.ResolvedCoordinatorRole{}, errors.New("daemon: coordinator role resolver is required")
	}

	resolvedRole, effectiveConfig, err := r.roles.resolveEffective(ctx, workspaceID, compozyconfig.RoleCoordinator)
	if err != nil {
		recordErr := r.roles.recordRoleResolveError(ctx, workspaceID, compozyconfig.RoleCoordinator, err)
		return compozyconfig.ResolvedCoordinatorRole{}, fmt.Errorf(
			"daemon: resolve coordinator role: %w",
			errors.Join(err, recordErr),
		)
	}
	effective := effectiveConfig.Roles.Coordinator
	return compozyconfig.ResolvedCoordinatorRole{
		Enabled:                       resolvedRole.Enabled,
		AgentName:                     resolvedRole.AgentName,
		Provider:                      resolvedRole.Provider,
		Model:                         resolvedRole.Model,
		ReasoningEffort:               resolvedRole.ReasoningEffort,
		Fallbacks:                     append([]compozyconfig.RoleFallback(nil), resolvedRole.Fallbacks...),
		TTL:                           effective.TTL,
		MaxChildren:                   effective.MaxChildren,
		MaxActiveSessionsPerWorkspace: effective.MaxActiveSessionsPerWorkspace,
	}, nil
}
