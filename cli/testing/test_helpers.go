package testing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/cli/api"
	"github.com/compozy/compozy/cli/tui/models"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/mock"
)

// TestContext creates a test context with logger
func TestContext(t *testing.T) context.Context {
	t.Helper()
	log := logger.NewForTests()
	return logger.ContextWithLogger(t.Context(), log)
}

// TestCommand creates a test cobra command with common flags
func TestCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test command",
	}
	// Add common flags
	cmd.Flags().Bool("json", false, "JSON output")
	cmd.Flags().Bool("tui", false, "TUI output")
	cmd.Flags().String("output", "", "Output format")
	cmd.Flags().String("format", "", "Format (deprecated)")
	return cmd
}

// TestConfig creates a test configuration
func TestConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 5001,
		},
		CLI: config.CLIConfig{
			APIKey:  "test-api-key",
			Timeout: 30,
		},
		Runtime: config.RuntimeConfig{
			LogLevel: "debug",
		},
		RateLimit: config.RateLimitConfig{
			GlobalRate: config.RateConfig{
				Limit: 100,
			},
		},
	}
}

// CaptureOutput captures stdout and stderr during test execution
type CaptureOutput struct {
	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

// NewCaptureOutput creates a new output capturer
func NewCaptureOutput() *CaptureOutput {
	return &CaptureOutput{
		stdout: new(bytes.Buffer),
		stderr: new(bytes.Buffer),
	}
}

// Stdout returns captured stdout
func (c *CaptureOutput) Stdout() string {
	return c.stdout.String()
}

// Stderr returns captured stderr
func (c *CaptureOutput) Stderr() string {
	return c.stderr.String()
}

// StdoutWriter returns the stdout writer
func (c *CaptureOutput) StdoutWriter() io.Writer {
	return c.stdout
}

// StderrWriter returns the stderr writer
func (c *CaptureOutput) StderrWriter() io.Writer {
	return c.stderr
}

// Reset clears captured output
func (c *CaptureOutput) Reset() {
	c.stdout.Reset()
	c.stderr.Reset()
}

// CommandTest helps test cobra commands
type CommandTest struct {
	t       *testing.T
	cmd     *cobra.Command
	capture *CaptureOutput
	ctx     context.Context
}

// NewCommandTest creates a new command test helper
func NewCommandTest(t *testing.T, cmd *cobra.Command) *CommandTest {
	t.Helper()
	return &CommandTest{
		t:       t,
		cmd:     cmd,
		capture: NewCaptureOutput(),
		ctx:     TestContext(t),
	}
}

// Execute runs the command with given arguments
func (ct *CommandTest) Execute(args ...string) error {
	ct.cmd.SetArgs(args)
	ct.cmd.SetOut(ct.capture.StdoutWriter())
	ct.cmd.SetErr(ct.capture.StderrWriter())
	ct.cmd.SetContext(ct.ctx)
	return ct.cmd.Execute()
}

// ExecuteWithInput runs the command with given input and arguments
func (ct *CommandTest) ExecuteWithInput(input string, args ...string) error {
	ct.cmd.SetIn(bytes.NewBufferString(input))
	return ct.Execute(args...)
}

// Stdout returns captured stdout
func (ct *CommandTest) Stdout() string {
	return ct.capture.Stdout()
}

// Stderr returns captured stderr
func (ct *CommandTest) Stderr() string {
	return ct.capture.Stderr()
}

// Reset clears the captured output
func (ct *CommandTest) Reset() {
	ct.capture.Reset()
}

// AssertSuccess asserts the command executed successfully
func (ct *CommandTest) AssertSuccess() {
	ct.t.Helper()
	if ct.capture.Stderr() != "" {
		ct.t.Errorf("expected no stderr output, got: %s", ct.capture.Stderr())
	}
}

// AssertError asserts the command failed with error output
func (ct *CommandTest) AssertError(expectedError string) {
	ct.t.Helper()
	stderr := ct.capture.Stderr()
	if stderr == "" {
		ct.t.Error("expected error output, got none")
		return
	}
	if expectedError != "" && !bytes.Contains([]byte(stderr), []byte(expectedError)) {
		ct.t.Errorf("expected error containing %q, got: %s", expectedError, stderr)
	}
}

// TestWorkflow creates a test workflow
func TestWorkflow(id string, name string) api.Workflow {
	return api.Workflow{
		ID:          core.ID(id),
		Name:        name,
		Description: "Test workflow",
		Status:      api.WorkflowStatusActive,
		CreatedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Tags:        []string{"test"},
		Metadata:    map[string]string{"test": "true"},
	}
}

// TestWorkflowDetail creates a detailed test workflow
func TestWorkflowDetail(id string, name string) api.WorkflowDetail {
	return api.WorkflowDetail{
		Workflow: TestWorkflow(id, name),
		Tasks: []api.Task{
			{
				ID:          core.ID("task-1"),
				Name:        "Test Task",
				Type:        "basic",
				Description: "A test task",
			},
		},
		Inputs: []api.InputSchema{
			{
				Name:        "input1",
				Type:        "string",
				Required:    true,
				Description: "Test input",
			},
		},
		Outputs: []api.OutputSchema{
			{
				Name:        "output1",
				Type:        "string",
				Description: "Test output",
			},
		},
		Statistics: &api.WorkflowStats{
			TotalExecutions:      10,
			SuccessfulExecutions: 8,
			FailedExecutions:     2,
			AverageExecutionTime: 5 * time.Minute,
		},
	}
}

// TestExecution creates a test execution
func TestExecution(id string, workflowID string) api.Execution {
	return api.Execution{
		ID:         core.ID(id),
		WorkflowID: core.ID(workflowID),
		Status:     api.ExecutionStatusRunning,
		StartedAt:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Input: map[string]any{
			"test": true,
		},
	}
}

// TestExecutionDetail creates a detailed test execution
func TestExecutionDetail(id string, workflowID string) api.ExecutionDetail {
	completedAt := time.Date(2024, 1, 1, 0, 1, 0, 0, time.UTC)
	return api.ExecutionDetail{
		Execution: TestExecution(id, workflowID),
		Logs: []api.LogEntry{
			{
				Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				Level:     "info",
				Message:   "Task started",
			},
		},
		TaskResults: []api.TaskResult{
			{
				TaskID:      core.ID("task-1"),
				Status:      "completed",
				Output:      map[string]any{"result": "success"},
				StartedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				CompletedAt: &completedAt,
			},
		},
		Metrics: &api.ExecutionMetrics{
			TotalTasks:     1,
			CompletedTasks: 1,
			FailedTasks:    0,
			ExecutionTime:  1 * time.Minute,
		},
	}
}

// TestSchedule creates a test schedule
func TestSchedule(workflowID string) api.Schedule {
	lastRun := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	return api.Schedule{
		WorkflowID: core.ID(workflowID),
		CronExpr:   "0 * * * *",
		Enabled:    true,
		NextRun:    time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC),
		LastRun:    &lastRun,
		Timezone:   "UTC",
	}
}

