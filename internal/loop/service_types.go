package loop

import (
	"context"
	"encoding/json"
	"time"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/gate"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/task"
)

// WorkspaceID is the workspace owner for loop aggregate state.
type WorkspaceID string

// RunID is the durable loop_runs primary key.
type RunID string

// NodeID aliases the DSL graph node identity.
type NodeID = dsl.NodeID

// Inputs carries user inputs plus runtime-only start metadata.
type Inputs struct {
	Values                     map[string]any         `json:"values,omitempty"`
	ParentLoopRunID            RunID                  `json:"parent_loop_run_id,omitempty"`
	ConfigOverrides            LoopConfig             `json:"config_overrides"`
	InheritedEnvironment       *dsl.EnvironmentSpec   `json:"-"`
	StartMetadata              map[string]any         `json:"start_metadata,omitempty"`
	NetworkParticipation       *participation.Request `json:"network_participation,omitempty"`
	NetworkParticipationSource participation.Source   `json:"-"`
	Admission                  *AdmissionIdentity     `json:"-"`
}

// Status is the closed loop_runs.status vocabulary.
type Status string

const (
	// BudgetGateID is the synthetic approval gate used by budget escalation.
	BudgetGateID NodeID = "budget"
)

const (
	// StatusQueued is a live deferred same-loop start.
	StatusQueued Status = "queued"
	// StatusRunning is a live coordinator-owned run.
	StatusRunning Status = "running"
	// StatusWatching is a dormant watch-source run.
	StatusWatching Status = "watching"
	// StatusNeedsApproval is a dormant human-gate run.
	StatusNeedsApproval Status = "needs-approval"
	// StatusPaused is a dormant operator-paused run.
	StatusPaused Status = "paused"
	// StatusDone is a verified terminal outcome.
	StatusDone Status = "done"
	// StatusNoOp is a truthful terminal no-work outcome.
	StatusNoOp Status = "no-op"
	// StatusBlocked is a terminal external-dependency outcome.
	StatusBlocked Status = "blocked"
	// StatusFailed is a terminal unrecoverable failure outcome.
	StatusFailed Status = "failed"
	// StatusExhausted is a terminal hard-limit outcome.
	StatusExhausted Status = "exhausted"
	// StatusStalled is a terminal no-progress outcome.
	StatusStalled Status = "stalled"
	// StatusCanceled is a terminal operator-canceled outcome.
	StatusCanceled Status = "canceled"
)

// TransitionCause records why a status transition happened.
type TransitionCause string

const (
	// TransitionCauseStart records initial run creation.
	TransitionCauseStart TransitionCause = "start"
	// TransitionCausePromote records queued-run promotion.
	TransitionCausePromote TransitionCause = "promote"
	// TransitionCauseOperatorCancel records a graceful operator cancellation.
	TransitionCauseOperatorCancel TransitionCause = "operator_cancel"
	// TransitionCauseOperatorKill records an immediate operator kill.
	TransitionCauseOperatorKill TransitionCause = "operator_kill"
	// TransitionCauseGoalReplace records an expected-run inline Goal replacement.
	TransitionCauseGoalReplace TransitionCause = "goal_replace"
	// TransitionCauseGoalClear records an inline Goal clear that also stops a live Run.
	TransitionCauseGoalClear TransitionCause = "goal_clear"
	// TransitionCausePauseBoundary records a generation boundary honoring pause_requested.
	TransitionCausePauseBoundary TransitionCause = "pause_boundary"
	// TransitionCauseOperatorResume records operator resume.
	TransitionCauseOperatorResume TransitionCause = "operator_resume"
	// TransitionCauseApproval records a human approval.
	TransitionCauseApproval TransitionCause = "approval"
	// TransitionCauseWaitExpired records an authored wait or approval timeout route.
	TransitionCauseWaitExpired TransitionCause = "wait_expired"
	// TransitionCauseGateRejected records a human gate rejection.
	TransitionCauseGateRejected TransitionCause = "gate_rejected"
	// TransitionCauseContract records a coordinator contract verdict.
	TransitionCauseContract TransitionCause = "contract"
	// TransitionCauseBudget records a hard budget outcome.
	TransitionCauseBudget TransitionCause = "budget"
	// TransitionCauseIterationCap records a generation iteration-cap outcome.
	TransitionCauseIterationCap TransitionCause = "iteration_cap"
	// TransitionCauseNoProgress records a coordinator no-progress outcome.
	TransitionCauseNoProgress TransitionCause = "no_progress"
	// TransitionCauseWatchPoll records a watch-source poll yielding dormancy.
	TransitionCauseWatchPoll TransitionCause = "watch_poll"
	// TransitionCauseWatchEvents records a watch-events source yielding dormancy.
	TransitionCauseWatchEvents TransitionCause = "watch_events"
	// TransitionCauseCoordinatorFailure records an execution failure before a boundary settled.
	TransitionCauseCoordinatorFailure TransitionCause = "coordinator_failure"
	// TransitionCauseOperatorRerun records reactivation from a settled generation.
	TransitionCauseOperatorRerun TransitionCause = "operator_rerun"
)

