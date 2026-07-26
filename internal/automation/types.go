package automation

import modelpkg "github.com/compozy/agh/internal/automation/model"

// DefaultTimezone is the default schedule timezone used by automation config.
const DefaultTimezone = modelpkg.DefaultTimezone

// DefaultMaxConcurrentJobs is the default global automation concurrency limit.
const DefaultMaxConcurrentJobs = modelpkg.DefaultMaxConcurrentJobs

// Scope identifies the visibility boundary of an automation resource.
type Scope = modelpkg.Scope

const (
	// AutomationScopeGlobal targets daemon-wide automation without a workspace binding.
	AutomationScopeGlobal = modelpkg.AutomationScopeGlobal
	// AutomationScopeWorkspace targets automation bound to a specific workspace.
	AutomationScopeWorkspace = modelpkg.AutomationScopeWorkspace
)

// JobSource identifies where a job or trigger definition originated.
type JobSource = modelpkg.JobSource

const (
	// JobSourceConfig identifies a TOML-backed automation definition.
	JobSourceConfig = modelpkg.JobSourceConfig
	// JobSourcePackage identifies a daemon-managed extension bundle definition.
	JobSourcePackage = modelpkg.JobSourcePackage
	// JobSourceDynamic identifies a runtime-created automation definition.
	JobSourceDynamic = modelpkg.JobSourceDynamic
)

// TargetKind identifies which runtime primitive an automation delegates to.
type TargetKind = modelpkg.TargetKind

const (
	// TargetKindAgent starts the existing agent-session automation path.
	TargetKindAgent = modelpkg.TargetKindAgent
	// TargetKindLoop starts a declared Loop through loop.Service.Start.
	TargetKindLoop = modelpkg.TargetKindLoop
)

// ScheduleMode identifies how a scheduled job determines its next fire time.
type ScheduleMode = modelpkg.ScheduleMode

const (
	// ScheduleModeCron evaluates a cron expression.
	ScheduleModeCron = modelpkg.ScheduleModeCron
	// ScheduleModeEvery evaluates a Go duration interval.
	ScheduleModeEvery = modelpkg.ScheduleModeEvery
	// ScheduleModeAt evaluates a one-shot RFC3339 timestamp.
	ScheduleModeAt = modelpkg.ScheduleModeAt
)

// RetryStrategy identifies how failed runs should be retried.
type RetryStrategy = modelpkg.RetryStrategy

const (
	// RetryStrategyNone disables retries after a failed run.
	RetryStrategyNone = modelpkg.RetryStrategyNone
	// RetryStrategyBackoff retries failed runs with exponential backoff.
	RetryStrategyBackoff = modelpkg.RetryStrategyBackoff
)

// RunStatus identifies the current lifecycle state of an automation run.
type RunStatus = modelpkg.RunStatus

const (
	// RunScheduled reports a run that has been accepted but not yet started.
	RunScheduled = modelpkg.RunScheduled
	// RunRunning reports a run that is actively dispatching or executing.
	RunRunning = modelpkg.RunRunning
	// RunDelegated reports a run that delegated execution into the task domain.
	RunDelegated = modelpkg.RunDelegated
	// RunCompleted reports a run that finished successfully.
	RunCompleted = modelpkg.RunCompleted
	// RunFailed reports a run that finished with an error.
	RunFailed = modelpkg.RunFailed
	// RunCancelled reports a run that was canceled before completion.
	RunCancelled = modelpkg.RunCancelled
)

// SchedulerCatchUpPolicy identifies how missed scheduled fires are reconciled.
type SchedulerCatchUpPolicy = modelpkg.SchedulerCatchUpPolicy

const (
	// SchedulerCatchUpPolicySkipMissed dispatches within grace and skips older missed fires.
	SchedulerCatchUpPolicySkipMissed = modelpkg.SchedulerCatchUpPolicySkipMissed
	// SchedulerCatchUpPolicyCoalesce dispatches one fire for the most recent missed instant.
	SchedulerCatchUpPolicyCoalesce = modelpkg.SchedulerCatchUpPolicyCoalesce
	// SchedulerCatchUpPolicyReplay dispatches missed instants chronologically.
	SchedulerCatchUpPolicyReplay = modelpkg.SchedulerCatchUpPolicyReplay
	// SchedulerCatchUpPolicyRunOnce dispatches only the latest missed instant.
	SchedulerCatchUpPolicyRunOnce = modelpkg.SchedulerCatchUpPolicyRunOnce
	// SchedulerSkipReasonGraceExceeded reports a missed fire outside its grace window.
	SchedulerSkipReasonGraceExceeded = modelpkg.SchedulerSkipReasonGraceExceeded
	// SchedulerSkipReasonSelfOverlap reports an active prior run for the same job.
	SchedulerSkipReasonSelfOverlap = modelpkg.SchedulerSkipReasonSelfOverlap
	// SchedulerSkipReasonMetadataKey identifies durable skip reason metadata.
	SchedulerSkipReasonMetadataKey = modelpkg.SchedulerSkipReasonMetadataKey
)

