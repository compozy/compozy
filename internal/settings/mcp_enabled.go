package settings

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/extensionmcp"
)

type mcpPublishedEnablement interface {
	SetPublishedMCPEnabled(context.Context, MCPAuthTargetRequest, bool) (MCPPublishedDefinition, bool, error)
}

type mcpExtensionEnablement interface {
	SetMCPExtensionEnabled(context.Context, MCPAuthTargetRequest, bool) (MCPExtensionDefinition, error)
}

func (s *service) SetMCPServerEnabled(
	ctx context.Context, req MCPAuthTargetRequest, enabled bool,
) (MCPServerItem, error) {
	s.applyMu.Lock()
	defer s.applyMu.Unlock()
	s.mcpProbeMu.Lock()
	defer func() {
		s.mcpProbeGeneration++
		s.mcpProbeMu.Unlock()
	}()
	scope, workspaceID, err := s.normalizeReadScope(req.Scope, req.WorkspaceID)
	if err != nil {
		return MCPServerItem{}, err
	}
	profileName, err := normalizeMCPSettingsProfileName(scope, workspaceID, req.ProfileName)
	if err != nil {
		return MCPServerItem{}, err
	}
	req.Scope, req.WorkspaceID, req.ProfileName = scope, workspaceID, profileName
	req.Name, req.Owner = strings.TrimSpace(req.Name), strings.TrimSpace(req.Owner)
	if _, err := normalizeMCPAuthTarget(req); err != nil {
		return MCPServerItem{}, err
	}
	if req.Owner != "" && req.Owner != mcpManualOwner {
		if item, found, err := s.setPublishedMCPServerEnabled(ctx, req, enabled); found || err != nil {
			return item, err
		}
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
	entry, ok := mcpEnablementConfigSource(req, sources)
	kind := entry.Target
	if !ok {
		if item, found, err := s.setPublishedMCPServerEnabled(ctx, req, enabled); found || err != nil {
			return item, err
		}
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
	entries := sources[req.Name]
	index := slices.IndexFunc(entries, func(source mcpSourceEntry) bool { return source.Target == kind })
	item := baseMCPServerItem(entry, entries[:index+1], req.Scope, req.WorkspaceID, req.ProfileName)
	err = s.populateMCPServerStatuses(ctx, &item, entry, true, false)
	return item, err
}

func mcpEnablementConfigSource(
	req MCPAuthTargetRequest, sources map[string][]mcpSourceEntry,
) (mcpSourceEntry, bool) {
	if req.DefinitionSource == "" {
		kind, ok := preferredMCPAuthTarget(req, req.Name, sources)
		if !ok {
			return mcpSourceEntry{}, false
		}
		return mcpSourceForTarget(req.Name, kind, sources)
	}
	for _, candidate := range sources[req.Name] {
		if candidate.Server.DefinitionSource == req.DefinitionSource && candidate.Source.Scope == req.Scope &&
			candidate.Source.WorkspaceID == req.WorkspaceID && candidate.Source.ProfileName == req.ProfileName {
			return candidate, true
		}
	}
	return mcpSourceEntry{}, false
}

func (s *service) setPublishedMCPServerEnabled(
	ctx context.Context, req MCPAuthTargetRequest, enabled bool,
) (MCPServerItem, bool, error) {
	management, ok := s.mcpConsent.(mcpPublishedEnablement)
	if !ok {
		return MCPServerItem{}, false, nil
	}
	definition, found, err := management.SetPublishedMCPEnabled(ctx, req, enabled)
	if err != nil || !found {
		return MCPServerItem{}, found, err
	}
	var item mcpCollectionItem
	if req.Owner == "" || req.Owner == mcpManualOwner {
		item = publishedMCPCollectionItem(definition, req)
	} else {
		item = extensionMCPCollectionItem(MCPExtensionDefinition{
			Target: definition.Target, Server: definition.Server,
		})
	}
	err = s.populateMCPServerStatuses(ctx, &item.item, item.entry, true, false)
	return item.item, true, err
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
