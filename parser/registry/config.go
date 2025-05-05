package registry

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/compozy/compozy/parser/author"
	"github.com/compozy/compozy/parser/common"
	"github.com/compozy/compozy/parser/project"
)

// RegistryError represents errors that can occur during registry configuration
type RegistryError struct {
	Message string
	Code    string
}

func (e *RegistryError) Error() string {
	return e.Message
}

// RegistryConfig represents a registry component configuration
type RegistryConfig struct {
	Type         ComponentType         `json:"type" yaml:"type"`
	Name         ComponentName         `json:"name" yaml:"name"`
	Version      ComponentVersion      `json:"version" yaml:"version"`
	Main         ComponentMainPath     `json:"main" yaml:"main"`
	License      *ComponentLicense     `json:"license,omitempty" yaml:"license,omitempty"`
	Description  *ComponentDescription `json:"description,omitempty" yaml:"description,omitempty"`
	Repository   *ComponentRepository  `json:"repository,omitempty" yaml:"repository,omitempty"`
	Tags         []ComponentTag        `json:"tags,omitempty" yaml:"tags,omitempty"`
	Author       *author.Author        `json:"author,omitempty" yaml:"author,omitempty"`
	Dependencies *project.Dependencies `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`

	cwd *common.CWD // internal field for current working directory
}

// SetCWD sets the current working directory for the registry
func (r *RegistryConfig) SetCWD(path string) {
	if r.cwd == nil {
		r.cwd = common.NewCWD(path)
	} else {
		r.cwd.Set(path)
	}
}

// GetCWD returns the current working directory
func (r *RegistryConfig) GetCWD() string {
	if r.cwd == nil {
		return ""
	}
	return r.cwd.Get()
}

// Load loads a registry configuration from a file
func Load(path string) (*RegistryConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, &RegistryError{
			Message: "Failed to open registry config file: " + err.Error(),
			Code:    "FILE_OPEN_ERROR",
		}
	}
	defer file.Close()

	var config RegistryConfig
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, &RegistryError{
			Message: "Failed to decode registry config: " + err.Error(),
			Code:    "DECODE_ERROR",
		}
	}

	config.SetCWD(filepath.Dir(path))
	return &config, nil
}

// Validate validates the registry configuration
func (r *RegistryConfig) Validate() error {
	if r.cwd == nil || r.cwd.Get() == "" {
		return &RegistryError{
			Message: "Missing file path for registry",
			Code:    "MISSING_FILE_PATH",
		}
	}

	// Validate component type
	switch r.Type {
	case ComponentTypeAgent, ComponentTypeTool, ComponentTypeTask:
		// Valid types
	default:
		return &RegistryError{
			Message: "Invalid component type: " + string(r.Type),
			Code:    "INVALID_COMPONENT_TYPE",
		}
	}

	// Validate main path exists
	mainPath := r.cwd.Join(string(r.Main))
	if _, err := os.Stat(mainPath); os.IsNotExist(err) {
		return &RegistryError{
			Message: "Main path does not exist: " + string(r.Main),
			Code:    "MAIN_PATH_NOT_FOUND",
		}
	}

	return nil
}
