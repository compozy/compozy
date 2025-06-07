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

// -----------------------------------------------------------------------------
// BaseConfig - Common fields shared between Config and ParallelTaskItem
// -----------------------------------------------------------------------------

type BaseConfig struct {
	ID           string          `json:"id,omitempty"         yaml:"id,omitempty"         mapstructure:"id,omitempty"`
	Type         Type            `json:"type,omitempty"       yaml:"type,omitempty"       mapstructure:"type,omitempty"`
	Config       core.GlobalOpts `json:"config"               yaml:"config"               mapstructure:"config"`
	Agent        *agent.Config   `json:"agent,omitempty"      yaml:"agent,omitempty"      mapstructure:"agent,omitempty"`
	Tool         *tool.Config    `json:"tool,omitempty"       yaml:"tool,omitempty"       mapstructure:"tool,omitempty"`
	InputSchema  *schema.Schema  `json:"input,omitempty"      yaml:"input,omitempty"      mapstructure:"input,omitempty"`
	OutputSchema *schema.Schema  `json:"output,omitempty"     yaml:"output,omitempty"     mapstructure:"output,omitempty"`
	With         *core.Input     `json:"with,omitempty"       yaml:"with,omitempty"       mapstructure:"with,omitempty"`
	Env          *core.EnvMap    `json:"env,omitempty"        yaml:"env,omitempty"        mapstructure:"env,omitempty"`
	// Task configuration
	OnSuccess *core.SuccessTransition `json:"on_success,omitempty" yaml:"on_success,omitempty" mapstructure:"on_success,omitempty"`
	OnError   *core.ErrorTransition   `json:"on_error,omitempty"   yaml:"on_error,omitempty"   mapstructure:"on_error,omitempty"`
	Sleep     string                  `json:"sleep"                yaml:"sleep"                mapstructure:"sleep"`
	Final     bool                    `json:"final"                yaml:"final"                mapstructure:"final"`
	// Private properties
	filePath string
	cwd      *core.CWD
}

// -----------------------------------------------------------------------------
// Task Types
// -----------------------------------------------------------------------------

type Type string

const (
	TaskTypeBasic    Type = "basic"
	TaskTypeDecision Type = "decision"
	TaskTypeParallel Type = "parallel"
)

// -----------------------------------------------------------------------------
// Basic Task
// -----------------------------------------------------------------------------

type BasicTask struct {
	Action string `json:"action,omitempty" yaml:"action,omitempty" mapstructure:"action,omitempty"`
}

// -----------------------------------------------------------------------------
// Decision Task
// -----------------------------------------------------------------------------

type DecisionTask struct {
	Condition string            `json:"condition,omitempty" yaml:"condition,omitempty" mapstructure:"condition,omitempty"`
	Routes    map[string]string `json:"routes,omitempty"    yaml:"routes,omitempty"    mapstructure:"routes,omitempty"`
}

// -----------------------------------------------------------------------------
// Parallel Task
// -----------------------------------------------------------------------------

type ParallelStrategy string

const (
	StrategyWaitAll    ParallelStrategy = "wait_all"    // Default: wait for all tasks to complete
	StrategyFailFast   ParallelStrategy = "fail_fast"   // Stop on first failure
	StrategyBestEffort ParallelStrategy = "best_effort" // Continue even if some tasks fail
	StrategyRace       ParallelStrategy = "race"        // Return when first task completes
)

type ParallelTask struct {
	Strategy   ParallelStrategy `json:"strategy,omitempty"    yaml:"strategy,omitempty"    mapstructure:"strategy,omitempty"`
	MaxWorkers int              `json:"max_workers,omitempty" yaml:"max_workers,omitempty" mapstructure:"max_workers,omitempty"`
	Timeout    string           `json:"timeout,omitempty"     yaml:"timeout,omitempty"     mapstructure:"timeout,omitempty"`
	Retries    int              `json:"retries,omitempty"     yaml:"retries,omitempty"     mapstructure:"retries,omitempty"`
	Tasks      []Config         `json:"tasks"                 yaml:"tasks"                 mapstructure:"tasks"`
	Task       *Config          `json:"task,omitempty"        yaml:"task,omitempty"        mapstructure:"task,omitempty"`
}

func (pt *ParallelTask) GetTasks() []Config {
	return pt.Tasks
}

func (pt *ParallelTask) GetTimeout() (time.Duration, error) {
	if pt.Timeout == "" {
		return 0, nil
	}
	return core.ParseHumanDuration(pt.Timeout)
}

func (pt *ParallelTask) GetStrategy() ParallelStrategy {
	if pt.Strategy == "" {
		return StrategyWaitAll
	}
	return pt.Strategy
}

func (pt *ParallelTask) GetMaxWorkers() int {
	if pt.MaxWorkers <= 0 {
		return len(pt.Tasks) // Default to number of tasks
	}
	return pt.MaxWorkers
}

// -----------------------------------------------------------------------------
// Config
// -----------------------------------------------------------------------------

type Config struct {
	BasicTask    `json:",inline" yaml:",inline" mapstructure:",squash"`
	DecisionTask `json:",inline" yaml:",inline" mapstructure:",squash"`
	ParallelTask `json:",inline" yaml:",inline" mapstructure:",squash"`
	BaseConfig   `json:",inline" yaml:",inline" mapstructure:",squash"`
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

func (t *Config) HasSchema() bool {
	return t.InputSchema != nil || t.OutputSchema != nil
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

func (t *Config) GetGlobalOpts() *core.GlobalOpts {
	return &t.Config
}

func (t *Config) Validate() error {
	// First run basic validation
	v := schema.NewCompositeValidator(
		schema.NewCWDValidator(t.cwd, t.ID),
		NewTaskTypeValidator(t),
	)
	if err := v.Validate(); err != nil {
		return err
	}
	// Then check for cycles in parallel tasks
	if t.Type == TaskTypeParallel {
		cycleValidator := NewCycleValidator()
		if err := cycleValidator.ValidateConfig(t); err != nil {
			return err
		}
	}
	return nil
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

func (t *Config) GetSleepDuration() (time.Duration, error) {
	if t.Sleep == "" {
		return 0, nil
	}
	return core.ParseHumanDuration(t.Sleep)
}

func (t *Config) GetExecType() ExecutionType {
	taskType := t.Type
	if taskType == "" {
		taskType = TaskTypeBasic
	}
	var executionType ExecutionType
	switch taskType {
	case TaskTypeParallel:
		executionType = ExecutionParallel
	default:
		executionType = ExecutionBasic
	}
	return executionType
}

func FindConfig(tasks []Config, taskID string) (*Config, error) {
	for i := range tasks {
		if tasks[i].ID == taskID {
			return &tasks[i], nil
		}
	}
	return nil, fmt.Errorf("task not found")
}

func propagateCWDToSubTasks(config *Config) error {
	if config.Type == TaskTypeParallel && len(config.Tasks) > 0 {
		for i := range config.Tasks {
			if config.Tasks[i].cwd == nil && config.cwd != nil {
				if err := config.Tasks[i].SetCWD(config.cwd.PathStr()); err != nil {
					return fmt.Errorf("failed to set CWD for sub-task %s: %w", config.Tasks[i].ID, err)
				}
			}
			// Recursively propagate CWD to nested parallel tasks
			if err := propagateCWDToSubTasks(&config.Tasks[i]); err != nil {
				return err
			}
		}
	}
	return nil
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
	if err := propagateCWDToSubTasks(config); err != nil {
		return nil, err
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
	if err := propagateCWDToSubTasks(config); err != nil {
		return nil, err
	}
	return config, nil
}
