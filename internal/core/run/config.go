package run

import (
	"strings"
	"time"

	"github.com/compozy/compozy/internal/core/model"
)

type config struct {
	workspaceRoot          string
	name                   string
	round                  int
	provider               string
	pr                     string
	reviewsDir             string
	tasksDir               string
	dryRun                 bool
	autoCommit             bool
	concurrent             int
	batchSize              int
	ide                    string
	model                  string
	addDirs                []string
	tailLines              int
	reasoningEffort        string
	accessMode             string
	mode                   model.ExecutionMode
	outputFormat           model.OutputFormat
	verbose                bool
	tui                    bool
	persist                bool
	runID                  string
	runArtifacts           model.RunArtifacts
	includeCompleted       bool
	includeResolved        bool
	timeout                time.Duration
	maxRetries             int
	retryBackoffMultiplier float64
}

type job struct {
	codeFiles     []string
	groups        map[string][]model.IssueEntry
	taskTitle     string
	taskType      string
	safeName      string
	prompt        []byte
	systemPrompt  string
	resumeRunID   string
	resumeSession string
	outPromptPath string
	outLog        string
	errLog        string
	status        string
	failure       string
	exitCode      int
	usage         model.Usage
	outBuffer     *lineBuffer
	errBuffer     *lineBuffer
}

func (j job) codeFileLabel() string {
	return strings.Join(j.codeFiles, ", ")
}

func (cfg *config) humanOutputEnabled() bool {
	return cfg != nil && (cfg.outputFormat == "" || cfg.outputFormat == model.OutputFormatText)
}

func (cfg *config) uiEnabled() bool {
	return cfg != nil && cfg.humanOutputEnabled() && !cfg.dryRun
}

func newConfig(src *model.RuntimeConfig, runArtifacts model.RunArtifacts) *config {
	if src == nil {
		return nil
	}
	return &config{
		workspaceRoot:          src.WorkspaceRoot,
		name:                   src.Name,
		round:                  src.Round,
		provider:               src.Provider,
		pr:                     src.PR,
		reviewsDir:             src.ReviewsDir,
		tasksDir:               src.TasksDir,
		dryRun:                 src.DryRun,
		autoCommit:             src.AutoCommit,
		concurrent:             src.Concurrent,
		batchSize:              src.BatchSize,
		ide:                    src.IDE,
		model:                  src.Model,
		addDirs:                append([]string(nil), src.AddDirs...),
		tailLines:              src.TailLines,
		reasoningEffort:        src.ReasoningEffort,
		accessMode:             src.AccessMode,
		mode:                   src.Mode,
		outputFormat:           src.OutputFormat,
		verbose:                src.Verbose,
		tui:                    src.TUI,
		persist:                src.Persist,
		runID:                  src.RunID,
		runArtifacts:           runArtifacts,
		includeCompleted:       src.IncludeCompleted,
		includeResolved:        src.IncludeResolved,
		timeout:                src.Timeout,
		maxRetries:             src.MaxRetries,
		retryBackoffMultiplier: src.RetryBackoffMultiplier,
	}
}

func newJobs(src []model.Job) []job {
	jobs := make([]job, 0, len(src))
	for i := range src {
		item := &src[i]
		jobs = append(jobs, job{
			codeFiles:     append([]string(nil), item.CodeFiles...),
			groups:        cloneGroups(item.Groups),
			taskTitle:     item.TaskTitle,
			taskType:      item.TaskType,
			safeName:      item.SafeName,
			prompt:        append([]byte(nil), item.Prompt...),
			systemPrompt:  item.SystemPrompt,
			outPromptPath: item.OutPromptPath,
			outLog:        item.OutLog,
			errLog:        item.ErrLog,
		})
	}
	return jobs
}

func cloneGroups(src map[string][]model.IssueEntry) map[string][]model.IssueEntry {
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
