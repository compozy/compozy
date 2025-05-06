package agent

import (
	"errors"
	"os"
	"path/filepath"

	"dario.cat/mergo"
	"gopkg.in/yaml.v3"

	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/package_ref"
	"github.com/compozy/compozy/internal/parser/tool"
	v "github.com/compozy/compozy/internal/parser/validator"
)

// AgentActionConfig represents an agent action configuration
type AgentActionConfig struct {
	ID           ActionID             `json:"id" yaml:"id"`
	Prompt       ActionPrompt         `json:"prompt" yaml:"prompt"`
	InputSchema  *common.InputSchema  `json:"input,omitempty" yaml:"input,omitempty"`
	OutputSchema *common.OutputSchema `json:"output,omitempty" yaml:"output,omitempty"`
	With         *common.WithParams   `json:"with,omitempty" yaml:"with,omitempty"`

	cwd *common.CWD // internal field for current working directory
}

// SetCWD sets the current working directory for the action
func (a *AgentActionConfig) SetCWD(path string) {
	if a.cwd == nil {
		a.cwd = common.NewCWD(path)
	} else {
		a.cwd.Set(path)
	}
}

// GetCWD returns the current working directory
func (a *AgentActionConfig) GetCWD() string {
	if a.cwd == nil {
		return ""
	}
	return a.cwd.Get()
}

// Validate validates the action configuration
func (a *AgentActionConfig) Validate() error {
	validator := common.NewCompositeValidator(
		v.NewCWDValidator(a.cwd, string(a.ID)),
		common.NewStructValidator(a),
	)
	return validator.Validate()
}

// AgentConfig represents an agent configuration
type AgentConfig struct {
	ID           *AgentID                      `json:"id,omitempty" yaml:"id,omitempty" validate:"required"`
	Use          *package_ref.PackageRefConfig `json:"use,omitempty" yaml:"use,omitempty"`
	Config       *ProviderConfig               `json:"config,omitempty" yaml:"config,omitempty" validate:"required"`
	Instructions *Instructions                 `json:"instructions,omitempty" yaml:"instructions,omitempty" validate:"required"`
	Tools        []*tool.ToolConfig            `json:"tools,omitempty" yaml:"tools,omitempty"`
	Actions      []*AgentActionConfig          `json:"actions,omitempty" yaml:"actions,omitempty"`
	InputSchema  *common.InputSchema           `json:"input,omitempty" yaml:"input,omitempty"`
	OutputSchema *common.OutputSchema          `json:"output,omitempty" yaml:"output,omitempty"`
	With         *common.WithParams            `json:"with,omitempty" yaml:"with,omitempty"`
	Env          common.EnvMap                 `json:"env,omitempty" yaml:"env,omitempty"`

	cwd *common.CWD // internal field for current working directory
}

// SetCWD sets the current working directory for the agent
func (a *AgentConfig) SetCWD(path string) {
	if a.cwd == nil {
		a.cwd = common.NewCWD(path)
	} else {
		a.cwd.Set(path)
	}
	if a.Actions != nil {
		for _, action := range a.Actions {
			action.SetCWD(path)
		}
	}
}

// GetCWD returns the current working directory
func (a *AgentConfig) GetCWD() string {
	if a.cwd == nil {
		return ""
	}
	return a.cwd.Get()
}

// Load loads an agent configuration from a file
func Load(path string) (*AgentConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, NewFileOpenError(err)
	}
	defer file.Close()

	var config AgentConfig
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, NewDecodeError(err)
	}

	config.SetCWD(filepath.Dir(path))
	return &config, nil
}

// Validate validates the agent configuration
func (a *AgentConfig) Validate() error {
	// Create a composite validator that combines both custom and struct validation
	validator := common.NewCompositeValidator(
		v.NewCWDValidator(a.cwd, string(*a.ID)),
		NewPackageRefValidator(a.Use, a.cwd),
		NewActionsValidator(a.Actions),
		common.NewStructValidator(a),
	)
	return validator.Validate()
}

// Merge merges another agent configuration into this one
func (a *AgentConfig) Merge(other interface{}) error {
	otherConfig, ok := other.(*AgentConfig)
	if !ok {
		return NewMergeError(errors.New("invalid type for merge"))
	}
	return mergo.Merge(a, otherConfig, mergo.WithOverride)
}
