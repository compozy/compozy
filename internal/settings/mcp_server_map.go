package settings

import (
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
)

func mcpServerMap(server aghconfig.MCPServer) map[string]any {
	values := map[string]any{}
	if server.Transport != "" {
		values["transport"] = string(server.Transport)
	}
	if strings.TrimSpace(server.Command) != "" {
		values["command"] = strings.TrimSpace(server.Command)
	}
	if len(server.Args) > 0 {
		values["args"] = append([]string(nil), server.Args...)
	}
	if len(server.Env) > 0 {
		values[settingsCredentialSourceEnv] = cloneStringMap(server.Env)
	}
	if len(server.SecretEnv) > 0 {
		values["secret_env"] = cloneStringMap(server.SecretEnv)
	}
	if strings.TrimSpace(server.URL) != "" {
		values["url"] = strings.TrimSpace(server.URL)
	}
	if !server.Auth.IsZero() {
		values["auth"] = mcpAuthMap(server.Auth)
	}
	if strings.TrimSpace(server.CatalogEntry) != "" {
		values["catalog_entry"] = strings.TrimSpace(server.CatalogEntry)
	}
	if strings.TrimSpace(server.CatalogVersion) != "" {
		values["catalog_version"] = strings.TrimSpace(server.CatalogVersion)
	}
	return values
}
