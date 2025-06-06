package task

import (
	"context"
	"errors"
	"fmt"
	"time"

	"dario.cat/mergo"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/ref"
)

type Type string

const (
	TaskTypeBasic    Type = "basic"
	TaskTypeDecision Type = "decision"
)

type Opts struct {
	core.GlobalOpts `json:",inline" yaml:",inline" mapstructure:",squash"`
}

type Config struct {
	ID           string         `json:"id,omitempty"     yaml:"id,omitempty"     mapstructure:"id,omitempty"`
	Type         Type           `json:"type,omitempty"   yaml:"type,omitempty"   mapstructure:"type,omitempty"`
	InputSchema  *schema.Schema `json:"input,omitempty"  yaml:"input,omitempty"  mapstructure:"input,omitempty"`
	OutputSchema *schema.Schema `json:"output,omitempty" yaml:"output,omitempty" mapstructure:"output,omitempty"`
	With         *core.Input    `json:"with,omitempty"   yaml:"with,omitempty"   mapstructure:"with,omitempty"`
	Env          *core.EnvMap   `json:"env,omitempty"    yaml:"env,omitempty"    mapstructure:"env,omitempty"`
	Opts         Opts           `json:"config"           yaml:"config"           mapstructure:"config"`

	// Task configuration
	OnSuccess *core.SuccessTransition `json:"on_success,omitempty" yaml:"on_success,omitempty" mapstructure:"on_success,omitempty"`
	OnError   *core.ErrorTransition   `json:"on_error,omitempty"   yaml:"on_error,omitempty"   mapstructure:"on_error,omitempty"`
	Sleep     string                  `json:"sleep"                yaml:"sleep"                mapstructure:"sleep"`
	Final     bool                    `json:"final"                yaml:"final"                mapstructure:"final"`

	// Basic task properties
	Action string `json:"action,omitempty" yaml:"action,omitempty" mapstructure:"action,omitempty"`

	// Decision task properties
	Condition string            `json:"condition,omitempty" yaml:"condition,omitempty" mapstructure:"condition,omitempty"`
	Routes    map[string]string `json:"routes,omitempty"    yaml:"routes,omitempty"    mapstructure:"routes,omitempty"`

	filePath string
	cwd      *core.CWD
	Agent    *agent.Config `json:"agent,omitempty" yaml:"agent,omitempty" mapstructure:"agent,omitempty"`
	Tool     *tool.Config  `json:"tool,omitempty"  yaml:"tool,omitempty"  mapstructure:"tool,omitempty"`
}

func (t *Config) Component() core.ConfigType {
	return core.ConfigTask
}

func (t *Config) GetFilePath() string {
	return t.filePath
}

func (t *Config) SetFilePath(path string) {
	t.filePath = path
}

func (t *Config) SetCWD(path string) error {
	cwd, err := core.CWDFromPath(path)
	if err != nil {
		return err
	}
	t.cwd = cwd
	return nil
}

func (t *Config) GetCWD() *core.CWD {
	return t.cwd
}

func (t *Config) GetEnv() core.EnvMap {
	if t.Env == nil {
		t.Env = &core.EnvMap{}
		return *t.Env
	}
	return *t.Env
}

func (t *Config) GetInput() *core.Input {
	if t.With == nil {
		t.With = &core.Input{}
	}
	return t.With
}

func (t *Config) GetAgent() *agent.Config {
	return t.Agent
}

func (t *Config) GetTool() *tool.Config {
	return t.Tool
}

// GetGlobalOpts returns the task-level global options (including timeouts and retry policy)
func (t *Config) GetGlobalOpts() *core.GlobalOpts {
	return &t.Opts.GlobalOpts
}

func (t *Config) Validate() error {
	v := schema.NewCompositeValidator(
		schema.NewCWDValidator(t.cwd, t.ID),
		NewTaskTypeValidator(t),
	)
	return v.Validate()
}

func (t *Config) ValidateInput(ctx context.Context, input *core.Input) error {
	if t.InputSchema == nil || input == nil {
		return nil
	}
	return schema.NewParamsValidator(input, t.InputSchema, t.ID).Validate(ctx)
}

func (t *Config) ValidateOutput(ctx context.Context, output *core.Output) error {
	if t.OutputSchema == nil || output == nil {
		return nil
	}
	return schema.NewParamsValidator(output, t.OutputSchema, t.ID).Validate(ctx)
}

func (t *Config) Merge(other any) error {
	otherConfig, ok := other.(*Config)
	if !ok {
		return fmt.Errorf("failed to merge task configs: %w", errors.New("invalid type for merge"))
	}
	return mergo.Merge(t, otherConfig, mergo.WithOverride)
}

func (t *Config) AsMap() (map[string]any, error) {
	return core.AsMapDefault(t)
}

// FromMap updates the provider configuration from a normalized map
func (t *Config) FromMap(data any) error {
	config, err := core.FromMapDefault[*Config](data)
	if err != nil {
		return err
	}
	return t.Merge(config)
}

func (t *Config) HasSchema() bool {
	return t.InputSchema != nil || t.OutputSchema != nil
}

func (t *Config) GetSleepDuration() (time.Duration, error) {
	if t.Sleep == "" {
		return 0, nil
	}
	return core.ParseHumanDuration(t.Sleep)
}

func FindConfig(tasks []Config, taskID string) (*Config, error) {
	for i := range tasks {
		if tasks[i].ID == taskID {
			return &tasks[i], nil
		}
	}
	return nil, fmt.Errorf("task not found")
}

func Load(cwd *core.CWD, path string) (*Config, error) {
	filePath, err := core.ResolvePath(cwd, path)
	if err != nil {
		return nil, err
	}
	config, _, err := core.LoadConfig[*Config](filePath)
	if err != nil {
		return nil, err
	}
	if string(config.Type) == "" {
		config.Type = TaskTypeBasic
	}
	return config, nil
}

func LoadAndEval(cwd *core.CWD, path string, ev *ref.Evaluator) (*Config, error) {
	filePath, err := core.ResolvePath(cwd, path)
	if err != nil {
		return nil, err
	}
	scope, err := core.MapFromFilePath(filePath)
	if err != nil {
		return nil, err
	}
	ev.WithLocalScope(scope)
	config, _, err := core.LoadConfigWithEvaluator[*Config](filePath, ev)
	if err != nil {
		return nil, err
	}
	if string(config.Type) == "" {
		config.Type = TaskTypeBasic
	}
	return config, nil
}
