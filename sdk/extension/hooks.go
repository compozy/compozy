package extension

import "encoding/json"

// PlanPreDiscoverPayload is delivered for plan.pre_discover.
type PlanPreDiscoverPayload struct {
	RunID        string        `json:"run_id"`
	Workflow     string        `json:"workflow"`
	Mode         ExecutionMode `json:"mode"`
	ExtraSources []string      `json:"extra_sources,omitempty"`
}

// PlanPostDiscoverPayload is delivered for plan.post_discover.
type PlanPostDiscoverPayload struct {
	RunID    string       `json:"run_id"`
	Workflow string       `json:"workflow"`
	Entries  []IssueEntry `json:"entries,omitempty"`
}

// PlanPreGroupPayload is delivered for plan.pre_group.
type PlanPreGroupPayload struct {
	RunID   string       `json:"run_id"`
	Entries []IssueEntry `json:"entries,omitempty"`
}

// PlanPostGroupPayload is delivered for plan.post_group.
type PlanPostGroupPayload struct {
	RunID  string                  `json:"run_id"`
	Groups map[string][]IssueEntry `json:"groups,omitempty"`
}

// PlanPrePrepareJobsPayload is delivered for plan.pre_prepare_jobs.
type PlanPrePrepareJobsPayload struct {
	RunID  string                  `json:"run_id"`
	Groups map[string][]IssueEntry `json:"groups,omitempty"`
}

// PlanPreResolveTaskRuntimePayload is delivered for plan.pre_resolve_task_runtime.
type PlanPreResolveTaskRuntimePayload struct {
	RunID   string          `json:"run_id"`
	Task    TaskRuntimeTask `json:"task"`
	Runtime TaskRuntime     `json:"runtime"`
}

// PlanPostPrepareJobsPayload is delivered for plan.post_prepare_jobs.
type PlanPostPrepareJobsPayload struct {
	RunID string `json:"run_id"`
	Jobs  []Job  `json:"jobs,omitempty"`
}

// PromptPreBuildPayload is delivered for prompt.pre_build.
type PromptPreBuildPayload struct {
	RunID       string      `json:"run_id"`
	JobID       string      `json:"job_id"`
	BatchParams BatchParams `json:"batch_params"`
}

// PromptPostBuildPayload is delivered for prompt.post_build.
type PromptPostBuildPayload struct {
	RunID       string      `json:"run_id"`
	JobID       string      `json:"job_id"`
	PromptText  string      `json:"prompt_text"`
	BatchParams BatchParams `json:"batch_params"`
}

// PromptPreSystemPayload is delivered for prompt.pre_system.
type PromptPreSystemPayload struct {
	RunID          string      `json:"run_id"`
	JobID          string      `json:"job_id"`
	SystemAddendum string      `json:"system_addendum"`
	BatchParams    BatchParams `json:"batch_params"`
}

// AgentPreSessionCreatePayload is delivered for agent.pre_session_create.
type AgentPreSessionCreatePayload struct {
	RunID          string         `json:"run_id"`
	JobID          string         `json:"job_id"`
	SessionRequest SessionRequest `json:"session_request"`
}

// AgentPostSessionCreatePayload is delivered for agent.post_session_create.
type AgentPostSessionCreatePayload struct {
	RunID     string          `json:"run_id"`
	JobID     string          `json:"job_id"`
	SessionID string          `json:"session_id"`
	Identity  SessionIdentity `json:"identity"`
}

// AgentPreSessionResumePayload is delivered for agent.pre_session_resume.
type AgentPreSessionResumePayload struct {
	RunID         string               `json:"run_id"`
	JobID         string               `json:"job_id"`
	ResumeRequest ResumeSessionRequest `json:"resume_request"`
}

// AgentOnSessionUpdatePayload is delivered for agent.on_session_update.
type AgentOnSessionUpdatePayload struct {
	RunID     string        `json:"run_id"`
	JobID     string        `json:"job_id"`
	SessionID string        `json:"session_id"`
	Update    SessionUpdate `json:"update"`
}

// AgentPostSessionEndPayload is delivered for agent.post_session_end.
type AgentPostSessionEndPayload struct {
	RunID     string         `json:"run_id"`
	JobID     string         `json:"job_id"`
	SessionID string         `json:"session_id"`
	Outcome   SessionOutcome `json:"outcome"`
}

// JobPreExecutePayload is delivered for job.pre_execute.
type JobPreExecutePayload struct {
	RunID string `json:"run_id"`
	Job   Job    `json:"job"`
}

// JobPostExecutePayload is delivered for job.post_execute.
type JobPostExecutePayload struct {
	RunID  string    `json:"run_id"`
	Job    Job       `json:"job"`
	Result JobResult `json:"result"`
}

// JobPreRetryPayload is delivered for job.pre_retry.
type JobPreRetryPayload struct {
	RunID     string `json:"run_id"`
	Job       Job    `json:"job"`
	Attempt   int    `json:"attempt"`
	LastError string `json:"last_error"`
}

