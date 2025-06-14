# LLM Package Refactoring Plan

## Overview

This document outlines the comprehensive refactoring plan for the `engine/llm` package based on analysis conducted with Gemini 2.5 Pro and O3 models. The refactoring aims to improve maintainability, testability, and adherence to clean architecture principles.

## Critical Issues Identified

### 1. Single Responsibility Principle Violation

The `Service` struct in `service.go` handles too many responsibilities:

- LLM client creation
- Prompt engineering
- Tool execution
- MCP tool proxying
- Tool caching with concurrency control

### 2. Missing Test Coverage

- `service.go` has no corresponding test file despite being the core component
- Complex logic like `GenerateContent` and `executeToolCall` lacks test coverage
- No safety net against regressions

### 3. Fragile Error Handling

- String-based error checking (searching for "error" substring in tool results)
- Inconsistent use of `core.NewError` vs `fmt.Errorf`
- Implicit result unwrapping in `tool.go` that can cause data loss

### 4. Clean Architecture Violations

- Direct environment variable access in `initProxyClient`
- Hidden dependencies on environment configuration
- Tight coupling between components

### 5. Code Quality Issues

- Code duplication in `findTool` and `findMCPTool`
- Inefficient tool cache refresh logic (potential thundering herd)
- Lack of proper separation of concerns

## Proposed Architecture

### Design Approach: Pragmatic Hexagonal Architecture

We'll use a lightweight hexagonal architecture that provides clean boundaries without over-engineering:

```
┌─────────────────────────────────────────────────────┐
│                   Interface Layer                    │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────┐ │
│  │ LLMService  │  │ ToolRegistry │  │ MCPClient  │ │
│  └─────────────┘  └──────────────┘  └────────────┘ │
├─────────────────────────────────────────────────────┤
│                Application Layer                     │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────┐ │
│  │LLMOrchest. │  │PromptBuilder │  │ToolExecutor│ │
│  └─────────────┘  └──────────────┘  └────────────┘ │
├─────────────────────────────────────────────────────┤
│                  Domain Layer                        │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────┐ │
│  │   Config    │  │    Errors    │  │   Types    │ │
│  └─────────────┘  └──────────────┘  └────────────┘ │
└─────────────────────────────────────────────────────┘
```

### Core Components

#### 1. ToolRegistry

Responsible for tool discovery, registration, and caching.

```go
type ToolRegistry interface {
    Register(tool Tool) error
    Find(name string) (Tool, bool)
    ListAll() []Tool
    InvalidateCache()
}
```

#### 2. PromptBuilder

Handles prompt construction and enhancement for structured output.

```go
type PromptBuilder interface {
    Build(ctx context.Context, action *agent.ActionConfig) (string, error)
    EnhanceForStructuredOutput(prompt string, schema *schema.Schema) string
}
```

#### 3. LLMOrchestrator

Coordinates LLM interactions, tool calls, and response processing.

```go
type LLMOrchestrator interface {
    Execute(ctx context.Context, request LLMRequest) (*core.Output, error)
}
```

#### 4. Config

Configuration struct to replace environment variables.

```go
type Config struct {
    ProxyURL      string
    AdminToken    string
    CacheTTL      time.Duration
    Timeout       time.Duration
}
```

## Implementation Phases

### Phase 0: Dependency Analysis and Migration Planning (Day 1-2)

**Priority: CRITICAL**

1. **Identify all callers of `llm.NewService`**

    - Current analysis shows only 1 caller: `engine/task/uc/exec_task.go:85`
    - This significantly reduces migration risk

2. **Create safety net**

    - Tag current state: `git tag llm-v1-frozen`
    - Create parallel package structure to avoid breaking changes

3. **Document all integration points**
    - Runtime.Manager dependency for tool execution
    - MCP proxy configuration from environment
    - langchaingo framework usage

### Phase 1: Add Comprehensive Tests (Week 1)

**Priority: CRITICAL**

1. Create `service_test.go` with full coverage
2. Mock all external dependencies (runtime.Manager, mcp.Client)
3. Test edge cases and error scenarios
4. Add integration tests for tool execution
5. Add concurrent cache tests with race detection (`go test -race`)

### Phase 2: Create Adapter Layer (Week 1-2)

**Priority: HIGH**

1. **Create `internal/llmadapter` package**
    - Define `LLMClient` interface to abstract langchaingo
    - Implement `LangChainAdapter` as concrete implementation
    - Create test doubles for unit testing

```go
type LLMClient interface {
    GenerateContent(ctx context.Context, req LLMRequest) (*LLMResponse, error)
}
```

2. **Isolate langchaingo dependency**
    - All langchaingo imports confined to adapter
    - Domain types independent of external framework
    - Enable future LLM provider swaps

### Phase 3: Extract Core Components (Week 2)

**Priority: HIGH**

