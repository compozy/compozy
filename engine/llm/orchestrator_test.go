package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockTool implements the tool.Tool interface for testing
type MockTool struct {
	mock.Mock
}

func (m *MockTool) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockTool) Description() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockTool) Call(ctx context.Context, input string) (string, error) {
	args := m.Called(ctx, input)
	return args.String(0), args.Error(1)
}

// MockToolRegistry implements the ToolRegistry interface for testing
type MockToolRegistry struct {
	mock.Mock
}

func (m *MockToolRegistry) Register(ctx context.Context, tool Tool) error {
	args := m.Called(ctx, tool)
	return args.Error(0)
}

func (m *MockToolRegistry) Find(ctx context.Context, name string) (Tool, bool) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Bool(1)
	}
	return args.Get(0).(Tool), args.Bool(1)
}

func (m *MockToolRegistry) ListAll(ctx context.Context) ([]Tool, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]Tool), args.Error(1)
}

func (m *MockToolRegistry) InvalidateCache(ctx context.Context) {
	m.Called(ctx)
}

func (m *MockToolRegistry) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockPromptBuilder implements the PromptBuilder interface for testing
type MockPromptBuilder struct {
	mock.Mock
}

func TestExecuteSingleToolCallRaw_JSONContentPopulation(t *testing.T) {
	t.Run("populates JSONContent when tool returns valid JSON", func(t *testing.T) {
		mockRegistry := &MockToolRegistry{}
		mockTool := &MockTool{}
		o := &llmOrchestrator{config: OrchestratorConfig{ToolRegistry: mockRegistry}}
		ctx := context.Background()
		tc := llmadapter.ToolCall{ID: "id1", Name: "json-tool", Arguments: []byte(`{"arg":1}`)}

		mockRegistry.On("Find", mock.Anything, "json-tool").Return(mockTool, true)
		mockTool.On("Call", mock.Anything, `{"arg":1}`).Return(`{"ok":true}`, nil)

		res := o.executeSingleToolCallRaw(ctx, tc)
		assert.Equal(t, "id1", res.ID)
		assert.Equal(t, "json-tool", res.Name)
		assert.Equal(t, `{"ok":true}`, res.Content)
		require.NotNil(t, res.JSONContent)
		assert.JSONEq(t, `{"ok":true}`, string(res.JSONContent))

		mockRegistry.AssertExpectations(t)
		mockTool.AssertExpectations(t)
	})

	t.Run("leaves JSONContent nil for non-JSON tool output", func(t *testing.T) {
		mockRegistry := &MockToolRegistry{}
		mockTool := &MockTool{}
		o := &llmOrchestrator{config: OrchestratorConfig{ToolRegistry: mockRegistry}}
		ctx := context.Background()
		tc := llmadapter.ToolCall{ID: "id2", Name: "text-tool", Arguments: []byte(`{"foo":"bar"}`)}

		mockRegistry.On("Find", mock.Anything, "text-tool").Return(mockTool, true)
		mockTool.On("Call", mock.Anything, `{"foo":"bar"}`).Return("plain text response", nil)

		res := o.executeSingleToolCallRaw(ctx, tc)
		assert.Equal(t, "plain text response", res.Content)
		assert.Nil(t, res.JSONContent)

		mockRegistry.AssertExpectations(t)
		mockTool.AssertExpectations(t)
	})

	t.Run("returns JSON error payload when tool not found", func(t *testing.T) {
		mockRegistry := &MockToolRegistry{}
		o := &llmOrchestrator{config: OrchestratorConfig{ToolRegistry: mockRegistry}}
		ctx := context.Background()
		tc := llmadapter.ToolCall{ID: "id3", Name: "missing", Arguments: []byte(`{}`)}

		mockRegistry.On("Find", mock.Anything, "missing").Return(nil, false)

		res := o.executeSingleToolCallRaw(ctx, tc)
		require.NotNil(t, res.JSONContent)
		// Ensure JSONContent is valid JSON and includes an error field
		var decoded map[string]any
		require.NoError(t, json.Unmarshal(res.JSONContent, &decoded))
		assert.Contains(t, decoded, "error")
		assert.NotEmpty(t, res.Content)

		mockRegistry.AssertExpectations(t)
	})

	t.Run("returns JSON error payload when tool execution fails", func(t *testing.T) {
		mockRegistry := &MockToolRegistry{}
		mockTool := &MockTool{}
		o := &llmOrchestrator{config: OrchestratorConfig{ToolRegistry: mockRegistry}}
		ctx := context.Background()
		tc := llmadapter.ToolCall{ID: "id4", Name: "err", Arguments: []byte(`{"q":true}`)}

		mockRegistry.On("Find", mock.Anything, "err").Return(mockTool, true)
		mockTool.On("Call", mock.Anything, `{"q":true}`).Return("", fmt.Errorf("boom"))

		res := o.executeSingleToolCallRaw(ctx, tc)
		require.NotNil(t, res.JSONContent)
		var decoded map[string]any
		require.NoError(t, json.Unmarshal(res.JSONContent, &decoded))
		assert.Contains(t, decoded, "error")
		assert.NotEmpty(t, res.Content)

		mockRegistry.AssertExpectations(t)
		mockTool.AssertExpectations(t)
	})
}

