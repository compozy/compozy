package daemon

import (
	"context"
	"log/slog"
	"strings"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

// operatorAuthorizedMCPSourceGrants turns one completed OAuth login into an allow grant bound to
// the exact extension resource that scope projects, never to a normalized server name.
// tools.policy.external_default stays the default for every other source; only a credential bound
// to the current definition counts as consent, so logout, a changed definition, and a disabled
// extension each withdraw the grant. A status failure grants nothing, which keeps the
// deny-by-default outcome.
func operatorAuthorizedMCPSourceGrants(
	ctx context.Context,
	state *bootState,
	scope toolspkg.Scope,
) ([]toolspkg.SourceGrant, error) {
	if state == nil || state.mcpAuthStatus == nil {
		return nil, nil
	}
	sources, err := daemonMCPSources(ctx, state)
	if err != nil {
		// Grants only add access, so one unreadable record must not stop policy for native tools.
		logMCPSourceGrantSkipped(ctx, state, toolspkg.SourceRef{}, err)
		return nil, nil
	}
	var grants []toolspkg.SourceGrant
	for _, source := range toolspkg.MCPSourcesForScope(sources, scope) {
		if strings.TrimSpace(source.ResourceID) == "" {
			continue
		}
		resolved, err := resolveDaemonMCPServer(ctx, state, source, "")
		if err != nil {
			logMCPSourceGrantSkipped(ctx, state, source, err)
			continue
		}
		if !strings.HasPrefix(resolved.Target.Owner, "extension:") || !resolved.Server.Auth.Enabled() {
			continue
		}
		status, err := state.mcpAuthStatus.Status(ctx, source, "")
		if err != nil {
			logMCPSourceGrantSkipped(ctx, state, source, err)
			continue
		}
		if !status.TokenPresent {
			continue
		}
		grants = append(grants, toolspkg.SourceGrant{
			Kind: toolspkg.SourceMCP, Owner: source.Owner, ResourceID: source.ResourceID,
		})
	}
	return grants, nil
}

func logMCPSourceGrantSkipped(ctx context.Context, state *bootState, source toolspkg.SourceRef, err error) {
	logger := state.logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.DebugContext(ctx, "daemon.mcp.source_grant_skipped", "server", source.RawServerName, "error", err)
}
