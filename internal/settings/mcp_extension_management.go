package settings

import (
	"context"
	"errors"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/extensionmcp"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
)

var ErrMCPServerNameTaken = errors.New("mcp_server_name_taken")

// MCPExtensionDefinition is an owner-qualified published server with editable override state.
type MCPExtensionDefinition struct {
	Target           mcpauth.Target
	ManagementTarget mcpauth.Target
	Server           compozyconfig.MCPServer
	Override         extensionmcp.Override
}

// MCPExtensionManagement reads published definitions and protects retained runtime allocations.
type MCPExtensionManagement interface {
	ListMCPExtensionDefinitions(context.Context, MCPAuthTargetRequest) ([]MCPExtensionDefinition, error)
	ValidateManualMCPName(context.Context, MCPAuthTargetRequest) error
	UpdateMCPExtensionOverride(
		context.Context,
		MCPAuthTargetRequest,
		extensionmcp.Override,
	) (MCPExtensionDefinition, error)
}

func extensionMCPCollectionItem(definition *MCPExtensionDefinition) mcpCollectionItem {
	target := definition.ManagementTarget
	if target.Scope == "" {
		target = definition.Target
	}
	req := mcpAuthTargetRequest(target)
	authTarget, override := definition.Target, definition.Override
	entry := mcpSourceEntry{
		Server:     definition.Server,
		AuthTarget: &authTarget,
		Source: SourceRef{
			Kind:        SourceKindExtension,
			Scope:       req.Scope,
			WorkspaceID: req.WorkspaceID,
			ProfileName: req.ProfileName,
		},
	}
	item := baseMCPServerItem(entry, []mcpSourceEntry{entry}, req.Scope, req.WorkspaceID, req.ProfileName)
	item.SourceMetadata.AvailableTargets = []WriteTargetKind{}
	item.Override = &override
	return mcpCollectionItem{item: item, entry: entry}
}

func (s *service) appendExtensionMCPItems(
	ctx context.Context, items []mcpCollectionItem, req MCPAuthTargetRequest,
) ([]mcpCollectionItem, error) {
	if s.mcpExtensionManagement == nil {
		return items, nil
	}
	definitions, err := s.mcpExtensionManagement.ListMCPExtensionDefinitions(ctx, req)
	if err != nil {
		return nil, err
	}
	for i := range definitions {
		items = append(items, extensionMCPCollectionItem(&definitions[i]))
	}
	return items, nil
}

func (s *service) validateManualMCPName(ctx context.Context, req MCPAuthTargetRequest) error {
	if s.mcpExtensionManagement == nil {
		return nil
	}
	if err := s.mcpExtensionManagement.ValidateManualMCPName(ctx, req); err != nil {
		if errors.Is(err, extensionmcp.ErrNameTaken) {
			return unprocessableError(ErrMCPServerNameTaken)
		}
		return err
	}
	return nil
}
