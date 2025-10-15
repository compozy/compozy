package usage

import (
	"context"
	"fmt"
	"sync"

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

// NewCollector constructs a collector bound to the provided repository and metadata.
func NewCollector(repo Repository, meta Metadata) *Collector {
	if repo == nil {
		return nil
	}
	return &Collector{repo: repo, meta: meta}
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
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.provider == "" || c.model == "" {
		return nil
	}

	total := c.total
	if !c.totalProvided {
		total = c.prompt + c.completion
	}

	row := &Row{
		WorkflowExecID:     optionalID(c.meta.WorkflowExecID),
		TaskExecID:         optionalID(c.meta.TaskExecID),
		Component:          c.meta.Component,
		AgentID:            parseAgentID(c.meta.AgentID),
		Provider:           c.provider,
		Model:              c.model,
		PromptTokens:       c.prompt,
		CompletionTokens:   c.completion,
		TotalTokens:        total,
		ReasoningTokens:    optionalInt(c.hasReasoning, c.reasoning),
		CachedPromptTokens: optionalInt(c.hasCachedPrompt, c.cachedPrompt),
		InputAudioTokens:   optionalInt(c.hasInputAudio, c.inputAudio),
		OutputAudioTokens:  optionalInt(c.hasOutputAudio, c.outputAudio),
	}

	if err := c.repo.Upsert(ctx, row); err != nil {
		log := logger.FromContext(ctx)
		log.Error(
			"Failed to persist LLM usage",
			"status", status,
			"component", c.meta.Component,
			"task_exec_id", maybeString(row.TaskExecID),
			"workflow_exec_id", maybeString(row.WorkflowExecID),
			"error", err,
		)
		return fmt.Errorf("finalize usage: %w", err)
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
