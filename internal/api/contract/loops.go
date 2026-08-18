package contract

import (
	"encoding/json"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/network/participation"
)

// LoopSource is the public source-provenance tier for a Loop definition.
type LoopSource string

const (
	LoopSourceMarketplace LoopSource = "marketplace"
	LoopSourceUser        LoopSource = "user"
	LoopSourceAdditional  LoopSource = "additional"
	LoopSourceWorkspace   LoopSource = "workspace"
)

// LoopCompletionState distinguishes full success from accepted partial completion.
type LoopCompletionState string

const (
	LoopCompletionStateComplete LoopCompletionState = "complete"
	LoopCompletionStatePartial  LoopCompletionState = "partial"
)

// LoopRunStatus is the public loop run state vocabulary.
type LoopRunStatus string

const (
	LoopRunStatusQueued        LoopRunStatus = "queued"
	LoopRunStatusRunning       LoopRunStatus = "running"
	LoopRunStatusWatching      LoopRunStatus = "watching"
	LoopRunStatusNeedsApproval LoopRunStatus = "needs-approval"
	LoopRunStatusPaused        LoopRunStatus = "paused"
	LoopRunStatusDone          LoopRunStatus = "done"
	LoopRunStatusNoOp          LoopRunStatus = "no-op"
	LoopRunStatusBlocked       LoopRunStatus = "blocked"
	LoopRunStatusFailed        LoopRunStatus = "failed"
	LoopRunStatusExhausted     LoopRunStatus = "exhausted"
	LoopRunStatusStalled       LoopRunStatus = "stalled"
	LoopRunStatusCanceled      LoopRunStatus = "canceled"
)

// LoopRunTransitionCause identifies why a run status changed.
type LoopRunTransitionCause string

const (
	LoopRunTransitionCauseStart              LoopRunTransitionCause = "start"
	LoopRunTransitionCausePromote            LoopRunTransitionCause = "promote"
	LoopRunTransitionCauseOperatorCancel     LoopRunTransitionCause = "operator_cancel"
	LoopRunTransitionCauseOperatorKill       LoopRunTransitionCause = "operator_kill"
	LoopRunTransitionCauseGoalReplace        LoopRunTransitionCause = "goal_replace"
	LoopRunTransitionCauseGoalClear          LoopRunTransitionCause = "goal_clear"
	LoopRunTransitionCausePauseBoundary      LoopRunTransitionCause = "pause_boundary"
	LoopRunTransitionCauseOperatorResume     LoopRunTransitionCause = "operator_resume"
	LoopRunTransitionCauseApproval           LoopRunTransitionCause = "approval"
	LoopRunTransitionCauseWaitExpired        LoopRunTransitionCause = "wait_expired"
	LoopRunTransitionCauseGateRejected       LoopRunTransitionCause = "gate_rejected"
	LoopRunTransitionCauseContract           LoopRunTransitionCause = "contract"
	LoopRunTransitionCauseBudget             LoopRunTransitionCause = "budget"
	LoopRunTransitionCauseIterationCap       LoopRunTransitionCause = "iteration_cap"
	LoopRunTransitionCauseNoProgress         LoopRunTransitionCause = "no_progress"
	LoopRunTransitionCauseWatchPoll          LoopRunTransitionCause = "watch_poll"
	LoopRunTransitionCauseWatchEvents        LoopRunTransitionCause = "watch_events"
	LoopRunTransitionCauseCoordinatorFailure LoopRunTransitionCause = "coordinator_failure"
	LoopRunTransitionCauseOperatorRerun      LoopRunTransitionCause = "operator_rerun"
)

// LoopNodeClass is the public loop graph node class vocabulary.
type LoopNodeClass string

const (
	LoopNodeClassAction  LoopNodeClass = "action"
	LoopNodeClassControl LoopNodeClass = "control"
	LoopNodeClassSource  LoopNodeClass = "source"
)

// LoopReattemptStrategy configures generation re-attempt breadth.
type LoopReattemptStrategy string

const (
	LoopReattemptFailedOnly LoopReattemptStrategy = "failed_only"
	LoopReattemptFullBody   LoopReattemptStrategy = "full_body"
)

// LoopBudgetExceeded controls the outcome for a budget breach.
type LoopBudgetExceeded string

const (
	LoopBudgetExceededHalt     LoopBudgetExceeded = "halt"
	LoopBudgetExceededEscalate LoopBudgetExceeded = "escalate"
)

// LoopGateDecision is the human gate approval decision vocabulary.
type LoopGateDecision string

