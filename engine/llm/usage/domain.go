package usage

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/engine/core"
)

// ErrNotFound indicates that usage data does not exist for the requested execution.
var ErrNotFound = errors.New("usage not found")

// Row represents a persisted LLM usage record for a workflow, task, or agent execution.
// It captures model attribution and token counts for downstream reporting.
type Row struct {
	ID                 int64
	WorkflowExecID     *core.ID
	TaskExecID         *core.ID
	Component          core.ComponentType
	AgentID            *core.ID
	Provider           string
	Model              string
	PromptTokens       int
	CompletionTokens   int
	TotalTokens        int
	ReasoningTokens    *int
	CachedPromptTokens *int
	InputAudioTokens   *int
	OutputAudioTokens  *int
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// Repository exposes storage operations for usage rows so collectors can persist results.
// Implementations must enforce referential integrity against workflow and task executions.
type Repository interface {
	Upsert(ctx context.Context, row *Row) error
	GetByTaskExecID(ctx context.Context, id core.ID) (*Row, error)
	GetByWorkflowExecID(ctx context.Context, id core.ID) (*Row, error)
	// SummarizeByWorkflowExecID aggregates usage across all components for a workflow execution.
	SummarizeByWorkflowExecID(ctx context.Context, id core.ID) (*Row, error)
	// SummariesByWorkflowExecIDs preloads aggregated usage summaries for multiple workflow executions.
	// Implementations should return summaries keyed by workflow execution ID and skip missing rows gracefully.
	SummariesByWorkflowExecIDs(ctx context.Context, ids []core.ID) (map[core.ID]*Row, error)
}

// Metrics captures observability hooks for usage collection so callers can emit counters
// without depending on concrete monitoring implementations.
type Metrics interface {
	RecordSuccess(
		ctx context.Context,
		component core.ComponentType,
		provider string,
		model string,
		promptTokens int,
		completionTokens int,
		latency time.Duration,
	)
	RecordFailure(
		ctx context.Context,
		component core.ComponentType,
		provider string,
		model string,
		latency time.Duration,
	)
}
