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
	Resource     string          `json:"resource,omitempty"   yaml:"resource,omitempty"   mapstructure:"resource,omitempty"`
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
	// Composite and Paralle tasks
	Timeout   string `json:"timeout,omitempty"    yaml:"timeout,omitempty"    mapstructure:"timeout,omitempty"`
	Retries   int    `json:"retries,omitempty"    yaml:"retries,omitempty"    mapstructure:"retries,omitempty"`
	Condition string `json:"condition,omitempty"  yaml:"condition,omitempty"  mapstructure:"condition,omitempty"`
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
	TaskTypeAggregate  Type = "aggregate"
	TaskTypeComposite  Type = "composite"
	TaskTypeSignal     Type = "signal"
	TaskTypeWait       Type = "wait"
	TaskTypeMemory     Type = "memory"
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
	Routes map[string]any `json:"routes,omitempty" yaml:"routes,omitempty" mapstructure:"routes,omitempty"`
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
}

func (pt *ParallelTask) GetStrategy() ParallelStrategy {
	if pt.Strategy == "" {
		return StrategyWaitAll
	}
	return pt.Strategy
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

// -----------------------------------------------------------------------------
// Signal Task
// -----------------------------------------------------------------------------

type SignalTask struct {
	Signal *SignalConfig `json:"signal,omitempty" yaml:"signal,omitempty" mapstructure:"signal,omitempty"`
}

type SignalConfig struct {
	ID      string         `json:"id"                yaml:"id"                mapstructure:"id"`
	Payload map[string]any `json:"payload,omitempty" yaml:"payload,omitempty" mapstructure:"payload,omitempty"`
}

// -----------------------------------------------------------------------------
// Wait Task
// -----------------------------------------------------------------------------

type WaitTask struct {
	WaitFor   string  `json:"wait_for,omitempty"   yaml:"wait_for,omitempty"   mapstructure:"wait_for,omitempty"`
	Processor *Config `json:"processor,omitempty"  yaml:"processor,omitempty"  mapstructure:"processor,omitempty"`
	OnTimeout string  `json:"on_timeout,omitempty" yaml:"on_timeout,omitempty" mapstructure:"on_timeout,omitempty"`
}

// -----------------------------------------------------------------------------
// Memory Task
// -----------------------------------------------------------------------------

type MemoryOpType string

const (
	MemoryOpRead   MemoryOpType = "read"
	MemoryOpWrite  MemoryOpType = "write"
	MemoryOpAppend MemoryOpType = "append"
	MemoryOpDelete MemoryOpType = "delete"
	MemoryOpFlush  MemoryOpType = "flush"
	MemoryOpHealth MemoryOpType = "health"
	MemoryOpClear  MemoryOpType = "clear"
	MemoryOpStats  MemoryOpType = "stats"
)

type MemoryTask struct {
	Operation   MemoryOpType `json:"operation"     yaml:"operation"     mapstructure:"operation"`
	MemoryRef   string       `json:"memory_ref"    yaml:"memory_ref"    mapstructure:"memory_ref"`
	KeyTemplate string       `json:"key_template"  yaml:"key_template"  mapstructure:"key_template"`
	Payload     any          `json:"payload"       yaml:"payload"       mapstructure:"payload,omitempty"`
	// Performance controls
	BatchSize int `json:"batch_size"    yaml:"batch_size"    mapstructure:"batch_size,omitempty"`
	MaxKeys   int `json:"max_keys"      yaml:"max_keys"      mapstructure:"max_keys,omitempty"`
	// Operation-specific configs
	FlushConfig  *FlushConfig  `json:"flush_config"  yaml:"flush_config"  mapstructure:"flush_config,omitempty"`
	HealthConfig *HealthConfig `json:"health_config" yaml:"health_config" mapstructure:"health_config,omitempty"`
	StatsConfig  *StatsConfig  `json:"stats_config"  yaml:"stats_config"  mapstructure:"stats_config,omitempty"`
	ClearConfig  *ClearConfig  `json:"clear_config"  yaml:"clear_config"  mapstructure:"clear_config,omitempty"`
}

type FlushConfig struct {
	Strategy  string  `json:"strategy"  yaml:"strategy"  mapstructure:"strategy"`
	MaxKeys   int     `json:"max_keys"  yaml:"max_keys"  mapstructure:"max_keys"`
	DryRun    bool    `json:"dry_run"   yaml:"dry_run"   mapstructure:"dry_run"`
	Force     bool    `json:"force"     yaml:"force"     mapstructure:"force"`
	Threshold float64 `json:"threshold" yaml:"threshold" mapstructure:"threshold"`
}

type HealthConfig struct {
	IncludeStats      bool `json:"include_stats"      yaml:"include_stats"      mapstructure:"include_stats"`
	CheckConnectivity bool `json:"check_connectivity" yaml:"check_connectivity" mapstructure:"check_connectivity"`
}

type StatsConfig struct {
	IncludeContent bool   `json:"include_content" yaml:"include_content" mapstructure:"include_content"`
	GroupBy        string `json:"group_by"        yaml:"group_by"        mapstructure:"group_by"`
}

type ClearConfig struct {
	Confirm bool `json:"confirm" yaml:"confirm" mapstructure:"confirm"`
	Backup  bool `json:"backup"  yaml:"backup"  mapstructure:"backup"`
}

// -----------------------------------------------------------------------------
// Config
// -----------------------------------------------------------------------------

type Config struct {
	BasicTask        `         json:",inline"        yaml:",inline"        mapstructure:",squash"`
	RouterTask       `         json:",inline"        yaml:",inline"        mapstructure:",squash"`
	ParallelTask     `         json:",inline"        yaml:",inline"        mapstructure:",squash"`
	CollectionConfig `         json:",inline"        yaml:",inline"        mapstructure:",squash"`
	SignalTask       `         json:",inline"        yaml:",inline"        mapstructure:",squash"`
	WaitTask         `         json:",inline"        yaml:",inline"        mapstructure:",squash"`
	MemoryTask       `         json:",inline"        yaml:",inline"        mapstructure:",squash"`
	BaseConfig       `         json:",inline"        yaml:",inline"        mapstructure:",squash"`
	Tasks            []Config `json:"tasks"          yaml:"tasks"          mapstructure:"tasks"`
	Task             *Config  `json:"task,omitempty" yaml:"task,omitempty" mapstructure:"task,omitempty"`
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

// GetMaxWorkers returns the maximum number of workers for the task
func (t *Config) GetMaxWorkers() int {
	return t.MaxWorkers
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
	// Validate wait task specific fields
	if t.Type == TaskTypeWait {
		if err := t.validateWaitTask(); err != nil {
			return fmt.Errorf("invalid wait task '%s': %w", t.ID, err)
		}
	}
	// Validate memory task specific fields
	if t.Type == TaskTypeMemory {
		if err := t.validateMemoryTask(); err != nil {
			return fmt.Errorf("invalid memory task '%s': %w", t.ID, err)
		}
	}
	return nil
}

// validateWaitTask performs comprehensive validation for wait task configuration
func (t *Config) validateWaitTask() error {
	// Required field validation
	if t.WaitFor == "" {
		return fmt.Errorf("wait_for field is required")
	}
	if t.Condition == "" {
		return fmt.Errorf("condition field is required")
	}
	// CEL expression syntax validation
	if err := t.validateWaitCondition(); err != nil {
		return fmt.Errorf("invalid condition: %w", err)
	}
	// Timeout validation
	if err := t.validateWaitTimeout(); err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}
	// Processor configuration validation
	if t.Processor != nil {
		if err := t.validateWaitProcessor(); err != nil {
			return fmt.Errorf("invalid processor configuration: %w", err)
		}
	}
	return nil
}

// validateWaitCondition validates the CEL expression syntax
func (t *Config) validateWaitCondition() error {
	evaluator, err := NewCELEvaluator()
	if err != nil {
		return fmt.Errorf("failed to create CEL evaluator: %w", err)
	}
	return evaluator.ValidateExpression(t.Condition)
}

// validateWaitTimeout validates the timeout value if specified
func (t *Config) validateWaitTimeout() error {
	if t.Timeout == "" {
		return nil // Timeout is optional
	}
	duration, err := core.ParseHumanDuration(t.Timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout format '%s': %w", t.Timeout, err)
	}
	if duration <= 0 {
		return fmt.Errorf("timeout must be positive, got %v", duration)
	}
	return nil
}

// validateWaitProcessor validates the processor configuration
func (t *Config) validateWaitProcessor() error {
	if t.Processor.ID == "" {
		return fmt.Errorf("processor ID is required")
	}
	if t.Processor.Type == "" {
		return fmt.Errorf("processor type is required")
	}
	// Recursively validate the processor configuration
	if err := t.Processor.Validate(); err != nil {
		return fmt.Errorf("processor validation failed: %w", err)
	}
	return nil
}

// validateMemoryTask performs comprehensive validation for memory task configuration
func (t *Config) validateMemoryTask() error {
	// Required field validation
	if err := t.validateMemoryRequiredFields(); err != nil {
		return err
	}
	// Validate operation type
	if err := t.validateMemoryOperation(); err != nil {
		return err
	}
	// Validate performance limits
	if err := t.validateMemoryLimits(); err != nil {
		return err
	}
	// Operation-specific validation
	return t.validateMemoryOperationSpecific()
}

func (t *Config) validateMemoryRequiredFields() error {
	if t.Operation == "" {
		return fmt.Errorf("operation field is required")
	}
	if t.MemoryRef == "" {
		return fmt.Errorf("memory_ref field is required")
	}
	if t.KeyTemplate == "" {
		return fmt.Errorf("key_template field is required")
	}
	return nil
}

func (t *Config) validateMemoryOperation() error {
	switch t.Operation {
	case MemoryOpRead, MemoryOpWrite, MemoryOpAppend, MemoryOpDelete,
		MemoryOpFlush, MemoryOpHealth, MemoryOpClear, MemoryOpStats:
		return nil
	default:
		return fmt.Errorf(
			"invalid operation '%s', must be one of: read, write, append, delete, flush, health, clear, stats",
			t.Operation,
		)
	}
}

func (t *Config) validateMemoryLimits() error {
	if t.MaxKeys < 0 {
		return fmt.Errorf("max_keys cannot be negative")
	}
	if t.MaxKeys > 50000 {
		return fmt.Errorf("max_keys cannot exceed 50,000 for safety")
	}
	if t.BatchSize < 0 {
		return fmt.Errorf("batch_size cannot be negative")
	}
	if t.BatchSize > 10000 {
		return fmt.Errorf("batch_size cannot exceed 10,000 for safety")
	}
	return nil
}

func (t *Config) validateMemoryOperationSpecific() error {
	switch t.Operation {
	case MemoryOpWrite, MemoryOpAppend:
		if t.Payload == nil {
			return fmt.Errorf("%s operation requires payload", t.Operation)
		}
	case MemoryOpFlush:
		if t.FlushConfig != nil {
			if t.FlushConfig.MaxKeys < 0 {
				return fmt.Errorf("flush max_keys cannot be negative")
			}
			if t.FlushConfig.Threshold < 0 || t.FlushConfig.Threshold > 1 {
				return fmt.Errorf("flush threshold must be between 0 and 1")
			}
		}
	case MemoryOpClear:
		if t.ClearConfig == nil || !t.ClearConfig.Confirm {
			return fmt.Errorf("clear operation requires confirm flag to be true")
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

// GetStrategy returns the strategy for the task based on its type
func (t *Config) GetStrategy() ParallelStrategy {
	switch t.Type {
	case TaskTypeParallel:
		// Use the embedded ParallelTask's GetStrategy method
		return t.ParallelTask.GetStrategy()
	case TaskTypeCollection:
		// Collections can have a strategy defined via the embedded ParallelTask
		return t.ParallelTask.GetStrategy()
	case TaskTypeComposite:
		// Composite tasks are always sequential (WaitAll)
		return StrategyWaitAll
	default:
		// Other task types don't have a strategy concept, default to WaitAll
		return StrategyWaitAll
	}
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
	case TaskTypeComposite:
		executionType = ExecutionComposite
	case TaskTypeAggregate:
		executionType = ExecutionBasic
	case TaskTypeSignal:
		executionType = ExecutionBasic
	case TaskTypeWait:
		executionType = ExecutionWait
	case TaskTypeMemory:
		executionType = ExecutionBasic
	default:
		executionType = ExecutionBasic
	}
	return executionType
}

func (t *Config) Clone() (*Config, error) {
	if t == nil {
		return nil, nil
	}
	return core.DeepCopy(t)
}

func FindConfig(tasks []Config, taskID string) (*Config, error) {
	for i := range tasks {
		if tasks[i].ID == taskID {
			return &tasks[i], nil
		}
	}
	availableIDs := make([]string, len(tasks))
	for i := range tasks {
		availableIDs[i] = tasks[i].ID
	}
	return nil, fmt.Errorf("task not found: searched for '%s', available tasks: %v", taskID, availableIDs)
}

func applyDefaults(config *Config) {
	// Apply defaults for collection tasks
	if config.Type == TaskTypeCollection {
		config.Default()
	}
	// Recursively apply defaults to sub-tasks for any task type with a tasks array
	hasSubTasks := config.Type == TaskTypeParallel ||
		config.Type == TaskTypeComposite ||
		config.Type == TaskTypeCollection
	if hasSubTasks && len(config.Tasks) > 0 {
		for i := range config.Tasks {
			applyDefaults(&config.Tasks[i])
		}
	}

	// Handle collection tasks with task template
	if config.Type == TaskTypeCollection && config.Task != nil {
		applyDefaults(config.Task)
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

// PropagateTaskListCWD propagates CWD to a list of tasks
func PropagateTaskListCWD(tasks []Config, parentCWD *core.PathCWD, taskType string) error {
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

// PropagateSingleTaskCWD propagates CWD to a single task
func PropagateSingleTaskCWD(task *Config, parentCWD *core.PathCWD, taskType string) error {
	if err := setCWDForTask(task, parentCWD, taskType); err != nil {
		return err
	}
	return propagateCWDToSubTasks(task)
}

func propagateCWDToSubTasks(config *Config) error {
	switch config.Type {
	case TaskTypeParallel, TaskTypeComposite:
		if len(config.Tasks) > 0 {
			return PropagateTaskListCWD(config.Tasks, config.CWD, "sub-task")
		}
	case TaskTypeCollection:
		if config.Task != nil {
			if err := PropagateSingleTaskCWD(config.Task, config.CWD, "collection task template"); err != nil {
				return err
			}
		}
		if len(config.Tasks) > 0 {
			return PropagateTaskListCWD(config.Tasks, config.CWD, "collection task")
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