const (
	LoopGateDecisionApprove        LoopGateDecision = "approve"
	LoopGateDecisionRequestChanges LoopGateDecision = "request_changes"
	LoopGateDecisionReject         LoopGateDecision = "reject"
)

// LoopMetricDirection determines whether a gate metric should increase or decrease.
type LoopMetricDirection string

const (
	LoopMetricMaximize LoopMetricDirection = "maximize"
	LoopMetricMinimize LoopMetricDirection = "minimize"
)

// LoopLintSeverity classifies a lint diagnostic.
type LoopLintSeverity string

const (
	LoopLintSeverityError   LoopLintSeverity = "error"
	LoopLintSeverityWarning LoopLintSeverity = "warning"
)

// LoopDefinitionPayload is the inspect/publish response for one Loop definition.
type LoopDefinitionPayload struct {
	Name               string                       `json:"name"`
	Version            int                          `json:"version"`
	Description        string                       `json:"description,omitempty"`
	Source             LoopSource                   `json:"source"`
	Catalog            LoopCatalogResourceSpec      `json:"catalog"`
	Definition         LoopDefinitionDocument       `json:"definition"`
	EffectiveLifecycle *LoopResolvedLifecycleConfig `json:"effective_lifecycle,omitempty"`
}

// LoopResolvedLifecycleConfig reports effective node-family defaults and their source layer.
type LoopResolvedLifecycleConfig struct {
	RetryMaxAttempts       int               `json:"retry_max_attempts"`
	RetryBackoffBase       string            `json:"retry_backoff_base"`
	RetryBackoffMax        string            `json:"retry_backoff_max"`
	RetryNonRetryable      []string          `json:"retry_non_retryable"`
	LivenessSilenceWindow  string            `json:"liveness_silence_window"`
	ResumeDeathStreakLimit int               `json:"resume_death_streak_limit"`
	PredicateCostLimit     uint64            `json:"predicate_cost_limit"`
	WaitAdmissionAttempts  int               `json:"wait_admission_attempts"`
	WaitAdmissionInterval  string            `json:"wait_admission_interval"`
	AdmissionHorizon       string            `json:"admission_horizon"`
	Sources                map[string]string `json:"sources"`
}

// LoopResponse wraps one Loop definition.
type LoopResponse struct {
	Loop LoopDefinitionPayload `json:"loop"`
}

// CreateLoopRequest creates a writable Loop definition or forks a read-only Loop by name.
type CreateLoopRequest struct {
	Definition   *LoopDefinitionDocument `json:"definition,omitempty"`
	ForkFromName string                  `json:"fork_from_name,omitempty"`
}

// PatchLoopRequest atomically lints, compiles, and publishes a Loop definition.
type PatchLoopRequest struct {
	ExpectedVersion *int                   `json:"expected_version,omitempty"`
	Definition      LoopDefinitionDocument `json:"definition"`
}

// ValidateLoopRequest lints and compiles a Loop definition without saving it.
type ValidateLoopRequest struct {
	Definition LoopDefinitionDocument `json:"definition"`
}

// LoopValidationResponse reports lint/compile diagnostics.
type LoopValidationResponse struct {
	Valid             bool                               `json:"valid"`
	Errors            []LoopLintErrorPayload             `json:"errors,omitempty"`
	RuntimeValidation []LoopRuntimeValidationItemPayload `json:"runtime_validation,omitempty"`
	InputDefault      *LoopInputDefaultErrorPayload      `json:"input_default,omitempty"`
}

// LoopInputDefaultErrorPayload reports one input resolution failure.
type LoopInputDefaultErrorPayload struct {
	Loop   string `json:"loop"`
	Key    string `json:"key"`
	Reason string `json:"reason"`
}

// LoopLintErrorPayload is the per-node 422 payload surfaced to authoring clients.
type LoopLintErrorPayload struct {
	NodeID   string           `json:"node_id,omitempty"`
	Code     string           `json:"code"`
	Message  string           `json:"message"`
	Severity LoopLintSeverity `json:"severity"`
}

// LoopVersionConflictResponse reports the current published version after a CAS miss.
type LoopVersionConflictResponse struct {
	Error          string `json:"error"`
	CurrentVersion int    `json:"current_version"`
}

// RunLoopRequest starts a Loop, or previews it when dry=true.
type RunLoopRequest struct {
	Inputs               map[string]any         `json:"inputs,omitempty"`
	ParentLoopRunID      string                 `json:"parent_loop_run_id,omitempty"`
	ConfigOverrides      *LoopConfig            `json:"config_overrides,omitempty"`
	NetworkParticipation *participation.Request `json:"network_participation,omitempty"`
}

