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
	Outputs      *core.Input     `json:"outputs,omitempty"    yaml:"outputs,omitempty"    mapstructure:"outputs,omitempty"`
	Env          *core.EnvMap    `json:"env,omitempty"        yaml:"env,omitempty"        mapstructure:"env,omitempty"`
	// Task configuration
	OnSuccess *core.SuccessTransition `json:"on_success,omitempty" yaml:"on_success,omitempty" mapstructure:"on_success,omitempty"`
	OnError   *core.ErrorTransition   `json:"on_error,omitempty"   yaml:"on_error,omitempty"   mapstructure:"on_error,omitempty"`
	Sleep     string                  `json:"sleep"                yaml:"sleep"                mapstructure:"sleep"`
	Final     bool                    `json:"final"                yaml:"final"                mapstructure:"final"`
	// Path and working directory properties
	FilePath string        `json:"file_path,omitempty"  yaml:"file_path,omitempty"  mapstructure:"file_path,omitempty"`
	CWD      *core.PathCWD `json:"CWD,omitempty"        yaml:"CWD,omitempty"        mapstructure:"CWD,omitempty"`
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

// ValidateStrategy checks if the given string is a valid ParallelStrategy
func ValidateStrategy(strategy string) bool {
	switch ParallelStrategy(strategy) {
	case StrategyWaitAll, StrategyFailFast, StrategyBestEffort, StrategyRace:
		return true
	default:
		return false
	}
}

type ParallelTask struct {
	Strategy   ParallelStrategy `json:"strategy,omitempty"    yaml:"strategy,omitempty"    mapstructure:"strategy,omitempty"`
	MaxWorkers int              `json:"max_workers,omitempty" yaml:"max_workers,omitempty" mapstructure:"max_workers,omitempty"`
	Timeout    string           `json:"timeout,omitempty"     yaml:"timeout,omitempty"     mapstructure:"timeout,omitempty"`
	Retries    int              `json:"retries,omitempty"     yaml:"retries,omitempty"     mapstructure:"retries,omitempty"`
	Tasks      []Config         `json:"tasks"                 yaml:"tasks"                 mapstructure:"tasks"`
	Task       *Config          `json:"task,omitempty"        yaml:"task,omitempty"        mapstructure:"task,omitempty"`
}

// -----------------------------------------------------------------------------
// Collection Task
// -----------------------------------------------------------------------------

type CollectionMode string

const (
	CollectionModeParallel   CollectionMode = "parallel"
	CollectionModeSequential CollectionMode = "sequential"
)

// ValidateCollectionMode checks if the given string is a valid CollectionMode
func ValidateCollectionMode(mode string) bool {
	switch CollectionMode(mode) {
	case CollectionModeParallel, CollectionModeSequential:
		return true
	default:
		return false
	}
}

type CollectionConfig struct {
	Items    string         `json:"items"               yaml:"items"               mapstructure:"items"`
	Filter   string         `json:"filter,omitempty"    yaml:"filter,omitempty"    mapstructure:"filter,omitempty"`
	ItemVar  string         `json:"item_var,omitempty"  yaml:"item_var,omitempty"  mapstructure:"item_var,omitempty"`
	IndexVar string         `json:"index_var,omitempty" yaml:"index_var,omitempty" mapstructure:"index_var,omitempty"`
	Mode     CollectionMode `json:"mode,omitempty"      yaml:"mode,omitempty"      mapstructure:"mode,omitempty"`
	Batch    int            `json:"batch,omitempty"     yaml:"batch,omitempty"     mapstructure:"batch,omitempty"`
}

// Default sets sensible defaults for collection configuration
func (cc *CollectionConfig) Default() {
	if cc.Mode == "" {
		cc.Mode = CollectionModeParallel
	}
	if cc.ItemVar == "" {
		cc.ItemVar = "item"
	}
	if cc.IndexVar == "" {
		cc.IndexVar = "index"
	}
	if cc.Batch == 0 {
		cc.Batch = 0 // Keep 0 as default (no batching)
	}
}

// GetItemVar returns the item variable name
func (cc *CollectionConfig) GetItemVar() string {
	return cc.ItemVar
}

// GetIndexVar returns the index variable name
func (cc *CollectionConfig) GetIndexVar() string {
	return cc.IndexVar
}

