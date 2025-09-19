package agent

import (
	"fmt"

	"github.com/compozy/compozy/engine/core"
	// "github.com/compozy/compozy/engine/autoload" // Will be needed for registry lookup later
)

// Memory access mode constants.
const (
	MemoryModeReadWrite = "read-write" // Full read and write access
	MemoryModeReadOnly  = "read-only"  // Read-only access
)

// ActionsValidator validates agent action configurations.
type ActionsValidator struct {
	actions []*ActionConfig
}

// NewActionsValidator creates a validator for agent actions.
func NewActionsValidator(actions []*ActionConfig) *ActionsValidator {
	return &ActionsValidator{actions: actions}
}

// Validate ensures all actions have valid configurations.
func (v *ActionsValidator) Validate() error {
	if v.actions == nil {
		return nil
	}
	for _, action := range v.actions {
		if err := action.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// MemoryValidator validates memory references in agent configurations.
type MemoryValidator struct {
	references []core.MemoryReference
}

// NewMemoryValidator creates a validator for memory references.
func NewMemoryValidator(refs []core.MemoryReference /*, reg *autoload.Registry */) *MemoryValidator {
	return &MemoryValidator{
		references: refs,
	}
}

// Validate ensures memory references have valid IDs, keys, and access modes.
func (v *MemoryValidator) Validate() error {
	if v.references == nil {
		// This means no memory configuration was found or explicitly set to none, which is valid.
		return nil
	}

	if len(v.references) == 0 {
		// Also valid (e.g. memory: false, or just no memory fields)
		return nil
	}

	for i, ref := range v.references {
		// Basic structural validation should have been done by normalizeAndValidateMemoryConfig
		// and schema.NewStructValidator if applied to core.MemoryReference itself.
		// Here, we focus on cross-reference validation, like existence in a registry.
		if ref.ID == "" {
			// This should ideally be caught earlier by struct validation tags on MemoryReference
			return fmt.Errorf("memory reference at index %d has an empty ID", i)
		}
		// Key may be empty in ID-only form; runtime will use the memory.Config default_key_template.
		if ref.Mode != MemoryModeReadWrite && ref.Mode != MemoryModeReadOnly {
			return fmt.Errorf(
				"memory reference for ID '%s' (index %d) has invalid mode '%s'; must be '%s' or '%s'",
				ref.ID,
				i,
				ref.Mode,
				MemoryModeReadWrite,
				MemoryModeReadOnly,
			)
		}
	}
	return nil
}
