package test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileReferences tests the CWD handling with file references
func TestFileReferences(t *testing.T) {
	// Find the absolute path to the example directory
	examplePath, err := filepath.Abs(filepath.Join("../../examples/quotes", "compozy.yaml"))
	require.NoError(t, err, "Failed to get absolute path to example")
	// Load the project configuration
	projectCWD, err := core.CWDFromPath(filepath.Dir(examplePath))
	require.NoError(t, err, "Failed to get project CWD")
	projectConfig, err := project.Load(context.Background(), projectCWD, examplePath)
	require.NoError(t, err, "Failed to load project config")
	// Test paths we'll use across multiple tests
	expectedProjectCWD, err := filepath.Abs("../../examples/quotes")
	require.NoError(t, err, "Failed to get expected project CWD")
	tasksPath, err := filepath.Abs("../../examples/quotes/tasks")
	require.NoError(t, err, "Failed to get tasks path")
	toolsPath, err := filepath.Abs("../../examples/quotes/tools")
	require.NoError(t, err, "Failed to get tools path")
	agentsPath, err := filepath.Abs("../../examples/quotes/agents")
	require.NoError(t, err, "Failed to get agents path")

	// Load workflows once for all tests
	workflows, err := workflow.WorkflowsFromProject(context.Background(), projectConfig)
	require.NoError(t, err, "Failed to load workflows")
	require.NotEmpty(t, workflows, "No workflows were loaded")
	require.Len(t, workflows, 1, "Expected one workflow")
	workflowConfig := workflows[0]

	t.Run("Should set project CWD correctly", func(t *testing.T) {
		assert.Equal(t, expectedProjectCWD, projectConfig.GetCWD().PathStr(), "Project CWD not set correctly")
	})

	t.Run("Should set workflow CWD correctly", func(t *testing.T) {
		assert.Equal(t, expectedProjectCWD, workflowConfig.GetCWD().PathStr(), "Workflow CWD not set correctly")
	})

	t.Run("Should load tasks correctly", func(t *testing.T) {
		require.Len(t, workflowConfig.Tasks, 3, "Expected three tasks")
	})

	t.Run("First task (get_quote)", func(t *testing.T) {
		getQuoteTask := &workflowConfig.Tasks[0]
		getQuoteTaskCWD := getQuoteTask.GetCWD()
		require.NotNil(t, getQuoteTask.Executor.Ref, "Task executor ref is nil")
		assert.Equal(t, getQuoteTaskCWD.PathStr(), tasksPath, "Task CWD not set correctly")
		assert.Equal(t, "get_quote", getQuoteTask.ID, "Task ID not set correctly")
		assert.Equal(t, "basic", string(getQuoteTask.Type), "Task type not set correctly")
		assert.Equal(t, "tool", string(getQuoteTask.Executor.Type), "Expected tool executor type")
		tool, err := getQuoteTask.Executor.GetTool()
		require.NoError(t, err, "Failed to get resolved tool config")
		assert.Equal(t, "get_quote", tool.ID, "Tool ID not resolved correctly")
	})

	t.Run("Second task (translate_quote)", func(t *testing.T) {
		translateTask := &workflowConfig.Tasks[1]
		assert.Equal(t, "translate_quote", translateTask.ID, "Task ID not set correctly")
		assert.Equal(t, "basic", string(translateTask.Type), "Task type not set correctly")
		assert.Equal(t, tasksPath, translateTask.GetCWD().PathStr(), "Task CWD not set correctly")
		assert.Equal(t, "agent", string(translateTask.Executor.Type), "Expected agent executor type")
		agent, err := translateTask.Executor.GetAgent()
		require.NoError(t, err, "Failed to get resolved agent config")
		assert.Equal(t, "translator", agent.ID, "Agent ID not resolved correctly")
	})

	t.Run("Third task (save_results)", func(t *testing.T) {
		saveTask := &workflowConfig.Tasks[2]
		assert.Equal(t, "save_results", saveTask.ID, "Task ID not set correctly")
		assert.Equal(t, "basic", string(saveTask.Type), "Task type not set correctly")
		assert.Equal(t, "tool", string(saveTask.Executor.Type), "Expected tool executor type")
		tool, err := saveTask.Executor.GetTool()
		require.NoError(t, err, "Failed to get resolved tool config")
		assert.Equal(t, "save_data", tool.ID, "Tool ID not resolved correctly")
		assert.True(t, saveTask.Final, "Expected task to be final")
	})

	t.Run("Should load tools referenced in tasks", func(t *testing.T) {
		require.GreaterOrEqual(t, len(workflowConfig.Tools), 2, "Expected at least 2 tools to be loaded")
	})

	t.Run("get_quote tool from first task", func(t *testing.T) {
		var getQuoteTool *tool.Config
		for i := range workflowConfig.Tools {
			tool := &workflowConfig.Tools[i]
			if tool.ID == "get_quote" {
				getQuoteTool = tool
				break
			}
		}
		require.NotNil(t, getQuoteTool, "get_quote tool not found in workflow config")
		assert.Equal(t, "get_quote", getQuoteTool.ID, "Tool ID not set correctly")
		assert.Equal(t, "Get a random Game of Thrones quote", getQuoteTool.Description, "Tool description not set correctly")
		assert.Equal(t, "./get_quote.ts", getQuoteTool.Execute, "Tool execute command not set correctly")
		assert.Equal(t, toolsPath, getQuoteTool.GetCWD().PathStr(), "Tool CWD not set correctly")
	})

	t.Run("save_data tool from third task", func(t *testing.T) {
		var saveDataTool *tool.Config
		for i := range workflowConfig.Tools {
			tool := &workflowConfig.Tools[i]
			if tool.ID == "save_data" {
				saveDataTool = tool
				break
			}
		}
		require.NotNil(t, saveDataTool, "save_data tool not found in workflow config")
		assert.Equal(t, "save_data", saveDataTool.ID, "Tool ID not set correctly")
		assert.Equal(t, "Save data to a file", saveDataTool.Description, "Tool description not set correctly")
		assert.Equal(t, "./save_data.ts", saveDataTool.Execute, "Tool execute command not set correctly")
		require.NotNil(t, saveDataTool.InputSchema, "Tool input schema is nil")
		require.NotNil(t, saveDataTool.OutputSchema, "Tool output schema is nil")
		toolCount := 0
		for _, tool := range workflowConfig.Tools {
			if tool.ID == "save_data" {
				toolCount++
			}
		}
		assert.Equal(t, 1, toolCount, "Tool was added multiple times")
	})

	t.Run("Should load agents referenced in tasks", func(t *testing.T) {
		require.GreaterOrEqual(t, len(workflowConfig.Agents), 1, "Expected at least 1 agent to be loaded")
	})

	t.Run("translator agent from second task", func(t *testing.T) {
		var translatorAgent *agent.Config
		for i := range workflowConfig.Agents {
			agent := &workflowConfig.Agents[i]
			if agent.ID == "translator" {
				translatorAgent = agent
				break
			}
		}
		require.NotNil(t, translatorAgent, "translator agent not found in workflow config")
		assert.Equal(t, "translator", translatorAgent.ID, "Agent ID not set correctly")
		assert.Equal(t, "groq", string(translatorAgent.Config.Provider), "Agent provider not set correctly")
		assert.Equal(t, "llama-3.3-70b-versatile", string(translatorAgent.Config.Model), "Agent model not set correctly")
		require.GreaterOrEqual(t, len(translatorAgent.Actions), 1, "Expected at least 1 action in translator agent")
		assert.Equal(t, "translate", translatorAgent.Actions[0].ID, "Agent action ID not set correctly")
		assert.Equal(t, agentsPath, translatorAgent.GetCWD().PathStr(), "Agent CWD not set correctly")
		agentCount := 0
		for _, agent := range workflowConfig.Agents {
			if agent.ID == "translator" {
				agentCount++
			}
		}
		assert.Equal(t, 1, agentCount, "Agent was added multiple times")
	})

	t.Run("Should correctly handle file references through all loader functions", func(t *testing.T) {
		firstTask := &workflowConfig.Tasks[0]
		assert.Equal(t, "tool", string(firstTask.Executor.Type), "Expected tool executor type")
		toolFound := false
		for _, tool := range workflowConfig.Tools {
			if tool.ID == "get_quote" {
				toolFound = true
				break
			}
		}
		assert.True(t, toolFound, "Referenced tool was not loaded")
		secondTask := &workflowConfig.Tasks[1]
		assert.Equal(t, "agent", string(secondTask.Executor.Type), "Expected agent executor type")
		agentFound := false
		for _, agent := range workflowConfig.Agents {
			if agent.ID == "translator" {
				agentFound = true
				break
			}
		}
		assert.True(t, agentFound, "Referenced agent was not loaded")
	})
}
