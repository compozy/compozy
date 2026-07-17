package workspace

import (
	"strings"

	"github.com/compozy/compozy/internal/core/model"
)

const (
	TaskRunMultipleModeEnqueued = "enqueued"
	TaskRunMultipleModeParallel = "parallel"

	// DefaultRunMultipleParallelLimit is the effective parallel multi-run fanout
	// limit applied when run_multiple_parallel_limit is unset.
	DefaultRunMultipleParallelLimit = 2

	DefaultRecoveryEnabled         = false
	DefaultRecoveryIDE             = model.IDECodex
	DefaultRecoveryModel           = model.DefaultCodexModel
	DefaultRecoveryReasoningEffort = "medium"
	DefaultRecoveryMaxAttempts     = 1
	MaxRecoveryAttempts            = 3

	DefaultParallelTasksEnabled        = false
	DefaultParallelTasksMaxConcurrency = 4
)

type Context struct {
	Root                string
	CompozyDir          string
	ConfigPath          string
	WorkspaceConfigPath string
	GlobalConfigPath    string
	Config              ProjectConfig
}

type ProjectConfig struct {
	Defaults     DefaultsConfig      `toml:"defaults,omitempty"`
	Tasks        TasksConfig         `toml:"tasks,omitempty"`
	FixReviews   FixReviewsConfig    `toml:"fix_reviews,omitempty"`
	FetchReviews FetchReviewsConfig  `toml:"fetch_reviews,omitempty"`
	WatchReviews WatchReviewsConfig  `toml:"watch_reviews,omitempty"`
	Exec         ExecConfig          `toml:"exec,omitempty"`
	Runs         RunsConfig          `toml:"runs,omitempty"`
	Recovery     AgentRecoveryConfig `toml:"recovery,omitempty"`
	Sound        SoundConfig         `toml:"sound,omitempty"`
}

type RuntimeOverrides struct {
	IDE                    *string        `toml:"ide,omitempty"`
	Model                  *string        `toml:"model,omitempty"`
	OutputFormat           *string        `toml:"output_format,omitempty"`
	ReasoningEffort        *string        `toml:"reasoning_effort,omitempty"`
	AccessMode             *string        `toml:"access_mode,omitempty"`
	Timeout                *string        `toml:"timeout,omitempty"`
	TailLines              *int           `toml:"tail_lines,omitempty"`
	AddDirs                *[]string      `toml:"add_dirs,omitempty"`
	AutoCommit             *bool          `toml:"auto_commit,omitempty"`
	MaxRetries             *int           `toml:"max_retries,omitempty"`
	RetryBackoffMultiplier *float64       `toml:"retry_backoff_multiplier,omitempty"`
	Stall                  StallOverrides `toml:"stall,omitempty"`
}

// StallOverrides are the optional TOML knobs for stall detection and recovery.
// Durations are parsed from Go duration strings (e.g. "3m") consistent with the
// existing timeout override handling. All fields are resolved into
// model.RuntimeConfig, where defaults and the child > idle invariant are applied.
type StallOverrides struct {
	Enabled                *bool   `toml:"enabled,omitempty"                  json:"enabled,omitempty"`
	Timeout                *string `toml:"timeout,omitempty"                  json:"timeout,omitempty"`
	ChildTimeout           *string `toml:"child_timeout,omitempty"            json:"child_timeout,omitempty"`
	TerminalCommandTimeout *string `toml:"terminal_command_timeout,omitempty" json:"terminal_command_timeout,omitempty"`
	Retries                *int    `toml:"retries,omitempty"                  json:"retries,omitempty"`
}

// TaskRuntimeOverrides are the runtime fields that may vary with task
// complexity. Execution-wide fields stay in RuntimeOverrides.
type TaskRuntimeOverrides struct {
	IDE             *string `toml:"ide,omitempty"`
	Model           *string `toml:"model,omitempty"`
	ReasoningEffort *string `toml:"reasoning_effort,omitempty"`
}

// TaskRuntimeByComplexityConfig declares optional runtime defaults for the four
// task complexities accepted by the v2 task schema.
type TaskRuntimeByComplexityConfig struct {
	Low      TaskRuntimeOverrides `toml:"low,omitempty"`
	Medium   TaskRuntimeOverrides `toml:"medium,omitempty"`
	High     TaskRuntimeOverrides `toml:"high,omitempty"`
	Critical TaskRuntimeOverrides `toml:"critical,omitempty"`
}

type DefaultsConfig struct {
	RuntimeOverrides
	ByComplexity TaskRuntimeByComplexityConfig `toml:"by_complexity,omitempty"`
}

