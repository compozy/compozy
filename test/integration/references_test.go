package integration

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/internal/parser/agent"
	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/project"
	"github.com/compozy/compozy/internal/parser/tool"
)

// TestFileReferences tests the CWD handling with file references
func TestFileReferences(t *testing.T) {
	// Find the absolute path to the example directory
	examplePath, err := filepath.Abs("../../examples/quotes/compozy.yaml")
	require.NoError(t, err, "Failed to get absolute path to example")

	// Load the project configuration
	projectCWD, err := common.CWDFromPath(".")
	require.NoError(t, err, "Failed to get project CWD")

	projectConfig, err := project.Load(projectCWD, examplePath)
	require.NoError(t, err, "Failed to load project config")

	// Test paths we'll use across multiple tests
	expectedProjectCWD, err := filepath.Abs("../../examples/quotes")
	require.NoError(t, err, "Failed to get expected project CWD")

	tasksPath, err := filepath.Abs("../../examples/quotes/tasks")
	require.NoError(t, err, "Failed to get tasks path")

	t.Run("Should set project CWD correctly", func(t *testing.T) {
		assert.Equal(t, expectedProjectCWD, projectConfig.GetCWD().PathStr(), "Project CWD not set correctly")
	})

	t.Run("Should load workflows from sources", func(t *testing.T) {
		workflows, err := projectConfig.WorkflowsFromSources()
		require.NoError(t, err, "Failed to load workflows")
		require.NotEmpty(t, workflows, "No workflows were loaded")
		require.Len(t, workflows, 1, "Expected one workflow")

		workflow := workflows[0]

		t.Run("Should set workflow CWD correctly", func(t *testing.T) {
			assert.Equal(t, expectedProjectCWD, workflow.GetCWD().PathStr(), "Workflow CWD not set correctly")
		})

		t.Run("Should load tasks correctly", func(t *testing.T) {
			require.Len(t, workflow.Tasks, 3, "Expected three tasks")

			t.Run("First task (get_quote)", func(t *testing.T) {
				getQuoteTask := &workflow.Tasks[0]
				getQuoteTaskCWD := getQuoteTask.GetCWD()
				require.NotNil(t, getQuoteTask.Use, "Task use is nil")
				assert.Equal(t, getQuoteTaskCWD.PathStr(), tasksPath, "Task CWD not set correctly")
				assert.Equal(t, "get_quote", getQuoteTask.ID, "Task ID not set correctly")
				assert.Equal(t, "basic", string(getQuoteTask.Type), "Task type not set correctly")

				// Verify use reference
				useRef, err := getQuoteTask.Use.IntoRef()
				require.NoError(t, err, "Failed to parse task use reference")
				assert.True(t, useRef.Component.IsTool(), "Expected tool reference")
				assert.True(t, useRef.Type.IsFile(), "Expected file reference")
			})

			t.Run("Second task (translate_quote)", func(t *testing.T) {
				translateTask := &workflow.Tasks[1]
				assert.Equal(t, "translate_quote", translateTask.ID, "Task ID not set correctly")
				assert.Equal(t, "basic", string(translateTask.Type), "Task type not set correctly")
				assert.Equal(t, tasksPath, translateTask.GetCWD().PathStr(), "Task CWD not set correctly")

				// Verify use reference
				translateUseRef, err := translateTask.Use.IntoRef()
				require.NoError(t, err, "Failed to parse task use reference")
				assert.True(t, translateUseRef.Component.IsAgent(), "Expected agent reference")
				assert.True(t, translateUseRef.Type.IsFile(), "Expected file reference")
				assert.Equal(t, "translate", translateTask.Action, "Task action not set correctly")
			})

			t.Run("Third task (save_results)", func(t *testing.T) {
				saveTask := &workflow.Tasks[2]
				assert.Equal(t, "save_results", saveTask.ID, "Task ID not set correctly")
				assert.Equal(t, "basic", string(saveTask.Type), "Task type not set correctly")

				// Verify use reference
				saveUseRef, err := saveTask.Use.IntoRef()
				require.NoError(t, err, "Failed to parse task use reference")
				assert.True(t, saveUseRef.Component.IsTool(), "Expected tool reference")
				assert.True(t, saveUseRef.Type.IsFile(), "Expected file reference")
				assert.True(t, saveTask.Final == "true", "Expected task to be final")
			})
		})

		t.Run("Should load tools referenced in tasks", func(t *testing.T) {
			// Verify that tools are loaded and added to workflow config via loadToolsRefOnTask
			require.GreaterOrEqual(t, len(workflow.Tools), 2, "Expected at least 2 tools to be loaded")

			t.Run("get_quote tool from first task", func(t *testing.T) {
				// Find the get_quote tool
				var getQuoteTool *tool.ToolConfig
				for i := range workflow.Tools {
					tool := &workflow.Tools[i]
					if tool.ID == "get_quote" {
						getQuoteTool = tool
						break
					}
				}

				require.NotNil(t, getQuoteTool, "get_quote tool not found in workflow config")
				assert.Equal(t, "get_quote", getQuoteTool.ID, "Tool ID not set correctly")
				assert.Equal(t, "Get a random Game of Thrones quote", getQuoteTool.Description, "Tool description not set correctly")
				assert.Equal(t, "./get_quote.ts", getQuoteTool.Execute, "Tool execute command not set correctly")

				// Verify tool CWD
				toolsPath, err := filepath.Abs("../../examples/quotes/tools")
				require.NoError(t, err, "Failed to get tools path")
				assert.Equal(t, toolsPath, getQuoteTool.GetCWD().PathStr(), "Tool CWD not set correctly")
			})

			t.Run("save_data tool from third task", func(t *testing.T) {
				// Find the save_data tool
				var saveDataTool *tool.ToolConfig
				for i := range workflow.Tools {
					tool := &workflow.Tools[i]
					if tool.ID == "save_data" {
						saveDataTool = tool
						break
					}
				}

				require.NotNil(t, saveDataTool, "save_data tool not found in workflow config")
				assert.Equal(t, "save_data", saveDataTool.ID, "Tool ID not set correctly")
				assert.Equal(t, "Save data to a file", saveDataTool.Description, "Tool description not set correctly")
				assert.Equal(t, "./save_data.ts", saveDataTool.Execute, "Tool execute command not set correctly")

				// Verify the save_data tool has an input schema for message
				require.NotNil(t, saveDataTool.InputSchema, "Tool input schema is nil")

				// Verify the save_data tool has output schema
				require.NotNil(t, saveDataTool.OutputSchema, "Tool output schema is nil")

				// Verify no duplicate tools were added
				toolCount := 0
				for _, tool := range workflow.Tools {
					if tool.ID == "save_data" {
						toolCount++
					}
				}
				assert.Equal(t, 1, toolCount, "Tool was added multiple times")
			})
		})

		t.Run("Should load agents referenced in tasks", func(t *testing.T) {
			// Verify that agents are loaded and added to workflow config via loadAgentsRefOnTask
			require.GreaterOrEqual(t, len(workflow.Agents), 1, "Expected at least 1 agent to be loaded")

			t.Run("translator agent from second task", func(t *testing.T) {
				// Find the translator agent
				var translatorAgent *agent.AgentConfig
				for i := range workflow.Agents {
					agent := &workflow.Agents[i]
					if agent.ID == "translator" {
						translatorAgent = agent
						break
					}
				}

				require.NotNil(t, translatorAgent, "translator agent not found in workflow config")
				assert.Equal(t, "translator", translatorAgent.ID, "Agent ID not set correctly")
				assert.Equal(t, "groq", string(translatorAgent.Config.Provider), "Agent provider not set correctly")
				assert.Equal(t, "llama-3.3-70b-versatile", string(translatorAgent.Config.Model), "Agent model not set correctly")

				// Verify agent has the correct action
				require.GreaterOrEqual(t, len(translatorAgent.Actions), 1, "Expected at least 1 action in translator agent")
				assert.Equal(t, "translate", translatorAgent.Actions[0].ID, "Agent action ID not set correctly")

				// Verify agent CWD
				agentsPath, err := filepath.Abs("../../examples/quotes/agents")
				require.NoError(t, err, "Failed to get agents path")
				assert.Equal(t, agentsPath, translatorAgent.GetCWD().PathStr(), "Agent CWD not set correctly")

				// Verify no duplicate agents were added
				agentCount := 0
				for _, agent := range workflow.Agents {
					if agent.ID == "translator" {
						agentCount++
					}
				}
				assert.Equal(t, 1, agentCount, "Agent was added multiple times")
			})
		})

		t.Run("Should correctly handle file references through all loader functions", func(t *testing.T) {
			// This test verifies that the full loading process (LoadTasksRef, LoadAgentsRef, LoadToolsRef) works correctly

			// Check that both task-referenced and directly-referenced tools and agents are loaded

			// Verify that tasks correctly reference their tools and agents
			firstTask := &workflow.Tasks[0]
			useRef, err := firstTask.Use.IntoRef()
			require.NoError(t, err, "Failed to parse task use reference")
			assert.True(t, useRef.Component.IsTool(), "Expected tool reference")

			// Verify that the referenced tool was loaded
			toolFound := false
			for _, tool := range workflow.Tools {
				if tool.ID == "get_quote" {
					toolFound = true
					break
				}
			}
			assert.True(t, toolFound, "Referenced tool was not loaded")

			// Check that the second task correctly references the translator agent
			secondTask := &workflow.Tasks[1]
			agentRef, err := secondTask.Use.IntoRef()
			require.NoError(t, err, "Failed to parse task use reference")
			assert.True(t, agentRef.Component.IsAgent(), "Expected agent reference")

			// Verify that the referenced agent was loaded
			agentFound := false
			for _, agent := range workflow.Agents {
				if agent.ID == "translator" {
					agentFound = true
					break
				}
			}
			assert.True(t, agentFound, "Referenced agent was not loaded")
		})
	})

}
