package agent

import (
	"fmt"
	"os"

	"dario.cat/mergo"

	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/pkgref"
	"github.com/compozy/compozy/internal/parser/provider"
	"github.com/compozy/compozy/internal/parser/schema"
	"github.com/compozy/compozy/internal/parser/tool"
	"github.com/compozy/compozy/internal/parser/validator"
)

// AgentActionConfig represents an agent action configuration
type AgentActionConfig struct {
	ID           string               `json:"id" yaml:"id"`
	Prompt       string               `json:"prompt" yaml:"prompt"`
	InputSchema  *schema.InputSchema  `json:"input,omitempty" yaml:"input,omitempty"`
	OutputSchema *schema.OutputSchema `json:"output,omitempty" yaml:"output,omitempty"`
	With         *common.Input        `json:"with,omitempty" yaml:"with,omitempty"`
	cwd          *common.CWD          // internal field for current working directory
}

// SetCWD sets the current working directory for the action
func (a *AgentActionConfig) SetCWD(path string) error {
	normalizedPath, err := common.CWDFromPath(path)
	if err != nil {
		return fmt.Errorf("failed to normalize path: %w", err)
	}
	a.cwd = normalizedPath
	return nil
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
	v := validator.NewCompositeValidator(
		validator.NewCWDValidator(a.cwd, string(a.ID)),
		validator.NewStructValidator(a),
	)
	return v.Validate()
}

func (t *AgentActionConfig) ValidateParams(input map[string]any) error {
	return validator.NewParamsValidator(input, t.InputSchema.Schema, t.ID).Validate()
}

// AgentConfig represents an agent configuration
type AgentConfig struct {
	ID           string                   `json:"id" yaml:"id" validate:"required"`
	Use          *pkgref.PackageRefConfig `json:"use,omitempty" yaml:"use,omitempty"`
	Config       provider.ProviderConfig  `json:"config" yaml:"config" validate:"required"`
	Instructions string                   `json:"instructions" yaml:"instructions" validate:"required"`
	Tools        []tool.ToolConfig        `json:"tools,omitempty" yaml:"tools,omitempty"`
	Actions      []*AgentActionConfig     `json:"actions,omitempty" yaml:"actions,omitempty"`
	InputSchema  *schema.InputSchema      `json:"input,omitempty" yaml:"input,omitempty"`
	OutputSchema *schema.OutputSchema     `json:"output,omitempty" yaml:"output,omitempty"`
	With         *common.Input            `json:"with,omitempty" yaml:"with,omitempty"`
	Env          common.EnvMap            `json:"env,omitempty" yaml:"env,omitempty"`
	cwd          *common.CWD              // internal field for current working directory
}

func (a *AgentConfig) Component() common.ComponentType {
	return common.ComponentAgent
}

// SetCWD sets the current working directory for the agent
func (a *AgentConfig) SetCWD(path string) error {
	normalizedPath, err := common.CWDFromPath(path)
	if err != nil {
		return fmt.Errorf("failed to normalize path: %w", err)
	}
	a.cwd = normalizedPath
	for i := range a.Actions {
		a.Actions[i].SetCWD(path)
	}
	return nil
}

// GetCWD returns the current working directory
func (a *AgentConfig) GetCWD() string {
	if a.cwd == nil {
		return ""
	}
	return a.cwd.Get()
}

// Load loads an agent configuration from a file
func Load(cwd *common.CWD, path string) (*AgentConfig, error) {
	config, err := common.LoadConfig[*AgentConfig](cwd, path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to open agent config file: %w", err)
		}
		return nil, fmt.Errorf("failed to decode agent config: %w", err)
	}
	return config, nil
}

// Validate validates the agent configuration
func (a *AgentConfig) Validate() error {
	v := validator.NewCompositeValidator(
		validator.NewCWDValidator(a.cwd, string(a.ID)),
		schema.NewSchemaValidator(a.Use, a.InputSchema, a.OutputSchema),
		NewPackageRefValidator(a.Use, a.cwd),
		NewActionsValidator(a.Actions),
		validator.NewStructValidator(a),
	)
	return v.Validate()
}

func (a *AgentConfig) ValidateParams(input map[string]any) error {
	return validator.NewParamsValidator(input, a.InputSchema.Schema, a.ID).Validate()
}

// Merge merges another agent configuration into this one
func (a *AgentConfig) Merge(other any) error {
	otherConfig, ok := other.(*AgentConfig)
	if !ok {
		return fmt.Errorf("failed to merge agent configs: %s", "invalid type for merge")
	}
	return mergo.Merge(a, otherConfig, mergo.WithOverride)
}

// LoadID loads the ID from either the direct ID field or resolves it from a package reference
func (a *AgentConfig) LoadID() (string, error) {
	return common.LoadID(a, a.ID, a.Use)
}
