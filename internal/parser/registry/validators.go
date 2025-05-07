package registry

import (
	"os"

	"github.com/compozy/compozy/internal/parser/common"
)

// CWDValidator validates the current working directory
type CWDValidator struct {
	cwd *common.CWD
}

func NewCWDValidator(cwd *common.CWD) *CWDValidator {
	return &CWDValidator{cwd: cwd}
}

func (v *CWDValidator) Validate() error {
	if v.cwd == nil || v.cwd.Get() == "" {
		return NewMissingPathError()
	}
	return nil
}

// ComponentTypeValidator validates the component type
type ComponentTypeValidator struct {
	componentType ComponentType
}

func NewComponentTypeValidator(componentType ComponentType) *ComponentTypeValidator {
	return &ComponentTypeValidator{componentType: componentType}
}

func (v *ComponentTypeValidator) Validate() error {
	switch v.componentType {
	case ComponentTypeAgent, ComponentTypeTool, ComponentTypeTask:
		return nil
	default:
		return NewInvalidTypeError(string(v.componentType))
	}
}

// MainPathValidator validates that the main path exists
type MainPathValidator struct {
	cwd      *common.CWD
	mainPath string
}

func NewMainPathValidator(cwd *common.CWD, mainPath string) *MainPathValidator {
	return &MainPathValidator{
		cwd:      cwd,
		mainPath: mainPath,
	}
}

func (v *MainPathValidator) Validate() error {
	mainPath := v.cwd.Join(string(v.mainPath))
	if _, err := os.Stat(mainPath); os.IsNotExist(err) {
		return NewMainPathNotFoundError(string(v.mainPath))
	}
	return nil
}
