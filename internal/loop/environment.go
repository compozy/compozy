package loop

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
)

// ResolveActionEnvironment applies node-over-loop precedence and validates the closed environment shape.
func ResolveActionEnvironment(
	node dsl.EnvironmentSpec,
	loopDefault dsl.EnvironmentSpec,
) (dsl.EnvironmentSpec, error) {
	resolved := normalizeEnvironmentSpec(node)
	if resolved.Mode == "" && (resolved.WorktreeRef != "" || resolved.Directory != "") {
		return dsl.EnvironmentSpec{}, fmt.Errorf(
			"%w: node environment fields require an explicit mode",
			ErrValidation,
		)
	}
	if resolved.Mode == "" {
		resolved = normalizeEnvironmentSpec(loopDefault)
	}
	if resolved.Mode == "" {
		resolved.Mode = dsl.EnvironmentRoot
	}
	if err := validateEnvironmentSpec(resolved); err != nil {
		return dsl.EnvironmentSpec{}, err
	}
	return resolved, nil
}

func normalizeEnvironmentSpec(spec dsl.EnvironmentSpec) dsl.EnvironmentSpec {
	spec.Mode = dsl.EnvironmentMode(strings.ToLower(strings.TrimSpace(string(spec.Mode))))
	spec.WorktreeRef = strings.TrimSpace(spec.WorktreeRef)
	spec.Directory = strings.TrimSpace(spec.Directory)
	return spec
}

func validateEnvironmentSpec(spec dsl.EnvironmentSpec) error {
	switch spec.Mode {
	case "", dsl.EnvironmentRoot, dsl.EnvironmentPerRun:
		if spec.WorktreeRef != "" || spec.Directory != "" {
			return fmt.Errorf("%w: environment mode %q cannot set worktree_ref or directory", ErrValidation, spec.Mode)
		}
	case dsl.EnvironmentWorktree:
		if spec.WorktreeRef == "" || spec.Directory != "" {
			return fmt.Errorf("%w: worktree environment requires only worktree_ref", ErrValidation)
		}
	case dsl.EnvironmentDirectory:
		if spec.Directory == "" || spec.WorktreeRef != "" {
			return fmt.Errorf("%w: directory environment requires only directory", ErrValidation)
		}
	default:
		return fmt.Errorf("%w: environment mode is invalid: %q", ErrValidation, spec.Mode)
	}
	return nil
}
