package usage

import (
	"context"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/core"
)

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

// Finalized bundles the aggregated usage summary for the execution.
type Finalized struct {
	Metadata Metadata
	Summary  *Summary
}

// Collector aggregates usage snapshots for an execution.
type Collector struct {
	mu      sync.Mutex
	metrics Metrics
	meta    Metadata
	summary *Summary
}

// NewCollector constructs a collector bound to the provided metrics sink and metadata.
func NewCollector(metrics Metrics, meta Metadata) *Collector {
	return &Collector{
		metrics: metrics,
		meta:    meta,
		summary: &Summary{},
	}
}

type contextKey struct{}

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
	if c == nil || snapshot == nil {
		return
	}
	if snapshot.Provider == "" || snapshot.Model == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	entry := Entry{
		Provider:           snapshot.Provider,
		Model:              snapshot.Model,
		PromptTokens:       snapshot.PromptTokens,
		CompletionTokens:   snapshot.CompletionTokens,
		TotalTokens:        snapshot.TotalTokens,
		ReasoningTokens:    snapshot.ReasoningTokens,
		CachedPromptTokens: snapshot.CachedPromptTokens,
		InputAudioTokens:   snapshot.InputAudioTokens,
		OutputAudioTokens:  snapshot.OutputAudioTokens,
		Source:             string(SourceTask),
	}
	if c.meta.AgentID != nil && *c.meta.AgentID != "" {
		entry.AgentIDs = []string{*c.meta.AgentID}
	}
	now := time.Now().UTC()
	entry.CapturedAt = &now
	entry.UpdatedAt = &now
	c.summary.MergeEntry(&entry)
}

// Finalize returns the aggregated usage summary for downstream persistence.
func (c *Collector) Finalize(ctx context.Context, status core.StatusType) (*Finalized, error) {
	if c == nil {
		return nil, nil
	}
	summary := c.snapshotSummary()
	if summary == nil || len(summary.Entries) == 0 {
		return nil, nil
	}
	c.emitMetrics(ctx, status, summary)
	return &Finalized{
		Metadata: c.meta,
		Summary:  summary,
	}, nil
}

func (c *Collector) snapshotSummary() *Summary {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.summary == nil || len(c.summary.Entries) == 0 {
		return nil
	}
	clone := c.summary.Clone()
	c.summary = &Summary{}
	if clone != nil {
		clone.Sort()
	}
	return clone
}

func (c *Collector) emitMetrics(ctx context.Context, status core.StatusType, summary *Summary) {
	if c.metrics == nil || summary == nil {
		return
	}
	for i := range summary.Entries {
		entry := &summary.Entries[i]
		c.metrics.RecordSuccess(
			ctx,
			c.meta.Component,
			entry.Provider,
			entry.Model,
			entry.PromptTokens,
			entry.CompletionTokens,
			0,
		)
		if status != core.StatusSuccess {
			c.metrics.RecordFailure(ctx, c.meta.Component, entry.Provider, entry.Model, 0)
		}
	}
}
