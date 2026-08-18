package loop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
)

const (
	RequestKindAsk         = "ask"
	RequestKindReview      = "review"
	RequestStatePending    = "pending"
	RequestStateAnswered   = "answered"
	RequestStateExpired    = "expired"
	RequestStateCanceled   = "canceled"
	RequestDecisionRespond = "respond"
	RequestDecisionApprove = "approve"
	RequestDecisionEdit    = "edit"
	RequestDecisionReject  = "reject"
)

const (
	ReasonCodeRequestNotFound         ReasonCode = "request_not_found"
	ReasonCodeRequestValidationFailed ReasonCode = "request_validation_failed"
	ReasonCodeRequestAlreadyAnswered  ReasonCode = "request_already_answered"
	ReasonCodeRequestExpired          ReasonCode = "request_expired"
	ReasonCodeRequestCanceled         ReasonCode = "request_canceled"
	ReasonCodeRespondNotPermitted     ReasonCode = "respond_not_permitted"
	ReasonCodeRespondSelfDenied       ReasonCode = "respond_self_denied"
)

const ReasonMetaRecordedDecision = "recorded_decision"

var (
	ErrRequestNotFound         = errors.New("loop: request not found")
	ErrRequestAlreadyAnswered  = errors.New("loop: request already answered")
	ErrRequestExpired          = errors.New("loop: request expired")
	ErrRequestCanceled         = errors.New("loop: request canceled")
	ErrRespondNotPermitted     = errors.New("loop: respond not permitted")
	ErrRespondSelfDenied       = errors.New("loop: respond self denied")
	ErrRequestValidationFailed = errors.New("loop: request validation failed")
)

// RequestIntent persists one request-shaped read model beside its authoritative wait.
type RequestIntent struct {
	WorkspaceID     WorkspaceID              `json:"workspace_id"`
	NodeID          NodeID                   `json:"node_id"`
	ItemIndex       int                      `json:"item_index,omitempty"`
	Kind            string                   `json:"kind"`
	Prompt          string                   `json:"prompt"`
	Context         json.RawMessage          `json:"context"`
	ContextPreview  json.RawMessage          `json:"context_preview"`
	AnswerSchema    json.RawMessage          `json:"answer_schema"`
	EditSchema      json.RawMessage          `json:"edit_schema,omitempty"`
	RespondSchema   json.RawMessage          `json:"respond_schema,omitempty"`
	Decisions       json.RawMessage          `json:"decisions"`
	Proposed        json.RawMessage          `json:"proposed,omitempty"`
	ProposedPreview json.RawMessage          `json:"proposed_preview,omitempty"`
	Agents          dsl.ResponderAgentPolicy `json:"agents"`
	ExpiresAt       *time.Time               `json:"expires_at,omitempty"`
	IssuedEpoch     int64                    `json:"issued_epoch"`
	OpenedAt        time.Time                `json:"opened_at"`
}

func (i RequestIntent) normalized() RequestIntent {
	i.WorkspaceID = WorkspaceID(strings.TrimSpace(string(i.WorkspaceID)))
	i.NodeID = NodeID(strings.TrimSpace(string(i.NodeID)))
	i.Kind = strings.TrimSpace(i.Kind)
	i.Prompt = strings.TrimSpace(i.Prompt)
	if i.Agents == "" {
		i.Agents = dsl.ResponderAgentsDeny
	}
	if i.ExpiresAt != nil {
		value := i.ExpiresAt.UTC()
		i.ExpiresAt = &value
	}
	i.OpenedAt = i.OpenedAt.UTC()
	i.Context = cloneRawMessage(i.Context)
	i.ContextPreview = cloneRawMessage(i.ContextPreview)
	i.AnswerSchema = cloneRawMessage(i.AnswerSchema)
	i.EditSchema = cloneRawMessage(i.EditSchema)
	i.RespondSchema = cloneRawMessage(i.RespondSchema)
	i.Decisions = cloneRawMessage(i.Decisions)
	i.Proposed = cloneRawMessage(i.Proposed)
	i.ProposedPreview = cloneRawMessage(i.ProposedPreview)
	return i
}

