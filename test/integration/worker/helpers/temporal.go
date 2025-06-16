package helpers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

// TemporalHelper provides Temporal test environment setup for integration tests
type TemporalHelper struct {
	suite     *testsuite.WorkflowTestSuite
	env       *testsuite.TestWorkflowEnvironment
	taskQueue string
}

// NewTemporalHelper creates a new Temporal test helper
func NewTemporalHelper(_ *testing.T, suite *testsuite.WorkflowTestSuite, taskQueue string) *TemporalHelper {
	env := suite.NewTestWorkflowEnvironment()

	// Set default workflow execution timeout
	env.SetWorkflowRunTimeout(30 * time.Second)

	return &TemporalHelper{
		suite:     suite,
		env:       env,
		taskQueue: taskQueue,
	}
}

// GetEnvironment returns the test workflow environment
func (h *TemporalHelper) GetEnvironment() *testsuite.TestWorkflowEnvironment {
	return h.env
}

// RegisterWorkflow registers a workflow function for testing
func (h *TemporalHelper) RegisterWorkflow(workflow any) {
	h.env.RegisterWorkflow(workflow)
}

// RegisterActivity registers an activity function for testing
func (h *TemporalHelper) RegisterActivity(activity any) {
	h.env.RegisterActivity(activity)
}

// ExecuteWorkflow executes a workflow and returns the result
func (h *TemporalHelper) ExecuteWorkflow(workflowFunc any, args ...any) (any, error) {
	h.env.ExecuteWorkflow(workflowFunc, args...)

	if h.env.IsWorkflowCompleted() {
		var result any
		err := h.env.GetWorkflowResult(&result)
		return result, err
	}

	return nil, h.env.GetWorkflowError()
}

// ExecuteWorkflowSync executes a workflow synchronously and waits for completion
func (h *TemporalHelper) ExecuteWorkflowSync(workflowFunc any, args ...any) {
	h.env.ExecuteWorkflow(workflowFunc, args...)
}

// StartWorkflowAsync starts a workflow asynchronously without blocking
func (h *TemporalHelper) StartWorkflowAsync(workflowFunc any, args ...any) {
	go h.env.ExecuteWorkflow(workflowFunc, args...)
}

// SignalWorkflow sends a signal to the running workflow
func (h *TemporalHelper) SignalWorkflow(signalName string, arg any) {
	h.env.SignalWorkflow(signalName, arg)
}

// QueryWorkflow queries the running workflow
func (h *TemporalHelper) QueryWorkflow(queryType string, args ...any) (any, error) {
	value, err := h.env.QueryWorkflow(queryType, args...)
	if err != nil {
		return nil, err
	}

	var result any
	err = value.Get(&result)
	return result, err
}

// IsWorkflowCompleted checks if the workflow has completed
func (h *TemporalHelper) IsWorkflowCompleted() bool {
	return h.env.IsWorkflowCompleted()
}

// GetWorkflowResult gets the workflow execution result
func (h *TemporalHelper) GetWorkflowResult(valuePtr any) error {
	return h.env.GetWorkflowResult(valuePtr)
}

// GetWorkflowError gets any workflow execution error
func (h *TemporalHelper) GetWorkflowError() error {
	return h.env.GetWorkflowError()
}

// AssertExpectations asserts that all expected calls were made
func (h *TemporalHelper) AssertExpectations(t *testing.T) {
	h.env.AssertExpectations(t)
}

// OnActivity sets up a mock expectation for an activity
func (h *TemporalHelper) OnActivity(activity any, args ...any) *testsuite.MockCallWrapper {
	return h.env.OnActivity(activity, args...)
}

// OnWorkflow sets up a mock expectation for a child workflow
func (h *TemporalHelper) OnWorkflow(workflow any, args ...any) *testsuite.MockCallWrapper {
	return h.env.OnWorkflow(workflow, args...)
}

// SetTestTimeout sets a timeout for the test
func (h *TemporalHelper) SetTestTimeout(timeout time.Duration) {
	h.env.SetTestTimeout(timeout)
}

// SetWorkflowRunTimeout sets the workflow run timeout
func (h *TemporalHelper) SetWorkflowRunTimeout(timeout time.Duration) {
	h.env.SetWorkflowRunTimeout(timeout)
}

// RegisterDelayedCallback registers a callback to be called after a delay
func (h *TemporalHelper) RegisterDelayedCallback(callback func(), delay time.Duration) {
	h.env.RegisterDelayedCallback(callback, delay)
}

// Cleanup cleans up test environment resources
func (h *TemporalHelper) Cleanup(t *testing.T) {
	// The test environment doesn't require explicit cleanup
	// but we can add any custom cleanup logic here if needed
	t.Logf("Temporal test environment cleanup completed")
}

// WorkflowTestContext provides context for workflow testing
type WorkflowTestContext struct {
	t         *testing.T
	helper    *TemporalHelper
	startTime time.Time
}

// NewWorkflowTestContext creates a new workflow test context
func NewWorkflowTestContext(t *testing.T, helper *TemporalHelper) *WorkflowTestContext {
	return &WorkflowTestContext{
		t:         t,
		helper:    helper,
		startTime: time.Now(),
	}
}

// ExecuteAndVerifyWorkflow executes a workflow and verifies completion
func (ctx *WorkflowTestContext) ExecuteAndVerifyWorkflow(
	workflowFunc any,
	expectedResult any,
	args ...any,
) {
	result, err := ctx.helper.ExecuteWorkflow(workflowFunc, args...)
	require.NoError(ctx.t, err, "Workflow execution failed")
	require.Equal(ctx.t, expectedResult, result, "Workflow result mismatch")

	duration := time.Since(ctx.startTime)
	ctx.t.Logf("Workflow completed successfully in %v", duration)
}

// ExecuteAndExpectError executes a workflow and expects an error
func (ctx *WorkflowTestContext) ExecuteAndExpectError(
	workflowFunc any,
	expectedError string,
	args ...any,
) {
	_, err := ctx.helper.ExecuteWorkflow(workflowFunc, args...)
	require.Error(ctx.t, err, "Expected workflow to fail")
	require.Contains(ctx.t, err.Error(), expectedError, "Unexpected error message")
}