// OutputModeTest helps test different output modes
type OutputModeTest struct {
	Mode     models.Mode
	JSONFlag bool
	TUIFlag  bool
	Output   string
}

// OutputModeTests returns common output mode test cases
func OutputModeTests() []OutputModeTest {
	return []OutputModeTest{
		{Mode: models.ModeJSON, JSONFlag: true, Output: "json"},
		{Mode: models.ModeTUI, TUIFlag: true, Output: "tui"},
		{Mode: models.ModeJSON, Output: "json"},
		{Mode: models.ModeTUI, Output: "tui"},
	}
}

// SetOutputFlags sets output flags on a command for testing
func SetOutputFlags(cmd *cobra.Command, test OutputModeTest) error {
	if test.JSONFlag {
		if err := cmd.Flags().Set("json", "true"); err != nil {
			return err
		}
	}
	if test.TUIFlag {
		if err := cmd.Flags().Set("tui", "true"); err != nil {
			return err
		}
	}
	if test.Output != "" {
		if err := cmd.Flags().Set("output", test.Output); err != nil {
			return err
		}
	}
	return nil
}

// MockHandlerFunc creates a mock handler function for testing
func MockHandlerFunc(expectedErr error, action func()) func(context.Context, *cobra.Command, any, []string) error {
	return func(_ context.Context, _ *cobra.Command, _ any, _ []string) error {
		if action != nil {
			action()
		}
		return expectedErr
	}
}

