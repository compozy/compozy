package common

type Config interface {
	Component() ComponentType
	SetCWD(path string) error
	GetCWD() string
	Validate() error
	ValidateParams(input map[string]any) error
	Merge(other any) error
	LoadID() (string, error)
}

type ComponentType string

const (
	ComponentWorkflow ComponentType = "workflow"
	ComponentTask     ComponentType = "task"
	ComponentAgent    ComponentType = "agent"
	ComponentTool     ComponentType = "tool"
)
