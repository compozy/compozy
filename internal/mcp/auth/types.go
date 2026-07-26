package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
)

// ErrTokenNotFound reports missing persisted MCP auth state for one server.
var ErrTokenNotFound = errors.New("mcp auth: token not found")

// Scope identifies the owner of one MCP OAuth credential set.
type Scope string

const (
	// ScopeGlobal identifies a daemon-global MCP server.
	ScopeGlobal Scope = "global"
	// ScopeWorkspace identifies a workspace-owned MCP server.
	ScopeWorkspace Scope = "workspace"
)

// Target is the complete identity of one MCP OAuth credential set.
type Target struct {
	Scope       Scope  `json:"scope"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	ServerName  string `json:"server_name"`
}

// Normalize trims target fields without changing their meaning.
func (t Target) Normalize() Target {
	t.Scope = Scope(strings.TrimSpace(string(t.Scope)))
	t.WorkspaceID = strings.TrimSpace(t.WorkspaceID)
	t.ServerName = strings.TrimSpace(t.ServerName)
	return t
}

// Validate ensures the target has one unambiguous owner.
func (t Target) Validate() error {
	t = t.Normalize()
	switch {
	case strings.ContainsRune(t.WorkspaceID, '\x00'):
		return errors.New("mcp auth: workspace_id cannot contain NUL")
	case t.Scope == ScopeGlobal && t.WorkspaceID != "":
		return errors.New("mcp auth: global target cannot include workspace_id")
	case t.Scope == ScopeWorkspace && t.WorkspaceID == "":
		return errors.New("mcp auth: workspace target requires workspace_id")
	case t.Scope != ScopeGlobal && t.Scope != ScopeWorkspace:
		return fmt.Errorf("mcp auth: unsupported target scope %q", t.Scope)
	default:
		if err := aghconfig.ValidateMCPServerName(t.ServerName); err != nil {
			return fmt.Errorf("mcp auth: %w", err)
		}
		return nil
	}
}

// Key returns a collision-safe in-memory identity after validation.
func (t Target) Key() (string, error) {
	t = t.Normalize()
	if err := t.Validate(); err != nil {
		return "", err
	}
	return string(t.Scope) + "\x00" + t.WorkspaceID + "\x00" + t.ServerName, nil
}

// StatusValue is the redacted operator-facing authentication state.
type StatusValue string

const (
	StatusUnconfigured  StatusValue = "unconfigured"
	StatusNeedsLogin    StatusValue = "needs_login"
	StatusAuthenticated StatusValue = "authenticated"
	StatusExpired       StatusValue = "expired"
	StatusInvalid       StatusValue = "invalid"
)

// ServerConfig is the token-free auth configuration used by the OAuth service.
type ServerConfig struct {
	Target           Target
	Transport        string
	RemoteURL        string
	Type             string
	IssuerURL        string
	MetadataURL      string
	AuthorizationURL string
	TokenURL         string
	RevocationURL    string
	ClientID         string
	ClientSecret     string
	ClientSecretRef  string
	Scopes           []string
}

// Metadata is the OAuth authorization server metadata needed for PKCE flows.
type Metadata struct {
	Issuer                        string   `json:"issuer,omitempty"`
	AuthorizationEndpoint         string   `json:"authorization_endpoint"`
	TokenEndpoint                 string   `json:"token_endpoint"`
	RevocationEndpoint            string   `json:"revocation_endpoint,omitempty"`
	CodeChallengeMethodsSupported []string `json:"code_challenge_methods_supported,omitempty"`
	ScopesSupported               []string `json:"scopes_supported,omitempty"`
}

// TokenRecord is the durable token-store row. It must never be rendered
// directly in public API or CLI output.
type TokenRecord struct {
	Target                Target
	DefinitionFingerprint string
	Issuer                string
	ClientID              string
	Scopes                []string
	AccessToken           string
	RefreshToken          string
	TokenType             string
	ExpiresAt             time.Time
	ObtainedAt            time.Time
	UpdatedAt             time.Time
}

// Status is the token-redacted state used by CLI and settings APIs.
type Status struct {
	ServerName       string      `json:"server_name"`
	Scope            Scope       `json:"scope"`
	WorkspaceID      string      `json:"workspace_id,omitempty"`
	Status           StatusValue `json:"status"`
	RemoteURL        string      `json:"remote_url,omitempty"`
	AuthType         string      `json:"auth_type,omitempty"`
	ClientID         string      `json:"client_id,omitempty"`
	Issuer           string      `json:"issuer,omitempty"`
	Scopes           []string    `json:"scopes,omitempty"`
	ExpiresAt        *time.Time  `json:"expires_at,omitempty"`
	UpdatedAt        *time.Time  `json:"updated_at,omitempty"`
	Refreshable      bool        `json:"refreshable"`
	TokenPresent     bool        `json:"token_present"`
	RevocationURL    string      `json:"revocation_url,omitempty"`
	Diagnostic       string      `json:"diagnostic,omitempty"`
	AuthorizationURL string      `json:"authorization_url,omitempty"`
}

// TokenStore persists OAuth token material behind a narrow boundary.
type TokenStore interface {
	SaveMCPAuthToken(ctx context.Context, token TokenRecord) error
	GetMCPAuthToken(ctx context.Context, target Target) (TokenRecord, error)
	ListMCPAuthTokens(ctx context.Context) ([]TokenRecord, error)
	DeleteMCPAuthToken(ctx context.Context, target Target) error
}

// SecretRefResolver resolves configured env: or vault: refs to plaintext for OAuth token requests.
type SecretRefResolver func(ctx context.Context, ref string) (string, error)

// ServerConfigFromMCP converts a config MCP server into token-free auth
// service input. resolveSecret receives the configured client_secret_ref and
// returns the actual secret value when present.
func ServerConfigFromMCP(
	ctx context.Context,
	target Target,
	server aghconfig.MCPServer,
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
	if server.Auth.IsZero() {
		return ServerConfig{
			Target:    target,
			Transport: string(server.EffectiveTransport()),
			RemoteURL: strings.TrimSpace(server.URL),
		}, nil
	}

	secretRef := strings.TrimSpace(server.Auth.ClientSecretRef)
	secret := ""
	if secretRef != "" && resolveSecret != nil {
		resolved, err := resolveSecret(ctx, secretRef)
		if err != nil {
			return ServerConfig{}, fmt.Errorf("mcp auth: resolve client secret ref %q: %w", secretRef, err)
		}
		secret = resolved
	}

	return ServerConfig{
		Target:           target,
		Transport:        string(server.EffectiveTransport()),
		RemoteURL:        strings.TrimSpace(server.URL),
		Type:             strings.TrimSpace(string(server.Auth.Type)),
		IssuerURL:        strings.TrimSpace(server.Auth.IssuerURL),
		MetadataURL:      strings.TrimSpace(server.Auth.MetadataURL),
		AuthorizationURL: strings.TrimSpace(server.Auth.AuthorizationURL),
		TokenURL:         strings.TrimSpace(server.Auth.TokenURL),
		RevocationURL:    strings.TrimSpace(server.Auth.RevocationURL),
		ClientID:         strings.TrimSpace(server.Auth.ClientID),
		ClientSecret:     secret,
		ClientSecretRef:  secretRef,
		Scopes:           trimStrings(server.Auth.Scopes),
	}, nil
}

// ServerConfigsFromMCP returns auth service configs for every auth-enabled MCP
// server in the supplied list.
func ServerConfigsFromMCP(
	ctx context.Context,
	servers map[Target]aghconfig.MCPServer,
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

// Validate checks whether a server config is sufficient for auth actions.
func (c ServerConfig) Validate() error {
	if err := c.Target.Validate(); err != nil {
		return err
	}
	switch {
	case strings.TrimSpace(c.Type) == "":
		return errors.New("mcp auth: auth type is required")
	case strings.TrimSpace(c.Type) != string(aghconfig.MCPAuthTypeOAuth2PKCE):
		return errors.New("mcp auth: auth type must be oauth2_pkce")
	case strings.TrimSpace(c.ClientID) == "":
		return errors.New("mcp auth: client id is required")
	case strings.TrimSpace(c.MetadataURL) == "" &&
		strings.TrimSpace(c.IssuerURL) == "" &&
		(strings.TrimSpace(c.AuthorizationURL) == "" || strings.TrimSpace(c.TokenURL) == ""):
		return errors.New("mcp auth: OAuth metadata or authorization/token endpoints are required")
	default:
		return nil
	}
}

func trimStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
