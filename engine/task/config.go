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

// Constants for configuration defaults
const (
	DefaultMaxWorkers         = 10
	DefaultBatchSize          = 1
	DefaultItemVariable       = "item"
	DefaultIndexVariable      = "index"
	DefaultCollectionMode     = CollectionModeParallel
	DefaultMaxBatchSize       = 1000  // Prevent excessive batch sizes
	DefaultMaxParallelWorkers = 100   // Prevent resource exhaustion
	DefaultMaxCollectionItems = 10000 // Prevent memory issues
)

// -----------------------------------------------------------------------------
// BaseConfig - Common fields shared between Config and ParallelTaskItem
// -----------------------------------------------------------------------------

type BaseConfig struct {
	ID           string          `json:"id,omitempty"      yaml:"id,omitempty"      mapstructure:"id,omitempty"`
	Type         Type            `json:"type,omitempty"    yaml:"type,omitempty"    mapstructure:"type,omitempty"`
	Config       core.GlobalOpts `json:"config"            yaml:"config"            mapstructure:"config"`
	Agent        *agent.Config   `json:"agent,omitempty"   yaml:"agent,omitempty"   mapstructure:"agent,omitempty"`
	Tool         *tool.Config    `json:"tool,omitempty"    yaml:"tool,omitempty"    mapstructure:"tool,omitempty"`
	InputSchema  *schema.Schema  `json:"input,omitempty"   yaml:"input,omitempty"   mapstructure:"input,omitempty"`
	OutputSchema *schema.Schema  `json:"output,omitempty"  yaml:"output,omitempty"  mapstructure:"output,omitempty"`
	With         *core.Input     `json:"with,omitempty"    yaml:"with,omitempty"    mapstructure:"with,omitempty"`
	Outputs      *core.Input     `json:"outputs,omitempty" yaml:"outputs,omitempty" mapstructure:"outputs,omitempty"`
	Env          *core.EnvMap    `json:"env,omitempty"     yaml:"env,omitempty"     mapstructure:"env,omitempty"`

	// Shared parallel execution fields (used by both parallel and collection tasks)
	Strategy   ParallelStrategy `json:"strategy,omitempty"    yaml:"strategy,omitempty"    mapstructure:"strategy,omitempty"`
	MaxWorkers int              `json:"max_workers,omitempty" yaml:"max_workers,omitempty" mapstructure:"max_workers,omitempty"`
	Timeout    string           `json:"timeout,omitempty"     yaml:"timeout,omitempty"     mapstructure:"timeout,omitempty"`

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
	TaskTypeBasic      Type = "basic"
	TaskTypeRouter     Type = "router"
	TaskTypeParallel   Type = "parallel"
	TaskTypeCollection Type = "collection"
)

// -----------------------------------------------------------------------------
// Basic Task
// -----------------------------------------------------------------------------

type BasicTask struct {
	Action string `json:"action,omitempty" yaml:"action,omitempty" mapstructure:"action,omitempty"`
}

// -----------------------------------------------------------------------------
// Router Task
// -----------------------------------------------------------------------------

type RouterTask struct {
	Condition string         `json:"condition,omitempty" yaml:"condition,omitempty" mapstructure:"condition,omitempty"`
	Routes    map[string]any `json:"routes,omitempty"    yaml:"routes,omitempty"    mapstructure:"routes,omitempty"`
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
	Retries int      `json:"retries,omitempty" yaml:"retries,omitempty" mapstructure:"retries,omitempty"`
	Tasks   []Config `json:"tasks"             yaml:"tasks"             mapstructure:"tasks"`
	Task    *Config  `json:"task,omitempty"    yaml:"task,omitempty"    mapstructure:"task,omitempty"`
}

func (pt *ParallelTask) GetTasks() []Config {
	return pt.Tasks
}

// -----------------------------------------------------------------------------
// Collection Task
// -----------------------------------------------------------------------------

type CollectionMode string

const (
	CollectionModeParallel   CollectionMode = "parallel"
	CollectionModeSequential CollectionMode = "sequential"
)