// AssertJSONOutput asserts that output is valid JSON
func AssertJSONOutput(t *testing.T, output string) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("expected valid JSON output, got error: %v\nOutput: %s", err, output)
	}
	return result
}

// AssertTUIOutput asserts that output contains expected TUI elements
func AssertTUIOutput(t *testing.T, output string, elements ...string) {
	t.Helper()
	for _, element := range elements {
		if !strings.Contains(output, element) {
			t.Errorf("expected output to contain %q, got: %s", element, output)
		}
	}
}

// AssertErrorOutput asserts error output format based on mode
func AssertErrorOutput(t *testing.T, output string, mode models.Mode, errorCode string) {
	t.Helper()
	switch mode {
	case models.ModeJSON:
		data := AssertJSONOutput(t, output)
		if errData, ok := data["error"].(map[string]any); ok {
			if code, ok := errData["code"].(string); ok && code != errorCode {
				t.Errorf("expected error code %q, got %q", errorCode, code)
			}
		} else {
			t.Errorf("expected JSON error structure, got: %+v", data)
		}
	case models.ModeTUI:
		if !strings.Contains(output, "❌") && !strings.Contains(output, "Error") {
			t.Errorf("expected TUI error output, got: %s", output)
		}
	}
}

// MockAPIClient creates a mock API client for testing
type MockAPIClient struct {
	mock.Mock

	// Service mocks
	WorkflowMock        *MockWorkflowService
	WorkflowMutateMock  *MockWorkflowMutateService
	ExecutionMock       *MockExecutionService
	ExecutionMutateMock *MockExecutionMutateService
	ScheduleMock        *MockScheduleService
	ScheduleMutateMock  *MockScheduleMutateService
	EventMock           *MockEventService
}

// Mock service implementations
type MockWorkflowService struct {
	mock.Mock
}

func (m *MockWorkflowService) List(
	ctx context.Context,
	filters *api.WorkflowFilters,
) ([]api.Workflow, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]api.Workflow), args.Error(1) //nolint:errcheck // type assertion safe after nil check
}

func (m *MockWorkflowService) Get(ctx context.Context, id core.ID) (*api.WorkflowDetail, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	result, ok := args.Get(0).(*api.WorkflowDetail)
	if !ok {
		return nil, fmt.Errorf("expected *api.WorkflowDetail, got %T", args.Get(0))
	}
	return result, args.Error(1)
}

type MockWorkflowMutateService struct {
	mock.Mock
}

func (m *MockWorkflowMutateService) Execute(
	ctx context.Context,
	id core.ID,
	input api.ExecutionInput,
) (*api.ExecutionResult, error) {
	args := m.Called(ctx, id, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	result, ok := args.Get(0).(*api.ExecutionResult)
	if !ok {
		return nil, fmt.Errorf("expected *api.ExecutionResult, got %T", args.Get(0))
	}
	return result, args.Error(1)
}

type MockExecutionService struct {
	mock.Mock
}

func (m *MockExecutionService) List(
	ctx context.Context,
	filters api.ExecutionFilters,
) ([]api.Execution, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]api.Execution), args.Error(1) //nolint:errcheck // type assertion safe after nil check
}

func (m *MockExecutionService) Get(ctx context.Context, id core.ID) (*api.ExecutionDetail, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	result, ok := args.Get(0).(*api.ExecutionDetail)
	if !ok {
		return nil, fmt.Errorf("expected *api.ExecutionDetail, got %T", args.Get(0))
	}
	return result, args.Error(1)
}