func (i RequestIntent) validate() error {
	if !i.hasValidIdentity() {
		return fmt.Errorf("%w: request intent identity is incomplete", ErrValidation)
	}
	if err := validateRequiredRequestJSON(i); err != nil {
		return err
	}
	if i.Kind == RequestKindAsk && (len(i.AnswerSchema) == 0 || !json.Valid(i.AnswerSchema)) {
		return fmt.Errorf("%w: ask answer_schema must be valid JSON", ErrValidation)
	}
	if i.Kind == RequestKindReview && (len(i.Proposed) == 0 || !json.Valid(i.Proposed) ||
		len(i.ProposedPreview) == 0 || !json.Valid(i.ProposedPreview)) {
		return fmt.Errorf("%w: review proposal must be valid JSON", ErrValidation)
	}
	if err := validateOptionalRequestJSON(i); err != nil {
		return err
	}
	if !i.Agents.Valid() {
		return fmt.Errorf("%w: request agents policy is invalid", ErrValidation)
	}
	return nil
}

func (i RequestIntent) hasValidIdentity() bool {
	return i.WorkspaceID != "" && i.NodeID != "" && i.ItemIndex >= 0 &&
		(i.Kind == RequestKindAsk || i.Kind == RequestKindReview) &&
		i.Prompt != "" && i.IssuedEpoch >= 1 && !i.OpenedAt.IsZero()
}

func validateRequiredRequestJSON(i RequestIntent) error {
	for name, raw := range map[string]json.RawMessage{
		"context": i.Context, "context_preview": i.ContextPreview, "decisions": i.Decisions,
	} {
		if len(raw) == 0 || !json.Valid(raw) {
			return fmt.Errorf("%w: request %s must be valid JSON", ErrValidation, name)
		}
	}
	return nil
}

func validateOptionalRequestJSON(i RequestIntent) error {
	for name, raw := range map[string]json.RawMessage{
		"edit_schema": i.EditSchema, "respond_schema": i.RespondSchema,
	} {
		if len(raw) > 0 && !json.Valid(raw) {
			return fmt.Errorf("%w: request %s must be valid JSON", ErrValidation, name)
		}
	}
	return nil
}

// RequestRef identifies one request cell.
type RequestRef struct {
	RunID      RunID
	Generation int
	NodeID     NodeID
	ItemIndex  int
}

// Request is the public redaction-safe request projection.
type Request struct {
	LoopRunID        RunID                    `json:"loop_run_id"`
	LoopName         string                   `json:"loop_name,omitempty"`
	Generation       int                      `json:"generation"`
	NodeID           NodeID                   `json:"node_id"`
	ItemIndex        int                      `json:"item_index"`
	Kind             string                   `json:"kind"`
	State            string                   `json:"state"`
	Prompt           string                   `json:"prompt"`
	Context          json.RawMessage          `json:"context"`
	Expect           json.RawMessage          `json:"expect"`
	EditSchema       json.RawMessage          `json:"edit_schema,omitempty"`
	RespondSchema    json.RawMessage          `json:"respond_schema,omitempty"`
	ProposedPreview  json.RawMessage          `json:"proposed_preview,omitempty"`
	Decisions        []string                 `json:"decisions"`
	Agents           dsl.ResponderAgentPolicy `json:"agents"`
	AnsweredDecision string                   `json:"answered_decision,omitempty"`
	ActorKind        string                   `json:"actor_kind,omitempty"`
	ActorID          string                   `json:"actor_id,omitempty"`
	OpenedAt         time.Time                `json:"opened_at"`
	ResolvedAt       *time.Time               `json:"resolved_at,omitempty"`
	ExpiresAt        *time.Time               `json:"expires_at,omitempty"`
}

type RequestQuery struct {
	RunID  RunID
	State  string
	Cursor string
	Limit  int
}

type RequestPage struct {
	Items      []Request `json:"items"`
	Pending    int       `json:"-"`
	NextCursor string    `json:"next_cursor"`
}

type RequestDetail = Request

type RespondInput struct {
	WorkspaceID WorkspaceID
	RunID       RunID
	NodeID      NodeID
	ItemIndex   int
	Decision    string
	Payload     json.RawMessage
	Note        string
	Actor       task.ActorContext
	RequestKind string
	RejectRoute NodeID
}

type RespondResult struct {
	Request     Request   `json:"request"`
	Coordinator *task.Run `json:"-"`
	Won         bool      `json:"won"`
}

// ResponderPolicy is the daemon-owned trust boundary for self-operation rules.
type ResponderPolicy interface {
	DeniesSelfOperation(
		ctx context.Context,
		workspaceID string,
		runID string,
		actor task.ActorContext,
	) (bool, error)
}

type RequestStore interface {
	ListRequests(context.Context, WorkspaceID, RequestQuery) (RequestPage, error)
	GetRequest(context.Context, WorkspaceID, RequestRef, bool) (Request, error)
	RespondRequest(context.Context, RespondInput) (RespondResult, error)
}
