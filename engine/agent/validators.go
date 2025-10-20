package agent

import (
	"context"
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
func (v *ActionsValidator) Validate(ctx context.Context) error {
	if v.actions == nil {
		return nil
	}
	for i, action := range v.actions {
		if action == nil {
			return fmt.Errorf("action at index %d is nil", i)
		}
		if err := action.Validate(ctx); err != nil {
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
func (v *MemoryValidator) Validate(_ context.Context) error {
	if v.references == nil {
		return nil
	}
	if len(v.references) == 0 {
		return nil
	}
	for i, ref := range v.references {
		if ref.ID == "" {
			return fmt.Errorf("memory reference at index %d has an empty ID", i)
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
