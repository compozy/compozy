package config

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/compozy/compozy/internal/vault"
)

// Validate ensures the MCP server entry is usable.
func (s MCPServer) Validate(path string) error {
	transport := s.EffectiveTransport()
	if err := ValidateMCPServerName(s.Name); err != nil {
		return fmt.Errorf("%s.name: %w", path, err)
	}
	if err := transport.Validate(path + ".transport"); err != nil {
		return err
	}
	if err := s.Auth.Validate(path + ".auth"); err != nil {
		return err
	}
	switch {
	case transport == MCPServerTransportStdio && strings.TrimSpace(s.Command) == "":
		return fmt.Errorf("%s.command is required", path)
	case transport == MCPServerTransportStdio && strings.TrimSpace(s.URL) != "":
		return fmt.Errorf("%s.url requires remote transport", path)
	case transport != MCPServerTransportStdio && strings.TrimSpace(s.URL) == "":
		return fmt.Errorf("%s.url is required for %s transport", path, transport)
	case transport != MCPServerTransportStdio && strings.TrimSpace(s.Command) != "":
		return fmt.Errorf("%s.command is only valid for stdio transport", path)
	case transport != MCPServerTransportStdio && len(s.Args) > 0:
		return fmt.Errorf("%s.args is only valid for stdio transport", path)
	case transport != MCPServerTransportStdio && len(s.Env) > 0:
		return fmt.Errorf("%s.env is only valid for stdio transport", path)
	case transport == MCPServerTransportStdio && !s.Auth.IsZero():
		return fmt.Errorf("%s.auth is only valid for remote MCP servers", path)
	case transport != MCPServerTransportStdio && len(s.SecretEnv) > 0:
		return fmt.Errorf("%s.secret_env is only valid for stdio transport", path)
	default:
		return validateStdioMCPEnv(path, transport, s.Env, s.SecretEnv)
	}
}

// Validate ensures remote MCP OAuth configuration has enough metadata to run
// the authorization-code flow without placing token material in config files.
func (a MCPAuthConfig) Validate(path string) error {
	if a.IsZero() {
		return nil
	}
	registration := a.Registration
	if registration == "" {
		registration = MCPAuthRegistrationAuto
	}
	switch registration {
	case MCPAuthRegistrationAuto:
		if strings.TrimSpace(a.IssuerURL) != "" || strings.TrimSpace(a.ClientID) != "" ||
			strings.TrimSpace(a.ClientSecretRef) != "" {
			return fmt.Errorf("%s issuer_url, client_id, and client_secret_ref require registration = %q", path, MCPAuthRegistrationPreRegistered)
		}
	case MCPAuthRegistrationPreRegistered:
		if strings.TrimSpace(a.IssuerURL) == "" {
			return fmt.Errorf("%s.issuer_url is required for registration = %q", path, MCPAuthRegistrationPreRegistered)
		}
		if strings.TrimSpace(a.ClientID) == "" {
			return fmt.Errorf("%s.client_id is required for registration = %q", path, MCPAuthRegistrationPreRegistered)
		}
		if err := ValidateMCPOAuthURL(path+".issuer_url", a.IssuerURL); err != nil {
			return err
		}
		if strings.TrimSpace(a.ClientSecretRef) != "" {
			if err := vault.ValidateRefNamespace(a.ClientSecretRef, "mcp"); err != nil {
				return fmt.Errorf("%s.client_secret_ref is invalid: %w", path, err)
			}
		}
	default:
		return fmt.Errorf("%s.registration must be one of %q or %q", path, MCPAuthRegistrationAuto, MCPAuthRegistrationPreRegistered)
	}
	seenScopes := make(map[string]struct{}, len(a.Scopes))
	for idx, scope := range a.Scopes {
		trimmed := strings.TrimSpace(scope)
		if trimmed == "" {
			return fmt.Errorf("%s.scopes[%d] is required", path, idx)
		}
		if _, exists := seenScopes[trimmed]; exists {
			return fmt.Errorf("%s.scopes[%d] duplicates %q", path, idx, trimmed)
		}
		seenScopes[trimmed] = struct{}{}
	}
	return nil
}

