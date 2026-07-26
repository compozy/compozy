package hooks

import (
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/compozy/agh/internal/vault"
)

// ErrExecutorResolverRequired reports that full normalization needs an executor
// resolver to attach the execution implementation.
var ErrExecutorResolverRequired = errors.New("hooks: executor resolver is required")

// ExecutorResolver binds a normalized declaration to its executor
// implementation.
type ExecutorResolver func(HookDecl) (Executor, error)

// ValidateHookDecl validates one declaration without binding an executor.
func ValidateHookDecl(decl HookDecl) error {
	_, err := normalizeHookDecl(decl, nil, false)
	return err
}

// CanonicalizeHookDecl validates one declaration and applies defaults without
// binding an executor.
func CanonicalizeHookDecl(decl HookDecl) (HookDecl, error) {
	return sanitizedHookDecl(decl)
}

// ValidateHookDecls validates a declaration slice and stops at the first error.
func ValidateHookDecls(decls []HookDecl) error {
	for idx, decl := range decls {
		if err := ValidateHookDecl(decl); err != nil {
			return fmt.Errorf("hooks: validate declaration %d (%q): %w", idx, strings.TrimSpace(decl.Name), err)
		}
	}
	return nil
}

// NormalizeHookDecl validates one declaration, applies defaults, and binds the
// executor.
func NormalizeHookDecl(decl HookDecl, resolve ExecutorResolver) (ResolvedHook, error) {
	return normalizeHookDecl(decl, resolve, true)
}

// NormalizeHookDecls normalizes every declaration in order and stops at the
// first error.
func NormalizeHookDecls(decls []HookDecl, resolve ExecutorResolver) ([]ResolvedHook, error) {
	resolved := make([]ResolvedHook, 0, len(decls))
	for idx, decl := range decls {
		if !decl.EnabledValue() {
			continue
		}
		hook, err := NormalizeHookDecl(decl, resolve)
		if err != nil {
			return nil, fmt.Errorf("hooks: normalize declaration %d (%q): %w", idx, strings.TrimSpace(decl.Name), err)
		}
		resolved = append(resolved, hook)
	}
	return resolved, nil
}

func normalizeHookDecl(decl HookDecl, resolve ExecutorResolver, bindExecutor bool) (ResolvedHook, error) {
	normalized, err := sanitizedHookDecl(decl)
	if err != nil {
		return ResolvedHook{}, err
	}

	registered := RegisteredHook{
		Name:     normalized.Name,
		Event:    normalized.Event,
		Source:   normalized.Source,
		Mode:     normalized.Mode,
		Required: normalized.Required,
		Priority: normalized.Priority,
		Timeout:  normalized.Timeout,
		Matcher:  normalized.Matcher,
		Metadata: cloneStringMap(normalized.Metadata),
	}

	if bindExecutor {
		if resolve == nil {
			return ResolvedHook{}, fmt.Errorf(
				"hooks: normalize hook %q: %w",
				normalized.Name,
				ErrExecutorResolverRequired,
			)
		}

		executor, err := resolve(normalized)
		if err != nil {
			return ResolvedHook{}, fmt.Errorf("hooks: resolve executor for hook %q: %w", normalized.Name, err)
		}
		if executor == nil {
			return ResolvedHook{}, fmt.Errorf("hooks: resolve executor for hook %q: nil executor", normalized.Name)
		}
		if executor.Kind() != normalized.ExecutorKind {
			return ResolvedHook{}, fmt.Errorf(
				"hooks: resolve executor for hook %q returned kind %q, want %q",
				normalized.Name,
				executor.Kind(),
				normalized.ExecutorKind,
			)
		}
		registered.Executor = executor
	}

	if err := registered.Validate(); err != nil {
		return ResolvedHook{}, err
	}

	resolved := ResolvedHook{
		RegisteredHook: registered,
		Decl:           normalized,
	}
	if bindExecutor {
		if err := resolved.Validate(); err != nil {
			return ResolvedHook{}, err
		}
	}

	return resolved, nil
}