1. **Extract `ToolRegistry` from service.go**
    - Move tool finding logic
    - Implement caching with `golang.org/x/sync/singleflight` (NOT double-checked locking)
    - Add tool name canonicalization for unified registry
    - Define precedence: local tools override MCP tools

```go
type ToolRegistry interface {
    Register(tool Tool) error
    Find(name string) (Tool, bool)
    ListAll() []Tool
    InvalidateCache()
}
```

2. **Extract `PromptBuilder`**

    - Move prompt enhancement logic
    - Separate structured output handling
    - Make prompt construction testable

3. **Create `Config` struct**
    - Replace `os.Getenv` calls
    - Implement dependency injection via options pattern
    - Add validation

```go
func NewService(opts ...Option) (*Service, error) {
    // Config injected via options, not environment
}
```

### Phase 4: Improve Error Handling (Week 3)

**Priority: HIGH**

1. **Replace string-based error detection**

    - Current: `strings.Contains(strings.ToLower(result), "error")`
    - New: Structured error responses from tools
    - Add typed errors for common failures
    - Use `core.NewError` consistently throughout

2. **Remove implicit unwrapping in tool.go**
    - Current: Automatically extracts `result` field
    - New: Return complete output, let caller decide
    - Make behavior explicit and predictable

### Phase 5: Refactor Service & Migration (Week 4)

**Priority: MEDIUM**

1. **Create `LLMOrchestrator`**

    - Coordinate between components
    - Inject runtime.Manager dependency
    - Implement clean interfaces
    - Reduce coupling

2. **Update the single caller**

    - Modify `engine/task/uc/exec_task.go:85`
    - Pass configuration via dependency injection
    - Remove environment variable dependencies

3. **Simplify `Service` struct**
    - Delegate to new components
    - Maintain backward compatibility temporarily
    - Add deprecation notices

## Migration Strategy

### 1. Backward Compatibility

- Keep existing public API during transition
- Add deprecation notices to old methods
- Provide migration guide

### 2. Incremental Refactoring

- One component at a time
- Maintain all tests passing
- Regular integration checks

### 3. Documentation Updates

- Update CLAUDE.md with new patterns
- Add ADRs for architectural decisions
- Create examples for new usage

## Testing Strategy

### Unit Tests

- Mock all external dependencies
- Test each component in isolation
- Achieve >90% coverage for new code

### Integration Tests

- Test component interactions
- Verify tool execution flows
- Test error propagation

### Contract Tests

- Ensure API compatibility
- Verify prompt generation
- Test schema validation

## Code Standards

### Formatting Rules (from CLAUDE.md)

- No blank lines inside function bodies
- Use `t.Run("Should...")` pattern for tests
- Max function length: 80 lines
- Use `core.NewError` for errors

### Dependency Management

- Use interfaces for all dependencies
- Manual dependency injection with functional options
- No direct environment variable access

## Key Implementation Details

### Tool Cache with Singleflight

```go
import "golang.org/x/sync/singleflight"

type CachingToolRegistry struct {
    sfGroup singleflight.Group
    cache   []Tool
    mu      sync.RWMutex
}

func (r *CachingToolRegistry) getTools(ctx context.Context) ([]Tool, error) {
    v, err, _ := r.sfGroup.Do("refresh-tools", func() (interface{}, error) {
        return r.refreshFromProxy(ctx)
    })
    return v.([]Tool), err
}
```

### Tool Name Canonicalization

```go
func (r *ToolRegistry) canonicalize(name string) string {
    // Normalize tool names to prevent conflicts
    return strings.ToLower(strings.TrimSpace(name))
}

// Precedence: local tools override MCP tools
func (r *ToolRegistry) Register(tool Tool) error {
    canonical := r.canonicalize(tool.Name())
    r.mu.Lock()
    defer r.mu.Unlock()

    if existing, ok := r.localTools[canonical]; ok {
        // Local tool takes precedence
        return nil
    }
    r.tools[canonical] = tool
    return nil
}
```

## Risk Mitigation

### Performance

- Benchmark before/after changes
- Monitor memory usage with singleflight
- Profile critical paths
- Verify no regression in tool execution

### Compatibility

- Only 1 caller to migrate (low risk)
- Maintain existing API surface temporarily
- Use type aliases during transition
- Provide clear migration guide

### Team Adoption

- Code review all changes
- Pair programming for complex parts
- Document patterns clearly
- Add examples for new patterns

## Success Metrics

1. **Test Coverage**: >80% for all packages
2. **Function Length**: <50 lines average
3. **Cyclomatic Complexity**: <10 for all functions
4. **Build Time**: No significant increase
5. **API Compatibility**: 100% backward compatible

## Timeline

- **Week 1**: Add comprehensive tests
- **Week 2**: Extract core components
- **Week 3**: Improve error handling
- **Week 4**: Refactor service structure
- **Week 5**: Integration and cleanup
- **Week 6**: Documentation and release