// GateDecision is the closed approval decision vocabulary consumed by Approve.
type GateDecision string

const (
	// GateDecisionApprove resumes the run.
	GateDecisionApprove GateDecision = "approve"
	// GateDecisionRequestChanges resumes the run after a requested revision.
	GateDecisionRequestChanges GateDecision = "request_changes"
	// GateDecisionReject terminates the run as blocked.
	GateDecisionReject GateDecision = "reject"
)

// ReattemptStrategy configures generation re-attempt breadth.
type ReattemptStrategy string

const (
	// ReattemptFailedOnly retries failed work only.
	ReattemptFailedOnly ReattemptStrategy = "failed_only"
	// ReattemptFullBody retries the whole generation body.
	ReattemptFullBody ReattemptStrategy = "full_body"
)

// CompletionState records whether a terminal run satisfied full or partial coverage.
type CompletionState string

const (
	CompletionComplete CompletionState = "complete"
	CompletionPartial  CompletionState = "partial"
)

func (s CompletionState) Valid() bool {
	return s == CompletionComplete || s == CompletionPartial
}

// LoopConfig is the raw per-loop or per-run override layer.
//
//nolint:revive // TechSpec "Core Interfaces" names this public type LoopConfig.
type LoopConfig struct {
	HumanGateEnabled   *bool                `json:"human_gate_enabled,omitempty"   yaml:"human_gate_enabled,omitempty"`
	ReattemptStrategy  *ReattemptStrategy   `json:"reattempt_strategy,omitempty"   yaml:"reattempt_strategy,omitempty"`
	EnabledChecks      json.RawMessage      `json:"enabled_checks_json,omitempty"  yaml:"enabled_checks_json,omitempty"`
	IterationCap       *int                 `json:"iteration_cap,omitempty"        yaml:"iteration_cap,omitempty"`
	BudgetTokens       *int                 `json:"budget_tokens,omitempty"        yaml:"budget_tokens,omitempty"`
	BudgetWallSec      *int                 `json:"budget_wall_sec,omitempty"      yaml:"budget_wall_sec,omitempty"`
	BudgetOnExceeded   *dsl.BudgetExceeded  `json:"budget_on_exceeded,omitempty"   yaml:"budget_on_exceeded,omitempty"`
	NoProgressWindow   *int                 `json:"no_progress_window,omitempty"   yaml:"no_progress_window,omitempty"`
	FanOutWidth        *int                 `json:"fan_out_width,omitempty"        yaml:"fan_out_width,omitempty"`
	GateMaxRevisions   *int                 `json:"gate_max_revisions,omitempty"   yaml:"gate_max_revisions,omitempty"`
	RuntimeDefaults    *RuntimeDefaults     `json:"runtime_defaults,omitempty"     yaml:"runtime_defaults,omitempty"`
	RuntimeRules       []RuntimeRule        `json:"runtime_rules,omitempty"        yaml:"runtime_rules,omitempty"`
	Environment        *dsl.EnvironmentSpec `json:"environment,omitempty"          yaml:"environment,omitempty"`
	Lifecycle          *LifecycleConfig     `json:"lifecycle,omitempty"            yaml:"lifecycle,omitempty"`
	RequestExpireAfter *string              `json:"request_expire_after,omitempty" yaml:"request_expire_after,omitempty"`
}

// EffectiveConfig is the fully resolved non-null runtime config.
type EffectiveConfig struct {
	HumanGateEnabled   bool                `json:"human_gate_enabled"`
	ReattemptStrategy  ReattemptStrategy   `json:"reattempt_strategy"`
	EnabledChecks      json.RawMessage     `json:"enabled_checks_json"`
	IterationCap       int                 `json:"iteration_cap"`
	BudgetTokens       int                 `json:"budget_tokens"`
	BudgetWallSec      int                 `json:"budget_wall_sec"`
	BudgetOnExceeded   dsl.BudgetExceeded  `json:"budget_on_exceeded"`
	NoProgressWindow   int                 `json:"no_progress_window"`
	FanOutWidth        int                 `json:"fan_out_width"`
	GateMaxRevisions   int                 `json:"gate_max_revisions"`
	RuntimeDefaults    RuntimeDefaults     `json:"runtime_defaults"`
	RuntimeRules       []RuntimeRule       `json:"runtime_rules"`
	RunRuntimeRules    []RuntimeRule       `json:"run_runtime_rules"`
	Environment        dsl.EnvironmentSpec `json:"environment"`
	Lifecycle          LifecycleConfig     `json:"lifecycle"`
	RequestExpireAfter string              `json:"request_expire_after"`
}

