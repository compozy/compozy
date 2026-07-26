package extensionpkg

import (
	"fmt"
	"os"

	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"

	"github.com/compozy/agh/internal/vault"
)

func (d *manifestDocument) toManifest() (Manifest, error) {
	name, err := mergeManifestValue(manifestNameKey, d.Name, d.Extension.Name)
	if err != nil {
		return Manifest{}, err
	}
	versionValue, err := mergeManifestValue("version", d.Version, d.Extension.Version)
	if err != nil {
		return Manifest{}, err
	}
	description, err := mergeManifestValue("description", d.Description, d.Extension.Description)
	if err != nil {
		return Manifest{}, err
	}
	minVersion, err := mergeManifestValue("min_agh_version", d.MinAGHVersion, d.Extension.MinAGHVersion)
	if err != nil {
		return Manifest{}, err
	}
	requiresEnv, err := mergeManifestStringSlice("requires_env", d.RequiresEnv, d.Extension.RequiresEnv)
	if err != nil {
		return Manifest{}, err
	}

	manifest := Manifest{
		Name:                 name,
		Version:              versionValue,
		Description:          description,
		MinAGHVersion:        minVersion,
		RequiresEnv:          normalizeStrings(requiresEnv),
		NetworkParticipation: d.NetworkParticipation.Normalize(),
		Resources:            normalizeResourcesConfig(d.Resources),
		Capabilities:         normalizeCapabilitiesConfig(d.Capabilities),
		Actions:              normalizeActionsConfig(d.Actions),
		Subprocess:           normalizeSubprocessConfig(d.Subprocess),
		Security:             normalizeSecurityConfig(d.Security),
		Bridge:               normalizeBridgeConfig(d.Bridge),
	}
	return manifest, nil
}

func mergeManifestValue(field, rootValue, wrappedValue string) (string, error) {
	rootValue = strings.TrimSpace(rootValue)
	wrappedValue = strings.TrimSpace(wrappedValue)
	switch {
	case rootValue == "":
		return wrappedValue, nil
	case wrappedValue == "":
		return rootValue, nil
	case rootValue == wrappedValue:
		return rootValue, nil
	default:
		return "", &ManifestValidationError{
			Field:   field,
			Message: "conflicting root and extension values",
		}
	}
}

func mergeManifestStringSlice(field string, rootValues []string, wrappedValues []string) ([]string, error) {
	normalizedRoot := normalizeStrings(rootValues)
	normalizedWrapped := normalizeStrings(wrappedValues)
	switch {
	case len(normalizedRoot) == 0:
		return normalizedWrapped, nil
	case len(normalizedWrapped) == 0:
		return normalizedRoot, nil
	case stringSlicesEqual(normalizedRoot, normalizedWrapped):
		return normalizedRoot, nil
	default:
		return nil, &ManifestValidationError{
			Field:   field,
			Message: "conflicting root and extension values",
		}
	}
}

func stringSlicesEqual(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for idx := range left {
		if left[idx] != right[idx] {
			return false
		}
	}
	return true
}

// MissingEnv returns manifest-required environment variable names that are unset or empty.
func (m *Manifest) MissingEnv(getenv func(string) string) []string {
	if m == nil || len(m.RequiresEnv) == 0 {
		return nil
	}
	if getenv == nil {
		getenv = os.Getenv
	}
	missing := make([]string, 0, len(m.RequiresEnv))
	for _, name := range m.RequiresEnv {
		if strings.TrimSpace(getenv(name)) == "" {
			missing = append(missing, name)
		}
	}
	return missing
}

func normalizeResourcesConfig(cfg ResourcesConfig) ResourcesConfig {
	return ResourcesConfig{
		Skills:     normalizeStrings(cfg.Skills),
		Loops:      normalizeStrings(cfg.Loops),
		Agents:     normalizeStrings(cfg.Agents),
		Bundles:    normalizeStrings(cfg.Bundles),
		Hooks:      normalizeHooks(cfg.Hooks),
		Tools:      normalizeTools(cfg.Tools),
		MCPServers: normalizeMCPServers(cfg.MCPServers),
		Publish:    normalizeResourceGrantRequest(cfg.Publish),
	}
}

func normalizeResourceGrantRequest(cfg ResourceGrantRequest) ResourceGrantRequest {
	return ResourceGrantRequest{
		Families: normalizeStrings(cfg.Families),
		MaxScope: cfg.MaxScope.Normalize(),
	}
}

func normalizeCapabilitiesConfig(cfg CapabilitiesConfig) CapabilitiesConfig {
	return CapabilitiesConfig{
		Provides: normalizeStrings(cfg.Provides),
	}
}

func normalizeActionsConfig(cfg ActionsConfig) ActionsConfig {
	return ActionsConfig{
		Requires: normalizeStrings(cfg.Requires),
	}
}

