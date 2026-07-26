package session

import (
	"errors"
	"fmt"

	"sort"
	"strings"

	hookspkg "github.com/compozy/agh/internal/hooks"
	envpkg "github.com/compozy/agh/internal/sandbox"
	"github.com/compozy/agh/internal/store"
)

func initialSessionSandboxMeta(
	sandboxID string,
	resolved envpkg.Resolved,
	previous *store.SessionSandboxMeta,
) *store.SessionSandboxMeta {
	meta := cloneSessionSandboxMeta(previous)
	if meta == nil {
		meta = &store.SessionSandboxMeta{}
	}
	meta.SandboxID = strings.TrimSpace(sandboxID)
	meta.Backend = string(resolved.Backend)
	meta.Profile = strings.TrimSpace(resolved.Profile)
	meta.State = sandboxStateCreating
	return meta
}

func normalizePreparedSandboxState(
	prepared envpkg.Prepared,
	meta *store.SessionSandboxMeta,
	resolved envpkg.Resolved,
) (envpkg.SessionState, error) {
	state := prepared.State
	if strings.TrimSpace(state.SandboxID) == "" {
		state.SandboxID = strings.TrimSpace(meta.SandboxID)
	}
	if state.Backend == "" {
		state.Backend = resolved.Backend
	}
	if strings.TrimSpace(state.Profile) == "" {
		state.Profile = strings.TrimSpace(resolved.Profile)
	}
	if strings.TrimSpace(state.State) == "" {
		state.State = sandboxStatePrepared
	}
	if strings.TrimSpace(state.RuntimeRootDir) == "" {
		state.RuntimeRootDir = strings.TrimSpace(prepared.RuntimeRootDir)
	}
	if strings.TrimSpace(state.RuntimeRootDir) == "" {
		state.RuntimeRootDir = strings.TrimSpace(prepared.Launch.Cwd)
	}
	if len(state.RuntimeAdditionalDirs) == 0 {
		state.RuntimeAdditionalDirs = append([]string(nil), prepared.RuntimeAdditionalDirs...)
	}
	if state.ProviderState == nil {
		state.ProviderState = cloneRawMessage(meta.ProviderState)
	}
	if strings.TrimSpace(state.SandboxID) == "" {
		return envpkg.SessionState{}, errors.New("session: prepared sandbox id is required")
	}
	if !state.Backend.Valid() {
		return envpkg.SessionState{}, fmt.Errorf("session: prepared sandbox backend %q is invalid", state.Backend)
	}
	if strings.TrimSpace(state.RuntimeRootDir) == "" {
		return envpkg.SessionState{}, errors.New("session: prepared runtime root dir is required")
	}
	return state, nil
}

func sessionSandboxMetaFromState(
	state envpkg.SessionState,
	fallbackState string,
) *store.SessionSandboxMeta {
	if strings.TrimSpace(state.SandboxID) == "" && state.Backend == "" {
		return nil
	}
	sessionState := strings.TrimSpace(state.State)
	if sessionState == "" {
		sessionState = fallbackState
	}
	return &store.SessionSandboxMeta{
		SandboxID:             strings.TrimSpace(state.SandboxID),
		Backend:               string(state.Backend),
		Profile:               strings.TrimSpace(state.Profile),
		State:                 sessionState,
		InstanceID:            strings.TrimSpace(state.InstanceID),
		RuntimeRootDir:        strings.TrimSpace(state.RuntimeRootDir),
		RuntimeAdditionalDirs: append([]string(nil), state.RuntimeAdditionalDirs...),
		ProviderState:         cloneRawMessage(state.ProviderState),
		SSHAccessExpiresAt:    cloneTimePointer(state.SSHAccessExpiresAt),
	}
}

