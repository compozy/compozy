package agent

import (
	"fmt"

	"github.com/compozy/compozy/engine/memory" // Assuming memory.MemoryReference is here
	// "github.com/compozy/compozy/engine/autoload" // Will be needed for registry lookup later
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

// AgentMemoryValidator validates the resolved memory references in an agent's configuration.
type AgentMemoryValidator struct {
	references []memory.MemoryReference
	// registry *autoload.Registry // Will be needed in Task 4.0 to check if memory IDs exist
}

// NewAgentMemoryValidator creates a new validator for agent memory configurations.
// It expects normalized memory.MemoryReference structs.
func NewAgentMemoryValidator(refs []memory.MemoryReference /*, reg *autoload.Registry */) *AgentMemoryValidator {
	return &AgentMemoryValidator{
		references: refs,
		// registry: reg,
	}
}

func (v *AgentMemoryValidator) Validate() error {
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
		// and schema.NewStructValidator if applied to memory.MemoryReference itself.
		// Here, we focus on cross-reference validation, like existence in a registry.

		if ref.ID == "" {
			// This should ideally be caught earlier by struct validation tags on MemoryReference
			return fmt.Errorf("memory reference at index %d has an empty ID", i)
		}
		if ref.Key == "" {
			// Also should be caught by struct validation on MemoryReference
			return fmt.Errorf("memory reference for ID '%s' (index %d) has an empty key template", ref.ID, i)
		}
		if ref.Mode != "read-write" && ref.Mode != "read-only" {
			return fmt.Errorf("memory reference for ID '%s' (index %d) has invalid mode '%s'; must be 'read-write' or 'read-only'", ref.ID, i, ref.Mode)
		}

		// Placeholder for checking if ref.ID exists in the memory resource registry (Task 4.0)
		// if v.registry != nil {
		// 	if !v.registry.MemoryExists(ref.ID) { // Assuming a method like MemoryExists
		// 		return fmt.Errorf("memory resource with ID '%s' referenced by agent is not defined", ref.ID)
		// 	}
		// } else {
		// 	// Log warning or handle if registry is not available during this validation phase
		//  // For now, we can't perform this check.
		// }
	}
	return nil
}
