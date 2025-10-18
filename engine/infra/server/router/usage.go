package router

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/llm/usage"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/apitypes"
	"github.com/compozy/compozy/pkg/logger"
)

// UsageSummary serializes execution-level LLM usage metrics for API consumers.
type UsageSummary = apitypes.UsageSummary

// UsageEntry mirrors the API transport entry shape.
type UsageEntry = apitypes.UsageEntry

// NewUsageSummary constructs a UsageSummary from an aggregated usage summary.
func NewUsageSummary(summary *usage.Summary) *UsageSummary {
	if summary == nil || len(summary.Entries) == 0 {
		return nil
	}
	clone := summary.Clone()
	if clone == nil || len(clone.Entries) == 0 {
		return nil
	}
	clone.Sort()
	entries := make([]UsageEntry, len(clone.Entries))
	for i := range clone.Entries {
		entries[i] = convertUsageEntry(&clone.Entries[i])
	}
	result := UsageSummary{Entries: entries}
	return &result
}

// ResolveTaskUsageSummary loads usage data for a task execution and maps it to the transport shape.
func ResolveTaskUsageSummary(ctx context.Context, repo task.Repository, taskExecID core.ID) *UsageSummary {
	if repo == nil || taskExecID.IsZero() {
		return nil
	}
	summary, err := repo.GetUsageSummary(ctx, taskExecID)
	if err != nil {
		if errors.Is(err, store.ErrTaskNotFound) {
			return nil
		}
		logger.FromContext(ctx).Warn(
			"Failed to load task usage summary",
			"task_exec_id", taskExecID.String(),
			"error", err,
		)
		return nil
	}
	return NewUsageSummary(summary)
}

// ResolveWorkflowUsageSummary loads aggregated usage for a workflow execution.
func ResolveWorkflowUsageSummary(ctx context.Context, repo workflow.Repository, workflowExecID core.ID) *UsageSummary {
	if repo == nil || workflowExecID.IsZero() {
		return nil
	}
	state, err := repo.GetState(ctx, workflowExecID)
	if err != nil {
		if errors.Is(err, store.ErrWorkflowNotFound) {
			return nil
		}
		logger.FromContext(ctx).Warn(
			"Failed to load workflow usage summary",
			"workflow_exec_id", workflowExecID.String(),
			"error", err,
		)
		return nil
	}
	return NewUsageSummary(state.Usage)
}

func convertUsageEntry(entry *usage.Entry) UsageEntry {
	if entry == nil {
		return UsageEntry{}
	}
	var capturedAt *time.Time
	if entry.CapturedAt != nil {
		ts := entry.CapturedAt.UTC()
		capturedAt = &ts
	}
	var updatedAt *time.Time
	if entry.UpdatedAt != nil {
		ts := entry.UpdatedAt.UTC()
		updatedAt = &ts
	}
	ids := cloneStrings(entry.AgentIDs)
	return UsageEntry{
		Provider:           entry.Provider,
		Model:              entry.Model,
		PromptTokens:       entry.PromptTokens,
		CompletionTokens:   entry.CompletionTokens,
		TotalTokens:        entry.TotalTokens,
		ReasoningTokens:    cloneInt(entry.ReasoningTokens),
		CachedPromptTokens: cloneInt(entry.CachedPromptTokens),
		InputAudioTokens:   cloneInt(entry.InputAudioTokens),
		OutputAudioTokens:  cloneInt(entry.OutputAudioTokens),
		AgentIDs:           ids,
		CapturedAt:         capturedAt,
		UpdatedAt:          updatedAt,
		Source:             entry.Source,
	}
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	clone := make([]string, len(values))
	copy(clone, values)
	return clone
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}
