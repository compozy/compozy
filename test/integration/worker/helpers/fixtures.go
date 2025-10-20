package helpers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestFixture represents a test fixture for integration tests
type TestFixture struct {
	Name     string           `yaml:"name"`
	Workflow *workflow.Config `yaml:"workflow"`
	Tasks    []*task.Config   `yaml:"tasks,omitempty"`
	Input    map[string]any   `yaml:"input,omitempty"`
	Expected ExpectedResults  `yaml:"expected"`
	// ExpectedOutputs maps each action ID to a core.Output value
	// for per-action assertions when using DynamicMockLLM in integration tests.
	ExpectedOutputs map[string]core.Output `yaml:"expected_outputs,omitempty"`
}

// ExpectedResults represents the expected results of a test
type ExpectedResults struct {
	WorkflowState WorkflowStateExpectation `yaml:"workflow_state"`
	TaskStates    []TaskStateExpectation   `yaml:"task_states"`
	Error         string                   `yaml:"error,omitempty"`
}

// WorkflowStateExpectation represents expected workflow state
type WorkflowStateExpectation struct {
	Status         string         `yaml:"status"`
	TotalTasks     int            `yaml:"total_tasks,omitempty"`
	CompletedTasks int            `yaml:"completed_tasks,omitempty"`
	Output         map[string]any `yaml:"output,omitempty"`
}

// TaskStateExpectation represents expected task state
type TaskStateExpectation struct {
	Name           string         `yaml:"name"`
	ID             string         `yaml:"id,omitempty"`
	Status         string         `yaml:"status"`
	Inputs         map[string]any `yaml:"inputs,omitempty"`
	Output         map[string]any `yaml:"output,omitempty"`
	Error          string         `yaml:"error,omitempty"`
	Parent         string         `yaml:"parent,omitempty"`
	ExecutionOrder int            `yaml:"execution_order,omitempty"`
	ChildrenCount  int            `yaml:"children_count,omitempty"`
}

// FixtureLoader provides functionality to load test fixtures
type FixtureLoader struct {
	basePath string
}

// NewFixtureLoader creates a new fixture loader
func NewFixtureLoader(basePath string) *FixtureLoader {
	return &FixtureLoader{
		basePath: basePath,
	}
}

// LoadFixture loads a test fixture from a YAML file
func (l *FixtureLoader) LoadFixture(t *testing.T, taskType, fixtureName string) *TestFixture {
	// Construct the file path
	filePath := filepath.Join(l.basePath, "fixtures", taskType, fixtureName+".yaml")
	// Read the file
	data, err := os.ReadFile(filePath)
	require.NoError(t, err, "Failed to read fixture file: %s", filePath)
	// Parse the YAML
	var fixture TestFixture
	err = yaml.Unmarshal(data, &fixture)
	require.NoError(t, err, "Failed to parse fixture YAML: %s", filePath)
	// Validate the fixture
	l.validateFixture(t, &fixture)
	return &fixture
}

// LoadWorkflowConfig loads just the workflow configuration from a fixture
func (l *FixtureLoader) LoadWorkflowConfig(t *testing.T, taskType, fixtureName string) *workflow.Config {
	fixture := l.LoadFixture(t, taskType, fixtureName)
	require.NotNil(t, fixture.Workflow, "Fixture must contain a workflow configuration")
	return fixture.Workflow
}

// LoadTaskConfigs loads task configurations from a fixture
func (l *FixtureLoader) LoadTaskConfigs(t *testing.T, taskType, fixtureName string) []*task.Config {
	fixture := l.LoadFixture(t, taskType, fixtureName)
	if fixture.Tasks != nil {
		return fixture.Tasks
	}
	// If no separate tasks, extract from workflow
	if fixture.Workflow != nil && len(fixture.Workflow.Tasks) > 0 {
		// Convert []task.Config to []*task.Config
		tasks := make([]*task.Config, len(fixture.Workflow.Tasks))
		for i := range fixture.Workflow.Tasks {
			tasks[i] = &fixture.Workflow.Tasks[i]
		}
		return tasks
	}
	return []*task.Config{}
}

