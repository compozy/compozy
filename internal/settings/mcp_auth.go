package settings

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
)

const mcpAuthDefaultProfileName = "default"

// MCPAuthTargetRequest selects one exact user, profile, or workspace MCP definition.
type MCPAuthTargetRequest struct {
	Scope       ScopeKind
	WorkspaceID string
	ProfileName string
	Name        string
	Owner       string
}

// MCPAuthBeginRequest starts one daemon-owned OAuth session.
type MCPAuthBeginRequest struct {
	MCPAuthTargetRequest
	CallbackURL            string
	ApprovedScopes         []string
	ApproveScopeEscalation bool
}

// MCPAuthExchangeRequest completes one daemon-owned OAuth session.
type MCPAuthExchangeRequest struct {
	MCPAuthTargetRequest
	RedirectURL string
}

// GetMCPAuthStatus returns redacted auth state for one exact configured MCP server.
func (s *service) GetMCPAuthStatus(
	ctx context.Context,
	req MCPAuthTargetRequest,
) (mcpauth.Status, error) {
	if s.mcpAuth == nil {
		return mcpauth.Status{}, unavailableError(errors.New("settings: MCP auth runtime is unavailable"))
	}
	resolved, err := s.resolveMCPAuthDefinition(ctx, req)
	if err != nil {
		return mcpauth.Status{}, err
	}
	status, err := s.mcpAuth.MCPAuthStatus(ctx, resolved.target, resolved.server)
	if err != nil {
		return mcpauth.Status{}, fmt.Errorf(
			"settings: load MCP auth status for %q: %w",
			resolved.target.ServerName,
			err,
		)
	}
	status.Inherited = resolved.inherited
	return status, nil
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
	if len(req.ApprovedScopes) > 0 && !req.ApproveScopeEscalation {
		return mcpauth.BeginResult{}, validationError(
			errors.New("settings: MCP auth approved_scopes require approve_scope_escalation = true"),
		)
	}
	callbackURL := strings.TrimSpace(req.CallbackURL)
	var result mcpauth.BeginResult
	if req.ApproveScopeEscalation {
		result, err = s.mcpAuth.MCPAuthBeginStepUp(ctx, target, server, callbackURL, req.ApprovedScopes, true)
	} else {
		result, err = s.mcpAuth.MCPAuthBegin(ctx, target, server, callbackURL)
	}
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
		RedirectURL: req.RedirectURL,
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
	request := MCPAuthTargetRequest{
		Scope: ScopeKind(target.Scope), WorkspaceID: target.WorkspaceID, Name: target.ServerName,
		Owner: target.Owner,
	}
	switch target.Scope {
	case mcpauth.ScopeProfile:
		request.ProfileName = target.WorkspaceID
		request.WorkspaceID = ""
		request.Scope = ScopeProfile
	case mcpauth.ScopeWorkspaceProfile:
		workspaceID, profileName, _ := strings.Cut(target.WorkspaceID, "@pf:")
		request.Scope = ScopeProfile
		request.WorkspaceID = strings.TrimSpace(workspaceID)
		request.ProfileName = strings.TrimSpace(profileName)
	}
	return request
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

// mcpAuthDefinition is the MCP definition whose declaring scope owns the OAuth credential.
type mcpAuthDefinition struct {
	target    mcpauth.Target
	server    compozyconfig.MCPServer
	inherited bool
}

// resolveMCPAuthTarget resolves the exact credential owner for an auth mutation. A request for a
// scope that only inherits the server from a broader layer fails instead of mutating the broader
// layer's credential, because the runtime always uses the declaring scope's credential.
func (s *service) resolveMCPAuthTarget(
	ctx context.Context,
	req MCPAuthTargetRequest,
) (mcpauth.Target, compozyconfig.MCPServer, error) {
	resolved, err := s.resolveMCPAuthDefinition(ctx, req)
	if err != nil {
		return mcpauth.Target{}, compozyconfig.MCPServer{}, err
	}
	if resolved.inherited {
		return mcpauth.Target{}, compozyconfig.MCPServer{}, validationError(fmt.Errorf(
			"settings: MCP server %q is declared at %s scope and only inherited by the requested %s scope; "+
				"OAuth credentials belong to the declaring scope, so authorize it at %s",
			resolved.target.ServerName, publicMCPAuthScope(resolved.target), req.Scope,
			describeMCPAuthTarget(resolved.target),
		))
	}
	return resolved.target, resolved.server, nil
}

// resolveMCPAuthDefinition applies the documented OAuth credential resolution order: the
// credential owner is the scope that declares the effective definition (workspace profile,
// workspace, profile, then user). When the requested scope has no own definition, the effective
// inherited definition is returned and marked inherited.
func (s *service) resolveMCPAuthDefinition(
	ctx context.Context,
	req MCPAuthTargetRequest,
) (mcpAuthDefinition, error) {
	target, err := normalizeMCPAuthTarget(req)
	if err != nil {
		return mcpAuthDefinition{}, err
	}
	if target.Owner != "manual" {
		resolved, server, err := s.resolveExtensionMCPAuthTarget(ctx, req)
		if err != nil {
			return mcpAuthDefinition{}, err
		}
		return mcpAuthDefinition{
			target: resolved, server: server, inherited: !mcpAuthTargetCovers(target, resolved),
		}, nil
	}
	_, sources, err := s.resolveMCPTargetContext(
		ctx,
		req.Scope,
		req.WorkspaceID,
		req.ProfileName,
	)
	if err != nil {
		return mcpAuthDefinition{}, err
	}
	targetKind, ok := preferredMCPAuthTarget(req, target.ServerName, sources)
	if !ok {
		entries := sources[target.ServerName]
		if len(entries) > 0 {
			effective := entries[len(entries)-1]
			inherited, err := mcpAuthTargetForSource(effective)
			if err != nil {
				return mcpAuthDefinition{}, err
			}
			resolvedTarget := inherited.Normalize()
			return mcpAuthDefinition{
				target:    resolvedTarget,
				server:    effective.Server,
				inherited: !mcpAuthTargetCovers(target, resolvedTarget),
			}, nil
		}
		return mcpAuthDefinition{}, notFoundError(
			fmt.Errorf("settings: MCP server %q has no definition in %s scope", target.ServerName, target.Scope),
		)
	}
	entry, ok := mcpSourceForTarget(target.ServerName, targetKind, sources)
	if !ok {
		return mcpAuthDefinition{}, notFoundError(
			fmt.Errorf("settings: MCP server %q has no definition in %s scope", target.ServerName, target.Scope),
		)
	}
	return mcpAuthDefinition{target: target, server: entry.Server}, nil
}

// mcpAuthTargetCovers reports whether the resolved credential owner is the requested scope itself.
// The default profile layers alias the user and workspace credentials they project.
func mcpAuthTargetCovers(requested mcpauth.Target, resolved mcpauth.Target) bool {
	if requested.Scope == resolved.Scope && requested.WorkspaceID == resolved.WorkspaceID {
		return true
	}
	switch requested.Scope {
	case mcpauth.ScopeProfile:
		return requested.WorkspaceID == mcpAuthDefaultProfileName && resolved.Scope == mcpauth.ScopeUser
	case mcpauth.ScopeWorkspaceProfile:
		workspaceID, profileName, _ := strings.Cut(requested.WorkspaceID, "@pf:")
		return profileName == mcpAuthDefaultProfileName && resolved.Scope == mcpauth.ScopeWorkspace &&
			resolved.WorkspaceID == workspaceID
	default:
		return false
	}
}

func publicMCPAuthScope(target mcpauth.Target) string {
	if target.Scope == mcpauth.ScopeWorkspaceProfile {
		return string(ScopeProfile)
	}
	return string(target.Scope)
}

func describeMCPAuthTarget(target mcpauth.Target) string {
	switch target.Scope {
	case mcpauth.ScopeWorkspace:
		return fmt.Sprintf("workspace scope (workspace %q)", target.WorkspaceID)
	case mcpauth.ScopeProfile:
		return fmt.Sprintf("profile scope (profile %q)", target.WorkspaceID)
	case mcpauth.ScopeWorkspaceProfile:
		workspaceID, profileName, _ := strings.Cut(target.WorkspaceID, "@pf:")
		return fmt.Sprintf("profile scope (profile %q, workspace %q)", profileName, workspaceID)
	default:
		return "user scope"
	}
}

func preferredMCPAuthTarget(
	req MCPAuthTargetRequest,
	name string,
	sources map[string][]mcpSourceEntry,
) (WriteTargetKind, bool) {
	if req.Scope != ScopeProfile || strings.TrimSpace(req.WorkspaceID) == "" {
		return preferredMCPDeleteTarget(req.Scope, name, sources)
	}
	for _, entry := range slices.Backward(sources[strings.TrimSpace(name)]) {
		switch entry.Target {
		case WriteTargetWorkspaceProfileMCPSidecar, WriteTargetWorkspaceProfileConfig:
			return entry.Target, true
		}
	}
	return "", false
}

func normalizeMCPAuthTarget(req MCPAuthTargetRequest) (mcpauth.Target, error) {
	scope := req.Scope
	if scope == "" {
		return mcpauth.Target{}, validationError(errors.New("settings: MCP auth scope is required"))
	}
	if scope != ScopeUser && scope != ScopeProfile && scope != ScopeWorkspace {
		return mcpauth.Target{}, validationError(fmt.Errorf("settings: MCP auth scope %q is unsupported", scope))
	}
	scopeID := strings.TrimSpace(req.WorkspaceID)
	if scope == ScopeProfile {
		profileName := strings.TrimSpace(req.ProfileName)
		if scopeID == "" {
			scopeID = profileName
		} else {
			scopeID += "@pf:" + profileName
		}
	}
	authScope := mcpauth.Scope(scope)
	if scope == ScopeProfile && strings.TrimSpace(req.WorkspaceID) != "" {
		authScope = mcpauth.ScopeWorkspaceProfile
	}
	target := mcpauth.Target{
		Scope: authScope, WorkspaceID: scopeID,
		ServerName: strings.TrimSpace(req.Name), Owner: req.Owner,
	}.Normalize()
	if err := target.Validate(); err != nil {
		return mcpauth.Target{}, validationError(err)
	}
	return target, nil
}

func mcpAuthTargetForSource(entry mcpSourceEntry) (mcpauth.Target, error) {
	if entry.AuthTarget != nil {
		target := entry.AuthTarget.Normalize()
		if err := target.Validate(); err != nil {
			return mcpauth.Target{}, err
		}
		return target, nil
	}
	target := mcpauth.Target{ServerName: strings.TrimSpace(entry.Server.Name)}
	switch entry.Target {
	case WriteTargetGlobalConfig, WriteTargetGlobalMCPSidecar:
		target.Scope = mcpauth.ScopeUser
	case WriteTargetProfileConfig, WriteTargetProfileMCPSidecar:
		target.Scope = mcpauth.ScopeProfile
		target.WorkspaceID = strings.TrimSpace(entry.Source.ProfileName)
	case WriteTargetWorkspaceConfig, WriteTargetWorkspaceMCPSidecar:
		target.Scope = mcpauth.ScopeWorkspace
		target.WorkspaceID = strings.TrimSpace(entry.Source.WorkspaceID)
	case WriteTargetWorkspaceProfileConfig, WriteTargetWorkspaceProfileMCPSidecar:
		target.Scope = mcpauth.ScopeWorkspaceProfile
		target.WorkspaceID = strings.TrimSpace(entry.Source.WorkspaceID) + "@pf:" +
			strings.TrimSpace(entry.Source.ProfileName)
	default:
		return mcpauth.Target{}, fmt.Errorf("settings: unsupported MCP auth source %q", entry.Target)
	}
	if err := target.Validate(); err != nil {
		return mcpauth.Target{}, fmt.Errorf("settings: invalid MCP auth target: %w", err)
	}
	return target, nil
}
