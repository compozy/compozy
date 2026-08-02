package extensionpkg

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/extensionenv"
)

var (
	// ErrExtensionEnvBindingInvalid reports malformed extension secret binding input.
	ErrExtensionEnvBindingInvalid = errors.New("extension: invalid environment binding")
	// ErrExtensionEnvBindingUndeclared reports a binding outside the current requires_env contract.
	ErrExtensionEnvBindingUndeclared = errors.New("extension: undeclared environment binding")
	// ErrExtensionEnvBindingDangling reports a binding to absent Vault material.
	ErrExtensionEnvBindingDangling = errors.New("extension: environment binding secret is not present")
)

const ExtensionEnvBindingKind = extensionenv.BindingKind

// EnvBinding is one instance-scoped extension environment binding.
type EnvBinding = extensionenv.Binding

// EnvBindingStore persists instance-scoped extension environment bindings.
type EnvBindingStore = extensionenv.Store

// EnvBindingLifecycleStore supports cleanup and safe owned-ref garbage collection.
type EnvBindingLifecycleStore = extensionenv.LifecycleStore

// EnvBindingValidationError carries safe, deterministic binding diagnostics.
type EnvBindingValidationError struct {
	EnvName  string
	Declared []string
	Cause    error
}

func (e *EnvBindingValidationError) Error() string {
	if e == nil {
		return ErrExtensionEnvBindingInvalid.Error()
	}
	name := strings.TrimSpace(e.EnvName)
	if errors.Is(e.Cause, ErrExtensionEnvBindingUndeclared) {
		declared := slices.Clone(e.Declared)
		slices.Sort(declared)
		return fmt.Sprintf(
			"extension: environment %q is not declared; declared names: %s",
			name,
			strings.Join(declared, ", "),
		)
	}
	if name != "" {
		return fmt.Sprintf("extension: environment %q: %v", name, e.Cause)
	}
	return fmt.Sprintf("extension: environment binding: %v", e.Cause)
}

// Unwrap exposes the stable validation class.
func (e *EnvBindingValidationError) Unwrap() error {
	if e == nil || e.Cause == nil {
		return ErrExtensionEnvBindingInvalid
	}
	return e.Cause
}
