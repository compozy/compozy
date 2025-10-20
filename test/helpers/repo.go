package helpers

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/infra/postgres"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
)

func SetupTestRepos(ctx context.Context, t *testing.T) (task.Repository, workflow.Repository, func()) {
	pool, cleanup, err := SetupTestReposWithRetry(ctx, t)
	if err != nil {
		t.Fatalf("Failed to setup test repositories: %v", err)
	}
	if err := TestContainerHealthCheck(ctx, pool); err != nil {
		cleanup()
		t.Fatalf("Test container health check failed: %v", err)
	}
	taskRepo := postgres.NewTaskRepo(pool)
	workflowRepo := postgres.NewWorkflowRepo(pool)
	return taskRepo, workflowRepo, cleanup
}
