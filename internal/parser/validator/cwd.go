package validator

import (
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/parser/common"
)

type CWDValidator struct {
	id  string
	cwd *common.CWD
}

func NewCWDValidator(cwd *common.CWD, id string) *CWDValidator {
	return &CWDValidator{cwd: cwd, id: id}
}

func (v *CWDValidator) Validate() error {
	if v.cwd == nil || v.cwd.Get() == "" {
		return fmt.Errorf("%w for %s", errors.New("current working directory is required"), v.id)
	}
	return nil
}
