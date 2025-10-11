package llmadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

func TestLangChainAdapter_ConvertMessages(t *testing.T) {
	adapter := &LangChainAdapter{}

	t.Run("Should convert messages with system prompt", func(t *testing.T) {
		req := LLMRequest{
			SystemPrompt: "You are a helpful assistant",
			Messages: []Message{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there!"},
			},
		}

		messages, err := adapter.convertMessages(context.Background(), &req)
		require.NoError(t, err)

		assert.Len(t, messages, 3)
		// Check system message
		assert.Equal(t, llms.ChatMessageTypeSystem, messages[0].Role)
		assert.Equal(t, "You are a helpful assistant", messages[0].Parts[0].(llms.TextContent).Text)
		// Check user message
		assert.Equal(t, llms.ChatMessageTypeHuman, messages[1].Role)
		assert.Equal(t, "Hello", messages[1].Parts[0].(llms.TextContent).Text)
		// Check assistant message
		assert.Equal(t, llms.ChatMessageTypeAI, messages[2].Role)
		assert.Equal(t, "Hi there!", messages[2].Parts[0].(llms.TextContent).Text)
	})

	t.Run("Should handle messages without system prompt", func(t *testing.T) {
		req := LLMRequest{
			Messages: []Message{
				{Role: "user", Content: "Test message"},
			},
		}

		messages, err := adapter.convertMessages(context.Background(), &req)
		require.NoError(t, err)

		assert.Len(t, messages, 1)
		assert.Equal(t, llms.ChatMessageTypeHuman, messages[0].Role)
	})
}

func TestLangChainAdapter_ConvertMessages_WithImageParts(t *testing.T) {
	adapter := &LangChainAdapter{}

	req := LLMRequest{
		Messages: []Message{
			{
				Role:    "user",
				Content: "Identify the object in the image",
				Parts: []ContentPart{
					ImageURLPart{URL: "https://example.com/img.png", Detail: "high"},
				},
			},
		},
	}

	msgs, err := adapter.convertMessages(context.Background(), &req)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	parts := msgs[0].Parts
	// Expect at least the text content and the image content
	require.GreaterOrEqual(t, len(parts), 2)
	// First part should be text from Content
	if tc, ok := parts[0].(llms.TextContent); ok {
		assert.Contains(t, tc.Text, "Identify the object")
	} else {
		t.Fatalf("first part should be TextContent, got %T", parts[0])
	}
	// One of the parts must be ImageURLContent
	var foundImage bool
	for _, p := range parts {
		if img, ok := p.(llms.ImageURLContent); ok {
			foundImage = true
			assert.Equal(t, "https://example.com/img.png", img.URL)
			assert.Equal(t, "high", img.Detail)
		}
	}
	assert.True(t, foundImage, "expected ImageURLContent part")
}

func TestLangChainAdapter_BuildContentParts_AudioVideo_ByProvider(t *testing.T) {
	// Prepare a message carrying audio and video binary parts plus text
	mkMsg := func() Message {
		return Message{
			Role:    "user",
			Content: "Analyze the attached media",
			Parts: []ContentPart{
				BinaryPart{MIMEType: "audio/wav", Data: []byte{0x01, 0x02}},
				BinaryPart{MIMEType: "video/mp4", Data: []byte{0x03, 0x04}},
			},
		}
	}

	t.Run("Google should forward audio/video as binary parts", func(t *testing.T) {
		adapter := &LangChainAdapter{provider: core.ProviderConfig{Provider: core.ProviderGoogle}}
		req := LLMRequest{Messages: []Message{mkMsg()}}
		msgs, err := adapter.convertMessages(context.Background(), &req)
		require.NoError(t, err)
		require.Len(t, msgs, 1)
		// Expect first part is TextContent and two BinaryContent parts for media
		var binCount int
		for _, p := range msgs[0].Parts {
			if _, ok := p.(llms.BinaryContent); ok {
				binCount++
			}
		}
		assert.Equal(t, 2, binCount)
	})

	// Providers that should skip audio/video (OpenAI-compatible and Ollama/Anthropic)
	// Note: OpenAI now supports audio via data URL conversion, still skips video.
	providers := []core.ProviderName{
		core.ProviderGroq,
		core.ProviderDeepSeek,
		core.ProviderXAI,
		core.ProviderOllama,
		core.ProviderAnthropic,
	}
	for _, p := range providers {
		p := p
		t.Run("Provider "+string(p)+" should skip audio/video", func(t *testing.T) {
			t.Parallel()
			adapter := &LangChainAdapter{provider: core.ProviderConfig{Provider: p}}
			req := LLMRequest{Messages: []Message{mkMsg()}}
			msgs, err := adapter.convertMessages(context.Background(), &req)
			require.NoError(t, err)
			require.Len(t, msgs, 1)
			for _, part := range msgs[0].Parts {
				if _, ok := part.(llms.BinaryContent); ok {
					t.Fatalf("unexpected BinaryContent for provider %s", p)
				}
			}
		})
	}
}

