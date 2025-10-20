package llmadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/ollama/ollama/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newMockOllamaServerWithTools(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/chat", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.Header.Get("Content-Type"), "application/json")

		var req api.ChatRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		assert.Equal(t, "test-model", req.Model)
		require.Len(t, req.Tools, 1)
		assert.Equal(t, "function", req.Tools[0].Type)
		assert.Equal(t, "get_weather", req.Tools[0].Function.Name)
		require.NotNil(t, req.Stream)
		assert.False(t, *req.Stream)
		if len(req.Format) > 0 {
			var format string
			require.NoError(t, json.Unmarshal(req.Format, &format))
			assert.Equal(t, "json", format)
		}

		w.Header().Set("Content-Type", "application/json")
		response := api.ChatResponse{
			Model: "test-model",
			Message: api.Message{
				Role:    RoleAssistant,
				Content: "",
				ToolCalls: []api.ToolCall{
					{
						Function: api.ToolCallFunction{
							Name: "get_weather",
							Arguments: api.ToolCallFunctionArguments{
								"location": "Tokyo",
								"format":   "celsius",
							},
						},
					},
				},
			},
			Done: true,
		}
		payload, err := json.Marshal(response)
		require.NoError(t, err)
		fmt.Fprintf(w, "%s\n", payload)
	}))
}

func TestOllamaAdapter_GenerateContent_WithTools(t *testing.T) {
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	t.Run("Should call Ollama API with tools", func(t *testing.T) {
		server := newMockOllamaServerWithTools(t)
		t.Cleanup(server.Close)
		// Create adapter
		baseAdapter := &LangChainAdapter{
			provider: core.ProviderConfig{
				Provider: core.ProviderOllama,
				Model:    "test-model",
			},
		}
		adapter := newOllamaAdapter(baseAdapter, server.URL, WithOllamaHTTPClient(server.Client()))
		// Create request with tools
		req := &LLMRequest{
			SystemPrompt: "You are a helpful assistant",
			Messages: []Message{
				{Role: RoleUser, Content: "What's the weather in Tokyo?"},
			},
			Tools: []ToolDefinition{
				{
					Name:        "get_weather",
					Description: "Get current weather",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location": map[string]any{"type": "string"},
							"format":   map[string]any{"type": "string"},
						},
						"required": []string{"location", "format"},
					},
				},
			},
			Options: CallOptions{
				ToolChoice: "auto",
			},
		}
		// Call adapter
		response, err := adapter.GenerateContent(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, response)
		// Verify response contains tool calls
		assert.Len(t, response.ToolCalls, 1)
		assert.Equal(t, "get_weather", response.ToolCalls[0].Name)
		var args map[string]any
		err = json.Unmarshal(response.ToolCalls[0].Arguments, &args)
		require.NoError(t, err)
		assert.Equal(t, "Tokyo", args["location"])
		assert.Equal(t, "celsius", args["format"])
	})
	t.Run("Should fallback to base adapter when no tools provided", func(t *testing.T) {
		// Create adapter with mock base
		mockLLM := NewMockLLM("test-model")
		baseAdapter := &LangChainAdapter{
			provider: core.ProviderConfig{
				Provider: core.ProviderOllama,
				Model:    "test-model",
			},
			baseModel: mockLLM,
			buildModel: func(_ context.Context, _ *core.ProviderConfig, _ *openai.ResponseFormat) (llms.Model, error) {
				return mockLLM, nil
			},
			modelCache: map[string]llms.Model{"default": mockLLM},
		}
		adapter := newOllamaAdapter(baseAdapter, "http://localhost:11434")
		// Create request without tools
		req := &LLMRequest{
			Messages: []Message{
				{Role: RoleUser, Content: "Hello"},
			},
		}
		// Should use base implementation (mock LLM)
		response, err := adapter.GenerateContent(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Contains(t, response.Content, "Mock response")
	})
}

