package llmadapter

import (
	"context"
	"encoding/json"
	"fmt"
)

// TestAdapter is a test implementation of LLMClient for unit testing
type TestAdapter struct {
	// Response to return
	Response *LLMResponse
	Error    error

	// Record of calls made
	Calls []LLMRequest
}

// NewTestAdapter creates a new test adapter
func NewTestAdapter() *TestAdapter {
	return &TestAdapter{
		Calls: make([]LLMRequest, 0),
	}
}

// GenerateContent implements LLMClient interface
func (t *TestAdapter) GenerateContent(_ context.Context, req *LLMRequest) (*LLMResponse, error) {
	// Record the call
	t.Calls = append(t.Calls, *req)

	// Return configured response or error
	if t.Error != nil {
		return nil, t.Error
	}

	if t.Response != nil {
		return t.Response, nil
	}

	// Default response
	return &LLMResponse{
		Content: "Test response",
	}, nil
}

// SetResponse configures the response to return
func (t *TestAdapter) SetResponse(content string, toolCalls ...ToolCall) {
	t.Response = &LLMResponse{
		Content:   content,
		ToolCalls: toolCalls,
	}
}

// SetError configures an error to return
func (t *TestAdapter) SetError(err error) {
	t.Error = err
}

// GetLastCall returns the most recent call made to the adapter
func (t *TestAdapter) GetLastCall() *LLMRequest {
	if len(t.Calls) == 0 {
		return nil
	}
	return &t.Calls[len(t.Calls)-1]
}

// Reset clears all recorded calls and configured responses
func (t *TestAdapter) Reset() {
	t.Calls = make([]LLMRequest, 0)
	t.Response = nil
	t.Error = nil
}

// MockToolAdapter is a test adapter that simulates tool-calling behavior
type MockToolAdapter struct {
	*TestAdapter
	ToolResults map[string]string // Map of tool name to result
}

// NewMockToolAdapter creates a new mock tool adapter
func NewMockToolAdapter() *MockToolAdapter {
	return &MockToolAdapter{
		TestAdapter: NewTestAdapter(),
		ToolResults: make(map[string]string),
	}
}

// GenerateContent simulates tool calling behavior
func (m *MockToolAdapter) GenerateContent(_ context.Context, req *LLMRequest) (*LLMResponse, error) {
	// Record the call
	m.Calls = append(m.Calls, *req)

	if m.Error != nil {
		return nil, m.Error
	}

	// If tools are provided and we have a configured result, simulate a tool call
	if len(req.Tools) > 0 && len(m.ToolResults) > 0 {
		for _, tool := range req.Tools {
			if _, ok := m.ToolResults[tool.Name]; ok {
				// Simulate calling the tool
				args := map[string]any{"input": "test"}
				argsJSON, err := json.Marshal(args)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal args: %w", err)
				}

				return &LLMResponse{
					ToolCalls: []ToolCall{
						{
							ID:        fmt.Sprintf("call_%s", tool.Name),
							Name:      tool.Name,
							Arguments: string(argsJSON),
						},
					},
				}, nil
			}
		}
	}

	// Default to configured response or simple text
	if m.Response != nil {
		return m.Response, nil
	}

	return &LLMResponse{
		Content: "Mock response",
	}, nil
}

// SetToolResult configures a result for a specific tool
func (m *MockToolAdapter) SetToolResult(toolName, result string) {
	m.ToolResults[toolName] = result
}