func TestLangChainAdapter_OpenAI_AudioSupport(t *testing.T) {
	t.Run("Should skip audio for OpenAI with warn mode", func(t *testing.T) {
		adapter := &LangChainAdapter{
			provider:       core.ProviderConfig{Provider: core.ProviderOpenAI},
			validationMode: ValidationModeWarn,
		}
		audioData := []byte{0x01, 0x02, 0x03}
		msg := Message{
			Role:    "user",
			Content: "Analyze this audio",
			Parts:   []ContentPart{BinaryPart{MIMEType: "audio/wav", Data: audioData}},
		}
		req := LLMRequest{Messages: []Message{msg}}
		msgs, err := adapter.convertMessages(context.Background(), &req)
		require.NoError(t, err)
		require.Len(t, msgs, 1)
		assert.Equal(t, llms.TextContent{Text: "Analyze this audio"}, msgs[0].Parts[0])
		assert.Len(t, msgs[0].Parts, 1, "Audio should be skipped, not converted")
	})
	t.Run("Should return error for OpenAI audio with error mode", func(t *testing.T) {
		adapter := &LangChainAdapter{
			provider:       core.ProviderConfig{Provider: core.ProviderOpenAI},
			validationMode: ValidationModeError,
		}
		audioData := []byte{0x01, 0x02, 0x03}
		msg := Message{
			Role:    "user",
			Content: "Analyze this audio",
			Parts:   []ContentPart{BinaryPart{MIMEType: "audio/wav", Data: audioData}},
		}
		req := LLMRequest{Messages: []Message{msg}}
		_, err := adapter.convertMessages(context.Background(), &req)
		require.Error(t, err)
		assert.ErrorContains(t, err, "OpenAI audio input not supported")
	})
}

func TestLangChainAdapter_OpenAI_SkipGenericBinary(t *testing.T) {
	t.Run("Should skip generic binary parts for OpenAI", func(t *testing.T) {
		adapter := &LangChainAdapter{provider: core.ProviderConfig{Provider: core.ProviderOpenAI}}
		msg := Message{Role: "user", Content: "Analyze", Parts: []ContentPart{
			BinaryPart{MIMEType: "application/octet-stream", Data: []byte{0xAA, 0xBB}},
		}}
		req := LLMRequest{Messages: []Message{msg}}
		msgs, err := adapter.convertMessages(context.Background(), &req)
		require.NoError(t, err)
		require.Len(t, msgs, 1)
		for _, p := range msgs[0].Parts {
			if _, ok := p.(llms.BinaryContent); ok {
				t.Fatalf("unexpected BinaryContent for OpenAI provider")
			}
		}
	})
}

func TestLangChainAdapter_EnsureProviderTools(t *testing.T) {
	base := []ToolDefinition{{
		Name:        "weather_tool",
		Description: "Weather",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}}

	t.Run("Should append json tool for Groq when missing", func(t *testing.T) {
		adapter := &LangChainAdapter{provider: core.ProviderConfig{Provider: core.ProviderGroq}}
		out := adapter.ensureProviderTools(base)
		require.Len(t, out, 2)
		assert.Equal(t, "json", out[1].Name)
	})

	t.Run("Should not duplicate json tool when already present", func(t *testing.T) {
		adapter := &LangChainAdapter{provider: core.ProviderConfig{Provider: core.ProviderGroq}}
		withJSON := append(append([]ToolDefinition{}, base...), ToolDefinition{Name: "json"})
		out := adapter.ensureProviderTools(withJSON)
		require.Len(t, out, 2)
	})

	t.Run("Should leave tools unchanged for non-Groq providers", func(t *testing.T) {
		adapter := &LangChainAdapter{provider: core.ProviderConfig{Provider: core.ProviderOpenAI}}
		out := adapter.ensureProviderTools(base)
		require.Equal(t, base, out)
	})
}

