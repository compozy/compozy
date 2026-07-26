package core

import (
	"bytes"
	"encoding/json"

	"path/filepath"
	"sort"
	"strings"

	"github.com/compozy/agh/internal/api/contract"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	aghconfig "github.com/compozy/agh/internal/config"

	workspacepkg "github.com/compozy/agh/internal/workspace"
)

// WorkspacePayloadFromWorkspace converts a workspace into the shared payload.
func WorkspacePayloadFromWorkspace(workspace workspacepkg.Workspace) contract.WorkspacePayload {
	addDirs := make([]string, 0, len(workspace.AdditionalDirs))
	addDirs = append(addDirs, workspace.AdditionalDirs...)

	return contract.WorkspacePayload{
		ID:           workspace.ID,
		RootDir:      workspace.RootDir,
		AddDirs:      addDirs,
		Name:         workspace.Name,
		DefaultAgent: workspace.DefaultAgent,
		SandboxRef:   workspace.SandboxRef,
		CreatedAt:    workspace.CreatedAt,
		UpdatedAt:    workspace.UpdatedAt,
	}
}

// WorkspaceSkillPayloads converts workspace skill paths into response payloads.
func WorkspaceSkillPayloads(skills []workspacepkg.SkillPath) []contract.WorkspaceSkillPayload {
	payload := make([]contract.WorkspaceSkillPayload, 0, len(skills))
	for _, skill := range skills {
		payload = append(payload, contract.WorkspaceSkillPayload{
			Name:   filepath.Base(skill.Dir),
			Dir:    skill.Dir,
			Source: skill.Source,
		})
	}
	return payload
}

// SessionProviderOptionPayloadsFromConfig converts the merged workspace config
// into a stable, UI-ready list of visible provider options.
func SessionProviderOptionPayloadsFromConfig(cfg *aghconfig.Config) []contract.SessionProviderOptionPayload {
	payloadsByName := make(map[string]contract.SessionProviderOptionPayload, len(aghconfig.BuiltinProviders()))
	for name := range aghconfig.BuiltinProviders() {
		payload, ok := sessionProviderOptionPayloadFromConfig(cfg, name)
		if !ok {
			continue
		}
		payloadsByName[payload.Name] = payload
	}
	if cfg != nil {
		for name := range cfg.Providers {
			payload, ok := sessionProviderOptionPayloadFromConfig(cfg, name)
			if !ok {
				continue
			}
			payloadsByName[payload.Name] = payload
		}
	}

	defaultProvider := ""
	if cfg != nil {
		defaultProvider = aghconfig.CanonicalProviderName(cfg.Defaults.Provider)
	}
	return sortSessionProviderOptionPayloads(payloadsByName, defaultProvider)
}

func sessionProviderOptionPayloadFromConfig(
	cfg *aghconfig.Config,
	name string,
) (contract.SessionProviderOptionPayload, bool) {
	providerName := aghconfig.CanonicalProviderName(name)
	if providerName == "" {
		return contract.SessionProviderOptionPayload{}, false
	}
	var resolved aghconfig.ProviderConfig
	var err error
	if cfg == nil {
		var empty aghconfig.Config
		resolved, err = empty.ResolveProvider(providerName)
	} else {
		resolved, err = cfg.ResolveProvider(providerName)
	}
	if err != nil {
		return contract.SessionProviderOptionPayload{}, false
	}
	return contract.SessionProviderOptionPayload{
		Name:            providerName,
		DisplayName:     strings.TrimSpace(resolved.DisplayName),
		Harness:         string(resolved.EffectiveHarness()),
		RuntimeProvider: strings.TrimSpace(resolved.RuntimeProviderName(providerName)),
		AuthMode:        string(resolved.EffectiveAuthMode()),
		EnvPolicy:       string(resolved.EffectiveEnvPolicy()),
		HomePolicy:      string(resolved.EffectiveHomePolicy()),
	}, true
}

func sessionProviderOptionPayloads(names []string) []contract.SessionProviderOptionPayload {
	values := make(map[string]contract.SessionProviderOptionPayload, len(names))
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		values[trimmed] = contract.SessionProviderOptionPayload{Name: trimmed}
	}
	return sortSessionProviderOptionPayloads(values, "")
}

func sortSessionProviderOptionPayloads(
	values map[string]contract.SessionProviderOptionPayload,
	defaultProvider string,
) []contract.SessionProviderOptionPayload {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	defaultProvider = aghconfig.CanonicalProviderName(defaultProvider)
	if defaultProvider != "" {
		for i, name := range names {
			if name != defaultProvider {
				continue
			}
			copy(names[1:i+1], names[:i])
			names[0] = defaultProvider
			break
		}
	}
	payloads := make([]contract.SessionProviderOptionPayload, 0, len(names))
	for _, name := range names {
		payloads = append(payloads, values[name])
	}
	return payloads
}

// PayloadJSON coerces raw strings into valid JSON response bodies.
func PayloadJSON(raw string) json.RawMessage {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return json.RawMessage("null")
	}
	if json.Valid([]byte(trimmed)) {
		return json.RawMessage(trimmed)
	}

	encoded, err := json.Marshal(trimmed)
	if err != nil {
		return json.RawMessage("null")
	}
	return json.RawMessage(encoded)
}

func payloadJSONBytes(raw []byte) json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return json.RawMessage("null")
	}
	if json.Valid(trimmed) {
		return append(json.RawMessage(nil), trimmed...)
	}

	encoded, err := json.Marshal(string(trimmed))
	if err != nil {
		return json.RawMessage("null")
	}
	return json.RawMessage(encoded)
}

func cloneBridgeDegradation(value *bridgepkg.BridgeDegradation) *bridgepkg.BridgeDegradation {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
