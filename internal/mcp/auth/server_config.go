package auth

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/vault"
)

// ServerConfigFromMCP converts a config MCP server into token-free auth
// service input. resolveSecret receives the configured client_secret_ref and
// returns the actual secret value when present.
func ServerConfigFromMCP(
	ctx context.Context,
	target Target,
	server compozyconfig.MCPServer,
	resolveSecret SecretRefResolver,
) (ServerConfig, error) {
	target = target.Normalize()
	if err := target.Validate(); err != nil {
		return ServerConfig{}, err
	}
	if target.ServerName != strings.TrimSpace(server.Name) {
		return ServerConfig{}, errors.New("mcp auth: target server name does not match config")
	}
	if err := server.Validate("mcp_server"); err != nil {
		return ServerConfig{}, err
	}
	if !OAuthCapable(server) {
		return ServerConfig{
			Target:    target,
			Transport: string(server.EffectiveTransport()),
			RemoteURL: strings.TrimSpace(server.URL),
		}, nil
	}

	secretRef := strings.TrimSpace(server.Auth.ClientSecretRef)
	if secretRef != "" {
		var err error
		secretRef, err = vault.NormalizeMCPClientSecretRef(secretRef, target.VaultTarget())
		if err != nil {
			return ServerConfig{}, fmt.Errorf("mcp auth: client secret ownership: %w", err)
		}
	}
	secret := ""
	if secretRef != "" && resolveSecret != nil {
		resolved, err := resolveSecret(ctx, secretRef)
		if err != nil {
			return ServerConfig{}, fmt.Errorf(
				"mcp auth: resolve client secret ref %q: %w",
				secretRef,
				err,
			)
		}
		secret = resolved
	}

	cfg := ServerConfig{
		Implicit:        server.Auth.IsZero(),
		Target:          target,
		Transport:       string(server.EffectiveTransport()),
		RemoteURL:       strings.TrimSpace(server.URL),
		CatalogEntry:    strings.TrimSpace(server.CatalogEntry),
		Type:            authTypeOAuth,
		IssuerURL:       strings.TrimSpace(server.Auth.IssuerURL),
		ClientID:        strings.TrimSpace(server.Auth.ClientID),
		ClientSecret:    secret,
		ClientSecretRef: secretRef,
		Scopes:          trimStrings(server.Auth.Scopes),
		Registration:    RegistrationStrategy(server.Auth.Registration),
	}
	if cfg.Implicit && OAuthWithPluginHeaders(server) {
		cfg.pluginHeaderOAuth = true
		resource, err := url.Parse(cfg.RemoteURL)
		if err != nil {
			return ServerConfig{}, errors.New("mcp auth: invalid plugin resource URL")
		}
		resource.RawQuery, resource.Fragment, resource.RawFragment, resource.ForceQuery = "", "", "", false
		if canonical := resource.String(); canonical != cfg.RemoteURL {
			cfg.ResourceURL = canonical
		}
	}
	return cfg, nil
}

// OAuthCapable reports whether a server definition permits OAuth sign-in.
func OAuthCapable(server compozyconfig.MCPServer) bool {
	return server.EffectiveTransport() == compozyconfig.MCPServerTransportHTTP &&
		(!server.Auth.IsZero() || (len(server.Headers) == 0 && len(server.SecretHeaders) == 0) ||
			OAuthWithPluginHeaders(server))
}

// OAuthWithPluginHeaders permits discovery alongside headers declared by an Agent Plugin.
func OAuthWithPluginHeaders(server compozyconfig.MCPServer) bool {
	return server.AgentPlugin && len(server.Headers) > 0 && len(server.SecretHeaders) == 0
}

// ServerConfigsFromMCP returns auth service configs for every auth-enabled MCP
// server in the supplied list.
func ServerConfigsFromMCP(
	ctx context.Context,
	servers map[Target]compozyconfig.MCPServer,
	resolveSecret SecretRefResolver,
) ([]ServerConfig, error) {
	configs := make([]ServerConfig, 0, len(servers))
	for target, server := range servers {
		if server.Auth.IsZero() {
			continue
		}
		cfg, err := ServerConfigFromMCP(ctx, target, server, resolveSecret)
		if err != nil {
			return nil, err
		}
		configs = append(configs, cfg)
	}
	return configs, nil
}
