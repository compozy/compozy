package router

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/llm/usage"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
)

func TestNewUsageSummary(t *testing.T) {
	t.Parallel()

	t.Run("returns nil for nil summary", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, NewUsageSummary(nil))
	})

	t.Run("serializes entries and preserves metadata", func(t *testing.T) {
		t.Parallel()
		captured := time.Now().Add(-time.Minute).UTC()
		updated := time.Now().UTC()
		reasoning := 3
		entry := usage.Entry{
			Provider:         "openai",
			Model:            "gpt-4o-mini",
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
			ReasoningTokens:  &reasoning,
			AgentIDs:         []string{"agent-a", "agent-b"},
			CapturedAt:       &captured,
			UpdatedAt:        &updated,
			Source:           string(usage.SourceTask),
		}
		summary := &usage.Summary{Entries: []usage.Entry{entry}}

		apiSummary := NewUsageSummary(summary)
		require.NotNil(t, apiSummary)
		require.Len(t, apiSummary.Entries, 1)

		result := apiSummary.Entries[0]
		assert.Equal(t, entry.Provider, result.Provider)
		assert.Equal(t, entry.Model, result.Model)
		assert.Equal(t, entry.TotalTokens, result.TotalTokens)
		require.NotNil(t, result.ReasoningTokens)
		assert.Equal(t, reasoning, *result.ReasoningTokens)
		require.NotNil(t, result.CapturedAt)
		assert.Equal(t, captured.UTC(), result.CapturedAt.UTC())
		require.NotNil(t, result.UpdatedAt)
		assert.Equal(t, updated.UTC(), result.UpdatedAt.UTC())
		assert.Equal(t, entry.AgentIDs, result.AgentIDs)
		assert.Equal(t, entry.Source, result.Source)
	})
}

func TestResolveTaskUsageSummary(t *testing.T) {
	t.Parallel()
	taskID := core.MustNewID()
	ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())

	t.Run("nil repo returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, ResolveTaskUsageSummary(ctx, nil, taskID))
	})

	t.Run("zero task exec ID returns nil", func(t *testing.T) {
		t.Parallel()
		repo := &stubTaskRepo{}
		assert.Nil(t, ResolveTaskUsageSummary(ctx, repo, core.ID("")))
	})

	t.Run("returns summary when state loaded", func(t *testing.T) {
		t.Parallel()
		summary := &usage.Summary{Entries: []usage.Entry{{Provider: "anthropic", Model: "claude-3"}}}
		repo := &stubTaskRepo{
			state: &task.State{
				TaskExecID: taskID,
				Usage:      summary,
			},
		}
		result := ResolveTaskUsageSummary(ctx, repo, taskID)
		require.NotNil(t, result)
		require.Len(t, result.Entries, 1)
		assert.Equal(t, "anthropic", result.Entries[0].Provider)
	})

	t.Run("not found error returns nil", func(t *testing.T) {
		t.Parallel()
		repo := &stubTaskRepo{err: store.ErrTaskNotFound}
		assert.Nil(t, ResolveTaskUsageSummary(ctx, repo, taskID))
	})

	t.Run("unexpected error returns nil", func(t *testing.T) {
		t.Parallel()
		repo := &stubTaskRepo{err: errors.New("boom")}
		assert.Nil(t, ResolveTaskUsageSummary(ctx, repo, taskID))
	})
}

func TestResolveWorkflowUsageSummary(t *testing.T) {
	t.Parallel()
	execID := core.MustNewID()
	ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())

	t.Run("nil repo returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, ResolveWorkflowUsageSummary(ctx, nil, execID))
	})

	t.Run("zero workflow exec ID returns nil", func(t *testing.T) {
		t.Parallel()
		repo := &stubWorkflowRepo{}
		assert.Nil(t, ResolveWorkflowUsageSummary(ctx, repo, core.ID("")))
	})

	t.Run("returns summary when state loaded", func(t *testing.T) {
		t.Parallel()
		summary := &usage.Summary{Entries: []usage.Entry{{Provider: "openai", Model: "gpt-4o"}}}
		repo := &stubWorkflowRepo{
			state: &workflow.State{
				WorkflowExecID: execID,
				Usage:          summary,
			},
		}
		result := ResolveWorkflowUsageSummary(ctx, repo, execID)
		require.NotNil(t, result)
		require.Len(t, result.Entries, 1)
		assert.Equal(t, "openai", result.Entries[0].Provider)
	})

	t.Run("not found error returns nil", func(t *testing.T) {
		t.Parallel()
		repo := &stubWorkflowRepo{err: store.ErrWorkflowNotFound}
		assert.Nil(t, ResolveWorkflowUsageSummary(ctx, repo, execID))
	})

	t.Run("unexpected error returns nil", func(t *testing.T) {
		t.Parallel()
		repo := &stubWorkflowRepo{err: errors.New("db offline")}
		assert.Nil(t, ResolveWorkflowUsageSummary(ctx, repo, execID))
	})
}

