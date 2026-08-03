package extensionpkg

import (
	"context"

	"errors"
	"fmt"

	"slices"
	"strings"

	"github.com/compozy/compozy/internal/diagnostics"

	"github.com/compozy/compozy/internal/vault"
)

func (m *Manager) resolveCommand(rootDir string, value string) (string, error) {
	return resolveManifestCommand(rootDir, value, m.getenv, m.compozyExecutable)
}

func (m *Manager) resolveStringSlice(rootDir string, values []string) ([]string, error) {
	return resolveManifestStringSlice(rootDir, values, m.getenv, m.compozyExecutable)
}

func (m *Manager) resolveStringMap(rootDir string, env map[string]string) (map[string]string, error) {
	return resolveManifestStringMap(rootDir, env, m.getenv, m.compozyExecutable)
}

func (m *Manager) resolveEnvMap(
	ctx context.Context,
	rootDir string,
	env map[string]string,
	secretEnv map[string]string,
) ([]string, []func(), error) {
	return m.resolveInstanceEnvMap(ctx, InstanceKey{}, nil, rootDir, env, secretEnv)
}

func (m *Manager) resolveInstanceEnvMap(
	ctx context.Context,
	key InstanceKey,
	requiresEnv []string,
	rootDir string,
	env map[string]string,
	secretEnv map[string]string,
) ([]string, []func(), error) {
	resolvedMap, err := m.resolveStringMap(rootDir, env)
	if err != nil {
		return nil, nil, err
	}
	secretMap, cleanups, err := m.resolveSecretEnvMap(ctx, secretEnv)
	if err != nil {
		return nil, nil, err
	}
	bindingMap, bindingCleanups, err := m.resolveBoundEnvMap(ctx, key, requiresEnv)
	if err != nil {
		runExtensionRedactionCleanups(cleanups)
		return nil, nil, err
	}
	cleanups = append(cleanups, bindingCleanups...)

	valuesMap := make(map[string]string, len(safeSubprocessEnvKeys)+len(resolvedMap)+len(secretMap)+len(bindingMap))
	order := make([]string, 0, len(safeSubprocessEnvKeys)+len(resolvedMap)+len(secretMap)+len(bindingMap))
	for _, key := range safeSubprocessEnvKeys {
		if _, exists := valuesMap[key]; exists {
			continue
		}
		valuesMap[key] = m.getenv(key)
		order = append(order, key)
	}

	keys := make([]string, 0, len(resolvedMap))
	for key := range resolvedMap {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	for _, key := range keys {
		if _, exists := valuesMap[key]; !exists {
			order = append(order, key)
		}
		valuesMap[key] = resolvedMap[key]
	}
	secretKeys := make([]string, 0, len(secretMap))
	for key := range secretMap {
		secretKeys = append(secretKeys, key)
	}
	slices.Sort(secretKeys)
	for _, key := range secretKeys {
		if _, exists := valuesMap[key]; !exists {
			order = append(order, key)
		}
		valuesMap[key] = secretMap[key]
	}
	bindingKeys := make([]string, 0, len(bindingMap))
	for key := range bindingMap {
		bindingKeys = append(bindingKeys, key)
	}
	slices.Sort(bindingKeys)
	for _, key := range bindingKeys {
		if _, exists := valuesMap[key]; !exists {
			order = append(order, key)
		}
		valuesMap[key] = bindingMap[key]
	}

	values := make([]string, 0, len(order))
	for _, key := range order {
		values = append(values, key+"="+valuesMap[key])
	}
	return values, cleanups, nil
}

func (m *Manager) resolveBoundEnvMap(
	ctx context.Context,
	key InstanceKey,
	requiresEnv []string,
) (map[string]string, []func(), error) {
	key = key.Normalize()
	if m.envBindings == nil || key.Name == "" || len(requiresEnv) == 0 {
		return nil, nil, nil
	}
	if ctx == nil {
		return nil, nil, errors.New("extension: bound env context is required")
	}
	bindings, err := m.envBindings.ListEnvBindings(ctx, key.Name, key.WorkspaceID)
	if err != nil {
		return nil, nil, fmt.Errorf("extension: list subprocess env bindings for %q: %w", key.runtimeID(), err)
	}
	declared := make(map[string]struct{}, len(requiresEnv))
	for _, name := range normalizeStrings(requiresEnv) {
		declared[name] = struct{}{}
	}
	values := make(map[string]string, len(bindings))
	cleanups := make([]func(), 0, len(bindings))
	slices.SortFunc(bindings, func(left, right EnvBinding) int {
		return strings.Compare(left.EnvName, right.EnvName)
	})
	for _, binding := range bindings {
		name := strings.TrimSpace(binding.EnvName)
		if _, ok := declared[name]; !ok {
			continue
		}
		ref := vault.NormalizeRef(binding.SecretRef)
		value, err := m.resolveSecretRef(ctx, ref)
		if err != nil {
			runExtensionRedactionCleanups(cleanups)
			return nil, nil, &boundEnvResolutionError{envName: name, cause: err}
		}
		values[name] = value
		cleanups = append(cleanups, diagnostics.RegisterDynamicSecret(value))
	}
	return values, cleanups, nil
}

type boundEnvResolutionError struct {
	envName string
	cause   error
}

func (e *boundEnvResolutionError) Error() string {
	return fmt.Sprintf("extension: resolve subprocess env binding %s", strings.TrimSpace(e.envName))
}

func (e *boundEnvResolutionError) Unwrap() error {
	return e.cause
}

func (m *Manager) resolveSecretEnvMap(
	ctx context.Context,
	secretEnv map[string]string,
) (map[string]string, []func(), error) {
	if len(secretEnv) == 0 {
		return nil, nil, nil
	}
	if ctx == nil {
		return nil, nil, errors.New("extension: secret env context is required")
	}
	values := make(map[string]string, len(secretEnv))
	cleanups := []func(){}
	keys := make([]string, 0, len(secretEnv))
	for key := range secretEnv {
		keys = append(keys, strings.TrimSpace(key))
	}
	slices.Sort(keys)
	for _, key := range keys {
		ref := vault.NormalizeRef(secretEnv[key])
		value, err := m.resolveSecretRef(ctx, ref)
		if err != nil {
			runExtensionRedactionCleanups(cleanups)
			return nil, nil, fmt.Errorf("extension: resolve subprocess secret_env.%s: %w", key, err)
		}
		values[key] = value
		cleanups = append(cleanups, diagnostics.RegisterDynamicSecret(value))
	}
	return values, cleanups, nil
}

func (m *Manager) resolveSecretRef(ctx context.Context, ref string) (string, error) {
	if m.secretResolver != nil {
		return m.secretResolver.ResolveRef(ctx, ref)
	}
	envName, err := vault.EnvNameFromRef(ref)
	if err != nil {
		return "", err
	}
	value := getenvValue(m.getenv, envName)
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%w: env:%s", vault.ErrMissingSecret, envName)
	}
	return value, nil
}

func runExtensionRedactionCleanups(cleanups []func()) {
	for _, cleanup := range slices.Backward(cleanups) {
		if cleanup != nil {
			cleanup()
		}
	}
}

func (m *Manager) resolveString(rootDir string, value string) (string, error) {
	return resolveManifestString(rootDir, value, m.getenv, m.compozyExecutable)
}
