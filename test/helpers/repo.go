package utils

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/infra/store"
)

func SetupTestRepos(ctx context.Context, t *testing.T) (*store.TaskRepo, *store.WorkflowRepo, func()) {
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
	taskRepo := store.NewTaskRepo(pool)
	workflowRepo := store.NewWorkflowRepo(pool)
	return taskRepo, workflowRepo, cleanup
}
