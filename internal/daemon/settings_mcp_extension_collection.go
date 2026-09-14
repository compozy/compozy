package daemon

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/extensionmcp"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/store"
)

func (s settingsMCPExtensionDefinitions) ListMCPExtensionDefinitions(
	ctx context.Context, req settingspkg.MCPAuthTargetRequest,
) ([]settingspkg.MCPExtensionDefinition, error) {
	definitions := []settingspkg.MCPExtensionDefinition{}
	if s.state == nil || s.state.mcpServerCatalog == nil {
		return definitions, nil
	}
	allocations, err := s.allocations(ctx)
	if err != nil {
		return nil, err
	}
	byTarget := make(map[extensionmcp.Target]extensionmcp.Override, len(allocations))
	for _, record := range allocations {
		byTarget[record.Target] = record.Override
	}
	for _, record := range s.state.mcpServerCatalog.Snapshot() {
		owner := record.Owner.Normalize()
		if owner.Kind != extensionResourceOwnerKind {
			continue
		}
		if req.Owner != "" && req.Owner != "extension:"+owner.ID {
			continue
		}
		target, err := mcpAuthTargetForResource(ctx, s.state, record.Scope, record.Spec.Name, owner)
		if err != nil {
			return nil, err
		}
		if !settingsMCPExtensionTargetVisible(req, target) {
			continue
		}
		server, err := projectExtensionSecretHeaders(ctx, s.state, record, "")
		if err != nil {
			return nil, err
		}
		profileID, workspaceID, err := mcpExtensionBindingOwner(ctx, s.state, record.Scope)
		if err != nil {
			return nil, err
		}
		allocation := extensionmcp.Target{
			Extension:   owner.ID,
			ProfileID:   profileID,
			WorkspaceID: workspaceID,
			ServerName:  record.Spec.Name,
		}
		definitions = append(definitions, settingspkg.MCPExtensionDefinition{
			Target: target, Server: server, Override: byTarget[allocation],
		})
	}
	return definitions, nil
}

func settingsMCPExtensionTargetVisible(req settingspkg.MCPAuthTargetRequest, target mcpauth.Target) bool {
	if settingsMCPExtensionTargetMatches(req, target) {
		return true
	}
	if strings.TrimSpace(req.WorkspaceID) == "" {
		return false
	}
	parent := req
	parent.WorkspaceID = ""
	if parent.Scope == settingspkg.ScopeWorkspace {
		parent.Scope = settingspkg.ScopeUser
	}
	return settingsMCPExtensionTargetMatches(parent, target)
}

func (s settingsMCPExtensionDefinitions) allocations(ctx context.Context) ([]extensionmcp.Record, error) {
	if s.state == nil || s.state.extensionMCP == nil {
		return nil, nil
	}
	return s.state.extensionMCP.ListAll(ctx)
}

// ValidateManualMCPName protects retained allocations, including disabled extensions.
func (s settingsMCPExtensionDefinitions) ValidateManualMCPName(
	ctx context.Context, req settingspkg.MCPAuthTargetRequest,
) error {
	allocations, err := s.allocations(ctx)
	if err != nil {
		return err
	}
	profileID := store.DefaultProfileID
	if req.Scope == settingspkg.ScopeProfile {
		if s.state == nil || s.state.profiles == nil {
			return errors.New("daemon: profile catalog is required for MCP name validation")
		}
		profileID, err = s.state.profiles.AvailableProfileID(ctx, req.ProfileName)
		if err != nil {
			return err
		}
	}
	for _, record := range allocations {
		if record.RuntimeName != strings.TrimSpace(req.Name) {
			continue
		}
		if req.Scope == settingspkg.ScopeProfile && record.ProfileID != profileID {
			continue
		}
		if req.Scope == settingspkg.ScopeWorkspace && record.WorkspaceID == "" &&
			record.ProfileID != store.DefaultProfileID {
			continue
		}
		workspaceID := strings.TrimSpace(req.WorkspaceID)
		if workspaceID != "" && record.WorkspaceID != "" && workspaceID != record.WorkspaceID {
			continue
		}
		return extensionmcp.ErrNameTaken
	}
	return nil
}
