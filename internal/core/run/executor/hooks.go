package executor

import (
	"time"

	"github.com/compozy/compozy/internal/core/model"
)

type jobPreExecutePayload struct {
	RunID string    `json:"run_id"`
	Job   model.Job `json:"job"`
}

type jobPostExecutePayload struct {
	RunID  string          `json:"run_id"`
	Job    model.Job       `json:"job"`
	Result model.JobResult `json:"result"`
}

type jobPreRetryPayload struct {
	RunID     string    `json:"run_id"`
	Job       model.Job `json:"job"`
	Attempt   int       `json:"attempt"`
	LastError string    `json:"last_error"`
	Proceed   *bool     `json:"proceed,omitempty"`
	DelayMS   int64     `json:"delay_ms,omitempty"`
}

type runPreStartPayload struct {
	RunID     string              `json:"run_id"`
	Config    model.RuntimeConfig `json:"config"`
	Artifacts model.RunArtifacts  `json:"artifacts"`
}

type runPostStartPayload struct {
	RunID  string              `json:"run_id"`
	Config model.RuntimeConfig `json:"config"`
}

type runPreShutdownPayload struct {
	RunID  string `json:"run_id"`
	Reason string `json:"reason"`
}

type runPostShutdownPayload struct {
	RunID   string           `json:"run_id"`
	Reason  string           `json:"reason"`
	Summary model.RunSummary `json:"summary"`
}

type reviewPostFixPayload struct {
	RunID   string           `json:"run_id"`
	PR      string           `json:"pr"`
	Issue   model.IssueEntry `json:"issue"`
	Outcome model.FixOutcome `json:"outcome"`
}

type reviewPreResolvePayload struct {
	RunID   string           `json:"run_id"`
	PR      string           `json:"pr"`
	Issue   model.IssueEntry `json:"issue"`
	Outcome model.FixOutcome `json:"outcome"`
	Resolve *bool            `json:"resolve,omitempty"`
	Message string           `json:"message,omitempty"`
}

type resolvedReviewIssue struct {
	Entry    model.IssueEntry
	Provider providerResolvedIssue
}

type providerResolvedIssue struct {
	FilePath    string
	ProviderRef string
}

func hookModelJob(src *job) model.Job {
	if src == nil {
		return model.Job{}
	}
	return model.Job{
		CodeFiles:       append([]string(nil), src.CodeFiles...),
		Groups:          cloneIssueGroups(src.Groups),
		TaskTitle:       src.TaskTitle,
		TaskType:        src.TaskType,
		SafeName:        src.SafeName,
		IDE:             src.IDE,
		Model:           src.Model,
		ReasoningEffort: src.ReasoningEffort,
		Prompt:          append([]byte(nil), src.Prompt...),
		SystemPrompt:    src.SystemPrompt,
		OutPromptPath:   src.OutPromptPath,
		OutLog:          src.OutLog,
		ErrLog:          src.ErrLog,
	}
}

func applyHookModelJob(dst *job, updated model.Job) {
	if dst == nil {
		return
	}
	dst.CodeFiles = append([]string(nil), updated.CodeFiles...)
	dst.Groups = cloneIssueGroups(updated.Groups)
	dst.TaskTitle = updated.TaskTitle
	dst.TaskType = updated.TaskType
	dst.SafeName = updated.SafeName
	dst.IDE = updated.IDE
	dst.Model = updated.Model
	dst.ReasoningEffort = updated.ReasoningEffort
	dst.Prompt = append([]byte(nil), updated.Prompt...)
	dst.SystemPrompt = updated.SystemPrompt
	dst.OutPromptPath = updated.OutPromptPath
	dst.OutLog = updated.OutLog
	dst.ErrLog = updated.ErrLog
}

