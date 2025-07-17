package testing_test

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/compozy/cli/api"
	clitesting "github.com/compozy/compozy/cli/testing"
	"github.com/compozy/compozy/cli/tui/models"
	"github.com/compozy/compozy/engine/core"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Example of testing a command with mocked services
func TestExampleCommand(t *testing.T) {
	t.Run("Should execute workflow with JSON output", func(t *testing.T) {
		// Create mock API client
		mockClient := clitesting.NewMockAPIClient()

		// Set up expectations
		expectedID := core.ID("test-workflow-id")
		expectedInput := api.ExecutionInput{
			Data: map[string]any{"test": true},
		}
		expectedResult := &api.ExecutionResult{
			ExecutionID: core.ID("exec-123"),
			Status:      api.ExecutionStatusRunning,
			Message:     "Workflow started",
		}

		mockClient.WorkflowMutateMock.On("Execute", mock.Anything, expectedID, expectedInput).
			Return(expectedResult, nil)

		// Create test command
		cmd := &cobra.Command{
			Use: "test",
			RunE: func(cmd *cobra.Command, _ []string) error {
				// This would be your actual command logic
				result, err := mockClient.WorkflowMutate().Execute(
					cmd.Context(),
					expectedID,
					expectedInput,
				)
				if err != nil {
					return err
				}

				// Output result
				_, err = cmd.OutOrStdout().Write([]byte(result.Message))
				return err
			},
		}

		// Create command test helper
		ct := clitesting.NewCommandTest(t, cmd)

		// Execute command
		err := ct.Execute()
		require.NoError(t, err)

		// Verify output
		assert.Contains(t, ct.Stdout(), "Workflow started")
		assert.Empty(t, ct.Stderr())

		// Verify mock expectations
		mockClient.WorkflowMutateMock.AssertExpectations(t)
	})

	t.Run("Should handle errors appropriately", func(t *testing.T) {
		// Create mock API client
		mockClient := clitesting.NewMockAPIClient()

		// Set up error expectation
		expectedErr := errors.New("workflow not found")
		mockClient.WorkflowMutateMock.On("Execute", mock.Anything, mock.Anything, mock.Anything).
			Return(nil, expectedErr)

		// Create test command
		cmd := &cobra.Command{
			Use: "test",
			RunE: func(cmd *cobra.Command, _ []string) error {
				_, err := mockClient.WorkflowMutate().Execute(
					cmd.Context(),
					core.ID("test"),
					api.ExecutionInput{},
				)
				return err
			},
		}

		// Create command test helper
		ct := clitesting.NewCommandTest(t, cmd)

		// Execute command
		err := ct.Execute()
		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
	})
}

// Example of testing output modes
func TestOutputModes(t *testing.T) {
	tests := clitesting.OutputModeTests()

	for _, tt := range tests {
		t.Run("Should handle "+string(tt.Mode)+" mode", func(t *testing.T) {
			// Create test command
			cmd := clitesting.TestCommand()

			// Set output flags
			err := clitesting.SetOutputFlags(cmd, tt)
			require.NoError(t, err)

			// Add command logic
			cmd.RunE = func(cmd *cobra.Command, _ []string) error {
				// Detect mode
				jsonFlag, _ := cmd.Flags().GetBool("json")
				tuiFlag, _ := cmd.Flags().GetBool("tui")
				outputFlag, _ := cmd.Flags().GetString("output")

				var mode models.Mode
				if jsonFlag || outputFlag == "json" {
					mode = models.ModeJSON
				} else if tuiFlag || outputFlag == "tui" {
					mode = models.ModeTUI
				}

				// Output based on mode
				switch mode {
				case models.ModeJSON:
					_, err := cmd.OutOrStdout().Write([]byte(`{"result": "success"}`))
					return err
				case models.ModeTUI:
					_, err := cmd.OutOrStdout().Write([]byte("✓ Success"))
					return err
				default:
					return errors.New("unknown mode")
				}
			}

			// Create command test helper
			ct := clitesting.NewCommandTest(t, cmd)

			// Execute command
			err = ct.Execute()
			require.NoError(t, err)

			// Verify output based on mode
			output := ct.Stdout()
			switch tt.Mode {
			case models.ModeJSON:
				clitesting.AssertJSONOutput(t, output)
			case models.ModeTUI:
				clitesting.AssertTUIOutput(t, output, "✓")
			}
		})
	}
}

// Example of testing with test data generators
func TestWithGeneratedData(t *testing.T) {
	t.Run("Should work with generated workflow data", func(t *testing.T) {
		// Generate test data
		workflow := clitesting.TestWorkflow("wf-123", "Test Workflow")
		workflowDetail := clitesting.TestWorkflowDetail("wf-123", "Test Workflow")
		execution := clitesting.TestExecution("exec-123", "wf-123")
		executionDetail := clitesting.TestExecutionDetail("exec-123", "wf-123")
		schedule := clitesting.TestSchedule("wf-123")

		// Use test data in assertions
		assert.Equal(t, core.ID("wf-123"), workflow.ID)
		assert.Equal(t, "Test Workflow", workflow.Name)
		assert.Equal(t, api.WorkflowStatusActive, workflow.Status)

		assert.NotNil(t, workflowDetail.Statistics)
		assert.Equal(t, int64(10), workflowDetail.Statistics.TotalExecutions)

		assert.Equal(t, api.ExecutionStatusRunning, execution.Status)
		assert.NotNil(t, execution.Input)

		assert.NotNil(t, executionDetail.Metrics)
		assert.Equal(t, 1, executionDetail.Metrics.CompletedTasks)

		assert.Equal(t, "0 * * * *", schedule.CronExpr)
		assert.True(t, schedule.Enabled)
	})
}

// Example of testing error formatting
func TestErrorFormatting(t *testing.T) {
	t.Run("Should format errors based on output mode", func(t *testing.T) {
		// Create mock client that returns an error
		mockClient := clitesting.NewMockAPIClient()
		mockErr := errors.New("API_ERROR: authentication failed")

		mockClient.WorkflowMock.On("List", mock.Anything, mock.Anything).
			Return(nil, mockErr)

		// Test JSON error formatting
		cmd := clitesting.TestCommand()
		err := cmd.Flags().Set("json", "true")
		require.NoError(t, err)

		cmd.RunE = func(cmd *cobra.Command, _ []string) error {
			_, err := mockClient.Workflow().List(cmd.Context(), &api.WorkflowFilters{})
			return err
		}

		ct := clitesting.NewCommandTest(t, cmd)
		err = ct.Execute()
		assert.Error(t, err)
	})
}

// Example of testing with context and timeout
func TestWithContext(t *testing.T) {
	t.Run("Should respect context cancellation", func(t *testing.T) {
		// Create a context that will be canceled
		ctx, cancel := context.WithCancel(context.Background())

		// Create mock client
		mockClient := clitesting.NewMockAPIClient()

		// Set up mock to return cancellation error
		mockClient.ExecutionMock.On("Follow", mock.Anything, mock.Anything).
			Return(nil, context.Canceled)

		// Create command
		cmd := &cobra.Command{
			Use: "test",
			RunE: func(cmd *cobra.Command, _ []string) error {
				_, err := mockClient.Execution().Follow(
					cmd.Context(),
					core.ID("exec-123"),
				)
				return err
			},
		}
		cmd.SetContext(ctx)

		// Cancel context after a short delay
		go func() {
			cancel()
		}()

		// Execute command
		ct := clitesting.NewCommandTest(t, cmd)
		err := ct.Execute()

		// Verify context cancellation
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})
}
