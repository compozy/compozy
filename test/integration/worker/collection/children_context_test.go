package collection

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/test/integration/worker/helpers"
)

const (
	// Error markers for missing children context
	errNoChildrenContext   = "NO_CHILDREN"
	errNoChildFound        = "NO_CHILD"
	errNoChildrenProperty  = "NO_CHILDREN_PROPERTY"
	errNoChildFoundInTasks = "NO_CHILD_FOUND"
)

func TestCollectionTask_ChildrenContextAccess(t *testing.T) {
	t.Run("Should access children properties through new context pattern", func(t *testing.T) {
		t.Parallel()
		basePath := getTestDir()
		fixtureLoader := helpers.NewFixtureLoader(basePath)
		dbHelper := helpers.NewDatabaseHelper(t)
		t.Cleanup(func() { dbHelper.Cleanup(t) })
		fixture := fixtureLoader.LoadFixture(t, "", "children_context_access")
		t.Log("Executing workflow with children context access")
		agentConfig := createChildrenContextTestAgent()
		result := helpers.ExecuteWorkflowAndGetState(t, fixture, dbHelper, "test-project-children-context", agentConfig)
		fixture.AssertWorkflowState(t, result)
		verifyChildrenContextAccess(t, result)
	})
}

func createChildrenContextTestAgent() *agent.Config {
	return &agent.Config{
		ID:           "test-children-context-agent",
		Config:       core.ProviderConfig{Provider: core.ProviderMock, Model: "test-model"},
		Instructions: "Test agent for children context validation",
		Actions: []*agent.ActionConfig{
			{
				ID:     "validate_item",
				Prompt: "Validate an item against threshold",
				InputSchema: &schema.Schema{
					"type": "object",
					"properties": map[string]any{
						"item": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"name":  map[string]any{"type": "string"},
								"value": map[string]any{"type": "number"},
							},
						},
						"threshold": map[string]any{"type": "number"},
					},
				},
			},
			{
				ID:     "process_results",
				Prompt: "Process validation results",
				InputSchema: &schema.Schema{
					"type":                 "object",
					"additionalProperties": true,
				},
			},
		},
	}
}

func verifyChildrenContextAccess(t *testing.T, result *workflow.State) {
	require.NotNil(t, result, "Workflow state should not be nil")
	require.NotNil(t, result.Tasks, "Tasks map should not be nil")
	var validateTask *task.State
	for _, taskState := range result.Tasks {
		if taskState.TaskID == "validate-items" {
			validateTask = taskState
			break
		}
	}
	require.NotNil(t, validateTask, "validate-items task should exist")
	require.NotNil(t, validateTask.Output, "validate-items task output should not be nil")
	validateOutput := *validateTask.Output
	allResults, hasAllResults := validateOutput["all_results"]
	assert.True(t, hasAllResults, "Should have all_results in output")
	assert.NotNil(t, allResults, "all_results should not be nil")
	// Check if children context worked
	firstChildOutput, hasFirstChildOutput := validateOutput["first_child_output"]
	assert.True(t, hasFirstChildOutput, "Should have first_child_output in output")
	assert.NotEqual(t, errNoChildrenContext, firstChildOutput, "Children context should be available")
	assert.NotEqual(t, errNoChildFound, firstChildOutput, "validate-0 child should exist")
	firstChildStatus, hasFirstChildStatus := validateOutput["first_child_status"]
	assert.True(t, hasFirstChildStatus, "Should have first_child_status in output")
	assert.NotEqual(t, errNoChildrenContext, firstChildStatus, "Children context should be available")
	assert.NotEqual(t, errNoChildFound, firstChildStatus, "validate-0 child should exist")
	var processTask *task.State
	for _, taskState := range result.Tasks {
		if taskState.TaskID == "process-validated" {
			processTask = taskState
			break
		}
	}
	require.NotNil(t, processTask, "process-validated task should exist")
	assert.Equal(t, core.StatusSuccess, processTask.Status, "Process task should succeed")
}
