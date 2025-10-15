package router

import (
	"context"
	"errors"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm/usage"
	"github.com/compozy/compozy/pkg/logger"
)

// UsageSummary serializes execution-level LLM usage metrics for API consumers.
type UsageSummary struct {
	Provider           string `json:"provider"`
	Model              string `json:"model"`
	PromptTokens       int    `json:"prompt_tokens"`
	CompletionTokens   int    `json:"completion_tokens"`
	TotalTokens        int    `json:"total_tokens"`
	ReasoningTokens    *int   `json:"reasoning_tokens,omitempty"`
	CachedPromptTokens *int   `json:"cached_prompt_tokens,omitempty"`
	InputAudioTokens   *int   `json:"input_audio_tokens,omitempty"`
	OutputAudioTokens  *int   `json:"output_audio_tokens,omitempty"`
}

// NewUsageSummary constructs a UsageSummary from a persisted usage row.
func NewUsageSummary(row *usage.Row) *UsageSummary {
	if row == nil {
		return nil
	}
	return &UsageSummary{
		Provider:           row.Provider,
		Model:              row.Model,
		PromptTokens:       row.PromptTokens,
		CompletionTokens:   row.CompletionTokens,
		TotalTokens:        row.TotalTokens,
		ReasoningTokens:    row.ReasoningTokens,
		CachedPromptTokens: row.CachedPromptTokens,
		InputAudioTokens:   row.InputAudioTokens,
		OutputAudioTokens:  row.OutputAudioTokens,
	}
}

// ResolveTaskUsageSummary loads usage data for a task execution and maps it to the transport shape.
func ResolveTaskUsageSummary(ctx context.Context, repo usage.Repository, taskExecID core.ID) *UsageSummary {
	if repo == nil || taskExecID.IsZero() {
		return nil
	}
	row, err := repo.GetByTaskExecID(ctx, taskExecID)
	if err != nil {
		if !errors.Is(err, usage.ErrNotFound) {
			logger.FromContext(ctx).Warn(
				"Failed to load task usage summary",
				"task_exec_id", taskExecID.String(),
				"error", err,
			)
		}
		return nil
	}
	return NewUsageSummary(row)
}

// ResolveWorkflowUsageSummary loads aggregated usage for a workflow execution and maps it to UsageSummary.
func ResolveWorkflowUsageSummary(ctx context.Context, repo usage.Repository, workflowExecID core.ID) *UsageSummary {
	if repo == nil || workflowExecID.IsZero() {
		return nil
	}
	row, err := repo.GetByWorkflowExecID(ctx, workflowExecID)
	if err != nil {
		if errors.Is(err, usage.ErrNotFound) {
			row, err = repo.SummarizeByWorkflowExecID(ctx, workflowExecID)
		}
		if err != nil {
			if !errors.Is(err, usage.ErrNotFound) {
				logger.FromContext(ctx).Warn(
					"Failed to load workflow usage summary",
					"workflow_exec_id", workflowExecID.String(),
					"error", err,
				)
			}
			return nil
		}
	}
	return NewUsageSummary(row)
}
