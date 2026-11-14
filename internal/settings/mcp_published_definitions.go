package settings

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/config"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
)

// MCPPublishedDefinition exposes agent and skill sidecars absent from config layers.
type MCPPublishedDefinition struct {
	Target mcpauth.Target
	Server config.MCPServer
}

type MCPPublishedDefinitionProvider interface {
	PublishedMCPDefinitions(context.Context, MCPAuthTargetRequest) ([]MCPPublishedDefinition, error)
}

func (s *service) publishedMCPDefinitions(ctx context.Context, req MCPAuthTargetRequest) ([]MCPPublishedDefinition, error) {
	provider, ok := s.mcpConsent.(MCPPublishedDefinitionProvider)
	if !ok {
		return nil, nil
	}
	return provider.PublishedMCPDefinitions(ctx, req)
}

func (s *service) appendPublishedMCPItems(ctx context.Context, items []mcpCollectionItem, req MCPAuthTargetRequest) ([]mcpCollectionItem, error) {
	definitions, err := s.publishedMCPDefinitions(ctx, req)
	if err != nil {
		return nil, err
	}
	for _, definition := range definitions {
		found := false
		for _, item := range items {
			if item.entry.Server.DefinitionSource == definition.Server.DefinitionSource && item.item.Name == definition.Server.Name {
				found = true
				break
			}
		}
		if found {
			continue
		}
		entry := mcpSourceEntry{Server: definition.Server, AuthTarget: &definition.Target, Source: SourceRef{Kind: SourceKindWorkspaceAgentFile, Scope: req.Scope, WorkspaceID: req.WorkspaceID, ProfileName: req.ProfileName}}
		item := baseMCPServerItem(entry, []mcpSourceEntry{entry}, req.Scope, req.WorkspaceID, req.ProfileName)
		item.SourceMetadata.AvailableTargets = []WriteTargetKind{}
		items = append(items, mcpCollectionItem{item: item, entry: entry})
	}
	return items, nil
}

func (s *service) resolvePublishedMCPDefinition(ctx context.Context, req MCPAuthTargetRequest) (mcpAuthDefinition, bool, error) {
	definitions, err := s.publishedMCPDefinitions(ctx, req)
	if err != nil {
		return mcpAuthDefinition{}, false, err
	}
	var result mcpAuthDefinition
	found := false
	for _, definition := range definitions {
		if definition.Server.Name != req.Name {
			continue
		}
		if found {
			return result, false, conflictError(fmt.Errorf("MCP server %q has multiple published definitions", req.Name))
		}
		result = mcpAuthDefinition{target: definition.Target, server: definition.Server}
		found = true
	}
	return result, found, nil
}

func mcpApprovalNotFound(name string, err error) error {
	if !errors.Is(err, ErrNotFound) {
		return err
	}
	return &MCPApprovalError{Code: "mcp_server_not_found", Message: fmt.Sprintf("No MCP server named %s in CompozyOS", name)}
}

// mcpApprovalDefinitions keeps same-name declarations isolated by their declaring file.
func (s *service) mcpApprovalDefinitions(ctx context.Context, req MCPAuthTargetRequest) ([]mcpAuthDefinition, error) {
	published, err := s.publishedMCPDefinitions(ctx, req)
	if err != nil {
		return nil, err
	}
	var definitions []mcpAuthDefinition
	for _, definition := range published {
		if definition.Server.Name == req.Name {
			definitions = append(definitions, mcpAuthDefinition{target: definition.Target, server: definition.Server})
		}
	}
	resolved, err := s.resolveMCPAuthDefinition(ctx, req)
	if err != nil && !errors.Is(err, ErrNotFound) && len(definitions) == 0 {
		return nil, err
	}
	if err == nil {
		found := false
		for _, definition := range definitions {
			if definition.server.DefinitionSource == resolved.server.DefinitionSource {
				found = true
				break
			}
		}
		if !found {
			definitions = append(definitions, resolved)
		}
	}
	if len(definitions) == 0 {
		return nil, mcpApprovalNotFound(req.Name, notFoundError(fmt.Errorf("MCP definition unavailable")))
	}
	return definitions, nil
}

func selectMCPApprovalDefinition(name string, definitions []mcpAuthDefinition, digest string) (mcpAuthDefinition, error) {
	var selected mcpAuthDefinition
	matches := 0
	for _, definition := range definitions {
		current, err := config.MCPDefinitionDigest(definition.server)
		if err != nil {
			return selected, err
		}
		if current == digest {
			selected = definition
			matches++
		}
	}
	if matches > 1 {
		return mcpAuthDefinition{}, conflictError(fmt.Errorf("MCP server %q has multiple declarations with this digest; use distinct names", name))
	}
	if matches == 0 {
		return mcpAuthDefinition{}, &MCPApprovalError{Code: "mcp_approval_definition_changed", Message: fmt.Sprintf("%s changed since you looked at it. List it again and approve the new definition", name)}
	}
	return selected, nil
}
