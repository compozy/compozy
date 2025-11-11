package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	helpers "github.com/compozy/compozy/test/helpers"
)

type repoTestEnv struct {
	ctx          context.Context
	taskRepo     task.Repository
	workflowRepo workflow.Repository
}

func newRepoTestEnv(t *testing.T) repoTestEnv {
	t.Helper()
	ctx := helpers.NewTestContext(t)
	taskRepo, workflowRepo, cleanup := helpers.SetupTestRepos(ctx, t)
	t.Cleanup(cleanup)
	return repoTestEnv{
		ctx:          ctx,
		taskRepo:     taskRepo,
		workflowRepo: workflowRepo,
	}
}

func upsertWorkflowState(
	t *testing.T,
	env repoTestEnv,
	workflowID string,
	execID core.ID,
	input *core.Input,
) {
	t.Helper()
	state := &workflow.State{
		WorkflowID:     workflowID,
		WorkflowExecID: execID,
		Status:         core.StatusRunning,
		Input:          input,
		Tasks:          make(map[string]*task.State),
	}
	require.NoError(t, env.workflowRepo.UpsertState(env.ctx, state))
}
