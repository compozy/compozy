package pkgref

import (
	"fmt"
)

type ComponentValidator func(Component) bool

type PackageRefValidator struct {
	cwd               string
	pkgRef            *PackageRefConfig
	validateComponent ComponentValidator
}

func NewPackageRefValidator(pkgRef *PackageRefConfig, cwd string, validator ComponentValidator) *PackageRefValidator {
	return &PackageRefValidator{
		cwd:               cwd,
		pkgRef:            pkgRef,
		validateComponent: validator,
	}
}

func (v *PackageRefValidator) Validate() error {
	if v.cwd == "" {
		return fmt.Errorf("cwd is required")
	}
	if v.pkgRef == nil {
		return nil
	}
	ref, err := v.pkgRef.IntoRef()
	if err != nil {
		return fmt.Errorf("invalid package reference: %w", err)
	}
	if v.validateComponent != nil && !v.validateComponent(ref.Component) {
		return fmt.Errorf("package reference has invalid component type")
	}
	if err := ref.Type.Validate(v.cwd); err != nil {
		return fmt.Errorf("invalid package reference: %w", err)
	}
	return nil
}
