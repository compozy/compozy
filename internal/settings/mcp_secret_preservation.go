package settings

import (
	"fmt"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/vault"
)

func preserveMCPSecretBindings(
	name string,
	target WriteTargetKind,
	sources map[string][]mcpSourceEntry,
	server *aghconfig.MCPServer,
	preservation MCPSecretPreservation,
) error {
	if len(preservation.SecretEnv) == 0 && !preservation.OAuthClientSecret {
		return nil
	}
	if server == nil {
		return validationError(fmt.Errorf("settings: MCP server %q is required", name))
	}
	previous, ok := mcpSourceForTarget(name, target, sources)
	if !ok {
		return validationError(fmt.Errorf(
			"settings: MCP server %q has no existing bindings in %q to preserve",
			name,
			target,
		))
	}
	for _, rawName := range preservation.SecretEnv {
		envName := strings.TrimSpace(rawName)
		if !vault.EnvNamePattern.MatchString(envName) {
			return validationError(fmt.Errorf("settings: MCP preserved secret_env key %q is invalid", envName))
		}
		if _, explicit := mcpSecretEnvRef(server.SecretEnv, envName); explicit {
			continue
		}
		ref, exists := mcpSecretEnvRef(previous.Server.SecretEnv, envName)
		if !exists {
			return validationError(fmt.Errorf(
				"settings: MCP server %q has no existing secret_env binding for %q",
				name,
				envName,
			))
		}
		setMCPSecretEnvRef(server, envName, ref)
	}
	if preservation.OAuthClientSecret && strings.TrimSpace(server.Auth.ClientSecretRef) == "" {
		ref := strings.TrimSpace(previous.Server.Auth.ClientSecretRef)
		if ref == "" {
			return validationError(fmt.Errorf(
				"settings: MCP server %q has no existing OAuth client secret binding",
				name,
			))
		}
		server.Auth.ClientSecretRef = ref
	}
	return nil
}

func mcpSecretEnvRef(values map[string]string, envName string) (string, bool) {
	for key, ref := range values {
		if strings.TrimSpace(key) == envName && strings.TrimSpace(ref) != "" {
			return strings.TrimSpace(ref), true
		}
	}
	return "", false
}