// RunPreStartPayload is delivered for run.pre_start.
type RunPreStartPayload struct {
	RunID     string        `json:"run_id"`
	Config    RuntimeConfig `json:"config"`
	Artifacts RunArtifacts  `json:"artifacts"`
}

// RunPostStartPayload is delivered for run.post_start.
type RunPostStartPayload struct {
	RunID  string        `json:"run_id"`
	Config RuntimeConfig `json:"config"`
}

// RunPreShutdownPayload is delivered for run.pre_shutdown.
type RunPreShutdownPayload struct {
	RunID  string `json:"run_id"`
	Reason string `json:"reason"`
}

// RunPostShutdownPayload is delivered for run.post_shutdown.
type RunPostShutdownPayload struct {
	RunID   string     `json:"run_id"`
	Reason  string     `json:"reason"`
	Summary RunSummary `json:"summary"`
}

// ReviewPreFetchPayload is delivered for review.pre_fetch.
type ReviewPreFetchPayload struct {
	RunID       string      `json:"run_id"`
	PR          string      `json:"pr"`
	Provider    string      `json:"provider"`
	FetchConfig FetchConfig `json:"fetch_config"`
}

// ReviewPostFetchPayload is delivered for review.post_fetch.
type ReviewPostFetchPayload struct {
	RunID  string       `json:"run_id"`
	PR     string       `json:"pr"`
	Issues []IssueEntry `json:"issues,omitempty"`
}

// ReviewPreBatchPayload is delivered for review.pre_batch.
type ReviewPreBatchPayload struct {
	RunID  string                  `json:"run_id"`
	PR     string                  `json:"pr"`
	Groups map[string][]IssueEntry `json:"groups,omitempty"`
}

// ReviewPostFixPayload is delivered for review.post_fix.
type ReviewPostFixPayload struct {
	RunID   string     `json:"run_id"`
	PR      string     `json:"pr"`
	Issue   IssueEntry `json:"issue"`
	Outcome FixOutcome `json:"outcome"`
}

// ReviewPreResolvePayload is delivered for review.pre_resolve.
type ReviewPreResolvePayload struct {
	RunID   string     `json:"run_id"`
	PR      string     `json:"pr"`
	Issue   IssueEntry `json:"issue"`
	Outcome FixOutcome `json:"outcome"`
}

// ReviewWatchPreRoundPayload is delivered for review.watch_pre_round.
type ReviewWatchPreRoundPayload struct {
	RunID            string          `json:"run_id"`
	Provider         string          `json:"provider"`
	PR               string          `json:"pr"`
	Workflow         string          `json:"workflow"`
	Round            int             `json:"round"`
	HeadSHA          string          `json:"head_sha"`
	ReviewID         string          `json:"review_id,omitempty"`
	ReviewState      string          `json:"review_state,omitempty"`
	Status           string          `json:"status,omitempty"`
	Nitpicks         bool            `json:"nitpicks"`
	RuntimeOverrides json.RawMessage `json:"runtime_overrides,omitempty"`
	Batching         json.RawMessage `json:"batching,omitempty"`
	Continue         bool            `json:"continue"`
	StopReason       string          `json:"stop_reason,omitempty"`
}

// ReviewWatchPostRoundPayload is delivered for review.watch_post_round.
type ReviewWatchPostRoundPayload struct {
	RunID      string `json:"run_id"`
	Provider   string `json:"provider"`
	PR         string `json:"pr"`
	Workflow   string `json:"workflow"`
	Round      int    `json:"round"`
	HeadSHA    string `json:"head_sha,omitempty"`
	ChildRunID string `json:"child_run_id,omitempty"`
	Status     string `json:"status,omitempty"`
	Remote     string `json:"remote,omitempty"`
	Branch     string `json:"branch,omitempty"`
	Total      int    `json:"total,omitempty"`
	Resolved   int    `json:"resolved,omitempty"`
	Unresolved int    `json:"unresolved,omitempty"`
	Pushed     bool   `json:"pushed,omitempty"`
	StopReason string `json:"stop_reason,omitempty"`
	Error      string `json:"error,omitempty"`
}

// ReviewWatchPrePushPayload is delivered for review.watch_pre_push.
type ReviewWatchPrePushPayload struct {
	RunID      string `json:"run_id"`
	Provider   string `json:"provider"`
	PR         string `json:"pr"`
	Workflow   string `json:"workflow"`
	Round      int    `json:"round"`
	HeadSHA    string `json:"head_sha"`
	Remote     string `json:"remote"`
	Branch     string `json:"branch"`
	Push       bool   `json:"push"`
	StopReason string `json:"stop_reason,omitempty"`
}

