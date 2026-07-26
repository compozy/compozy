package config

import (
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/vault"
)

// Validate reports whether the harness is supported.
func (h ProviderHarness) Validate(path string) error {
	switch h {
	case "", ProviderHarnessACP, ProviderHarnessPiACP:
		return nil
	default:
		return fmt.Errorf("%s must be one of acp or pi_acp", path)
	}
}

// Validate reports whether the provider auth mode is supported.
func (m ProviderAuthMode) Validate(path string) error {
	switch m {
	case "", ProviderAuthModeNativeCLI, ProviderAuthModeBoundSecret, ProviderAuthModeNone:
		return nil
	default:
		return fmt.Errorf("%s must be one of native_cli, bound_secret, or none", path)
	}
}

// Validate reports whether the provider env policy is supported.
func (p ProviderEnvPolicy) Validate(path string) error {
	switch p {
	case "", ProviderEnvPolicyFiltered, ProviderEnvPolicyIsolated:
		return nil
	default:
		return fmt.Errorf("%s must be one of filtered or isolated", path)
	}
}

// Validate reports whether the provider home policy is supported.
func (p ProviderHomePolicy) Validate(path string) error {
	switch p {
	case "", ProviderHomePolicyOperator, ProviderHomePolicyIsolated:
		return nil
	default:
		return fmt.Errorf("%s must be one of operator or isolated", path)
	}
}

// Validate reports whether the auth_mode=none safety rationale is supported.
func (s ProviderNoneSecurity) Validate(path string) error {
	switch s {
	case "",
		ProviderNoneSecurityLocalTransport,
		ProviderNoneSecurityExternalIdentity,
		ProviderNoneSecurityPublicReadonly:
		return nil
	default:
		return fmt.Errorf("%s must be one of local_transport, external_identity, or public_readonly", path)
	}
}

// Validate reports whether the provider credential slot can be resolved at launch.
func (s ProviderCredentialSlot) Validate(path string) error {
	switch {
	case strings.TrimSpace(s.Name) == "":
		return fmt.Errorf("%s.name is required", path)
	case strings.TrimSpace(s.TargetEnv) == "":
		return fmt.Errorf("%s.target_env is required", path)
	case !vault.EnvNamePattern.MatchString(strings.TrimSpace(s.TargetEnv)):
		return fmt.Errorf("%s.target_env must be an environment variable name", path)
	case strings.TrimSpace(s.SecretRef) == "":
		return fmt.Errorf("%s.secret_ref is required", path)
	case !validProviderSecretRef(s.SecretRef):
		return fmt.Errorf("%s.secret_ref must be env:VAR or vault:providers/<provider>/<slot>", path)
	default:
		return nil
	}
}

func validProviderSecretRef(ref string) bool {
	normalized := vault.NormalizeRef(ref)
	if vault.IsEnvRef(normalized) {
		return vault.ValidateRef(normalized) == nil
	}
	if err := vault.ValidateSecretRefNamespace(normalized, providersConfigKey); err != nil {
		return false
	}
	path := strings.TrimPrefix(normalized, "vault:providers/")
	segments := strings.Split(path, "/")
	if len(segments) < 2 {
		return false
	}
	for _, segment := range segments {
		if !providerSecretRefSegmentPattern.MatchString(segment) {
			return false
		}
	}
	return true
}

// EffectiveTransport returns the explicit transport or the compatibility
// default. Local command servers remain stdio; servers with a URL default to
// streamable HTTP.
func (s MCPServer) EffectiveTransport() MCPServerTransport {
	if s.Transport != "" {
		return s.Transport
	}
	if strings.TrimSpace(s.URL) != "" {
		return MCPServerTransportHTTP
	}
	return MCPServerTransportStdio
}

// Validate reports whether the transport is supported.
func (t MCPServerTransport) Validate(path string) error {
	switch t {
	case "", MCPServerTransportStdio, MCPServerTransportHTTP, MCPServerTransportSSE:
		return nil
	default:
		return fmt.Errorf("%s must be one of stdio, http, or sse", path)
	}
}

// IsZero reports whether the auth config is empty.
func (a MCPAuthConfig) IsZero() bool {
	return strings.TrimSpace(string(a.Type)) == "" &&
		strings.TrimSpace(a.IssuerURL) == "" &&
		strings.TrimSpace(a.MetadataURL) == "" &&
		strings.TrimSpace(a.AuthorizationURL) == "" &&
		strings.TrimSpace(a.TokenURL) == "" &&
		strings.TrimSpace(a.RevocationURL) == "" &&
		strings.TrimSpace(a.ClientID) == "" &&
		strings.TrimSpace(a.ClientSecretRef) == "" &&
		len(a.Scopes) == 0
}

// Enabled reports whether auth is configured.
func (a MCPAuthConfig) Enabled() bool {
	return !a.IsZero()
}