// ConfigSnapshot keeps the stored override and daemon-resolved runtime config from one read.
type ConfigSnapshot struct {
	Stored    *LoopConfig
	Effective EffectiveConfig
}

// LoopDefaults carries the `[loops.defaults.*]` layer consumed by the resolver.
//
//nolint:revive // Kept parallel to LoopConfig for the loop defaults layer.
type LoopDefaults struct {
	Delivery LoopConfig
	Watch    LoopConfig
}

// Run is the durable loop_run aggregate returned by the service.
type Run struct {
	ID                    RunID
	WorkspaceID           WorkspaceID
	LoopName              string
	Status                Status
	Generation            int
	BestGeneration        *int64
	BestScore             *float64
	ReattemptStrategy     ReattemptStrategy
	CreatedAt             time.Time
	StartedAt             time.Time
	LastProgressAt        time.Time
	StartedBy             task.ActorIdentity
	StartedOrigin         task.Origin
	DefinitionVersion     int
	DefinitionDigest      string
	DefinitionSnapshot    json.RawMessage
	ActiveGateID          NodeID
	ActiveHumanCriteria   json.RawMessage
	BudgetApprovalSeq     int
	StartMetadata         map[string]any
	IterationCap          int
	BudgetTokens          int
	BudgetWallSec         int
	BudgetOnExceeded      dsl.BudgetExceeded
	TokensUsed            int64
	ParentLoopRunID       RunID
	Historical            bool
	PauseRequested        bool
	CancelRequested       bool
	CancelKind            RunCancelKind
	ControlActor          task.ActorIdentity
	ControlRequestedAt    time.Time
	GoalContextNudgeRatio float64
	*RunStartState
	Origin *RunOrigin
	Inputs map[string]any
}

// RunAdmission groups the transient watch-admission command and its optional suppression answer.
type RunAdmission struct {
	Identity   AdmissionIdentity
	Suppressed bool
	Claim      *AdmissionClaim
}

// DefinitionSnapshot is the content-addressed executed definition pinned by one or more runs.
type DefinitionSnapshot struct {
	WorkspaceID WorkspaceID
	Digest      string
	Version     int
	Definition  json.RawMessage
	ByteSize    int
	CreatedAt   time.Time
	LastUsedAt  time.Time
}

// GateDecisionRecord persists one human approval decision for a gate criterion.
type GateDecisionRecord struct {
	WorkspaceID WorkspaceID
	RunID       RunID
	Generation  int
	GateID      NodeID
	CriterionID string
	Decision    GateDecision
	Actor       task.ActorContext
	Note        string
	DecidedAt   time.Time
}

// PlanNodePreview is one gen-1 node materialized by DryRun.
type PlanNodePreview struct {
	ID        dsl.NodeID    `json:"id"`
	Class     dsl.NodeClass `json:"class"`
	Kind      string        `json:"kind"`
	DependsOn []dsl.NodeID  `json:"depends_on,omitempty"`
}

// PlanPreview is the no-state preview returned by DryRun.
type PlanPreview struct {
	LoopName                     string                 `json:"loop_name"`
	ResolvedInputs               map[string]any         `json:"resolved_inputs"`
	InputOrigins                 map[string]InputOrigin `json:"input_origins"`
	Generation                   int                    `json:"generation"`
	Nodes                        []PlanNodePreview      `json:"nodes"`
	Contract                     dsl.Contract           `json:"contract"`
	MaterializedContract         dsl.Contract           `json:"materialized_contract"`
	EffectiveConfig              EffectiveConfig        `json:"effective_config"`
	ResolvedNetworkParticipation participation.Spec     `json:"resolved_network_participation"`
}

// DisplayCost is derived UI-only cost information.
type DisplayCost struct {
	Tokens           int64   `json:"tokens"`
	PricePerTokenUSD float64 `json:"price_per_token_usd"`
	USD              float64 `json:"usd"`
}

// DefinitionResolver resolves a loop name to a compiled definition.
type DefinitionResolver interface {
	ResolveLoop(ctx context.Context, ws WorkspaceID, name string) (*ResolvedDefinition, error)
}

// DefinitionResolverFunc adapts a function to DefinitionResolver.
type DefinitionResolverFunc func(context.Context, WorkspaceID, string) (*ResolvedDefinition, error)

// ResolveLoop implements DefinitionResolver.
func (f DefinitionResolverFunc) ResolveLoop(
	ctx context.Context,
	ws WorkspaceID,
	name string,
) (*ResolvedDefinition, error) {
	return f(ctx, ws, name)
}

