package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	bootstrapDefaultsKey = "defaults"
)

const (
	toolSurfaceModelsKey = "models"
)

const (
	bootstrapDefaultKey     = "default"
	bootstrapPermissionsKey = "permissions"
	bootstrapProviderKey    = "provider"
)

// LoadGlobalConfig loads only the user-global AGH config from the resolved home.
func LoadGlobalConfig(homePaths HomePaths) (Config, error) {
	return loadWithHome(homePaths, "", false, processEnvLookup)
}

// ResolveAgentName resolves an explicit session agent name or falls back to config defaults.
func ResolveAgentName(name string, defaults DefaultsConfig) (string, error) {
	if resolved := strings.TrimSpace(name); resolved != "" {
		return resolved, nil
	}
	if resolved := strings.TrimSpace(defaults.Agent); resolved != "" {
		return resolved, nil
	}
	return "", errors.New("agent name is required; run `agh install` or set defaults.agent")
}

// SaveBootstrapConfig writes the global bootstrap config managed by `agh install`.
func SaveBootstrapConfig(homePaths HomePaths, provider string, model string) (Config, error) {
	selectedProvider := CanonicalProviderName(provider)
	if selectedProvider == "" {
		return Config{}, errors.New("bootstrap provider is required")
	}
	selectedModel := strings.TrimSpace(model)
	if err := EnsureHomeLayout(homePaths); err != nil {
		return Config{}, err
	}

	current, err := LoadGlobalConfig(homePaths)
	if err != nil {
		return Config{}, err
	}
	resolvedProvider, err := current.ResolveProvider(selectedProvider)
	if err != nil {
		return Config{}, fmt.Errorf("resolve bootstrap provider %q: %w", selectedProvider, err)
	}
	if selectedModel == "" && resolvedProvider.RequiresRuntimeModel() {
		return Config{}, fmt.Errorf("bootstrap model is required for provider %q", selectedProvider)
	}

	target, err := ResolveConfigWriteTarget(homePaths, "", WriteScopeGlobal)
	if err != nil {
		return Config{}, err
	}

	return EditConfigOverlay(homePaths, "", target, func(editor *OverlayEditor) error {
		if err := editor.SetValue(
			[]string{bootstrapDefaultsKey, string(AgentResourceKind)},
			DefaultAgentName,
		); err != nil {
			return err
		}
		if err := editor.SetValue([]string{bootstrapDefaultsKey, bootstrapProviderKey}, selectedProvider); err != nil {
			return err
		}
		if err := editor.SetValue(
			[]string{bootstrapPermissionsKey, "mode"},
			string(PermissionModeApproveAll),
		); err != nil {
			return err
		}
		if selectedModel == "" {
			return nil
		}
		return editor.SetValue(
			[]string{providersConfigKey, selectedProvider, toolSurfaceModelsKey, bootstrapDefaultKey},
			selectedModel,
		)
	})
}

// EnsureBootstrapAgent creates the managed default agent definition if it does not already exist.
func EnsureBootstrapAgent(homePaths HomePaths) (string, bool, error) {
	return ensureManagedAgent(homePaths, DefaultAgentName, bootstrapAgentContents())
}

func ensureManagedAgent(homePaths HomePaths, agentName string, contents string) (string, bool, error) {
	if err := EnsureHomeLayout(homePaths); err != nil {
		return "", false, err
	}

	path := filepath.Join(homePaths.AgentsDir, agentName, agentDefName)
	agentDir := filepath.Dir(path)
	if err := os.MkdirAll(agentDir, privateDirMode); err != nil {
		return "", false, fmt.Errorf("create managed agent directory %q: %w", filepath.Dir(path), err)
	}
	if err := os.Chmod(agentDir, privateDirMode); err != nil {
		return "", false, fmt.Errorf("secure managed agent directory %q: %w", agentDir, err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, privateFileMode)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return path, false, nil
		}
		return "", false, fmt.Errorf("create managed agent file %q: %w", path, err)
	}
	if _, err := file.WriteString(contents); err != nil {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		return "", false, fmt.Errorf("write managed agent file %q: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return "", false, fmt.Errorf("close managed agent file %q: %w", path, err)
	}
	return path, true, nil
}

func bootstrapAgentContents() string {
	return strings.Join([]string{
		"---",
		"name: " + DefaultAgentName,
		"---",
		"",
		"You are AGH's default general-purpose agent.",
		"",
		"Operate autonomously, complete tasks end-to-end, and follow the active workspace instructions.",
		"Provider and model are resolved from the user's AGH configuration unless this agent is overridden.",
	}, "\n")
}