func normalizeSubprocessConfig(cfg SubprocessConfig) SubprocessConfig {
	return SubprocessConfig{
		Command:             strings.TrimSpace(cfg.Command),
		Args:                normalizeStrings(cfg.Args),
		Env:                 normalizeStringMap(cfg.Env),
		SecretEnv:           normalizeStringMap(cfg.SecretEnv),
		HealthCheckInterval: cfg.HealthCheckInterval,
		ShutdownTimeout:     cfg.ShutdownTimeout,
	}
}

func normalizeSecurityConfig(cfg SecurityConfig) SecurityConfig {
	return SecurityConfig{
		Capabilities: normalizeStrings(cfg.Capabilities),
	}
}

func normalizeBridgeConfig(cfg BridgeConfig) BridgeConfig {
	var configSchema *bridgepkg.BridgeProviderConfigSchema
	if cfg.ConfigSchema != nil {
		normalized := cfg.ConfigSchema.Normalize()
		if !normalized.IsZero() {
			configSchema = &normalized
		}
	}

	return BridgeConfig{
		Platform:     strings.TrimSpace(cfg.Platform),
		DisplayName:  strings.TrimSpace(cfg.DisplayName),
		SecretSlots:  normalizeBridgeSecretSlots(cfg.SecretSlots),
		ConfigSchema: configSchema,
	}
}

func normalizeBridgeSecretSlots(src []bridgepkg.BridgeSecretSlot) []bridgepkg.BridgeSecretSlot {
	if len(src) == 0 {
		return nil
	}

	dst := make([]bridgepkg.BridgeSecretSlot, 0, len(src))
	for _, slot := range src {
		dst = append(dst, slot.Normalize())
	}
	return dst
}

func validateBridgeSecretSlots(slots []bridgepkg.BridgeSecretSlot) error {
	seen := make(map[string]struct{}, len(slots))
	for idx, slot := range slots {
		normalized := slot.Normalize()
		if err := normalized.Validate(); err != nil {
			return &ManifestValidationError{
				Field:   fmt.Sprintf("bridge.secret_slots[%d]", idx),
				Message: err.Error(),
			}
		}
		key := normalized.Name
		if _, ok := seen[key]; ok {
			return &ManifestValidationError{
				Field:   fmt.Sprintf("bridge.secret_slots[%d].name", idx),
				Value:   key,
				Message: "duplicate secret slot name",
			}
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validateEnvRequirements(field string, values []string) error {
	seen := make(map[string]struct{}, len(values))
	for idx, value := range values {
		name := strings.TrimSpace(value)
		if !validEnvRequirementName(name) {
			return &ManifestValidationError{
				Field:   fmt.Sprintf("%s[%d]", field, idx),
				Value:   value,
				Message: "environment variable name must match [A-Za-z_][A-Za-z0-9_]*",
			}
		}
		if _, ok := seen[name]; ok {
			return &ManifestValidationError{
				Field:   fmt.Sprintf("%s[%d]", field, idx),
				Value:   name,
				Message: "duplicate environment variable requirement",
			}
		}
		seen[name] = struct{}{}
	}
	return nil
}

func validateManifestEnvMaps(
	field string,
	namespace string,
	env map[string]string,
	secretEnv map[string]string,
) error {
	if err := vault.ValidateNonSecretEnvMap(field, env); err != nil {
		return &ManifestValidationError{Field: field, Message: err.Error()}
	}
	if err := vault.ValidateSecretEnvMap(field, namespace, secretEnv); err != nil {
		return &ManifestValidationError{Field: field, Message: err.Error()}
	}
	return nil
}

func validateManifestHookEnv(hooks []HookConfig) error {
	for idx := range hooks {
		hook := &hooks[idx]
		field := fmt.Sprintf("resources.hooks[%d]", idx)
		rootSpecified := strings.TrimSpace(hook.Command) != "" || len(hook.Args) > 0 ||
			len(hook.Env) > 0 || len(hook.SecretEnv) > 0
		nestedSpecified := strings.TrimSpace(hook.Executor.Command) != "" || len(hook.Executor.Args) > 0 ||
			len(hook.Executor.Env) > 0 || len(hook.Executor.SecretEnv) > 0
		if rootSpecified && nestedSpecified {
			return &ManifestValidationError{
				Field:   field,
				Message: "hook executor fields must be declared either at the top level or under executor, not both",
			}
		}
		env := hook.Env
		secretEnv := hook.SecretEnv
		if nestedSpecified {
			env = hook.Executor.Env
			secretEnv = hook.Executor.SecretEnv
		}
		if err := validateManifestEnvMaps(field, "hooks", env, secretEnv); err != nil {
			return err
		}
	}
	return nil
}

func validateManifestMCPServerEnv(servers map[string]MCPServerConfig) error {
	for _, name := range sortedMapKeys(servers) {
		field := "resources.mcp_servers." + strings.TrimSpace(name)
		server := servers[name]
		if err := validateManifestEnvMaps(field, "mcp", server.Env, server.SecretEnv); err != nil {
			return err
		}
	}
	return nil
}