func TestOllamaAdapter_ConvertToOllamaRequest(t *testing.T) {
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	adapter := newOllamaAdapter(&LangChainAdapter{
		provider: core.ProviderConfig{
			Provider: core.ProviderOllama,
			Model:    "test-model",
		},
	}, "http://localhost:11434")
	t.Run("Should convert request with system prompt and tools", func(t *testing.T) {
		req := &LLMRequest{
			SystemPrompt: "You are helpful",
			Messages: []Message{
				{Role: RoleUser, Content: "Test"},
			},
			Tools: []ToolDefinition{
				{
					Name:        "test_tool",
					Description: "A test tool",
					Parameters:  map[string]any{"type": "object"},
				},
			},
			Options: CallOptions{
				ToolChoice: "auto",
			},
		}
		ollamaReq, err := adapter.convertToOllamaRequest(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "test-model", ollamaReq.Model)
		assert.Len(t, ollamaReq.Messages, 2) // system + user
		assert.Equal(t, RoleSystem, ollamaReq.Messages[0].Role)
		assert.Equal(t, "You are helpful", ollamaReq.Messages[0].Content)
		assert.Equal(t, RoleUser, ollamaReq.Messages[1].Role)
		assert.Len(t, ollamaReq.Tools, 1)
		assert.Equal(t, "test_tool", ollamaReq.Tools[0].Function.Name)
		if assert.NotNil(t, ollamaReq.Stream) {
			assert.False(t, *ollamaReq.Stream)
		}
	})
	t.Run("Should include tool calls, tool results, and options", func(t *testing.T) {
		req := &LLMRequest{
			Messages: []Message{
				{
					Role:    RoleAssistant,
					Content: "Checking lookup",
					ToolCalls: []ToolCall{
						{
							Name:      "lookup",
							Arguments: json.RawMessage(`{"query":"42"}`),
						},
					},
				},
				{
					Role: RoleTool,
					ToolResults: []ToolResult{
						{
							Name:        "lookup",
							JSONContent: json.RawMessage(`{"value":"answer"}`),
						},
					},
				},
			},
			Options: CallOptions{
				Temperature:       0.2,
				MaxTokens:         128,
				StopWords:         []string{"END"},
				RepetitionPenalty: 1.1,
				Metadata: map[string]any{
					"mirostat": 1,
				},
				ForceJSON: true,
			},
		}
		result, err := adapter.convertToOllamaRequest(ctx, req)
		require.NoError(t, err)
		require.Len(t, result.Messages, 2)
		assistant := result.Messages[0]
		require.Len(t, assistant.ToolCalls, 1)
		assert.Equal(t, "lookup", assistant.ToolCalls[0].Function.Name)
		assert.Equal(t, "42", assistant.ToolCalls[0].Function.Arguments["query"])
		tool := result.Messages[1]
		assert.Equal(t, RoleTool, tool.Role)
		assert.Equal(t, "lookup", tool.ToolName)
		assert.Equal(t, `{"value":"answer"}`, tool.Content)
		require.NotNil(t, result.Options)
		assert.Equal(t, 0.2, result.Options["temperature"])
		assert.EqualValues(t, 128, result.Options["num_predict"])
		assert.Equal(t, []string{"END"}, result.Options["stop"])
		assert.EqualValues(t, 1.1, result.Options["repeat_penalty"])
		assert.Equal(t, 1, result.Options["mirostat"])
		var format string
		require.NoError(t, json.Unmarshal(result.Format, &format))
		assert.Equal(t, "json", format)
	})
	t.Run("Should split multiple tool results into separate messages", func(t *testing.T) {
		req := &LLMRequest{
			Messages: []Message{
				{
					Role: RoleTool,
					ToolResults: []ToolResult{
						{Name: "alpha", JSONContent: json.RawMessage(`{"value":1}`)},
						{Name: "beta", Content: "handled"},
					},
				},
			},
		}
		result, err := adapter.convertToOllamaRequest(ctx, req)
		require.NoError(t, err)
		require.Len(t, result.Messages, 2)
		assert.Equal(t, RoleTool, result.Messages[0].Role)
		assert.Equal(t, "alpha", result.Messages[0].ToolName)
		assert.Equal(t, "beta", result.Messages[1].ToolName)
	})

	t.Run("Should fetch remote image URLs into binary data", func(t *testing.T) {
		imageData := []byte{0x01, 0x02, 0x03}
		client := &http.Client{
			Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				rec := httptest.NewRecorder()
				rec.WriteHeader(http.StatusOK)
				_, _ = rec.Write(imageData)
				return rec.Result(), nil
			}),
		}

		remoteAdapter := newOllamaAdapter(
			&LangChainAdapter{
				provider: core.ProviderConfig{
					Provider: core.ProviderOllama,
					Model:    "test-model",
				},
			},
			"https://example.com",
			WithOllamaHTTPClient(client),
		)

		req := &LLMRequest{
			Messages: []Message{
				{
					Role:  RoleUser,
					Parts: []ContentPart{ImageURLPart{URL: "https://example.com/image.png"}},
				},
			},
		}
		result, err := remoteAdapter.convertToOllamaRequest(ctx, req)
		require.NoError(t, err)
		require.Len(t, result.Messages, 1)
		require.Len(t, result.Messages[0].Images, 1)
		assert.Equal(t, imageData, []byte(result.Messages[0].Images[0]))
	})

	t.Run("Should add placeholder for oversized image binaries", func(t *testing.T) {
		req := &LLMRequest{
			Messages: []Message{
				{
					Role: RoleUser,
					Parts: []ContentPart{
						BinaryPart{MIMEType: "image/png", Data: make([]byte, maxInlineImageBytes+1)},
					},
				},
			},
		}
		result, err := adapter.convertToOllamaRequest(ctx, req)
		require.NoError(t, err)
		require.Len(t, result.Messages, 1)
		assert.Contains(t, result.Messages[0].Content, "Image omitted")
	})

	t.Run("Should include small inline binary images", func(t *testing.T) {
		req := &LLMRequest{
			Messages: []Message{
				{
					Role: RoleUser,
					Parts: []ContentPart{
						BinaryPart{MIMEType: "image/png", Data: make([]byte, maxInlineImageBytes-1)},
					},
				},
			},
		}
		result, err := adapter.convertToOllamaRequest(ctx, req)
		require.NoError(t, err)
		require.Len(t, result.Messages, 1)
		require.Len(t, result.Messages[0].Images, 1)
		assert.NotEmpty(t, result.Messages[0].Images[0])
		assert.NotContains(t, result.Messages[0].Content, "Image omitted")
	})

	t.Run("Should append directive when tool choice forbids tools", func(t *testing.T) {
		req := &LLMRequest{
			SystemPrompt: "Follow instructions",
			Messages:     []Message{{Role: RoleUser, Content: "Explain"}},
			Options:      CallOptions{ToolChoice: "none"},
		}
		result, err := adapter.convertToOllamaRequest(ctx, req)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(result.Messages), 1)
		assert.Equal(t, RoleSystem, result.Messages[0].Role)
		assert.Contains(t, result.Messages[0].Content, "Do not invoke any tool")
	})

	t.Run("Should append directive when tool choice specifies a tool", func(t *testing.T) {
		req := &LLMRequest{
			Messages: []Message{{Role: RoleUser, Content: "Lookup"}},
			Tools:    []ToolDefinition{{Name: "preferred"}},
			Options:  CallOptions{ToolChoice: "preferred"},
		}
		result, err := adapter.convertToOllamaRequest(ctx, req)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(result.Messages), 1)
		assert.Equal(t, RoleSystem, result.Messages[0].Role)
		assert.Contains(t, result.Messages[0].Content, "must invoke the function \"preferred\"")
	})
}
