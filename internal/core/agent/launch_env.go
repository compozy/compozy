package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
)

const (
	openCodeConfigDirEnv           = "OPENCODE_CONFIG_DIR"
	orcaOpenCodeConfigDirEnv       = "ORCA_OPENCODE_CONFIG_DIR"
	orcaOpenCodeSourceConfigDirEnv = "ORCA_OPENCODE_SOURCE_CONFIG_DIR"
)

func buildLaunchEnvironment(spec Spec, extraEnv map[string]string) []string {
	env := currentEnvironmentMap()
	sanitizeInheritedLaunchEnvironment(spec, env)
	overlayEnvironment(env, spec.EnvVars)
	overlayEnvironment(env, extraEnv)
	return environmentAssignments(env)
}

func currentEnvironmentMap() map[string]string {
	assignments := os.Environ()
	env := make(map[string]string, len(assignments))
	for _, assignment := range assignments {
		key, value, ok := strings.Cut(assignment, "=")
		if !ok || strings.TrimSpace(key) == "" {
			continue
		}
		env[key] = value
	}
	return env
}

func sanitizeInheritedLaunchEnvironment(spec Spec, env map[string]string) {
	if env == nil || !isOpenCodeSpec(spec) {
		return
	}

	currentConfigDir := strings.TrimSpace(env[openCodeConfigDirEnv])
	overlayConfigDir := strings.TrimSpace(env[orcaOpenCodeConfigDirEnv])
	sourceDir := strings.TrimSpace(env[orcaOpenCodeSourceConfigDirEnv])
	currentLooksOrcaManaged := currentConfigDir == "" || currentConfigDir == overlayConfigDir ||
		isOrcaOpenCodeConfigDir(currentConfigDir)

	switch {
	case sourceDir != "" && currentLooksOrcaManaged:
		env[openCodeConfigDirEnv] = sourceDir
	case overlayConfigDir != "" && currentLooksOrcaManaged:
		delete(env, openCodeConfigDirEnv)
	case overlayConfigDir == "" && sourceDir == "" && isOrcaOpenCodeConfigDir(currentConfigDir):
		delete(env, openCodeConfigDirEnv)
	}

	delete(env, orcaOpenCodeConfigDirEnv)
	delete(env, orcaOpenCodeSourceConfigDirEnv)
}

func isOpenCodeSpec(spec Spec) bool {
	if spec.ID == model.IDEOpenCode {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(spec.SetupAgentName), "opencode")
}

func overlayEnvironment(env map[string]string, overrides map[string]string) {
	if env == nil || len(overrides) == 0 {
		return
	}
	for key, value := range overrides {
		if strings.TrimSpace(key) == "" {
			continue
		}
		env[key] = value
	}
}

func environmentAssignments(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	assignments := make([]string, 0, len(keys))
	for _, key := range keys {
		assignments = append(assignments, fmt.Sprintf("%s=%s", key, env[key]))
	}
	return assignments
}

func isOrcaOpenCodeConfigDir(path string) bool {
	clean := strings.TrimSpace(path)
	if clean == "" {
		return false
	}

	segments := strings.Split(filepath.ToSlash(filepath.Clean(clean)), "/")
	for i := 0; i+1 < len(segments); i++ {
		if segments[i] != "orca" {
			continue
		}
		if segments[i+1] == "opencode-hooks" || segments[i+1] == "opencode-config-overlays" {
			return true
		}
	}
	return false
}