// validateFixture validates that a fixture has the required fields
func (l *FixtureLoader) validateFixture(t *testing.T, fixture *TestFixture) {
	require.NotEmpty(t, fixture.Name, "Fixture must have a name")
	require.NotEmpty(t, fixture.Expected.WorkflowState.Status, "Fixture must have expected workflow status")
	// Either workflow or tasks must be present
	if fixture.Workflow == nil && len(fixture.Tasks) == 0 {
		t.Fatal("Fixture must contain either a workflow or tasks")
	}
}

// CreateBasicWorkflowFixture creates a basic workflow fixture for testing
func CreateBasicWorkflowFixture(name string, taskConfig *task.Config) *TestFixture {
	return &TestFixture{
		Name: name,
		Workflow: &workflow.Config{
			ID:      name + "-workflow",
			Version: "1.0.0",
			Tasks:   []task.Config{*taskConfig},
		},
		Expected: ExpectedResults{
			WorkflowState: WorkflowStateExpectation{
				Status:         string(core.StatusSuccess),
				TotalTasks:     1,
				CompletedTasks: 1,
			},
			TaskStates: []TaskStateExpectation{
				{
					Name:   taskConfig.ID,
					Status: string(core.StatusSuccess),
				},
			},
		},
	}
}

// CreateBasicTaskConfig creates a basic task configuration
func CreateBasicTaskConfig(id string) *task.Config {
	return &task.Config{
		BaseConfig: task.BaseConfig{
			ID:   id,
			Type: task.TaskTypeBasic,
		},
		BasicTask: task.BasicTask{
			Action: "mock",
		},
	}
}

// AssertWorkflowState asserts that the workflow state matches expectations
func (f *TestFixture) AssertWorkflowState(t *testing.T, state *workflow.State) {
	expected := f.Expected.WorkflowState
	assert := require.New(t)
	assert.Equal(expected.Status, string(state.Status), "Workflow status mismatch")
	if expected.TotalTasks > 0 {
		actualTotalTasks := len(state.Tasks)
		assert.Equal(expected.TotalTasks, actualTotalTasks, "Total tasks count mismatch")
	}
	if expected.CompletedTasks > 0 {
		completedCount := 0
		for _, taskState := range state.Tasks {
			// Count tasks that have finished execution (success or failed) as completed
			// Running/pending tasks are not completed yet
			if taskState.Status == core.StatusSuccess || taskState.Status == core.StatusFailed {
				completedCount++
			}
		}
		assert.Equal(expected.CompletedTasks, completedCount, "Completed tasks count mismatch")
	}
	if expected.Output != nil {
		assert.NotNil(state.Output, "Expected workflow output but got nil")
		if state.Output != nil {
			for key, expectedValue := range expected.Output {
				actualValue, ok := (*state.Output)[key]
				assert.True(ok, "Output key %s not found in workflow output", key)
				if expectedStr, okStr := expectedValue.(string); okStr {
					actualStr := fmt.Sprint(actualValue)
					if !strings.Contains(actualStr, expectedStr) {
						t.Logf("Workflow output for %s:\n%q", key, actualStr)
					}
					assert.Contains(actualStr, expectedStr, "Output mismatch for key %s", key)
				} else {
					assert.Equal(expectedValue, actualValue, "Output mismatch for key %s", key)
				}
			}
		}
	}
}

// AssertTaskStates asserts that task states match expectations.
// It builds a lookup map and delegates detailed checks to helper functions.
func (f *TestFixture) AssertTaskStates(t *testing.T, states []*task.State) {
	t.Helper()
	assert := require.New(t)
	stateMap := mapTaskStatesByID(states)
	for i := range f.Expected.TaskStates {
		expected := &f.Expected.TaskStates[i]
		taskID := resolveExpectedTaskID(expected)
		state := requireTaskState(assert, stateMap, taskID)

		assertTaskStatusMatches(assert, expected, state, taskID)
		assertTaskParentMatches(assert, expected, state, taskID)
		assertTaskOutputMatches(t, assert, expected, state, taskID)
		assertTaskErrorMatches(assert, expected, state, taskID)
	}
}

