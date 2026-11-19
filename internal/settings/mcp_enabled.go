package settings

import (
	"context"
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/extensionmcp"
)

type mcpExtensionEnablement interface {
	SetMCPExtensionEnabled(context.Context, MCPAuthTargetRequest, bool) (MCPExtensionDefinition, error)
}

func (s *service) SetMCPServerEnabled(
	ctx context.Context, req MCPAuthTargetRequest, enabled bool,
) (MCPServerItem, error) {
	s.applyMu.Lock()
	defer s.applyMu.Unlock()
	scope, workspaceID, err := s.normalizeReadScope(req.Scope, req.WorkspaceID)
	if err != nil {
		return MCPServerItem{}, err
	}
	profileName, err := normalizeSettingsProfileName(scope, req.ProfileName)
	if err != nil {
		return MCPServerItem{}, err
	}
	req.Scope, req.WorkspaceID, req.ProfileName = scope, workspaceID, profileName
	req.Name, req.Owner = strings.TrimSpace(req.Name), strings.TrimSpace(req.Owner)
	if _, err := normalizeMCPAuthTarget(req); err != nil {
		return MCPServerItem{}, err
	}
	if req.Owner != "" && req.Owner != mcpManualOwner {
		return s.setExtensionMCPServerEnabled(ctx, req, enabled)
	}
	return s.setManualMCPServerEnabled(ctx, req, enabled)
}

func (s *service) setManualMCPServerEnabled(
	ctx context.Context, req MCPAuthTargetRequest, enabled bool,
) (MCPServerItem, error) {
	s.mcpMutationMu.Lock()
	defer s.mcpMutationMu.Unlock()
	root, sources, err := s.resolveMCPTargetContext(ctx, req.Scope, req.WorkspaceID, req.ProfileName)
	if err != nil {
		return MCPServerItem{}, err
	}
	kind, ok := preferredMCPAuthTarget(req, req.Name, sources)
	if !ok {
		return MCPServerItem{}, notFoundError(
			fmt.Errorf("settings: MCP server %q has no declaration in this scope", req.Name),
		)
	}
	entry, ok := mcpSourceForTarget(req.Name, kind, sources)
	if !ok {
		return MCPServerItem{}, notFoundError(
			fmt.Errorf("settings: MCP server %q has no declaration in this file", req.Name),
		)
	}
	if _, err := s.ensureActiveConfigState(ctx); err != nil {
		return MCPServerItem{}, err
	}
	sidecar := isMCPDefinitionSidecarTarget(kind) || kind == WriteTargetWorkspaceProfileMCPSidecar
	target, err := compozyconfig.ResolveMCPEnabledWriteTarget(
		s.homePaths, root, req.Scope.configWriteScope(), req.ProfileName, sidecar,
	)
	if err != nil {
		return MCPServerItem{}, err
	}
	entry.Server.Disabled = new(!enabled)
	if sidecar {
		_, err = compozyconfig.SetMCPSidecarServerDisabled(s.homePaths, root, target, req.Name, !enabled)
	} else {
		_, err = compozyconfig.EditConfigOverlay(
			s.homePaths,
			root,
			target,
			func(editor *compozyconfig.OverlayEditor) error {
				return editor.SetMCPServerDisabled(req.Name, !enabled)
			},
		)
	}
	if err != nil {
		return MCPServerItem{}, err
	}
	result := mutationResultAtPath(
		mutationResultForCollection(CollectionMCPServers, req.Scope, req.WorkspaceID, kind),
		target.Path(),
	)
	result.ProfileName = req.ProfileName
	result = applyCollectionLifecycle(result, CollectionMCPServers, collectionMutationPut, true)
	applied, err := s.recordMutationApply(ctx, result)
	if err != nil {
		return MCPServerItem{}, err
	}
	if !applied.Applied {
		return MCPServerItem{}, unavailableError(errors.New("settings: MCP server state could not be applied"))
	}
	if _, err := s.finalizeCommittedCollectionMutation(ctx, result, nil, "replace", false); err != nil {
		return MCPServerItem{}, err
	}
	item := baseMCPServerItem(entry, []mcpSourceEntry{entry}, req.Scope, req.WorkspaceID, req.ProfileName)
	err = s.populateMCPServerStatuses(ctx, &item, entry, true, false)
	return item, err
}

func (s *service) setExtensionMCPServerEnabled(
	ctx context.Context, req MCPAuthTargetRequest, enabled bool,
) (MCPServerItem, error) {
	management, ok := s.mcpExtensionManagement.(mcpExtensionEnablement)
	if !ok {
		return MCPServerItem{}, unavailableError(errors.New("settings: extension MCP management is unavailable"))
	}
	definition, err := management.SetMCPExtensionEnabled(ctx, req, enabled)
	if errors.Is(err, extensionmcp.ErrNotFound) {
		return MCPServerItem{}, notFoundError(err)
	}
	if err != nil {
		return MCPServerItem{}, err
	}
	item := extensionMCPCollectionItem(definition)
	err = s.populateMCPServerStatuses(ctx, &item.item, item.entry, true, false)
	return item.item, err
}
