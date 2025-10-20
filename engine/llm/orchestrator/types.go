package orchestrator

import (
	"context"
	"time"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/llm/contracts"
	providermetrics "github.com/compozy/compozy/engine/llm/provider/metrics"
	"github.com/compozy/compozy/engine/llm/telemetry"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/tool"
)

// AsyncHook provides hooks for monitoring async operations.
type AsyncHook interface {
	OnMemoryStoreComplete(err error)
}

// Request represents an orchestrator request.
type Request struct {
	Agent     *agent.Config
	Action    *agent.ActionConfig
	Prompt    PromptPayload
	Knowledge KnowledgePayload
	Execution ExecutionOptions
}

// PromptPayload aggregates prompt attachments and dynamic metadata.
type PromptPayload struct {
	Attachments    []llmadapter.ContentPart
	DynamicContext PromptDynamicContext
}

// KnowledgePayload groups retrieved knowledge entries for prompt injection.
type KnowledgePayload struct {
	Entries []KnowledgeEntry
}

// ExecutionOptions captures provider-specific execution capabilities.
type ExecutionOptions struct {
	ProviderCaps llmadapter.ProviderCapabilities
}

// RequestBuildOutput captures the constructed LLM request and prompt metadata.
type RequestBuildOutput struct {
	Request        llmadapter.LLMRequest
	PromptTemplate PromptTemplateState
	PromptContext  PromptDynamicContext
}

// KnowledgeEntry bundles the binding metadata, retrieval settings, and resolved contexts for prompt injection.
type KnowledgeEntry struct {
	BindingID string
	Retrieval knowledge.RetrievalConfig
	Contexts  []knowledge.RetrievedContext
	Status    knowledge.RetrievalStatus
	Notice    string
}

// Config configures orchestrator behavior.
type Config struct {
	ToolRegistry                   ToolRegistry
	PromptBuilder                  PromptBuilder
	SystemPromptRenderer           SystemPromptRenderer
	RuntimeManager                 runtime.Runtime
	LLMFactory                     llmadapter.Factory
	ProviderMetrics                providermetrics.Recorder
	MemoryProvider                 contracts.MemoryProvider
	MemorySync                     MemorySync
	AsyncHook                      AsyncHook
	Timeout                        time.Duration
	MaxConcurrentTools             int
	MaxToolIterations              int
	MaxSequentialToolErrors        int
	StructuredOutputRetryAttempts  int
	RetryAttempts                  int
	RetryBackoffBase               time.Duration
	RetryBackoffMax                time.Duration
	RetryJitter                    bool
	RetryJitterMax                 time.Duration
	MaxConsecutiveSuccesses        int
	EnableProgressTracking         bool
	NoProgressThreshold            int
	EnableLoopRestarts             bool
	RestartStallThreshold          int
	MaxLoopRestarts                int
	EnableContextCompaction        bool
	ContextCompactionThreshold     float64
	ContextCompactionCooldown      int
	EnableAgentCallCompletionHints bool
	EnableDynamicPromptState       bool
	ToolCallCaps                   ToolCallCaps
	ToolSuggestionLimit            int
	Middlewares                    []Middleware
	FinalizeOutputRetryAttempts    int
	ProjectRoot                    string
	RateLimiter                    *llmadapter.RateLimiterRegistry
	TelemetryRecorder              telemetry.RunRecorder
	TelemetryOptions               *telemetry.Options
}

// Orchestrator coordinates LLM interactions, tool calls, and response processing.
type Orchestrator interface {
	Execute(ctx context.Context, request Request) (*core.Output, error)
	Close() error
}

// RegistryTool represents a callable tool resolved from the tool registry.
type RegistryTool interface {
	Name() string
	Description() string
	Call(ctx context.Context, input string) (string, error)
	ParameterSchema() map[string]any
}

type ToolRegistry interface {
	Find(ctx context.Context, name string) (RegistryTool, bool)
	ListAll(ctx context.Context) ([]RegistryTool, error)
	Close() error
}

// ToolCallCaps defines global and per-tool invocation limits enforced by the orchestrator.
type ToolCallCaps struct {
	Default   int
	Overrides map[string]int
}

// PromptExample represents a dynamic exemplar injected into the prompt template.
type PromptExample struct {
	Summary string
	Content string
}

// PromptDynamicContext conveys dynamic slots rendered into the prompt template.
type PromptDynamicContext struct {
	Examples        []PromptExample
	FailureGuidance []string
}

