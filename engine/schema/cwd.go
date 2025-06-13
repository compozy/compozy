package schema

import (
	"errors"
	"fmt"

	"github.com/compozy/compozy/engine/core"
)

var ErrMissingCWD = errors.New("current working directory is required")

type CWDValidator struct {
	id  string
	CWD *core.PathCWD
}

func NewCWDValidator(cwd *core.PathCWD, id string) *CWDValidator {
	return &CWDValidator{CWD: cwd, id: id}
}

func (v *CWDValidator) Validate() error {
	if v.CWD == nil || v.CWD.PathStr() == "" {
		return fmt.Errorf("%w for %s", ErrMissingCWD, v.id)
	}
	return nil
}
