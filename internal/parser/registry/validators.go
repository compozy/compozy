package registry

import (
	"fmt"
	"os"

	"github.com/compozy/compozy/internal/parser/common"
)

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
		return fmt.Errorf("invalid component type: %s", string(v.componentType))
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
		return fmt.Errorf("main path does not exist: %s", string(v.mainPath))
	}
	return nil
}
