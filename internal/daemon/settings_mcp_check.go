package daemon

import (
	"cmp"
	"context"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/deadentity"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

var _ settingspkg.MCPRuntimeChecker = (*settingsRuntimeSurface)(nil)

func (s *settingsRuntimeSurface) CheckMCPServerRuntime(
	ctx context.Context,
	target mcpauth.Target,
	server compozyconfig.MCPServer,
) (settingspkg.MCPServerRuntimeStatus, error) {
	key, tracked, err := s.settingsMCPDeadEntityKey(ctx, target, server.EffectiveRuntimeName())
	if err != nil {
		return settingspkg.MCPServerRuntimeStatus{}, err
	}
	if tracked && s.deadEntities != nil {
		ctx = deadentity.WithProbeRetry(ctx, key)
	}
	return s.MCPServerRuntimeStatus(toolspkg.WithFreshMCPDiscovery(ctx), target, server)
}

func (s *settingsRuntimeSurface) checkPublishedMCPRuntime(
	ctx context.Context, target mcpauth.Target, server compozyconfig.MCPServer,
	key store.DeadEntityKey, tracked bool,
) (settingspkg.MCPServerRuntimeStatus, bool, error) {
	if s.state == nil {
		return settingspkg.MCPServerRuntimeStatus{}, false, nil
	}
	provider, hasProvider := s.state.mcpToolProvider.(*toolspkg.MCPProvider)
	executor, hasExecutor := s.state.mcpAuthStatus.(*mcppkg.CallExecutor)
	if !hasProvider || !hasExecutor {
		return settingspkg.MCPServerRuntimeStatus{}, false, nil
	}
	source, found := s.publishedMCPCheckSource(ctx, target, server)
	if !found {
		return settingspkg.MCPServerRuntimeStatus{}, false, nil
	}

	var discovery mcppkg.ToolDiscovery
	var probeErr error
	probed := false
	result, err := provider.CheckMCPSource(
		ctx,
		source,
		func(ctx context.Context) ([]toolspkg.MCPToolDescriptor, error) {
			probeCtx, cancel := context.WithTimeout(ctx, s.mcpStatusTimeoutOrDefault())
			defer cancel()
			probed = true
			discovery, probeErr = executor.DiscoverTools(probeCtx, source)
			return discovery.Descriptors, probeErr
		},
	)
	if err != nil {
		return settingspkg.MCPServerRuntimeStatus{}, true, err
	}
	if !probed {
		return waitingMCPRuntimeStatus(), true, nil
	}
	if result.Rejection != nil {
		status := runtimeStatusFromMCPProbeError(result.Rejection)
		status.State = settingspkg.MCPServerRuntimeStateRuntimeUnavailable
		status.Probe = settingspkg.MCPServerProbeFailed
		status.Initialized = true
		return status, true, nil
	}
	if probeErr != nil {
		status := s.mcpRuntimeProbeFailure(target, server, probeErr)
		status, err = s.mcpStatusAfterProbeFailure(ctx, key, tracked, status)
		return status, true, err
	}
	return mcpRuntimeDiscoveryStatus(discovery), true, nil
}

func (s *settingsRuntimeSurface) publishedMCPCheckSource(
	ctx context.Context, target mcpauth.Target, server compozyconfig.MCPServer,
) (toolspkg.SourceRef, bool) {
	target = target.Normalize()
	selection := settingsConsentRequest(target, server)
	lens := mcpauth.ConsentScopeFromContext(ctx)
	profileID := lens.ProfileID
	if profileID == "" {
		var err error
		profileID, err = (settingsMCPConsent{state: s.state}).profileID(
			ctx,
			cmp.Or(lens.ProfileName, selection.ProfileName),
		)
		if err != nil {
			return toolspkg.SourceRef{}, false
		}
	}
	sources, err := daemonMCPSources(ctx, s.state)
	if err != nil {
		return toolspkg.SourceRef{}, false
	}
	digest, err := compozyconfig.MCPDefinitionDigest(server, s.digestKey)
	if err != nil {
		return toolspkg.SourceRef{}, false
	}
	scope := toolspkg.Scope{ProfileID: profileID, WorkspaceID: selection.WorkspaceID}
	for _, source := range toolspkg.MCPSourcesForScope(sources, scope) {
		if source.RawServerName != server.EffectiveRuntimeName() {
			continue
		}
		definitionSource := source
		if strings.HasPrefix(target.Owner, "extension:") {
			// Settings selects the extension's authored definition through its install owner.
			definitionSource.ProfileID = ""
		}
		published, err := resolveDaemonMCPServer(ctx, s.state, definitionSource, target.Owner)
		if err != nil || published.Target.Normalize() != target || published.Server.IsDisabled() ||
			strings.TrimSpace(published.Server.DefinitionSource) != strings.TrimSpace(server.DefinitionSource) {
			continue
		}
		// Settings can select a file whose contents have yet to be published to the catalog.
		if current, err := compozyconfig.MCPDefinitionDigest(
			published.Server,
			s.digestKey,
		); err == nil &&
			current == digest {
			return source, true
		}
		if s.state.logger != nil {
			s.state.logger.DebugContext(
				ctx,
				"mcp: checked definition differs from the published catalog",
				"scope",
				source.Scope,
			)
		}
	}
	return toolspkg.SourceRef{}, false
}
