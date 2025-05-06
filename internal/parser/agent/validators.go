package agent

import (
	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/package_ref"
)

// CWDValidator validates the current working directory
type CWDValidator struct {
	cwd *common.CWD
	id  *AgentID
}

func NewCWDValidator(cwd *common.CWD, id *AgentID) *CWDValidator {
	return &CWDValidator{cwd: cwd, id: id}
}

func (v *CWDValidator) Validate() error {
	if v.cwd == nil || v.cwd.Get() == "" {
		return NewMissingPathError(string(*v.id))
	}
	return nil
}

// IDValidator validates the agent ID
type IDValidator struct {
	id *AgentID
}

func NewIDValidator(id *AgentID) *IDValidator {
	return &IDValidator{id: id}
}

func (v *IDValidator) Validate() error {
	if v.id == nil {
		return NewMissingAgentIDError()
	}
	return nil
}

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

// SchemaValidator validates input/output schemas
type SchemaValidator struct {
	schema interface{ Validate() error }
}

func NewSchemaValidator(schema interface{ Validate() error }) *SchemaValidator {
	return &SchemaValidator{schema: schema}
}

func (v *SchemaValidator) Validate() error {
	if v.schema == nil {
		return nil
	}
	if s, ok := v.schema.(*common.InputSchema); ok && s == nil {
		return nil
	}
	if s, ok := v.schema.(*common.OutputSchema); ok && s == nil {
		return nil
	}
	return v.schema.Validate()
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
