package model

import (
	"path/filepath"
	"time"
)

const (
	UnknownFileName        = "unknown"
	IDECodex               = "codex"
	IDEClaude              = "claude"
	IDEDroid               = "droid"
	IDECursor              = "cursor-agent"
	IDEOpenCode            = "opencode"
	IDEPi                  = "pi"
	DefaultCodexModel      = "gpt-5.4"
	DefaultClaudeModel     = "opus"
	DefaultCursorModel     = "composer-1"
	DefaultOpenCodeModel   = "anthropic/claude-opus-4-6"
	DefaultPiModel         = "anthropic/claude-opus-4-6"
	DefaultActivityTimeout = 10 * time.Minute
	WorkflowRootDirName    = ".compozy"
	WorkflowTasksDirName   = "tasks"
	ModeCodeReview         = "pr-review"
	ModePRDTasks           = "prd-tasks"
)

type ExecutionMode string

const (
	ExecutionModePRReview ExecutionMode = ModeCodeReview
	ExecutionModePRDTasks ExecutionMode = ModePRDTasks
)

type RuntimeConfig struct {
	Name                   string
	Round                  int
	Provider               string
	PR                     string
	ReviewsDir             string
	TasksDir               string
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
	IncludeResolved        bool
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

func TasksBaseDir() string {
	return filepath.Join(WorkflowRootDirName, WorkflowTasksDirName)
}

func TaskDirectory(name string) string {
	return filepath.Join(TasksBaseDir(), name)
}

type IssueEntry struct {
	Name     string
	AbsPath  string
	Content  string
	CodeFile string
}

type ReviewContext struct {
	Status      string
	File        string
	Line        int
	Severity    string
	Author      string
	ProviderRef string
}

type RoundMeta struct {
	Provider   string
	PR         string
	Round      int
	CreatedAt  time.Time
	Total      int
	Resolved   int
	Unresolved int
}

type TaskMeta struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	Total     int
	Completed int
	Pending   int
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

type TaskFileMeta struct {
	Status       string   `yaml:"status"`
	Domain       string   `yaml:"domain,omitempty"`
	TaskType     string   `yaml:"type,omitempty"`
	Scope        string   `yaml:"scope,omitempty"`
	Complexity   string   `yaml:"complexity,omitempty"`
	Dependencies []string `yaml:"dependencies,omitempty"`
}

type ReviewFileMeta struct {
	Status      string `yaml:"status"`
	File        string `yaml:"file,omitempty"`
	Line        int    `yaml:"line,omitempty"`
	Severity    string `yaml:"severity,omitempty"`
	Author      string `yaml:"author,omitempty"`
	ProviderRef string `yaml:"provider_ref,omitempty"`
}

type SolvePreparation struct {
	Jobs              []Job
	InputDir          string
	InputDirPath      string
	ResolvedName      string
	ResolvedPR        string
	ResolvedProvider  string
	ResolvedRound     int
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