// ReviewWatchFinishedPayload is delivered for review.watch_finished.
type ReviewWatchFinishedPayload struct {
	RunID          string `json:"run_id"`
	ChildRunID     string `json:"child_run_id,omitempty"`
	Provider       string `json:"provider"`
	PR             string `json:"pr"`
	Workflow       string `json:"workflow"`
	Round          int    `json:"round,omitempty"`
	HeadSHA        string `json:"head_sha,omitempty"`
	Status         string `json:"status"`
	TerminalReason string `json:"terminal_reason,omitempty"`
	Stopped        bool   `json:"stopped,omitempty"`
	Clean          bool   `json:"clean,omitempty"`
	MaxRounds      bool   `json:"max_rounds,omitempty"`
	Error          string `json:"error,omitempty"`
}

// ArtifactPreWritePayload is delivered for artifact.pre_write.
type ArtifactPreWritePayload struct {
	RunID          string `json:"run_id"`
	Path           string `json:"path"`
	ContentPreview string `json:"content_preview"`
}

// ArtifactPostWritePayload is delivered for artifact.post_write.
type ArtifactPostWritePayload struct {
	RunID        string `json:"run_id"`
	Path         string `json:"path"`
	BytesWritten int    `json:"bytes_written"`
}

// ExtraSourcesPatch replaces the extra source list for plan.pre_discover.
type ExtraSourcesPatch struct {
	ExtraSources *[]string `json:"extra_sources,omitempty"`
}

// EntriesPatch replaces one issue entry slice.
type EntriesPatch struct {
	Entries *[]IssueEntry `json:"entries,omitempty"`
}

// IssuesPatch replaces one review issue slice.
type IssuesPatch struct {
	Issues *[]IssueEntry `json:"issues,omitempty"`
}

// GroupsPatch replaces one grouped issue map.
type GroupsPatch struct {
	Groups *map[string][]IssueEntry `json:"groups,omitempty"`
}

// TaskRuntimePatch replaces the effective runtime for one task.
type TaskRuntimePatch struct {
	Runtime *TaskRuntime `json:"runtime,omitempty"`
}

// JobsPatch replaces one prepared job slice.
type JobsPatch struct {
	Jobs *[]Job `json:"jobs,omitempty"`
}

// BatchParamsPatch replaces prompt build parameters.
type BatchParamsPatch struct {
	BatchParams *BatchParams `json:"batch_params,omitempty"`
}

// PromptTextPatch replaces the rendered prompt text.
type PromptTextPatch struct {
	PromptText *string `json:"prompt_text,omitempty"`
}

// SystemAddendumPatch replaces the system prompt addendum.
type SystemAddendumPatch struct {
	SystemAddendum *string `json:"system_addendum,omitempty"`
}

// SessionRequestPatch replaces the ACP create-session request payload.
type SessionRequestPatch struct {
	SessionRequest *SessionRequest `json:"session_request,omitempty"`
}

// ResumeSessionRequestPatch replaces the ACP resume-session request payload.
type ResumeSessionRequestPatch struct {
	ResumeRequest *ResumeSessionRequest `json:"resume_request,omitempty"`
}

// JobPatch replaces one job payload.
type JobPatch struct {
	Job *Job `json:"job,omitempty"`
}

// RetryDecisionPatch controls retry continuation and delay.
type RetryDecisionPatch struct {
	Proceed *bool  `json:"proceed,omitempty"`
	DelayMS *int64 `json:"delay_ms,omitempty"`
}

// RuntimeConfigPatch replaces the run configuration payload.
type RuntimeConfigPatch struct {
	Config *RuntimeConfig `json:"config,omitempty"`
}

// FetchConfigPatch replaces the review fetch configuration.
type FetchConfigPatch struct {
	FetchConfig *FetchConfig `json:"fetch_config,omitempty"`
}

// ResolveDecisionPatch controls remote issue resolution.
type ResolveDecisionPatch struct {
	Resolve *bool   `json:"resolve,omitempty"`
	Message *string `json:"message,omitempty"`
}

// ReviewWatchPreRoundPatch controls one review-watch round before fetch/fix.
type ReviewWatchPreRoundPatch struct {
	Nitpicks         *bool            `json:"nitpicks,omitempty"`
	RuntimeOverrides *json.RawMessage `json:"runtime_overrides,omitempty"`
	Batching         *json.RawMessage `json:"batching,omitempty"`
	Continue         *bool            `json:"continue,omitempty"`
	StopReason       *string          `json:"stop_reason,omitempty"`
}

// ReviewWatchPrePushPatch controls one review-watch push attempt.
type ReviewWatchPrePushPatch struct {
	Remote     *string `json:"remote,omitempty"`
	Branch     *string `json:"branch,omitempty"`
	Push       *bool   `json:"push,omitempty"`
	StopReason *string `json:"stop_reason,omitempty"`
}

// ArtifactWritePatch mutates an artifact write request.
type ArtifactWritePatch struct {
	Path    *string `json:"path,omitempty"`
	Content *string `json:"content,omitempty"`
	Cancel  *bool   `json:"cancel,omitempty"`
}
