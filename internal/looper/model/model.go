package model

import "time"

const (
	UnknownFileName        = "unknown"
	IDECodex               = "codex"
	IDEClaude              = "claude"
	IDEDroid               = "droid"
	IDECursor              = "cursor-agent"
	DefaultCodexModel      = "gpt-5.4"
	DefaultClaudeModel     = "opus"
	DefaultCursorModel     = "composer-1"
	DefaultActivityTimeout = 10 * time.Minute
	ModeCodeReview         = "pr-review"
	ModePRDTasks           = "prd-tasks"
)

type ExecutionMode string

const (
	ExecutionModePRReview ExecutionMode = ModeCodeReview
	ExecutionModePRDTasks ExecutionMode = ModePRDTasks
)

type RuntimeConfig struct {
	PR                     string
	IssuesDir              string
	DryRun                 bool
	AutoCommit             bool
	Concurrent             int
	BatchSize              int
	IDE                    string
	Model                  string
	AddDirs                []string
	Grouped                bool
	TailLines              int
	ReasoningEffort        string
	Mode                   ExecutionMode
	IncludeCompleted       bool
	Timeout                time.Duration
	MaxRetries             int
	RetryBackoffMultiplier float64
}

func (cfg *RuntimeConfig) ApplyDefaults() {
	if cfg.Concurrent <= 0 {
		cfg.Concurrent = 1
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 1
	}
	if cfg.IDE == "" {
		cfg.IDE = IDECodex
	}
	if cfg.TailLines <= 0 {
		cfg.TailLines = 30
	}
	if cfg.ReasoningEffort == "" {
		cfg.ReasoningEffort = "medium"
	}
	if cfg.Mode == "" {
		cfg.Mode = ExecutionModePRReview
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultActivityTimeout
	}
	if cfg.RetryBackoffMultiplier <= 0 {
		cfg.RetryBackoffMultiplier = 1.5
	}
}

type IssueEntry struct {
	Name     string
	AbsPath  string
	Content  string
	CodeFile string
}

type TaskEntry struct {
	Content      string
	Status       string
	Domain       string
	TaskType     string
	Scope        string
	Complexity   string
	Dependencies []string
}

type SolvePreparation struct {
	Jobs              []Job
	IssuesDir         string
	ResolvedPR        string
	IssuesDirPath     string
	GroupedSummarized bool
}

type Job struct {
	CodeFiles     []string
	Groups        map[string][]IssueEntry
	SafeName      string
	Prompt        []byte
	OutPromptPath string
	OutLog        string
	ErrLog        string
}

func (j Job) IssueCount() int {
	total := 0
	for _, items := range j.Groups {
		total += len(items)
	}
	return total
}
