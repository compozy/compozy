package model

import (
	"path/filepath"
	"strings"
	"time"
)

const (
	UnknownFileName         = "unknown"
	IDECodex                = "codex"
	IDEClaude               = "claude"
	IDEDroid                = "droid"
	IDECursor               = "cursor-agent"
	IDEOpenCode             = "opencode"
	IDEPi                   = "pi"
	IDEGemini               = "gemini"
	DefaultCodexModel       = "gpt-5.4"
	DefaultClaudeModel      = "opus"
	DefaultCursorModel      = "composer-1"
	DefaultOpenCodeModel    = "anthropic/claude-opus-4-6"
	DefaultPiModel          = "anthropic/claude-opus-4-6"
	DefaultGeminiModel      = "gemini-2.5-pro"
	DefaultActivityTimeout  = 10 * time.Minute
	WorkflowRootDirName     = ".compozy"
	WorkflowConfigFileName  = "config.toml"
	WorkflowTasksDirName    = "tasks"
	ArchivedWorkflowDirName = "_archived"
	ModeCodeReview          = "pr-review"
	ModePRDTasks            = "prd-tasks"
)

type ExecutionMode string

const (
	ExecutionModePRReview ExecutionMode = ModeCodeReview
	ExecutionModePRDTasks ExecutionMode = ModePRDTasks
)

type RuntimeConfig struct {
	WorkspaceRoot          string
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
	SystemPrompt           string
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
	if cfg.TailLines < 0 {
		cfg.TailLines = 0
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
	return TasksBaseDirForWorkspace("")
}

func TaskDirectory(name string) string {
	return TaskDirectoryForWorkspace("", name)
}

func CompozyDir(workspaceRoot string) string {
	trimmed := strings.TrimSpace(workspaceRoot)
	if trimmed == "" {
		return WorkflowRootDirName
	}
	return filepath.Join(filepath.Clean(trimmed), WorkflowRootDirName)
}

func ConfigPathForWorkspace(workspaceRoot string) string {
	return filepath.Join(CompozyDir(workspaceRoot), WorkflowConfigFileName)
}

func TasksBaseDirForWorkspace(workspaceRoot string) string {
	return filepath.Join(CompozyDir(workspaceRoot), WorkflowTasksDirName)
}

func TaskDirectoryForWorkspace(workspaceRoot, name string) string {
	return filepath.Join(TasksBaseDirForWorkspace(workspaceRoot), name)
}

func ArchivedTasksDir(baseDir string) string {
	return filepath.Join(baseDir, ArchivedWorkflowDirName)
}

func IsActiveWorkflowDirName(name string) bool {
	trimmed := strings.TrimSpace(name)
	return trimmed != "" && !strings.HasPrefix(trimmed, ".") && trimmed != ArchivedWorkflowDirName
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
	SystemPrompt  string
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
