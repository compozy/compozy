package tool

import (
	"fmt"
)

// -----------------------------------------------------------------------------
// ExecuteValidator
// -----------------------------------------------------------------------------

type ExecuteValidator struct {
	config *Config
}

func NewExecuteValidator(config *Config) *ExecuteValidator {
	return &ExecuteValidator{config: config}
}

func (v *ExecuteValidator) Validate() error {
	if v.config.Execute == "" {
		return nil
	}

	if !IsTypeScript(v.config.Execute) {
		if v.config.ID == "" {
			return fmt.Errorf("tool ID is required for TypeScript execution")
		}
		return fmt.Errorf("invalid typescript file: %s", v.config.Execute)
	}

	_, err := v.config.CWD.JoinAndCheck(v.config.Execute)
	if err != nil {
		return err
	}
	return nil
}