func TestCountSuccessfulResults_PrefersJSONContent(t *testing.T) {
	results := []llmadapter.ToolResult{
		{ID: "1", Name: "a", JSONContent: json.RawMessage(`{"ok":true}`)},     // success
		{ID: "2", Name: "b", JSONContent: json.RawMessage(`{"error":"bad"}`)}, // error
		{ID: "3", Name: "c", Content: "non-json plain text"},                  // success
		{ID: "4", Name: "d", Content: "{\"error\":\"boom\"}"},                 // error (string check)
	}
	count := countSuccessfulResults(results)
	assert.Equal(t, 2, count)
}

func (m *MockPromptBuilder) Build(ctx context.Context, action *agent.ActionConfig) (string, error) {
	args := m.Called(ctx, action)
	return args.String(0), args.Error(1)
}

func (m *MockPromptBuilder) ShouldUseStructuredOutput(
	provider string,
	action *agent.ActionConfig,
	tools []tool.Config,
) bool {
	args := m.Called(provider, action, tools)
	return args.Bool(0)
}

func (m *MockPromptBuilder) EnhanceForStructuredOutput(
	ctx context.Context,
	basePrompt string,
	outputSchema *schema.Schema,
	hasTools bool,
) string {
	args := m.Called(ctx, basePrompt, outputSchema, hasTools)
	return args.String(0)
}

func TestOrchestrator_validateInput(t *testing.T) {
	t.Run("Should validate input with schema successfully", func(t *testing.T) {
		orchestrator := &llmOrchestrator{}
		ctx := context.Background()
		inputData := core.Input(map[string]any{
			"name": "test",
			"age":  25,
		})
		inputSchema := &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
				"age":  map[string]any{"type": "number"},
			},
			"required": []string{"name", "age"},
		}
		request := Request{
			Agent: &agent.Config{
				Instructions: "test instructions",
			},
			Action: &agent.ActionConfig{
				Prompt:      "test prompt",
				With:        &inputData,
				InputSchema: inputSchema,
			},
		}
		err := orchestrator.validateInput(ctx, request)
		assert.NoError(t, err)
	})

	t.Run("Should return error for invalid input schema", func(t *testing.T) {
		orchestrator := &llmOrchestrator{}
		ctx := context.Background()
		inputData := core.Input(map[string]any{
			"name": "test",
		})
		inputSchema := &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
				"age":  map[string]any{"type": "number"},
			},
			"required": []string{"name", "age"},
		}
		request := Request{
			Agent: &agent.Config{
				Instructions: "test instructions",
			},
			Action: &agent.ActionConfig{
				Prompt:      "test prompt",
				With:        &inputData,
				InputSchema: inputSchema,
			},
		}
		err := orchestrator.validateInput(ctx, request)
		assert.ErrorContains(t, err, "input validation failed")
	})

	t.Run("Should skip validation when no input schema provided", func(t *testing.T) {
		orchestrator := &llmOrchestrator{}
		ctx := context.Background()
		request := Request{
			Agent: &agent.Config{
				Instructions: "test instructions",
			},
			Action: &agent.ActionConfig{
				Prompt: "test prompt",
			},
		}
		err := orchestrator.validateInput(ctx, request)
		assert.NoError(t, err)
	})
}