func sessionSandboxStateFromMeta(meta *store.SessionSandboxMeta) envpkg.SessionState {
	if meta == nil {
		return envpkg.SessionState{}
	}
	return envpkg.SessionState{
		SandboxID:             strings.TrimSpace(meta.SandboxID),
		Backend:               envpkg.Backend(strings.TrimSpace(meta.Backend)),
		Profile:               strings.TrimSpace(meta.Profile),
		State:                 strings.TrimSpace(meta.State),
		InstanceID:            strings.TrimSpace(meta.InstanceID),
		RuntimeRootDir:        strings.TrimSpace(meta.RuntimeRootDir),
		RuntimeAdditionalDirs: append([]string(nil), meta.RuntimeAdditionalDirs...),
		ProviderState:         cloneRawMessage(meta.ProviderState),
		SSHAccessExpiresAt:    cloneTimePointer(meta.SSHAccessExpiresAt),
	}
}

func sessionSandboxID(meta *store.SessionSandboxMeta) string {
	if meta == nil {
		return ""
	}
	return strings.TrimSpace(meta.SandboxID)
}

func normalizeResolvedSandbox(resolved envpkg.Resolved) envpkg.Resolved {
	if !resolved.Backend.Valid() {
		resolved.Backend = envpkg.BackendLocal
	}
	if strings.TrimSpace(resolved.Profile) == "" {
		resolved.Profile = string(resolved.Backend)
	}
	return resolved
}

func sandboxAgentEnv(base []string, resolved envpkg.Resolved) []string {
	env := append([]string(nil), base...)
	if len(resolved.Env) == 0 {
		return env
	}
	keys := make([]string, 0, len(resolved.Env))
	for key := range resolved.Env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		env = setSessionStartEnvValue(env, key, resolved.Env[key])
	}
	return env
}

func sandboxProfilePayload(resolved envpkg.Resolved) hookspkg.SandboxProfilePayload {
	return hookspkg.SandboxProfilePayload{
		Profile:        strings.TrimSpace(resolved.Profile),
		Backend:        string(resolved.Backend),
		SyncMode:       string(resolved.SyncMode),
		Persistence:    string(resolved.Persistence),
		RuntimeRootDir: strings.TrimSpace(resolved.RuntimeRootDir),
		DestroyOnStop:  resolved.DestroyOnStop,
		Env:            mergeSandboxEnv(nil, resolved.Env),
		SecretEnv:      mergeSandboxEnv(nil, resolved.SecretEnv),
	}
}

func mergeSandboxEnv(base map[string]string, overrides map[string]string) map[string]string {
	if len(base) == 0 && len(overrides) == 0 {
		return nil
	}
	merged := make(map[string]string, len(base)+len(overrides))
	for key, value := range base {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			merged[trimmed] = value
		}
	}
	for key, value := range overrides {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			merged[trimmed] = value
		}
	}
	return merged
}

func applySandboxEnvOverrides(env []string, overrides map[string]string) []string {
	next := append([]string(nil), env...)
	keys := make([]string, 0, len(overrides))
	values := make(map[string]string, len(overrides))
	for key, value := range overrides {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			keys = append(keys, trimmed)
			values[trimmed] = value
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		next = setSessionStartEnvValue(next, key, values[key])
	}
	return next
}

func applySandboxMetaFallbacks(
	sandboxID *string,
	backend *string,
	profile *string,
	instanceID *string,
	meta *store.SessionSandboxMeta,
) {
	if meta == nil {
		return
	}
	if sandboxID != nil && strings.TrimSpace(*sandboxID) == "" {
		*sandboxID = strings.TrimSpace(meta.SandboxID)
	}
	if backend != nil && strings.TrimSpace(*backend) == "" {
		*backend = strings.TrimSpace(meta.Backend)
	}
	if profile != nil && strings.TrimSpace(*profile) == "" {
		*profile = strings.TrimSpace(meta.Profile)
	}
	if instanceID != nil && strings.TrimSpace(*instanceID) == "" {
		*instanceID = strings.TrimSpace(meta.InstanceID)
	}
}
