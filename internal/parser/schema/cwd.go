package schema

import (
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
		return NewMissingCWDError(v.id)
	}
	return nil
}
