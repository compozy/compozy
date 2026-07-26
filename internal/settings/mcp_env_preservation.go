package settings

import (
	"fmt"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/vault"
)

func preserveMCPEnvValues(
	name string,
	target WriteTargetKind,
	sources map[string][]mcpSourceEntry,
	server *aghconfig.MCPServer,
	preservation []string,
) error {
	if len(preservation) == 0 {
		return nil
	}
	if server == nil {
		return validationError(fmt.Errorf("settings: MCP server %q is required", name))
	}
	previous, ok := mcpSourceForTarget(name, target, sources)
	if !ok {
		return validationError(fmt.Errorf(
			"settings: MCP server %q has no existing env values in %q to preserve",
			name,
			target,
		))
	}
	for _, rawName := range preservation {
		envName := strings.TrimSpace(rawName)
		if !vault.EnvNamePattern.MatchString(envName) {
			return validationError(fmt.Errorf("settings: MCP preserved env key %q is invalid", envName))
		}
		if _, explicit := mcpEnvValue(server.Env, envName); explicit {
			continue
		}
		value, exists := mcpEnvValue(previous.Server.Env, envName)
		if !exists {
			return validationError(fmt.Errorf(
				"settings: MCP server %q has no existing env value for %q",
				name,
				envName,
			))
		}
		if server.Env == nil {
			server.Env = make(map[string]string)
		}
		server.Env[envName] = value
	}
	return nil
}

func mcpEnvValue(values map[string]string, envName string) (string, bool) {
	for key, value := range values {
		if strings.TrimSpace(key) == envName {
			return value, true
		}
	}
	return "", false
}
