package agent

import (
	"fmt"

	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/pkgref"
)

// PackageRefValidator validates the package reference
type PackageRefValidator struct {
	pkgRef *pkgref.PackageRefConfig
	cwd    *common.CWD
}

func NewPackageRefValidator(pkgRef *pkgref.PackageRefConfig, cwd *common.CWD) *PackageRefValidator {
	return &PackageRefValidator{pkgRef: pkgRef, cwd: cwd}
}

func (v *PackageRefValidator) Validate() error {
	if v.pkgRef == nil {
		return nil
	}
	ref, err := pkgref.Parse(string(*v.pkgRef))
	if err != nil {
		return fmt.Errorf("invalid package reference: %w", err)
	}
	if !ref.Component.IsAgent() {
		return fmt.Errorf("package reference must be an agent")
	}
	if err := ref.Type.Validate(v.cwd.Get()); err != nil {
		return fmt.Errorf("invalid package reference: %w", err)
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
