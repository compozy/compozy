package tool

import (
	"fmt"
	"os"
	"path/filepath"

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
	ref, err := v.pkgRef.IntoRef()
	if err != nil {
		return fmt.Errorf("invalid package reference: %w", err)
	}
	if !ref.Component.IsTool() {
		return fmt.Errorf("package reference must be a tool")
	}
	if err := ref.Type.Validate(v.cwd.Get()); err != nil {
		return fmt.Errorf("invalid package reference: %w", err)
	}
	return nil
}

// ExecuteValidator validates the tool execution path
type ExecuteValidator struct {
	execute string
	cwd     *common.CWD
	id      string
}

func NewExecuteValidator(execute string, cwd *common.CWD) *ExecuteValidator {
	return &ExecuteValidator{execute: execute, cwd: cwd}
}

func (v *ExecuteValidator) WithID(id string) *ExecuteValidator {
	v.id = id
	return v
}

func (v *ExecuteValidator) Validate() error {
	if v.execute == "" {
		return nil
	}
	executePath := v.cwd.Join(v.execute)
	executePath, err := filepath.Abs(executePath)
	if err != nil {
		return fmt.Errorf("invalid execute path: %w", err)
	}
	if !TestMode && IsTypeScript(v.execute) && !fileExists(executePath) {
		if v.id == "" {
			return fmt.Errorf("tool ID is required for TypeScript execution")
		}
		return fmt.Errorf("invalid tool execute path: %s", executePath)
	}
	return nil
}

// Helper function to check if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
