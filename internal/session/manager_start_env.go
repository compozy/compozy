package session

import (
	"os"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/procutil"
)

func (m *Manager) sessionStartOpts(
	s *sessionStartSpec,
	session *Session,
	resolved compozyconfig.ResolvedAgent,
	mcpServers []compozyconfig.MCPServer,
) acp.StartOpts {
	env := sessionStartEnvForProvider(
		os.Environ(),
		session,
		resolved.EnvPolicy,
		strings.TrimSpace(s.reasoningEffort),
	)
	return acp.StartOpts{
		AgentName:       resolved.Name,
		Command:         resolved.Command,
		Cwd:             s.cwd,
		AdditionalDirs:  append([]string(nil), s.workspace.AdditionalDirs...),
		Env:             env,
		MCPServers:      mcpServers,
		Permissions:     m.startPermissions(session.Type, startSpecPermissions(s, resolved.Permissions)),
		SystemPrompt:    resolved.Prompt,
		PreferredModel:  preferredACPModel(resolved, strings.TrimSpace(s.model) != ""),
		ReasoningEffort: strings.TrimSpace(s.reasoningEffort),
		Speed:           s.speed,
		ResumeSessionID: s.acpSessionID,
		ToolGateway:     newProviderNativeToolGateway(m, session, s.runtimeMode),
	}
}

func startSpecPermissions(s *sessionStartSpec, fallback string) string {
	if s == nil || strings.TrimSpace(string(s.permissions)) == "" {
		return fallback
	}
	return string(s.permissions)
}

func sessionStartEnv(base []string, session *Session) []string {
	reasoningEffort := ""
	if session != nil {
		reasoningEffort = session.ReasoningEffort
	}
	return sessionStartEnvForProvider(
		base,
		session,
		compozyconfig.ProviderEnvPolicyFiltered,
		reasoningEffort,
	)
}

func sessionStartEnvForProvider(
	base []string,
	session *Session,
	envPolicy compozyconfig.ProviderEnvPolicy,
	reasoningEffort string,
) []string {
	env := procutil.FilteredDaemonEnv(base)
	if envPolicy == compozyconfig.ProviderEnvPolicyIsolated {
		env = procutil.IsolatedDaemonEnv(base)
	}
	if session == nil {
		return env
	}

	env = setSessionStartEnvValue(env, "COMPOZY_SESSION_ID", strings.TrimSpace(session.ID))
	env = setSessionStartEnvValue(env, "COMPOZY_AGENT", strings.TrimSpace(session.AgentName))
	env = setSessionStartEnvValue(env, "COMPOZY_AGENT_NAME", strings.TrimSpace(session.AgentName))
	env = unsetSessionStartEnvKeys(env, "COMPOZY_SESSION_CHANNEL", "COMPOZY_PEER_ID")

	if effort := strings.TrimSpace(reasoningEffort); effort != "" {
		env = setSessionStartEnvValue(env, "COMPOZY_REASONING_EFFORT", effort)
	} else {
		env = unsetSessionStartEnvKeys(env, "COMPOZY_REASONING_EFFORT")
	}

	if session.NetworkParticipation.Mode != participation.ModeLive {
		return env
	}
	channel := strings.TrimSpace(session.NetworkParticipation.ChannelID)

	env = setSessionStartEnvValue(env, "COMPOZY_SESSION_CHANNEL", channel)
	env = setSessionStartEnvValue(env, "COMPOZY_PEER_ID", networkPeerID(session.AgentName, session.ID))
	return env
}

func setSessionStartEnvValue(env []string, key string, value string) []string {
	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" {
		return env
	}
	entry := trimmedKey + "=" + value
	for i, current := range env {
		existingKey, _, ok := strings.Cut(current, "=")
		if ok && existingKey == trimmedKey {
			env[i] = entry
			return env
		}
	}
	return append(env, entry)
}

func unsetSessionStartEnvKeys(env []string, keys ...string) []string {
	if len(env) == 0 || len(keys) == 0 {
		return env
	}

	blocked := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey != "" {
			blocked[trimmedKey] = struct{}{}
		}
	}
	if len(blocked) == 0 {
		return env
	}

	filtered := make([]string, 0, len(env))
	for _, current := range env {
		existingKey, _, ok := strings.Cut(current, "=")
		if ok {
			if _, blockedKey := blocked[existingKey]; blockedKey {
				continue
			}
		}
		filtered = append(filtered, current)
	}
	return filtered
}
