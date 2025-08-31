package llm

import (
	"context"
	"sync"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// testAsyncHook provides synchronization for async operations in tests
type testAsyncHook struct {
	wg     sync.WaitGroup
	mu     sync.Mutex
	errors []error
}

func newTestAsyncHook() *testAsyncHook {
	return &testAsyncHook{
		errors: make([]error, 0),
	}
}

func (h *testAsyncHook) OnMemoryStoreComplete(err error) {
	h.mu.Lock()
	if err != nil {
		h.errors = append(h.errors, err)
	}
	h.mu.Unlock()
	h.wg.Done()
}

func (h *testAsyncHook) expectMemoryStore() {
	h.wg.Add(1)
}

func (h *testAsyncHook) wait() {
	h.wg.Wait()
}

func (h *testAsyncHook) getErrors() []error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]error{}, h.errors...)
}

// Mock LLM client for testing
type MockLLMClient struct {
	mock.Mock
}

func (m *MockLLMClient) GenerateContent(
	ctx context.Context,
	req *llmadapter.LLMRequest,
) (*llmadapter.LLMResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*llmadapter.LLMResponse), args.Error(1)
}

func (m *MockLLMClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

// Mock LLM factory for testing
type MockLLMFactory struct {
	mock.Mock
}

func (m *MockLLMFactory) CreateClient(config *core.ProviderConfig) (llmadapter.LLMClient, error) {
	args := m.Called(config)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(llmadapter.LLMClient), args.Error(1)
}

func TestOrchestrator_ExecuteWithMemory(t *testing.T) {
	ctx := context.Background()

	t.Run("Should execute without memory when no memory provider", func(t *testing.T) {
		// Setup mocks
		mockRegistry := &MockToolRegistry{}
		mockPromptBuilder := &MockPromptBuilder{}
		mockFactory := &MockLLMFactory{}
		mockClient := &MockLLMClient{}

		orchestrator := NewOrchestrator(&OrchestratorConfig{
			ToolRegistry:   mockRegistry,
			PromptBuilder:  mockPromptBuilder,
			LLMFactory:     mockFactory,
			MemoryProvider: nil, // No memory provider
		})

		// Setup request
		agentCfg := &agent.Config{
			ID:           "test-agent",
			Instructions: "You are a helpful assistant",
			Config: core.ProviderConfig{
				Provider: "openai",
				Model:    "gpt-4",
			},
			CWD: &core.PathCWD{Path: "."},
		}
		actionCfg := &agent.ActionConfig{
			ID:     "test-action",
			Prompt: "Hello, how are you?",
		}
		request := Request{
			Agent:  agentCfg,
			Action: actionCfg,
		}

		// Setup expectations
		mockPromptBuilder.On("Build", ctx, actionCfg).Return("Hello, how are you?", nil)
		mockPromptBuilder.On("ShouldUseStructuredOutput", "openai", actionCfg, agentCfg.Tools).Return(false)
		mockFactory.On("CreateClient", &agentCfg.Config).Return(mockClient, nil)
		mockClient.On("GenerateContent", ctx, mock.MatchedBy(func(req *llmadapter.LLMRequest) bool {
			// Verify no memory messages are included
			return len(req.Messages) == 1 && req.Messages[0].Content == "Hello, how are you?"
		})).Return(&llmadapter.LLMResponse{
			Content: "I'm doing well, thank you!",
		}, nil)
		mockClient.On("Close").Return(nil)

		// Execute
		output, err := orchestrator.Execute(ctx, request)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, output)

		// Verify expectations
		mockPromptBuilder.AssertExpectations(t)
		mockFactory.AssertExpectations(t)
		mockClient.AssertExpectations(t)
	})

	t.Run("Should include memory messages when memory is available", func(t *testing.T) {
		// Setup mocks
		mockRegistry := &MockToolRegistry{}
		mockPromptBuilder := &MockPromptBuilder{}
		mockFactory := &MockLLMFactory{}
		mockClient := &MockLLMClient{}
		mockMemoryProvider := &mockMemoryProvider{}
		mockMemory := &mockMemory{id: "test-memory"}
		asyncHook := newTestAsyncHook()

		orchestrator := NewOrchestrator(&OrchestratorConfig{
			ToolRegistry:   mockRegistry,
			PromptBuilder:  mockPromptBuilder,
			LLMFactory:     mockFactory,
			MemoryProvider: mockMemoryProvider,
			AsyncHook:      asyncHook,
		})

		// Setup request with memory references
		agentCfg := &agent.Config{
			ID:           "test-agent",
			Instructions: "You are a helpful assistant",
			Config: core.ProviderConfig{
				Provider: "openai",
				Model:    "gpt-4",
			},
			Memory: []core.MemoryReference{
				{ID: "test-memory", Key: "user-123", Mode: "read-write"},
			},
			CWD: &core.PathCWD{Path: "."},
		}
		// Call Validate to set resolved memory references
		err := agentCfg.Validate()
		assert.NoError(t, err)

		actionCfg := &agent.ActionConfig{
			ID:     "test-action",
			Prompt: "What did we talk about last time?",
		}
		request := Request{
			Agent:  agentCfg,
			Action: actionCfg,
		}

		// Setup memory expectations
		memoryMessages := []Message{
			{Role: MessageRoleUser, Content: "Tell me about cats"},
			{Role: MessageRoleAssistant, Content: "Cats are wonderful pets..."},
		}
		mockMemoryProvider.On("GetMemory", ctx, "test-memory", "user-123").Return(mockMemory, nil)
		mockMemory.On("Read", ctx).Return(memoryMessages, nil)

		// Expect messages to be stored asynchronously after response using AppendMany
		asyncHook.expectMemoryStore() // Signal that we expect one memory store operation
		mockMemory.On("AppendMany", mock.Anything, mock.MatchedBy(func(msgs []Message) bool {
			return len(msgs) == 2 &&
				msgs[0].Role == MessageRoleUser && msgs[0].Content == "What did we talk about last time?" &&
				msgs[1].Role == MessageRoleAssistant && msgs[1].Content == "We were discussing cats and their characteristics."
		})).Return(nil)

		// Setup other expectations
		mockPromptBuilder.On("Build", ctx, actionCfg).Return("What did we talk about last time?", nil)
		mockPromptBuilder.On("ShouldUseStructuredOutput", "openai", actionCfg, agentCfg.Tools).Return(false)
		mockFactory.On("CreateClient", &agentCfg.Config).Return(mockClient, nil)
		mockClient.On("GenerateContent", ctx, mock.MatchedBy(func(req *llmadapter.LLMRequest) bool {
			// Verify memory messages are included
			return len(req.Messages) == 3 &&
				req.Messages[0].Role == "user" && req.Messages[0].Content == "Tell me about cats" &&
				req.Messages[1].Role == "assistant" && req.Messages[1].Content == "Cats are wonderful pets..." &&
				req.Messages[2].Role == "user" && req.Messages[2].Content == "What did we talk about last time?"
		})).Return(&llmadapter.LLMResponse{
			Content: "We were discussing cats and their characteristics.",
		}, nil)
		mockClient.On("Close").Return(nil)

		// Execute
		output, err := orchestrator.Execute(ctx, request)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, output)

		// Wait for async memory storage to complete
		asyncHook.wait()

		// Check if any errors occurred during async operations
		errors := asyncHook.getErrors()
		assert.Empty(t, errors, "No errors should occur during async memory storage")

		// Verify expectations
		mockMemoryProvider.AssertExpectations(t)
		mockMemory.AssertExpectations(t)
		mockPromptBuilder.AssertExpectations(t)
		mockFactory.AssertExpectations(t)
		mockClient.AssertExpectations(t)
	})

	t.Run("Should handle memory read errors gracefully", func(t *testing.T) {
		// Setup mocks
		mockRegistry := &MockToolRegistry{}
		mockPromptBuilder := &MockPromptBuilder{}
		mockFactory := &MockLLMFactory{}
		mockClient := &MockLLMClient{}
		mockMemoryProvider := &mockMemoryProvider{}

		orchestrator := NewOrchestrator(&OrchestratorConfig{
			ToolRegistry:   mockRegistry,
			PromptBuilder:  mockPromptBuilder,
			LLMFactory:     mockFactory,
			MemoryProvider: mockMemoryProvider,
		})

		// Setup request with memory references
		agentCfg := &agent.Config{
			ID:           "test-agent",
			Instructions: "You are a helpful assistant",
			Config: core.ProviderConfig{
				Provider: "openai",
				Model:    "gpt-4",
			},
			Memory: []core.MemoryReference{
				{ID: "test-memory", Key: "user-123", Mode: "read-write"},
			},
			CWD: &core.PathCWD{Path: "."},
		}
		err := agentCfg.Validate()
		assert.NoError(t, err)

		actionCfg := &agent.ActionConfig{
			ID:     "test-action",
			Prompt: "Hello",
		}
		request := Request{
			Agent:  agentCfg,
			Action: actionCfg,
		}

		// Setup memory error
		mockMemoryProvider.On("GetMemory", ctx, "test-memory", "user-123").Return(nil, assert.AnError)

		// Setup other expectations - should continue without memory
		mockPromptBuilder.On("Build", ctx, actionCfg).Return("Hello", nil)
		mockPromptBuilder.On("ShouldUseStructuredOutput", "openai", actionCfg, agentCfg.Tools).Return(false)
		mockFactory.On("CreateClient", &agentCfg.Config).Return(mockClient, nil)
		mockClient.On("GenerateContent", ctx, mock.MatchedBy(func(req *llmadapter.LLMRequest) bool {
			// Should proceed with just the user message
			return len(req.Messages) == 1 && req.Messages[0].Content == "Hello"
		})).Return(&llmadapter.LLMResponse{
			Content: "Hi there!",
		}, nil)
		mockClient.On("Close").Return(nil)

		// Execute
		output, err := orchestrator.Execute(ctx, request)

		// Assert - should succeed despite memory error
		assert.NoError(t, err)
		assert.NotNil(t, output)

		// Verify expectations
		mockMemoryProvider.AssertExpectations(t)
		mockPromptBuilder.AssertExpectations(t)
		mockFactory.AssertExpectations(t)
		mockClient.AssertExpectations(t)
	})
}
