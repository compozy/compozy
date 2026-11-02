package session

import (
	"os"
	"runtime"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/procutil"
	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) sessionStartOpts(
	s *sessionStartSpec,
	session *Session,
	resolved compozyconfig.ResolvedAgent,
	mcpServers []compozyconfig.MCPServer,
) (acp.StartOpts, error) {
	env := sessionStartEnvForProvider(
		os.Environ(),
		session,
		resolved.EnvPolicy,
		strings.TrimSpace(s.reasoningEffort),
	)
	env = setSessionStartEnvValue(env, "COMPOZY_HOME", m.homePaths.HomeDir)
	providerID := strings.TrimSpace(resolved.RuntimeProvider)
	if providerID == "" {
		providerID = strings.TrimSpace(resolved.Provider)
	}
	preferredModel := preferredACPModel(resolved, startModelSelectionIsExplicit(s, resolved))
	launchModelID := ""
	switch providerID {
	case runtimeProviderClaude:
		if transportModel := strings.TrimSpace(s.transportModel); transportModel != "" {
			preferredModel = transportModel
		}
	case cursorRuntimeProvider:
		launchModelID = strings.TrimSpace(s.transportModel)
	}
	shellWaitGuard := m.sessionCanRedirectShellWaits(s, resolved)
	opts := acp.StartOpts{
		AgentName:         resolved.Name,
		CompozySessionID:  s.sessionID,
		WorkspaceID:       s.workspace.ID,
		ProfileID:         s.profileID,
		RuntimeGeneration: max(session.RuntimeGeneration, 1),
		Command:           resolved.Command,
		Cwd:               s.cwd,
		AdditionalDirs:    s.executionAdditionalDirs(),
		Env:               env,
		MCPServers:        mcpServers,
		Permissions:       m.startPermissions(startSpecPermissions(s, resolved.Permissions)),
		SystemPrompt:      resolved.Prompt,
		PreferredModel:    preferredModel,
		LaunchModelID:     launchModelID,
		ReasoningEffort:   strings.TrimSpace(s.reasoningEffort),
		Speed:             s.speed,
		ACPOptions:        acp.CloneSessionConfigOptionSelections(s.acpOptions),
		ResumeSessionID:   s.acpSessionID,
		InputOnly:         IsInternalSpawnRole(store.NormalizeSessionLineage(s.sessionID, s.lineage).SpawnRole),
		ShellWaitGuard:    shellWaitGuard,
		ToolGateway:       newProviderNativeToolGateway(m, session, s.runtimeMode, shellWaitGuard),
		ActivateMCPServers: m.sessionMCPServerActivator(
			s.sessionID,
			mcpServers,
		),
	}
	if s.sessionType == SessionTypeTriage {
		opts.ExecutionPolicy = acp.ExecutionPolicyTriage
		opts.Permissions = compozyconfig.PermissionModeApproveReads
		opts.MCPServers = nil
		opts.ActivateMCPServers = nil
		opts.ShellWaitGuard = false
	}
	return applyProviderRuntimeAdapter(resolved, opts)
}

func startModelSelectionIsExplicit(s *sessionStartSpec, resolved compozyconfig.ResolvedAgent) bool {
	if s != nil && strings.TrimSpace(s.model) != "" {
		return true
	}
	runtimeProvider := strings.TrimSpace(resolved.RuntimeProvider)
	if runtimeProvider == "" {
		runtimeProvider = strings.TrimSpace(resolved.Provider)
	}
	return runtimeProvider == cursorRuntimeProvider && strings.TrimSpace(resolved.Model) != ""
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
	env = setSessionStartEnvValue(env, "COMPOZY_PROFILE_ID", session.ProfileID)
	env = setSessionStartEnvValue(env, "COMPOZY_WORKSPACE_ID", session.WorkspaceID)
	env = setSessionStartEnvValue(env, "COMPOZY_AGENT", strings.TrimSpace(session.AgentName))
	env = setSessionStartEnvValue(env, "COMPOZY_AGENT_NAME", strings.TrimSpace(session.AgentName))

	if effort := strings.TrimSpace(reasoningEffort); effort != "" {
		env = setSessionStartEnvValue(env, "COMPOZY_REASONING_EFFORT", effort)
	} else {
		env = unsetSessionStartEnvKeys(env, "COMPOZY_REASONING_EFFORT")
	}

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
		if ok && sessionEnvironmentKey(existingKey) == sessionEnvironmentKey(trimmedKey) {
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
			blocked[sessionEnvironmentKey(trimmedKey)] = struct{}{}
		}
	}
	if len(blocked) == 0 {
		return env
	}

	filtered := make([]string, 0, len(env))
	for _, current := range env {
		existingKey, _, ok := strings.Cut(current, "=")
		if ok {
			if _, blockedKey := blocked[sessionEnvironmentKey(existingKey)]; blockedKey {
				continue
			}
		}
		filtered = append(filtered, current)
	}
	return filtered
}

func sessionEnvironmentKey(name string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(name)
	}
	return name
}