// Store is the loop aggregate persistence contract.
type Store interface {
	CreateLoopRunForStart(ctx context.Context, run Run, policy dsl.ConcurrencyPolicy) (Run, error)
	GetLoopRun(ctx context.Context, ws WorkspaceID, runID RunID) (Run, error)
	GetLoopRunByID(ctx context.Context, runID RunID) (Run, error)
	GetLoopDefinitionSnapshot(ctx context.Context, ws WorkspaceID, digest string) (DefinitionSnapshot, error)
	FindActiveLoopRun(ctx context.Context, ws WorkspaceID, loopName string) (*Run, error)
	CompareAndSwapLoopRunStatus(
		ctx context.Context,
		runID RunID,
		from Status,
		to Status,
		cause TransitionCause,
		at time.Time,
	) error
	RecordLoopGateDecisions(ctx context.Context, records []GateDecisionRecord) error
	ListLoopGateDecisions(
		ctx context.Context,
		ws WorkspaceID,
		runID RunID,
		generation int,
		gateID NodeID,
	) (map[string]gate.HumanDecision, error)
	SetLoopRunPauseRequested(
		ctx context.Context,
		ws WorkspaceID,
		runID RunID,
		requested bool,
		actor task.ActorContext,
	) error
	UpsertLoopConfig(ctx context.Context, ws WorkspaceID, loopName string, cfg LoopConfig) error
	GetLoopConfig(ctx context.Context, ws WorkspaceID, loopName string) (*LoopConfig, error)
}

// Service is the task_04 loop aggregate API surface.
type Service interface {
	Start(ctx context.Context, ws WorkspaceID, name string, inputs Inputs, actor task.ActorContext) (*Run, error)
	StartInline(
		ctx context.Context,
		ws WorkspaceID,
		definition dsl.Definition,
		inputs Inputs,
		origin RunOrigin,
		actor task.ActorContext,
	) (*Run, error)
	ReplaceInline(
		ctx context.Context,
		expectedRunID RunID,
		ws WorkspaceID,
		definition dsl.Definition,
		inputs Inputs,
		origin RunOrigin,
		actor task.ActorContext,
	) (InlineReplaceResult, error)
	ClearInlineGoal(
		ctx context.Context,
		ws WorkspaceID,
		originSessionID string,
		actor task.ActorContext,
	) error
	DryRun(ctx context.Context, ws WorkspaceID, name string, inputs Inputs) (*PlanPreview, error)
	CancelRun(ctx context.Context, ws WorkspaceID, runID RunID, reason string, actor task.ActorContext) error
	KillRun(ctx context.Context, ws WorkspaceID, runID RunID, reason string, actor task.ActorContext) error
	CancelNode(
		ctx context.Context,
		ws WorkspaceID,
		runID RunID,
		nodeID NodeID,
		itemIndex *int,
		reason string,
		actor task.ActorContext,
	) error
	KillNode(
		ctx context.Context,
		ws WorkspaceID,
		runID RunID,
		nodeID NodeID,
		itemIndex *int,
		reason string,
		actor task.ActorContext,
	) error
	Pause(ctx context.Context, ws WorkspaceID, runID RunID, actor task.ActorContext) error
	// Resume clears pause_requested on running runs or transitions paused runs back to running.
	Resume(ctx context.Context, ws WorkspaceID, runID RunID, actor task.ActorContext) error
	Approve(
		ctx context.Context,
		ws WorkspaceID,
		runID RunID,
		gateID NodeID,
		decision GateDecision,
		actor task.ActorContext,
	) error
	ListRequests(ctx context.Context, ws WorkspaceID, query RequestQuery) (RequestPage, error)
	GetRequest(ctx context.Context, ws WorkspaceID, ref RequestRef) (RequestDetail, error)
	Respond(ctx context.Context, input RespondInput) (RespondResult, error)
	AmendNodeOutput(ctx context.Context, input AmendInput) (NodeAmendment, error)
	Configure(ctx context.Context, ws WorkspaceID, name string, cfg LoopConfig) error
	GetConfig(ctx context.Context, ws WorkspaceID, name string) (*LoopConfig, error)
	GetConfigSnapshot(ctx context.Context, ws WorkspaceID, name string) (ConfigSnapshot, error)
	Get(ctx context.Context, ws WorkspaceID, runID RunID) (*Run, error)
	Transition(ctx context.Context, runID RunID, to Status, cause TransitionCause) error
}

// TimeTravelService exposes the optional historical read and mutation surface.
type TimeTravelService interface {
	DiffRun(ctx context.Context, workspaceID WorkspaceID, query DiffQuery) (DiffResult, error)
	RerunFromNode(ctx context.Context, input RerunInput) (RerunResult, error)
	ForkRun(ctx context.Context, input ForkInput) (StartResult, error)
}
