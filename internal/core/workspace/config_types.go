package workspace

type Context struct {
	Root       string
	CompozyDir string
	ConfigPath string
	Config     ProjectConfig
}

type ProjectConfig struct {
	Defaults     DefaultsConfig     `toml:"defaults"`
	Start        StartConfig        `toml:"start"`
	Tasks        TasksConfig        `toml:"tasks"`
	FixReviews   FixReviewsConfig   `toml:"fix_reviews"`
	FetchReviews FetchReviewsConfig `toml:"fetch_reviews"`
	Exec         ExecConfig         `toml:"exec"`
}

type RuntimeOverrides struct {
	IDE                    *string   `toml:"ide"`
	Model                  *string   `toml:"model"`
	OutputFormat           *string   `toml:"output_format"`
	ReasoningEffort        *string   `toml:"reasoning_effort"`
	AccessMode             *string   `toml:"access_mode"`
	Timeout                *string   `toml:"timeout"`
	TailLines              *int      `toml:"tail_lines"`
	AddDirs                *[]string `toml:"add_dirs"`
	AutoCommit             *bool     `toml:"auto_commit"`
	MaxRetries             *int      `toml:"max_retries"`
	RetryBackoffMultiplier *float64  `toml:"retry_backoff_multiplier"`
}

type DefaultsConfig RuntimeOverrides

type StartConfig struct {
	IncludeCompleted *bool   `toml:"include_completed"`
	OutputFormat     *string `toml:"output_format"`
	TUI              *bool   `toml:"tui"`
}

type TasksConfig struct {
	Types *[]string `toml:"types"`
}

type FixReviewsConfig struct {
	Concurrent      *int    `toml:"concurrent"`
	BatchSize       *int    `toml:"batch_size"`
	IncludeResolved *bool   `toml:"include_resolved"`
	OutputFormat    *string `toml:"output_format"`
	TUI             *bool   `toml:"tui"`
}

type FetchReviewsConfig struct {
	Provider *string `toml:"provider"`
	Nitpicks *bool   `toml:"nitpicks"`
}

type ExecConfig struct {
	RuntimeOverrides
	Verbose *bool `toml:"verbose"`
	TUI     *bool `toml:"tui"`
	Persist *bool `toml:"persist"`
}