func cloneIssueGroups(src map[string][]model.IssueEntry) map[string][]model.IssueEntry {
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

func hookRuntimeConfig(src *config) model.RuntimeConfig {
	if src == nil {
		return model.RuntimeConfig{}
	}
	return model.RuntimeConfig{
		WorkspaceRoot:          src.WorkspaceRoot,
		Name:                   src.Name,
		Round:                  src.Round,
		Provider:               src.Provider,
		PR:                     src.PR,
		ReviewsDir:             src.ReviewsDir,
		TasksDir:               src.TasksDir,
		DryRun:                 src.DryRun,
		AutoCommit:             src.AutoCommit,
		Concurrent:             src.Concurrent,
		BatchSize:              src.BatchSize,
		IDE:                    src.IDE,
		Model:                  src.Model,
		AddDirs:                append([]string(nil), src.AddDirs...),
		TailLines:              src.TailLines,
		ReasoningEffort:        src.ReasoningEffort,
		AccessMode:             src.AccessMode,
		TaskRuntimeRules:       model.CloneTaskRuntimeRules(src.TaskRuntimeRules),
		Mode:                   src.Mode,
		OutputFormat:           src.OutputFormat,
		Verbose:                src.Verbose,
		TUI:                    src.TUI,
		Persist:                src.Persist,
		RunID:                  src.RunID,
		IncludeCompleted:       src.IncludeCompleted,
		IncludeResolved:        src.IncludeResolved,
		Timeout:                src.Timeout,
		MaxRetries:             src.MaxRetries,
		RetryBackoffMultiplier: src.RetryBackoffMultiplier,
		SoundEnabled:           src.SoundEnabled,
		SoundOnCompleted:       src.SoundOnCompleted,
		SoundOnFailed:          src.SoundOnFailed,
	}
}

func applyHookRuntimeConfig(dst *config, updated model.RuntimeConfig) {
	if dst == nil {
		return
	}
	dst.WorkspaceRoot = updated.WorkspaceRoot
	dst.Name = updated.Name
	dst.Round = updated.Round
	dst.Provider = updated.Provider
	dst.PR = updated.PR
	dst.ReviewsDir = updated.ReviewsDir
	dst.TasksDir = updated.TasksDir
	dst.DryRun = updated.DryRun
	dst.AutoCommit = updated.AutoCommit
	dst.Concurrent = updated.Concurrent
	dst.BatchSize = updated.BatchSize
	dst.IDE = updated.IDE
	dst.Model = updated.Model
	dst.AddDirs = append([]string(nil), updated.AddDirs...)
	dst.TailLines = updated.TailLines
	dst.ReasoningEffort = updated.ReasoningEffort
	dst.AccessMode = updated.AccessMode
	dst.TaskRuntimeRules = model.CloneTaskRuntimeRules(updated.TaskRuntimeRules)
	dst.Mode = updated.Mode
	dst.OutputFormat = updated.OutputFormat
	dst.Verbose = updated.Verbose
	dst.TUI = updated.TUI
	dst.Persist = updated.Persist
	dst.RunID = updated.RunID
	dst.IncludeCompleted = updated.IncludeCompleted
	dst.IncludeResolved = updated.IncludeResolved
	dst.Timeout = updated.Timeout
	dst.MaxRetries = updated.MaxRetries
	dst.RetryBackoffMultiplier = updated.RetryBackoffMultiplier
	dst.SoundEnabled = updated.SoundEnabled
	dst.SoundOnCompleted = updated.SoundOnCompleted
	dst.SoundOnFailed = updated.SoundOnFailed
}

func hookRunSummary(result executionResult) model.RunSummary {
	summary := model.RunSummary{
		Status: result.Status,
		Error:  result.Error,
	}
	for idx := range result.Jobs {
		job := result.Jobs[idx]
		summary.JobsTotal++
		switch job.Status {
		case runStatusSucceeded:
			summary.JobsSucceeded++
		case runStatusFailed:
			summary.JobsFailed++
		case runStatusCanceled:
			summary.JobsCanceled++
		}
	}
	if result.TeardownError != "" {
		summary.TeardownError = result.TeardownError
	}
	return summary
}

func hookShutdownReason(result executionResult) string {
	if result.TeardownError != "" {
		return "shutdown_error"
	}
	return result.Status
}

func hookFixOutcome(err error) model.FixOutcome {
	outcome := model.FixOutcome{Status: runStatusSucceeded}
	if err != nil {
		outcome.Status = runStatusFailed
		outcome.Error = err.Error()
	}
	return outcome
}

func (r *jobRunner) hookJobResult() model.JobResult {
	if r == nil || r.job == nil || r.lifecycle == nil {
		return model.JobResult{}
	}

	attempts := r.lifecycle.attempt
	if attempts == 0 && jobStatusOrDefault(r.job.Status) != runStatusUnknown {
		attempts = 1
	}

	durationMS := int64(0)
	if !r.lifecycle.startedAt.IsZero() {
		durationMS = time.Since(r.lifecycle.startedAt).Milliseconds()
		if durationMS < 0 {
			durationMS = 0
		}
	}

	return model.JobResult{
		Status:     jobStatusOrDefault(r.job.Status),
		ExitCode:   r.job.ExitCode,
		Attempts:   attempts,
		DurationMS: durationMS,
		Error:      r.job.Failure,
	}
}
