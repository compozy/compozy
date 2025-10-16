package store

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm/usage"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/require"
)

func TestTaskRepoMergeUsage(t *testing.T) {
	env := newRepoTestEnv(t)
	state := &task.State{
		TaskID:         "task-1",
		TaskExecID:     core.MustNewID(),
		WorkflowID:     "wf-1",
		WorkflowExecID: core.MustNewID(),
		Status:         core.StatusRunning,
	}
	upsertWorkflowState(t, env, state.WorkflowID, state.WorkflowExecID, nil)
	require.NoError(t, env.taskRepo.UpsertState(env.ctx, state))

	firstSummary := &usage.Summary{Entries: []usage.Entry{{
		Provider:         "openai",
		Model:            "gpt-4o",
		PromptTokens:     10,
		CompletionTokens: 4,
		TotalTokens:      14,
	}}}
	require.NoError(t, env.taskRepo.MergeUsage(env.ctx, state.TaskExecID, firstSummary))

	secondSummary := &usage.Summary{Entries: []usage.Entry{{
		Provider:         "openai",
		Model:            "gpt-4o",
		PromptTokens:     6,
		CompletionTokens: 5,
		TotalTokens:      12,
	}}}
	require.NoError(t, env.taskRepo.MergeUsage(env.ctx, state.TaskExecID, secondSummary))

	merged, err := env.taskRepo.GetState(env.ctx, state.TaskExecID)
	require.NoError(t, err)
	require.NotNil(t, merged.Usage)
	require.Len(t, merged.Usage.Entries, 1)
	entry := merged.Usage.Entries[0]
	require.Equal(t, 16, entry.PromptTokens)
	require.Equal(t, 9, entry.CompletionTokens)
	require.Equal(t, 26, entry.TotalTokens)
}

func TestWorkflowRepoMergeUsage(t *testing.T) {
	env := newRepoTestEnv(t)
	execID := core.MustNewID()
	upsertWorkflowState(t, env, "wf-1", execID, nil)

	firstSummary := &usage.Summary{Entries: []usage.Entry{{
		Provider:         "anthropic",
		Model:            "claude-3",
		PromptTokens:     11,
		CompletionTokens: 3,
		TotalTokens:      15,
	}}}
	require.NoError(t, env.workflowRepo.MergeUsage(env.ctx, execID, firstSummary))

	secondSummary := &usage.Summary{Entries: []usage.Entry{{
		Provider:         "anthropic",
		Model:            "claude-3",
		PromptTokens:     5,
		CompletionTokens: 2,
		TotalTokens:      7,
	}}}
	require.NoError(t, env.workflowRepo.MergeUsage(env.ctx, execID, secondSummary))

	state, err := env.workflowRepo.GetState(env.ctx, execID)
	require.NoError(t, err)
	require.NotNil(t, state)
	require.NotNil(t, state.Usage)
	require.Len(t, state.Usage.Entries, 1)
	entry := state.Usage.Entries[0]
	require.Equal(t, 16, entry.PromptTokens)
	require.Equal(t, 5, entry.CompletionTokens)
	require.Equal(t, 22, entry.TotalTokens)
	logger.FromContext(env.ctx).Info("workflow usage merge result", "entry", entry)
}
