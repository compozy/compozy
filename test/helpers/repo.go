package helpers

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
)

// SetupTestRepos returns task and workflow repositories backed by the default
// test database driver (SQLite in-memory). Tests can override the driver by
// passing an explicit value (e.g. "postgres").
func SetupTestRepos(
	_ context.Context,
	t *testing.T,
	driver ...string,
) (task.Repository, workflow.Repository, func()) {
	t.Helper()
	selectedDriver := ""
	if len(driver) > 0 {
		selectedDriver = driver[0]
	}
	provider, cleanup := SetupTestDatabase(t, selectedDriver)
	taskRepo := provider.NewTaskRepo()
	workflowRepo := provider.NewWorkflowRepo()
	return taskRepo, workflowRepo, cleanup
}