// ComplexityRuntimeRules converts defaults into the shared task-runtime rule
// representation used by planning. The stable order is the task schema order.
func (cfg DefaultsConfig) ComplexityRuntimeRules() []model.TaskRuntimeRule {
	entries := []struct {
		complexity string
		overrides  TaskRuntimeOverrides
	}{
		{complexity: "low", overrides: cfg.ByComplexity.Low},
		{complexity: "medium", overrides: cfg.ByComplexity.Medium},
		{complexity: "high", overrides: cfg.ByComplexity.High},
		{complexity: "critical", overrides: cfg.ByComplexity.Critical},
	}
	rules := make([]model.TaskRuntimeRule, 0, len(entries))
	for _, entry := range entries {
		complexity := entry.complexity
		rule := model.TaskRuntimeRule{
			Complexity:      &complexity,
			IDE:             entry.overrides.IDE,
			Model:           entry.overrides.Model,
			ReasoningEffort: entry.overrides.ReasoningEffort,
		}
		if rule.HasOverride() {
			rules = append(rules, rule)
		}
	}
	return model.CloneTaskRuntimeRules(rules)
}

type TaskRunConfig struct {
	IncludeCompleted         *bool                    `toml:"include_completed"`
	Recursive                *bool                    `toml:"recursive"`
	OutputFormat             *string                  `toml:"output_format"`
	RunMultipleMode          *string                  `toml:"run_multiple_mode"`
	RunMultipleParallelLimit *int                     `toml:"run_multiple_parallel_limit"`
	Parallel                 ParallelTasksConfig      `toml:"parallel"`
	TUI                      *bool                    `toml:"tui"`
	TaskRuntimeRules         *[]model.TaskRuntimeRule `toml:"task_runtime_rules"`
}

func (cfg TaskRunConfig) EffectiveRunMultipleMode() string {
	if cfg.RunMultipleMode == nil {
		return TaskRunMultipleModeEnqueued
	}
	mode := strings.TrimSpace(*cfg.RunMultipleMode)
	if mode == "" {
		return TaskRunMultipleModeEnqueued
	}
	return mode
}

// EffectiveRunMultipleParallelLimit returns the configured parallel multi-run
// fanout limit, defaulting to DefaultRunMultipleParallelLimit when unset.
func (cfg TaskRunConfig) EffectiveRunMultipleParallelLimit() int {
	if cfg.RunMultipleParallelLimit == nil {
		return DefaultRunMultipleParallelLimit
	}
	return *cfg.RunMultipleParallelLimit
}

type ParallelTasksConfig struct {
	Enabled          *bool                   `toml:"enabled"           json:"enabled,omitempty"`
	MaxConcurrency   *int                    `toml:"max_concurrency"   json:"max_concurrency,omitempty"`
	ConflictResolver *ConflictResolverConfig `toml:"conflict_resolver" json:"conflict_resolver,omitempty"`
}

func DefaultParallelTasksConfig() ParallelTasksConfig {
	enabled := DefaultParallelTasksEnabled
	maxConcurrency := DefaultParallelTasksMaxConcurrency
	conflictResolver := DefaultConflictResolverConfig()
	return ParallelTasksConfig{
		Enabled:          &enabled,
		MaxConcurrency:   &maxConcurrency,
		ConflictResolver: &conflictResolver,
	}
}

func (cfg ParallelTasksConfig) ApplyDefaults() ParallelTasksConfig {
	defaults := DefaultParallelTasksConfig()
	if cfg.Enabled == nil {
		cfg.Enabled = defaults.Enabled
	}
	if cfg.MaxConcurrency == nil {
		cfg.MaxConcurrency = defaults.MaxConcurrency
	}
	if cfg.ConflictResolver == nil {
		cfg.ConflictResolver = defaults.ConflictResolver
		return cfg
	}
	conflictResolver := cfg.ConflictResolver.ApplyDefaults()
	cfg.ConflictResolver = &conflictResolver
	return cfg
}

type TasksConfig struct {
	Types *[]string     `toml:"types,omitempty"`
	Run   TaskRunConfig `toml:"run,omitempty"`
}

type FixReviewsConfig struct {
	Concurrent      *int    `toml:"concurrent"`
	BatchSize       *int    `toml:"batch_size"`
	IncludeResolved *bool   `toml:"include_resolved"`
	OutputFormat    *string `toml:"output_format"`
	TUI             *bool   `toml:"tui"`
}

type FetchReviewsConfig struct {
	Provider *string `toml:"provider,omitempty"`
	Nitpicks *bool   `toml:"nitpicks,omitempty"`
}

type WatchReviewsConfig struct {
	MaxRounds     *int    `toml:"max_rounds"`
	PollInterval  *string `toml:"poll_interval"`
	ReviewTimeout *string `toml:"review_timeout"`
	QuietPeriod   *string `toml:"quiet_period"`
	AutoPush      *bool   `toml:"auto_push"`
	UntilClean    *bool   `toml:"until_clean"`
	PushRemote    *string `toml:"push_remote"`
	PushBranch    *string `toml:"push_branch"`
}

