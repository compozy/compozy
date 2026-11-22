package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

var (
	// ErrExtensionDependenciesRequired reports unmet dependencies without consent to install them. Experimental.
	ErrExtensionDependenciesRequired = errors.New("extension: dependencies required")
	// ErrExtensionDependencyGraphInvalid reports a cyclic, too deep or conflicting dependency graph. Experimental.
	ErrExtensionDependencyGraphInvalid = errors.New("extension: dependency graph invalid")
)

// Dependency graph refusal reasons.
const (
	DependencyGraphReasonCycle             = "cycle"
	DependencyGraphReasonDepth             = "depth"
	DependencyGraphReasonConflictingSource = "conflicting_source"
)

// DependenciesRequiredError lists the unmet dependencies of one install or enable.
type DependenciesRequiredError struct {
	Dependencies []contract.ExtensionDependencyPayload
}

func (e *DependenciesRequiredError) Error() string {
	names := make([]string, 0, len(e.Dependencies))
	for _, dependency := range e.Dependencies {
		names = append(names, dependency.Name+" ("+dependency.State+")")
	}
	return fmt.Sprintf("%s: %s", ErrExtensionDependenciesRequired, strings.Join(names, ", "))
}

func (e *DependenciesRequiredError) Unwrap() error { return ErrExtensionDependenciesRequired }

// DependencyGraphInvalidError refuses a graph before any mutation.
type DependencyGraphInvalidError struct {
	Reason  string
	Path    []string
	Parents []contract.ExtensionDependencyParentPayload
}

func (e *DependencyGraphInvalidError) Error() string {
	return fmt.Sprintf("%s: %s: %s", ErrExtensionDependencyGraphInvalid, e.Reason, strings.Join(e.Path, " -> "))
}

func (e *DependencyGraphInvalidError) Unwrap() error { return ErrExtensionDependencyGraphInvalid }

// DependencyGateError attributes a trust, gateway or input refusal to the dependency it belongs to.
type DependencyGateError struct {
	Dependency string
	Err        error
}

func (e *DependencyGateError) Error() string {
	return fmt.Sprintf("dependency %q: %v", e.Dependency, e.Err)
}

func (e *DependencyGateError) Unwrap() error { return e.Err }

// DependenciesInstalledError reports the dependencies a consented install completed before it failed.
type DependenciesInstalledError struct {
	Installed []string
	Err       error
}

func (e *DependenciesInstalledError) Error() string {
	return fmt.Sprintf("%v (dependencies installed: %s)", e.Err, strings.Join(e.Installed, ", "))
}

func (e *DependenciesInstalledError) Unwrap() error { return e.Err }

// ErrExtensionDependencyInUse refuses a remove or disable an installed dependent still needs. Experimental.
var ErrExtensionDependencyInUse = errors.New("extension: dependency in use")

// DependencyInUseError lists the installed dependents that still resolve their dependency to the target.
type DependencyInUseError struct {
	Name       string
	Dependents []contract.ExtensionDependentPayload
}

func (e *DependencyInUseError) Error() string {
	names := make([]string, 0, len(e.Dependents))
	for _, dependent := range e.Dependents {
		names = append(names, dependent.Name)
	}
	return fmt.Sprintf("%s: %s is required by %s; pass force to proceed", ErrExtensionDependencyInUse, e.Name,
		strings.Join(names, ", "))
}

func (e *DependencyInUseError) Unwrap() error { return ErrExtensionDependencyInUse }

type dependencyForceKey struct{}

// WithDependencyForce carries a remove's consent to leave its dependents unmet (D10). Experimental.
func WithDependencyForce(ctx context.Context, force bool) context.Context {
	if !force {
		return ctx
	}
	return context.WithValue(ctx, dependencyForceKey{}, true)
}

// DependencyForced reports whether the remove carried force.
func DependencyForced(ctx context.Context) bool {
	forced, _ := ctx.Value(dependencyForceKey{}).(bool)
	return forced
}
