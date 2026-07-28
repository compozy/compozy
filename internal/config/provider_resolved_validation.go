package config

import (
	"fmt"

	"strings"
)

func validateResolvedProvider(name string, provider ProviderConfig) error {
	if strings.TrimSpace(provider.Command) == "" {
		return fmt.Errorf("provider %q command is required", name)
	}
	if err := provider.Models.Validate(fmt.Sprintf("providers.%s.models", name)); err != nil {
		return err
	}
	if err := provider.EffectiveHarness().Validate(fmt.Sprintf("providers.%s.harness", name)); err != nil {
		return err
	}
	if err := provider.EffectiveAuthMode().Validate(fmt.Sprintf("providers.%s.auth_mode", name)); err != nil {
		return err
	}
	if err := provider.EffectiveEnvPolicy().Validate(fmt.Sprintf("providers.%s.env_policy", name)); err != nil {
		return err
	}
	if err := provider.EffectiveHomePolicy().Validate(fmt.Sprintf("providers.%s.home_policy", name)); err != nil {
		return err
	}
	if err := provider.EffectiveNoneSecurity().Validate(fmt.Sprintf("providers.%s.none_security", name)); err != nil {
		return err
	}
	if provider.EffectiveHarness() == ProviderHarnessPiACP &&
		strings.TrimSpace(provider.RuntimeProviderName(name)) == "" {
		return fmt.Errorf("providers.%s.runtime_provider is required for pi_acp providers", name)
	}
	slots, err := validateProviderAuthSlots(name, provider)
	if err != nil {
		return err
	}
	for i, slot := range slots {
		if err := slot.Validate(fmt.Sprintf("providers.%s.credential_slots[%d]", name, i)); err != nil {
			return err
		}
	}

	for i, server := range provider.MCPServers {
		if err := server.Validate(fmt.Sprintf("providers.%s.mcp_servers[%d]", name, i)); err != nil {
			return err
		}
	}

	return nil
}

func validateProviderAuthSlots(name string, provider ProviderConfig) ([]ProviderCredentialSlot, error) {
	authMode := provider.EffectiveAuthMode()
	slots := provider.EffectiveCredentialSlots()
	if builtinNativeAuthProvider(name) && provider.AuthMode == "" && len(slots) > 0 {
		return nil, fmt.Errorf(
			"providers.%s.auth_mode must be %q before credential_slots can override native CLI authentication",
			name,
			ProviderAuthModeBoundSecret,
		)
	}
	switch authMode {
	case ProviderAuthModeBoundSecret:
		if len(slots) == 0 {
			return nil, fmt.Errorf("providers.%s.credential_slots is required when auth_mode is bound_secret", name)
		}
	case ProviderAuthModeNativeCLI:
		if len(slots) > 0 {
			return nil, fmt.Errorf(
				"providers.%s.credential_slots requires auth_mode = %q; native_cli uses provider-owned login state",
				name,
				ProviderAuthModeBoundSecret,
			)
		}
	case ProviderAuthModeNone:
		if len(slots) > 0 {
			return nil, fmt.Errorf("providers.%s.credential_slots cannot be set when auth_mode is none", name)
		}
		if strings.TrimSpace(provider.AuthStatusCmd) != "" {
			return nil, fmt.Errorf("providers.%s.auth_status_command cannot be set when auth_mode is none", name)
		}
		if strings.TrimSpace(provider.AuthLoginCmd) != "" {
			return nil, fmt.Errorf("providers.%s.auth_login_command cannot be set when auth_mode is none", name)
		}
	}
	return slots, nil
}

func builtinNativeAuthProvider(name string) bool {
	builtin, ok := builtinProviders[name]
	return ok && builtin.EffectiveAuthMode() == ProviderAuthModeNativeCLI
}

func mergeMCPServerInto(merged *MCPServer, override MCPServer) {
	if strings.TrimSpace(override.Name) != "" {
		merged.Name = override.Name
	}
	if override.Transport != "" {
		merged.Transport = override.Transport
	}
	if strings.TrimSpace(override.Command) != "" {
		merged.Command = override.Command
	}
	if len(override.Args) > 0 {
		merged.Args = append([]string(nil), override.Args...)
	}
	if len(override.Env) > 0 {
		merged.Env = mergeStringMaps(merged.Env, override.Env)
	}
	if len(override.SecretEnv) > 0 {
		merged.SecretEnv = mergeStringMaps(merged.SecretEnv, override.SecretEnv)
	}
	if strings.TrimSpace(override.URL) != "" {
		merged.URL = override.URL
	}
	if !override.Auth.IsZero() {
		merged.Auth = mergeMCPAuthConfig(merged.Auth, override.Auth)
	}
	if strings.TrimSpace(override.CatalogEntry) != "" {
		merged.CatalogEntry = override.CatalogEntry
	}
	if strings.TrimSpace(override.CatalogVersion) != "" {
		merged.CatalogVersion = override.CatalogVersion
	}
}

func mergeMCPAuthConfig(base MCPAuthConfig, override MCPAuthConfig) MCPAuthConfig {
	merged := cloneMCPAuthConfig(base)
	if override.Type != "" {
		merged.Type = override.Type
	}
	if strings.TrimSpace(override.IssuerURL) != "" {
		merged.IssuerURL = override.IssuerURL
	}
	if strings.TrimSpace(override.MetadataURL) != "" {
		merged.MetadataURL = override.MetadataURL
	}
	if strings.TrimSpace(override.AuthorizationURL) != "" {
		merged.AuthorizationURL = override.AuthorizationURL
	}
	if strings.TrimSpace(override.TokenURL) != "" {
		merged.TokenURL = override.TokenURL
	}
	if strings.TrimSpace(override.RevocationURL) != "" {
		merged.RevocationURL = override.RevocationURL
	}
	if strings.TrimSpace(override.ClientID) != "" {
		merged.ClientID = override.ClientID
	}
	if strings.TrimSpace(override.ClientSecretRef) != "" {
		merged.ClientSecretRef = override.ClientSecretRef
	}
	if len(override.Scopes) > 0 {
		merged.Scopes = append([]string(nil), override.Scopes...)
	}
	return merged
}