// PromptBuildInput carries all data required to render the user prompt template.
type PromptBuildInput struct {
	Action       *agent.ActionConfig
	ProviderCaps llmadapter.ProviderCapabilities
	Tools        []tool.Config
	Dynamic      PromptDynamicContext
}

// PromptTemplateState allows re-rendering the prompt with updated dynamic context.
type PromptTemplateState interface {
	Render(ctx context.Context, dynamic PromptDynamicContext) (string, error)
}

// PromptBuildResult contains the rendered prompt and associated template metadata.
type PromptBuildResult struct {
	Prompt   string
	Format   llmadapter.OutputFormat
	Template PromptTemplateState
	Context  PromptDynamicContext
}

// PromptBuilder renders user prompts using reusable templates.
type PromptBuilder interface {
	Build(ctx context.Context, input PromptBuildInput) (PromptBuildResult, error)
}

// SystemPromptRenderer renders the system prompt including built-in tool guidance.
type SystemPromptRenderer interface {
	Render(ctx context.Context, instructions string) (string, error)
}

// MemorySync defines minimal contract for multi-memory coordination.
type MemorySync interface {
	WithMultipleLocks(memoryIDs []string, fn func())
}

// Middleware exposes lifecycle hooks to extend orchestrator behavior.
type Middleware interface {
	BeforeLLMRequest(
		ctx context.Context,
		loopCtx *LoopContext,
		req *llmadapter.LLMRequest,
	) error
	AfterLLMResponse(
		ctx context.Context,
		loopCtx *LoopContext,
		resp *llmadapter.LLMResponse,
	) error
	BeforeToolExecution(
		ctx context.Context,
		loopCtx *LoopContext,
		call *llmadapter.ToolCall,
	) error
	AfterToolExecution(
		ctx context.Context,
		loopCtx *LoopContext,
		call *llmadapter.ToolCall,
		result *llmadapter.ToolResult,
	) error
	BeforeFinalize(ctx context.Context, loopCtx *LoopContext) error
}

// MiddlewareFuncs provides an adapter with optional hook implementations.
type MiddlewareFuncs struct {
	BeforeLLMRequestFn func(
		ctx context.Context,
		loopCtx *LoopContext,
		req *llmadapter.LLMRequest,
	) error
	AfterLLMResponseFn func(
		ctx context.Context,
		loopCtx *LoopContext,
		resp *llmadapter.LLMResponse,
	) error
	BeforeToolExecFn func(
		ctx context.Context,
		loopCtx *LoopContext,
		call *llmadapter.ToolCall,
	) error
	AfterToolExecFn func(
		ctx context.Context,
		loopCtx *LoopContext,
		call *llmadapter.ToolCall,
		result *llmadapter.ToolResult,
	) error
	BeforeFinalizeFn func(ctx context.Context, loopCtx *LoopContext) error
}

func (m MiddlewareFuncs) BeforeLLMRequest(
	ctx context.Context,
	loopCtx *LoopContext,
	req *llmadapter.LLMRequest,
) error {
	if m.BeforeLLMRequestFn != nil {
		return m.BeforeLLMRequestFn(ctx, loopCtx, req)
	}
	return nil
}

func (m MiddlewareFuncs) AfterLLMResponse(
	ctx context.Context,
	loopCtx *LoopContext,
	resp *llmadapter.LLMResponse,
) error {
	if m.AfterLLMResponseFn != nil {
		return m.AfterLLMResponseFn(ctx, loopCtx, resp)
	}
	return nil
}

func (m MiddlewareFuncs) BeforeToolExecution(
	ctx context.Context,
	loopCtx *LoopContext,
	call *llmadapter.ToolCall,
) error {
	if m.BeforeToolExecFn != nil {
		return m.BeforeToolExecFn(ctx, loopCtx, call)
	}
	return nil
}

func (m MiddlewareFuncs) AfterToolExecution(
	ctx context.Context,
	loopCtx *LoopContext,
	call *llmadapter.ToolCall,
	result *llmadapter.ToolResult,
) error {
	if m.AfterToolExecFn != nil {
		return m.AfterToolExecFn(ctx, loopCtx, call, result)
	}
	return nil
}

func (m MiddlewareFuncs) BeforeFinalize(ctx context.Context, loopCtx *LoopContext) error {
	if m.BeforeFinalizeFn != nil {
		return m.BeforeFinalizeFn(ctx, loopCtx)
	}
	return nil
}
