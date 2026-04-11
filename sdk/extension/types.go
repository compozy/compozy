package extension

import (
	"encoding/json"
	"time"

	"github.com/compozy/compozy/pkg/compozy/events"
	eventkinds "github.com/compozy/compozy/pkg/compozy/events/kinds"
)

// ProtocolVersion identifies the extension subprocess protocol version the SDK
// speaks.
const ProtocolVersion = "1"

// Capability declares one extension capability grant.
type Capability string

// Supported capability values.
const (
	CapabilityEventsRead        Capability = "events.read"
	CapabilityEventsPublish     Capability = "events.publish"
	CapabilityPromptMutate      Capability = "prompt.mutate"
	CapabilityPlanMutate        Capability = "plan.mutate"
	CapabilityAgentMutate       Capability = "agent.mutate"
	CapabilityJobMutate         Capability = "job.mutate"
	CapabilityRunMutate         Capability = "run.mutate"
	CapabilityReviewMutate      Capability = "review.mutate"
	CapabilityArtifactsRead     Capability = "artifacts.read"
	CapabilityArtifactsWrite    Capability = "artifacts.write"
	CapabilityTasksRead         Capability = "tasks.read"
	CapabilityTasksCreate       Capability = "tasks.create"
	CapabilityRunsStart         Capability = "runs.start"
	CapabilityMemoryRead        Capability = "memory.read"
	CapabilityMemoryWrite       Capability = "memory.write"
	CapabilityProvidersRegister Capability = "providers.register"
	CapabilitySkillsShip        Capability = "skills.ship"
	CapabilitySubprocessSpawn   Capability = "subprocess.spawn"
	CapabilityNetworkEgress     Capability = "network.egress"
)

// HookName identifies one canonical extension hook event.
type HookName string

// Supported hook names.
const (
	HookPlanPreDiscover        HookName = "plan.pre_discover"
	HookPlanPostDiscover       HookName = "plan.post_discover"
	HookPlanPreGroup           HookName = "plan.pre_group"
	HookPlanPostGroup          HookName = "plan.post_group"
	HookPlanPrePrepareJobs     HookName = "plan.pre_prepare_jobs"
	HookPlanPostPrepareJobs    HookName = "plan.post_prepare_jobs"
	HookPromptPreBuild         HookName = "prompt.pre_build"
	HookPromptPostBuild        HookName = "prompt.post_build"
	HookPromptPreSystem        HookName = "prompt.pre_system"
	HookAgentPreSessionCreate  HookName = "agent.pre_session_create"
	HookAgentPostSessionCreate HookName = "agent.post_session_create"
	HookAgentPreSessionResume  HookName = "agent.pre_session_resume"
	HookAgentOnSessionUpdate   HookName = "agent.on_session_update"
	HookAgentPostSessionEnd    HookName = "agent.post_session_end"
	HookJobPreExecute          HookName = "job.pre_execute"
	HookJobPostExecute         HookName = "job.post_execute"
	HookJobPreRetry            HookName = "job.pre_retry"
	HookRunPreStart            HookName = "run.pre_start"
	HookRunPostStart           HookName = "run.post_start"
	HookRunPreShutdown         HookName = "run.pre_shutdown"
	HookRunPostShutdown        HookName = "run.post_shutdown"
	HookReviewPreFetch         HookName = "review.pre_fetch"
	HookReviewPostFetch        HookName = "review.post_fetch"
	HookReviewPreBatch         HookName = "review.pre_batch"
	HookReviewPostFix          HookName = "review.post_fix"
	HookReviewPreResolve       HookName = "review.pre_resolve"
	HookArtifactPreWrite       HookName = "artifact.pre_write"
	HookArtifactPostWrite      HookName = "artifact.post_write"
)

// ExecutionMode identifies the target Compozy execution mode.
type ExecutionMode string

// Supported execution modes.
const (
	ExecutionModePRReview ExecutionMode = "pr-review"
	ExecutionModePRDTasks ExecutionMode = "prd-tasks"
	ExecutionModeExec     ExecutionMode = "exec"
)

// OutputFormat identifies the requested output rendering mode.
type OutputFormat string