func TestOrchestrator_validateOutput(t *testing.T) {
	t.Run("Should validate output with schema successfully", func(t *testing.T) {
		orchestrator := &llmOrchestrator{}
		ctx := context.Background()
		output := core.Output(map[string]any{
			"result": "success",
			"count":  42,
		})
		outputSchema := &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"result": map[string]any{"type": "string"},
				"count":  map[string]any{"type": "number"},
			},
			"required": []string{"result", "count"},
		}
		action := &agent.ActionConfig{
			OutputSchema: outputSchema,
		}
		err := orchestrator.validateOutput(ctx, &output, action)
		assert.NoError(t, err)
	})

	t.Run("Should return error for invalid output schema", func(t *testing.T) {
		orchestrator := &llmOrchestrator{}
		ctx := context.Background()
		output := core.Output(map[string]any{
			"result": "success",
		})
		outputSchema := &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"result": map[string]any{"type": "string"},
				"count":  map[string]any{"type": "number"},
			},
			"required": []string{"result", "count"},
		}
		action := &agent.ActionConfig{
			OutputSchema: outputSchema,
		}
		err := orchestrator.validateOutput(ctx, &output, action)
		require.Error(t, err)
	})

	t.Run("Should skip validation when no output schema provided", func(t *testing.T) {
		orchestrator := &llmOrchestrator{}
		ctx := context.Background()
		output := core.Output(map[string]any{
			"result": "success",
		})
		action := &agent.ActionConfig{}
		err := orchestrator.validateOutput(ctx, &output, action)
		assert.NoError(t, err)
	})
}

