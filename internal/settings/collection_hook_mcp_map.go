package settings

import (
	"fmt"

	"path/filepath"

	"strings"

	aghconfig "github.com/compozy/agh/internal/config"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

func normalizeHookDeclaration(name string, declaration hookspkg.HookDecl) (hookspkg.HookDecl, error) {
	normalized := cloneHookDecl(declaration)
	normalized.Name = strings.TrimSpace(normalized.Name)
	if normalized.Name == "" {
		normalized.Name = name
	}
	if normalized.Name != name {
		return hookspkg.HookDecl{}, validationError(fmt.Errorf(
			"settings: hook payload name %q does not match request name %q",
			normalized.Name,
			name,
		))
	}
	if err := hookspkg.ValidateHookDecl(normalized); err != nil {
		return hookspkg.HookDecl{}, validationError(fmt.Errorf("settings: validate hook %q: %w", name, err))
	}
	return normalized, nil
}

func hookMatcherMap(declaration hookspkg.HookDecl) map[string]any {
	matcher := map[string]any{}
	hookMatcherString(matcher, "agent_name", declaration.Matcher.AgentName)
	hookMatcherString(matcher, "agent_type", declaration.Matcher.AgentType)
	hookMatcherString(matcher, "workspace_id", declaration.Matcher.WorkspaceID)
	hookMatcherString(matcher, "workspace_root", declaration.Matcher.WorkspaceRoot)
	hookMatcherString(matcher, "session_type", declaration.Matcher.SessionType)
	hookMatcherString(matcher, "input_class", declaration.Matcher.InputClass)
	hookMatcherString(matcher, "acp_event_type", declaration.Matcher.ACPEventType)
	hookMatcherString(matcher, "turn_id", declaration.Matcher.TurnID)
	hookMatcherString(matcher, "tool_id", declaration.Matcher.ToolID)
	hookMatcherString(matcher, "tool_name", declaration.Matcher.ToolName)
	if declaration.Matcher.ToolReadOnly != nil {
		matcher["tool_read_only"] = *declaration.Matcher.ToolReadOnly
	}
	hookMatcherString(matcher, "decision_class", declaration.Matcher.DecisionClass)
	hookMatcherString(matcher, "message_role", declaration.Matcher.MessageRole)
	hookMatcherString(matcher, "message_delta_type", declaration.Matcher.MessageDeltaType)
	hookNetworkMatcherMap(matcher, declaration.Matcher.NetworkMatcher)
	hookCompactionMatcherMap(matcher, declaration.Matcher.CompactionMatcher)
	return matcher
}

func hookMatcherString(matcher map[string]any, key string, value string) {
	if strings.TrimSpace(value) != "" {
		matcher[key] = value
	}
}

func hookNetworkMatcherMap(matcher map[string]any, network *hookspkg.NetworkMatcher) {
	if network == nil {
		return
	}
	hookMatcherString(matcher, "channel", network.Channel)
	hookMatcherString(matcher, "surface", network.Surface)
	hookMatcherString(matcher, "kind", network.Kind)
	hookMatcherString(matcher, "direction", network.Direction)
	hookMatcherString(matcher, "work_state", network.WorkState)
}

func hookCompactionMatcherMap(matcher map[string]any, compaction *hookspkg.CompactionMatcher) {
	if compaction == nil {
		return
	}
	hookMatcherString(matcher, "compaction_reason", compaction.Reason)
	hookMatcherString(matcher, "compaction_strategy", compaction.Strategy)
}

func hookExecutorMap(declaration hookspkg.HookDecl) map[string]any {
	values := map[string]any{}
	if declaration.ExecutorKind != "" {
		values["kind"] = string(declaration.ExecutorKind)
	}
	if strings.TrimSpace(declaration.Command) != "" {
		values["command"] = declaration.Command
	}
	if len(declaration.Args) > 0 {
		values["args"] = append([]string(nil), declaration.Args...)
	}
	if len(declaration.Env) > 0 {
		values[settingsCredentialSourceEnv] = cloneStringMap(declaration.Env)
	}
	if len(declaration.SecretEnv) > 0 {
		values["secret_env"] = cloneStringMap(declaration.SecretEnv)
	}
	return values
}

func mcpAuthMap(auth aghconfig.MCPAuthConfig) map[string]any {
	values := map[string]any{}
	if auth.Type != "" {
		values["type"] = string(auth.Type)
	}
	if strings.TrimSpace(auth.IssuerURL) != "" {
		values["issuer_url"] = strings.TrimSpace(auth.IssuerURL)
	}
	if strings.TrimSpace(auth.MetadataURL) != "" {
		values["metadata_url"] = strings.TrimSpace(auth.MetadataURL)
	}
	if strings.TrimSpace(auth.AuthorizationURL) != "" {
		values["authorization_url"] = strings.TrimSpace(auth.AuthorizationURL)
	}
	if strings.TrimSpace(auth.TokenURL) != "" {
		values["token_url"] = strings.TrimSpace(auth.TokenURL)
	}
	if strings.TrimSpace(auth.RevocationURL) != "" {
		values["revocation_url"] = strings.TrimSpace(auth.RevocationURL)
	}
	if strings.TrimSpace(auth.ClientID) != "" {
		values["client_id"] = strings.TrimSpace(auth.ClientID)
	}
	if strings.TrimSpace(auth.ClientSecretRef) != "" {
		values["client_secret_ref"] = strings.TrimSpace(auth.ClientSecretRef)
	}
	if len(auth.Scopes) > 0 {
		values["scopes"] = append([]string(nil), auth.Scopes...)
	}
	return values
}

func (s *service) commandAvailable(command string) bool {
	binary := firstCommandToken(command)
	if binary == "" {
		return false
	}
	_, err := s.commandLookPath(binary)
	return err == nil
}

func firstCommandToken(command string) string {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func (s *service) envPresent(name string) bool {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return false
	}
	value, ok := s.lookupEnv(trimmed)
	return ok && strings.TrimSpace(value) != ""
}

func workspaceMCPSidecarPath(root string) string {
	return filepath.Join(strings.TrimSpace(root), aghconfig.DirName, aghconfig.MCPJSONName)
}