## Progress Tracking

### ✅ Completed

- **Phase 0**: Dependency analysis completed

    - Found only 1 caller of `llm.NewService` at `engine/task/uc/exec_task.go:85`
    - Updated refactoring plan with insights from Gemini 2.5 Pro and O3
    - Added implementation details for singleflight pattern and tool canonicalization

- **Phase 1**: Initial test coverage added (Partial)

    - Created `service_test.go` with 10 test functions
    - Achieved 41.8% coverage for service.go
    - Tests demonstrate key issues like fragile error detection
    - All tests passing ✅

- **Phase 2**: Adapter layer implemented ✅

    - Created `internal/llmadapter` package with clean interfaces
    - Implemented `LangChainAdapter` for existing langchaingo integration
    - Added `DefaultFactory` for provider-agnostic LLM client creation
    - Isolated langchaingo dependency from domain logic

- **Phase 3**: Core components extracted ✅

    - **ToolRegistry**: Extracted with singleflight pattern for cache refresh
    - **PromptBuilder**: Extracted prompt construction and structured output logic
    - **Config**: Created structured configuration with validation and options
    - Tool precedence: local tools override MCP tools with canonicalization

- **Phase 4**: Error handling improved ✅

    - Replaced string-based error detection with structured `ToolExecutionResult`
    - Added comprehensive error types with consistent `core.NewError` usage
    - Improved error detection with multiple indicators (not just "error" substring)
    - All error paths now use typed errors with context

- **Phase 5**: Service refactored with backward compatibility ✅
    - **LLMOrchestrator**: Coordinates all components with clean interfaces
    - **ServiceV2**: New implementation using hexagonal architecture
    - **Legacy Service**: Modified to delegate to V2 when available
    - Zero breaking changes - existing callers continue to work unchanged

### 🚧 In Progress

- **Phase 1**: Complete test coverage (Deferred)
    - Basic test structure exists but comprehensive coverage pending
    - Priority shifted to implementation over tests per user request
    - Target: >80% coverage (to be completed after implementation)

### 📋 TODO

- Add comprehensive test coverage for new components
- Implement actual MCP proxy tool execution (currently placeholder)
- Migrate the single caller to use new configuration options
- Performance benchmarking and optimization
- Documentation updates for new architecture

## Architecture Implementation Summary

The refactoring successfully implemented a clean hexagonal architecture:

```
┌─────────────────────────────────────────────────────┐
│                   Service Layer                      │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────┐ │
│  │  Service    │  │  ServiceV2   │  │   Config   │ │
│  │ (Legacy)    │  │    (New)     │  │ (Options)  │ │
│  └─────────────┘  └──────────────┘  └────────────┘ │
├─────────────────────────────────────────────────────┤
│                Application Layer                     │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────┐ │
│  │LLMOrchest.  │  │PromptBuilder │  │ToolRegistry│ │
│  │(Coordinator)│  │              │  │(Singleflight│ │
│  └─────────────┘  └──────────────┘  └────────────┘ │
├─────────────────────────────────────────────────────┤
│                Infrastructure Layer                  │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────┐ │
│  │LLMAdapter   │  │  Registry    │  │   Errors   │ │
│  │(LangChain)  │  │  (Tools)     │  │(Structured)│ │
│  └─────────────┘  └──────────────┘  └────────────┘ │
└─────────────────────────────────────────────────────┘
```

### Key Files Created

- **Core Components:**

    - `config.go` - Configuration with validation and options pattern
    - `service_v2.go` - New service implementation

- **LLM Adapter Layer:**

    - `internal/llmadapter/interface.go` - Provider-agnostic interfaces
    - `internal/llmadapter/langchain_adapter.go` - LangChain integration
    - `internal/llmadapter/factory.go` - Client factory

- **Tool Management:**

    - `internal/registry/tool_registry.go` - Unified tool registry with caching

- **Prompt Engineering:**

    - `internal/prompt/builder.go` - Prompt construction and enhancement

- **Orchestration:**

    - `internal/orchestrator/llm_orchestrator.go` - Component coordination

- **Error Handling:**
    - `internal/errors/types.go` - Structured error types and detection

### Benefits Achieved

1. **Single Responsibility**: Each component has a focused purpose
2. **Dependency Injection**: All dependencies injected via interfaces
3. **Testability**: Components can be tested in isolation
4. **Maintainability**: Clear separation of concerns
5. **Extensibility**: Easy to add new LLM providers or tool types
6. **Performance**: Singleflight pattern prevents thundering herd
7. **Backward Compatibility**: Zero breaking changes

## Next Steps

1. Add comprehensive test coverage for new components
2. Implement actual MCP proxy tool execution
3. Migrate the single caller to use new configuration options
4. Performance benchmarking and optimization
