package config

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/compozy/agh/internal/resources"
)

const (
	// MCPServerResourceKind is the canonical desired-state resource kind for MCP server records.
	MCPServerResourceKind     resources.ResourceKind = "mcp_server"
	mcpServerResourceMaxBytes                        = 256 << 10
)

// NewMCPServerResourceCodec builds the canonical MCP server resource codec.
func NewMCPServerResourceCodec() (resources.KindCodec[MCPServer], error) {
	return resources.NewJSONCodec(MCPServerResourceKind, mcpServerResourceMaxBytes, validateMCPServerSpec)
}

func validateMCPServerSpec(
	_ context.Context,
	scope resources.ResourceScope,
	spec MCPServer,
) (MCPServer, error) {
	normalizedScope := scope.Normalize()
	if err := normalizedScope.Validate("scope"); err != nil {
		return MCPServer{}, fmt.Errorf("config: validate mcp resource scope: %w", err)
	}

	normalized := normalizeMCPServerResourceSpec(spec)
	if err := normalized.Validate("mcp_server"); err != nil {
		return MCPServer{}, fmt.Errorf("config: validate mcp resource spec: %w", err)
	}
	return normalized, nil
}

func normalizeMCPServerResourceSpec(spec MCPServer) MCPServer {
	normalized := cloneMCPServer(spec)
	normalized.Name = strings.TrimSpace(normalized.Name)
	normalized.Transport = MCPServerTransport(strings.TrimSpace(string(normalized.Transport)))
	normalized.Command = strings.TrimSpace(normalized.Command)
	normalized.URL = strings.TrimSpace(normalized.URL)
	for idx, arg := range normalized.Args {
		normalized.Args[idx] = strings.TrimSpace(arg)
	}
	normalized.Auth = normalizeMCPAuthConfig(normalized.Auth)
	if len(normalized.Env) > 0 {
		keys := make([]string, 0, len(normalized.Env))
		for key := range normalized.Env {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		canonicalEnv := make(map[string]string, len(keys))
		for _, key := range keys {
			trimmedKey := strings.TrimSpace(key)
			if trimmedKey == "" {
				continue
			}
			canonicalEnv[trimmedKey] = strings.TrimSpace(normalized.Env[key])
		}
		if len(canonicalEnv) == 0 {
			normalized.Env = nil
		} else {
			normalized.Env = canonicalEnv
		}
	}
	if len(normalized.SecretEnv) > 0 {
		keys := make([]string, 0, len(normalized.SecretEnv))
		for key := range normalized.SecretEnv {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		canonicalSecretEnv := make(map[string]string, len(keys))
		for _, key := range keys {
			trimmedKey := strings.TrimSpace(key)
			if trimmedKey == "" {
				continue
			}
			canonicalSecretEnv[trimmedKey] = strings.TrimSpace(normalized.SecretEnv[key])
		}
		if len(canonicalSecretEnv) == 0 {
			normalized.SecretEnv = nil
		} else {
			normalized.SecretEnv = canonicalSecretEnv
		}
	}

	return normalized
}

func normalizeMCPAuthConfig(auth MCPAuthConfig) MCPAuthConfig {
	auth.Type = MCPAuthType(strings.TrimSpace(string(auth.Type)))
	auth.IssuerURL = strings.TrimSpace(auth.IssuerURL)
	auth.MetadataURL = strings.TrimSpace(auth.MetadataURL)
	auth.AuthorizationURL = strings.TrimSpace(auth.AuthorizationURL)
	auth.TokenURL = strings.TrimSpace(auth.TokenURL)
	auth.RevocationURL = strings.TrimSpace(auth.RevocationURL)
	auth.ClientID = strings.TrimSpace(auth.ClientID)
	auth.ClientSecretRef = strings.TrimSpace(auth.ClientSecretRef)
	for idx, scope := range auth.Scopes {
		auth.Scopes[idx] = strings.TrimSpace(scope)
	}
	return auth
}
