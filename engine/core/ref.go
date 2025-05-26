package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// IsType checks if the component matches the specified type
func (c ComponentType) IsType(componentType ComponentType) bool {
	return c == componentType
}

// IsAgent checks if the component is an agent
func (c ComponentType) IsAgent() bool {
	return c.IsType(ComponentAgent)
}

// IsTool checks if the component is a tool
func (c ComponentType) IsTool() bool {
	return c.IsType(ComponentTool)
}

// IsTask checks if the component is a task
func (c ComponentType) IsTask() bool {
	return c.IsType(ComponentTask)
}

// IsWorkflow checks if the component is a workflow
func (c ComponentType) IsWorkflow() bool {
	return c.IsType(ComponentWorkflow)
}

// Pattern returns the regex pattern for the component
func (c ComponentType) Pattern() string {
	return fmt.Sprintf("^%s\\((id|file|dep)=([^)]+)\\)$", c)
}

// RefTypeName represents the type of reference
type RefTypeName string

const (
	RefTypeNameID   RefTypeName = "id"
	RefTypeNameFile RefTypeName = "file"
	RefTypeNameDep  RefTypeName = "dep"
)

// RefType represents the type of reference
type RefType struct {
	Type  RefTypeName
	Value string
}

// parseDependencyParts splits a dependency string into its components
func parseDependencyParts(value string) (owner, repo, packageName, version string, err error) {
	// Split version if present
	parts := strings.Split(value, "@")
	basePart := parts[0]
	if len(parts) > 1 {
		version = parts[1]
	}

	// Split package name if present
	repoParts := strings.Split(basePart, ":")
	ownerRepoPart := repoParts[0]
	if len(repoParts) > 1 {
		packageName = repoParts[1]
	}

	// Split owner and repo
	ownerRepo := strings.Split(ownerRepoPart, "/")
	if len(ownerRepo) != 2 || ownerRepo[0] == "" || ownerRepo[1] == "" {
		return "", "", "", "", fmt.Errorf(
			"invalid dependency %q: %s",
			value,
			"dependency reference must include owner and repository (format: owner/repo)",
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

// ParseRefType parses a reference type string
func ParseRefType(typeStr RefTypeName, value string) (*RefType, error) {
	switch typeStr {
	case RefTypeNameID:
		if strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("reference value cannot be empty")
		}
		return &RefType{
			Type:  typeStr,
			Value: value,
		}, nil
	case RefTypeNameFile:
		return &RefType{
			Type:  typeStr,
			Value: value,
		}, nil
	case RefTypeNameDep:
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
		return nil, fmt.Errorf("invalid type %q: %s", typeStr, "unknown type")
	}
}

// String returns the string representation of the reference type
func (r *RefType) String() string {
	return fmt.Sprintf("%s=%s", r.Type, r.Value)
}

// Validate validates the reference type against a file path
func (r *RefType) Validate(cwd string) error {
	switch r.Type {
	case RefTypeNameID:
		if strings.TrimSpace(r.Value) == "" {
			return fmt.Errorf("reference value cannot be empty")
		}
	case RefTypeNameFile:
		path := filepath.Join(cwd, r.Value)
		if !filepath.IsAbs(path) {
			return fmt.Errorf("invalid file %q: %s", path, "file path must be absolute")
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("invalid file %q: %s", path, "file not found")
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return fmt.Errorf("invalid file %q: %s", path, "invalid file extension: expected yaml or yml, got "+ext)
		}
		return nil
	case RefTypeNameDep:
		parts := strings.Split(r.Value, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf(
				"invalid dependency %q: %s",
				r.Value,
				"dependency reference must include non-empty owner and repository",
			)
		}
	}
	return nil
}

// IsType checks if the reference type matches the specified type
func (r *RefType) IsType(refType RefTypeName) bool {
	return r.Type == refType
}

// IsFile checks if the reference type is a file
func (r *RefType) IsFile() bool {
	return r.IsType(RefTypeNameFile)
}

// IsDep checks if the reference type is a dependency
func (r *RefType) IsDep() bool {
	return r.IsType(RefTypeNameDep)
}

// IsID checks if the reference type is an ID
func (r *RefType) IsID() bool {
	return r.IsType(RefTypeNameID)
}

// PackageRef represents a reference to a package component
type PackageRef struct {
	Component ComponentType `json:"component" yaml:"component"`
	Type      *RefType      `json:"type"      yaml:"type"`
}

// validateComponent checks if a component string is valid
func validateComponent(component string) (ComponentType, error) {
	validComponents := map[string]ComponentType{
		string(ComponentAgent):    ComponentAgent,
		string(ComponentTool):     ComponentTool,
		string(ComponentTask):     ComponentTask,
		string(ComponentWorkflow): ComponentWorkflow,
	}

	if comp, ok := validComponents[component]; ok {
		return comp, nil
	}
	return "", fmt.Errorf("invalid component %q: %s", component, "unknown component")
}

// MarshalJSON implements custom JSON marshaling for PackageRef
func (p *PackageRef) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		ComponentType string `json:"component"`
		Type          string `json:"type"`
	}{
		ComponentType: string(p.Component),
		Type:          p.Type.String(),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling for PackageRef
func (p *PackageRef) UnmarshalJSON(data []byte) error {
	aux := &struct {
		ComponentType string `json:"component"`
		Type          string `json:"type"`
	}{}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	component, err := validateComponent(aux.ComponentType)
	if err != nil {
		return err
	}
	p.Component = component

	parts := strings.SplitN(aux.Type, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid type %q: %s", aux.Type, "invalid format")
	}

	refType, err := ParseRefType(RefTypeName(parts[0]), parts[1])
	if err != nil {
		return err
	}
	p.Type = refType

	return nil
}

// Parse parses a package reference string into a PackageRef
func Parse(ref *PackageRefConfig) (*PackageRef, error) {
	if ref == nil {
		return nil, fmt.Errorf("package reference cannot be empty")
	}

	refStr := string(*ref)
	components := []ComponentType{
		ComponentAgent,
		ComponentTool,
		ComponentTask,
		ComponentWorkflow,
	}

	for _, component := range components {
		pattern := component.Pattern()
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(refStr); matches != nil {
			typeStr := matches[1]
			value := matches[2]
			refType, err := ParseRefType(RefTypeName(typeStr), value)
			if err != nil {
				return nil, err
			}
			return &PackageRef{
				Component: component,
				Type:      refType,
			}, nil
		}
	}

	return nil, fmt.Errorf("invalid type %q: %s", refStr, "invalid format")
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
	return Parse(c)
}
