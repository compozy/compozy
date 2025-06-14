package llmadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// TestAdapter is a test implementation of LLMClient for unit testing
type TestAdapter struct {
	mu sync.RWMutex

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
	t.mu.Lock()
	t.Calls = append(t.Calls, *req)
	response := t.Response
	err := t.Error
	t.mu.Unlock()
	if err != nil {
		return nil, err
	}
	if response != nil {
		return response, nil
	}
	return &LLMResponse{
		Content: "Test response",
	}, nil
}

// SetResponse configures the response to return
func (t *TestAdapter) SetResponse(content string, toolCalls ...ToolCall) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Response = &LLMResponse{
		Content:   content,
		ToolCalls: toolCalls,
	}
}

// SetError configures an error to return
func (t *TestAdapter) SetError(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Error = err
}

// GetLastCall returns the most recent call made to the adapter
func (t *TestAdapter) GetLastCall() *LLMRequest {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if len(t.Calls) == 0 {
		return nil
	}
	return &t.Calls[len(t.Calls)-1]
}

// Reset clears all recorded calls and configured responses
func (t *TestAdapter) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Calls = make([]LLMRequest, 0)
	t.Response = nil
	t.Error = nil
}

// MockToolAdapter is a test adapter that simulates tool-calling behavior
type MockToolAdapter struct {
	*TestAdapter
	toolMu      sync.RWMutex
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
	m.mu.Lock()
	m.Calls = append(m.Calls, *req)
	err := m.Error
	response := m.Response
	m.mu.Unlock()
	if err != nil {
		return nil, err
	}
	m.toolMu.RLock()
	toolResults := make(map[string]string)
	for k, v := range m.ToolResults {
		toolResults[k] = v
	}
	m.toolMu.RUnlock()
	if len(req.Tools) > 0 && len(toolResults) > 0 {
		for _, tool := range req.Tools {
			if _, ok := toolResults[tool.Name]; ok {
				args := map[string]any{"input": "test"}
				argsJSON, err := json.Marshal(args)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal args: %w", err)
				}
				return &LLMResponse{
					Content: "",
					ToolCalls: []ToolCall{
						{
							ID:        fmt.Sprintf("call_%s", tool.Name),
							Name:      tool.Name,
							Arguments: argsJSON,
						},
					},
				}, nil
			}
		}
	}
	if response != nil {
		return response, nil
	}
	return &LLMResponse{
		Content: "Mock response",
	}, nil
}

// SetToolResult configures a result for a specific tool
func (m *MockToolAdapter) SetToolResult(toolName, result string) {
	m.toolMu.Lock()
	defer m.toolMu.Unlock()
	m.ToolResults[toolName] = result
}
