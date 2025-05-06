package tool

import (
	"os"
	"path/filepath"

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
	ref, err := v.pkgRef.IntoRef()
	if err != nil {
		return NewInvalidPackageRefError(err)
	}
	if !ref.Component.IsTool() {
		return NewInvalidTypeError()
	}
	if err := ref.Type.Validate(v.cwd.Get()); err != nil {
		return NewInvalidPackageRefError(err)
	}
	return nil
}

// ExecuteValidator validates the tool execution path
type ExecuteValidator struct {
	execute *ToolExecute
	cwd     *common.CWD
	id      *ToolID
}

func NewExecuteValidator(execute *ToolExecute, cwd *common.CWD) *ExecuteValidator {
	return &ExecuteValidator{execute: execute, cwd: cwd}
}

func (v *ExecuteValidator) WithID(id *ToolID) *ExecuteValidator {
	v.id = id
	return v
}

func (v *ExecuteValidator) Validate() error {
	if v.execute == nil {
		return nil
	}
	executePath := v.cwd.Join(string(*v.execute))
	executePath, err := filepath.Abs(executePath)
	if err != nil {
		return NewInvalidExecutePathError(err)
	}
	if !TestMode && v.execute.IsTypeScript() && !fileExists(executePath) {
		if v.id == nil {
			return NewMissingToolIDError()
		}
		return NewInvalidToolExecuteError(executePath)
	}
	return nil
}

// Helper function to check if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
