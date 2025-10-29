package database_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/repo"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/llm/usage"
	"github.com/compozy/compozy/test/helpers"
)

func TestMultiDriver_EdgeCases(t *testing.T) {
	forEachDriver(t, "EdgeCases", func(t *testing.T, _ string, provider *repo.Provider) {
		t.Run("Should error when workflow missing", func(t *testing.T) {
			ctx := helpers.NewTestContext(t)
			_, err := provider.NewWorkflowRepo().GetState(ctx, core.MustNewID())
			require.ErrorIs(t, err, store.ErrWorkflowNotFound)
		})

		t.Run("Should error when task missing", func(t *testing.T) {
			ctx := helpers.NewTestContext(t)
			_, err := provider.NewTaskRepo().GetState(ctx, core.MustNewID())
			require.ErrorIs(t, err, store.ErrTaskNotFound)
		})

		t.Run("Should merge usage summaries", func(t *testing.T) {
			ctx := helpers.NewTestContext(t)
			repo := provider.NewWorkflowRepo()
			execID := createWorkflowState(ctx, t, repo, "edge-usage", core.StatusRunning)
			summary := &usage.Summary{
				Entries: []usage.Entry{
					{Provider: "openai", Model: "gpt-4.1", PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
				},
			}
			require.NoError(t, repo.MergeUsage(ctx, execID, summary))

			state, err := repo.GetState(ctx, execID)
			require.NoError(t, err)
			require.NotNil(t, state.Usage)
			require.Len(t, state.Usage.Entries, 1)
			require.Equal(t, 30, state.Usage.Entries[0].TotalTokens)
		})
	})
}
