package utils

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	wf "github.com/compozy/compozy/engine/workflow"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/testsuite"
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

// CreateContainerTestConfig creates a test configuration using the dedicated test PostgreSQL database
// No cleanup needed - uses dedicated test database!
func CreateContainerTestConfig(t *testing.T) *ContainerTestConfig {
	// Use dedicated test database connection - no setup overhead!
	dbPool := GetSharedTestDB(t)

	// Generate unique test ID for complete data isolation
	testID := GenerateUniqueTestID(t.Name())

	// Create repositories using shared test database connection
	workflowRepo := store.NewWorkflowRepo(dbPool)
	taskRepo := store.NewTaskRepo(dbPool)

	// Create test workflow configuration with unique ID
	workflowID := fmt.Sprintf("%s-workflow", testID)
	agentConfig := CreateTestAgentConfigWithAction(
		"test-agent",
		"You are a test assistant. Respond with the message provided.",
		"test-action",
		"Process this message: {{.parent.input.message}}",
	)
	workflowConfig := &wf.Config{
		ID:          workflowID,
		Version:     "1.0.0",
		Description: "Test workflow for integration testing",
		Tasks: []task.Config{
			{
				BaseConfig: task.BaseConfig{
					ID:    "test-task",
					Type:  task.TaskTypeBasic,
					Agent: agentConfig,
					With: &core.Input{
						"message": "Hello, World!",
					},
				},
				BasicTask: task.BasicTask{
					Action: "test-action",
				},
			},
		},
		Agents: []agent.Config{*agentConfig},
		Opts: wf.Opts{
			Env: &core.EnvMap{
				"TEST_MODE": "true",
			},
		},
	}

	// Create project configuration
	projectConfig := &project.Config{
		Name:    "test-project",
		Version: "1.0.0",
	}
	if err := projectConfig.SetCWD(t.TempDir()); err != nil {
		t.Fatalf("Failed to set project CWD: %v", err)
	}

	return &ContainerTestConfig{
		WorkflowConfig:   workflowConfig,
		ProjectConfig:    projectConfig,
		WorkflowRepo:     workflowRepo,
		TaskRepo:         taskRepo,
		DB:               dbPool,
		ExpectedDuration: 30 * time.Second,
		testID:           testID,
	}
}

// CreateContainerTestConfigForMultiTask creates a container test configuration for multi-task workflows
func CreateContainerTestConfigForMultiTask(t *testing.T, workflowConfig *wf.Config) *ContainerTestConfig {
	config := CreateContainerTestConfig(t)
	config.WorkflowConfig = workflowConfig
	return config
}

// CreateContainerTestConfigForCancellation creates a container test configuration for cancellation workflows
func CreateContainerTestConfigForCancellation(t *testing.T, workflowConfig *wf.Config) *ContainerTestConfig {
	config := CreateContainerTestConfig(t)
	config.WorkflowConfig = workflowConfig
	return config
}

// Cleanup is now a no-op since we use a dedicated test database
// No cleanup needed - test database isolation provides complete separation!
func (c *ContainerTestConfig) Cleanup(t *testing.T) {
	t.Cleanup(func() {
		// Nothing to cleanup! 🎉
		// The dedicated test database provides complete isolation
		// and can be reset entirely if needed via ResetTestDatabase()
		t.Logf("Test completed - no cleanup needed with dedicated test database")
	})
}

