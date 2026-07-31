package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	"github.com/compozy/compozy/internal/mcp/securehttp"
)

func newSettingsMCPAuthManagerWithConfig(
	store mcpauth.TokenStore,
	notifier mcpauth.LifecycleNotifier,
	registrations mcpauth.RegistrationStore,
	secretResolver mcpauth.SecretRefResolver,
	oauthConfig compozyconfig.MCPOAuthConfig,
	generation ...*mcpauth.MutationGeneration,
) (*mcpauth.Manager, error) {
	if store == nil {
		return nil, nil
	}
	serviceOptions := []mcpauth.ServiceOption{
		mcpauth.WithClientMetadataURL(oauthConfig.ClientMetadataURL),
		mcpauth.WithDefaultRedirectURL(oauthConfig.RedirectURI),
		mcpauth.WithRegistrationStore(registrations),
		mcpauth.WithSecretRefResolver(secretResolver),
		mcpauth.WithSecureHTTPClient(securehttp.NewClient()),
	}
	if len(generation) > 0 && generation[0] != nil {
		serviceOptions = append(serviceOptions, mcpauth.WithMutationGeneration(generation[0]))
	}
	service, err := mcpauth.NewService(store, serviceOptions...)
	if err != nil {
		return nil, fmt.Errorf("daemon: create MCP auth service: %w", err)
	}
	manager, err := mcpauth.NewManager(service, mcpauth.WithLifecycleNotifier(notifier))
	if err != nil {
		return nil, fmt.Errorf("daemon: create MCP auth manager: %w", err)
	}
	return manager, nil
}

func (s *settingsRuntimeSurface) MCPAuthStatus(
	ctx context.Context,
	target mcpauth.Target,
	server compozyconfig.MCPServer,
) (mcpauth.Status, error) {
	cfg, err := s.mcpAuthServerConfig(ctx, target, server)
	if err != nil {
		return mcpauth.Status{}, err
	}
	if s.mcpAuthManager == nil {
		return unavailableMCPAuthStatus(cfg), nil
	}
	status, err := s.mcpAuthManager.Status(ctx, cfg)
	if err != nil {
		return mcpauth.Status{}, fmt.Errorf("daemon: load MCP auth status: %w", err)
	}
	return status, nil
}

func (s *settingsRuntimeSurface) MCPAuthBegin(
	ctx context.Context,
	target mcpauth.Target,
	server compozyconfig.MCPServer,
	callbackURL string,
) (mcpauth.BeginResult, error) {
	if s.mcpAuthManager == nil {
		return mcpauth.BeginResult{}, errors.New("daemon: MCP auth manager is unavailable")
	}
	cfg, err := s.mcpAuthServerConfig(ctx, target, server)
	if err != nil {
		return mcpauth.BeginResult{}, err
	}
	return s.mcpAuthManager.Begin(ctx, cfg, callbackURL)
}

func (s *settingsRuntimeSurface) MCPAuthBeginStepUp(
	ctx context.Context,
	target mcpauth.Target,
	server compozyconfig.MCPServer,
	callbackURL string,
	approvedScopes []string,
	approved bool,
) (mcpauth.BeginResult, error) {
	if s.mcpAuthManager == nil {
		return mcpauth.BeginResult{}, errors.New("daemon: MCP auth manager is unavailable")
	}
	cfg, err := s.mcpAuthServerConfig(ctx, target, server)
	if err != nil {
		return mcpauth.BeginResult{}, err
	}
	return s.mcpAuthManager.BeginStepUp(ctx, cfg, callbackURL, approvedScopes, approved)
}

func (s *settingsRuntimeSurface) MCPAuthExchange(
	ctx context.Context,
	target mcpauth.Target,
	server compozyconfig.MCPServer,
	input mcpauth.ExchangeInput,
) (mcpauth.Status, error) {
	if s.mcpAuthManager == nil {
		return mcpauth.Status{}, errors.New("daemon: MCP auth manager is unavailable")
	}
	cfg, err := s.mcpAuthServerConfig(ctx, target, server)
	if err != nil {
		return mcpauth.Status{}, err
	}
	return s.mcpAuthManager.Exchange(ctx, cfg, input)
}

func (s *settingsRuntimeSurface) MCPAuthCallbackTarget(callbackURL string) (mcpauth.Target, error) {
	if s.mcpAuthManager == nil {
		return mcpauth.Target{}, errors.New("daemon: MCP auth manager is unavailable")
	}
	return s.mcpAuthManager.CallbackTarget(callbackURL)
}

func (s *settingsRuntimeSurface) MCPAuthCompleteCallback(
	ctx context.Context,
	target mcpauth.Target,
	server compozyconfig.MCPServer,
	callbackURL string,
) (mcpauth.Status, error) {
	if s.mcpAuthManager == nil {
		return mcpauth.Status{}, errors.New("daemon: MCP auth manager is unavailable")
	}
	cfg, err := s.mcpAuthServerConfig(ctx, target, server)
	if err != nil {
		return mcpauth.Status{}, err
	}
	return s.mcpAuthManager.ExchangeCallback(ctx, cfg, callbackURL)
}

func (s *settingsRuntimeSurface) MCPAuthInvalidate(target mcpauth.Target) error {
	if s.mcpAuthManager != nil {
		return s.mcpAuthManager.Invalidate(target)
	}
	return nil
}

func (s *settingsRuntimeSurface) MCPAuthDeleteState(ctx context.Context, target mcpauth.Target) error {
	if s.mcpAuthManager == nil {
		return errors.New("daemon: MCP auth manager is unavailable")
	}
	if err := s.mcpAuthManager.DeleteState(ctx, target); err != nil {
		return fmt.Errorf("daemon: delete MCP auth state: %w", err)
	}
	return nil
}

func (s *settingsRuntimeSurface) MCPAuthLogout(
	ctx context.Context,
	target mcpauth.Target,
	server compozyconfig.MCPServer,
) (mcpauth.Status, error) {
	if s.mcpAuthManager == nil {
		return mcpauth.Status{}, errors.New("daemon: MCP auth manager is unavailable")
	}
	cfg, err := s.mcpAuthServerConfig(ctx, target, server)
	if err != nil {
		return mcpauth.Status{}, err
	}
	return s.mcpAuthManager.Logout(ctx, cfg)
}

func (s *settingsRuntimeSurface) mcpAuthServerConfig(
	ctx context.Context,
	target mcpauth.Target,
	server compozyconfig.MCPServer,
) (mcpauth.ServerConfig, error) {
	cfg, err := mcpauth.ServerConfigFromMCP(ctx, target, server, s.secretResolver)
	if err != nil {
		return mcpauth.ServerConfig{}, fmt.Errorf("daemon: resolve MCP auth config: %w", err)
	}
	return cfg, nil
}

func unavailableMCPAuthStatus(cfg mcpauth.ServerConfig) mcpauth.Status {
	return mcpauth.Status{
		ServerName:  cfg.Target.ServerName,
		Scope:       cfg.Target.Scope,
		WorkspaceID: cfg.Target.WorkspaceID,
		Status:      mcpauth.StatusNeedsLogin,
		RemoteURL:   strings.TrimSpace(cfg.RemoteURL),
		AuthType:    strings.TrimSpace(cfg.Type),
		ClientID:    strings.TrimSpace(cfg.ClientID),
		Scopes:      append([]string(nil), cfg.Scopes...),
		Diagnostic:  "token store unavailable",
	}
}
