package model

import (
	"time"

	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
)

// DefaultTimezone is the default schedule timezone used by automation config.
const DefaultTimezone = "UTC"

// DefaultMaxConcurrentJobs is the default global automation concurrency limit.
const DefaultMaxConcurrentJobs = 5

// Scope identifies the visibility boundary of an automation resource.
type Scope string

const (
	// AutomationScopeGlobal targets daemon-wide automation without a workspace binding.
	AutomationScopeGlobal Scope = "global"
	// AutomationScopeWorkspace targets automation bound to a specific workspace.
	AutomationScopeWorkspace Scope = "workspace"
)

// JobSource identifies where a job or trigger definition originated.
type JobSource string

const (
	// JobSourceConfig identifies a TOML-backed automation definition.
	JobSourceConfig JobSource = "config"
	// JobSourcePackage identifies a daemon-managed extension bundle definition.
	JobSourcePackage JobSource = "package"
	// JobSourceDynamic identifies a runtime-created automation definition.
	JobSourceDynamic JobSource = "dynamic"
)

// TargetKind identifies which runtime primitive an automation delegates to.
type TargetKind string

const (
	// TargetKindAgent starts the existing agent-session automation path.
	TargetKindAgent TargetKind = "agent"
	// TargetKindLoop starts a declared Loop through loop.Service.Start.
	TargetKindLoop TargetKind = "loop"
)

// ScheduleMode identifies how a scheduled job determines its next fire time.
type ScheduleMode string

const (
	// ScheduleModeCron evaluates a cron expression.
	ScheduleModeCron ScheduleMode = "cron"
	// ScheduleModeEvery evaluates a Go duration interval.
	ScheduleModeEvery ScheduleMode = "every"
	// ScheduleModeAt evaluates a one-shot RFC3339 timestamp.
	ScheduleModeAt ScheduleMode = "at"
)

// RetryStrategy identifies how failed runs should be retried.
type RetryStrategy string

const (
	// RetryStrategyNone disables retries after a failed run.
	RetryStrategyNone RetryStrategy = "none"
	// RetryStrategyBackoff retries failed runs with exponential backoff.
	RetryStrategyBackoff RetryStrategy = "backoff"
)

// RunStatus identifies the current lifecycle state of an automation run.
type RunStatus string

const (
	// RunScheduled reports a run that has been accepted but not yet started.
	RunScheduled RunStatus = "scheduled"
	// RunRunning reports a run that is actively dispatching or executing.
	RunRunning RunStatus = "running"
	// RunDelegated reports an automation activation that delegated execution into the task domain.
	RunDelegated RunStatus = "delegated"
	// RunCompleted reports a run that finished successfully.
	RunCompleted RunStatus = "completed"
	// RunFailed reports a run that finished with an error.
	RunFailed RunStatus = "failed"
	// RunCancelled reports a run that was canceled before completion.
	RunCancelled RunStatus = "canceled"
)

// SchedulerCatchUpPolicy identifies how missed scheduled fires are reconciled
// after daemon downtime.
type SchedulerCatchUpPolicy string

const (
	// SchedulerCatchUpPolicySkipMissed dispatches the latest missed fire within
	// grace and otherwise advances after recording a durable skip.
	SchedulerCatchUpPolicySkipMissed SchedulerCatchUpPolicy = "skip_missed"
	// SchedulerCatchUpPolicyCoalesce dispatches one fire for the most recent missed instant.
	SchedulerCatchUpPolicyCoalesce SchedulerCatchUpPolicy = "coalesce"
	// SchedulerCatchUpPolicyReplay dispatches missed instants chronologically.
	SchedulerCatchUpPolicyReplay SchedulerCatchUpPolicy = "replay"
	// SchedulerCatchUpPolicyRunOnce dispatches only the latest missed instant.
	SchedulerCatchUpPolicyRunOnce SchedulerCatchUpPolicy = "run_once_on_catchup"
)

// SchedulerSkipReason identifies why a scheduled fire advanced without dispatch.
type SchedulerSkipReason string