func (m *MockExecutionService) Follow(ctx context.Context, id core.ID) (<-chan api.ExecutionEvent, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	result, ok := args.Get(0).(<-chan api.ExecutionEvent)
	if !ok {
		return nil, fmt.Errorf("expected <-chan api.ExecutionEvent, got %T", args.Get(0))
	}
	return result, args.Error(1)
}

type MockExecutionMutateService struct {
	mock.Mock
}

func (m *MockExecutionMutateService) Signal(ctx context.Context, execID core.ID, signal string, payload any) error {
	args := m.Called(ctx, execID, signal, payload)
	return args.Error(0)
}

func (m *MockExecutionMutateService) Cancel(ctx context.Context, execID core.ID) error {
	args := m.Called(ctx, execID)
	return args.Error(0)
}

type MockScheduleService struct {
	mock.Mock
}

func (m *MockScheduleService) List(ctx context.Context) ([]api.Schedule, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]api.Schedule), args.Error(1) //nolint:errcheck // type assertion safe after nil check
}

func (m *MockScheduleService) Get(ctx context.Context, workflowID core.ID) (*api.Schedule, error) {
	args := m.Called(ctx, workflowID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	result, ok := args.Get(0).(*api.Schedule)
	if !ok {
		return nil, fmt.Errorf("expected *api.Schedule, got %T", args.Get(0))
	}
	return result, args.Error(1)
}

type MockScheduleMutateService struct {
	mock.Mock
}

func (m *MockScheduleMutateService) Update(
	ctx context.Context,
	workflowID core.ID,
	req api.UpdateScheduleRequest,
) error {
	args := m.Called(ctx, workflowID, req)
	return args.Error(0)
}

func (m *MockScheduleMutateService) Delete(ctx context.Context, workflowID core.ID) error {
	args := m.Called(ctx, workflowID)
	return args.Error(0)
}

type MockEventService struct {
	mock.Mock
}

func (m *MockEventService) Send(ctx context.Context, event api.Event) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

// NewMockAPIClient creates a new mock API client
func NewMockAPIClient() *MockAPIClient {
	return &MockAPIClient{
		WorkflowMock:        new(MockWorkflowService),
		WorkflowMutateMock:  new(MockWorkflowMutateService),
		ExecutionMock:       new(MockExecutionService),
		ExecutionMutateMock: new(MockExecutionMutateService),
		ScheduleMock:        new(MockScheduleService),
		ScheduleMutateMock:  new(MockScheduleMutateService),
		EventMock:           new(MockEventService),
	}
}

// Workflow returns the mock workflow service
func (m *MockAPIClient) Workflow() api.WorkflowService {
	return m.WorkflowMock
}

// WorkflowMutate returns the mock workflow mutate service
func (m *MockAPIClient) WorkflowMutate() api.WorkflowMutateService {
	return m.WorkflowMutateMock
}

// Execution returns the mock execution service
func (m *MockAPIClient) Execution() api.ExecutionService {
	return m.ExecutionMock
}

// ExecutionMutate returns the mock execution mutate service
func (m *MockAPIClient) ExecutionMutate() api.ExecutionMutateService {
	return m.ExecutionMutateMock
}

// Schedule returns the mock schedule service
func (m *MockAPIClient) Schedule() api.ScheduleService {
	return m.ScheduleMock
}

// ScheduleMutate returns the mock schedule mutate service
func (m *MockAPIClient) ScheduleMutate() api.ScheduleMutateService {
	return m.ScheduleMutateMock
}

// Event returns the mock event service
func (m *MockAPIClient) Event() api.EventService {
	return m.EventMock
}

// TestExecutionEvent creates a test execution event
func TestExecutionEvent(execID string, eventType string) api.ExecutionEvent {
	return api.ExecutionEvent{
		ExecutionID: core.ID(execID),
		Type:        eventType,
		Message:     "Test event",
		Timestamp:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Data:        map[string]any{"test": true},
	}
}

// TestEvent creates a test event
func TestEvent(name string) api.Event {
	return api.Event{
		Name:      name,
		Payload:   map[string]any{"test": true},
		Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Source:    "test",
	}
}
