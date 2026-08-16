package loop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/task"
)

var (
	ErrAmendNotParked     = errors.New("loop: amendment target is not parked")
	ErrAmendNoOutput      = errors.New("loop: amendment target has no output")
	ErrAmendSchemaMissing = errors.New("loop: amendment target has no declared output schema")
)

const (
	ReasonCodeAmendNotParked     ReasonCode = "amend_not_parked"
	ReasonCodeAmendNoOutput      ReasonCode = "amend_no_output"
	ReasonCodeAmendSchemaMissing ReasonCode = "amend_schema_missing"
)

type AmendInput struct {
	WorkspaceID WorkspaceID
	RunID       RunID
	Generation  int
	NodeID      NodeID
	ItemIndex   int
	Payload     json.RawMessage
	Schema      json.RawMessage
	Reason      string
	Actor       task.ActorContext
	RequestedAt time.Time
}

func (in AmendInput) Validate() error {
	if strings.TrimSpace(string(in.WorkspaceID)) == "" || strings.TrimSpace(string(in.RunID)) == "" ||
		in.Generation < 1 || strings.TrimSpace(string(in.NodeID)) == "" || in.ItemIndex < 0 ||
		!json.Valid(in.Payload) || in.RequestedAt.IsZero() {
		return fmt.Errorf("%w: amendment identity or payload is invalid", ErrValidation)
	}
	if len(in.Schema) == 0 {
		return NewRequestReasonError(ReasonCodeAmendSchemaMissing, ErrAmendSchemaMissing, nil)
	}
	if err := in.Actor.Validate(); err != nil {
		return fmt.Errorf("%w: amendment actor: %w", ErrValidation, err)
	}
	return nil
}

type NodeAmendment struct {
	WorkspaceID WorkspaceID
	LoopRunID   RunID
	Generation  int
	NodeID      NodeID
	ItemIndex   int
	Sequence    int
	OriginalRef string
	AmendedRef  string
	Original    json.RawMessage
	Amended     json.RawMessage
	ActorKind   string
	ActorID     string
	Reason      string
	CreatedAt   time.Time
}

type AmendmentStore interface {
	AmendNodeOutput(context.Context, AmendInput) (NodeAmendment, error)
	ListNodeAmendments(context.Context, WorkspaceID, RunID) ([]NodeAmendment, error)
}

type GenerationOutputOverlayReader interface {
	ApplyGenerationOutputOverlays(
		context.Context,
		WorkspaceID,
		RunID,
		int,
		[]GenerationOutput,
	) ([]GenerationOutput, error)
}