const (
	// SchedulerSkipReasonGraceExceeded reports a missed fire outside its grace window.
	SchedulerSkipReasonGraceExceeded SchedulerSkipReason = "misfire_grace_exceeded"
	// SchedulerSkipReasonSelfOverlap reports that the same job still has an active prior run.
	SchedulerSkipReasonSelfOverlap SchedulerSkipReason = "self_overlap"
	// SchedulerSkipReasonMetadataKey is the automation-run metadata key for durable skip evidence.
	SchedulerSkipReasonMetadataKey = "reason"
)

// ActivationSource identifies which ingress path produced an activation envelope.
type ActivationSource string

const (
	// ActivationSourceObserver identifies observer-backed trigger ingress.
	ActivationSourceObserver ActivationSource = "observer"
	// ActivationSourceHook identifies hook-backed trigger ingress.
	ActivationSourceHook ActivationSource = "hook"
	// ActivationSourceWebhook identifies external webhook ingress.
	ActivationSourceWebhook ActivationSource = "webhook"
	// ActivationSourceExtension identifies extension-provided ingress.
	ActivationSourceExtension ActivationSource = "extension"
)

// JobTaskConfig configures direct automation-to-task materialization for one job.
type JobTaskConfig struct {
	Title                string                 `json:"title,omitempty"                 toml:"title,omitempty"`
	Description          string                 `json:"description,omitempty"           toml:"description,omitempty"`
	Owner                *taskpkg.Ownership     `json:"owner,omitempty"                 toml:"owner,omitempty"`
	NetworkParticipation *participation.Request `json:"network_participation,omitempty" toml:"network_participation,omitempty"`
}

// LoopTarget configures an automation fire to start a Loop instead of an agent session.
type LoopTarget struct {
	WorkspaceID string `json:"workspace_id" toml:"workspace_id"`

	LoopName string `json:"loop_name" toml:"loop_name"`

	Inputs map[string]any `json:"inputs,omitempty" toml:"inputs,omitempty"`

	InputMapping map[string]string `json:"input_mapping,omitempty" toml:"input_mapping,omitempty"`

	NetworkParticipation *participation.Request `json:"network_participation,omitempty" toml:"network_participation,omitempty"`
}

