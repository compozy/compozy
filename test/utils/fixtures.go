package utils

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/workflow"
	pkgutils "github.com/compozy/compozy/pkg/utils"
	"github.com/stretchr/testify/require"
)

// LoadExampleWorkflow loads a workflow from the examples directory
func LoadExampleWorkflow(t *testing.T, exampleName string) (*workflow.Config, *core.CWD) {
	t.Helper()

	// Get the absolute path to the examples directory
	examplesDir, err := filepath.Abs(filepath.Join("..", "..", "..", "examples", exampleName))
	require.NoError(t, err, "Failed to get absolute path to examples directory")

	cwd, err := core.CWDFromPath(examplesDir)
	require.NoError(t, err, "Failed to create CWD from examples path")

	// Load the workflow
	workflowPath := filepath.Join(examplesDir, "workflow.yaml")
	workflowConfig, err := workflow.Load(context.Background(), cwd, examplesDir, workflowPath)
	require.NoError(t, err, "Failed to load workflow from examples")

	return workflowConfig, cwd
}

// LoadExampleProject loads a full project from the examples directory
func LoadExampleProject(t *testing.T, exampleName string) (*project.Config, *core.CWD) {
	t.Helper()

	// Get the absolute path to the examples directory
	examplesDir, err := filepath.Abs(filepath.Join("..", "..", "..", "examples", exampleName))
	require.NoError(t, err, "Failed to get absolute path to examples directory")

	cwd, err := core.CWDFromPath(examplesDir)
	require.NoError(t, err, "Failed to create CWD from examples path")

	// Load the project
	projectPath := filepath.Join(examplesDir, "compozy.yaml")
	projectConfig, err := project.Load(context.Background(), cwd, projectPath)
	require.NoError(t, err, "Failed to load project from examples")

	return projectConfig, cwd
}

// SetupExampleFixture copies an example directory to a temporary location using the existing fixture utilities
func SetupExampleFixture(t *testing.T, exampleName string) (*core.CWD, string) {
	t.Helper()

	// Use the existing SetupFixture function from pkg/utils
	examplesDir := "examples"
	examplePath := filepath.Join(examplesDir, exampleName)

	// Copy the example to a temporary directory
	tempPath := pkgutils.SetupFixture(t, examplePath)

	// Create CWD from the temporary path
	cwd, err := core.CWDFromPath(tempPath)
	require.NoError(t, err, "Failed to create CWD from temporary example path")

	return cwd, tempPath
}

// CreateTestWorkflowFromExample creates a workflow execution using an example workflow
func CreateTestWorkflowFromExample(
	t *testing.T,
	tb *IntegrationTestBed,
	exampleName string,
	input *core.Input,
) (*workflow.Config, core.ID) {
	t.Helper()

	// Load the example workflow
	workflowConfig, _ := LoadExampleWorkflow(t, exampleName)

	// Create workflow execution
	workflowExecID := CreateTestWorkflowExecution(
		t, tb, workflowConfig.ID,
		core.EnvMap{}, input,
	)

	return workflowConfig, workflowExecID
}

// GetWeatherAgentTestInput returns a valid input for the weather-agent workflow
func GetWeatherAgentTestInput() *core.Input {
	return &core.Input{
		"city": "San Francisco",
	}
}

// GetQuotesTestInput returns a valid input for the quotes workflow
func GetQuotesTestInput() *core.Input {
	return &core.Input{
		"to_language": "Spanish",
	}
}
