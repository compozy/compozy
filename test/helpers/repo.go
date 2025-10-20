package helpers

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/infra/postgres"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
)

func SetupTestRepos(ctx context.Context, t *testing.T) (task.Repository, workflow.Repository, func()) {
	// Use retry logic for testcontainer setup
	pool, cleanup, err := SetupTestReposWithRetry(ctx, t)
	if err != nil {
		t.Fatalf("Failed to setup test repositories: %v", err)
	}
	// Perform health check
	if err := TestContainerHealthCheck(ctx, pool); err != nil {
		cleanup()
		t.Fatalf("Test container health check failed: %v", err)
	}
	// Create real repository instances
	taskRepo := postgres.NewTaskRepo(pool)
	workflowRepo := postgres.NewWorkflowRepo(pool)
	return taskRepo, workflowRepo, cleanup
}