func TestLangChainAdapter_ShouldForceJSON(t *testing.T) {
	req := &LLMRequest{Options: CallOptions{ForceJSON: true}}

	t.Run("Should enable JSON mode for OpenAI", func(t *testing.T) {
		adapter := &LangChainAdapter{
			provider:     core.ProviderConfig{Provider: core.ProviderOpenAI},
			capabilities: ProviderCapabilities{StructuredOutput: true},
		}
		options := adapter.buildCallOptions(req)
		var callOpts llms.CallOptions
		for _, opt := range options {
			opt(&callOpts)
		}
		assert.True(t, callOpts.JSONMode)
	})

	t.Run("Should disable JSON mode for unsupported providers", func(t *testing.T) {
		adapter := &LangChainAdapter{provider: core.ProviderConfig{Provider: core.ProviderGoogle}}
		options := adapter.buildCallOptions(req)
		var callOpts llms.CallOptions
		for _, opt := range options {
			opt(&callOpts)
		}
		assert.False(t, callOpts.JSONMode)
	})

	t.Run("Should ignore when request does not require JSON", func(t *testing.T) {
		adapter := &LangChainAdapter{
			provider:     core.ProviderConfig{Provider: core.ProviderOpenAI},
			capabilities: ProviderCapabilities{StructuredOutput: true},
		}
		options := adapter.buildCallOptions(&LLMRequest{Options: CallOptions{ForceJSON: false}})
		var callOpts llms.CallOptions
		for _, opt := range options {
			opt(&callOpts)
		}
		assert.False(t, callOpts.JSONMode)
	})
}

func TestLangChainAdapter_HandleBinary_ImageLimit(t *testing.T) {
	t.Run("Should omit oversized images with sentinel text", func(t *testing.T) {
		adapter := &LangChainAdapter{
			provider: core.ProviderConfig{Provider: core.ProviderOpenAI},
		}
		data := make([]byte, maxInlineImageBytes+1)
		out, err := adapter.handleBinary(
			context.Background(),
			BinaryPart{MIMEType: "image/png", Data: data},
			nil,
		)
		require.NoError(t, err)
		require.Len(t, out, 1)
		textPart, ok := out[0].(llms.TextContent)
		require.True(t, ok)
		assert.Equal(
			t,
			fmt.Sprintf(oversizedImageMsgPattern, len(data), maxInlineImageBytes),
			textPart.Text,
		)
	})
}

func TestLangChainAdapter_ForceJSONUnsupportedProvider(t *testing.T) {
	adapter := &LangChainAdapter{
		provider:     core.ProviderConfig{Provider: core.ProviderGroq},
		capabilities: ProviderCapabilities{},
		buildModel: func(context.Context, *core.ProviderConfig, *openai.ResponseFormat) (llms.Model, error) {
			return stubModel{}, nil
		},
		modelCache:  map[string]llms.Model{"default": stubModel{}},
		errorParser: NewErrorParser(string(core.ProviderGroq)),
	}

	resp, err := adapter.GenerateContent(
		context.Background(),
		&LLMRequest{Options: CallOptions{ForceJSON: true}},
	)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestLangChainAdapter_MapMessageRole(t *testing.T) {
	adapter := &LangChainAdapter{}

	tests := []struct {
		role     string
		expected llms.ChatMessageType
	}{
		{"system", llms.ChatMessageTypeSystem},
		{"user", llms.ChatMessageTypeHuman},
		{"assistant", llms.ChatMessageTypeAI},
		{"tool", llms.ChatMessageTypeTool},
		{"unknown", llms.ChatMessageTypeHuman}, // Default
	}

	for _, tt := range tests {
		t.Run("Should map role "+tt.role, func(t *testing.T) {
			t.Parallel()
			result := adapter.mapMessageRole(tt.role)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLangChainAdapter_BuildCallOptions(t *testing.T) {
	adapter := &LangChainAdapter{}

	t.Run("Should build options with temperature and max tokens", func(t *testing.T) {
		req := LLMRequest{
			Options: CallOptions{
				Temperature: 0.7,
				MaxTokens:   100,
			},
		}

		options := adapter.buildCallOptions(&req)

		assert.Len(t, options, 2)
		// Note: We can't easily test the actual values as they're wrapped in functions
	})

	t.Run("Should include tools when provided", func(t *testing.T) {
		req := LLMRequest{
			Tools: []ToolDefinition{
				{
					Name:        "test_tool",
					Description: "A test tool",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"input": map[string]any{"type": "string"},
						},
					},
				},
			},
			Options: CallOptions{
				ToolChoice: "auto",
			},
		}

		options := adapter.buildCallOptions(&req)

		// Should have WithTools and WithToolChoice
		assert.GreaterOrEqual(t, len(options), 2)
	})

	t.Run("Should set tool choice to none for all providers", func(t *testing.T) {
		// Test with OpenAI
		openAIAdapter := &LangChainAdapter{
			provider: core.ProviderConfig{Provider: core.ProviderOpenAI},
		}
		req := LLMRequest{Options: CallOptions{ToolChoice: "none"}}
		options := openAIAdapter.buildCallOptions(&req)
		assert.Len(t, options, 1, "OpenAI should only have WithToolChoice, not deprecated WithFunctionCallBehavior")
		// Test with Groq
		groqAdapter := &LangChainAdapter{
			provider: core.ProviderConfig{Provider: core.ProviderGroq},
		}
		groqOptions := groqAdapter.buildCallOptions(&req)
		assert.Len(t, groqOptions, 1, "Groq should only have WithToolChoice")
	})
}

func TestLangChainAdapter_ConvertTools(t *testing.T) {
	adapter := &LangChainAdapter{}

	t.Run("Should convert tool definitions", func(t *testing.T) {
		tools := []ToolDefinition{
			{
				Name:        "search",
				Description: "Search for information",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "Search query",
						},
					},
					"required": []string{"query"},
				},
			},
		}

		llmTools := adapter.convertTools(tools)

		require.Len(t, llmTools, 1)
		assert.Equal(t, "function", llmTools[0].Type)
		assert.NotNil(t, llmTools[0].Function)
		assert.Equal(t, "search", llmTools[0].Function.Name)
		assert.Equal(t, "Search for information", llmTools[0].Function.Description)
		assert.NotNil(t, llmTools[0].Function.Parameters)
	})
}

