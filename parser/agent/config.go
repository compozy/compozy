package agent

import (
	"os"

	"gopkg.in/yaml.v3"

	"github.com/compozy/compozy/parser/common"
	"github.com/compozy/compozy/parser/package_ref"
	"github.com/compozy/compozy/parser/tool"
)

// AgentConfigError represents errors that can occur during agent configuration
type AgentConfigError struct {
	Message string
	Code    string
}

func (e *AgentConfigError) Error() string {
	return e.Message
}

// AgentActionConfig represents an agent action configuration
type AgentActionConfig struct {
	ID           ActionID             `json:"id" yaml:"id"`
	Prompt       ActionPrompt         `json:"prompt" yaml:"prompt"`
	InputSchema  *common.InputSchema  `json:"input,omitempty" yaml:"input,omitempty"`
	OutputSchema *common.OutputSchema `json:"output,omitempty" yaml:"output,omitempty"`
	With         *common.WithParams   `json:"with,omitempty" yaml:"with,omitempty"`

	cwd string // internal field for current working directory
}

// SetCWD sets the current working directory for the action
func (a *AgentActionConfig) SetCWD(path string) {
	a.cwd = path
}

// GetCWD returns the current working directory
func (a *AgentActionConfig) GetCWD() string {
	return a.cwd
}

// Validate validates the action configuration
func (a *AgentActionConfig) Validate() error {
	if a.cwd == "" {
		return &AgentConfigError{
			Message: "Missing file path for action: " + string(a.ID),
			Code:    "MISSING_FILE_PATH",
		}
	}

	// Validate input schema if present
	if a.InputSchema != nil {
		if err := a.InputSchema.Validate(); err != nil {
			return &AgentConfigError{
				Message: "Invalid input schema: " + err.Error(),
				Code:    "INVALID_INPUT_SCHEMA",
			}
		}
	}

	// Validate output schema if present
	if a.OutputSchema != nil {
		if err := a.OutputSchema.Validate(); err != nil {
			return &AgentConfigError{
				Message: "Invalid output schema: " + err.Error(),
				Code:    "INVALID_OUTPUT_SCHEMA",
			}
		}
	}

	return nil
}

// AgentConfig represents an agent configuration
type AgentConfig struct {
	ID           *AgentID                      `json:"id,omitempty" yaml:"id,omitempty"`
	PackageRef   *package_ref.PackageRefConfig `json:"package_ref,omitempty" yaml:"package_ref,omitempty"`
	Config       *ProviderConfig               `json:"config,omitempty" yaml:"config,omitempty"`
	Instructions *Instructions                 `json:"instructions,omitempty" yaml:"instructions,omitempty"`
	Tools        []*tool.ToolConfig            `json:"tools,omitempty" yaml:"tools,omitempty"`
	Actions      []*AgentActionConfig          `json:"actions,omitempty" yaml:"actions,omitempty"`
	InputSchema  *common.InputSchema           `json:"input,omitempty" yaml:"input,omitempty"`
	OutputSchema *common.OutputSchema          `json:"output,omitempty" yaml:"output,omitempty"`
	With         *common.WithParams            `json:"with,omitempty" yaml:"with,omitempty"`
	Env          common.EnvMap                 `json:"env,omitempty" yaml:"env,omitempty"`

	cwd string // internal field for current working directory
}

// SetCWD sets the current working directory for the agent
func (a *AgentConfig) SetCWD(path string) {
	a.cwd = path
	if a.Actions != nil {
		for _, action := range a.Actions {
			action.SetCWD(path)
		}
	}
}

// GetCWD returns the current working directory
func (a *AgentConfig) GetCWD() string {
	return a.cwd
}

// Load loads an agent configuration from a file
func Load(path string) (*AgentConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, &AgentConfigError{
			Message: "Failed to open agent config file: " + err.Error(),
			Code:    "FILE_OPEN_ERROR",
		}
	}
	defer file.Close()

	var config AgentConfig
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, &AgentConfigError{
			Message: "Failed to decode agent config: " + err.Error(),
			Code:    "DECODE_ERROR",
		}
	}

	config.SetCWD(path)
	return &config, nil
}

// Validate validates the agent configuration
func (a *AgentConfig) Validate() error {
	if a.cwd == "" {
		return &AgentConfigError{
			Message: "Missing file path for agent",
			Code:    "MISSING_FILE_PATH",
		}
	}

	// Validate package reference if present
	if a.PackageRef != nil {
		ref, err := a.PackageRef.IntoRef()
		if err != nil {
			return &AgentConfigError{
				Message: "Invalid package reference: " + err.Error(),
				Code:    "INVALID_PACKAGE_REF",
			}
		}

		// Validate that it's an agent reference
		if !ref.Component.IsAgent() {
			return &AgentConfigError{
				Message: "Package reference must be an agent",
				Code:    "INVALID_COMPONENT_TYPE",
			}
		}

		// Validate the reference against the current working directory
		if err := ref.Type.Validate(a.cwd); err != nil {
			return &AgentConfigError{
				Message: "Invalid package reference: " + err.Error(),
				Code:    "INVALID_PACKAGE_REF",
			}
		}
	}

	// Validate input schema if present
	if a.InputSchema != nil {
		if err := a.InputSchema.Validate(); err != nil {
			return &AgentConfigError{
				Message: "Invalid input schema: " + err.Error(),
				Code:    "INVALID_INPUT_SCHEMA",
			}
		}
	}

	// Validate output schema if present
	if a.OutputSchema != nil {
		if err := a.OutputSchema.Validate(); err != nil {
			return &AgentConfigError{
				Message: "Invalid output schema: " + err.Error(),
				Code:    "INVALID_OUTPUT_SCHEMA",
			}
		}
	}

	// Validate actions if present
	if a.Actions != nil {
		for _, action := range a.Actions {
			if err := action.Validate(); err != nil {
				return err
			}
		}
	}

	return nil
}

// Merge merges another agent configuration into this one
func (a *AgentConfig) Merge(other *AgentConfig) error {
	if a.Env == nil {
		a.Env = other.Env
	} else if other.Env != nil {
		a.Env.Merge(other.Env)
	}
	if a.With == nil {
		a.With = other.With
	}
	return nil
}
