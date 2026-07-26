package loop

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/tools"
)

const (
	defaultActionSessionHandle = "main"
	actionCancelMaxWait        = 5 * time.Second
	actionCancelWaitGrace      = 100 * time.Millisecond
	harvestKindSync            = "sync"
	harvestKindEventRange      = "event_range"
	harvestKindAsync           = "async"
	harvestKindChannelResult   = "channel_result"
	actionDependencyMetaKey    = "dependency"
	actionKindMetaKey          = "kind"
	outputSchemaParamKey       = "output_schema"
	jsonSchemaTypeKey          = "type"
	jsonSchemaObjectType       = "object"
	jsonSchemaPropertiesKey    = "properties"
	jsonSchemaRequiredKey      = "required"
	jsonSchemaStringType       = "string"
	jsonSchemaTitleKey         = "title"
)

var (
	// ErrActionUnknownKind reports a non-reserved action kind missing from RuntimeRegistry.
	ErrActionUnknownKind = errors.New("loop: unknown action kind")
	// ErrActionDependencyMissing reports an unwired action runtime collaborator.
	ErrActionDependencyMissing = errors.New("loop: action dependency missing")
	// ErrActionStalled reports a harvest that must wait for an external result.
	ErrActionStalled = errors.New("loop: action stalled")
	// ErrActionSchemaInvalid reports run-agent structured output that failed output_schema validation.
	ErrActionSchemaInvalid = errors.New("loop: action schema validation failed")
	// ErrActionTimeout reports an action turn canceled by the node timeout.
	ErrActionTimeout = errors.New("loop: action timeout")
)

const (
	// ReasonCodeUnknownActionKind reports an action kind missing from reserved kinds and RuntimeRegistry.
	ReasonCodeUnknownActionKind ReasonCode = "unknown_action_kind"
	// ReasonCodeActionDependencyMissing reports an unwired runtime collaborator for a reserved action.
	ReasonCodeActionDependencyMissing ReasonCode = "action_dependency_missing"
	// ReasonCodeActionStalled reports a harvest window that produced no designated result.
	ReasonCodeActionStalled ReasonCode = "action_stalled"
	// ReasonCodeActionSchemaInvalid reports run-agent output_schema validation failure.
	ReasonCodeActionSchemaInvalid ReasonCode = "action_schema_invalid"
	// ReasonCodeActionTimeout reports timeout cancellation of an action turn.
	ReasonCodeActionTimeout ReasonCode = "action_timeout"
	// ReasonCodeActionContractStale reports runtime tool schemas that differ from the Run snapshot.
	ReasonCodeActionContractStale ReasonCode = "action_contract_stale"
)

// ActionExecutor runs one loop action node and converts its raw result into node output.
type ActionExecutor interface {
	Execute(ctx context.Context, node dsl.Node, in ActionExecutionInput) (ActionRawResult, error)
	Harvest(ctx context.Context, raw ActionRawResult, node dsl.Node) (ActionOutput, error)
}

// ActionExecutionInput is the loop-owned execution context for one node instance.
type ActionExecutionInput struct {
	WorkspaceID              WorkspaceID
	LoopRunID                RunID
	Generation               int
	NodeID                   dsl.NodeID
	ItemIndex                int
	Namespace                map[string]any
	Contract                 *dsl.Contract
	ToolScope                tools.Scope
	Actor                    task.ActorContext
	CorrelationID            string
	WorkerModel              string
	JudgeModel               string
	CWD                      string
	AllowedTools             []string
	OriginSessionID          string
	OriginCreationProfileRef string
	OriginPolicySpecDigest   string
	OriginCreationDigest     string
	GoalContextNudgeRatio    *float64
	UsageReporter            ActionUsageReporter
	PersistedTaskTokensUsed  int64
	GoalSegmentEpoch         int64
	NetworkParticipation     *participation.Spec
}

// ActionRawResult captures backend-specific action output before harvest policy.
type ActionRawResult struct {
	ToolResult      tools.ToolResult
	Structured      json.RawMessage
	Value           any
	Text            string
	WorkspaceID     string
	SessionID       string
	EventStartSeq   int64
	EventEndSeq     int64
	TokensUsed      int64
	ChildLoopRunID  RunID
	Status          string
	RenderedParams  json.RawMessage
	RenderedHarvest *dsl.HarvestSpec
	Control         *ActionControl
}

// ActionOutput is the typed loop-node output written into generation snapshots.
type ActionOutput struct {
	Structured     json.RawMessage
	Value          any
	Text           string
	SessionID      string
	EventStartSeq  int64
	EventEndSeq    int64
	TokensUsed     int64
	ChildLoopRunID RunID
	Status         string
}

// ActionEvent is a stable projection of one ACP/network event harvested by sequence.
type ActionEvent struct {
	Sequence int64           `json:"sequence"`
	Type     string          `json:"type,omitempty"`
	Text     string          `json:"text,omitempty"`
	Content  json.RawMessage `json:"content,omitempty"`
}