func TestLangChainAdapter_ConvertResponse(t *testing.T) {
	adapter := &LangChainAdapter{}

	t.Run("Should convert simple text response", func(t *testing.T) {
		langchainResp := &llms.ContentResponse{
			Choices: []*llms.ContentChoice{
				{
					Content: "Hello, world!",
				},
			},
		}

		resp, err := adapter.convertResponse(langchainResp)

		require.NoError(t, err)
		assert.Equal(t, "Hello, world!", resp.Content)
		assert.Empty(t, resp.ToolCalls)
	})

	t.Run("Should convert response with tool calls", func(t *testing.T) {
		langchainResp := &llms.ContentResponse{
			Choices: []*llms.ContentChoice{
				{
					Content: "",
					ToolCalls: []llms.ToolCall{
						{
							ID: "call_123",
							FunctionCall: &llms.FunctionCall{
								Name:      "search",
								Arguments: `{"query": "test"}`,
							},
						},
					},
				},
			},
		}

		resp, err := adapter.convertResponse(langchainResp)

		require.NoError(t, err)
		assert.Empty(t, resp.Content)
		require.Len(t, resp.ToolCalls, 1)
		assert.Equal(t, "call_123", resp.ToolCalls[0].ID)
		assert.Equal(t, "search", resp.ToolCalls[0].Name)
		assert.Equal(t, `{"query": "test"}`, string(resp.ToolCalls[0].Arguments))
	})

	// Note: Usage information is not supported by langchaingo ContentResponse
	// This is documented in the convertResponse method

	t.Run("Should return error for empty response", func(t *testing.T) {
		langchainResp := &llms.ContentResponse{
			Choices: []*llms.ContentChoice{},
		}

		resp, err := adapter.convertResponse(langchainResp)

		assert.ErrorContains(t, err, "empty response")
		assert.Nil(t, resp)
	})

	t.Run("Should return error for nil response", func(t *testing.T) {
		resp, err := adapter.convertResponse(nil)

		assert.ErrorContains(t, err, "empty response")
		assert.Nil(t, resp)
	})
}

func TestNewLangChainAdapter(t *testing.T) {
	t.Run("Should create adapter for mock provider", func(t *testing.T) {
		config := core.ProviderConfig{
			Provider: core.ProviderMock,
			Model:    "mock-model",
		}

		builderCalls := 0
		builder := func(
			_ context.Context,
			cfg *core.ProviderConfig,
			format *openai.ResponseFormat,
		) (llms.Model, error) {
			builderCalls++
			assert.Equal(t, &config, cfg)
			assert.Nil(t, format)
			return stubModel{}, nil
		}

		caps := ProviderCapabilities{StructuredOutput: true}
		adapter, err := NewLangChainAdapter(context.Background(), &config, builder, caps)

		require.NoError(t, err)
		assert.NotNil(t, adapter)
		assert.NotNil(t, adapter.baseModel)
		assert.Equal(t, config, adapter.provider)
		assert.Equal(t, caps, adapter.capabilities)
		assert.Equal(t, 1, builderCalls)
	})
}

