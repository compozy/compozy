package common

type Config interface {
	Component() ComponentType
	SetCWD(path string) error
	GetCWD() *CWD
	Validate() error
	ValidateParams(input map[string]any) error
	Merge(other any) error
	LoadID() (string, error)
}

type ComponentType string

const (
	ComponentProject  ComponentType = "project"
	ComponentWorkflow ComponentType = "workflow"
	ComponentTask     ComponentType = "task"
	ComponentAgent    ComponentType = "agent"
	ComponentTool     ComponentType = "tool"
)
