package llm

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

func (m *MockToolRegistry) Register(tool Tool) error {
	args := m.Called(tool)
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

func (m *MockToolRegistry) InvalidateCache() {
	m.Called()
}

func (m *MockToolRegistry) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockPromptBuilder implements the PromptBuilder interface for testing
type MockPromptBuilder struct {
	mock.Mock
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
	basePrompt string,
	outputSchema *schema.Schema,
	hasTools bool,
) string {
	args := m.Called(basePrompt, outputSchema, hasTools)
	return args.String(0)
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
		mockRegistry.On("Find", ctx, "test-tool").Return(mockTool, true)
		mockTool.On("Call", ctx, `{"arg": "value"}`).Return(`{"result": "success"}`, nil)
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
				Arguments: []byte(`{"arg1": "value1"}`),
			},
			{
				ID:        "call2",
				Name:      "tool2",
				Arguments: []byte(`{"arg2": "value2"}`),
			},
		}
		mockRegistry.On("Find", ctx, "tool1").Return(mockTool1, true)
		mockRegistry.On("Find", ctx, "tool2").Return(mockTool2, true)
		mockTool1.On("Call", ctx, `{"arg1": "value1"}`).Return(`{"result1": "success1"}`, nil)
		mockTool2.On("Call", ctx, `{"arg2": "value2"}`).Return(`{"result2": "success2"}`, nil)
		result, err := orchestrator.executeToolCalls(ctx, toolCalls, request)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		resultMap := map[string]any(*result)
		assert.Contains(t, resultMap, "results")
		toolResults := resultMap["results"].([]map[string]any)
		assert.Len(t, toolResults, 2)
		assert.Equal(t, "call1", toolResults[0]["tool_call_id"])
		assert.Equal(t, "tool1", toolResults[0]["tool_name"])
		assert.Equal(t, "call2", toolResults[1]["tool_call_id"])
		assert.Equal(t, "tool2", toolResults[1]["tool_name"])
		mockRegistry.AssertExpectations(t)
		mockTool1.AssertExpectations(t)
		mockTool2.AssertExpectations(t)
	})

	t.Run("Should return error when tool not found", func(t *testing.T) {
		mockRegistry := &MockToolRegistry{}
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
				Name:      "nonexistent-tool",
				Arguments: []byte(`{"arg": "value"}`),
			},
		}
		mockRegistry.On("Find", ctx, "nonexistent-tool").Return(nil, false)
		result, err := orchestrator.executeToolCalls(ctx, toolCalls, request)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "tool not found")
		mockRegistry.AssertExpectations(t)
	})
}
