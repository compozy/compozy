package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm/usage"
	"github.com/compozy/compozy/engine/task"
)

func TestUsageRepoIntegration(t *testing.T) {
	t.Run("Should upsert and fetch task usage", func(t *testing.T) {
		env := newRepoTestEnv(t)
		truncateRepoTables(env.ctx, t, env.pool)

		workflowID := "wf-usage-task"
		workflowExecID := core.MustNewID()
		upsertWorkflowState(t, env, workflowID, workflowExecID, nil)

		taskExecID := core.MustNewID()
		taskState := &task.State{
			Component:      core.ComponentTask,
			Status:         core.StatusPending,
			TaskID:         "task-usage",
			TaskExecID:     taskExecID,
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			ExecutionType:  task.ExecutionBasic,
		}
		require.NoError(t, env.taskRepo.UpsertState(env.ctx, taskState))

		reasoning := 2
		cachedPrompt := 3
		row := &usage.Row{
			WorkflowExecID:     &workflowExecID,
			TaskExecID:         &taskExecID,
			Component:          core.ComponentTask,
			Provider:           "openai",
			Model:              "gpt-4o",
			PromptTokens:       10,
			CompletionTokens:   5,
			TotalTokens:        15,
			ReasoningTokens:    &reasoning,
			CachedPromptTokens: &cachedPrompt,
		}

		require.NoError(t, env.usageRepo.Upsert(env.ctx, row))

		stored, err := env.usageRepo.GetByTaskExecID(env.ctx, taskExecID)
		require.NoError(t, err)
		assert.Equal(t, "openai", stored.Provider)
		assert.Equal(t, 15, stored.TotalTokens)
		require.NotNil(t, stored.ReasoningTokens)
		assert.Equal(t, 2, *stored.ReasoningTokens)

		row.CompletionTokens = 7
		row.TotalTokens = 17
		require.NoError(t, env.usageRepo.Upsert(env.ctx, row))

		updated, err := env.usageRepo.GetByTaskExecID(env.ctx, taskExecID)
		require.NoError(t, err)
		assert.Equal(t, 7, updated.CompletionTokens)
		assert.Equal(t, 17, updated.TotalTokens)
	})

	t.Run("Should summarize workflow usage from task rows", func(t *testing.T) {
		env := newRepoTestEnv(t)
		truncateRepoTables(env.ctx, t, env.pool)

		workflowID := "wf-usage-summary"
		workflowExecID := core.MustNewID()
		upsertWorkflowState(t, env, workflowID, workflowExecID, nil)

		firstTask := core.MustNewID()
		firstState := &task.State{
			Component:      core.ComponentTask,
			Status:         core.StatusSuccess,
			TaskID:         "task-a",
			TaskExecID:     firstTask,
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			ExecutionType:  task.ExecutionBasic,
		}
		require.NoError(t, env.taskRepo.UpsertState(env.ctx, firstState))

		secondTask := core.MustNewID()
		secondState := &task.State{
			Component:      core.ComponentAgent,
			Status:         core.StatusSuccess,
			TaskID:         "task-b",
			TaskExecID:     secondTask,
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			ExecutionType:  task.ExecutionBasic,
		}
		require.NoError(t, env.taskRepo.UpsertState(env.ctx, secondState))

		reasoning := 4
		require.NoError(t, env.usageRepo.Upsert(env.ctx, &usage.Row{
			WorkflowExecID:   &workflowExecID,
			TaskExecID:       &firstTask,
			Component:        core.ComponentTask,
			Provider:         "openai",
			Model:            "gpt-4o",
			PromptTokens:     12,
			CompletionTokens: 6,
			TotalTokens:      18,
			ReasoningTokens:  &reasoning,
		}))

		cachedPrompt := 8
		inputAudio := 2
		require.NoError(t, env.usageRepo.Upsert(env.ctx, &usage.Row{
			WorkflowExecID:     &workflowExecID,
			TaskExecID:         &secondTask,
			Component:          core.ComponentAgent,
			Provider:           "anthropic",
			Model:              "claude-3",
			PromptTokens:       20,
			CompletionTokens:   9,
			TotalTokens:        29,
			CachedPromptTokens: &cachedPrompt,
			InputAudioTokens:   &inputAudio,
		}))

		summary, err := env.usageRepo.SummarizeByWorkflowExecID(env.ctx, workflowExecID)
		require.NoError(t, err)
		require.NotNil(t, summary)
		assert.Equal(t, core.ComponentWorkflow, summary.Component)
		require.NotNil(t, summary.WorkflowExecID)
		assert.Equal(t, workflowExecID.String(), summary.WorkflowExecID.String())
		assert.Equal(t, 32, summary.PromptTokens)
		assert.Equal(t, 15, summary.CompletionTokens)
		assert.Equal(t, 47, summary.TotalTokens)
		require.NotNil(t, summary.ReasoningTokens)
		assert.Equal(t, 4, *summary.ReasoningTokens)
		require.NotNil(t, summary.CachedPromptTokens)
		assert.Equal(t, 8, *summary.CachedPromptTokens)
		require.NotNil(t, summary.InputAudioTokens)
		assert.Equal(t, 2, *summary.InputAudioTokens)
		assert.Equal(t, "mixed", summary.Provider)
		assert.Equal(t, "mixed", summary.Model)
	})

	t.Run("Should upsert workflow usage without task reference", func(t *testing.T) {
		env := newRepoTestEnv(t)
		truncateRepoTables(env.ctx, t, env.pool)

		workflowID := "wf-usage-workflow"
		workflowExecID := core.MustNewID()
		upsertWorkflowState(t, env, workflowID, workflowExecID, nil)

		row := &usage.Row{
			WorkflowExecID:   &workflowExecID,
			Component:        core.ComponentWorkflow,
			Provider:         "anthropic",
			Model:            "claude-3",
			PromptTokens:     20,
			CompletionTokens: 10,
			TotalTokens:      30,
		}

		require.NoError(t, env.usageRepo.Upsert(env.ctx, row))

		stored, err := env.usageRepo.GetByWorkflowExecID(env.ctx, workflowExecID)
		require.NoError(t, err)
		assert.Equal(t, "anthropic", stored.Provider)
		assert.Equal(t, 30, stored.TotalTokens)
		assert.Nil(t, stored.TaskExecID)

		row.PromptTokens = 25
		row.TotalTokens = 35
		require.NoError(t, env.usageRepo.Upsert(env.ctx, row))

		updated, err := env.usageRepo.GetByWorkflowExecID(env.ctx, workflowExecID)
		require.NoError(t, err)
		assert.Equal(t, 25, updated.PromptTokens)
		assert.Equal(t, 35, updated.TotalTokens)
	})

	t.Run("Should return not found when usage absent", func(t *testing.T) {
		env := newRepoTestEnv(t)
		truncateRepoTables(env.ctx, t, env.pool)

		missingTask := core.MustNewID()
		_, err := env.usageRepo.GetByTaskExecID(env.ctx, missingTask)
		require.ErrorIs(t, err, usage.ErrNotFound)

		missingWorkflow := core.MustNewID()
		_, err = env.usageRepo.GetByWorkflowExecID(env.ctx, missingWorkflow)
		require.ErrorIs(t, err, usage.ErrNotFound)
	})

	t.Run("Should enforce foreign key integrity", func(t *testing.T) {
		env := newRepoTestEnv(t)
		truncateRepoTables(env.ctx, t, env.pool)

		workflowExecID := core.MustNewID()
		taskExecID := core.MustNewID()
		row := &usage.Row{
			WorkflowExecID: &workflowExecID,
			TaskExecID:     &taskExecID,
			Component:      core.ComponentTask,
			Provider:       "openai",
			Model:          "gpt-4o-mini",
			PromptTokens:   5,
		}

		err := env.usageRepo.Upsert(env.ctx, row)
		require.Error(t, err)
		assert.ErrorContains(t, err, "foreign key")
	})
}
