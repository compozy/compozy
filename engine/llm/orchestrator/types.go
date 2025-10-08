package orchestrator

import (
	"context"
	"time"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/llm/contracts"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
)

// AsyncHook provides hooks for monitoring async operations.
type AsyncHook interface {
	OnMemoryStoreComplete(err error)
}

// Request represents an orchestrator request.
type Request struct {
	Agent           *agent.Config
	Action          *agent.ActionConfig
	AttachmentParts []llmadapter.ContentPart
	Knowledge       []KnowledgeEntry
}

type KnowledgeEntry struct {
	BindingID string
	Retrieval knowledge.RetrievalConfig
	Contexts  []knowledge.RetrievedContext
}

// Config configures orchestrator behavior.
type Config struct {
	ToolRegistry                  ToolRegistry
	PromptBuilder                 PromptBuilder
	RuntimeManager                runtime.Runtime
	LLMFactory                    llmadapter.Factory
	MemoryProvider                contracts.MemoryProvider
	MemorySync                    MemorySync
	AsyncHook                     AsyncHook
	Timeout                       time.Duration
	MaxConcurrentTools            int
	MaxToolIterations             int
	MaxSequentialToolErrors       int
	StructuredOutputRetryAttempts int
	RetryAttempts                 int
	RetryBackoffBase              time.Duration
	RetryBackoffMax               time.Duration
	RetryJitter                   bool
	MaxConsecutiveSuccesses       int
	EnableProgressTracking        bool
	NoProgressThreshold           int
	ProjectRoot                   string
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
}

type ToolRegistry interface {
	Find(ctx context.Context, name string) (RegistryTool, bool)
	ListAll(ctx context.Context) ([]RegistryTool, error)
	Close() error
}

// PromptBuilder mirrors engine/llm/prompt_builder behavior but abstracted for orchestration.
type PromptBuilder interface {
	Build(ctx context.Context, action *agent.ActionConfig) (string, error)
	EnhanceForStructuredOutput(ctx context.Context, prompt string, schema *schema.Schema, hasTools bool) string
	ShouldUseStructuredOutput(provider string, action *agent.ActionConfig, tools []tool.Config) bool
}

// MemorySync defines minimal contract for multi-memory coordination.
type MemorySync interface {
	WithMultipleLocks(memoryIDs []string, fn func())
}
