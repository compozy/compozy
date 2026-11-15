package daemon

import (
	"context"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/workspace"
)

type mcpAuthorityReadKey struct{}
type mcpAuthorityScopeKey struct{ profile, workspace string }
type mcpAuthorityScope struct {
	workspace *workspace.ResolvedAgentConfig
	cfg       *config.Config
	inputs    tools.PolicyInputs
	names     []string
	err       error
}

type mcpAuthorityReadCache map[mcpAuthorityScopeKey]mcpAuthorityScope

func (r *nativeToolPolicyResolver) MCPAuthorityReadContext(ctx context.Context) context.Context {
	if mcpAuthorityRead(ctx) {
		return ctx
	}
	return context.WithValue(ctx, mcpAuthorityReadKey{}, make(mcpAuthorityReadCache))
}

func mcpAuthorityRead(ctx context.Context) bool {
	return ctx.Value(mcpAuthorityReadKey{}) != nil
}

func (r *nativeToolPolicyResolver) resolveMCPAuthorityScope(
	ctx context.Context,
	scope tools.Scope,
) (mcpAuthorityScope, error) {
	cache, ok := ctx.Value(mcpAuthorityReadKey{}).(mcpAuthorityReadCache)
	if !ok {
		cache = nil
	}
	key := mcpAuthorityScopeKey{scope.ProfileID, scope.WorkspaceID}
	if cached, ok := cache[key]; ok {
		return cached, cached.err
	}
	var result mcpAuthorityScope
	result.workspace, result.cfg, result.err = r.resolveWorkspaceConfig(ctx, scope)
	if result.err == nil {
		result.inputs, result.err = nativeToolPolicyInputs(result.cfg)
	}
	if result.err == nil {
		result.err = r.applyScopedSourceGrants(ctx, &result.inputs, scope, result.workspace)
	}
	if result.err == nil {
		result.names = r.admittedMCPNames(result.inputs, result.cfg.MCPServers)
	}
	if cache != nil {
		cache[key] = result
	}
	return result, result.err
}