// Job is the canonical scheduled automation definition used by runtime and storage layers.
type Job struct {
	ID          string          `json:"id"`
	Scope       Scope           `json:"scope"`
	Name        string          `json:"name"`
	TargetKind  TargetKind      `json:"target_kind"`
	AgentName   string          `json:"agent_name"`
	WorkspaceID string          `json:"workspace_id,omitempty"`
	Prompt      string          `json:"prompt"`
	Schedule    *ScheduleSpec   `json:"schedule,omitempty"`
	Task        *JobTaskConfig  `json:"task,omitempty"`
	LoopTarget  *LoopTarget     `json:"loop_target,omitempty"`
	Enabled     bool            `json:"enabled"`
	Retry       RetryConfig     `json:"retry"`
	FireLimit   FireLimitConfig `json:"fire_limit"`
	Source      JobSource       `json:"source"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// ScheduleSpec describes how a job should be scheduled.
type ScheduleSpec struct {
	Mode                ScheduleMode           `json:"mode"                            toml:"mode"`
	Expr                string                 `json:"expr,omitempty"                  toml:"expr,omitempty"`
	Interval            string                 `json:"interval,omitempty"              toml:"interval,omitempty"`
	Time                string                 `json:"time,omitempty"                  toml:"time,omitempty"`
	CatchUpPolicy       SchedulerCatchUpPolicy `json:"catch_up_policy,omitempty"       toml:"catch_up_policy,omitempty"`
	MisfireGraceSeconds int                    `json:"misfire_grace_seconds,omitempty" toml:"misfire_grace_seconds,omitempty"`
}

// Trigger is the canonical event-driven automation definition used by runtime and storage layers.
type Trigger struct {
	ID               string            `json:"id"`
	Scope            Scope             `json:"scope"`
	Name             string            `json:"name"`
	TargetKind       TargetKind        `json:"target_kind"`
	AgentName        string            `json:"agent_name"`
	WorkspaceID      string            `json:"workspace_id,omitempty"`
	Prompt           string            `json:"prompt"`
	Event            string            `json:"event"`
	Filter           map[string]string `json:"filter,omitempty"`
	LoopTarget       *LoopTarget       `json:"loop_target,omitempty"`
	Enabled          bool              `json:"enabled"`
	Retry            RetryConfig       `json:"retry"`
	FireLimit        FireLimitConfig   `json:"fire_limit"`
	Source           JobSource         `json:"source"`
	WebhookID        string            `json:"webhook_id,omitempty"`
	EndpointSlug     string            `json:"endpoint_slug,omitempty"`
	WebhookSecretRef string            `json:"webhook_secret_ref,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

// RetryConfig defines retry behavior for a failed automation run.
type RetryConfig struct {
	Strategy   RetryStrategy `json:"strategy"    toml:"strategy"`
	MaxRetries int           `json:"max_retries" toml:"max_retries"`
	BaseDelay  string        `json:"base_delay"  toml:"base_delay"`
}

// FireLimitConfig caps how often a job or trigger may fire within a rolling window.
type FireLimitConfig struct {
	Max    int    `json:"max"    toml:"max"`
	Window string `json:"window" toml:"window"`
}

// Run records the execution state of a single automation fire.
type Run struct {
	ID                   string                 `json:"id"`
	JobID                string                 `json:"job_id,omitempty"`
	TriggerID            string                 `json:"trigger_id,omitempty"`
	SessionID            string                 `json:"session_id,omitempty"`
	TaskID               string                 `json:"task_id,omitempty"`
	TaskRunID            string                 `json:"task_run_id,omitempty"`
	LoopRunID            string                 `json:"loop_run_id,omitempty"`
	FireID               string                 `json:"fire_id,omitempty"`
	Status               RunStatus              `json:"status"`
	Attempt              int                    `json:"attempt"`
	ScheduledAt          *time.Time             `json:"scheduled_at,omitempty"`
	StartedAt            *time.Time             `json:"started_at,omitempty"`
	EndedAt              *time.Time             `json:"ended_at,omitempty"`
	Error                string                 `json:"error,omitempty"`
	DeliveryError        string                 `json:"delivery_error,omitempty"`
	DeliveryErrorAt      *time.Time             `json:"delivery_error_at,omitempty"`
	NetworkParticipation *participation.Request `json:"network_participation,omitempty"`
	Metadata             map[string]any         `json:"metadata,omitempty"`
}

// ActivationEnvelope is the normalized trigger input regardless of source.
type ActivationEnvelope struct {
	Kind        string           `json:"kind"`
	Scope       Scope            `json:"scope"`
	WorkspaceID string           `json:"workspace_id,omitempty"`
	Source      ActivationSource `json:"source"`
	Data        map[string]any   `json:"data"`
}

// SchedulerState stores the durable scheduling cursor for one automation job.
type SchedulerState struct {
	JobID                     string                 `json:"job_id"`
	NextRunAt                 *time.Time             `json:"next_run_at,omitempty"`
	LastRunAt                 *time.Time             `json:"last_run_at,omitempty"`
	LastScheduledAt           *time.Time             `json:"last_scheduled_at,omitempty"`
	LastFireID                string                 `json:"last_fire_id,omitempty"`
	ScheduleHash              string                 `json:"schedule_hash,omitempty"`
	CatchUpPolicy             SchedulerCatchUpPolicy `json:"catch_up_policy"`
	MisfireGraceSeconds       int                    `json:"misfire_grace_seconds"`
	ConsecutiveResumeFailures int                    `json:"consecutive_resume_failures"`
	LastMisfireAt             *time.Time             `json:"last_misfire_at,omitempty"`
	MisfireCount              int                    `json:"misfire_count"`
	UpdatedAt                 time.Time              `json:"updated_at"`
}

// SchedulerClaim reserves one scheduled fire after the durable cursor has
// been advanced.
type SchedulerClaim struct {
	JobID                string
	RunID                string
	FireID               string
	ScheduledAt          time.Time
	NextRunAt            *time.Time
	ClaimedAt            time.Time
	ScheduleHash         string
	CatchUpPolicy        SchedulerCatchUpPolicy
	MisfireGraceSeconds  int
	CatchUp              bool
	Misfire              bool
	SkipReason           SchedulerSkipReason
	NetworkParticipation *participation.Request
}

// SchedulerClaimResult reports the state and pre-created run for one claimed
// scheduled fire.
type SchedulerClaimResult struct {
	State      SchedulerState
	Run        Run
	Skipped    bool
	SkipReason SchedulerSkipReason
}