type ExecConfig struct {
	RuntimeOverrides
	Verbose *bool `toml:"verbose,omitempty"`
	TUI     *bool `toml:"tui,omitempty"`
	Persist *bool `toml:"persist,omitempty"`
}

type RunsConfig struct {
	DefaultAttachMode    *string `toml:"default_attach_mode"`
	KeepTerminalDays     *int    `toml:"keep_terminal_days"`
	KeepMax              *int    `toml:"keep_max"`
	ShutdownDrainTimeout *string `toml:"shutdown_drain_timeout"`
}

type AgentRecoveryConfig struct {
	Enabled         *bool   `toml:"enabled"          json:"enabled,omitempty"`
	IDE             *string `toml:"ide"              json:"ide,omitempty"`
	Model           *string `toml:"model"            json:"model,omitempty"`
	ReasoningEffort *string `toml:"reasoning_effort" json:"reasoning_effort,omitempty"`
	MaxAttempts     *int    `toml:"max_attempts"     json:"max_attempts,omitempty"`
}

type ConflictResolverConfig struct {
	Enabled           *bool     `toml:"enabled"            json:"enabled,omitempty"`
	IDE               *string   `toml:"ide"                json:"ide,omitempty"`
	Model             *string   `toml:"model"              json:"model,omitempty"`
	ReasoningEffort   *string   `toml:"reasoning_effort"   json:"reasoning_effort,omitempty"`
	MaxAttempts       *int      `toml:"max_attempts"       json:"max_attempts,omitempty"`
	ValidationCommand *[]string `toml:"validation_command" json:"validation_command,omitempty"`
}

func DefaultAgentRecoveryConfig() AgentRecoveryConfig {
	enabled := DefaultRecoveryEnabled
	ide := DefaultRecoveryIDE
	modelName := DefaultRecoveryModel
	reasoningEffort := DefaultRecoveryReasoningEffort
	maxAttempts := DefaultRecoveryMaxAttempts
	return AgentRecoveryConfig{
		Enabled:         &enabled,
		IDE:             &ide,
		Model:           &modelName,
		ReasoningEffort: &reasoningEffort,
		MaxAttempts:     &maxAttempts,
	}
}

func DefaultConflictResolverConfig() ConflictResolverConfig {
	recovery := DefaultAgentRecoveryConfig()
	return ConflictResolverConfig{
		Enabled:         recovery.Enabled,
		IDE:             recovery.IDE,
		Model:           recovery.Model,
		ReasoningEffort: recovery.ReasoningEffort,
		MaxAttempts:     recovery.MaxAttempts,
	}
}

func (cfg AgentRecoveryConfig) ApplyDefaults() AgentRecoveryConfig {
	defaults := DefaultAgentRecoveryConfig()
	if cfg.Enabled == nil {
		cfg.Enabled = defaults.Enabled
	}
	if cfg.IDE == nil {
		cfg.IDE = defaults.IDE
	}
	if cfg.Model == nil {
		cfg.Model = defaults.Model
	}
	if cfg.ReasoningEffort == nil {
		cfg.ReasoningEffort = defaults.ReasoningEffort
	}
	if cfg.MaxAttempts == nil {
		cfg.MaxAttempts = defaults.MaxAttempts
	}
	return cfg
}

func (cfg ConflictResolverConfig) ApplyDefaults() ConflictResolverConfig {
	defaults := DefaultConflictResolverConfig()
	if cfg.Enabled == nil {
		cfg.Enabled = defaults.Enabled
	}
	if cfg.IDE == nil {
		cfg.IDE = defaults.IDE
	}
	if cfg.Model == nil {
		cfg.Model = defaults.Model
	}
	if cfg.ReasoningEffort == nil {
		cfg.ReasoningEffort = defaults.ReasoningEffort
	}
	if cfg.MaxAttempts == nil {
		cfg.MaxAttempts = defaults.MaxAttempts
	}
	cfg.ValidationCommand = cloneStringSlicePointer(cfg.ValidationCommand)
	return cfg
}

func ValidateAgentRecoveryConfig(scope string, cfg AgentRecoveryConfig) error {
	return validateRecovery(scope, cfg)
}

func ValidateConflictResolverConfig(scope string, cfg ConflictResolverConfig) error {
	return validateConflictResolverConfig(scope, "conflict_resolver", cfg)
}

// SoundConfig controls optional audio notifications on run lifecycle events.
// Disabled by default; opt-in via `[sound] enabled = true` in .compozy/config.toml.
type SoundConfig struct {
	Enabled     *bool   `toml:"enabled"`
	OnCompleted *string `toml:"on_completed"`
	OnFailed    *string `toml:"on_failed"`
	OnParked    *string `toml:"on_parked"`
}
