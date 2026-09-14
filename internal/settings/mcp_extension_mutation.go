package settings

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/config/lifecycle"
	"github.com/compozy/compozy/internal/extensionmcp"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
)

func (s *service) extensionMCPMutationTarget(
	ctx context.Context,
	req MCPAuthTargetRequest,
) (mcpauth.Target, bool, error) {
	target, _, err := s.resolveMCPAuthTarget(ctx, req)
	if errors.Is(err, ErrNotFound) && (req.Owner == "" || req.Owner == "manual") {
		return mcpauth.Target{}, false, nil
	}
	if err != nil {
		return mcpauth.Target{}, false, err
	}
	return target, strings.HasPrefix(target.Owner, "extension:"), nil
}

func (s *service) putExtensionMCPOverride(
	ctx context.Context,
	target mcpauth.Target,
	req CollectionItemPutRequest,
) (MutationResult, error) {
	server := req.MCPServer
	if server == nil {
		return MutationResult{}, validationError(errors.New("settings: MCP override payload is required"))
	}
	if server.Transport != "" || server.Command != "" || server.CWD != "" || len(server.Args) != 0 ||
		!server.Auth.IsZero() ||
		len(server.SecretEnv) != 0 || len(server.SecretHeaders) != 0 || !req.MCPSecrets.Empty() ||
		len(req.MCPSecretPreservation.SecretEnv) != 0 ||
		req.MCPSecretPreservation.OAuthClientSecret ||
		len(req.MCPEnvPreservation) != 0 {
		return MutationResult{}, validationError(
			errors.New("settings: extension MCP overrides accept only env, headers, and url"),
		)
	}
	return s.updateExtensionMCPOverride(
		ctx,
		target,
		req.Target,
		extensionmcp.Override{Env: server.Env, Headers: server.Headers, URL: server.URL},
	)
}

func (s *service) updateExtensionMCPOverride(
	ctx context.Context,
	target mcpauth.Target,
	selector TargetSelector,
	override extensionmcp.Override,
) (MutationResult, error) {
	if selector != "" && selector != TargetAuto {
		return MutationResult{}, validationError(
			errors.New("settings: extension MCP overrides have no config-file target"),
		)
	}
	if s.mcpExtensionManagement == nil {
		return MutationResult{}, unavailableError(errors.New("settings: extension MCP management is unavailable"))
	}
	if err := override.Validate(); err != nil {
		return MutationResult{}, validationError(err)
	}
	req := mcpAuthTargetRequest(target)
	definition, err := s.mcpExtensionManagement.UpdateMCPExtensionOverride(ctx, req, override)
	if errors.Is(err, extensionmcp.ErrNotFound) {
		return MutationResult{}, notFoundError(err)
	}
	if err != nil {
		return MutationResult{}, err
	}
	item := cloneMCPServerItem(extensionMCPCollectionItem(definition).item)
	return MutationResult{
		Section: SectionName(
			CollectionMCPServers,
		),
		Scope:       req.Scope,
		WorkspaceID: req.WorkspaceID,
		ProfileName: req.ProfileName,
		Behavior:    MutationBehaviorAppliedNow,
		Applied:     true,
		Lifecycle:   lifecycle.Live,
		DiffClass:   lifecycle.DiffClass(lifecycle.Live),
		MCPServer:   &item,
	}, nil
}

func (s *service) deleteMCPCollectionItem(
	ctx context.Context,
	scope ScopeKind,
	workspaceID, profileName, name string,
	req CollectionItemDeleteRequest,
) (MutationResult, error) {
	target, owned, err := s.extensionMCPMutationTarget(
		ctx,
		MCPAuthTargetRequest{
			Scope:       scope,
			WorkspaceID: workspaceID,
			ProfileName: profileName,
			Name:        name,
			Owner:       req.Owner,
		},
	)
	if err != nil {
		return MutationResult{}, err
	}
	if owned {
		return s.updateExtensionMCPOverride(ctx, target, req.Target, extensionmcp.Override{})
	}
	return s.deleteMCPServer(ctx, scope, workspaceID, profileName, name, req.Target)
}

func isExtensionMCPMutation(result MutationResult) bool {
	return result.MCPServer != nil && strings.HasPrefix(result.MCPServer.Owner, "extension:")
}

// recordExtensionMCPApply records an already-reconciled override without applying pending config.toml changes.
func (s *service) recordExtensionMCPApply(ctx context.Context, result MutationResult) (ApplyResult, error) {
	state, err := s.ensureActiveConfigState(ctx)
	if err != nil {
		return ApplyResult{}, err
	}
	desiredHash, _, err := s.currentDesiredConfigHash()
	if err != nil {
		return ApplyResult{}, fmt.Errorf("settings: read desired config after MCP override: %w", err)
	}
	record, err := s.createTerminalApplyRecord(ctx, applyRecordInput{
		desiredHash: desiredHash, activeHash: state.hash, generation: state.generation,
		lifecycle: lifecycle.Live, status: lifecycle.StatusApplied, appliedAtNow: true,
	})
	if err != nil {
		return ApplyResult{}, err
	}
	return ApplyResult{
		Record:      record,
		Section:     result.Section,
		Scope:       result.Scope,
		WorkspaceID: result.WorkspaceID,
		ProfileName: result.ProfileName,
		Applied:     true,
		NextAction:  lifecycle.NextActionNone,
		Warnings:    result.Warnings,
		MCPServer:   result.MCPServer,
	}, nil
}