// Supported output formats.
const (
	OutputFormatText    OutputFormat = "text"
	OutputFormatJSON    OutputFormat = "json"
	OutputFormatRawJSON OutputFormat = "raw-json"
)

// MemoryWriteMode identifies how a memory document write is applied.
type MemoryWriteMode string

// Supported memory write modes.
const (
	MemoryWriteModeReplace MemoryWriteMode = "replace"
	MemoryWriteModeAppend  MemoryWriteMode = "append"
)

// SessionStatus identifies the public session terminal state.
type SessionStatus = eventkinds.SessionStatus

// SessionUpdate is the public streamed session update shape.
type SessionUpdate = eventkinds.SessionUpdate

// Event is the public Compozy event envelope forwarded to extensions.
type Event = events.Event

// EventKind identifies one forwarded bus event kind.
type EventKind = events.EventKind

// Usage is the public usage summary shape embedded in session updates.
type Usage = eventkinds.Usage

// HookInfo describes the current hook invocation metadata.
type HookInfo struct {
	Name      string   `json:"name"`
	Event     HookName `json:"event"`
	Mutable   bool     `json:"mutable"`
	Required  bool     `json:"required"`
	Priority  int      `json:"priority"`
	TimeoutMS int64    `json:"timeout_ms"`
}

// HookContext carries request metadata and Host API access for one handler
// invocation.
type HookContext struct {
	InvocationID string
	Hook         HookInfo
	Host         *HostAPI
}

// InitializeRequest is the host-originated initialize request.
type InitializeRequest struct {
	ProtocolVersion           string                    `json:"protocol_version"`
	SupportedProtocolVersions []string                  `json:"supported_protocol_versions"`
	CompozyVersion            string                    `json:"compozy_version"`
	Extension                 InitializeRequestIdentity `json:"extension"`
	GrantedCapabilities       []Capability              `json:"granted_capabilities,omitempty"`
	Runtime                   InitializeRuntime         `json:"runtime"`
}

// InitializeRequestIdentity identifies the extension instance the host loaded.
type InitializeRequestIdentity struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Source  string `json:"source"`
}

// InitializeRuntime describes the run-scoped runtime contract.
type InitializeRuntime struct {
	RunID                 string `json:"run_id"`
	ParentRunID           string `json:"parent_run_id"`
	WorkspaceRoot         string `json:"workspace_root"`
	InvokingCommand       string `json:"invoking_command"`
	ShutdownTimeoutMS     int64  `json:"shutdown_timeout_ms"`
	DefaultHookTimeoutMS  int64  `json:"default_hook_timeout_ms"`
	HealthCheckIntervalMS int64  `json:"health_check_interval_ms"`
}

// InitializeResponse is the extension's initialize acknowledgement.
type InitializeResponse struct {
	ProtocolVersion           string                 `json:"protocol_version"`
	ExtensionInfo             InitializeResponseInfo `json:"extension_info"`
	AcceptedCapabilities      []Capability           `json:"accepted_capabilities,omitempty"`
	SupportedHookEvents       []HookName             `json:"supported_hook_events,omitempty"`
	RegisteredReviewProviders []string               `json:"registered_review_providers,omitempty"`
	Supports                  Supports               `json:"supports"`
}

// InitializeResponseInfo describes the running SDK identity.
type InitializeResponseInfo struct {
	Name       string `json:"name,omitempty"`
	Version    string `json:"version,omitempty"`
	SDKName    string `json:"sdk_name,omitempty"`
	SDKVersion string `json:"sdk_version,omitempty"`
}

// Supports reports which optional base methods the extension serves.
type Supports struct {
	HealthCheck bool `json:"health_check"`
	OnEvent     bool `json:"on_event"`
}

// ExecuteHookRequest is one host-originated hook invocation envelope.
type ExecuteHookRequest struct {
	InvocationID string          `json:"invocation_id"`
	Hook         HookInfo        `json:"hook"`
	Payload      json.RawMessage `json:"payload"`
}

// ExecuteHookResponse is the hook response envelope returned to the host.
type ExecuteHookResponse struct {
	Patch json.RawMessage `json:"patch,omitempty"`
}

// OnEventRequest is one host-originated event delivery request.
type OnEventRequest struct {
	Event Event `json:"event"`
}

