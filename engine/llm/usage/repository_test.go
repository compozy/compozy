package usage

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/require"
)

func TestRepositoryPersistInvokesCallback(t *testing.T) {
	t.Helper()
	var received atomic.Pointer[Finalized]
	repo, err := NewRepository(func(_ context.Context, finalized *Finalized) error {
		received.Store(finalized)
		return nil
	}, &RepositoryOptions{QueueCapacity: 4, WorkerCount: 1})
	require.NoError(t, err)
	t.Cleanup(repo.Stop)

	agentID := "agent-123"
	summary := &Summary{Entries: []Entry{{
		Provider:         "openai",
		Model:            "gpt-4o-mini",
		PromptTokens:     10,
		CompletionTokens: 5,
	}}}
	finalized := &Finalized{
		Metadata: Metadata{
			Component:      core.ComponentTask,
			WorkflowExecID: core.MustNewID(),
			TaskExecID:     core.MustNewID(),
			AgentID:        &agentID,
		},
		Summary: summary,
	}

	ctx := logger.ContextWithLogger(context.Background(), logger.NewLogger(logger.TestConfig()))
	require.NoError(t, repo.Persist(ctx, finalized))

	require.Eventually(t, func() bool {
		return received.Load() != nil
	}, time.Second, 10*time.Millisecond, "repository did not invoke persist callback")

	result := received.Load()
	require.NotNil(t, result)
	require.NotSame(t, summary, result.Summary, "summary should be cloned before persistence")
	require.Equal(t, len(summary.Entries), len(result.Summary.Entries))
	require.NotSame(t, &summary.Entries[0], &result.Summary.Entries[0])
	require.NotNil(t, result.Metadata.AgentID)
	require.NotSame(t, finalized.Metadata.AgentID, result.Metadata.AgentID)
}

func TestRepositoryStopPreventsNewRequests(t *testing.T) {
	repo, err := NewRepository(func(_ context.Context, _ *Finalized) error {
		return nil
	}, nil)
	require.NoError(t, err)

	repo.Stop()
	require.ErrorIs(t, repo.Persist(context.Background(), &Finalized{
		Summary: &Summary{Entries: []Entry{{Provider: "openai", Model: "gpt"}}},
	}), ErrRepositoryClosed)

	// Stop should be idempotent
	repo.Stop()
}

func TestCategorizePersistenceError(t *testing.T) {
	require.Equal(t, repositoryErrorTimeout, categorizePersistenceError(context.DeadlineExceeded))
	require.Equal(t, repositoryErrorValidation, categorizePersistenceError(errors.New("task not found")))
	require.Equal(t, repositoryErrorValidation, categorizePersistenceError(errors.New("validation failed")))
	require.Equal(t, repositoryErrorDatabase, categorizePersistenceError(errors.New("database unavailable")))
}
