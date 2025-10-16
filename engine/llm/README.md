# `llm` – _LLM integration layer for AI agent orchestration_

> **Comprehensive LLM service providing agent orchestration, tool execution, memory management, and multi-provider support with clean architecture patterns.**

---

## 📑 Table of Contents

- [`llm` – _LLM integration layer for AI agent orchestration_](#llm--llm-integration-layer-for-ai-agent-orchestration)
  - [📑 Table of Contents](#-table-of-contents)
  - [🎯 Overview](#-overview)
  - [💡 Motivation](#-motivation)
  - [⚡ Design Highlights](#-design-highlights)
  - [🚀 Getting Started](#-getting-started)
    - [Prerequisites](#prerequisites)
    - [Quick Setup](#quick-setup)
  - [📖 Usage](#-usage)
    - [Service Setup](#service-setup)
    - [Agent Configuration](#agent-configuration)
    - [Tool Management](#tool-management)
    - [Memory Integration](#memory-integration)
    - [Custom Providers](#custom-providers)
  - [🔧 Configuration](#-configuration)
    - [LLM Service Configuration](#llm-service-configuration)
    - [Configuration Options](#configuration-options)
  - [🎨 Examples](#-examples)
    - [Basic Agent Execution](#basic-agent-execution)
    - [Tool-Enabled Agents](#tool-enabled-agents)
    - [Memory-Aware Agents](#memory-aware-agents)
    - [Multi-Provider Setup](#multi-provider-setup)
  - [📚 API Reference](#-api-reference)
    - [Service](#service)
    - [Orchestrator](#orchestrator)
    - [Config](#config)
    - [Tool Registry](#tool-registry)
    - [Memory Integration](#memory-integration-1)
    - [Configuration Options](#configuration-options-1)
    - [Error Types](#error-types)
  - [🧪 Testing](#-testing)
    - [Unit Testing](#unit-testing)
    - [Integration Testing](#integration-testing)
    - [Running Tests](#running-tests)
  - [📦 Contributing](#-contributing)
  - [📄 License](#-license)

---

## 🎯 Overview

The `llm` package provides a comprehensive LLM integration layer for Compozy's AI agent orchestration system. It implements clean architecture patterns to manage LLM interactions, tool execution, memory management, and multi-provider support.

Key capabilities include:

- **Agent Orchestration**: Coordinate AI agents with custom instructions and actions
- **Tool Integration**: Execute both local and remote tools through MCP (Model Context Protocol)
- **Memory Management**: Persistent conversation memory with context integration
- **Multi-Provider Support**: OpenAI, Anthropic, Google, Cerebras, and other LLM providers
- **Structured Output**: JSON schema validation and structured response parsing
- **Async Operations**: Non-blocking memory storage and tool execution

---

## 💡 Motivation

- **Agent Abstraction**: Provide a clean interface for AI agents without tight coupling to specific LLM providers
- **Tool Ecosystem**: Enable agents to extend capabilities through standardized tool interfaces
- **Memory Persistence**: Maintain conversation context across multiple interactions and sessions
- **Scalability**: Support concurrent agent execution with proper resource management
- **Flexibility**: Allow custom providers, tools, and memory backends through dependency injection

---

## ⚡ Design Highlights

- **Clean Architecture**: Separated concerns with interfaces, adapters, and dependency injection
- **Provider Agnostic**: Abstracted LLM interface supporting multiple providers through adapters
- **Tool Registry**: Centralized tool management with local and remote tool support
- **Memory Integration**: Pluggable memory providers with async storage for performance
- **Error Handling**: Comprehensive error types with detailed context and retry logic
- **Structured Output**: JSON schema validation with automatic parsing and type safety
- **Concurrent Safety**: Thread-safe operations with proper synchronization and resource management

---

## 🚀 Getting Started

### Prerequisites

- Go 1.21+ with generics support
- LLM provider API keys (OpenAI, Anthropic, etc.)
- Optional: MCP proxy server for remote tools

### Quick Setup

```go
package main

import (
    "context"
    "log"

    "github.com/compozy/compozy/engine/llm"
    "github.com/compozy/compozy/engine/agent"
    "github.com/compozy/compozy/engine/runtime"
)

func main() {
    ctx := context.Background()

    // Create runtime (manages tool execution)
    runtime, err := runtime.NewRuntime(ctx)
    if err != nil {
        log.Fatal(err)
    }

    // Configure agent
    agentConfig := &agent.Config{
        ID:           "assistant",
        Instructions: "You are a helpful AI assistant.",
        Config: agent.LLMConfig{
            Provider: "openai",
            Model:    "gpt-4",
            ApiKey:   "your-api-key",
        },
    }

    // Create LLM service
    service, err := llm.NewService(ctx, runtime, agentConfig)
    if err != nil {
        log.Fatal(err)
    }
    defer service.Close()

    // Create action
    action := &agent.ActionConfig{
        ID:     "greeting",
        Prompt: "Say hello to the user",
    }

    // Execute
    result, err := service.GenerateContent(ctx, agentConfig, action)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Response: %v", result)
}
```

---

## 📖 Usage

### Service Setup

The `Service` is the main entry point for LLM operations:

```go
// Basic service setup
service, err := llm.NewService(ctx, runtime, agentConfig)
if err != nil {
    return err
}

// Service with custom options
service, err := llm.NewService(ctx, runtime, agentConfig,
    llm.WithTimeout(30*time.Second),
    llm.WithMaxConcurrentTools(10),
    llm.WithMemoryProvider(memoryProvider),
)
if err != nil {
    return err
}
```

### Agent Configuration

Configure agents with specific instructions and LLM settings:

```go
agentConfig := &agent.Config{
    ID:           "data-analyst",
    Instructions: "You are a data analyst. Analyze data and provide insights.",
    Config: agent.LLMConfig{
        Provider:    "openai",
        Model:       "gpt-4",
        ApiKey:      "your-api-key",
        Temperature: 0.7,
        MaxTokens:   2000,
    },
    Tools: []tool.Config{
        {
            ID:          "calculator",
            Name:        "Calculator",
            Description: "Perform mathematical calculations",
        },
    },
}
```

### Tool Management

The tool registry manages both local and remote tools:

```go
// Tools are automatically registered from agent configuration
// Local tools are executed by the runtime
// Remote tools are proxied through MCP

// Check tool availability
toolRegistry := service.GetToolRegistry()
tool, found := toolRegistry.Find(ctx, "calculator")
if found {
    result, err := tool.Call(ctx, `{"expression": "2 + 2"}`)
    if err != nil {
        return err
    }
}
```

### Memory Integration

Enable persistent memory for conversation context:

```go
// Configure memory in agent
agentConfig.Memory = []core.MemoryReference{
    {
        ID:  "conversation",
        Key: "user-session-{{.user_id}}",
    },
}

// Memory provider implementation
type MyMemoryProvider struct {
    store map[string]llm.Memory
}

func (m *MyMemoryProvider) GetMemory(ctx context.Context, memoryID, keyTemplate string) (llm.Memory, error) {
    // Resolve template and return memory instance
    resolvedKey := resolveTemplate(keyTemplate, ctx)
    return m.store[resolvedKey], nil
}

// Use memory provider
service, err := llm.NewService(ctx, runtime, agentConfig,
    llm.WithMemoryProvider(&MyMemoryProvider{...}),
)
```

### Custom Providers

Add support for custom LLM providers:

```go
// Create custom adapter
type CustomLLMAdapter struct {
    client *customapi.Client
}

func (c *CustomLLMAdapter) GenerateContent(ctx context.Context, req *llmadapter.LLMRequest) (*llmadapter.LLMResponse, error) {
    // Implementation for custom provider
    response, err := c.client.Generate(ctx, req)
    if err != nil {
        return nil, err
    }

    return &llmadapter.LLMResponse{
        Content:   response.Text,
        ToolCalls: response.ToolCalls,
    }, nil
}

// Register custom factory
factory := &llmadapter.CustomFactory{
    "custom-provider": func(config *agent.LLMConfig) (llmadapter.LLMClient, error) {
        return &CustomLLMAdapter{
            client: customapi.NewClient(config.ApiKey),
        }, nil
    },
}

service, err := llm.NewService(ctx, runtime, agentConfig,
    llm.WithLLMFactory(factory),
)
```

---

## 🔧 Configuration

### LLM Service Configuration

```go
type Config struct {
    ProxyURL         string                 // MCP proxy URL for remote tools
    MaxConcurrentTools int                  // Maximum concurrent tool executions
    ToolCaching      bool                   // Enable tool result caching
    CacheTTL         time.Duration          // Cache time-to-live
    StructuredOutput bool                   // Enable structured output parsing
    Timeout          time.Duration          // Request timeout
    LLMFactory       llmadapter.Factory     // Custom LLM provider factory
    RateLimiter      *llmadapter.RateLimiterRegistry // Shared provider concurrency limiter
    MemoryProvider   MemoryProvider         // Memory provider implementation
}
```

### Configuration Options

````go
// Available configuration options
llm.WithTimeout(30*time.Second)
llm.WithMaxConcurrentTools(10)
llm.WithToolCaching(true)
llm.WithCacheTTL(5*time.Minute)
llm.WithStructuredOutput(true)
llm.WithMemoryProvider(memoryProvider)
llm.WithLLMFactory(customFactory)
llm.WithProxyURL("http://mcp-proxy:3000")
// Admin token option has been removed

### Provider Rate Limiting

Per-provider concurrency controls smooth out spikes and honor upstream quotas. Global defaults live under `llm.rate_limiting`:

```yaml
llm:
  rate_limiting:
    enabled: true
    default_concurrency: 10      # max in-flight requests per provider
    default_queue_size: 100      # queued requests waiting for a slot
    per_provider_limits:
      groq:
        concurrency: 20
        queue_size: 150
      cerebras:
        concurrency: 12
````

Agent/provider configs can fine-tune limits with a `rate_limit` block:

```yaml
models:
  - provider: groq
    model: llama-3.1-8b
    rate_limit:
      concurrency: 8
      queue_size: 50
```

When providers return `Retry-After` hints, the orchestrator adopts that delay before retrying, preventing hot-looping on tight quotas.

````

---

## 🎨 Examples

### Basic Agent Execution

```go
func ExampleBasicAgent() {
    ctx := context.Background()

    // Setup
    runtime, _ := runtime.NewRuntime(ctx)
    agentConfig := &agent.Config{
        ID:           "assistant",
        Instructions: "You are a helpful assistant.",
        Config: agent.LLMConfig{
            Provider: "openai",
            Model:    "gpt-4",
            ApiKey:   "your-api-key",
        },
    }

    service, err := llm.NewService(ctx, runtime, agentConfig)
    if err != nil {
        log.Fatal(err)
    }
    defer service.Close()

    // Create action
    action := &agent.ActionConfig{
        ID:     "question",
        Prompt: "Explain quantum computing in simple terms",
    }

    // Execute
    result, err := service.GenerateContent(ctx, agentConfig, action)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Response: %v\n", result)
}
````

### Tool-Enabled Agents

```go
func ExampleToolAgent() {
    ctx := context.Background()

    // Agent with tools
    agentConfig := &agent.Config{
        ID:           "calculator-agent",
        Instructions: "You can perform calculations using the calculator tool.",
        Config: agent.LLMConfig{
            Provider: "openai",
            Model:    "gpt-4",
            ApiKey:   "your-api-key",
        },
        Tools: []tool.Config{
            {
                ID:          "calculator",
                Name:        "calculator",
                Description: "Perform mathematical calculations",
                InputSchema: &map[string]any{
                    "type": "object",
                    "properties": map[string]any{
                        "expression": {
                            "type":        "string",
                            "description": "Mathematical expression to evaluate",
                        },
                    },
                    "required": []string{"expression"},
                },
            },
        },
    }

    service, err := llm.NewService(ctx, runtime, agentConfig)
    if err != nil {
        log.Fatal(err)
    }

    // Action that will use tools
    action := &agent.ActionConfig{
        ID:     "math-problem",
        Prompt: "What is 15% of 240?",
    }

    result, err := service.GenerateContent(ctx, agentConfig, action)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Calculation result: %v\n", result)
}
```

### Memory-Aware Agents

```go
func ExampleMemoryAgent() {
    ctx := context.Background()

    // Simple memory implementation
    type SimpleMemory struct {
        id       string
        messages []llm.Message
    }

    func (s *SimpleMemory) GetID() string {
        return s.id
    }

    func (s *SimpleMemory) Append(ctx context.Context, msg llm.Message) error {
        s.messages = append(s.messages, msg)
        return nil
    }

    func (s *SimpleMemory) Read(ctx context.Context) ([]llm.Message, error) {
        return s.messages, nil
    }

    // Memory provider
    type SimpleMemoryProvider struct {
        memories map[string]*SimpleMemory
    }

    func (s *SimpleMemoryProvider) GetMemory(ctx context.Context, memoryID, keyTemplate string) (llm.Memory, error) {
        if s.memories == nil {
            s.memories = make(map[string]*SimpleMemory)
        }

        key := memoryID + "-" + keyTemplate
        if memory, exists := s.memories[key]; exists {
            return memory, nil
        }

        memory := &SimpleMemory{
            id:       key,
            messages: []llm.Message{},
        }
        s.memories[key] = memory
        return memory, nil
    }

    // Agent with memory
    agentConfig := &agent.Config{
        ID:           "memory-agent",
        Instructions: "You remember previous conversations.",
        Config: agent.LLMConfig{
            Provider: "openai",
            Model:    "gpt-4",
            ApiKey:   "your-api-key",
        },
        Memory: []core.MemoryReference{
            {
                ID:  "conversation",
                Key: "user-123",
            },
        },
    }

    // Create service with memory provider
    service, err := llm.NewService(ctx, runtime, agentConfig,
        llm.WithMemoryProvider(&SimpleMemoryProvider{}),
    )
    if err != nil {
        log.Fatal(err)
    }

    // First interaction
    action1 := &agent.ActionConfig{
        ID:     "first-interaction",
        Prompt: "My favorite color is blue. Remember this.",
    }

    result1, err := service.GenerateContent(ctx, agentConfig, action1)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("First response: %v\n", result1)

    // Second interaction (will remember previous context)
    action2 := &agent.ActionConfig{
        ID:     "second-interaction",
        Prompt: "What's my favorite color?",
    }

    result2, err := service.GenerateContent(ctx, agentConfig, action2)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Second response: %v\n", result2)
}
```

### Multi-Provider Setup

```go
func ExampleMultiProvider() {
    ctx := context.Background()

    // Custom factory supporting multiple providers
    factory := llmadapter.NewDefaultFactory()

    // OpenAI agent
    openaiAgent := &agent.Config{
        ID:           "openai-agent",
        Instructions: "You are powered by OpenAI.",
        Config: agent.LLMConfig{
            Provider: "openai",
            Model:    "gpt-4",
            ApiKey:   "openai-key",
        },
    }

    // Anthropic agent
    anthropicAgent := &agent.Config{
        ID:           "anthropic-agent",
        Instructions: "You are powered by Anthropic.",
        Config: agent.LLMConfig{
            Provider: "anthropic",
            Model:    "claude-3-opus",
            ApiKey:   "anthropic-key",
        },
    }

    // Create services
    openaiService, err := llm.NewService(ctx, runtime, openaiAgent,
        llm.WithLLMFactory(factory),
    )
    if err != nil {
        log.Fatal(err)
    }

    anthropicService, err := llm.NewService(ctx, runtime, anthropicAgent,
        llm.WithLLMFactory(factory),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Use different providers for different tasks
    action := &agent.ActionConfig{
        ID:     "comparison",
        Prompt: "Explain the benefits of renewable energy",
    }

    // Get responses from both providers
    openaiResult, _ := openaiService.GenerateContent(ctx, openaiAgent, action)
    anthropicResult, _ := anthropicService.GenerateContent(ctx, anthropicAgent, action)

    fmt.Printf("OpenAI: %v\n", openaiResult)
    fmt.Printf("Anthropic: %v\n", anthropicResult)
}
```

---

## 📚 API Reference

### Service

```go
type Service struct {
    // Main LLM service managing orchestration
}

func NewService(ctx context.Context, runtime runtime.Runtime, agent *agent.Config, opts ...Option) (*Service, error)
func (s *Service) GenerateContent(ctx context.Context, agent *agent.Config, action *agent.ActionConfig) (*core.Output, error)
func (s *Service) InvalidateToolsCache(ctx context.Context)
func (s *Service) Close() error
```

### Orchestrator

```go
type Orchestrator interface {
    Execute(ctx context.Context, request Request) (*core.Output, error)
    Close() error
}

type Request struct {
    Agent  *agent.Config
    Action *agent.ActionConfig
}

func NewOrchestrator(config *OrchestratorConfig) Orchestrator
```

### Config

```go
type Config struct {
    ProxyURL           string
    MaxConcurrentTools int
    ToolCaching        bool
    CacheTTL           time.Duration
    StructuredOutput   bool
    Timeout            time.Duration
    LLMFactory         llmadapter.Factory
    MemoryProvider     MemoryProvider
}

func DefaultConfig() *Config
func (c *Config) Validate() error
```

### Tool Registry

```go
type ToolRegistry interface {
    Register(ctx context.Context, tool Tool) error
    Find(ctx context.Context, name string) (Tool, bool)
    List(ctx context.Context) []Tool
    InvalidateCache(ctx context.Context)
    Close() error
}

type Tool interface {
    Name() string
    Description() string
    Call(ctx context.Context, input string) (string, error)
}

func NewToolRegistry(config ToolRegistryConfig) ToolRegistry
```

### Memory Integration

```go
type MemoryProvider interface {
    GetMemory(ctx context.Context, memoryID string, keyTemplate string) (Memory, error)
}

type Memory interface {
    Append(ctx context.Context, msg Message) error
    Read(ctx context.Context) ([]Message, error)
    GetID() string
}

func PrepareMemoryContext(ctx context.Context, memories map[string]Memory, messages []adapter.Message) []adapter.Message
func StoreResponseInMemory(ctx context.Context, memories map[string]Memory, memoryRefs []core.MemoryReference, assistantResponse adapter.Message, userMessage adapter.Message) error
```

### Configuration Options

```go
type Option func(*Config)

func WithTimeout(timeout time.Duration) Option
func WithMaxConcurrentTools(maxTools int) Option
func WithToolCaching(enabled bool) Option
func WithCacheTTL(ttl time.Duration) Option
func WithStructuredOutput(enabled bool) Option
func WithMemoryProvider(provider MemoryProvider) Option
func WithLLMFactory(factory llmadapter.Factory) Option
func WithProxyURL(url string) Option
```

### Error Types

```go
type ToolError struct {
    Code     string
    Message  string
    ToolName string
    Details  map[string]any
}

func NewToolError(err error, code string, toolName string, details map[string]any) error
func NewValidationError(err error, field string, value any) error
func NewLLMError(err error, code string, details map[string]any) error
func IsToolExecutionError(result string) (*ToolError, bool)
```

---

## 🧪 Testing

### Unit Testing

```go
func TestService_GenerateContent(t *testing.T) {
    ctx := context.Background()

    // Mock runtime
    mockRuntime := &runtime.MockRuntime{}
    mockRuntime.On("ExecuteTool", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
        Return(&core.Output{"result": "success"}, nil)

    // Test agent
    agent := &agent.Config{
        ID:           "test-agent",
        Instructions: "Test instructions",
        Config: agent.LLMConfig{
            Provider: "test",
            Model:    "test-model",
        },
    }

    // Create service with test adapter
    service, err := llm.NewService(ctx, mockRuntime, agent)
    require.NoError(t, err)

    // Test action
    action := &agent.ActionConfig{
        ID:     "test-action",
        Prompt: "Test prompt",
    }

    // Execute
    result, err := service.GenerateContent(ctx, agent, action)
    require.NoError(t, err)
    assert.NotNil(t, result)

    // Verify expectations
    mockRuntime.AssertExpectations(t)
}
```

### Integration Testing

```go
func TestService_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    ctx := context.Background()

    // Real runtime setup
    runtime, err := runtime.NewRuntime(ctx)
    require.NoError(t, err)

    // Agent with real provider
    agent := &agent.Config{
        ID:           "integration-test",
        Instructions: "You are a test assistant",
        Config: agent.LLMConfig{
            Provider: "openai",
            Model:    "gpt-3.5-turbo",
            ApiKey:   os.Getenv("OPENAI_API_KEY"),
        },
    }

    service, err := llm.NewService(ctx, runtime, agent)
    require.NoError(t, err)
    defer service.Close()

    action := &agent.ActionConfig{
        ID:     "test",
        Prompt: "Say 'Hello, World!' exactly",
    }

    result, err := service.GenerateContent(ctx, agent, action)
    require.NoError(t, err)
    assert.Contains(t, fmt.Sprintf("%v", result), "Hello, World!")
}
```

### Running Tests

```bash
# Run unit tests
go test ./engine/llm/...

# Run with coverage
go test -cover ./engine/llm/...

# Run integration tests
go test -v -tags=integration ./engine/llm/...

# Run specific test
go test -v -run TestService_GenerateContent ./engine/llm/
```

---

## 📦 Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md) for development guidelines.

---

## 📄 License

BSL-1.1 License - see [LICENSE](../../LICENSE) for details.
