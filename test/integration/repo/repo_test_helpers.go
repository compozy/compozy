package store

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/postgres"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	helpers "github.com/compozy/compozy/test/helpers"
)

type repoTestEnv struct {
	ctx          context.Context
	pool         *pgxpool.Pool
	taskRepo     *postgres.TaskRepo
	workflowRepo *postgres.WorkflowRepo
}

func newRepoTestEnv(t *testing.T) repoTestEnv {
	t.Helper()

	baseCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	ctx := logger.ContextWithLogger(baseCtx, logger.NewForTests())

	manager := config.NewManager(config.NewService())
	_, err := manager.Load(ctx, config.NewDefaultProvider())
	require.NoError(t, err)
	ctx = config.ContextWithManager(ctx, manager)

	pool, cleanup := helpers.GetSharedPostgresDB(ctx, t)
	t.Cleanup(cleanup)

	require.NoError(t, helpers.EnsureTablesExistForTest(pool))

	truncateRepoTables(ctx, t, pool)

	return repoTestEnv{
		ctx:          ctx,
		pool:         pool,
		taskRepo:     postgres.NewTaskRepo(pool),
		workflowRepo: postgres.NewWorkflowRepo(pool),
	}
}

func truncateRepoTables(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(ctx, "TRUNCATE task_states, workflow_states CASCADE")
	require.NoError(t, err)
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