// Validate ensures global MCP OAuth client configuration is safe to use.
func (c MCPOAuthConfig) Validate(path string) error {
	if value := strings.TrimSpace(c.ClientMetadataURL); value != "" {
		if err := ValidateMCPOAuthURL(path+".client_metadata_url", value); err != nil {
			return err
		}
	}
	if value := strings.TrimSpace(c.RedirectURI); value != "" {
		if err := ValidateMCPOAuthURL(path+".redirect_uri", value); err != nil {
			return err
		}
	}
	return nil
}

// Validate ensures global MCP configuration is safe to use.
func (c MCPConfig) Validate() error {
	return c.OAuth.Validate("mcp.oauth")
}

func validateStdioMCPEnv(
	path string,
	transport MCPServerTransport,
	env map[string]string,
	secretEnv map[string]string,
) error {
	if transport != MCPServerTransportStdio {
		return nil
	}
	for key := range env {
		if err := ValidateMCPStdioEnvName(path+".env", key, false); err != nil {
			return err
		}
	}
	for key := range secretEnv {
		if err := ValidateMCPStdioEnvName(path+".secret_env", key, true); err != nil {
			return err
		}
	}
	return vault.ValidateSecretEnvMap(path, "mcp", secretEnv)
}

// ValidateMCPServerName enforces the identity grammar shared by settings and auth targets.
func ValidateMCPServerName(name string) error {
	trimmed := strings.TrimSpace(name)
	switch {
	case trimmed == "":
		return fmt.Errorf("server name is required")
	case strings.ContainsRune(trimmed, '\x00'):
		return fmt.Errorf("server name cannot contain NUL")
	case strings.Contains(trimmed, "/"):
		return fmt.Errorf("server name cannot contain a slash")
	default:
		return nil
	}
}

// ValidateMCPStdioEnvName enforces the runtime policy for one catalog or config env declaration.
func ValidateMCPStdioEnvName(path string, name string, secret bool) error {
	trimmed := strings.TrimSpace(name)
	if !vault.EnvNamePattern.MatchString(trimmed) {
		return fmt.Errorf("%s.%s must be an environment variable name", path, trimmed)
	}
	if forbiddenStdioMCPEnvKey(trimmed) {
		return fmt.Errorf("%s.%s is forbidden for stdio MCP servers", path, trimmed)
	}
	if !secret && vault.SecretLikeEnvName(trimmed) {
		return fmt.Errorf("%s.%s must move secret-like values to secret_env", path, trimmed)
	}
	return nil
}

// ValidateMCPOAuthURL enforces HTTPS for OAuth endpoints except on loopback hosts.
func ValidateMCPOAuthURL(path string, raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("%s must be an absolute HTTP(S) URL without credentials", path)
	}
	switch parsed.Scheme {
	case "https":
		return nil
	case "http":
		if mcpLoopbackHost(parsed.Hostname()) {
			return nil
		}
		return fmt.Errorf("%s must use https unless host is loopback", path)
	default:
		return fmt.Errorf("%s must use https", path)
	}
}

func mcpLoopbackHost(host string) bool {
	normalized := strings.Trim(strings.TrimSpace(host), "[]")
	if strings.EqualFold(normalized, "localhost") {
		return true
	}
	ip := net.ParseIP(normalized)
	return ip != nil && ip.IsLoopback()
}

func forbiddenStdioMCPEnvKey(key string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(key))
	switch normalized {
	case providerNodeOptionsValue, "PYTHONPATH", "PYTHONHOME", "LD_PRELOAD":
		return true
	default:
		return strings.HasPrefix(normalized, "DYLD_")
	}
}
