package agent

import (
	"context"
	"errors"
	"fmt"

	"dario.cat/mergo"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/ref"
)

type Config struct {
	Resource     string              `json:"resource,omitempty"       yaml:"resource,omitempty"       mapstructure:"resource,omitempty"`
	ID           string              `json:"id"                       yaml:"id"                       mapstructure:"id"                       validate:"required"`
	Config       core.ProviderConfig `json:"config"                   yaml:"config"                   mapstructure:"config"                   validate:"required"`
	Instructions string              `json:"instructions"             yaml:"instructions"             mapstructure:"instructions"             validate:"required"`
	Actions      []*ActionConfig     `json:"actions,omitempty"        yaml:"actions,omitempty"        mapstructure:"actions,omitempty"`
	With         *core.Input         `json:"with,omitempty"           yaml:"with,omitempty"           mapstructure:"with,omitempty"`
	Env          *core.EnvMap        `json:"env,omitempty"            yaml:"env,omitempty"            mapstructure:"env,omitempty"`
	// When defined here, the agent will have toolChoice defined as "auto"
	Tools         []tool.Config `json:"tools,omitempty"          yaml:"tools,omitempty"          mapstructure:"tools,omitempty"`
	MCPs          []mcp.Config  `json:"mcps,omitempty"           yaml:"mcps,omitempty"           mapstructure:"mcps,omitempty"`
	MaxIterations int           `json:"max_iterations,omitempty" yaml:"max_iterations,omitempty" mapstructure:"max_iterations,omitempty"`
	JSONMode      bool          `json:"json_mode"                yaml:"json_mode"                mapstructure:"json_mode"`
	// Memory configuration - simplified single format
	// memory:
	//   - id: user_memory
	//     key: "user:{{.workflow.input.user_id}}"
	//     mode: "read-write"  # optional, defaults to "read-write"
	Memory []core.MemoryReference `json:"memory,omitempty"         yaml:"memory,omitempty"         mapstructure:"memory,omitempty"`

	filePath string
	CWD      *core.PathCWD
}

func (a *Config) Component() core.ConfigType {
	return core.ConfigAgent
}

func (a *Config) GetFilePath() string {
	return a.filePath
}

func (a *Config) SetFilePath(path string) {
	a.filePath = path
}

func (a *Config) SetCWD(path string) error {
	CWD, err := core.CWDFromPath(path)
	if err != nil {
		return err
	}
	a.CWD = CWD
	for i := range a.Actions {
		if err := a.Actions[i].SetCWD(path); err != nil {
			return err
		}
	}
	return nil
}

func (a *Config) GetCWD() *core.PathCWD {
	return a.CWD
}

func (a *Config) GetInput() *core.Input {
	if a.With == nil {
		a.With = &core.Input{}
	}
	return a.With
}

func (a *Config) GetEnv() core.EnvMap {
	if a.Env == nil {
		a.Env = &core.EnvMap{}
		return *a.Env
	}
	return *a.Env
}

func (a *Config) HasSchema() bool {
	return false
}

func (a *Config) GetMaxIterations() int {
	if a.MaxIterations == 0 {
		return 5
	}
	return a.MaxIterations
}

// NormalizeAndValidateMemoryConfig processes the memory configuration for the agent.
// Validates the simplified memory configuration format.
func (a *Config) NormalizeAndValidateMemoryConfig() error {
	const defaultMemoryMode = "read-write"

	for i := range a.Memory {
		if a.Memory[i].ID == "" {
			return fmt.Errorf("memory reference %d missing required 'id' field", i)
		}
		if a.Memory[i].Key == "" {
			return fmt.Errorf("memory reference %d (id: %s) missing required 'key' field", i, a.Memory[i].ID)
		}
		if a.Memory[i].Mode == "" {
			a.Memory[i].Mode = defaultMemoryMode
		}
		if a.Memory[i].Mode != "read-write" && a.Memory[i].Mode != "read-only" {
			return fmt.Errorf(
				"memory reference %d (id: %s) has invalid mode '%s', must be 'read-write' or 'read-only'",
				i, a.Memory[i].ID, a.Memory[i].Mode,
			)
		}
	}
	return nil
}

func (a *Config) Validate() error {
	// Initial struct validation (for required fields like ID, Config, Instructions)
	baseValidator := schema.NewStructValidator(a)
	if err := baseValidator.Validate(); err != nil {
		return err
	}

	// Normalize and validate memory configuration first
	if err := a.NormalizeAndValidateMemoryConfig(); err != nil {
		return fmt.Errorf("invalid memory configuration: %w", err)
	}

	// Now build composite validator including memory (if any)
	v := schema.NewCompositeValidator(
		schema.NewCWDValidator(a.CWD, a.ID),
		NewActionsValidator(a.Actions),
		NewMemoryValidator(a.Memory),
	)
	if err := v.Validate(); err != nil {
		return fmt.Errorf("agent config validation failed: %w", err)
	}

	var mcpErrors []error
	for i := range a.MCPs {
		if err := a.MCPs[i].Validate(); err != nil {
			mcpErrors = append(mcpErrors, fmt.Errorf("mcp validation error: %w", err))
		}
	}
	if len(mcpErrors) > 0 {
		return errors.Join(mcpErrors...)
	}
	return nil
}

func (a *Config) ValidateInput(_ context.Context, _ *core.Input) error {
	// Does not make sense the agent having a schema
	return nil
}

func (a *Config) ValidateOutput(_ context.Context, _ *core.Output) error {
	// Does not make sense the agent having a schema
	return nil
}

func (a *Config) Merge(other any) error {
	otherConfig, ok := other.(*Config)
	if !ok {
		return fmt.Errorf("failed to merge agent configs: %s", "invalid type for merge")
	}
	return mergo.Merge(a, otherConfig, mergo.WithOverride)
}

func (a *Config) Clone() (*Config, error) {
	if a == nil {
		return nil, nil
	}
	return core.DeepCopy(a)
}

func (a *Config) AsMap() (map[string]any, error) {
	return core.AsMapDefault(a)
}

func (a *Config) FromMap(data any) error {
	config, err := core.FromMapDefault[*Config](data)
	if err != nil {
		return err
	}
	return a.Merge(config)
}

func Load(cwd *core.PathCWD, path string) (*Config, error) {
	filePath, err := core.ResolvePath(cwd, path)
	if err != nil {
		return nil, err
	}
	config, _, err := core.LoadConfig[*Config](filePath)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func LoadAndEval(cwd *core.PathCWD, path string, ev *ref.Evaluator) (*Config, error) {
	filePath, err := core.ResolvePath(cwd, path)
	if err != nil {
		return nil, err
	}
	config, _, err := core.LoadConfigWithEvaluator[*Config](filePath, ev)
	if err != nil {
		return nil, err
	}
	return config, nil
}
