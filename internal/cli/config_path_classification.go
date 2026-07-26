package cli

import (
	"fmt"

	"strings"
)

func classifyConfigMutationPath(path []string) (configSetValueKind, bool, error) {
	joined := strings.Join(path, ".")
	if kind, ok := configScalarMutationKinds[joined]; ok {
		return kind, false, nil
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
		return configSetString, false, nil
	}
	if kind, redacted, ok := classifySandboxMutationPath(path); ok {
		return kind, redacted, nil
	}
	if kind, ok := classifyWindowManagerMutationPath(path); ok {
		return kind, false, nil
	}

	return configSetString, false, fmt.Errorf("cli: config path %q is not supported by config set", joined)
}

func classifyWindowManagerMutationPath(path []string) (configSetValueKind, bool) {
	if len(path) == 2 && path[0] == configWindowManagerKey {
		switch path[1] {
		case "new_window_policy",
			"small_viewport_policy",
			"focus_policy",
			"drag_away_policy",
			"group_move_modifier",
			"swap_modifier",
			"desktop_transition":
			return configSetString, true
		case "focus_wrap", "focus_follows_pointer", "raise_on_focus":
			return configSetBool, true
		case "history_limit":
			return configSetInt, true
		}
	}
	if len(path) != 3 || path[0] != configWindowManagerKey {
		return configSetString, false
	}
	switch path[1] {
	case "gaps":
		switch path[2] {
		case "inner", "top", "right", "bottom", configWindowManagerGapLeft:
			return configSetInt, true
		}
	case "snap":
		switch path[2] {
		case "edge_band", "corner_reach", "exit_slack":
			return configSetInt, true
		case "repeat_ratios":
			return configSetFloatSlice, true
		}
	case "bindings":
		switch path[2] {
		case "top_center", "bottom_center":
			return configSetString, true
		}
	case "shortcuts":
		return configSetString, true
	}
	return configSetString, false
}

const (
	configPathSandboxes        = "sandboxes"
	configWindowManagerKey     = "window_manager"
	configWindowManagerGapLeft = "left"
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
			"auth_login_command":
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
		case configCommandKey, "endpoint", "timeout":
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
