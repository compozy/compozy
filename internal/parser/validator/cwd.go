package validator

import (
	"fmt"

	"github.com/compozy/compozy/internal/parser/common"
)

type CWDValidator struct {
	id  string
	cwd *common.CWD
}

type CWDValidatorError struct {
	Message string
	Code    string
}

func (e *CWDValidatorError) Error() string {
	return e.Message
}

func NewCWDValidator(cwd *common.CWD, id string) *CWDValidator {
	return &CWDValidator{cwd: cwd, id: id}
}

func (v *CWDValidator) Validate() error {
	if v.cwd == nil || v.cwd.Get() == "" {
		return &CWDValidatorError{
			Code:    "MISSING_CWD",
			Message: fmt.Sprintf("Current working directory is required for %s", v.id),
		}
	}
	return nil
}
