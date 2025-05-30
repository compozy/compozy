package agent

import (
	"fmt"
)

// -----------------------------------------------------------------------------
// ActionsValidator
// -----------------------------------------------------------------------------

type ActionsValidator struct {
	config *Config
}

func NewActionsValidator(config *Config) *ActionsValidator {
	return &ActionsValidator{config: config}
}

func (v *ActionsValidator) Validate() error {
	if v.config == nil || v.config.Actions == nil {
		return nil
	}
	actionIDs := make(map[string]bool)
	for i, action := range v.config.Actions {
		if action == nil {
			return fmt.Errorf("action at index %d is nil", i)
		}
		if action.ID == "" {
			return fmt.Errorf("action at index %d has empty ID", i)
		}
		if actionIDs[action.ID] {
			return fmt.Errorf("duplicate action ID: %s", action.ID)
		}
		actionIDs[action.ID] = true
		if err := action.Validate(); err != nil {
			return fmt.Errorf("action %s validation failed: %w", action.ID, err)
		}
	}
	return nil
}