// SchedulerSkipReason identifies why a scheduled fire advanced without dispatch.
type SchedulerSkipReason = modelpkg.SchedulerSkipReason

// ActivationSource identifies which ingress path produced an activation envelope.
type ActivationSource = modelpkg.ActivationSource

const (
	// ActivationSourceObserver identifies observer-backed trigger ingress.
	ActivationSourceObserver = modelpkg.ActivationSourceObserver
	// ActivationSourceHook identifies hook-backed trigger ingress.
	ActivationSourceHook = modelpkg.ActivationSourceHook
	// ActivationSourceWebhook identifies external webhook ingress.
	ActivationSourceWebhook = modelpkg.ActivationSourceWebhook
	// ActivationSourceExtension identifies extension-provided ingress.
	ActivationSourceExtension = modelpkg.ActivationSourceExtension
)

// JobTaskConfig configures direct automation-to-task materialization for one job.
type JobTaskConfig = modelpkg.JobTaskConfig

// LoopTarget configures an automation fire to start a Loop.
type LoopTarget = modelpkg.LoopTarget

// Job is the canonical scheduled automation definition used by runtime and storage layers.
type Job = modelpkg.Job

// DefaultSuggestionPendingCap bounds unresolved suggestions per workspace.
const DefaultSuggestionPendingCap = modelpkg.DefaultSuggestionPendingCap

// SuggestionSource identifies the emitter that proposed an automation Job.
type SuggestionSource = modelpkg.SuggestionSource

const (
	SuggestionSourceCatalog     = modelpkg.SuggestionSourceCatalog
	SuggestionSourceUsage       = modelpkg.SuggestionSourceUsage
	SuggestionSourceIntegration = modelpkg.SuggestionSourceIntegration
)

// SuggestionStatus identifies the consent resolution state.
type SuggestionStatus = modelpkg.SuggestionStatus

const (
	SuggestionStatusPending   = modelpkg.SuggestionStatusPending
	SuggestionStatusAccepted  = modelpkg.SuggestionStatusAccepted
	SuggestionStatusDismissed = modelpkg.SuggestionStatusDismissed
)

// Suggestion is a workspace-scoped proposal for a prefilled Job.
type Suggestion = modelpkg.Suggestion

var (
	// ErrSuggestionNotFound reports an unknown workspace-owned suggestion.
	ErrSuggestionNotFound = modelpkg.ErrSuggestionNotFound
	// ErrSuggestionResolved reports a losing or repeated resolution CAS.
	ErrSuggestionResolved = modelpkg.ErrSuggestionResolved
	// ErrSuggestionPendingCap reports that a workspace already has five pending suggestions.
	ErrSuggestionPendingCap = modelpkg.ErrSuggestionPendingCap
)

// ScheduleSpec describes how a job should be scheduled.
type ScheduleSpec = modelpkg.ScheduleSpec

// Trigger is the canonical event-driven automation definition used by runtime and storage layers.
type Trigger = modelpkg.Trigger

// RetryConfig defines retry behavior for a failed automation run.
type RetryConfig = modelpkg.RetryConfig

// FireLimitConfig caps how often a job or trigger may fire within a rolling window.
type FireLimitConfig = modelpkg.FireLimitConfig

// Run records the execution state of a single automation fire.
type Run = modelpkg.Run

// SchedulerState stores the durable scheduling cursor for one automation job.
type SchedulerState = modelpkg.SchedulerState

// SchedulerClaim reserves one scheduled fire after cursor advancement.
type SchedulerClaim = modelpkg.SchedulerClaim

// SchedulerClaimResult reports the durable state and run reservation for one scheduled fire.
type SchedulerClaimResult = modelpkg.SchedulerClaimResult

// ActivationEnvelope is the normalized trigger input regardless of source.
type ActivationEnvelope = modelpkg.ActivationEnvelope
