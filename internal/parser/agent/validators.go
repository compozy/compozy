package agent

import (
	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/package_ref"
)

// PackageRefValidator validates the package reference
type PackageRefValidator struct {
	pkgRef *package_ref.PackageRefConfig
	cwd    *common.CWD
}

func NewPackageRefValidator(pkgRef *package_ref.PackageRefConfig, cwd *common.CWD) *PackageRefValidator {
	return &PackageRefValidator{pkgRef: pkgRef, cwd: cwd}
}

func (v *PackageRefValidator) Validate() error {
	if v.pkgRef == nil {
		return nil
	}
	ref, err := package_ref.Parse(string(*v.pkgRef))
	if err != nil {
		return NewInvalidPackageRefError(err)
	}
	if !ref.Component.IsAgent() {
		return NewInvalidComponentTypeError()
	}
	if err := ref.Type.Validate(v.cwd.Get()); err != nil {
		return NewInvalidPackageRefError(err)
	}
	return nil
}

// ActionsValidator validates agent actions
type ActionsValidator struct {
	actions []*AgentActionConfig
}

func NewActionsValidator(actions []*AgentActionConfig) *ActionsValidator {
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
