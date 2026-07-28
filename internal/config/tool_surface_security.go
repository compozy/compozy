package config

import (
	"fmt"

	"sort"

	"strings"
)

func flattenConfigValue(entries *[]Entry, path string, value any, redacted bool) {
	switch typed := value.(type) {
	case map[string]any:
		if len(typed) == 0 {
			if path != "" {
				*entries = append(*entries, Entry{Path: path, Value: map[string]any{}, Redacted: redacted})
			}
			return
		}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			nextPath := key
			if path != "" {
				nextPath = path + "." + key
			}
			flattenConfigValue(entries, nextPath, typed[key], redacted || key == "env" || key == "secret_env")
		}
	case []any:
		if len(typed) == 0 {
			if path != "" {
				*entries = append(*entries, Entry{Path: path, Value: []any{}, Redacted: redacted})
			}
			return
		}
		for i, item := range typed {
			flattenConfigValue(entries, fmt.Sprintf("%s[%d]", path, i), item, redacted)
		}
	default:
		if path != "" {
			*entries = append(*entries, Entry{Path: path, Value: typed, Redacted: redacted})
		}
	}
}

func configPathHasArraySyntax(path []string) bool {
	for _, segment := range path {
		if strings.ContainsAny(segment, "[]") {
			return true
		}
	}
	return false
}

func configPathIsSecret(path []string) bool {
	for _, segment := range path {
		lower := strings.ToLower(strings.TrimSpace(segment))
		switch lower {
		case "env", "secret_env", "secret_ref", "client_secret_ref", toolSurfaceWebhookSecretRefKey:
			return true
		}
		if strings.Contains(lower, "secret") ||
			strings.Contains(lower, "token") ||
			strings.Contains(lower, "password") ||
			strings.Contains(lower, "authorization") {
			return true
		}
	}
	return false
}

func configPathIsTrustRoot(path []string) bool {
	if len(path) == 0 {
		return true
	}
	switch path[0] {
	case "daemon", string(MCPServerTransportHTTP), toolSurfacePermissionsKey, "observability", "log",
		toolSurfaceMCPServersKey, "sandboxes", "autonomy":
		return true
	case toolSurfaceHooksKey:
		return true
	case providersConfigKey:
		return providerConfigPathIsTrustRoot(path)
	case MemoryDirName:
		return memoryConfigPathIsTrustRoot(path)
	case "tools":
		return toolsConfigPathIsTrustRoot(path)
	case SkillsDirName:
		return skillsConfigPathIsTrustRoot(path)
	case "extensions":
		return true
	case toolSurfaceMarketplaceKey:
		return len(path) >= 3 && path[1] == "catalog" && path[2] == "base_url"
	}
	return false
}

func providerConfigPathIsTrustRoot(path []string) bool {
	if len(path) < 3 {
		return false
	}
	return path[2] == toolSurfaceCommandKey || path[2] == toolSurfaceMCPServersKey
}

func memoryConfigPathIsTrustRoot(path []string) bool {
	if len(path) < 2 {
		return false
	}
	switch path[1] {
	case "global_dir":
		return true
	case toolSurfaceExtractorKey:
		return len(path) >= 3 && (path[2] == "inbox_path" || path[2] == "dlq_path")
	case "session":
		return len(path) >= 3 && path[2] == "ledger_root"
	case "daily":
		return len(path) >= 3 && path[2] == "archive_path"
	case string(WriteScopeWorkspace):
		return len(path) >= 3 && path[2] == "toml_path"
	default:
		return false
	}
}

func toolsConfigPathIsTrustRoot(path []string) bool {
	if len(path) < 2 {
		return false
	}
	switch path[1] {
	case string(ToolsExternalDefaultEnabled), "hosted_mcp_enabled", "hosted_mcp", "policy":
		return true
	default:
		return false
	}
}

func skillsConfigPathIsTrustRoot(path []string) bool {
	if len(path) < 2 {
		return false
	}
	switch path[1] {
	case "allowed_marketplace_mcp", "allowed_marketplace_hooks", toolSurfaceMarketplaceKey:
		return true
	default:
		return false
	}
}
