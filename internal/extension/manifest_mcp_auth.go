package extensionpkg

import (
	"fmt"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func cloneManifestMCPAuth(auth *MCPServerAuthConfig) *MCPServerAuthConfig {
	if auth == nil {
		return nil
	}
	cloned := *auth
	cloned.Method = strings.TrimSpace(auth.Method)
	cloned.Registration = strings.TrimSpace(auth.Registration)
	cloned.IssuerURL = strings.TrimSpace(auth.IssuerURL)
	cloned.ClientID = strings.TrimSpace(auth.ClientID)
	cloned.Scopes = slices.Clone(auth.Scopes)
	return &cloned
}

func validateManifestMCPPolicies(servers map[string]MCPServerConfig) error {
	for _, name := range sortedMapKeys(servers) {
		server := servers[name]
		field := "resources.mcp_servers." + name
		switch server.DefaultScope {
		case "", "global", "workspace":
		default:
			return &ManifestValidationError{Field: field + ".default_scope", Message: "must be global or workspace"}
		}
		if err := validateManifestMCPAuth(server, field+".auth"); err != nil {
			return &ManifestValidationError{Field: field + ".auth", Message: err.Error()}
		}
	}
	return nil
}

func validateManifestMCPAuth(server MCPServerConfig, field string) error {
	if server.Auth == nil {
		return nil
	}
	auth := server.Auth
	switch auth.Method {
	case "none":
		if auth.Registration != "" || auth.IssuerURL != "" || auth.ClientID != "" || len(auth.Scopes) != 0 {
			return fmt.Errorf("method none must not declare OAuth fields")
		}
		return nil
	case "oauth":
		if server.Transport != forgeURLSchemeHTTP {
			return fmt.Errorf("OAuth requires http transport")
		}
	default:
		return fmt.Errorf("method must be none or oauth")
	}
	return manifestMCPAuthConfig(auth).Validate(field)
}

func manifestMCPAuthConfig(auth *MCPServerAuthConfig) compozyconfig.MCPAuthConfig {
	if auth == nil || auth.Method == "none" {
		return compozyconfig.MCPAuthConfig{}
	}
	registration := compozyconfig.MCPAuthRegistration(auth.Registration)
	if registration == "dynamic" || registration == "" {
		registration = compozyconfig.MCPAuthRegistrationAuto
	}
	return compozyconfig.MCPAuthConfig{
		Registration: registration,
		IssuerURL:    auth.IssuerURL,
		ClientID:     auth.ClientID,
		Scopes:       slices.Clone(auth.Scopes),
	}
}