func TestOrchestrator_executeToolCalls(t *testing.T) {
	t.Run("Should execute single tool call and return result directly", func(t *testing.T) {
		mockRegistry := &MockToolRegistry{}
		mockTool := &MockTool{}
		orchestrator := &llmOrchestrator{
			config: OrchestratorConfig{
				ToolRegistry: mockRegistry,
			},
		}
		ctx := context.Background()
		request := Request{
			Action: &agent.ActionConfig{},
		}
		toolCalls := []llmadapter.ToolCall{
			{
				ID:        "call1",
				Name:      "test-tool",
				Arguments: []byte(`{"arg": "value"}`),
			},
		}
		mockRegistry.On("Find", mock.Anything, "test-tool").Return(mockTool, true)
		mockTool.On("Call", mock.Anything, `{"arg": "value"}`).Return(`{"result": "success"}`, nil)
		result, err := orchestrator.executeToolCalls(ctx, toolCalls, request)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		expected := core.Output(map[string]any{"result": "success"})
		assert.Equal(t, &expected, result)
		mockRegistry.AssertExpectations(t)
		mockTool.AssertExpectations(t)
	})
	t.Run("Should execute multiple tool calls and return combined results", func(t *testing.T) {
		mockRegistry := &MockToolRegistry{}
		mockTool1 := &MockTool{}
		mockTool2 := &MockTool{}
		orchestrator := &llmOrchestrator{
			config: OrchestratorConfig{
				ToolRegistry: mockRegistry,
			},
		}
		ctx := context.Background()
		request := Request{
			Action: &agent.ActionConfig{},
		}
		toolCalls := []llmadapter.ToolCall{
			{
				ID:        "call1",
				Name:      "tool1",
				Arguments: []byte(`{"arg": "value1"}`),
			},
			{
				ID:        "call2",
				Name:      "tool2",
				Arguments: []byte(`{"arg": "value2"}`),
			},
		}
		mockRegistry.On("Find", mock.Anything, "tool1").Return(mockTool1, true)
		mockRegistry.On("Find", mock.Anything, "tool2").Return(mockTool2, true)
		mockTool1.On("Call", mock.Anything, `{"arg": "value1"}`).Return(`{"result": "success1"}`, nil)
		mockTool2.On("Call", mock.Anything, `{"arg": "value2"}`).Return(`{"result": "success2"}`, nil)
		result, err := orchestrator.executeToolCalls(ctx, toolCalls, request)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		// create addressable variables for nested outputs
		res1 := core.Output(map[string]any{"result": "success1"})
		res2 := core.Output(map[string]any{"result": "success2"})
		expected := core.Output(map[string]any{
			"results": []map[string]any{
				{
					"tool_call_id": "call1",
					"tool_name":    "tool1",
					"result":       &res1,
				},
				{
					"tool_call_id": "call2",
					"tool_name":    "tool2",
					"result":       &res2,
				},
			},
		})
		assert.Equal(t, &expected, result)
		mockRegistry.AssertExpectations(t)
		mockTool1.AssertExpectations(t)
		mockTool2.AssertExpectations(t)
	})
	t.Run("Should handle tool execution errors", func(t *testing.T) {
		mockRegistry := &MockToolRegistry{}
		mockTool := &MockTool{}
		orchestrator := &llmOrchestrator{
			config: OrchestratorConfig{
				ToolRegistry: mockRegistry,
			},
		}
		ctx := context.Background()
		request := Request{
			Action: &agent.ActionConfig{},
		}
		toolCalls := []llmadapter.ToolCall{
			{
				ID:        "call1",
				Name:      "test-tool",
				Arguments: []byte(`{"arg": "value"}`),
			},
		}
		expectedErr := fmt.Errorf("tool execution failed")
		mockRegistry.On("Find", mock.Anything, "test-tool").Return(mockTool, true)
		mockTool.On("Call", mock.Anything, `{"arg": "value"}`).Return("", expectedErr)
		result, err := orchestrator.executeToolCalls(ctx, toolCalls, request)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "tool execution failed")
		mockRegistry.AssertExpectations(t)
		mockTool.AssertExpectations(t)
	})
}

