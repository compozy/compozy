package workspace

import "github.com/compozy/compozy/internal/core/model"

type Context struct {
	Root                string
	CompozyDir          string
	ConfigPath          string
	WorkspaceConfigPath string
	GlobalConfigPath    string
	Config              ProjectConfig
}

type ProjectConfig struct {
	Defaults     DefaultsConfig     `toml:"defaults,omitempty"`
	Start        StartConfig        `toml:"start,omitempty"`
	Tasks        TasksConfig        `toml:"tasks,omitempty"`
	FixReviews   FixReviewsConfig   `toml:"fix_reviews,omitempty"`
	FetchReviews FetchReviewsConfig `toml:"fetch_reviews,omitempty"`
	Exec         ExecConfig         `toml:"exec,omitempty"`
	Runs         RunsConfig         `toml:"runs,omitempty"`
	Sound        SoundConfig        `toml:"sound,omitempty"`
}

type RuntimeOverrides struct {
	IDE                    *string   `toml:"ide,omitempty"`
	Model                  *string   `toml:"model,omitempty"`
	OutputFormat           *string   `toml:"output_format,omitempty"`
	ReasoningEffort        *string   `toml:"reasoning_effort,omitempty"`
	AccessMode             *string   `toml:"access_mode,omitempty"`
	Timeout                *string   `toml:"timeout,omitempty"`
	TailLines              *int      `toml:"tail_lines,omitempty"`
	AddDirs                *[]string `toml:"add_dirs,omitempty"`
	AutoCommit             *bool     `toml:"auto_commit,omitempty"`
	MaxRetries             *int      `toml:"max_retries,omitempty"`
	RetryBackoffMultiplier *float64  `toml:"retry_backoff_multiplier,omitempty"`
}

type DefaultsConfig RuntimeOverrides

type StartConfig struct {
	IncludeCompleted *bool                    `toml:"include_completed"`
	OutputFormat     *string                  `toml:"output_format"`
	TUI              *bool                    `toml:"tui"`
	TaskRuntimeRules *[]model.TaskRuntimeRule `toml:"task_runtime_rules"`
}

type TasksConfig struct {
	Types *[]string `toml:"types,omitempty"`
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

// SoundConfig controls optional audio notifications on run lifecycle events.
// Disabled by default; opt-in via `[sound] enabled = true` in .compozy/config.toml.
type SoundConfig struct {
	Enabled     *bool   `toml:"enabled"`
	OnCompleted *string `toml:"on_completed"`
	OnFailed    *string `toml:"on_failed"`
}
