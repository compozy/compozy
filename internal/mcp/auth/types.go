package auth

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

// ErrTokenNotFound reports missing persisted MCP auth state for one server.
var ErrTokenNotFound = errors.New("mcp auth: token not found")

// ErrTokenBindingInvalid reports token material that is not bound to the active server definition.
var ErrTokenBindingInvalid = errors.New("mcp auth: token binding is invalid")

// ErrTokenExpired reports a bearer token that cannot be used for a new request.
var ErrTokenExpired = errors.New("mcp auth: token is expired")

// ErrRegistrationNotFound reports missing durable OAuth client registration state.
var ErrRegistrationNotFound = errors.New("mcp auth: client registration not found")

// Scope identifies the owner of one MCP OAuth credential set.
type Scope string

const (
	// ScopeUser identifies a user-owned MCP server.
	ScopeUser Scope = "user"
	// ScopeWorkspace identifies a workspace-owned MCP server.
	ScopeWorkspace Scope = "workspace"
	// ScopeProfile identifies a profile-owned MCP server.
	ScopeProfile Scope = "profile"
	// ScopeWorkspaceProfile identifies a workspace-and-profile-owned MCP server.
	ScopeWorkspaceProfile Scope = "workspace_profile"
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
	case t.Scope == ScopeUser && t.WorkspaceID != "":
		return errors.New("mcp auth: user target cannot include scope identifier")
	case t.Scope != ScopeUser && t.WorkspaceID == "":
		return errors.New("mcp auth: non-user target requires a scope identifier")
	case t.Scope != ScopeUser && t.Scope != ScopeWorkspace &&
		t.Scope != ScopeProfile && t.Scope != ScopeWorkspaceProfile:
		return fmt.Errorf("mcp auth: unsupported target scope %q", t.Scope)
	default:
		if t.Scope == ScopeWorkspaceProfile {
			workspaceID, profileName, ok := strings.Cut(t.WorkspaceID, "@pf:")
			if !ok || strings.TrimSpace(workspaceID) == "" {
				return errors.New("mcp auth: workspace_profile target must match <workspace_id>@pf:<profile_name>")
			}
			if err := compozyconfig.ValidateResourceProfileName(profileName); err != nil {
				return fmt.Errorf("mcp auth: invalid workspace_profile target: %w", err)
			}
		}
		if err := compozyconfig.ValidateMCPServerName(t.ServerName); err != nil {
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

// RegistrationStrategy identifies how an OAuth client is registered.
type RegistrationStrategy string

const (
	// RegistrationAuto attempts CIMD before one DCR fallback.
	RegistrationAuto RegistrationStrategy = "auto"
	// RegistrationPreRegistered uses operator-provided client credentials.
	RegistrationPreRegistered RegistrationStrategy = "pre_registered"
)

const authTypeOAuth = "oauth"

// ServerConfig is the token-free auth configuration used by the OAuth service.
type ServerConfig struct {
	Target                  Target
	Transport               string
	RemoteURL               string
	CatalogEntry            string
	Type                    string
	IssuerURL               string
	ClientID                string
	ClientSecret            string
	ClientSecretRef         string
	Scopes                  []string
	Registration            RegistrationStrategy
	ResourceURL             string
	PRMURL                  string
	TokenEndpointAuthMethod string
}

// Metadata is the OAuth authorization server metadata needed for PKCE flows.
type Metadata struct {
	Issuer                        string   `json:"issuer,omitempty"`
	AuthorizationEndpoint         string   `json:"authorization_endpoint"`
	TokenEndpoint                 string   `json:"token_endpoint"`
	RevocationEndpoint            string   `json:"revocation_endpoint,omitempty"`
	CodeChallengeMethodsSupported []string `json:"code_challenge_methods_supported,omitempty"`
	ScopesSupported               []string `json:"scopes_supported,omitempty"`
	IssuerParameterSupported      bool     `json:"authorization_response_iss_parameter_supported,omitempty"`
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

// ClientRegistration is durable, target-scoped OAuth registration state. Secret
// material is addressed only by Vault references.
type ClientRegistration struct {
	Target                     Target
	DefinitionFingerprint      string
	ResourceURL                string
	Issuer                     string
	ClientID                   string
	RegistrationClientURI      string
	ClientSecretRef            string
	RegistrationAccessTokenRef string
	ClientIDIssuedAt           time.Time
	ClientSecretExpiresAt      time.Time
	TokenEndpointAuthMethod    string
	RedirectURL                string
	Scopes                     []string
	UpdatedAt                  time.Time
}

// Status is the token-redacted state used by CLI and settings APIs.
type Status struct {
	ServerName   string      `json:"server_name"`
	Scope        Scope       `json:"scope"`
	WorkspaceID  string      `json:"workspace_id,omitempty"`
	Status       StatusValue `json:"status"`
	RemoteURL    string      `json:"remote_url,omitempty"`
	AuthType     string      `json:"auth_type,omitempty"`
	ClientID     string      `json:"client_id,omitempty"`
	Issuer       string      `json:"issuer,omitempty"`
	Scopes       []string    `json:"scopes,omitempty"`
	ExpiresAt    *time.Time  `json:"expires_at,omitempty"`
	UpdatedAt    *time.Time  `json:"updated_at,omitempty"`
	Refreshable  bool        `json:"refreshable"`
	TokenPresent bool        `json:"token_present"`
	Diagnostic   string      `json:"diagnostic,omitempty"`
}

// TokenStore persists OAuth token material behind a narrow boundary.
type TokenStore interface {
	SaveMCPAuthToken(ctx context.Context, token TokenRecord) error
	GetMCPAuthToken(ctx context.Context, target Target) (TokenRecord, error)
	ListMCPAuthTokens(ctx context.Context) ([]TokenRecord, error)
	DeleteMCPAuthToken(ctx context.Context, target Target) error
	DeleteMCPAuthorizationState(ctx context.Context, target Target) error
}

// RegistrationStore persists target-scoped OAuth client registrations.
type RegistrationStore interface {
	GetMCPAuthRegistration(context.Context, Target) (ClientRegistration, error)
	SaveMCPAuthRegistration(
		context.Context,
		ClientRegistration,
		RegistrationSecrets,
	) (ClientRegistration, error)
	DeleteMCPAuthRegistration(context.Context, Target) error
}

// SecretRefResolver resolves configured env: or vault: refs to plaintext for OAuth token requests.
type SecretRefResolver func(ctx context.Context, ref string) (string, error)

// RegistrationSecrets carries DCR material only across the atomic persistence boundary.
type RegistrationSecrets struct {
	ClientSecret            string
	RegistrationAccessToken string
}

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
			return ServerConfig{}, fmt.Errorf(
				"mcp auth: resolve client secret ref %q: %w",
				secretRef,
				err,
			)
		}
		secret = resolved
	}

	return ServerConfig{
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
	}, nil
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

// Validate checks whether a server config is sufficient for auth actions.
func (c ServerConfig) Validate() error {
	if err := c.Target.Validate(); err != nil {
		return err
	}
	switch {
	case strings.TrimSpace(c.Type) == "":
		return errors.New("mcp auth: auth type is required")
	case strings.TrimSpace(c.Type) != authTypeOAuth:
		return fmt.Errorf("mcp auth: unsupported auth type %q", c.Type)
	case c.registrationStrategy() == RegistrationPreRegistered && strings.TrimSpace(c.ClientID) == "":
		return errors.New("mcp auth: pre-registered client id is required")
	case c.registrationStrategy() != RegistrationAuto && c.registrationStrategy() != RegistrationPreRegistered:
		return errors.New("mcp auth: registration must be auto or pre_registered")
	default:
		return nil
	}
}

func (c ServerConfig) registrationStrategy() RegistrationStrategy {
	strategy := RegistrationStrategy(strings.TrimSpace(string(c.Registration)))
	if strategy == "" {
		return RegistrationAuto
	}
	return strategy
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

func validateAbsoluteHTTPURL(label string, raw string) error {
	if err := compozyconfig.ValidateMCPOAuthURL(label, raw); err != nil {
		return fmt.Errorf("mcp auth: %w", err)
	}
	return nil
}

func normalizeRegistrationClientURI(endpoint string, raw string) (string, error) {
	clientURI := strings.TrimSpace(raw)
	if clientURI == "" {
		return "", nil
	}
	base, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || base.User != nil {
		return "", errors.New("mcp auth: invalid registration endpoint")
	}
	if err := validateAbsoluteHTTPURL("registration endpoint", base.String()); err != nil {
		return "", err
	}
	reference, err := url.Parse(clientURI)
	if err != nil || reference.User != nil {
		return "", errors.New("mcp auth: invalid registration client URI")
	}
	resolved := base.ResolveReference(reference)
	if err := validateAbsoluteHTTPURL("registration client URI", resolved.String()); err != nil {
		return "", err
	}
	if !sameHTTPOrigin(base, resolved) {
		return "", errors.New(
			"mcp auth: registration client URI must use the registration endpoint origin",
		)
	}
	return resolved.String(), nil
}

func normalizeRegistrationManagementCredentials(
	endpoint string,
	accessToken string,
	clientURI string,
) (string, string, error) {
	normalizedURI, err := normalizeRegistrationClientURI(endpoint, clientURI)
	if err != nil {
		return "", "", err
	}
	normalizedToken := strings.TrimSpace(accessToken)
	if normalizedToken == "" || normalizedURI == "" {
		return "", "", nil
	}
	return normalizedToken, normalizedURI, nil
}

func sameHTTPOrigin(left *url.URL, right *url.URL) bool {
	return strings.EqualFold(left.Scheme, right.Scheme) &&
		strings.EqualFold(left.Hostname(), right.Hostname()) &&
		httpURLPort(left) == httpURLPort(right)
}

func httpURLPort(parsed *url.URL) string {
	if port := parsed.Port(); port != "" {
		return port
	}
	if strings.EqualFold(parsed.Scheme, "https") {
		return "443"
	}
	if strings.EqualFold(parsed.Scheme, "http") {
		return "80"
	}
	return ""
}

func validateClientMetadataURL(raw string) error {
	if _, err := canonicalCIMDURL(raw); err != nil {
		return errors.New("mcp auth: client metadata URL must be a non-root HTTPS URL")
	}
	return nil
}

func canonicalCIMDURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || !strings.EqualFold(parsed.Scheme, "https") || parsed.Host == "" ||
		parsed.Path == "" ||
		parsed.Path == "/" ||
		parsed.User != nil {
		return "", errors.New("mcp auth: invalid CIMD URL")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	return parsed.String(), nil
}

func issuersEqual(left string, right string) bool {
	return strings.TrimRight(
		strings.TrimSpace(left),
		"/",
	) == strings.TrimRight(
		strings.TrimSpace(right),
		"/",
	)
}