func sanitizedHookDecl(decl HookDecl) (HookDecl, error) {
	normalized := HookDecl{
		Name:         strings.TrimSpace(decl.Name),
		Event:        decl.Event,
		Source:       decl.Source,
		Mode:         decl.Mode,
		Required:     decl.Required,
		Priority:     decl.Priority,
		PrioritySet:  decl.PrioritySet,
		Timeout:      decl.Timeout,
		Enabled:      cloneBoolPtr(decl.Enabled),
		Matcher:      normalizeHookMatcher(decl.Matcher),
		ExecutorKind: decl.ExecutorKind,
		Command:      strings.TrimSpace(decl.Command),
		Args:         append([]string(nil), decl.Args...),
		WorkingDir:   strings.TrimSpace(decl.WorkingDir),
		Env:          cloneStringMap(decl.Env),
		SecretEnv:    cloneStringMap(decl.SecretEnv),
		Metadata:     cloneStringMap(decl.Metadata),
		SkillSource:  decl.SkillSource,
	}

	if normalized.Name == "" {
		return HookDecl{}, fmt.Errorf("hooks: hook name is required")
	}
	if err := normalized.Event.Validate(); err != nil {
		return HookDecl{}, err
	}
	if err := normalized.Source.Validate(); err != nil {
		return HookDecl{}, err
	}
	if err := normalized.SkillSource.Validate(); err != nil {
		return HookDecl{}, err
	}
	if normalized.Source != HookSourceSkill && normalized.SkillSource != "" {
		return HookDecl{}, fmt.Errorf(
			"hooks: hook %q skill source is only valid for skill declarations",
			normalized.Name,
		)
	}

	if normalized.Mode == "" {
		normalized.Mode = defaultHookMode(normalized.Source)
	}
	if err := normalized.Mode.Validate(); err != nil {
		return HookDecl{}, err
	}
	if normalized.Required && normalized.Mode != HookModeSync {
		return HookDecl{}, fmt.Errorf("hooks: hook %q cannot be required in async mode", normalized.Name)
	}
	if normalized.Mode == HookModeSync && !normalized.Event.SyncEligible() {
		return HookDecl{}, fmt.Errorf(
			"hooks: hook %q cannot use sync mode for async-only event %q",
			normalized.Name,
			normalized.Event,
		)
	}
	if normalized.Timeout < 0 {
		return HookDecl{}, fmt.Errorf("hooks: hook %q timeout must be non-negative", normalized.Name)
	}

	priority, err := resolveHookPriority(normalized)
	if err != nil {
		return HookDecl{}, err
	}
	normalized.Priority = priority

	kind, err := resolveHookExecutorKind(normalized)
	if err != nil {
		return HookDecl{}, err
	}
	normalized.ExecutorKind = kind

	if err := ValidateMatcherForEvent(normalized.Event, normalized.Matcher); err != nil {
		return HookDecl{}, err
	}
	if err := validateHookEnv(normalized); err != nil {
		return HookDecl{}, err
	}

	return normalized, nil
}

func defaultHookMode(_ HookSource) HookMode {
	return HookModeAsync
}

func resolveHookPriority(decl HookDecl) (int32, error) {
	if decl.Priority != 0 || decl.PrioritySet {
		return decl.Priority, nil
	}

	return DefaultHookPriority(decl.Source)
}

func resolveHookExecutorKind(decl HookDecl) (HookExecutorKind, error) {
	kind := decl.ExecutorKind
	if kind == "" {
		switch {
		case decl.Command != "":
			kind = HookExecutorSubprocess
		case decl.Source == HookSourceNative:
			kind = HookExecutorNative
		default:
			return "", fmt.Errorf("hooks: hook %q executor kind is required", decl.Name)
		}
	}

	if err := kind.Validate(); err != nil {
		return "", err
	}

	if kind == HookExecutorNative && decl.Source != HookSourceNative {
		return "", fmt.Errorf("hooks: hook %q only native sources may use native executors", decl.Name)
	}
	if kind == HookExecutorSubprocess && decl.Command == "" {
		return "", fmt.Errorf("hooks: hook %q subprocess executor requires a command", decl.Name)
	}
	if kind != HookExecutorSubprocess && hasSubprocessFields(decl) {
		return "", fmt.Errorf("hooks: hook %q shell command fields require a subprocess executor", decl.Name)
	}

	return kind, nil
}

func hasSubprocessFields(decl HookDecl) bool {
	return decl.Command != "" || len(decl.Args) > 0 || len(decl.Env) > 0 || len(decl.SecretEnv) > 0
}

func validateHookEnv(decl HookDecl) error {
	path := fmt.Sprintf("hooks: hook %q", decl.Name)
	if err := vault.ValidateNonSecretEnvMap(path, decl.Env); err != nil {
		return err
	}
	if err := vault.ValidateSecretEnvMap(path, "hooks", decl.SecretEnv); err != nil {
		return err
	}
	return nil
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}

	dst := make(map[string]string, len(src))
	maps.Copy(dst, src)
	return dst
}

func cloneBoolPtr(src *bool) *bool {
	if src == nil {
		return nil
	}
	value := *src
	return &value
}
