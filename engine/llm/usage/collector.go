package usage

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
)

type contextKey struct{}

// Metadata captures execution identifiers used to persist LLM usage rows.
type Metadata struct {
	Component      core.ComponentType
	WorkflowExecID core.ID
	TaskExecID     core.ID
	AgentID        *string
}

// Snapshot represents a single LLM call usage payload emitted by a provider.
type Snapshot struct {
	Provider           string
	Model              string
	PromptTokens       int
	CompletionTokens   int
	TotalTokens        int
	ReasoningTokens    *int
	CachedPromptTokens *int
	InputAudioTokens   *int
	OutputAudioTokens  *int
}

// Collector aggregates usage snapshots for an execution and persists them via Repository.
type Collector struct {
	mu         sync.Mutex
	repo       Repository
	metrics    Metrics
	meta       Metadata
	provider   string
	model      string
	prompt     int
	completion int

	total         int
	totalProvided bool

	reasoning    int
	hasReasoning bool

	cachedPrompt    int
	hasCachedPrompt bool

	inputAudio    int
	hasInputAudio bool

	outputAudio    int
	hasOutputAudio bool
}

// NewCollector constructs a collector bound to the provided repository, metrics, and metadata.
func NewCollector(repo Repository, metrics Metrics, meta Metadata) *Collector {
	if repo == nil {
		return nil
	}
	return &Collector{
		repo:    repo,
		metrics: metrics,
		meta:    meta,
	}
}

// ContextWithCollector attaches the collector to the context so downstream orchestrator
// components can record usage snapshots without additional plumbing.
func ContextWithCollector(ctx context.Context, collector *Collector) context.Context {
	if collector == nil {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, collector)
}

// FromContext retrieves the collector stored in the context, if present.
func FromContext(ctx context.Context) *Collector {
	if ctx == nil {
		return nil
	}
	if collector, ok := ctx.Value(contextKey{}).(*Collector); ok {
		return collector
	}
	return nil
}

// Record adds a usage snapshot to the aggregate totals.
func (c *Collector) Record(_ context.Context, snapshot *Snapshot) {
	if c == nil || c.repo == nil || snapshot == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if snapshot.Provider != "" {
		c.provider = snapshot.Provider
	}
	if snapshot.Model != "" {
		c.model = snapshot.Model
	}
	c.prompt += snapshot.PromptTokens
	c.completion += snapshot.CompletionTokens

	if snapshot.TotalTokens > 0 {
		c.total += snapshot.TotalTokens
		c.totalProvided = true
	}

	if snapshot.ReasoningTokens != nil {
		c.reasoning += *snapshot.ReasoningTokens
		c.hasReasoning = true
	}
	if snapshot.CachedPromptTokens != nil {
		c.cachedPrompt += *snapshot.CachedPromptTokens
		c.hasCachedPrompt = true
	}
	if snapshot.InputAudioTokens != nil {
		c.inputAudio += *snapshot.InputAudioTokens
		c.hasInputAudio = true
	}
	if snapshot.OutputAudioTokens != nil {
		c.outputAudio += *snapshot.OutputAudioTokens
		c.hasOutputAudio = true
	}
}

// Finalize persists aggregated usage counts for the execution. It is safe to call Finalize
// multiple times; each call overwrites the previous totals using Repository.Upsert semantics.
func (c *Collector) Finalize(ctx context.Context, status core.StatusType) error {
	if c == nil || c.repo == nil {
		return nil
	}
	snapshot, ok := c.snapshotFinalizeState()
	if !ok {
		return nil
	}
	return persistFinalize(ctx, status, snapshot)
}

// finalizeSnapshot stores the data required to persist usage once the collector lock is released.
type finalizeSnapshot struct {
	repo     Repository
	metrics  Metrics
	meta     Metadata
	provider string
	model    string
	row      *Row
}

// snapshotFinalizeState captures the collector state needed to persist usage outside the lock.
func (c *Collector) snapshotFinalizeState() (*finalizeSnapshot, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.provider == "" || c.model == "" {
		return nil, false
	}
	meta := c.meta
	prompt := c.prompt
	completion := c.completion
	total := prompt + completion
	if c.totalProvided && c.total > total {
		total = c.total
	}
	var (
		workflowID *core.ID
		taskID     *core.ID
	)
	if !meta.TaskExecID.IsZero() {
		taskID = optionalID(meta.TaskExecID)
	} else {
		workflowID = optionalID(meta.WorkflowExecID)
	}
	row := &Row{
		WorkflowExecID:     workflowID,
		TaskExecID:         taskID,
		Component:          meta.Component,
		AgentID:            parseAgentID(meta.AgentID),
		Provider:           c.provider,
		Model:              c.model,
		PromptTokens:       prompt,
		CompletionTokens:   completion,
		TotalTokens:        total,
		ReasoningTokens:    optionalInt(c.hasReasoning, c.reasoning),
		CachedPromptTokens: optionalInt(c.hasCachedPrompt, c.cachedPrompt),
		InputAudioTokens:   optionalInt(c.hasInputAudio, c.inputAudio),
		OutputAudioTokens:  optionalInt(c.hasOutputAudio, c.outputAudio),
	}
	return &finalizeSnapshot{
		repo:     c.repo,
		metrics:  c.metrics,
		meta:     meta,
		provider: c.provider,
		model:    c.model,
		row:      row,
	}, true
}

// persistFinalize writes the usage row and records metrics using the captured snapshot.
func persistFinalize(
	ctx context.Context,
	status core.StatusType,
	snapshot *finalizeSnapshot,
) error {
	start := time.Now()
	err := snapshot.repo.Upsert(ctx, snapshot.row)
	duration := time.Since(start)
	if err != nil {
		log := logger.FromContext(ctx)
		workflowLogID := maybeString(optionalID(snapshot.meta.WorkflowExecID))
		log.Error(
			"Failed to persist LLM usage",
			"status", status,
			"component", snapshot.meta.Component,
			"task_exec_id", maybeString(snapshot.row.TaskExecID),
			"workflow_exec_id", workflowLogID,
			"error", err,
		)
		if snapshot.metrics != nil {
			snapshot.metrics.RecordFailure(ctx, snapshot.meta.Component, snapshot.provider, snapshot.model, duration)
		}
		return fmt.Errorf("finalize usage: %w", err)
	}
	if snapshot.metrics != nil {
		snapshot.metrics.RecordSuccess(
			ctx,
			snapshot.meta.Component,
			snapshot.provider,
			snapshot.model,
			snapshot.row.PromptTokens,
			snapshot.row.CompletionTokens,
			duration,
		)
	}
	return nil
}

func optionalID(id core.ID) *core.ID {
	if id.IsZero() {
		return nil
	}
	value := id
	return &value
}

func optionalInt(ok bool, value int) *int {
	if !ok {
		return nil
	}
	return &value
}

func parseAgentID(id *string) *core.ID {
	if id == nil || *id == "" {
		return nil
	}
	parsed, err := core.ParseID(*id)
	if err != nil {
		return nil
	}
	return &parsed
}

func maybeString(id *core.ID) string {
	if id == nil {
		return ""
	}
	return id.String()
}
