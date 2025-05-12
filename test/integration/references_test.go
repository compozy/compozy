package integration

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/project"
	"github.com/compozy/compozy/internal/parser/task"
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

	toolsPath, err := filepath.Abs("../../examples/quotes/tools")
	require.NoError(t, err, "Failed to get tools path")

	t.Run("Should set project CWD correctly", func(t *testing.T) {
		assert.Equal(t, expectedProjectCWD, projectConfig.GetCWD().PathStr(), "Project CWD not set correctly")
	})

	t.Run("Should load workflows from sources", func(t *testing.T) {
		workflows, err := projectConfig.WorkflowsFromSources()
		workflow := workflows[0]
		require.NoError(t, err, "Failed to load workflows")
		require.Len(t, workflows, 1, "Expected one workflow")

		t.Run("Should set workflow CWD correctly", func(t *testing.T) {
			assert.Equal(t, expectedProjectCWD, workflow.GetCWD().PathStr(), "Workflow CWD not set correctly")
		})

		t.Run("Should load tasks correctly", func(t *testing.T) {
			require.Len(t, workflow.Tasks, 3, "Expected three tasks")

			// Get the 'get_quote' task and verify its CWD
			getQuoteTask := &workflow.Tasks[0]
			require.NotNil(t, getQuoteTask.Use, "Task use is nil")

			t.Run("Should set task CWD to task file directory", func(t *testing.T) {
				taskCWD := getQuoteTask.GetCWD()
				expectedTaskCWD, err := filepath.EvalSymlinks(tasksPath)
				require.NoError(t, err, "Failed to resolve expected task CWD")

				actualTaskCWD, err := filepath.EvalSymlinks(taskCWD.PathStr())
				require.NoError(t, err, "Failed to resolve task CWD")
				assert.Equal(t, expectedTaskCWD, actualTaskCWD, "Task CWD not set correctly")
			})
		})
	})

	t.Run("Should handle direct loading of task with relative tool reference", func(t *testing.T) {
		getQuoteTaskPath := filepath.Join(tasksPath, "get_quote.yaml")

		// Create a CWD for the tasks directory
		tasksCWD, err := common.CWDFromPath(tasksPath)
		require.NoError(t, err, "Failed to create CWD for task")

		// Load the task with its correct CWD
		taskConfig, err := task.Load(tasksCWD, getQuoteTaskPath)
		require.NoError(t, err, "Failed to load task directly")

		t.Run("Should have a valid file reference to tool", func(t *testing.T) {
			require.NotNil(t, taskConfig.Use, "Task use reference is nil")
			ref, err := taskConfig.Use.IntoRef()
			require.NoError(t, err, "Failed to convert use reference")
			assert.Equal(t, "file", ref.Type.Type, "Expected file reference type")
			assert.Equal(t, "../tools/get_quote.yaml", ref.Type.Value, "Incorrect tool reference path")
		})
	})

	t.Run("Should load and validate tool directly", func(t *testing.T) {
		getQuoteToolPath := filepath.Join(toolsPath, "get_quote.yaml")

		// Create a CWD for the tools directory
		toolsCWD, err := common.CWDFromPath(toolsPath)
		require.NoError(t, err, "Failed to create CWD for tool")

		// Load the tool with its correct CWD
		toolConfig, err := tool.Load(toolsCWD, getQuoteToolPath)
		require.NoError(t, err, "Failed to load tool directly")

		t.Run("Should set tool CWD correctly", func(t *testing.T) {
			toolCWD := toolConfig.GetCWD()
			expectedToolCWD, err := filepath.EvalSymlinks(toolsPath)
			require.NoError(t, err, "Failed to resolve expected tool CWD")

			actualToolCWD, err := filepath.EvalSymlinks(toolCWD.PathStr())
			require.NoError(t, err, "Failed to resolve actual tool CWD")
			assert.Equal(t, expectedToolCWD, actualToolCWD, "Tool CWD not set correctly")
		})
	})

	t.Run("Should resolve relative paths with .. correctly", func(t *testing.T) {
		// Test that we can handle a path with ".." correctly
		// This simulates loading a tool reference from a task like "../tools/get_quote.yaml"
		relativePath := filepath.Join(tasksPath, "../tools/get_quote.yaml")
		resolvedPath, err := filepath.Abs(relativePath)
		require.NoError(t, err, "Failed to resolve relative path")

		getQuoteToolPath := filepath.Join(toolsPath, "get_quote.yaml")

		// The resolved path should match the direct path to the tool
		assert.Equal(t, getQuoteToolPath, resolvedPath, "Relative path resolution incorrect")
	})
}