func TestIsRetryableError(t *testing.T) {
	t.Run("Should not retry on context cancellation", func(t *testing.T) {
		err := context.Canceled
		retryable := isRetryableError(err)
		assert.False(t, retryable)
	})

	t.Run("Should retry on context deadline exceeded", func(t *testing.T) {
		err := context.DeadlineExceeded
		retryable := isRetryableError(err)
		assert.True(t, retryable)
	})

	t.Run("Should use structured LLM error retry decision", func(t *testing.T) {
		// Test retryable LLM error
		retryableErr := llmadapter.NewError(http.StatusTooManyRequests, "Rate limit exceeded", "openai", nil)
		retryable := isRetryableError(retryableErr)
		assert.True(t, retryable)

		// Test non-retryable LLM error
		nonRetryableErr := llmadapter.NewError(http.StatusUnauthorized, "Invalid API key", "openai", nil)
		retryable = isRetryableError(nonRetryableErr)
		assert.False(t, retryable)
	})

	t.Run("Should retry on network timeout errors", func(t *testing.T) {
		mockNetErr := &mockNetError{timeout: true}
		retryable := isRetryableError(mockNetErr)
		assert.True(t, retryable)
	})

	t.Run("Should not retry on non-timeout network errors", func(t *testing.T) {
		mockNetErr := &mockNetError{timeout: false}
		retryable := isRetryableError(mockNetErr)
		assert.False(t, retryable)
	})

	t.Run("Should retry on retryable string patterns", func(t *testing.T) {
		testCases := []struct {
			errorMsg string
			expected bool
		}{
			{"rate limit exceeded", true},
			{"429 Too Many Requests", true},
			{"service unavailable", true},
			{"503 Service Unavailable", true},
			{"gateway timeout", true},
			{"504 Gateway Timeout", true},
			{"connection reset", true},
			{"throttled request", true},
			{"quota exceeded", true},
			{"capacity error", true},
			{"temporary failure", true},
			{"invalid api key", false},
			{"unauthorized", false},
			{"401 Unauthorized", false},
			{"forbidden", false},
			{"403 Forbidden", false},
			{"invalid model", false},
			{"unknown error", false},
		}

		for _, tc := range testCases {
			t.Run(tc.errorMsg, func(t *testing.T) {
				err := errors.New(tc.errorMsg)
				retryable := isRetryableError(err)
				assert.Equal(t, tc.expected, retryable, "Error message: %s", tc.errorMsg)
			})
		}
	})

	t.Run("Should default to not retrying unknown errors", func(t *testing.T) {
		err := errors.New("completely unknown error type")
		retryable := isRetryableError(err)
		assert.False(t, retryable)
	})

	t.Run("Should avoid false positives with numbers in other contexts", func(t *testing.T) {
		// These error messages contain numbers that match HTTP status codes
		// but in different contexts, so they should NOT trigger retries
		falsePositives := []struct {
			errorMsg string
			desc     string
		}{
			{"timeout set to 500ms", "duration with 500ms"},
			{"response time was 429ms", "time measurement with 429ms"},
			{"file size is 503kb", "file size with 503kb"},
			{"version 401.2.3", "version number with 401"},
			{"localhost:403", "port number with 403"},
			{"waited 500ms for response", "timing with 500ms"},
			{"SHA256:429abc123def", "hash with 429"},
		}

		for _, tc := range falsePositives {
			t.Run(tc.desc, func(t *testing.T) {
				err := errors.New(tc.errorMsg)
				retryable := isRetryableError(err)
				assert.False(t, retryable, "Should not retry for: %s", tc.errorMsg)
			})
		}
	})

	t.Run("Should correctly identify actual HTTP status codes", func(t *testing.T) {
		// These are actual error messages with HTTP status codes
		// that SHOULD trigger retries
		truePositives := []struct {
			errorMsg string
			desc     string
		}{
			{"error 500: internal server error", "HTTP 500 error"},
			{"429 too many requests", "HTTP 429 status"},
			{"503 service unavailable", "HTTP 503 status"},
			{"504 gateway timeout", "HTTP 504 status"},
			{"HTTP 429 rate limit exceeded", "HTTP 429 with context"},
			{"status: 503", "status code 503"},
		}

		for _, tc := range truePositives {
			t.Run(tc.desc, func(t *testing.T) {
				err := errors.New(tc.errorMsg)
				retryable := isRetryableError(err)
				assert.True(t, retryable, "Should retry for: %s", tc.errorMsg)
			})
		}
	})
}

func TestLangChainAdapterErrorExtraction(t *testing.T) {
	t.Run("Should extract HTTP status codes from error messages", func(t *testing.T) {
		testCases := []struct {
			errorMsg     string
			expectedCode int
		}{
			{"HTTP 429: Rate limit exceeded", http.StatusTooManyRequests},
			{"status code: 503 service unavailable", http.StatusServiceUnavailable},
			{"error 500 internal server error", http.StatusInternalServerError},
			{"API returned 404 not found", http.StatusNotFound},
			{"request failed with 401", http.StatusUnauthorized},
			{"timeout error", 0}, // No status code
			{"generic error", 0}, // No status code
		}

		for _, tc := range testCases {
			t.Run(tc.errorMsg, func(t *testing.T) {
				// Use reflection or create a test method to access private extractHTTPStatusCode
				// For now, test the overall extractStructuredError behavior
				err := errors.New(tc.errorMsg)

				// This tests the pattern matching logic indirectly
				retryable := isRetryableError(err)

				// Verify that errors with retryable status codes are marked as retryable
				if tc.expectedCode == http.StatusTooManyRequests ||
					tc.expectedCode == http.StatusServiceUnavailable ||
					tc.expectedCode >= 500 {
					assert.True(
						t,
						retryable,
						"Error with status %d should be retryable: %s",
						tc.expectedCode,
						tc.errorMsg,
					)
				} else if tc.expectedCode >= 400 && tc.expectedCode < 500 && tc.expectedCode != http.StatusTooManyRequests {
					assert.False(t, retryable, "Error with status %d should not be retryable: %s", tc.expectedCode, tc.errorMsg)
				}
			})
		}
	})

	t.Run("Should handle provider-specific error patterns", func(t *testing.T) {
		testCases := []struct {
			errorMsg  string
			retryable bool
		}{
			{"OpenAI: insufficient_quota", true},
			{"Anthropic: rate_limit_error", true},
			{"Google: quota exceeded", true},
			{"invalid model specified", false},
			{"content policy violation", false},
		}

		for _, tc := range testCases {
			t.Run(tc.errorMsg, func(t *testing.T) {
				err := errors.New(tc.errorMsg)
				retryable := isRetryableError(err)
				assert.Equal(t, tc.retryable, retryable, "Error message: %s", tc.errorMsg)
			})
		}
	})
}

