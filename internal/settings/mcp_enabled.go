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
	defer s.mcpProbeMu.Unlock()
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
		_, err = compozyconfig.SetMCPSidecarServerDisabled(ctx, s.homePaths, root, target, req.Name, !enabled)
	} else {
		_, err = compozyconfig.EditConfigOverlay(
			s.homePaths,
			root,
			target,
			func(editor *compozyconfig.OverlayEditor) error {
				if err := ctx.Err(); err != nil {
					return err
				}
				return editor.SetMCPServerDisabled(req.Name, !enabled)
			},
		)
	}
	if err != nil {
		return MCPServerItem{}, fmt.Errorf(
			"settings: write MCP server %q enablement at %s scope: %w",
			req.Name,
			req.Scope,
			err,
		)
	}
	s.mcpProbeGeneration++
	if err := s.applyManualMCPEnablement(ctx, req, kind, target.Path()); err != nil {
		return MCPServerItem{}, err
	}
	entries := sources[req.Name]
	index := slices.IndexFunc(entries, func(source mcpSourceEntry) bool { return source.Target == kind })
	item := baseMCPServerItem(entry, entries[:index+1], req.Scope, req.WorkspaceID, req.ProfileName)
	s.enrichMCPEnablementResult(ctx, &item, entry)
	return item, nil
}

func (s *service) applyManualMCPEnablement(
	ctx context.Context, req MCPAuthTargetRequest, kind WriteTargetKind, path string,
) error {
	result := mutationResultAtPath(
		mutationResultForCollection(CollectionMCPServers, req.Scope, req.WorkspaceID, kind),
		path,
	)
	result.ProfileName = req.ProfileName
	result = applyCollectionLifecycle(result, CollectionMCPServers, collectionMutationPut, true)
	applied, err := s.recordMutationApply(ctx, result)
	if err != nil {
		return fmt.Errorf(
			"settings: apply persisted MCP server %q enablement at %s scope: %w",
			req.Name,
			req.Scope,
			err,
		)
	}
	if !applied.Applied {
		return unavailableError(fmt.Errorf(
			"settings: enablement for MCP server %q at %s scope was saved and could not be applied",
			req.Name,
			req.Scope,
		))
	}
	if _, err := s.finalizeCommittedCollectionMutation(ctx, result, nil, "replace", false); err != nil {
		return fmt.Errorf(
			"settings: finalize persisted MCP server %q enablement at %s scope: %w",
			req.Name,
			req.Scope,
			err,
		)
	}
	return nil
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
	s.mcpProbeGeneration++
	var item mcpCollectionItem
	if req.Owner == "" || req.Owner == mcpManualOwner {
		item = publishedMCPCollectionItem(definition, req)
	} else {
		item = extensionMCPCollectionItem(&MCPExtensionDefinition{
			Target: definition.Target, ManagementTarget: definition.ManagementTarget, Server: definition.Server,
		})
	}
	s.enrichMCPEnablementResult(ctx, &item.item, item.entry)
	return item.item, true, nil
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
	s.mcpProbeGeneration++
	item := extensionMCPCollectionItem(&definition)
	s.enrichMCPEnablementResult(ctx, &item.item, item.entry)
	return item.item, nil
}

func (s *service) enrichMCPEnablementResult(
	ctx context.Context, item *MCPServerItem, entry mcpSourceEntry,
) {
	if err := s.populateMCPServerStatuses(ctx, item, entry, true, false); err != nil {
		status := MCPServerRuntimeStatus{
			Configured: true, State: MCPServerRuntimeStateUnknown,
			Probe: MCPServerProbeSkipped, Reason: "not_observed",
		}
		if item.Disabled {
			status = OffMCPServerRuntimeStatus()
		}
		setMCPRuntimeObservation(item, entry.Server, status)
	}
}
