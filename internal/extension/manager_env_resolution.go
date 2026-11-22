package extensionpkg

import (
	"context"
	"maps"

	"errors"
	"fmt"

	"slices"
	"strings"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/extensioninput"
	"github.com/compozy/compozy/internal/marketplace"
	"github.com/compozy/compozy/internal/procutil"
	"github.com/compozy/compozy/internal/store"

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
	return m.resolveInstanceEnvMap(ctx, InstanceKey{}, nil, nil, rootDir, env, secretEnv)
}

func (m *Manager) resolveInstanceEnvMap(
	ctx context.Context,
	key InstanceKey,
	requiresEnv []string,
	subprocessInputs []launchInput,
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
	bindingMap, bindingCleanups, err := m.resolveBoundEnvMap(ctx, key, requiresEnv, subprocessInputs)
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
	// CA trust paths pass only when set: an empty SSL_CERT_FILE would make OpenSSL clients trust nothing.
	for _, key := range procutil.CATrustEnvKeys() {
		if value := m.getenv(key); value != "" {
			if _, exists := valuesMap[key]; !exists {
				order = append(order, key)
			}
			valuesMap[key] = value
		}
	}

	keys := slices.Sorted(maps.Keys(resolvedMap))

	for _, key := range keys {
		if _, exists := valuesMap[key]; !exists {
			order = append(order, key)
		}
		valuesMap[key] = resolvedMap[key]
	}
	secretKeys := slices.Sorted(maps.Keys(secretMap))
	for _, key := range secretKeys {
		if _, exists := valuesMap[key]; !exists {
			order = append(order, key)
		}
		valuesMap[key] = secretMap[key]
	}
	bindingKeys := slices.Sorted(maps.Keys(bindingMap))
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

// launchInput is one input delivered to the subprocess. A missing required server-consumed input stays
// ready only through a non-empty daemon environment value (rule 5a) or its active typed row.
type launchInput struct {
	ManifestInput
	serverConsumed bool
}

// launchInputs lists the subprocess inputs with their MCP consumption.
func (m *Manifest) launchInputs() []launchInput {
	inputs := m.SubprocessInputs()
	result := make([]launchInput, 0, len(inputs))
	for _, input := range inputs {
		result = append(result, launchInput{ManifestInput: input, serverConsumed: m.ServerInput(input)})
	}
	return result
}

// resolveBoundEnvMap delivers instance bindings for requires_env names. A subprocess-bound input's
// binding is re-checked against its manifest type and omitted when invalid; an unbound input falls back
// to its default. The daemon environment is never copied.
func (m *Manager) resolveBoundEnvMap(
	ctx context.Context,
	key InstanceKey,
	requiresEnv []string,
	subprocessInputs []launchInput,
) (map[string]string, []func(), error) {
	key = key.Normalize()
	if key.Name == "" || len(requiresEnv) == 0 {
		return nil, nil, nil
	}
	bindings, err := m.instanceEnvBindings(ctx, key)
	if err != nil {
		return nil, nil, err
	}
	declared := make(map[string]struct{}, len(requiresEnv))
	for _, name := range normalizeStrings(requiresEnv) {
		declared[name] = struct{}{}
	}
	inputTypes := make(map[string]string, len(subprocessInputs))
	for _, input := range subprocessInputs {
		inputTypes[strings.TrimSpace(input.Binding.Name)] = input.Type
	}
	values := make(map[string]string, len(bindings))
	invalid := make(map[string]bool)
	cleanups := make([]func(), 0, len(bindings))
	slices.SortFunc(bindings, func(left, right EnvBinding) int {
		return strings.Compare(left.EnvName, right.EnvName)
	})
	for _, binding := range bindings {
		if strings.TrimSpace(binding.MCPServer) != "" {
			continue
		}
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
		cleanups = append(cleanups, diagnostics.RegisterDynamicSecret(value))
		if inputType, bound := inputTypes[name]; bound && inputType != manifestInputTypeSecret {
			normalized, err := marketplace.NormalizeMCPInputValue(inputType, value)
			if err != nil {
				invalid[name] = true // never injected, and never replaced by the default (rule 4)
				continue
			}
			value = normalized
		}
		values[name] = value
	}
	satisfied, err := m.serverInputsSatisfiedElsewhere(ctx, key, values, invalid, subprocessInputs)
	if err == nil {
		err = applyLaunchInputDefaults(values, invalid, satisfied, subprocessInputs)
	}
	if err != nil {
		runExtensionRedactionCleanups(cleanups)
		return nil, nil, err
	}
	return values, cleanups, nil
}

// InputRowReader lists the typed input rows of one extension instance.
type InputRowReader interface {
	List(context.Context, extensioninput.Instance) (map[string]extensioninput.Record, error)
}

// serverInputsSatisfiedElsewhere names the unbound, valid, required server-consumed inputs without a
// default that stay ready without a binding: a non-empty daemon environment value (daemon_env, rule 5a)
// or an active typed row, either valid for the declared type. Neither is delivered to the subprocess.
func (m *Manager) serverInputsSatisfiedElsewhere(
	ctx context.Context, key InstanceKey, values map[string]string, invalid map[string]bool, inputs []launchInput,
) (map[string]bool, error) {
	satisfied := map[string]bool{}
	var rows map[string]extensioninput.Record
	rowsLoaded := false
	for _, input := range inputs {
		name := strings.TrimSpace(input.Binding.Name)
		if _, bound := values[name]; bound || invalid[name] || !input.Required || !input.serverConsumed ||
			len(input.Default) > 0 {
			continue
		}
		if raw := m.getenv(name); strings.TrimSpace(raw) != "" {
			_, err := marketplace.NormalizeMCPInputValue(input.Type, raw)
			satisfied[name] = err == nil // an invalid daemon value reads invalid, never daemon_env
			continue
		}
		if !rowsLoaded {
			var err error
			if rows, err = m.instanceInputRows(ctx, key); err != nil {
				return nil, err
			}
			rowsLoaded = true
		}
		if row, found := rows[input.ID]; found && row.Active && row.Type == input.Type {
			_, err := typedInputValue(input.Type, row.Value)
			satisfied[name] = err == nil
		}
	}
	return satisfied, nil
}

func (m *Manager) instanceInputRows(ctx context.Context, key InstanceKey) (map[string]extensioninput.Record, error) {
	if m.inputRows == nil {
		return nil, nil
	}
	profileID := key.ProfileID
	if profileID == "" {
		profileID = store.DefaultProfileID
	}
	rows, err := m.inputRows.List(ctx, extensioninput.Instance{
		Extension: key.Name, ProfileID: profileID, WorkspaceID: key.WorkspaceID,
	})
	if err != nil {
		return nil, fmt.Errorf("extension: list input rows for %q: %w", key.runtimeID(), err)
	}
	return rows, nil
}

func (m *Manager) instanceEnvBindings(ctx context.Context, key InstanceKey) ([]EnvBinding, error) {
	if m.envBindings == nil {
		return nil, nil
	}
	if ctx == nil {
		return nil, errors.New("extension: bound env context is required")
	}
	profileID := key.ProfileID
	if profileID == "" {
		profileID = store.DefaultProfileID
	}
	bindings, err := m.envBindings.ResolveEnvBindings(ctx, key.Name, profileID, key.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("extension: resolve subprocess env bindings for %q: %w", key.runtimeID(), err)
	}
	return bindings, nil
}

// applyLaunchInputDefaults fills unbound inputs from their defaults and refuses start when a required
// input stays missing or invalid (rules 4-5). An invalid binding is never replaced by the default; a
// satisfied name is ready without a delivered value.
func applyLaunchInputDefaults(
	values map[string]string, invalid, satisfied map[string]bool, inputs []launchInput,
) error {
	readiness := Readiness{MissingInputs: []string{}, MissingEnv: []string{}}
	manifest := &Manifest{}
	for _, input := range inputs {
		manifest.Inputs = append(manifest.Inputs, input.ManifestInput)
		name := strings.TrimSpace(input.Binding.Name)
		if _, bound := values[name]; bound {
			continue
		}
		if !invalid[name] && len(input.Default) > 0 {
			if value, err := typedInputValue(input.Type, input.Default); err == nil {
				values[name] = value
				continue
			}
		}
		if !input.Required || satisfied[name] {
			continue
		}
		readiness.MissingEnv = append(readiness.MissingEnv, name)
		if invalid[name] {
			readiness.InvalidInputs = append(readiness.InvalidInputs, input.ID)
		}
	}
	if len(readiness.MissingEnv) == 0 {
		return nil
	}
	slices.Sort(readiness.MissingEnv)
	return readiness.RequiredError(manifest)
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