// mapTaskStatesByID builds a lookup table for task states keyed by task identifier.
// It simplifies downstream assertions that reference states by ID or name.
func mapTaskStatesByID(states []*task.State) map[string]*task.State {
	stateMap := make(map[string]*task.State, len(states))
	for _, state := range states {
		stateMap[state.TaskID] = state
	}
	return stateMap
}

// resolveExpectedTaskID picks the best identifier available for the expected state.
// It prefers explicit IDs and falls back to the configured name.
func resolveExpectedTaskID(expected *TaskStateExpectation) string {
	if expected.ID != "" {
		return expected.ID
	}
	return expected.Name
}

// requireTaskState fetches a task state from the map and asserts its presence.
// Returning the state simplifies follow-up assertions.
func requireTaskState(
	assert *require.Assertions,
	stateMap map[string]*task.State,
	taskID string,
) *task.State {
	state, exists := stateMap[taskID]
	assert.True(exists, "Task state not found for task: %s", taskID)
	return state
}

// assertTaskStatusMatches verifies the task status aligns with expectations.
// Status mismatches indicate important behavioral regressions.
func assertTaskStatusMatches(
	assert *require.Assertions,
	expected *TaskStateExpectation,
	state *task.State,
	taskID string,
) {
	assert.Equal(expected.Status, string(state.Status), "Task %s status mismatch", taskID)
}

// assertTaskParentMatches ensures parent/child relationships align with fixtures.
// It fails fast when the fixture expects a parent that is missing.
func assertTaskParentMatches(
	assert *require.Assertions,
	expected *TaskStateExpectation,
	state *task.State,
	taskID string,
) {
	if expected.Parent == "" {
		assert.Nil(state.ParentStateID, "Expected no parent but found parent for task %s", taskID)
		return
	}
	if state.ParentStateID == nil {
		assert.NotNil(state.ParentStateID, "Expected parent %s but got nil for task %s", expected.Parent, taskID)
		return
	}
	assert.Equal(expected.Parent, string(*state.ParentStateID),
		"Parent mismatch for task %s", taskID)
}

// assertTaskOutputMatches compares expected output fragments with actual values.
// String expectations use substring matching to support templated content.
func assertTaskOutputMatches(
	t *testing.T,
	assert *require.Assertions,
	expected *TaskStateExpectation,
	state *task.State,
	taskID string,
) {
	t.Helper()
	if expected.Output == nil || state.Output == nil {
		return
	}
	for key, expectedValue := range expected.Output {
		actualValue, ok := (*state.Output)[key]
		assert.True(ok, "Output key %s not found in task %s", key, taskID)
		if expectedStr, okStr := expectedValue.(string); okStr {
			actualStr := fmt.Sprint(actualValue)
			if !strings.Contains(actualStr, expectedStr) {
				t.Logf("Actual output for %s/%s:\n%q", taskID, key, actualStr)
			}
			assert.Contains(actualStr, expectedStr, "Output mismatch for key %s in task %s", key, taskID)
			continue
		}
		assert.Equal(expectedValue, actualValue, "Output mismatch for key %s in task %s", key, taskID)
	}
}

// assertTaskErrorMatches verifies error expectations when provided in fixtures.
// It ensures the error struct is present and includes the expected details.
func assertTaskErrorMatches(
	assert *require.Assertions,
	expected *TaskStateExpectation,
	state *task.State,
	taskID string,
) {
	if expected.Error == "" {
		return
	}
	assert.NotNil(state.Error, "Expected error for task %s", taskID)
	if state.Error != nil {
		assert.Contains(state.Error.Message, expected.Error, "Error message mismatch for task %s", taskID)
	}
}
