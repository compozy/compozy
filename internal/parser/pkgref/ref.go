package pkgref

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

// parseDependencyParts splits a dependency string into its components
func parseDependencyParts(value string) (owner, repo, packageName, version string, err error) {
	// Split version if present
	parts := strings.Split(value, "@")
	basePart := parts[0]
	version = ""
	if len(parts) > 1 {
		version = parts[1]
	}

	// Split package name if present
	repoParts := strings.Split(basePart, ":")
	ownerRepoPart := repoParts[0]
	packageName = ""
	if len(repoParts) > 1 {
		packageName = repoParts[1]
	}

	// Split owner and repo
	ownerRepo := strings.Split(ownerRepoPart, "/")
	if len(ownerRepo) != 2 || ownerRepo[0] == "" || ownerRepo[1] == "" {
		return "", "", "", "", NewInvalidDependencyError(
			value,
			fmt.Errorf("dependency reference must include owner and repository (format: owner/repo)"),
		)
	}

	return ownerRepo[0], ownerRepo[1], packageName, version, nil
}

// buildDependencyValue constructs a dependency value string from its components
func buildDependencyValue(owner, repo, packageName, version string) string {
	value := fmt.Sprintf("%s/%s", owner, repo)
	if packageName != "" {
		value = fmt.Sprintf("%s:%s", value, packageName)
	}
	if version != "" {
		value = fmt.Sprintf("%s@%s", value, version)
	}
	return value
}

// validateFilePath checks if a file path is valid and exists
func validateFilePath(path string) error {
	if !filepath.IsAbs(path) {
		return NewInvalidFileError(
			path,
			fmt.Errorf("file path must be absolute"),
		)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return NewInvalidFileError(
			path,
			fmt.Errorf("file not found"),
		)
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".yaml" && ext != ".yml" {
		return NewInvalidFileError(
			path,
			fmt.Errorf("invalid file extension: expected yaml or yml, got %s", ext),
		)
	}
	return nil
}

// ParseRefType parses a reference type string
func ParseRefType(typeStr, value string) (*RefType, error) {
	switch typeStr {
	case "id":
		if strings.TrimSpace(value) == "" {
			return nil, NewEmptyValueError()
		}
		return &RefType{
			Type:  typeStr,
			Value: value,
		}, nil
	case "file":
		return &RefType{
			Type:  typeStr,
			Value: value,
		}, nil
	case "dep":
		owner, repo, packageName, version, err := parseDependencyParts(value)
		if err != nil {
			return nil, err
		}
		value = buildDependencyValue(owner, repo, packageName, version)
		return &RefType{
			Type:  typeStr,
			Value: value,
		}, nil
	default:
		return nil, NewInvalidTypeError(typeStr, fmt.Errorf("unknown type"))
	}
}

// String returns the string representation of the reference type
func (r *RefType) String() string {
	return fmt.Sprintf("%s=%s", r.Type, r.Value)
}

// Validate validates the reference type against a file path
func (r *RefType) Validate(filePath string) error {
	switch r.Type {
	case "id":
		if strings.TrimSpace(r.Value) == "" {
			return NewEmptyValueError()
		}
	case "file":
		path := filepath.Join(filepath.Dir(filePath), r.Value)
		return validateFilePath(path)
	case "dep":
		parts := strings.Split(r.Value, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return NewInvalidDependencyError(
				r.Value,
				fmt.Errorf("dependency reference must include non-empty owner and repository"),
			)
		}
	}
	return nil
}

// PackageRef represents a reference to a package component
type PackageRef struct {
	Component Component `json:"component" yaml:"component"`
	Type      *RefType  `json:"type" yaml:"type"`
}

// validateComponent checks if a component string is valid
func validateComponent(component string) (Component, error) {
	switch component {
	case string(ComponentAgent), string(ComponentMcp), string(ComponentTool),
		string(ComponentTask), string(ComponentWorkflow):
		return Component(component), nil
	default:
		return "", NewInvalidComponentError(component, fmt.Errorf("unknown component"))
	}
}

// MarshalJSON implements custom JSON marshaling for PackageRef
func (p *PackageRef) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Component string `json:"component"`
		Type      string `json:"type"`
	}{
		Component: string(p.Component),
		Type:      p.Type.String(),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling for PackageRef
func (p *PackageRef) UnmarshalJSON(data []byte) error {
	aux := &struct {
		Component string `json:"component"`
		Type      string `json:"type"`
	}{}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	component, err := validateComponent(aux.Component)
	if err != nil {
		return err
	}
	p.Component = component

	parts := strings.SplitN(aux.Type, "=", 2)
	if len(parts) != 2 {
		return NewInvalidTypeError(aux.Type, fmt.Errorf("invalid format"))
	}

	refType, err := ParseRefType(parts[0], parts[1])
	if err != nil {
		return err
	}
	p.Type = refType

	return nil
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

	return nil, NewInvalidTypeError(ref, fmt.Errorf("invalid format"))
}

// Value returns the value of the package reference
func (p *PackageRef) Value() string {
	return p.Type.Value
}

// PackageRefConfig represents a package reference configuration
type PackageRefConfig string

// NewPackageRefConfig creates a new package reference configuration
func NewPackageRefConfig(value string) *PackageRefConfig {
	ref := PackageRefConfig(value)
	return &ref
}

// IntoRef converts the configuration into a package reference
func (c *PackageRefConfig) IntoRef() (*PackageRef, error) {
	return Parse(string(*c))
}