func TestLangChainAdapter_StructuredOutputCapability(t *testing.T) {
	config := core.ProviderConfig{
		Provider: core.ProviderOpenAI,
		Model:    "gpt-test",
	}

	var formats []*openai.ResponseFormat
	builder := func(
		_ context.Context,
		_ *core.ProviderConfig,
		format *openai.ResponseFormat,
	) (llms.Model, error) {
		if format != nil {
			formats = append(formats, format)
		}
		return stubModel{}, nil
	}

	adapter, err := NewLangChainAdapter(
		context.Background(),
		&config,
		builder,
		ProviderCapabilities{StructuredOutput: true},
	)
	require.NoError(t, err)

	format := NewJSONSchemaOutputFormat("result", &schema.Schema{"type": "object"}, true)
	_, err = adapter.ensureModel(context.Background(), format)
	require.NoError(t, err)
	require.Len(t, formats, 1)
	assert.Equal(t, "json_schema", formats[0].Type)

	adapterNoJSON, err := NewLangChainAdapter(
		context.Background(),
		&config,
		builder,
		ProviderCapabilities{},
	)
	require.NoError(t, err)
	_, err = adapterNoJSON.ensureModel(context.Background(), format)
	require.NoError(t, err)
	assert.Len(t, formats, 1)
}

func TestTestAdapter(t *testing.T) {
	t.Run("Should record calls and return configured response", func(t *testing.T) {
		adapter := NewTestAdapter()
		adapter.SetResponse("Test content", ToolCall{
			ID:        "test_call",
			Name:      "test_tool",
			Arguments: json.RawMessage("{}"),
		})

		req := LLMRequest{
			SystemPrompt: "Test prompt",
			Messages: []Message{
				{Role: "user", Content: "Test message"},
			},
		}

		resp, err := adapter.GenerateContent(context.Background(), &req)

		require.NoError(t, err)
		assert.Equal(t, "Test content", resp.Content)
		assert.Len(t, resp.ToolCalls, 1)
		assert.Equal(t, "test_tool", resp.ToolCalls[0].Name)

		// Check recorded call
		assert.Len(t, adapter.Calls, 1)
		lastCall := adapter.GetLastCall()
		assert.NotNil(t, lastCall)
		assert.Equal(t, "Test prompt", lastCall.SystemPrompt)
	})

	t.Run("Should return configured error", func(t *testing.T) {
		adapter := NewTestAdapter()
		adapter.SetError(assert.AnError)

		resp, err := adapter.GenerateContent(context.Background(), &LLMRequest{})

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, assert.AnError, err)
	})

	t.Run("Should reset state", func(t *testing.T) {
		adapter := NewTestAdapter()
		adapter.SetResponse("Test")
		adapter.GenerateContent(context.Background(), &LLMRequest{})

		adapter.Reset()

		assert.Empty(t, adapter.Calls)
		assert.Nil(t, adapter.Response)
		assert.Nil(t, adapter.Error)
	})
}

func TestMockToolAdapter(t *testing.T) {
	t.Run("Should simulate tool calling", func(t *testing.T) {
		adapter := NewMockToolAdapter()
		adapter.SetToolResult("search", "Search results")

		req := LLMRequest{
			Tools: []ToolDefinition{
				{Name: "search", Description: "Search tool"},
			},
		}

		resp, err := adapter.GenerateContent(context.Background(), &req)

		require.NoError(t, err)
		assert.Empty(t, resp.Content)
		require.Len(t, resp.ToolCalls, 1)
		assert.Equal(t, "search", resp.ToolCalls[0].Name)
		assert.Equal(t, "call_search", resp.ToolCalls[0].ID)
	})

	t.Run("Should return default response when no matching tool", func(t *testing.T) {
		adapter := NewMockToolAdapter()

		req := LLMRequest{
			Tools: []ToolDefinition{
				{Name: "unknown", Description: "Unknown tool"},
			},
		}

		resp, err := adapter.GenerateContent(context.Background(), &req)

		require.NoError(t, err)
		assert.Equal(t, "Mock response", resp.Content)
		assert.Empty(t, resp.ToolCalls)
	})
}

type stubModel struct{}

func (stubModel) GenerateContent(
	context.Context,
	[]llms.MessageContent,
	...llms.CallOption,
) (*llms.ContentResponse, error) {
	return &llms.ContentResponse{
		Choices: []*llms.ContentChoice{{Content: "stub"}},
	}, nil
}

func (stubModel) Call(context.Context, string, ...llms.CallOption) (string, error) {
	return "", nil
}
