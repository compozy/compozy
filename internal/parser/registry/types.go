package registry

type ComponentType string

const (
	ComponentTypeAgent ComponentType = "agent"
	ComponentTypeTool  ComponentType = "tool"
	ComponentTypeTask  ComponentType = "task"
)

type ComponentName string
type ComponentVersion string
type ComponentLicense string
type ComponentDescription string
type ComponentRepository string
type ComponentMainPath string
type ComponentTag string
