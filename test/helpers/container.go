package utils

import (
	"testing"
	"time"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	wf "github.com/compozy/compozy/engine/workflow"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ContainerTestConfig holds configuration for a test scenario using dedicated test database
type ContainerTestConfig struct {
	WorkflowConfig   *wf.Config
	ProjectConfig    *project.Config
	WorkflowRepo     wf.Repository
	TaskRepo         task.Repository
	DB               *pgxpool.Pool
	ExpectedDuration time.Duration
	testID           string // Unique test identifier
}

// Cleanup is now a no-op since we use a dedicated test database
func (c *ContainerTestConfig) Cleanup(t *testing.T) {
	t.Cleanup(func() {
		t.Logf("Test completed - no cleanup needed with dedicated test database")
	})
}

// TestConfigBuilder provides a fluent interface for building test configurations
type TestConfigBuilder struct {
	testID        string
	description   string
	workflowTasks []task.Config
	agents        []agent.Config
	tools         []tool.Config
	envVars       map[string]string
	dbPool        *pgxpool.Pool
	projectDir    string
}

// NewTestConfigBuilder creates a new test configuration builder
func NewTestConfigBuilder(t *testing.T) *TestConfigBuilder {
	return &TestConfigBuilder{
		testID:      GenerateUniqueTestID(t.Name()),
		description: "Test workflow for integration testing",
		envVars:     map[string]string{"TEST_MODE": "true"},
		dbPool:      GetSharedTestDB(t),
		projectDir:  t.TempDir(),
	}
}
