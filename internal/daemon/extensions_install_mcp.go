package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/extensionmcp"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/store"
)

func (s *daemonExtensionService) prepareInstallMCPAllocations(
	ctx context.Context, prepared preparedDaemonExtensionInstall, requested string,
) (*extensionMCPAllocationSnapshot, error) {
	requested = strings.TrimSpace(requested)
	if s.mcpAllocations == nil {
		if requested != "" {
			return nil, errors.New("daemon: MCP allocation store is unavailable")
		}
		return nil, nil
	}
	if _, err := s.registry.Get(prepared.name); err == nil {
		return nil, extensionpkg.ErrExtensionExists
	} else if !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
		return nil, err
	}
	records, err := s.mcpAllocations.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	plan := snapshotExtensionMCPAllocations(extensionpkg.GlobalInstanceKey(prepared.name), records)
	if requested == "" {
		return plan, nil
	}
	if prepared.manifest == nil || len(prepared.manifest.Resources.MCPServers) != 1 {
		return nil, &extensionpkg.ManifestValidationError{
			Field:   "runtime_name",
			Message: "runtime_name requires a single-server extension",
		}
	}
	if err := compozyconfig.ValidateMCPServerName(requested); err != nil {
		return nil, &extensionpkg.ManifestValidationError{Field: "runtime_name", Message: err.Error()}
	}
	occupied, err := s.installMCPManualNames(ctx)
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		if record.ProfileID == store.DefaultProfileID && record.WorkspaceID != "" {
			occupied = append(occupied, record.RuntimeName)
		}
	}
	var serverName string
	for name := range prepared.manifest.Resources.MCPServers {
		serverName = name
	}
	_, err = s.mcpAllocations.Reserve(ctx, extensionmcp.Target{
		Extension: prepared.name, ProfileID: store.DefaultProfileID, ServerName: serverName,
	}, requested, occupied)
	if err != nil {
		return nil, err
	}
	return plan, nil
}

func (s *daemonExtensionService) installMCPManualNames(ctx context.Context) ([]string, error) {
	if s.resourceStore == nil {
		return nil, errors.New("daemon: MCP resource registry is unavailable")
	}
	records, err := s.resourceStore.ListRaw(
		ctx,
		s.resourceActor,
		resources.ResourceFilter{Kind: compozyconfig.MCPServerResourceKind},
	)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, record := range records {
		if record.Owner.Normalize().Kind == extensionResourceOwnerKind || !installMCPDefaultProfileScope(record.Scope) {
			continue
		}
		var server compozyconfig.MCPServer
		if err := json.Unmarshal(record.SpecJSON, &server); err != nil {
			return nil, fmt.Errorf("daemon: decode manual MCP identity: %w", err)
		}
		names = append(names, server.EffectiveRuntimeName())
	}
	return names, nil
}

func installMCPDefaultProfileScope(scope resources.ResourceScope) bool {
	scope = scope.Normalize()
	switch scope.Kind {
	case resources.ResourceScopeKindUser, resources.ResourceScopeKindWorkspace:
		return true
	case resources.ResourceScopeKindProfile:
		return scope.ID == store.DefaultProfileID
	case resources.ResourceScopeKindWorkspaceProfile:
		_, profile, valid := strings.Cut(scope.ID, "@pf:")
		return valid && profile == daemonDefaultProfileName
	default:
		return false
	}
}

func (s *daemonExtensionService) rollbackInstallMCPAllocations(
	ctx context.Context,
	plan *extensionMCPAllocationSnapshot,
) error {
	if plan == nil || s.mcpAllocations == nil {
		return nil
	}
	rollbackCtx, cancel := extensionSecretRollbackContext(ctx)
	defer cancel()
	// Preserve reservations if package/registry compensation itself failed.
	if _, err := s.registry.Get(plan.key.Name); err == nil {
		return nil
	} else if !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
		return err
	}
	return s.rollbackExtensionMCPAllocations(rollbackCtx, plan)
}