// mockNetError implements net.Error for testing
type mockNetError struct {
	timeout   bool
	temporary bool
	msg       string
}

func (e *mockNetError) Error() string {
	if e.msg != "" {
		return e.msg
	}
	return "mock network error"
}

func (e *mockNetError) Timeout() bool {
	return e.timeout
}

func (e *mockNetError) Temporary() bool {
	return e.temporary
}

func TestIsSuccessJSONRaw(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		wantSuccess bool
		wantParsed  bool
	}{
		{name: "object_no_error", input: `{"ok": true}`, wantSuccess: true, wantParsed: true},
		{name: "object_with_error_string", input: `{"error": "boom"}`, wantSuccess: false, wantParsed: true},
		{name: "object_with_error_null", input: `{"error": null}`, wantSuccess: false, wantParsed: true},
		{
			name:        "nested_error_not_top_level",
			input:       `{"data": {"error": "nested"}}`,
			wantSuccess: true,
			wantParsed:  true,
		},
		{name: "non_object_array", input: `[]`, wantSuccess: false, wantParsed: false},
		{name: "non_object_string", input: `"hi"`, wantSuccess: false, wantParsed: false},
		{name: "invalid_json", input: `{`, wantSuccess: false, wantParsed: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := json.RawMessage(tc.input)
			gotSuccess, gotParsed := isSuccessJSONRaw(raw)
			if gotSuccess != tc.wantSuccess || gotParsed != tc.wantParsed {
				t.Fatalf(
					"isSuccessJSONRaw(%s) = (%v,%v), want (%v,%v)",
					tc.name,
					gotSuccess,
					gotParsed,
					tc.wantSuccess,
					tc.wantParsed,
				)
			}
		})
	}
}

