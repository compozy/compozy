package schema

import (
	"errors"
	"fmt"

	"github.com/compozy/compozy/engine/core"
)

type CWDValidator struct {
	id  string
	cwd *core.CWD
}

func NewCWDValidator(cwd *core.CWD, id string) *CWDValidator {
	return &CWDValidator{cwd: cwd, id: id}
}

func (v *CWDValidator) Validate() error {
	if v.cwd == nil || v.cwd.PathStr() == "" {
		return fmt.Errorf("%w for %s", errors.New("current working directory is required"), v.id)
	}
	return nil
}