// HealthCheckRequest is the host-originated liveness probe payload.
type HealthCheckRequest struct{}

// HealthCheckResponse describes the extension health status.
type HealthCheckResponse struct {
	Healthy bool           `json:"healthy"`
	Message string         `json:"message,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

// ShutdownRequest is the host-originated graceful shutdown request.
type ShutdownRequest struct {
	Reason     string `json:"reason"`
	DeadlineMS int64  `json:"deadline_ms"`
}

// ShutdownResponse acknowledges a graceful shutdown request.
type ShutdownResponse struct {
	Acknowledged bool `json:"acknowledged"`
}

// ReviewProviderContext carries provider-local metadata and Host API access.
type ReviewProviderContext struct {
	Provider string
	Host     *HostAPI
}

// FetchRequest mirrors the host-side review provider fetch request.
type FetchRequest struct {
	PR              string `json:"pr"`
	IncludeNitpicks bool   `json:"include_nitpicks,omitempty"`
}

// ReviewItem mirrors the host-side normalized review item shape.
type ReviewItem struct {
	Title                   string `json:"title"`
	File                    string `json:"file"`
	Line                    int    `json:"line,omitempty"`
	Severity                string `json:"severity,omitempty"`
	Author                  string `json:"author,omitempty"`
	Body                    string `json:"body"`
	ProviderRef             string `json:"provider_ref,omitempty"`
	ReviewHash              string `json:"review_hash,omitempty"`
	SourceReviewID          string `json:"source_review_id,omitempty"`
	SourceReviewSubmittedAt string `json:"source_review_submitted_at,omitempty"`
}

// ResolvedIssue mirrors the host-side resolved issue payload.
type ResolvedIssue struct {
	FilePath    string `json:"file_path"`
	ProviderRef string `json:"provider_ref,omitempty"`
}

// ResolveIssuesRequest mirrors the host-side review provider resolve request.
type ResolveIssuesRequest struct {
	PR     string          `json:"pr"`
	Issues []ResolvedIssue `json:"issues,omitempty"`
}

// IssueEntry mirrors the issue/task entry shape used in planning hooks.
type IssueEntry struct {
	Name     string
	AbsPath  string
	Content  string
	CodeFile string
}

// WorkflowMemoryContext describes the current workflow memory documents.
type WorkflowMemoryContext struct {
	Directory               string `json:"directory,omitempty"`
	WorkflowPath            string `json:"workflow_path,omitempty"`
	TaskPath                string `json:"task_path,omitempty"`
	WorkflowNeedsCompaction bool   `json:"workflow_needs_compaction,omitempty"`
	TaskNeedsCompaction     bool   `json:"task_needs_compaction,omitempty"`
}

// BatchParams mirrors the prompt build input snapshot exposed to prompt hooks.
type BatchParams struct {
	Name        string                  `json:"name,omitempty"`
	Round       int                     `json:"round,omitempty"`
	Provider    string                  `json:"provider,omitempty"`
	PR          string                  `json:"pr,omitempty"`
	ReviewsDir  string                  `json:"reviews_dir,omitempty"`
	BatchGroups map[string][]IssueEntry `json:"batch_groups,omitempty"`
	AutoCommit  bool                    `json:"auto_commit,omitempty"`
	Mode        ExecutionMode           `json:"mode,omitempty"`
	Memory      *WorkflowMemoryContext  `json:"memory,omitempty"`
}

// MCPServer mirrors one stdio-backed MCP server attachment.
type MCPServer struct {
	Stdio *MCPServerStdio
}

// MCPServerStdio mirrors the stdio MCP server transport fields.
type MCPServerStdio struct {
	Name    string
	Command string
	Args    []string
	Env     map[string]string
}

// SessionRequest mirrors the mutable create-session payload delivered to agent
// hooks.
type SessionRequest struct {
	Prompt     []byte            `json:"prompt,omitempty"`
	WorkingDir string            `json:"working_dir,omitempty"`
	Model      string            `json:"model,omitempty"`
	MCPServers []MCPServer       `json:"mcp_servers,omitempty"`
	ExtraEnv   map[string]string `json:"extra_env,omitempty"`
}

// ResumeSessionRequest mirrors the mutable resume-session payload delivered to
// agent hooks.
type ResumeSessionRequest struct {
	SessionID  string            `json:"session_id,omitempty"`
	Prompt     []byte            `json:"prompt,omitempty"`
	WorkingDir string            `json:"working_dir,omitempty"`
	Model      string            `json:"model,omitempty"`
	MCPServers []MCPServer       `json:"mcp_servers,omitempty"`
	ExtraEnv   map[string]string `json:"extra_env,omitempty"`
}

// SessionIdentity captures the stable agent session identifiers.
type SessionIdentity struct {
	ACPSessionID   string `json:"acp_session_id"`
	AgentSessionID string `json:"agent_session_id,omitempty"`
	Resumed        bool   `json:"resumed,omitempty"`
}

// SessionOutcome mirrors the terminal session outcome payload.
type SessionOutcome struct {
	Status SessionStatus `json:"status"`
	Error  string        `json:"error,omitempty"`
}

// Job mirrors the planned job shape exposed to run/job hooks.
type Job struct {
	CodeFiles     []string
	Groups        map[string][]IssueEntry
	TaskTitle     string
	TaskType      string
	SafeName      string
	Prompt        []byte
	SystemPrompt  string
	MCPServers    []MCPServer
	OutPromptPath string
	OutLog        string
	ErrLog        string
}

// FetchConfig mirrors the review provider fetch configuration.
type FetchConfig struct {
	ReviewsDir      string `json:"reviews_dir,omitempty"`
	IncludeResolved bool   `json:"include_resolved,omitempty"`
}

// FixOutcome mirrors the review fix result payload.
type FixOutcome struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// JobResult mirrors the job execution result payload.
type JobResult struct {
	Status     string `json:"status"`
	ExitCode   int    `json:"exit_code,omitempty"`
	Attempts   int    `json:"attempts,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"`
	Error      string `json:"error,omitempty"`
}

// ExplicitRuntimeFlags mirrors which runtime flags were explicitly overridden.
type ExplicitRuntimeFlags struct {
	IDE             bool
	Model           bool
	ReasoningEffort bool
	AccessMode      bool
}

// RuntimeConfig mirrors the run configuration payload exposed to run hooks.
type RuntimeConfig struct {
	WorkspaceRoot              string
	Name                       string
	Round                      int
	Provider                   string
	PR                         string
	Nitpicks                   bool
	ReviewsDir                 string
	TasksDir                   string
	DryRun                     bool
	AutoCommit                 bool
	Concurrent                 int
	BatchSize                  int
	IDE                        string
	Model                      string
	AddDirs                    []string
	TailLines                  int
	ReasoningEffort            string
	AccessMode                 string
	AgentName                  string
	ExplicitRuntime            ExplicitRuntimeFlags
	Mode                       ExecutionMode
	OutputFormat               OutputFormat
	Verbose                    bool
	TUI                        bool
	Persist                    bool
	EnableExecutableExtensions bool
	RunID                      string
	ParentRunID                string
	PromptText                 string
	PromptFile                 string
	ReadPromptStdin            bool
	ResolvedPromptText         string
	IncludeCompleted           bool
	IncludeResolved            bool
	Timeout                    time.Duration
	MaxRetries                 int
	RetryBackoffMultiplier     float64
}

// RunArtifacts mirrors the run artifact directory layout exposed to run hooks.
type RunArtifacts struct {
	RunID       string
	RunDir      string
	RunMetaPath string
	EventsPath  string
	TurnsDir    string
	JobsDir     string
	ResultPath  string
}

// RunSummary mirrors the terminal run summary payload.
type RunSummary struct {
	Status        string `json:"status"`
	JobsTotal     int    `json:"jobs_total"`
	JobsSucceeded int    `json:"jobs_succeeded,omitempty"`
	JobsFailed    int    `json:"jobs_failed,omitempty"`
	JobsCanceled  int    `json:"jobs_canceled,omitempty"`
	Error         string `json:"error,omitempty"`
	TeardownError string `json:"teardown_error,omitempty"`
}

// Ptr returns a pointer to value.
func Ptr[T any](value T) *T {
	return &value
}