func TestIsSuccessText(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		wantSuccess bool
	}{
		// JSON cases
		{name: "Should treat JSON without error field as success", input: `{"message": "ok"}`, wantSuccess: true},
		{name: "Should treat JSON with error string as failure", input: `{"error": "nope"}`, wantSuccess: false},
		{name: "Should treat JSON with error null as failure", input: `{"error": null}`, wantSuccess: false},
		{
			name:        "Should treat JSON with nested error as success",
			input:       `{"data": {"error": "nested"}}`,
			wantSuccess: true,
		},
		// Plain text success cases
		{name: "Should treat plain text without error keywords as success", input: "all good", wantSuccess: true},
		{
			name:        "Should treat text with success message as success",
			input:       "Operation completed successfully",
			wantSuccess: true,
		},
		{name: "Should treat result text as success", input: "Result: 42", wantSuccess: true},
		// Plain text error cases (updated behavior)
		{
			name:        "Should treat text containing error word as failure",
			input:       "some text with error word",
			wantSuccess: false,
		},
		{name: "Should treat error message as failure", input: "Error: something went wrong", wantSuccess: false},
		{name: "Should treat failed message as failure", input: "Operation failed", wantSuccess: false},
		{
			name:        "Should treat missing required parameter as failure",
			input:       "missing required parameter: owner",
			wantSuccess: false,
		},
		{name: "Should treat invalid message as failure", input: "Invalid input provided", wantSuccess: false},
		{name: "Should treat not found message as failure", input: "Resource not found", wantSuccess: false},
		{name: "Should treat unauthorized message as failure", input: "Unauthorized access", wantSuccess: false},
		{
			name:        "Should treat forbidden message as failure",
			input:       "Forbidden: insufficient permissions",
			wantSuccess: false,
		},
		{
			name:        "Should treat bad request message as failure",
			input:       "Bad request: malformed input",
			wantSuccess: false,
		},
		{
			name:        "Should treat exception message as failure",
			input:       "Exception occurred during processing",
			wantSuccess: false,
		},
		{name: "Should treat cannot message as failure", input: "Cannot perform this operation", wantSuccess: false},
		{name: "Should treat unable message as failure", input: "Unable to complete request", wantSuccess: false},
		// Edge cases
		{name: "Should treat uppercase error as failure", input: "ERROR: SOMETHING WENT WRONG", wantSuccess: false},
		{
			name:        "Should treat mixed case failure as failure",
			input:       "Operation FAILED with code 500",
			wantSuccess: false,
		},
		{name: "Should treat empty string as success", input: "", wantSuccess: true},
		{name: "Should treat whitespace only as success", input: "   \n\t  ", wantSuccess: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isSuccessText(tc.input)
			if got != tc.wantSuccess {
				t.Fatalf("isSuccessText(%s) = %v, want %v", tc.name, got, tc.wantSuccess)
			}
		})
	}
}

func TestStableJSONFingerprint_NonJSONHashes(t *testing.T) {
	t.Run("Should hash non-JSON bytes instead of returning raw", func(t *testing.T) {
		input := []byte("plain-text-response")
		got := stableJSONFingerprint(input)
		// Expect a hex string of length 64 (sha256)
		require.Equal(t, 64, len(got))
	})
	t.Run("Should produce same hash for same non-JSON input", func(t *testing.T) {
		input := []byte("abcdef")
		a := stableJSONFingerprint(input)
		b := stableJSONFingerprint(input)
		assert.Equal(t, a, b)
	})
}

func TestBuildIterationFingerprint_StableAcrossOrder(t *testing.T) {
	t.Run("Should be stable for JSON arg key order and result key order", func(t *testing.T) {
		callsA := []llmadapter.ToolCall{{Name: "tool", Arguments: []byte(`{"a":1,"b":2}`)}}
		callsB := []llmadapter.ToolCall{{Name: "tool", Arguments: []byte(`{"b":2,"a":1}`)}}
		resA := []llmadapter.ToolResult{{Name: "tool", JSONContent: json.RawMessage(`{"x":true,"y":false}`)}}
		resB := []llmadapter.ToolResult{{Name: "tool", JSONContent: json.RawMessage(`{"y":false,"x":true}`)}}
		fp1 := buildIterationFingerprint(callsA, resA)
		fp2 := buildIterationFingerprint(callsB, resB)
		assert.Equal(t, fp1, fp2)
	})
}

func TestDetectNoProgress_StringCounter(t *testing.T) {
	t.Run("Should trigger after consecutive identical fingerprints and reset on change", func(t *testing.T) {
		o := &llmOrchestrator{}
		threshold := 2
		last := ""
		count := 0
		// first occurrence sets last, no trigger
		tr1 := o.detectNoProgress(threshold, "fp1", &last, &count)
		assert.False(t, tr1)
		// second equal increments counter=1, still below threshold
		tr2 := o.detectNoProgress(threshold, "fp1", &last, &count)
		assert.False(t, tr2)
		// third equal reaches threshold
		tr3 := o.detectNoProgress(threshold, "fp1", &last, &count)
		assert.True(t, tr3)
		// change fingerprint resets counter
		tr4 := o.detectNoProgress(threshold, "fp2", &last, &count)
		assert.False(t, tr4)
		assert.Equal(t, 0, count)
		assert.Equal(t, "fp2", last)
	})
}
