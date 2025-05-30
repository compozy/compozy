package utils

import (
	"context"
	"os"
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

	// Get the absolute path to the examples directory by finding the project root
	projectRoot, err := findProjectRoot()
	require.NoError(t, err, "Failed to find project root")

	exampleDir := filepath.Join(projectRoot, "examples", exampleName)
	cwd, err := core.CWDFromPath(exampleDir)
	require.NoError(t, err, "Failed to create CWD from examples path")

	// Load the project configuration first
	projectPath := filepath.Join(exampleDir, "compozy.yaml")
	projectConfig, err := project.Load(context.Background(), cwd, projectPath)
	require.NoError(t, err, "Failed to load workflows from project")

	// Load workflows from the project
	workflows, err := workflow.WorkflowsFromProject(context.Background(), projectConfig)
	require.NoError(t, err, "Failed to load workflows from project")
	require.Len(t, workflows, 1, "Expected exactly one workflow")

	workflowConfig := workflows[0]
	return workflowConfig, cwd
}

// LoadExampleProject loads a full project from the examples directory
func LoadExampleProject(t *testing.T, exampleName string) (*project.Config, *core.CWD) {
	t.Helper()

	// Get the absolute path to the examples directory by finding the project root
	projectRoot, err := findProjectRoot()
	require.NoError(t, err, "Failed to find project root")

	examplesDir := filepath.Join(projectRoot, "examples", exampleName)
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

// findProjectRoot finds the project root by looking for go.mod file
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		// Check if go.mod exists in current directory
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the root directory
			break
		}
		dir = parent
	}

	return "", os.ErrNotExist
}
