package llm

import (
	"context"
	"fmt"
	"testing"
	"time"

	_ "github.com/compozy/compozy/engine/tool/builtin/imports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

// mockRuntime is a mock implementation of runtime.Runtime for testing
type mockRuntime struct{}

func (m *mockRuntime) ExecuteTool(
	_ context.Context,
	_ string,
	_ core.ID,
	_ *core.Input,
	_ *core.Input,
	_ core.EnvMap,
) (*core.Output, error) {
	return &core.Output{}, nil
}

func (m *mockRuntime) ExecuteToolWithTimeout(
	_ context.Context,
	_ string,
	_ core.ID,
	_ *core.Input,
	_ *core.Input,
	_ core.EnvMap,
	_ time.Duration,
) (*core.Output, error) {
	return &core.Output{}, nil
}

func (m *mockRuntime) GetGlobalTimeout() time.Duration {
	return 60 * time.Second
}

func TestNewService(t *testing.T) {
	t.Run("Should create service with proper timeout configuration", func(t *testing.T) {
		runtimeMgr := &mockRuntime{}
		agentConfig := createTestAgentConfig()
		customTimeout := 45 * time.Second

		service, err := NewService(context.Background(), runtimeMgr, agentConfig, WithTimeout(customTimeout))

		require.NoError(t, err)
		assert.Equal(t, customTimeout, service.config.Timeout)
		assert.True(t, service.config.EnableStructuredOutput) // Default should be true
	})

	t.Run("Should register builtin tools when native tools enabled", func(t *testing.T) {
		runtimeMgr := &mockRuntime{}
		agentConfig := createTestAgentConfig()
		ctx := context.Background()
		ctx = logger.ContextWithLogger(ctx, logger.NewLogger(logger.TestConfig()))
		manager := appconfig.NewManager(appconfig.NewService())
		_, err := manager.Load(ctx, appconfig.NewDefaultProvider())
		require.NoError(t, err)
		ctx = appconfig.ContextWithManager(ctx, manager)
		service, err := NewService(ctx, runtimeMgr, agentConfig)
		require.NoError(t, err)
		t.Cleanup(func() { _ = service.Close() })
		toolEntry, ok := service.toolRegistry.Find(ctx, "cp__read_file")
		require.True(t, ok)
		assert.Equal(t, "cp__read_file", toolEntry.Name())
		callAgentEntry, callAgentFound := service.toolRegistry.Find(ctx, "cp__call_agent")
		require.True(t, callAgentFound)
		assert.Equal(t, "cp__call_agent", callAgentEntry.Name())
	})

	t.Run("Should skip builtin registration when disabled", func(t *testing.T) {
		runtimeMgr := &mockRuntime{}
		agentConfig := createTestAgentConfig()
		ctx := context.Background()
		ctx = logger.ContextWithLogger(ctx, logger.NewLogger(logger.TestConfig()))
		manager := appconfig.NewManager(appconfig.NewService())
		_, err := manager.Load(ctx, appconfig.NewDefaultProvider())
		require.NoError(t, err)
		cfg := manager.Get()
		cfg.Runtime.NativeTools.Enabled = false
		ctx = appconfig.ContextWithManager(ctx, manager)
		service, err := NewService(ctx, runtimeMgr, agentConfig)
		require.NoError(t, err)
		t.Cleanup(func() { _ = service.Close() })
		_, ok := service.toolRegistry.Find(ctx, "cp__read_file")
		assert.False(t, ok)
	})

	t.Run("Should error when runtime tool uses reserved cp prefix", func(t *testing.T) {
		runtimeMgr := &mockRuntime{}
		agentConfig := createTestAgentConfig()
		agentConfig.Tools = append(agentConfig.Tools, tool.Config{ID: "cp__custom"})
		ctx := context.Background()
		ctx = logger.ContextWithLogger(ctx, logger.NewLogger(logger.TestConfig()))
		manager := appconfig.NewManager(appconfig.NewService())
		_, err := manager.Load(ctx, appconfig.NewDefaultProvider())
		require.NoError(t, err)
		ctx = appconfig.ContextWithManager(ctx, manager)
		_, err = NewService(ctx, runtimeMgr, agentConfig)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cp__custom")
	})
}

// testFactory implements llmadapter.Factory for injecting a test client
type testFactory struct{ client llmadapter.LLMClient }

func (f testFactory) CreateClient(_ context.Context, _ *core.ProviderConfig) (llmadapter.LLMClient, error) {
	return f.client, nil
}

func (f testFactory) BuildRoute(
	cfg *core.ProviderConfig,
	fallbacks ...*core.ProviderConfig,
) (*llmadapter.ProviderRoute, error) {
	if cfg == nil {
		return nil, fmt.Errorf("provider config must not be nil")
	}
	registry := llmadapter.NewProviderRegistry()
	stub := &testProvider{name: cfg.Provider, client: f.client}
	if err := registry.Register(stub); err != nil {
		return nil, err
	}
	return registry.BuildRoute(cfg, fallbacks...)
}

func (f testFactory) Capabilities(name core.ProviderName) (llmadapter.ProviderCapabilities, error) {
	if name == core.ProviderOpenAI || name == core.ProviderXAI {
		return llmadapter.ProviderCapabilities{StructuredOutput: true, Streaming: true}, nil
	}
	return llmadapter.ProviderCapabilities{}, nil
}

type testProvider struct {
	name   core.ProviderName
	client llmadapter.LLMClient
}

func (p *testProvider) Name() core.ProviderName { return p.name }

func (p *testProvider) Capabilities() llmadapter.ProviderCapabilities {
	return llmadapter.ProviderCapabilities{}
}

func (p *testProvider) NewClient(context.Context, *core.ProviderConfig) (llmadapter.LLMClient, error) {
	return p.client, nil
}

// testClient wraps TestAdapter to satisfy LLMClient (adds Close)
type testClient struct{ *llmadapter.TestAdapter }

func (c testClient) Close() error { return nil }

func TestService_GenerateContent_DirectPrompt(t *testing.T) {
	t.Run("Should handle direct prompt without actionID", func(t *testing.T) {
		t.Parallel()
		runtimeMgr := &mockRuntime{}
		agentConfig := createTestAgentConfig()
		agentConfig.Instructions = "You are a helpful test agent"

		// Inject a test LLM client
		ta := llmadapter.NewTestAdapter()
		ta.SetResponse("Test response from direct prompt")
		service, err := NewService(
			context.Background(),
			runtimeMgr,
			agentConfig,
			WithLLMFactory(testFactory{client: testClient{ta}}),
		)
		require.NoError(t, err)
		t.Cleanup(func() { _ = service.Close() })

		out, err := service.GenerateContent(
			context.Background(),
			agentConfig,
			&core.Input{},
			"",
			"Analyze this text",
			[]llmadapter.ContentPart{},
		)
		require.NoError(t, err)
		require.NotNil(t, out)
		// Direct prompt returns text response parsed into {"response": ...}
		assert.Equal(t, "Test response from direct prompt", (*out)["response"])
	})

	t.Run("Should error when both actionID and prompt are empty", func(t *testing.T) {
		runtimeMgr := &mockRuntime{}
		agentConfig := createTestAgentConfig()
		agentConfig.Instructions = "Test agent"

		service, err := NewService(
			context.Background(),
			runtimeMgr,
			agentConfig,
			WithLLMFactory(testFactory{client: testClient{llmadapter.NewTestAdapter()}}),
		)
		require.NoError(t, err)
		t.Cleanup(func() { _ = service.Close() })

		_, err = service.GenerateContent(
			context.Background(),
			agentConfig,
			&core.Input{},
			"",
			"",
			[]llmadapter.ContentPart{},
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "either actionID or directPrompt must be provided")
	})

	t.Run("Should work with actionID for backward compatibility", func(t *testing.T) {
		runtimeMgr := &mockRuntime{}
		agentConfig := createTestAgentConfig()
		agentConfig.Instructions = "You are a helpful test agent"
		agentConfig.Actions = []*agent.ActionConfig{
			{
				ID:     "analyze",
				Prompt: "Analyze input: {{ .input.text }}",
				OutputSchema: &schema.Schema{
					"type": "object",
					"properties": map[string]any{
						"ok": map[string]any{"type": "boolean"},
					},
					"required": []string{"ok"},
				},
			},
		}

		ta := llmadapter.NewTestAdapter()
		ta.SetResponse(`{"ok":true}`)
		service, err := NewService(
			context.Background(),
			runtimeMgr,
			agentConfig,
			WithLLMFactory(testFactory{client: testClient{ta}}),
		)
		require.NoError(t, err)
		t.Cleanup(func() { _ = service.Close() })

		with := core.Input{"text": "hello"}
		out, err := service.GenerateContent(
			context.Background(),
			agentConfig,
			&with,
			"analyze",
			"",
			[]llmadapter.ContentPart{},
		)
		require.NoError(t, err)
		require.NotNil(t, out)
		assert.Equal(t, true, (*out)["ok"])
	})

	t.Run("Should support combined action and prompt for enhanced context", func(t *testing.T) {
		runtimeMgr := &mockRuntime{}
		agentConfig := createTestAgentConfig()
		agentConfig.Instructions = "You are a helpful test agent"
		agentConfig.Actions = []*agent.ActionConfig{
			{
				ID:     "analyze",
				Prompt: "Analyze the data",
				OutputSchema: &schema.Schema{
					"type": "object",
					"properties": map[string]any{
						"enhanced": map[string]any{"type": "boolean"},
						"focused":  map[string]any{"type": "boolean"},
					},
					"required": []string{"enhanced", "focused"},
				},
			},
		}

		ta := llmadapter.NewTestAdapter()
		ta.SetResponse(`{"enhanced":true, "focused":true}`)
		service, err := NewService(
			context.Background(),
			runtimeMgr,
			agentConfig,
			WithLLMFactory(testFactory{client: testClient{ta}}),
		)
		require.NoError(t, err)
		t.Cleanup(func() { _ = service.Close() })

		with := core.Input{"data": "test data"}
		// Provide both action and prompt for enhanced context
		out, err := service.GenerateContent(
			context.Background(),
			agentConfig,
			&with,
			"analyze",
			"Focus on security implications",
			[]llmadapter.ContentPart{},
		)
		require.NoError(t, err)
		require.NotNil(t, out)
		assert.Equal(t, true, (*out)["enhanced"])
		assert.Equal(t, true, (*out)["focused"])
	})
}

func TestService_applyKnowledgeOverrides(t *testing.T) {
	t.Run("Should apply runtime overrides to resolved knowledge bindings", func(t *testing.T) {
		t.Parallel()

		svc := &Service{
			knowledgeRuntimeEmbedders: map[string]*knowledge.EmbedderConfig{
				"openai_default": {
					ID:       "openai_default",
					APIKey:   "resolved-secret",
					Model:    "text-embedding-3-small",
					Provider: "openai",
					Config: knowledge.EmbedderRuntimeConfig{
						Dimension: 1536,
						BatchSize: 64,
					},
				},
			},
			knowledgeRuntimeVectorDBs: map[string]*knowledge.VectorDBConfig{
				"memory": {
					ID:   "memory",
					Type: knowledge.VectorDBTypeMemory,
					Config: knowledge.VectorDBConnConfig{
						Dimension: 1536,
					},
				},
			},
			knowledgeRuntimeKBs: map[string]*knowledge.BaseConfig{
				"kb": {
					ID:       "kb",
					Embedder: "openai_default",
					VectorDB: "memory",
				},
			},
		}
		binding := &knowledge.ResolvedBinding{
			ID: "binding",
			KnowledgeBase: knowledge.BaseConfig{
				ID:       "kb",
				Embedder: "openai_default",
				VectorDB: "memory",
			},
			Embedder: knowledge.EmbedderConfig{
				ID:       "openai_default",
				APIKey:   "{{ .env.OPENAI_API_KEY }}",
				Model:    "text-embedding-3-small",
				Provider: "openai",
				Config: knowledge.EmbedderRuntimeConfig{
					Dimension: 1536,
					BatchSize: 64,
				},
			},
			Vector: knowledge.VectorDBConfig{
				ID:   "memory",
				Type: knowledge.VectorDBTypeMemory,
				Config: knowledge.VectorDBConnConfig{
					Dimension: 1536,
				},
			},
			Retrieval: knowledge.RetrievalConfig{TopK: 5},
		}

		svc.applyKnowledgeOverrides(binding)

		assert.Equal(t, "resolved-secret", binding.Embedder.APIKey)
		assert.Equal(t, "memory", binding.Vector.ID)
		assert.Equal(t, "kb", binding.KnowledgeBase.ID)
	})
}

func TestBuildKnowledgeQuery(t *testing.T) {
	t.Parallel()

	t.Run("Should include prompt intent and input values when available", func(t *testing.T) {
		input := core.Input{
			"question": "What phases does NIST recommend for incident handling?",
			"context": map[string]any{
				"note": "Summarize key actions in each phase.",
			},
		}
		action := &agent.ActionConfig{
			Prompt: "Answer the question: {{ .input.question }}",
			With:   &input,
		}
		query := buildKnowledgeQuery(action)
		require.NotEmpty(t, query)
		assert.NotContains(t, query, "{{")
		assert.Contains(t, query, "Answer the question:")
		assert.Contains(t, query, "What phases does NIST recommend for incident handling?")
		assert.Contains(t, query, "Summarize key actions in each phase.")
	})

	t.Run("Should fall back to prompt when inputs are empty", func(t *testing.T) {
		action := &agent.ActionConfig{
			Prompt: "Outline the incident response lifecycle.",
		}
		query := buildKnowledgeQuery(action)
		assert.Equal(t, "Outline the incident response lifecycle.", query)
	})

	t.Run("Should ignore templated input values", func(t *testing.T) {
		input := core.Input{
			"question": "{{ .workflow.input.question }}",
		}
		action := &agent.ActionConfig{
			Prompt: "Provide a concise answer.",
			With:   &input,
		}
		query := buildKnowledgeQuery(action)
		assert.Equal(t, "Provide a concise answer.", query)
	})
}

// Helper functions for testing
func createTestAgentConfig() *agent.Config {
	return &agent.Config{
		ID:           "test-agent",
		Instructions: "Test instructions",
		LLMProperties: agent.LLMProperties{
			Tools: []tool.Config{},
			MCPs:  []mcp.Config{},
		},
		Model: agent.Model{Config: core.ProviderConfig{Provider: "test", Model: "test-model"}},
	}
}
