package settings

import (
	"context"
	"errors"
	"fmt"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	mcpauth "github.com/compozy/agh/internal/mcp/auth"
)

// MCPAuthTargetRequest selects one exact global or workspace MCP definition.
type MCPAuthTargetRequest struct {
	Scope       ScopeKind
	WorkspaceID string
	Name        string
}

// MCPAuthBeginRequest starts one daemon-owned OAuth session.
type MCPAuthBeginRequest struct {
	MCPAuthTargetRequest
	CallbackURL string
}

// MCPAuthExchangeRequest completes one daemon-owned OAuth session.
type MCPAuthExchangeRequest struct {
	MCPAuthTargetRequest
	Code        string
	RedirectURL string
}

// BeginMCPAuth starts a PKCE flow for one exact scoped MCP definition.
func (s *service) BeginMCPAuth(
	ctx context.Context,
	req MCPAuthBeginRequest,
) (mcpauth.BeginResult, error) {
	if s.mcpAuth == nil {
		return mcpauth.BeginResult{}, unavailableError(errors.New("settings: MCP auth runtime is unavailable"))
	}
	target, server, err := s.resolveMCPAuthTarget(ctx, req.MCPAuthTargetRequest)
	if err != nil {
		return mcpauth.BeginResult{}, err
	}
	result, err := s.mcpAuth.MCPAuthBegin(ctx, target, server, strings.TrimSpace(req.CallbackURL))
	if err != nil {
		return mcpauth.BeginResult{}, fmt.Errorf("settings: begin MCP auth for %q: %w", target.ServerName, err)
	}
	return result, nil
}

// ExchangeMCPAuth consumes one target's active PKCE session.
func (s *service) ExchangeMCPAuth(
	ctx context.Context,
	req MCPAuthExchangeRequest,
) (mcpauth.Status, error) {
	if s.mcpAuth == nil {
		return mcpauth.Status{}, unavailableError(errors.New("settings: MCP auth runtime is unavailable"))
	}
	target, server, err := s.resolveMCPAuthTarget(ctx, req.MCPAuthTargetRequest)
	if err != nil {
		return mcpauth.Status{}, err
	}
	status, err := s.mcpAuth.MCPAuthExchange(ctx, target, server, mcpauth.ExchangeInput{
		Code: req.Code, RedirectURL: req.RedirectURL,
	})
	if err != nil {
		return mcpauth.Status{}, classifyMCPAuthExchangeError(
			fmt.Errorf("settings: exchange MCP auth for %q: %w", target.ServerName, err),
		)
	}
	return status, nil
}

// CompleteMCPAuthCallback consumes one session selected by its random OAuth state.
func (s *service) CompleteMCPAuthCallback(ctx context.Context, callbackURL string) (mcpauth.Status, error) {
	if s.mcpAuth == nil {
		return mcpauth.Status{}, unavailableError(errors.New("settings: MCP auth runtime is unavailable"))
	}
	trimmedCallbackURL := strings.TrimSpace(callbackURL)
	target, err := s.mcpAuth.MCPAuthCallbackTarget(trimmedCallbackURL)
	if err != nil {
		return mcpauth.Status{}, classifyMCPAuthExchangeError(
			fmt.Errorf("settings: resolve MCP auth callback target: %w", err),
		)
	}
	resolvedTarget, server, err := s.resolveMCPAuthTarget(ctx, mcpAuthTargetRequest(target))
	if err != nil {
		return mcpauth.Status{}, err
	}
	status, err := s.mcpAuth.MCPAuthCompleteCallback(ctx, resolvedTarget, server, trimmedCallbackURL)
	if err != nil {
		return mcpauth.Status{}, classifyMCPAuthExchangeError(
			fmt.Errorf("settings: complete MCP auth callback: %w", err),
		)
	}
	return status, nil
}

func classifyMCPAuthExchangeError(err error) error {
	if errors.Is(err, mcpauth.ErrInvalidExchange) ||
		errors.Is(err, mcpauth.ErrLoginSessionNotFound) ||
		errors.Is(err, mcpauth.ErrLoginSessionExpired) ||
		errors.Is(err, mcpauth.ErrLoginSessionStale) ||
		errors.Is(err, mcpauth.ErrTokenMutationStale) {
		return validationError(err)
	}
	return err
}

