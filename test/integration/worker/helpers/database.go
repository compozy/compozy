package helpers

import (
	"testing"

	"github.com/compozy/compozy/engine/infra/repo"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	helpers "github.com/compozy/compozy/test/helpers"
)

// DatabaseHelper provides database setup and teardown for integration tests.
type DatabaseHelper struct {
	provider *repo.Provider
	cleanup  func()
}

// NewDatabaseHelper provisions a fast in-memory test database.
func NewDatabaseHelper(t *testing.T) *DatabaseHelper {
	t.Helper()
	provider, cleanup := helpers.SetupTestDatabase(t)
	return &DatabaseHelper{
		provider: provider,
		cleanup:  cleanup,
	}
}

// Provider exposes the underlying repository provider.
func (h *DatabaseHelper) Provider() *repo.Provider {
	return h.provider
}

// TaskRepo returns a task repository instance backed by the helper database.
func (h *DatabaseHelper) TaskRepo() task.Repository {
	return h.provider.NewTaskRepo()
}

// WorkflowRepo returns a workflow repository instance backed by the helper database.
func (h *DatabaseHelper) WorkflowRepo() workflow.Repository {
	return h.provider.NewWorkflowRepo()
}

// Cleanup releases database resources.
func (h *DatabaseHelper) Cleanup(t *testing.T) {
	if h.cleanup != nil {
		h.cleanup()
	}
	t.Logf("Database helper cleanup completed")
}