// RunLoopResponse returns either a persisted run or a dry-run plan preview.
type RunLoopResponse struct {
	Run    *LoopRunPayload  `json:"run,omitempty"`
	DryRun *LoopPlanPayload `json:"dry_run,omitempty"`
	WebURL string           `json:"web_url,omitempty"`
}

// LoopRunWebRoute is the public SPA route for one persisted Loop run.
const LoopRunWebRoute = "/loop-runs/%s"

// LoopPlanPayload is the public dry-run preview.
type LoopPlanPayload struct {
	LoopName                     string                     `json:"loop_name"`
	ResolvedInputs               map[string]any             `json:"resolved_inputs"`
	InputOrigins                 map[string]LoopInputOrigin `json:"input_origins"`
	Generation                   int                        `json:"generation"`
	Nodes                        []LoopPlanNodePreview      `json:"nodes"`
	Contract                     LoopContract               `json:"contract"`
	MaterializedContract         LoopContract               `json:"materialized_contract"`
	EffectiveConfig              LoopEffectiveConfig        `json:"effective_config"`
	ResolvedNetworkParticipation *participation.Spec        `json:"resolved_network_participation"`
}

// LoopInputOrigin is the public effective-input provenance vocabulary.
type LoopInputOrigin string

const (
	LoopInputOriginRun        LoopInputOrigin = "run"
	LoopInputOriginWorkspace  LoopInputOrigin = "workspace"
	LoopInputOriginGlobal     LoopInputOrigin = "global"
	LoopInputOriginDefinition LoopInputOrigin = "definition"
)

// LoopPlanNodePreview is one gen-1 node materialized by dry-run.
type LoopPlanNodePreview struct {
	ID        string        `json:"id"`
	Class     LoopNodeClass `json:"class"`
	Kind      string        `json:"kind"`
	DependsOn []string      `json:"depends_on,omitempty"`
}

// LoopConfigResponse returns the stored override and daemon-resolved runtime config.
type LoopConfigResponse struct {
	Config          *LoopConfig         `json:"config"`
	EffectiveConfig LoopEffectiveConfig `json:"effective_config"`
}

// PutLoopConfigRequest replaces the stored no-fork config override.
type PutLoopConfigRequest struct {
	Config LoopConfig `json:"config"`
}

// LoopEnvironmentMode selects the execution root for Loop agent actions.
type LoopEnvironmentMode string

const (
	LoopEnvironmentModeRoot      LoopEnvironmentMode = "root"
	LoopEnvironmentModeWorktree  LoopEnvironmentMode = "worktree"
	LoopEnvironmentModePerRun    LoopEnvironmentMode = "per_run"
	LoopEnvironmentModeDirectory LoopEnvironmentMode = "directory"
)

// LoopEnvironment is one loop-level execution environment default.
type LoopEnvironment struct {
	Mode        LoopEnvironmentMode `json:"mode"`
	WorktreeRef string              `json:"worktree_ref,omitempty"`
	Directory   string              `json:"directory,omitempty"`
}

// LoopConfig is the public per-loop or per-run override layer.
type LoopConfig struct {
	HumanGateEnabled  *bool                  `json:"human_gate_enabled,omitempty"`
	ReattemptStrategy *LoopReattemptStrategy `json:"reattempt_strategy,omitempty"`
	EnabledChecks     json.RawMessage        `json:"enabled_checks_json,omitempty"`
	IterationCap      *int                   `json:"iteration_cap,omitempty"`
	BudgetTokens      *int                   `json:"budget_tokens,omitempty"`
	BudgetWallSec     *int                   `json:"budget_wall_sec,omitempty"`
	BudgetOnExceeded  *LoopBudgetExceeded    `json:"budget_on_exceeded,omitempty"`
	NoProgressWindow  *int                   `json:"no_progress_window,omitempty"`
	FanOutWidth       *int                   `json:"fan_out_width,omitempty"`
	GateMaxRevisions  *int                   `json:"gate_max_revisions,omitempty"`
	RuntimeDefaults   *LoopRuntimeDefaults   `json:"runtime_defaults,omitempty"`
	RuntimeRules      []LoopRuntimeRule      `json:"runtime_rules,omitempty"`
	Environment       *LoopEnvironment       `json:"environment,omitempty"`
}

