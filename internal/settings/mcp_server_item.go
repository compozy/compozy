package settings

import (
	"sort"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
)

// MCPServerItem is one MCP server collection row.
type MCPServerItem struct {
	Name                   string
	Transport              compozyconfig.MCPServerTransport
	Command                string
	Args                   []string
	EnvKeys                []string
	SecretEnvKeys          []string
	URL                    string
	Auth                   compozyconfig.MCPAuthConfig
	ClientSecretConfigured bool
	AuthStatus             *mcpauth.Status
	RuntimeStatus          *MCPServerRuntimeStatus
	Scope                  ScopeKind
	WorkspaceID            string
	ProfileName            string
	CatalogEntry           string
	CatalogVersion         string
	SourceMetadata         SourceMetadata
}

func baseMCPServerItem(
	effective mcpSourceEntry,
	entries []mcpSourceEntry,
	scope ScopeKind,
	workspaceID string,
	profileName string,
) MCPServerItem {
	profileName = strings.TrimSpace(profileName)
	shadowed := make([]SourceRef, 0, len(entries)-1)
	for idx := len(entries) - 2; idx >= 0; idx-- {
		shadowed = append(shadowed, entries[idx].Source)
	}
	auth := effective.Server.Auth
	clientSecretConfigured := strings.TrimSpace(auth.ClientSecretRef) != ""
	auth.ClientSecretRef = ""
	return MCPServerItem{
		Name:                   effective.Server.Name,
		Transport:              effective.Server.EffectiveTransport(),
		Command:                effective.Server.Command,
		Args:                   append([]string(nil), effective.Server.Args...),
		EnvKeys:                mcpEnvKeys(effective.Server.Env),
		SecretEnvKeys:          mcpSecretEnvKeys(effective.Server.SecretEnv),
		URL:                    strings.TrimSpace(effective.Server.URL),
		Auth:                   auth,
		ClientSecretConfigured: clientSecretConfigured,
		Scope:                  scope,
		WorkspaceID:            workspaceID,
		ProfileName:            profileName,
		CatalogEntry:           strings.TrimSpace(effective.Server.CatalogEntry),
		CatalogVersion:         strings.TrimSpace(effective.Server.CatalogVersion),
		SourceMetadata: SourceMetadata{
			EffectiveSource:  effective.Source,
			ShadowedSources:  shadowed,
			AvailableTargets: availableTargetsForScope(scope),
		},
	}
}

func mcpEnvKeys(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			keys = append(keys, trimmed)
		}
	}
	sort.Strings(keys)
	return keys
}

func mcpSecretEnvKeys(secretEnv map[string]string) []string {
	if len(secretEnv) == 0 {
		return nil
	}
	keys := make([]string, 0, len(secretEnv))
	for key := range secretEnv {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			keys = append(keys, trimmed)
		}
	}
	sort.Strings(keys)
	return keys
}

func cloneMCPServerItem(value MCPServerItem) MCPServerItem {
	value.Args = append([]string(nil), value.Args...)
	value.EnvKeys = append([]string(nil), value.EnvKeys...)
	value.SecretEnvKeys = append([]string(nil), value.SecretEnvKeys...)
	value.Auth.Scopes = append([]string(nil), value.Auth.Scopes...)
	if value.AuthStatus != nil {
		status := *value.AuthStatus
		status.Scopes = append([]string(nil), value.AuthStatus.Scopes...)
		if value.AuthStatus.ExpiresAt != nil {
			expiresAt := *value.AuthStatus.ExpiresAt
			status.ExpiresAt = &expiresAt
		}
		if value.AuthStatus.UpdatedAt != nil {
			updatedAt := *value.AuthStatus.UpdatedAt
			status.UpdatedAt = &updatedAt
		}
		value.AuthStatus = &status
	}
	if value.RuntimeStatus != nil {
		status := *value.RuntimeStatus
		value.RuntimeStatus = &status
	}
	value.SourceMetadata = cloneSourceMetadata(value.SourceMetadata)
	return value
}

func committedMCPServerItem(
	server compozyconfig.MCPServer,
	scope ScopeKind,
	workspaceID string,
	profileName string,
	target WriteTargetKind,
	sources map[string][]mcpSourceEntry,
) MCPServerItem {
	committed := mcpSourceEntry{
		Source: sourceRefForWriteTarget(target, workspaceID, profileName),
		Target: target,
		Server: server,
	}
	entries := append([]mcpSourceEntry(nil), sources[strings.TrimSpace(server.Name)]...)
	replaced := false
	for idx := range entries {
		if entries[idx].Target == target {
			entries[idx] = committed
			replaced = true
			break
		}
	}
	if !replaced {
		entries = append(entries, committed)
	}
	sort.SliceStable(entries, func(left, right int) bool {
		return mcpSourcePrecedence(entries[left].Target) < mcpSourcePrecedence(entries[right].Target)
	})
	return baseMCPServerItem(entries[len(entries)-1], entries, scope, workspaceID, profileName)
}

func mcpSourcePrecedence(target WriteTargetKind) int {
	switch target {
	case WriteTargetGlobalConfig:
		return 0
	case WriteTargetGlobalMCPSidecar:
		return 1
	case WriteTargetProfileConfig:
		return 2
	case WriteTargetProfileMCPSidecar:
		return 3
	case WriteTargetWorkspaceConfig:
		return 4
	case WriteTargetWorkspaceMCPSidecar:
		return 5
	case WriteTargetWorkspaceProfileConfig:
		return 6
	case WriteTargetWorkspaceProfileMCPSidecar:
		return 7
	default:
		return -1
	}
}

func cloneMCPServerItemPointer(value *MCPServerItem) *MCPServerItem {
	if value == nil {
		return nil
	}
	cloned := cloneMCPServerItem(*value)
	return &cloned
}