// GetMode returns the collection mode
func (cc *CollectionConfig) GetMode() CollectionMode {
	return cc.Mode
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
	BasicTask        `json:",inline" yaml:",inline" mapstructure:",squash"`
	RouterTask       `json:",inline" yaml:",inline" mapstructure:",squash"`
	ParallelTask     `json:",inline" yaml:",inline" mapstructure:",squash"`
	CollectionConfig `json:",inline" yaml:",inline" mapstructure:",squash"`
	BaseConfig       `json:",inline" yaml:",inline" mapstructure:",squash"`
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
	return t.FilePath
}

func (t *Config) SetFilePath(path string) {
	t.FilePath = path
}

func (t *Config) SetCWD(path string) error {
	CWD, err := core.CWDFromPath(path)
	if err != nil {
		return err
	}
	t.CWD = CWD
	return nil
}

func (t *Config) GetCWD() *core.PathCWD {
	return t.CWD
}

func (t *Config) GetGlobalOpts() *core.GlobalOpts {
	return &t.Config
}

func (t *Config) Validate() error {
	// First run basic validation
	v := schema.NewCompositeValidator(
		schema.NewCWDValidator(t.CWD, t.ID),
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

func FindConfig(tasks []Config, taskID string) (*Config, error) {
	for i := range tasks {
		if tasks[i].ID == taskID {
			return &tasks[i], nil
		}
	}
	return nil, fmt.Errorf("task not found")
}

func applyDefaults(config *Config) {
	// Apply defaults for collection tasks
	if config.Type == TaskTypeCollection {
		config.Default()
	}

	// Recursively apply defaults to sub-tasks
	if config.Type == TaskTypeParallel && len(config.Tasks) > 0 {
		for i := range config.Tasks {
			applyDefaults(&config.Tasks[i])
		}
	}

	// Handle collection tasks with task template
	if config.Type == TaskTypeCollection && config.Task != nil {
		applyDefaults(config.Task)
	}

	// Handle collection tasks with tasks array
	if config.Type == TaskTypeCollection && len(config.Tasks) > 0 {
		for i := range config.Tasks {
			applyDefaults(&config.Tasks[i])
		}
	}
}

// setCWDForTask sets the CWD for a single task if needed
func setCWDForTask(task *Config, parentCWD *core.PathCWD, taskType string) error {
	if task.CWD != nil || parentCWD == nil {
		return nil
	}

	if err := task.SetCWD(parentCWD.PathStr()); err != nil {
		return fmt.Errorf("failed to set CWD for %s %s: %w", taskType, task.ID, err)
	}
	return nil
}

// propagateCWDToTaskList propagates CWD to a list of tasks
func propagateCWDToTaskList(tasks []Config, parentCWD *core.PathCWD, taskType string) error {
	for i := range tasks {
		if err := setCWDForTask(&tasks[i], parentCWD, taskType); err != nil {
			return err
		}
		if err := propagateCWDToSubTasks(&tasks[i]); err != nil {
			return err
		}
	}
	return nil
}

// propagateCWDToSingleTask propagates CWD to a single task
func propagateCWDToSingleTask(task *Config, parentCWD *core.PathCWD, taskType string) error {
	if err := setCWDForTask(task, parentCWD, taskType); err != nil {
		return err
	}
	return propagateCWDToSubTasks(task)
}

func propagateCWDToSubTasks(config *Config) error {
	switch config.Type {
	case TaskTypeParallel:
		if len(config.Tasks) > 0 {
			return propagateCWDToTaskList(config.Tasks, config.CWD, "sub-task")
		}
	case TaskTypeCollection:
		if config.Task != nil {
			if err := propagateCWDToSingleTask(config.Task, config.CWD, "collection task template"); err != nil {
				return err
			}
		}
		if len(config.Tasks) > 0 {
			return propagateCWDToTaskList(config.Tasks, config.CWD, "collection task")
		}
	}
	return nil
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
	if string(config.Type) == "" {
		config.Type = TaskTypeBasic
	}
	applyDefaults(config)
	if err := propagateCWDToSubTasks(config); err != nil {
		return nil, err
	}
	return config, nil
}

func LoadAndEval(cwd *core.PathCWD, path string, ev *ref.Evaluator) (*Config, error) {
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
	applyDefaults(config)
	if err := propagateCWDToSubTasks(config); err != nil {
		return nil, err
	}
	return config, nil
}