type CollectionTask struct {
	// Core collection fields
	Items           string         `json:"items"                       yaml:"items"                       mapstructure:"items"`
	Filter          string         `json:"filter,omitempty"            yaml:"filter,omitempty"            mapstructure:"filter,omitempty"`
	Template        *Config        `json:"template"                    yaml:"template"                    mapstructure:"template"`
	Mode            CollectionMode `json:"mode,omitempty"              yaml:"mode,omitempty"              mapstructure:"mode,omitempty"`
	Batch           int            `json:"batch,omitempty"             yaml:"batch,omitempty"             mapstructure:"batch,omitempty"`
	ContinueOnError bool           `json:"continue_on_error,omitempty" yaml:"continue_on_error,omitempty" mapstructure:"continue_on_error,omitempty"`

	// Variable names for template evaluation
	ItemVar  string `json:"item_var,omitempty"  yaml:"item_var,omitempty"  mapstructure:"item_var,omitempty"`
	IndexVar string `json:"index_var,omitempty" yaml:"index_var,omitempty" mapstructure:"index_var,omitempty"`

	// Optional early termination
	StopCondition string `json:"stop_condition,omitempty" yaml:"stop_condition,omitempty" mapstructure:"stop_condition,omitempty"`
}

func (ct *CollectionTask) GetMode() CollectionMode {
	if ct.Mode == "" {
		return DefaultCollectionMode
	}
	return ct.Mode
}

func (ct *CollectionTask) GetBatch() int {
	if ct.Batch <= 0 {
		return DefaultBatchSize
	}
	return ct.Batch
}

func (ct *CollectionTask) GetItemVar() string {
	if ct.ItemVar == "" {
		return DefaultItemVariable
	}
	return ct.ItemVar
}

func (ct *CollectionTask) GetIndexVar() string {
	if ct.IndexVar == "" {
		return DefaultIndexVariable
	}
	return ct.IndexVar
}

// -----------------------------------------------------------------------------
// Config
// -----------------------------------------------------------------------------

type Config struct {
	BasicTask      `json:",inline" yaml:",inline" mapstructure:",squash"`
	RouterTask     `json:",inline" yaml:",inline" mapstructure:",squash"`
	ParallelTask   `json:",inline" yaml:",inline" mapstructure:",squash"`
	CollectionTask `json:",inline" yaml:",inline" mapstructure:",squash"`
	BaseConfig     `json:",inline" yaml:",inline" mapstructure:",squash"`
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

func (t *Config) GetOutputs() *core.Input {
	return t.Outputs
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
	case TaskTypeRouter:
		executionType = ExecutionRouter
	case TaskTypeParallel:
		executionType = ExecutionParallel
	case TaskTypeCollection:
		executionType = ExecutionCollection
	default:
		executionType = ExecutionBasic
	}
	return executionType
}

// Shared parallel execution methods (used by both parallel and collection tasks)
func (t *Config) GetStrategy() ParallelStrategy {
	if t.Strategy == "" {
		return StrategyWaitAll
	}
	return t.Strategy
}

func (t *Config) GetMaxWorkers() int {
	if t.MaxWorkers <= 0 {
		if t.Type == TaskTypeParallel {
			return len(t.Tasks) // Default to number of tasks for parallel
		}
		return DefaultMaxWorkers // Default max workers for collection
	}
	return t.MaxWorkers
}

func (t *Config) GetTimeout() (time.Duration, error) {
	if t.Timeout == "" {
		return 0, nil
	}
	return core.ParseHumanDuration(t.Timeout)
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
	if config.Type == TaskTypeCollection && config.Template != nil {
		if config.Template.cwd == nil && config.cwd != nil {
			if err := config.Template.SetCWD(config.cwd.PathStr()); err != nil {
				return fmt.Errorf("failed to set CWD for collection template %s: %w", config.Template.ID, err)
			}
		}
		// Recursively propagate CWD to nested tasks in the template
		if err := propagateCWDToSubTasks(config.Template); err != nil {
			return err
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
