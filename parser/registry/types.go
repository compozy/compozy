package registry

// ComponentType represents the type of a component
type ComponentType string

const (
	ComponentTypeAgent ComponentType = "agent"
	ComponentTypeTool  ComponentType = "tool"
	ComponentTypeTask  ComponentType = "task"
)

// ComponentName represents a component name
type ComponentName string

// ComponentVersion represents a component version
type ComponentVersion string

// ComponentLicense represents a component license
type ComponentLicense string

// ComponentDescription represents a component description
type ComponentDescription string

// ComponentRepository represents a component repository
type ComponentRepository string

// ComponentMainPath represents a component's main path
type ComponentMainPath string

// ComponentTag represents a component tag
type ComponentTag string
