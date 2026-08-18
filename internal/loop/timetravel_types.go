package loop

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
)

var (
	ErrRerunBusy             = errors.New("loop: rerun generation is busy")
	ErrRerunNodeUnsettled    = errors.New("loop: rerun node is unsettled")
	ErrTimeTravelKeyReuse    = errors.New("loop: time travel request id reused")
	ErrForkGenerationUnknown = errors.New("loop: fork generation is unknown")
	ErrDiffCrossLoop         = errors.New("loop: diff requires the same loop")
	ErrTimeTravelSelfDenied  = errors.New("loop: time travel self-operation denied")
)

const (
	ReasonCodeRerunBusy             ReasonCode = "rerun_busy"
	ReasonCodeRerunNodeUnsettled    ReasonCode = "rerun_node_unsettled"
	ReasonCodeTimeTravelKeyReuse    ReasonCode = "timetravel_key_reuse"
	ReasonCodeForkGenerationUnknown ReasonCode = "fork_generation_unknown"
	ReasonCodeDiffCrossLoop         ReasonCode = "diff_cross_loop"
	ReasonCodeTimeTravelSelfDenied  ReasonCode = "timetravel_self_denied"
)

type ForkRef struct {
	RunID      RunID `json:"run_id"`
	Generation int64 `json:"generation"`
}

type DiffQuery struct {
	RunID             RunID
	Generation        int64
	AgainstGeneration int64
	AgainstRunID      RunID
}

type DiffEndpoint struct {
	RunID      RunID  `json:"run_id"`
	Generation int64  `json:"generation"`
	Status     Status `json:"status"`
	AsOf       bool   `json:"as_of,omitempty"`
}

type DiffValue struct {
	Inline json.RawMessage `json:"inline,omitempty"`
	Size   int             `json:"size,omitempty"`
	Hash   string          `json:"hash,omitempty"`
}

type DiffInputRow struct {
	Key     string    `json:"key"`
	Base    DiffValue `json:"base"`
	Against DiffValue `json:"against"`
}

type DiffNodeRow struct {
	NodeID    string    `json:"node_id"`
	ItemIndex int       `json:"item_index,omitempty"`
	Change    string    `json:"change"`
	Base      DiffValue `json:"base,omitzero"`
	Against   DiffValue `json:"against,omitzero"`
	Cause     string    `json:"cause,omitempty"`
}

type DiffTerminalRow struct {
	Base    Status `json:"base"`
	Against Status `json:"against"`
}

type DiffResult struct {
	Kind                 string           `json:"kind"`
	Base                 DiffEndpoint     `json:"base"`
	Against              DiffEndpoint     `json:"against"`
	Inputs               []DiffInputRow   `json:"inputs"`
	Nodes                []DiffNodeRow    `json:"nodes"`
	Terminal             *DiffTerminalRow `json:"terminal,omitempty"`
	DefinitionDivergence bool             `json:"definition_divergence,omitempty"`
}

type RerunInput struct {
	WorkspaceID WorkspaceID
	RunID       RunID
	FromNode    NodeID
	ItemIndex   *int
	Reason      string
	RequestID   string
	Actor       task.ActorContext
}

type RerunResult struct {
	RunID            RunID    `json:"run_id"`
	Generation       int64    `json:"generation"`
	ParentGeneration int64    `json:"parent_generation"`
	RerunNodes       []string `json:"rerun_nodes"`
	Carried          int      `json:"carried"`
	Replayed         bool     `json:"replayed,omitempty"`
}

type ForkInput struct {
	WorkspaceID WorkspaceID
	RunID       RunID
	Generation  int64
	Inputs      map[string]any
	Reason      string
	RequestID   string
	Actor       task.ActorContext
}

type StartResult struct {
	Run      Run  `json:"run"`
	Replayed bool `json:"replayed,omitempty"`
}

type TimeTravelOp struct {
	ID               string
	Kind             string
	IdempotencyKey   string
	RequestDigest    string
	SourceRunID      RunID
	SourceGeneration *int64
	FromNode         NodeID
	ItemIndex        *int
	Actor            task.ActorContext
	Reason           string
	ResultRunID      RunID
	ResultGeneration *int64
	CreatedAt        time.Time
}

type RerunStoreRequest struct {
	WorkspaceID    WorkspaceID
	Source         *Run
	NextOutputs    []GenerationOutput
	Intent         GenerationIntent
	Operation      TimeTravelOp
	RequestDigest  string
	IdempotencyKey string
	At             time.Time
}

type ForkStoreRequest struct {
	Source         *Run
	Child          *Run
	SeedOutputs    []GenerationOutput
	Concurrency    dsl.ConcurrencyPolicy
	Operation      TimeTravelOp
	RequestDigest  string
	IdempotencyKey string
	At             time.Time
}

type TimeTravelStore interface {
	LookupRerunReplay(context.Context, WorkspaceID, string, string) (RerunResult, bool, error)
	CreateRerun(context.Context, RerunStoreRequest) (RerunResult, bool, error)
	CreateFork(context.Context, ForkStoreRequest) (Run, bool, error)
	ListForks(context.Context, WorkspaceID, RunID) ([]ForkRef, error)
}
