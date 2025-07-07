package agent

import (
	"fmt"

	"github.com/compozy/compozy/engine/core"
	// "github.com/compozy/compozy/engine/autoload" // Will be needed for registry lookup later
)

const (
	MemoryModeReadWrite = "read-write"
	MemoryModeReadOnly  = "read-only"
)

// -----------------------------------------------------------------------------
// ActionsValidator
// -----------------------------------------------------------------------------

type ActionsValidator struct {
	actions []*ActionConfig
}

func NewActionsValidator(actions []*ActionConfig) *ActionsValidator {
	return &ActionsValidator{actions: actions}
}

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

// -----------------------------------------------------------------------------
// AgentMemoryValidator
// -----------------------------------------------------------------------------

// MemoryValidator validates the resolved memory references in an agent's configuration.
type MemoryValidator struct {
	references []core.MemoryReference
}

// NewMemoryValidator creates a new validator for agent memory configurations.
// It expects normalized core.MemoryReference structs.
func NewMemoryValidator(refs []core.MemoryReference /*, reg *autoload.Registry */) *MemoryValidator {
	return &MemoryValidator{
		references: refs,
	}
}

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
		if ref.Key == "" {
			// Also should be caught by struct validation on MemoryReference
			return fmt.Errorf("memory reference for ID '%s' (index %d) has an empty key template", ref.ID, i)
		}
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