func mcpAuthTargetRequest(target mcpauth.Target) MCPAuthTargetRequest {
	target = target.Normalize()
	return MCPAuthTargetRequest{
		Scope: ScopeKind(target.Scope), WorkspaceID: target.WorkspaceID, Name: target.ServerName,
	}
}

// LogoutMCPAuth revokes and removes one exact scoped credential set.
func (s *service) LogoutMCPAuth(
	ctx context.Context,
	req MCPAuthTargetRequest,
) (mcpauth.Status, error) {
	if s.mcpAuth == nil {
		return mcpauth.Status{}, unavailableError(errors.New("settings: MCP auth runtime is unavailable"))
	}
	target, server, err := s.resolveMCPAuthTarget(ctx, req)
	if err != nil {
		return mcpauth.Status{}, err
	}
	status, err := s.mcpAuth.MCPAuthLogout(ctx, target, server)
	if err != nil {
		return mcpauth.Status{}, fmt.Errorf("settings: logout MCP auth for %q: %w", target.ServerName, err)
	}
	return status, nil
}

func (s *service) resolveMCPAuthTarget(
	ctx context.Context,
	req MCPAuthTargetRequest,
) (mcpauth.Target, aghconfig.MCPServer, error) {
	target, err := normalizeMCPAuthTarget(req)
	if err != nil {
		return mcpauth.Target{}, aghconfig.MCPServer{}, err
	}
	_, sources, err := s.resolveMCPTargetContext(
		ctx,
		ScopeKind(target.Scope),
		target.WorkspaceID,
	)
	if err != nil {
		return mcpauth.Target{}, aghconfig.MCPServer{}, err
	}
	targetKind, ok := preferredMCPDeleteTarget(ScopeKind(target.Scope), target.ServerName, sources)
	if !ok {
		return mcpauth.Target{}, aghconfig.MCPServer{}, notFoundError(
			fmt.Errorf("settings: MCP server %q has no definition in %s scope", target.ServerName, target.Scope),
		)
	}
	entry, ok := mcpSourceForTarget(target.ServerName, targetKind, sources)
	if !ok {
		return mcpauth.Target{}, aghconfig.MCPServer{}, notFoundError(
			fmt.Errorf("settings: MCP server %q has no definition in %s scope", target.ServerName, target.Scope),
		)
	}
	return target, entry.Server, nil
}

func normalizeMCPAuthTarget(req MCPAuthTargetRequest) (mcpauth.Target, error) {
	scope := req.Scope
	if scope == "" {
		return mcpauth.Target{}, validationError(errors.New("settings: MCP auth scope is required"))
	}
	if scope != ScopeGlobal && scope != ScopeWorkspace {
		return mcpauth.Target{}, validationError(fmt.Errorf("settings: MCP auth scope %q is unsupported", scope))
	}
	target := mcpauth.Target{
		Scope: mcpauth.Scope(scope), WorkspaceID: strings.TrimSpace(req.WorkspaceID),
		ServerName: strings.TrimSpace(req.Name),
	}
	if err := target.Validate(); err != nil {
		return mcpauth.Target{}, validationError(err)
	}
	return target, nil
}

func mcpAuthTargetForSource(entry mcpSourceEntry) (mcpauth.Target, error) {
	target := mcpauth.Target{ServerName: strings.TrimSpace(entry.Server.Name)}
	switch entry.Target {
	case WriteTargetGlobalConfig, WriteTargetGlobalMCPSidecar:
		target.Scope = mcpauth.ScopeGlobal
	case WriteTargetWorkspaceConfig, WriteTargetWorkspaceMCPSidecar:
		target.Scope = mcpauth.ScopeWorkspace
		target.WorkspaceID = strings.TrimSpace(entry.Source.WorkspaceID)
	default:
		return mcpauth.Target{}, fmt.Errorf("settings: unsupported MCP auth source %q", entry.Target)
	}
	if err := target.Validate(); err != nil {
		return mcpauth.Target{}, fmt.Errorf("settings: invalid MCP auth target: %w", err)
	}
	return target, nil
}
