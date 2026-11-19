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
	Target           mcpauth.Target
	ManagementTarget mcpauth.Target
	Server           config.MCPServer
}

type MCPPublishedDefinitionProvider interface {
	PublishedMCPDefinitions(context.Context, MCPAuthTargetRequest) ([]MCPPublishedDefinition, error)
}

func (s *service) publishedMCPDefinitions(
	ctx context.Context,
	req MCPAuthTargetRequest,
) ([]MCPPublishedDefinition, error) {
	provider, ok := s.mcpConsent.(MCPPublishedDefinitionProvider)
	if !ok {
		return nil, nil
	}
	return provider.PublishedMCPDefinitions(ctx, req)
}

func (s *service) appendPublishedMCPItems(
	ctx context.Context,
	items []mcpCollectionItem,
	req MCPAuthTargetRequest,
) ([]mcpCollectionItem, error) {
	definitions, err := s.publishedMCPDefinitions(ctx, req)
	if err != nil {
		return nil, err
	}
	for _, definition := range definitions {
		found := false
		for i := range items {
			item := &items[i]
			if item.entry.Server.DefinitionSource == definition.Server.DefinitionSource &&
				item.item.Name == definition.Server.Name {
				found = true
				break
			}
		}
		if found {
			continue
		}
		items = append(items, publishedMCPCollectionItem(definition, req))
	}
	return items, nil
}

func publishedMCPCollectionItem(definition MCPPublishedDefinition, req MCPAuthTargetRequest) mcpCollectionItem {
	target := definition.ManagementTarget
	if target.Scope == "" {
		target = definition.Target
	}
	source := mcpAuthTargetRequest(target)
	kind := SourceKindGlobalAgentFile
	if source.WorkspaceID != "" {
		kind = SourceKindWorkspaceAgentFile
	}
	entry := mcpSourceEntry{
		Server: definition.Server, AuthTarget: &definition.Target,
		Source: SourceRef{
			Kind: kind, Scope: source.Scope, WorkspaceID: source.WorkspaceID, ProfileName: source.ProfileName,
		},
	}
	item := baseMCPServerItem(entry, []mcpSourceEntry{entry}, req.Scope, req.WorkspaceID, req.ProfileName)
	item.SourceMetadata.AvailableTargets = []WriteTargetKind{}
	return mcpCollectionItem{item: item, entry: entry}
}

func (s *service) resolvePublishedMCPDefinition(
	ctx context.Context, req MCPAuthTargetRequest, requested mcpauth.Target,
) (mcpAuthDefinition, bool, error) {
	definitions, err := s.publishedMCPDefinitions(ctx, req)
	if err != nil {
		return mcpAuthDefinition{}, false, err
	}
	var result mcpAuthDefinition
	bestRank, matches := -1, 0
	for _, definition := range definitions {
		if definition.Server.Name != req.Name {
			continue
		}
		rank := definition.ScopePrecedence()
		if rank < bestRank {
			continue
		}
		if rank > bestRank {
			result = publishedMCPAuthDefinition(definition, requested)
			bestRank, matches = rank, 0
		}
		matches++
	}
	if matches > 1 {
		return result, false, conflictError(fmt.Errorf("MCP server %q has multiple published definitions", req.Name))
	}
	return result, matches == 1, nil
}

// ScopePrecedence ranks the visible publication independently of its OAuth owner.
func (d *MCPPublishedDefinition) ScopePrecedence() int {
	scope := d.ManagementTarget.Scope
	if scope == "" {
		scope = d.Target.Scope
	}
	return publishedMCPScopeRank(scope)
}

func publishedMCPScopeRank(scope mcpauth.Scope) int {
	switch scope {
	case mcpauth.ScopeWorkspaceProfile:
		return 3
	case mcpauth.ScopeWorkspace:
		return 2
	case mcpauth.ScopeProfile:
		return 1
	default:
		return 0
	}
}

func publishedMCPAuthDefinition(definition MCPPublishedDefinition, requested mcpauth.Target) mcpAuthDefinition {
	return mcpAuthDefinition{
		target: definition.Target, server: definition.Server,
		inherited:  !mcpAuthTargetCovers(requested, definition.Target.Normalize()),
		precedence: definition.ScopePrecedence(),
	}
}

func mcpApprovalNotFound(name string, err error) error {
	if !errors.Is(err, ErrNotFound) {
		return err
	}
	return &MCPApprovalError{
		Code:    mcpServerNotFoundCode,
		Message: fmt.Sprintf("No MCP server named %s in CompozyOS", name),
	}
}

// mcpApprovalDefinitions keeps same-name declarations isolated by their declaring file.
func (s *service) mcpApprovalDefinitions(ctx context.Context, req MCPAuthTargetRequest) ([]mcpAuthDefinition, error) {
	published, err := s.publishedMCPDefinitions(ctx, req)
	if err != nil {
		return nil, err
	}
	requested, err := normalizeMCPAuthTarget(req)
	if err != nil {
		return nil, err
	}
	var definitions []mcpAuthDefinition
	for _, definition := range published {
		if definition.Server.Name == req.Name {
			definitions = append(definitions, publishedMCPAuthDefinition(definition, requested))
		}
	}
	resolved, err := s.resolveMCPAuthDefinition(ctx, req)
	if err != nil && !errors.Is(err, ErrNotFound) && len(definitions) == 0 {
		return nil, err
	}
	if err == nil {
		found := false
		for i := range definitions {
			if definitions[i].server.DefinitionSource == resolved.server.DefinitionSource {
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

func (s *service) selectMCPApprovalDefinition(
	ctx context.Context, name, profile string,
	definitions []mcpAuthDefinition,
	digest string,
) (mcpAuthDefinition, error) {
	if s.mcpConsent == nil {
		return mcpAuthDefinition{}, unavailableError(fmt.Errorf("settings: MCP consent is unavailable"))
	}
	var selected mcpAuthDefinition
	matches := 0
	for i := range definitions {
		definition := &definitions[i]
		req := consentRequestForTarget(definition.target, definition.server)
		req.ProfileName = profile
		decision, err := s.mcpConsent.MCPConsent(ctx, req)
		if err != nil {
			return selected, err
		}
		if decision.DefinitionDigest == digest {
			selected = *definition
			matches++
		}
	}
	if matches > 1 {
		return mcpAuthDefinition{}, conflictError(
			fmt.Errorf("MCP server %q has multiple declarations with this digest; use distinct names", name),
		)
	}
	if matches == 0 {
		return mcpAuthDefinition{}, &MCPApprovalError{
			Code: "mcp_approval_definition_changed",
			Message: fmt.Sprintf(
				"%s changed since you looked at it. List it again and approve the new definition",
				name,
			),
		}
	}
	return selected, nil
}
