package preflight

import (
	"testing"

	"github.com/compozy/compozy/internal/core/tasks"
)

func testValidationRegistry(t *testing.T) *tasks.TypeRegistry {
	t.Helper()

	registry, err := tasks.NewRegistry(nil)
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	return registry
}

func testValidationReport() tasks.Report {
	return tasks.Report{
		TasksDir: "/tmp/tasks",
		Scanned:  2,
		Issues: []tasks.Issue{
			{
				Path:    "/tmp/tasks/task_01.md",
				Field:   "title",
				Message: "title is required",
			},
			{
				Path:    "/tmp/tasks/task_02.md",
				Field:   "type",
				Message: `type "" must be one of: backend, bugfix, chore, docs, frontend, infra, refactor, test`,
			},
		},
	}
}
