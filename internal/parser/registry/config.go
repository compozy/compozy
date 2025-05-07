package registry

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/compozy/compozy/internal/parser/author"
	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/project"
)

type ComponentType string

const (
	ComponentTypeAgent ComponentType = "agent"
	ComponentTypeTool  ComponentType = "tool"
	ComponentTypeTask  ComponentType = "task"
)

// RegistryConfig represents a registry component configuration
type RegistryConfig struct {
	Type         ComponentType         `json:"type" yaml:"type"`
	Name         string                `json:"name" yaml:"name"`
	Version      string                `json:"version" yaml:"version"`
	Main         string                `json:"main" yaml:"main"`
	License      string                `json:"license,omitempty" yaml:"license,omitempty"`
	Description  string                `json:"description,omitempty" yaml:"description,omitempty"`
	Repository   string                `json:"repository,omitempty" yaml:"repository,omitempty"`
	Tags         []string              `json:"tags,omitempty" yaml:"tags,omitempty"`
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
		return nil, NewFileOpenError(err)
	}

	var config RegistryConfig
	decoder := yaml.NewDecoder(file)
	decodeErr := decoder.Decode(&config)
	closeErr := file.Close()

	if decodeErr != nil {
		return nil, NewDecodeError(decodeErr)
	}
	if closeErr != nil {
		return nil, NewFileCloseError(closeErr)
	}

	config.SetCWD(filepath.Dir(path))
	return &config, nil
}

// Validate validates the registry configuration
func (r *RegistryConfig) Validate() error {
	validator := common.NewCompositeValidator(
		NewCWDValidator(r.cwd),
		NewComponentTypeValidator(r.Type),
		NewMainPathValidator(r.cwd, r.Main),
	)
	return validator.Validate()
}
