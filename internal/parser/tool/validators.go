package tool

import (
	"fmt"

	"github.com/compozy/compozy/internal/parser/common"
)

type ExecuteValidator struct {
	execute string
	cwd     *common.CWD
	id      string
}

func NewExecuteValidator(execute string, cwd *common.CWD) *ExecuteValidator {
	return &ExecuteValidator{execute: execute, cwd: cwd}
}

func (v *ExecuteValidator) WithID(id string) *ExecuteValidator {
	v.id = id
	return v
}

func (v *ExecuteValidator) Validate() error {
	if v.execute == "" {
		return nil
	}

	if !IsTypeScript(v.execute) {
		if v.id == "" {
			return fmt.Errorf("tool ID is required for TypeScript execution")
		}
		return fmt.Errorf("invalid typescript file: %s", v.execute)
	}

	_, err := v.cwd.JoinAndCheck(v.execute)
	if err != nil {
		return err
	}
	return nil
}
