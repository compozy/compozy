package runshared

import (
	"strings"
	"time"

	"github.com/compozy/compozy/internal/core/model"
)

type Config struct {
	WorkspaceRoot          string
	Name                   string
	Round                  int
	Provider               string
	PR                     string
	ReviewsDir             string
	TasksDir               string
	DryRun                 bool
	AutoCommit             bool
	CloseOnComplete        bool
	Concurrent             int
	BatchSize              int
	IDE                    string
	Model                  string
	AddDirs                []string
	TailLines              int
	ReasoningEffort        string
	AccessMode             string
	Mode                   model.ExecutionMode
	OutputFormat           model.OutputFormat
	Verbose                bool
	TUI                    bool
	Persist                bool
	RunID                  string
	RunArtifacts           model.RunArtifacts
	IncludeCompleted       bool
	IncludeResolved        bool
	Timeout                time.Duration
	MaxRetries             int
	RetryBackoffMultiplier float64
}

type Job struct {
	CodeFiles     []string
	Groups        map[string][]model.IssueEntry
	TaskTitle     string
	TaskType      string
	SafeName      string
	Prompt        []byte
	SystemPrompt  string
	ResumeRunID   string
	ResumeSession string
	OutPromptPath string
	OutLog        string
	ErrLog        string
	Status        string
	Failure       string
	ExitCode      int
	Usage         model.Usage
	OutBuffer     *LineBuffer
	ErrBuffer     *LineBuffer
}

func (j Job) CodeFileLabel() string {
	return strings.Join(j.CodeFiles, ", ")
}

func (cfg *Config) HumanOutputEnabled() bool {
	return cfg != nil && (cfg.OutputFormat == "" || cfg.OutputFormat == model.OutputFormatText)
}

func (cfg *Config) UIEnabled() bool {
	return cfg != nil && cfg.HumanOutputEnabled() && !cfg.DryRun
}

func NewConfig(src *model.RuntimeConfig, runArtifacts model.RunArtifacts) *Config {
	if src == nil {
		return nil
	}
	return &Config{
		WorkspaceRoot:          src.WorkspaceRoot,
		Name:                   src.Name,
		Round:                  src.Round,
		Provider:               src.Provider,
		PR:                     src.PR,
		ReviewsDir:             src.ReviewsDir,
		TasksDir:               src.TasksDir,
		DryRun:                 src.DryRun,
		AutoCommit:             src.AutoCommit,
		CloseOnComplete:        src.CloseOnComplete,
		Concurrent:             src.Concurrent,
		BatchSize:              src.BatchSize,
		IDE:                    src.IDE,
		Model:                  src.Model,
		AddDirs:                append([]string(nil), src.AddDirs...),
		TailLines:              src.TailLines,
		ReasoningEffort:        src.ReasoningEffort,
		AccessMode:             src.AccessMode,
		Mode:                   src.Mode,
		OutputFormat:           src.OutputFormat,
		Verbose:                src.Verbose,
		TUI:                    src.TUI,
		Persist:                src.Persist,
		RunID:                  src.RunID,
		RunArtifacts:           runArtifacts,
		IncludeCompleted:       src.IncludeCompleted,
		IncludeResolved:        src.IncludeResolved,
		Timeout:                src.Timeout,
		MaxRetries:             src.MaxRetries,
		RetryBackoffMultiplier: src.RetryBackoffMultiplier,
	}
}

func NewJobs(src []model.Job) []Job {
	jobs := make([]Job, 0, len(src))
	for i := range src {
		item := &src[i]
		jobs = append(jobs, Job{
			CodeFiles:     append([]string(nil), item.CodeFiles...),
			Groups:        CloneGroups(item.Groups),
			TaskTitle:     item.TaskTitle,
			TaskType:      item.TaskType,
			SafeName:      item.SafeName,
			Prompt:        append([]byte(nil), item.Prompt...),
			SystemPrompt:  item.SystemPrompt,
			OutPromptPath: item.OutPromptPath,
			OutLog:        item.OutLog,
			ErrLog:        item.ErrLog,
		})
	}
	return jobs
}

func CloneGroups(src map[string][]model.IssueEntry) map[string][]model.IssueEntry {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string][]model.IssueEntry, len(src))
	for key, entries := range src {
		items := make([]model.IssueEntry, len(entries))
		copy(items, entries)
		cloned[key] = items
	}
	return cloned
}

func CountTotalIssues(job *Job) int {
	if job == nil {
		return 0
	}
	total := 0
	for _, items := range job.Groups {
		total += len(items)
	}
	return total
}
