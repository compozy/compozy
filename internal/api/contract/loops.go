package contract

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/compozy/agh/internal/network/participation"
)

// LoopSource is the public source-provenance tier for a Loop definition.
type LoopSource string

const (
	LoopSourceMarketplace LoopSource = "marketplace"
	LoopSourceUser        LoopSource = "user"
	LoopSourceAdditional  LoopSource = "additional"
	LoopSourceWorkspace   LoopSource = "workspace"
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

// LoopLintSeverity classifies a lint diagnostic.
type LoopLintSeverity string

const (
	LoopLintSeverityError   LoopLintSeverity = "error"
	LoopLintSeverityWarning LoopLintSeverity = "warning"
)

// LoopDefinitionPayload is the inspect/publish response for one Loop definition.
type LoopDefinitionPayload struct {
	Name        string                  `json:"name"`
	Version     int                     `json:"version"`
	Description string                  `json:"description,omitempty"`
	Source      LoopSource              `json:"source"`
	Catalog     LoopCatalogResourceSpec `json:"catalog"`
	Definition  LoopDefinitionDocument  `json:"definition"`
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
	Valid  bool                   `json:"valid"`
	Errors []LoopLintErrorPayload `json:"errors,omitempty"`
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
}

// LoopPlanPayload is the public dry-run preview.
type LoopPlanPayload struct {
	LoopName                     string                `json:"loop_name"`
	ResolvedInputs               map[string]any        `json:"resolved_inputs"`
	Generation                   int                   `json:"generation"`
	Nodes                        []LoopPlanNodePreview `json:"nodes"`
	Contract                     LoopContract          `json:"contract"`
	EffectiveConfig              LoopEffectiveConfig   `json:"effective_config"`
	ResolvedNetworkParticipation *participation.Spec   `json:"resolved_network_participation"`
}

// LoopPlanNodePreview is one gen-1 node materialized by dry-run.
type LoopPlanNodePreview struct {
	ID        string        `json:"id"`
	Class     LoopNodeClass `json:"class"`
	Kind      string        `json:"kind"`
	DependsOn []string      `json:"depends_on,omitempty"`
}

// LoopConfigResponse returns the stored override and daemon-resolved runtime config.
type LoopConfigResponse struct {
	Config          *LoopConfig         `json:"config,omitempty"`
	EffectiveConfig LoopEffectiveConfig `json:"effective_config"`
}

// PutLoopConfigRequest replaces the stored no-fork config override.
type PutLoopConfigRequest struct {
	Config LoopConfig `json:"config"`
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
	ModelDefaults     *LoopModelDefaults     `json:"model_defaults,omitempty"`
}

// LoopModelDefaults is one public loop model-default override layer.
type LoopModelDefaults struct {
	Worker *string `json:"worker,omitempty"`
	Judge  *string `json:"judge,omitempty"`
}

// LoopEffectiveConfig is the public fully resolved runtime config.
type LoopEffectiveConfig struct {
	HumanGateEnabled  bool                       `json:"human_gate_enabled"`
	ReattemptStrategy LoopReattemptStrategy      `json:"reattempt_strategy"`
	EnabledChecks     json.RawMessage            `json:"enabled_checks_json"`
	IterationCap      int                        `json:"iteration_cap"`
	BudgetTokens      int                        `json:"budget_tokens"`
	BudgetWallSec     int                        `json:"budget_wall_sec"`
	BudgetOnExceeded  LoopBudgetExceeded         `json:"budget_on_exceeded"`
	NoProgressWindow  int                        `json:"no_progress_window"`
	FanOutWidth       int                        `json:"fan_out_width"`
	GateMaxRevisions  int                        `json:"gate_max_revisions"`
	ModelDefaults     LoopEffectiveModelDefaults `json:"model_defaults"`
}

// LoopEffectiveModelDefaults is the public resolved loop-owned model routing.
type LoopEffectiveModelDefaults struct {
	Worker string `json:"worker"`
	Judge  string `json:"judge"`
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

// LoopRunPayload is the public loop_run aggregate projection.
type LoopRunPayload struct {
	ID                           string                `json:"id"`
	WorkspaceID                  string                `json:"workspace_id"`
	LoopName                     string                `json:"loop_name"`
	Status                       LoopRunStatus         `json:"status"`
	Generation                   int                   `json:"generation"`
	ReattemptStrategy            LoopReattemptStrategy `json:"reattempt_strategy"`
	CreatedAt                    time.Time             `json:"created_at"`
	StartedAt                    time.Time             `json:"started_at"`
	LastProgressAt               time.Time             `json:"last_progress_at"`
	StartedByKind                string                `json:"started_by_kind,omitempty"`
	StartedByRef                 string                `json:"started_by_ref,omitempty"`
	StartedOriginKind            string                `json:"started_origin_kind,omitempty"`
	StartedOriginRef             string                `json:"started_origin_ref,omitempty"`
	DefinitionVersion            int                   `json:"definition_version"`
	DefinitionDigest             string                `json:"definition_digest,omitempty"`
	ActiveGateID                 string                `json:"active_gate_id,omitempty"`
	BudgetApprovalSeq            int                   `json:"budget_approval_seq,omitempty"`
	StartMetadata                map[string]any        `json:"start_metadata,omitempty"`
	IterationCap                 int                   `json:"iteration_cap"`
	BudgetTokens                 int                   `json:"budget_tokens"`
	BudgetWallSec                int                   `json:"budget_wall_sec"`
	BudgetOnExceeded             LoopBudgetExceeded    `json:"budget_on_exceeded"`
	TokensUsed                   int64                 `json:"tokens_used"`
	ParentLoopRunID              string                `json:"parent_loop_run_id,omitempty"`
	PauseRequested               bool                  `json:"pause_requested"`
	Inputs                       map[string]any        `json:"inputs,omitempty"`
	ResolvedNetworkParticipation *participation.Spec   `json:"resolved_network_participation"`
}

// LoopRunsResponse lists workspace-scoped loop runs.
type LoopRunsResponse struct {
	Runs       []LoopRunPayload         `json:"runs"`
	Aggregates LoopRunsAggregatePayload `json:"aggregates"`
}

// LoopRunsAggregatePayload summarizes the returned run set.
type LoopRunsAggregatePayload struct {
	Total     int `json:"total"`
	Live      int `json:"live"`
	Terminal  int `json:"terminal"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
}

// LoopRunResponse returns one run with generation detail.
type LoopRunResponse struct {
	Run                LoopRunPayload          `json:"run"`
	ExecutedDefinition *LoopDefinitionDocument `json:"executed_definition,omitempty"`
	Generations        []LoopGenerationPayload `json:"generations,omitempty"`
	// WatchEvents is the parked watch-events read-model (present only while dormant).
	WatchEvents *LoopWatchEventsState `json:"watch_events,omitempty"`
}

// LoopGenerationPayload groups node state by generation.
type LoopGenerationPayload struct {
	Generation int                    `json:"generation"`
	Outputs    []LoopGenerationOutput `json:"outputs"`
}

// LoopGenerationOutput is one public generation output row.
type LoopGenerationOutput struct {
	Generation     int    `json:"generation,omitempty"`
	NodeID         string `json:"node_id"`
	ItemIndex      int    `json:"item_index,omitempty"`
	Status         string `json:"status"`
	OutputRef      string `json:"output_ref,omitempty"`
	TaskRunID      string `json:"task_run_id,omitempty"`
	ChildLoopRunID string `json:"child_loop_run_id,omitempty"`
}

// LoopRunEventPayload is one SSE/audit event for a loop run.
type LoopRunEventPayload struct {
	ID          string           `json:"id"`
	LoopRunID   string           `json:"loop_run_id"`
	WorkspaceID string           `json:"workspace_id"`
	Seq         int64            `json:"seq"`
	Kind        LoopRunEventKind `json:"kind"`
	Payload     json.RawMessage  `json:"payload"`
	At          time.Time        `json:"at"`
}

// ApproveLoopRunRequest applies a human-gate decision.
type ApproveLoopRunRequest struct {
	GateID   string           `json:"gate_id"`
	Decision LoopGateDecision `json:"decision"`
}

// LoopDefinitionDocument is the public agh.loop/v1 authoring document.
type LoopDefinitionDocument struct {
	APIVersion           string                 `json:"apiVersion"`
	Kind                 string                 `json:"kind"`
	Meta                 LoopDefinitionMeta     `json:"meta"`
	Concurrency          string                 `json:"concurrency,omitempty"`
	Inputs               map[string]LoopInput   `json:"inputs,omitempty"`
	Contract             LoopContract           `json:"contract"`
	Graph                LoopGraph              `json:"graph"`
	Start                []LoopStartBinding     `json:"start,omitempty"`
	NetworkParticipation *participation.Request `json:"network_participation,omitempty"`
}

// NewLoopDefinitionDocument converts any JSON-compatible loop document into the public DTO.
func NewLoopDefinitionDocument(value any) (LoopDefinitionDocument, error) {
	var out LoopDefinitionDocument
	if err := transcodeLoopDTO(value, &out); err != nil {
		return LoopDefinitionDocument{}, err
	}
	return out, nil
}

// Decode converts the public loop document into a caller-owned engine or SDK type.
func (d LoopDefinitionDocument) Decode(target any) error {
	return transcodeLoopDTO(d, target)
}

type LoopDefinitionMeta struct {
	Name        string          `json:"name"`
	Version     int             `json:"version,omitempty"`
	Description string          `json:"description,omitempty"`
	Catalog     LoopCatalogMeta `json:"catalog"`
}

type LoopCatalogMeta struct {
	UseWhen  string   `json:"use_when,omitempty"`
	Keywords []string `json:"keywords,omitempty"`
	Category string   `json:"category,omitempty"`
}

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
	Goal             string                     `json:"goal"`
	DefinitionOfDone string                     `json:"definition_of_done"`
	Constraints      []string                   `json:"constraints,omitempty"`
	Boundaries       []string                   `json:"boundaries,omitempty"`
	StopWhen         string                     `json:"stop_when,omitempty"`
	Verification     []LoopGateCriterion        `json:"verification,omitempty"`
	TerminalStates   []string                   `json:"terminal_states,omitempty"`
	IterationCap     int                        `json:"iteration_cap"`
	NoProgress       LoopNoProgress             `json:"no_progress"`
	Budget           LoopBudget                 `json:"budget"`
	ModelDefaults    *LoopContractModelDefaults `json:"model_defaults,omitempty"`
}

type LoopNoProgress struct {
	Window     int      `json:"window"`
	HashFields []string `json:"hash_fields,omitempty"`
}

type LoopBudget struct {
	Tokens       int                `json:"tokens"`
	WallClockSec int                `json:"wall_clock_sec"`
	OnExceeded   LoopBudgetExceeded `json:"on_exceeded,omitempty"`
}

// LoopContractModelDefaults defines authored model defaults for loop-owned sessions.
type LoopContractModelDefaults struct {
	Worker string `json:"worker,omitempty"`
	Judge  string `json:"judge,omitempty"`
}

type LoopGraph struct {
	Nodes []LoopGraphNode `json:"nodes"`
	Edges []LoopGraphEdge `json:"edges"`
}

type LoopGraphNode struct {
	ID            string                       `json:"id"`
	Class         LoopNodeClass                `json:"class"`
	Kind          string                       `json:"kind"`
	Session       map[string]any               `json:"session,omitempty"`
	Timeout       string                       `json:"timeout,omitempty"`
	Retry         map[string]any               `json:"retry,omitempty"`
	Harvest       map[string]any               `json:"harvest,omitempty"`
	Produces      map[string]any               `json:"produces,omitempty"`
	Params        map[string]any               `json:"params,omitempty"`
	Collection    string                       `json:"collection,omitempty"`
	Filter        string                       `json:"filter,omitempty"`
	BatchSize     int                          `json:"batch_size,omitempty"`
	MaxParallel   int                          `json:"max_parallel,omitempty"`
	MaxFanOut     int                          `json:"max_fan_out,omitempty"`
	Condition     string                       `json:"condition,omitempty"`
	Criteria      []LoopGateCriterion          `json:"criteria,omitempty"`
	VerdictPolicy string                       `json:"verdict_policy,omitempty"`
	OnResult      map[string]any               `json:"on_result,omitempty"`
	MaxRevisions  int                          `json:"max_revisions,omitempty"`
	Body          *LoopGraph                   `json:"body,omitempty"`
	Contract      *LoopContract                `json:"contract,omitempty"`
	InputRef      string                       `json:"input_ref,omitempty"`
	Pattern       string                       `json:"pattern,omitempty"`
	Parse         string                       `json:"parse,omitempty"`
	WatchSpec     map[string]any               `json:"watch,omitempty"`
	Events        []LoopWatchEventSubscription `json:"events,omitempty"`
}

type LoopGateCriterion struct {
	ID     string         `json:"id"`
	Type   string         `json:"type"`
	Check  string         `json:"check,omitempty"`
	Expect string         `json:"expect,omitempty"`
	Agent  string         `json:"agent,omitempty"`
	Model  string         `json:"model,omitempty"`
	Rubric string         `json:"rubric,omitempty"`
	Prompt string         `json:"prompt,omitempty"`
	Tool   string         `json:"tool,omitempty"`
	Inputs map[string]any `json:"inputs,omitempty"`
}

type LoopGraphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type LoopStartBinding struct {
	Kind         string            `json:"kind"`
	Inputs       map[string]any    `json:"inputs,omitempty"`
	InputMapping map[string]string `json:"input_mapping,omitempty"`
}

func transcodeLoopDTO(value any, target any) error {
	if target == nil {
		return fmt.Errorf("contract: loop DTO target is required")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("contract: encode loop DTO: %w", err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("contract: decode loop DTO: %w", err)
	}
	return nil
}