// ensureTablesExist creates the required tables if they don't exist
func ensureTablesExist(ctx context.Context, db *pgxpool.Pool) error {
	// Create workflow_states table (from migration)
	_, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS workflow_states (
			workflow_exec_id text NOT NULL PRIMARY KEY,
			workflow_id      text NOT NULL,
			status           text NOT NULL,
			input            jsonb,
			output           jsonb,
			error            jsonb,
			created_at       timestamptz NOT NULL DEFAULT now(),
			updated_at       timestamptz NOT NULL DEFAULT now()
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create workflow_states table: %w", err)
	}

	// Create task_states table (from migration)
	_, err = db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS task_states (
			component        text NOT NULL,
			status           text NOT NULL,
			task_exec_id     text NOT NULL PRIMARY KEY,
			task_id          text NOT NULL,
			workflow_exec_id text NOT NULL,
			workflow_id      text NOT NULL,
			agent_id         text,
			tool_id          text,
			action_id        text,
			input            jsonb,
			output           jsonb,
			error            jsonb,
			created_at       timestamptz NOT NULL DEFAULT now(),
			updated_at       timestamptz NOT NULL DEFAULT now(),

			-- parent linkage
			CONSTRAINT fk_workflow
			  FOREIGN KEY (workflow_exec_id)
			  REFERENCES workflow_states (workflow_exec_id)
			  ON DELETE CASCADE
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create task_states table: %w", err)
	}

	// Create indexes for performance (these are expensive, so we only create them once)
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_workflow_states_status ON workflow_states (status)",
		"CREATE INDEX IF NOT EXISTS idx_workflow_states_workflow_id ON workflow_states (workflow_id)",
		"CREATE INDEX IF NOT EXISTS idx_workflow_states_workflow_status ON workflow_states (workflow_id, status)",
		"CREATE INDEX IF NOT EXISTS idx_workflow_states_created_at ON workflow_states (created_at)",
		"CREATE INDEX IF NOT EXISTS idx_workflow_states_updated_at ON workflow_states (updated_at)",
		"CREATE INDEX IF NOT EXISTS idx_task_states_status ON task_states (status)",
		"CREATE INDEX IF NOT EXISTS idx_task_states_workflow_id ON task_states (workflow_id)",
		"CREATE INDEX IF NOT EXISTS idx_task_states_workflow_exec_id ON task_states (workflow_exec_id)",
		"CREATE INDEX IF NOT EXISTS idx_task_states_task_id ON task_states (task_id)",
		"CREATE INDEX IF NOT EXISTS idx_task_states_component ON task_states (component)",
		"CREATE INDEX IF NOT EXISTS idx_task_states_agent_id ON task_states (agent_id)",
		"CREATE INDEX IF NOT EXISTS idx_task_states_tool_id ON task_states (tool_id)",
		"CREATE INDEX IF NOT EXISTS idx_task_states_action_id ON task_states (action_id)",
		"CREATE INDEX IF NOT EXISTS idx_task_states_created_at ON task_states (created_at)",
		"CREATE INDEX IF NOT EXISTS idx_task_states_updated_at ON task_states (updated_at)",
		"CREATE INDEX IF NOT EXISTS idx_task_states_workflow_exec_id_task_id ON task_states (workflow_exec_id, task_id)",
		"CREATE INDEX IF NOT EXISTS idx_task_states_workflow_exec_id_agent_id ON task_states (workflow_exec_id, agent_id)",
		"CREATE INDEX IF NOT EXISTS idx_task_states_workflow_exec_id_tool_id ON task_states (workflow_exec_id, tool_id)",
		"CREATE INDEX IF NOT EXISTS idx_task_states_workflow_id_task_id ON task_states (workflow_id, task_id)",
		"CREATE INDEX IF NOT EXISTS idx_task_states_workflow_id_agent_id ON task_states (workflow_id, agent_id)",
		"CREATE INDEX IF NOT EXISTS idx_task_states_workflow_id_tool_id ON task_states (workflow_id, tool_id)",
		"CREATE INDEX IF NOT EXISTS idx_task_states_workflow_id_action_id ON task_states (workflow_id, action_id)",
	}

	for _, index := range indexes {
		if _, err := db.Exec(ctx, index); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	return nil
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

// CreatePauseableWorkflowConfig creates a workflow config with multiple tasks for pause/resume testing
func CreatePauseableWorkflowConfig() *wf.Config {
	testID := GenerateUniqueTestID("pauseable")
	actions := map[string]string{
		"action-1": "Process step 1: {{.parent.input.step}}",
		"action-2": "Process step 2: {{.parent.input.step}}",
		"action-3": "Process step 3: {{.parent.input.step}}",
	}
	agentConfig := CreateTestAgentConfigWithActions(
		"test-agent",
		"You are a test assistant. Respond with the message provided.",
		actions,
	)

	return &wf.Config{
		ID:          testID,
		Version:     "1.0.0",
		Description: "Multi-task workflow for pause/resume testing",
		Tasks: []task.Config{
			{
				BaseConfig: task.BaseConfig{
					ID:    "task-1",
					Type:  task.TaskTypeBasic,
					Agent: agentConfig,
					With: &core.Input{
						"step": "1",
					},
					OnSuccess: &core.SuccessTransition{
						Next: stringPtr("task-2"),
					},
				},
				BasicTask: task.BasicTask{
					Action: "action-1",
				},
			},
			{
				BaseConfig: task.BaseConfig{
					ID:    "task-2",
					Type:  task.TaskTypeBasic,
					Agent: agentConfig,
					With: &core.Input{
						"step": "2",
					},
					OnSuccess: &core.SuccessTransition{
						Next: stringPtr("task-3"),
					},
				},
				BasicTask: task.BasicTask{
					Action: "action-2",
				},
			},
			{
				BaseConfig: task.BaseConfig{
					ID:    "task-3",
					Type:  task.TaskTypeBasic,
					Agent: agentConfig,
					With: &core.Input{
						"step": "3",
					},
				},
				BasicTask: task.BasicTask{
					Action: "action-3",
				},
			},
		},
		Agents: []agent.Config{*agentConfig},
		Opts: wf.Opts{
			Env: &core.EnvMap{
				"TEST_MODE": "true",
			},
		},
	}
}

// CreateCancellableWorkflowConfig creates a workflow that can be canceled during execution
func CreateCancellableWorkflowConfig() *wf.Config {
	testID := GenerateUniqueTestID("cancellable")
	agentConfig := CreateTestAgentConfigWithAction(
		"slow-agent",
		"You are a slow test assistant. Take your time to process.",
		"long-action",
		"Process for duration: {{.parent.input.duration}}. Think deeply.",
	)
	return &wf.Config{
		ID:          testID,
		Version:     "1.0.0",
		Description: "Long-running workflow for cancellation testing",
		Tasks: []task.Config{
			{
				BaseConfig: task.BaseConfig{
					ID:    "long-task",
					Type:  task.TaskTypeBasic,
					Agent: agentConfig,
					With: &core.Input{
						"duration": "10s",
					},
					Sleep: "2s", // Add sleep to simulate long-running task that can be canceled
				},
				BasicTask: task.BasicTask{
					Action: "long-action",
				},
			},
		},
		Agents: []agent.Config{*agentConfig},
		Opts: wf.Opts{
			Env: &core.EnvMap{
				"TEST_MODE": "true",
			},
		},
	}
}

// SignalHelper provides utilities for testing signal operations
type SignalHelper struct {
	env *testsuite.TestWorkflowEnvironment
	t   *testing.T
}

// NewSignalHelper creates a new signal testing helper
func NewSignalHelper(env *testsuite.TestWorkflowEnvironment, t *testing.T) *SignalHelper {
	return &SignalHelper{
		env: env,
		t:   t,
	}
}

// WaitAndSendSignal waits for a duration then sends a signal
func (sh *SignalHelper) WaitAndSendSignal(waitDuration time.Duration, signalFunc func()) {
	sh.env.RegisterDelayedCallback(func() {
		signalFunc()
	}, waitDuration)
}

// StatusValidator helps validate workflow and task status changes
type StatusValidator struct {
	t              *testing.T
	expectedStates []core.StatusType
	currentIndex   int
}

// NewStatusValidator creates a new status validator
func NewStatusValidator(t *testing.T, expectedStates []core.StatusType) *StatusValidator {
	return &StatusValidator{
		t:              t,
		expectedStates: expectedStates,
		currentIndex:   0,
	}
}

// ValidateStatusTransition validates that the status changed as expected
func (sv *StatusValidator) ValidateStatusTransition(actualStatus core.StatusType) {
	if sv.currentIndex >= len(sv.expectedStates) {
		sv.t.Errorf("Unexpected status transition to %s - no more expected transitions", actualStatus)
		return
	}

	expected := sv.expectedStates[sv.currentIndex]
	if expected != actualStatus {
		sv.t.Errorf("Status transition %d: expected %s, got %s", sv.currentIndex, expected, actualStatus)
	}

	sv.currentIndex++
}

// IsComplete returns true if all expected status transitions have been validated
func (sv *StatusValidator) IsComplete() bool {
	return sv.currentIndex >= len(sv.expectedStates)
}