// LoopEffectiveConfig is the public fully resolved runtime config.
type LoopEffectiveConfig struct {
	HumanGateEnabled  bool                  `json:"human_gate_enabled"`
	ReattemptStrategy LoopReattemptStrategy `json:"reattempt_strategy"`
	EnabledChecks     json.RawMessage       `json:"enabled_checks_json"`
	IterationCap      int                   `json:"iteration_cap"`
	BudgetTokens      int                   `json:"budget_tokens"`
	BudgetWallSec     int                   `json:"budget_wall_sec"`
	BudgetOnExceeded  LoopBudgetExceeded    `json:"budget_on_exceeded"`
	NoProgressWindow  int                   `json:"no_progress_window"`
	FanOutWidth       int                   `json:"fan_out_width"`
	GateMaxRevisions  int                   `json:"gate_max_revisions"`
	RuntimeDefaults   LoopRuntimeDefaults   `json:"runtime_defaults"`
	RuntimeRules      []LoopRuntimeRule     `json:"runtime_rules"`
	RunRuntimeRules   []LoopRuntimeRule     `json:"run_runtime_rules,omitempty"`
	Environment       LoopEnvironment       `json:"environment"`
}

// LoopAnnotationPayload is one editor node position.
type LoopAnnotationPayload struct {
	NodeID string  `json:"node_id"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
}

// LoopAnnotationsResponse returns editor sidecar positions.
type LoopAnnotationsResponse struct {
	Annotations []LoopAnnotationPayload `json:"annotations"`
}

// PutLoopAnnotationsRequest replaces editor sidecar positions for one Loop.
type PutLoopAnnotationsRequest struct {
	Annotations []LoopAnnotationPayload `json:"annotations"`
}

// LoopInput is the bounded input projection returned by the Loop catalog.
type LoopInput struct {
	Type        string        `json:"type"`
	Required    bool          `json:"required,omitempty"`
	Description string        `json:"description,omitempty"`
	Ref         *LoopInputRef `json:"ref,omitempty"`
	Default     any           `json:"default,omitempty"`
}

type LoopInputRef struct {
	Kind string `json:"kind"`
}

type LoopContract struct {
	Goal             string               `json:"goal"`
	DefinitionOfDone string               `json:"definition_of_done"`
	Constraints      []string             `json:"constraints,omitempty"`
	Boundaries       []string             `json:"boundaries,omitempty"`
	StopWhen         dsl.StopWhenSpec     `json:"stop_when,omitzero"`
	Verification     []LoopGateCriterion  `json:"verification,omitempty"`
	TerminalStates   []string             `json:"terminal_states,omitempty"`
	IterationCap     int                  `json:"iteration_cap"`
	NoProgress       LoopNoProgress       `json:"no_progress"`
	Budget           LoopBudget           `json:"budget"`
	RuntimeDefaults  *LoopRuntimeDefaults `json:"runtime_defaults,omitempty"`
	RuntimeRules     []LoopRuntimeRule    `json:"runtime_rules,omitempty"`
	*LoopContractLifecycleState
}

type LoopNoProgress struct {
	Window int `json:"window"`
}

type LoopBudget struct {
	Tokens       int                `json:"tokens"`
	WallClockSec int                `json:"wall_clock_sec"`
	OnExceeded   LoopBudgetExceeded `json:"on_exceeded,omitempty"`
}

type LoopGateCriterion struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Check    string `json:"check,omitempty"`
	Expect   string `json:"expect,omitempty"`
	Contains string `json:"contains,omitempty"`
	Agent    string `json:"agent,omitempty"`
	//nolint:modernize // OpenAPI requires omitempty; JSON uses omitzero.
	Runtime LoopRuntimeSpec `json:"runtime,omitempty,omitzero"`
	Rubric  string          `json:"rubric,omitempty"`
	Prompt  string          `json:"prompt,omitempty"`
	Tool    string          `json:"tool,omitempty"`
	Inputs  map[string]any  `json:"inputs,omitempty"`
	Metric  *LoopMetricSpec `json:"metric,omitempty"`
}

// LoopMetricSpec declares the score contract for one machine criterion.
type LoopMetricSpec struct {
	Direction LoopMetricDirection `json:"direction"`
	MinDelta  *float64            `json:"min_delta,omitempty"`
}

// LoopStartBinding is the bounded start projection returned by the Loop catalog.
type LoopStartBinding struct {
	Kind         string            `json:"kind"`
	Inputs       map[string]any    `json:"inputs,omitempty"`
	InputMapping map[string]string `json:"input_mapping,omitempty"`
}