// ActionEventRangeRequest asks an injected event reader to harvest one append-only window.
type ActionEventRangeRequest struct {
	SessionID     string
	EventStartSeq int64
	EventEndSeq   int64
	Raw           ActionRawResult
	Node          dsl.Node
}

// ActionEventRangeResult is the structured result of an async event-range harvest.
type ActionEventRangeResult struct {
	Events     []ActionEvent
	Structured json.RawMessage
}

// ActionEventRangeReader reads ACP/network events by stable sequence.
type ActionEventRangeReader interface {
	ReadActionEventRange(ctx context.Context, req ActionEventRangeRequest) (ActionEventRangeResult, error)
}

// ChannelResultHarvestRequest asks for the designated result message after a network send.
type ChannelResultHarvestRequest struct {
	Window      string
	Responder   string
	ContentRule string
	Raw         ActionRawResult
	Node        dsl.Node
}

// ChannelResultHarvestResult is the designated result coordination message.
type ChannelResultHarvestResult struct {
	Found      bool
	MessageID  string
	Structured json.RawMessage
	Text       string
}

// ChannelResultHarvester harvests the ADR-021 channel_result semantics.
type ChannelResultHarvester interface {
	HarvestChannelResult(ctx context.Context, req *ChannelResultHarvestRequest) (ChannelResultHarvestResult, error)
}

// ActionLoopStarter is the run-loop seam implemented by loop.Service.
type ActionLoopStarter interface {
	Start(ctx context.Context, ws WorkspaceID, name string, inputs Inputs, actor task.ActorContext) (*Run, error)
}

// ActionSessionBinder binds and prompts ACP sessions for run-agent.
type ActionSessionBinder interface {
	BindActionSession(ctx context.Context, req ActionSessionBindRequest) (ActionSessionBinding, error)
	PromptActionSession(
		ctx context.Context,
		binding ActionSessionBinding,
		req ActionPromptRequest,
	) (ActionPromptResult, error)
	CancelActionSession(ctx context.Context, binding ActionSessionBinding) error
}

// ActionSessionBindRequest describes shared-default or isolated run-agent binding.
type ActionSessionBindRequest struct {
	WorkspaceID                    WorkspaceID
	LoopRunID                      RunID
	Generation                     int
	NodeID                         dsl.NodeID
	ItemIndex                      int
	Agent                          string
	CWD                            string
	Handle                         string
	Mode                           string
	OriginSessionID                string
	TargetBindingEpoch             int64
	ExpectedControlEpoch           int64
	ExpectedCheckpointPhase        string
	ExpectedTaskRunID              string
	ExpectedQueueEntryID           string
	ExpectedPromptID               string
	ExpectedCheckpointBindingEpoch int64
	ExpectedCheckpointSessionID    string
	ExpectedCheckpointHandle       string
	ReseedGrantID                  int64
	BindingAttemptID               string
	DesiredSessionID               string
	PinnedCreationProfileRef       string
	PinnedCreationDigest           string
	StaticPolicySpecDigest         string
	Isolated                       bool
	Model                          string
	AllowedTools                   []string
	MaxTurns                       int
	ContractBlock                  string
	NetworkParticipation           *participation.Spec
}

// ActionSessionCreationError carries provider-effect certainty without importing session internals.
type ActionSessionCreationError struct {
	EffectKnownFalse bool
	Code             string
	Err              error
}

// Error implements error.
func (e *ActionSessionCreationError) Error() string {
	if e == nil || e.Err == nil {
		return "action session creation failed"
	}
	return e.Err.Error()
}

// Unwrap preserves the provider/session creation cause.
func (e *ActionSessionCreationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// ActionSessionRetryRequest asks the binder to persist one Goal-owned retry decision.
type ActionSessionRetryRequest struct {
	BindRequest           ActionSessionBindRequest
	FailedBinding         ActionSessionBinding
	FailureCode           string
	ExpectedPromptAttempt int
	RetryWithFreshSession bool
}

// ActionSessionBinding is the durable session binding returned by the runtime.
type ActionSessionBinding struct {
	WorkspaceID        WorkspaceID
	LoopRunID          RunID
	SessionID          string
	Handle             string
	SharedKey          string
	ControlEpoch       int64
	BindingEpoch       int64
	BindingAttemptID   string
	CreationProfileRef string
	PolicySpecDigest   string
	CreationDigest     string
	State              string
	Ownership          string
	Isolated           bool
}

// ActionPromptRequest is one work-order turn inside a bound run-agent session.
type ActionPromptRequest struct {
	PromptID             string
	Message              string
	Kind                 string
	Owner                ActionPromptOwner
	UsageBaseTokens      int64
	UsageBaseReported    bool
	ContextUsageSequence *int64
	ContextUsageUsed     *int64
	UsageReporter        ActionUsageReporter
}

// ActionPromptResult captures one ACP prompt turn.
type ActionPromptResult struct {
	PromptID         string
	Outcome          ActionPromptOutcome
	Text             string
	Structured       json.RawMessage
	EventStartSeq    int64
	EventEndSeq      int64
	TokensUsed       int64
	TokensReported   bool
	StopReason       ActionStopReason
	FenceDisposition ActionDisposition
	ReasonCode       ReasonCode
}
