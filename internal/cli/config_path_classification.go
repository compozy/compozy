package cli

import (
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

const (
	providerAuthLoginCommandKey = "auth_login_command"
	cliTimeoutKey               = "timeout"
)

func classifyConfigMutationPath(path []string) (configSetValueKind, bool, error) {
	joined := strings.Join(path, ".")
	if replacement, removed := removedExtensionConfigSetReplacement(joined); removed {
		return configSetString, false, fmt.Errorf("cli: config path %q was removed; use %q", joined, replacement)
	}
	if kind, ok := configScalarMutationKinds[joined]; ok {
		return kind, false, nil
	}
	if len(path) == 4 && path[0] == compozyconfig.LoopsConfigKey && path[1] == compozyconfig.LoopInputsConfigKey {
		return configSetLoopInput, false, nil
	}
	if len(path) == 3 && path[0] == configProvidersKey && path[2] == configSessionMCPKey {
		return configSetBool, false, nil
	}
	if len(path) == 5 &&
		path[0] == configProvidersKey &&
		path[2] == configModelsKey &&
		path[3] == configDiscoveryKey &&
		path[4] == configEnabledKey {
		return configSetBool, false, nil
	}
	if isProviderMutationPath(path) {
		if path[2] == configCommandKey {
			return configSetStringOrStringSlice, false, nil
		}
		return configSetString, isProviderLoginCommandPath(path), nil
	}
	if kind, redacted, ok := classifySandboxMutationPath(path); ok {
		return kind, redacted, nil
	}
	if kind, ok := classifyWindowManagerMutationPath(path); ok {
		return kind, false, nil
	}
	if kind, redacted, ok := classifyAgentMutableConfigPath(path); ok {
		return kind, redacted, nil
	}

	return configSetString, false, fmt.Errorf("cli: config path %q is not supported by config set", joined)
}

func classifyAgentMutableConfigPath(path []string) (configSetValueKind, bool, bool) {
	policy, err := compozyconfig.ClassifyToolConfigPath(path)
	if err != nil || policy.Denial != compozyconfig.ConfigPathAllowed {
		return configSetString, false, false
	}

	var kind configSetValueKind
	switch policy.Kind {
	case compozyconfig.ConfigValueString:
		kind = configSetString
	case compozyconfig.ConfigValueBool:
		kind = configSetBool
	case compozyconfig.ConfigValueInt:
		kind = configSetInt
	case compozyconfig.ConfigValueInt64:
		kind = configSetInt64
	case compozyconfig.ConfigValueUint64:
		kind = configSetUint64
	case compozyconfig.ConfigValueFloat:
		kind = configSetFloat
	case compozyconfig.ConfigValueDuration:
		kind = configSetDuration
	case compozyconfig.ConfigValueStringSlice:
		kind = configSetStringSlice
	case compozyconfig.ConfigValueTable:
		kind = configSetTable
	case compozyconfig.ConfigValueScalar:
		kind = configSetScalar
	case compozyconfig.ConfigValueLoopInput:
		kind = configSetLoopInput
	default:
		return configSetString, false, false
	}
	return kind, policy.Redacted, true
}

func isProviderLoginCommandPath(path []string) bool {
	return len(path) == 3 && path[0] == configProvidersKey && path[2] == providerAuthLoginCommandKey
}

func classifyWindowManagerMutationPath(path []string) (configSetValueKind, bool) {
	spec, ok := lookupWindowManagerMutationSpec(path)
	if !ok {
		return configSetString, false
	}
	return spec.kind, true
}

const (
	configPathSandboxes                   = "sandboxes"
	configWindowManagerKey                = "window_manager"
	configWindowManagerShortcutsKey       = "shortcuts"
	configWindowManagerGlobalShortcutsKey = "global_shortcuts"
)

func isProviderMutationPath(path []string) bool {
	if len(path) == 3 && path[0] == configProvidersKey {
		switch path[2] {
		case configCommandKey,
			providerAuthModeKey,
			"env_policy",
			"home_policy",
			"runtime_provider",
			"transport",
			"base_url",
			"auth_status_command",
			providerAuthLoginCommandKey:
			return true
		}
	}
	if len(path) == 4 && path[0] == configProvidersKey && path[2] == configModelsKey {
		if path[3] == configDefaultKey {
			return true
		}
	}
	if len(path) == 5 &&
		path[0] == configProvidersKey &&
		path[2] == configModelsKey &&
		path[3] == configDiscoveryKey {
		switch path[4] {
		case configCommandKey, "endpoint", cliTimeoutKey:
			return true
		}
	}
	return false
}

func classifySandboxMutationPath(path []string) (configSetValueKind, bool, bool) {
	if len(path) == 4 && path[0] == configPathSandboxes {
		switch path[2] {
		case configEnvKey, configSecretEnvKey:
			return configSetString, true, true
		case configNetworkKey:
			return classifySandboxNetworkMutationPath(path[3])
		case "daytona":
			return classifySandboxDaytonaMutationPath(path[3])
		}
	}
	if len(path) == 3 && path[0] == configPathSandboxes {
		switch path[2] {
		case configBackendKey, "sync_mode", "persistence", "runtime_root":
			return configSetString, false, true
		}
	}
	return configSetString, false, false
}

func classifySandboxNetworkMutationPath(name string) (configSetValueKind, bool, bool) {
	switch name {
	case "allow_public_ingress", "allow_outbound", configRequiredKey:
		return configSetBool, false, true
	case "allow_list", "deny_list":
		return configSetStringSlice, false, true
	default:
		return configSetString, false, false
	}
}

func classifySandboxDaytonaMutationPath(name string) (configSetValueKind, bool, bool) {
	switch name {
	case "api_url", configTargetKey, "image", cliSnapshotKey, "class", "auto_stop", "auto_archive":
		return configSetString, false, true
	default:
		return configSetString, false, false
	}
}
