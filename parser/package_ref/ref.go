package package_ref

import (
	"fmt"
	"path/filepath"
	"regexp"
)

// Component represents the type of component being referenced
type Component string

const (
	ComponentAgent    Component = "agent"
	ComponentMcp      Component = "mcp"
	ComponentTool     Component = "tool"
	ComponentTask     Component = "task"
	ComponentWorkflow Component = "workflow"
)

// IsAgent checks if the component is an agent
func (c Component) IsAgent() bool {
	return c == ComponentAgent
}

// IsMcp checks if the component is an mcp
func (c Component) IsMcp() bool {
	return c == ComponentMcp
}

// IsTool checks if the component is a tool
func (c Component) IsTool() bool {
	return c == ComponentTool
}

// IsTask checks if the component is a task
func (c Component) IsTask() bool {
	return c == ComponentTask
}

// IsWorkflow checks if the component is a workflow
func (c Component) IsWorkflow() bool {
	return c == ComponentWorkflow
}

// Pattern returns the regex pattern for the component
func (c Component) Pattern() string {
	return fmt.Sprintf("^%s\\((id|file|dep)=([^)]+)\\)$", c)
}

// RefType represents the type of reference
type RefType struct {
	Type  string
	Value string
}

// Parse parses a reference type string
func ParseRefType(typeStr, value string) (*RefType, error) {
	switch typeStr {
	case "id", "file", "dep":
		return &RefType{
			Type:  typeStr,
			Value: value,
		}, nil
	default:
		return nil, fmt.Errorf("invalid reference type: %s", typeStr)
	}
}

// String returns the string representation of the reference type
func (r *RefType) String() string {
	return fmt.Sprintf("%s=%s", r.Type, r.Value)
}

// Validate validates the reference type against a file path
func (r *RefType) Validate(filePath string) error {
	if r.Type == "file" {
		path := filepath.Join(filepath.Dir(filePath), r.Value)
		if !filepath.IsAbs(path) {
			return fmt.Errorf("file path must be absolute: %s", path)
		}
	}
	return nil
}

// PackageRef represents a reference to a package component
type PackageRef struct {
	Component Component `json:"component" yaml:"component"`
	Type      *RefType  `json:"type" yaml:"type"`
}

// Parse parses a package reference string into a PackageRef
func Parse(ref string) (*PackageRef, error) {
	components := []Component{
		ComponentAgent,
		ComponentMcp,
		ComponentTool,
		ComponentTask,
		ComponentWorkflow,
	}

	for _, component := range components {
		pattern := component.Pattern()
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(ref); matches != nil {
			typeStr := matches[1]
			value := matches[2]
			refType, err := ParseRefType(typeStr, value)
			if err != nil {
				return nil, err
			}
			return &PackageRef{
				Component: component,
				Type:      refType,
			}, nil
		}
	}

	return nil, fmt.Errorf("invalid package reference format: %s", ref)
}

// Value returns the value of the package reference
func (p *PackageRef) Value() string {
	return p.Type.Value
}

// PackageRefConfig represents a package reference configuration
type PackageRefConfig struct {
	Use string `json:"use" yaml:"use"`
}

// New creates a new package reference configuration
func NewPackageRefConfig(value string) *PackageRefConfig {
	return &PackageRefConfig{
		Use: value,
	}
}

// IntoRef converts the configuration into a package reference
func (c *PackageRefConfig) IntoRef() (*PackageRef, error) {
	return Parse(c.Use)
}