type stubTaskRepo struct {
	state *task.State
	err   error
}

func (s *stubTaskRepo) ListStates(context.Context, *task.StateFilter) ([]*task.State, error) {
	panic("ListStates not implemented")
}

func (s *stubTaskRepo) UpsertState(context.Context, *task.State) error {
	panic("UpsertState not implemented")
}

func (s *stubTaskRepo) GetState(context.Context, core.ID) (*task.State, error) {
	return s.state, s.err
}

func (s *stubTaskRepo) WithTransaction(context.Context, func(task.Repository) error) error {
	panic("WithTransaction not implemented")
}

func (s *stubTaskRepo) GetStateForUpdate(context.Context, core.ID) (*task.State, error) {
	panic("GetStateForUpdate not implemented")
}

func (s *stubTaskRepo) ListTasksInWorkflow(context.Context, core.ID) (map[string]*task.State, error) {
	panic("ListTasksInWorkflow not implemented")
}

func (s *stubTaskRepo) ListTasksByStatus(context.Context, core.ID, core.StatusType) ([]*task.State, error) {
	panic("ListTasksByStatus not implemented")
}

func (s *stubTaskRepo) ListTasksByAgent(context.Context, core.ID, string) ([]*task.State, error) {
	panic("ListTasksByAgent not implemented")
}

func (s *stubTaskRepo) ListTasksByTool(context.Context, core.ID, string) ([]*task.State, error) {
	panic("ListTasksByTool not implemented")
}

func (s *stubTaskRepo) ListChildren(context.Context, core.ID) ([]*task.State, error) {
	panic("ListChildren not implemented")
}

func (s *stubTaskRepo) GetChildByTaskID(context.Context, core.ID, string) (*task.State, error) {
	panic("GetChildByTaskID not implemented")
}

func (s *stubTaskRepo) GetTaskTree(context.Context, core.ID) ([]*task.State, error) {
	panic("GetTaskTree not implemented")
}

func (s *stubTaskRepo) ListChildrenOutputs(context.Context, core.ID) (map[string]*core.Output, error) {
	panic("ListChildrenOutputs not implemented")
}

func (s *stubTaskRepo) GetProgressInfo(context.Context, core.ID) (*task.ProgressInfo, error) {
	panic("GetProgressInfo not implemented")
}

func (s *stubTaskRepo) MergeUsage(context.Context, core.ID, *usage.Summary) error {
	return nil
}

type stubWorkflowRepo struct {
	state *workflow.State
	err   error
}

func (s *stubWorkflowRepo) ListStates(context.Context, *workflow.StateFilter) ([]*workflow.State, error) {
	panic("ListStates not implemented")
}

func (s *stubWorkflowRepo) UpsertState(context.Context, *workflow.State) error {
	panic("UpsertState not implemented")
}

func (s *stubWorkflowRepo) UpdateStatus(context.Context, core.ID, core.StatusType) error {
	panic("UpdateStatus not implemented")
}

func (s *stubWorkflowRepo) GetState(context.Context, core.ID) (*workflow.State, error) {
	return s.state, s.err
}

func (s *stubWorkflowRepo) GetStateByID(context.Context, string) (*workflow.State, error) {
	panic("GetStateByID not implemented")
}

func (s *stubWorkflowRepo) GetStateByTaskID(context.Context, string, string) (*workflow.State, error) {
	panic("GetStateByTaskID not implemented")
}

func (s *stubWorkflowRepo) GetStateByAgentID(context.Context, string, string) (*workflow.State, error) {
	panic("GetStateByAgentID not implemented")
}

func (s *stubWorkflowRepo) GetStateByToolID(context.Context, string, string) (*workflow.State, error) {
	panic("GetStateByToolID not implemented")
}

func (s *stubWorkflowRepo) CompleteWorkflow(
	context.Context,
	core.ID,
	workflow.OutputTransformer,
) (*workflow.State, error) {
	panic("CompleteWorkflow not implemented")
}

func (s *stubWorkflowRepo) MergeUsage(context.Context, core.ID, *usage.Summary) error {
	return nil
}
