# Product Requirements Document: Multi-Agent Parallel Execution

## status: pending

<task_context>
<domain>engine/tool/builtin</domain>
<type>enhancement|implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>agent_executor|runtime|configuration|concurrency</dependencies>
</task_context>

---

<role>
You are a senior Go engineer with deep expertise in concurrent systems, design patterns, and Go 1.25+ idioms. You will design and implement a **parallel multi-agent execution builtin** (`cp__call_agents`) that extends Compozy's existing single-agent execution (`cp__call_agent`) to support executing multiple agents concurrently with proper resource management, error handling, and observability.
</role>

---

<dependent_tasks>
- Base on existing builtin: `engine/tool/builtin/callagent/` (single agent execution)
- Reference config patterns: `pkg/config/native_tools.go` (NativeCallAgentConfig)
- Follow builtin registration: `engine/tool/builtin/registry.go`
- Agent executor interface: `engine/runtime/toolenv/interface.go`
- Integration test patterns: `test/integration/tool/call_agent_integration_test.go`
</dependent_tasks>

---

<context>
Currently, `cp__call_agent` executes a single agent synchronously. Users need to orchestrate multiple agents in workflows but have no way to execute them in parallel, forcing sequential chains of single-agent calls that:

- Increase latency (agents execute serially when they could run concurrently)
- Complicate workflow definitions (manual task parallelization)
- Reduce resource utilization (idle CPU during sequential waits)

**Solution:** Create `cp__call_agents` (plural) for parallel execution while keeping `cp__call_agent` (singular) for single-agent use cases. This follows the principle of least surprise and allows gradual adoption.
</context>

---

<scope>
MVP for **parallel multi-agent execution only** (no sequential mode):

### In Scope

1. **New Builtin Tool: `cp__call_agents`**
   - Accept array of agent configurations (each with individual parameters)
   - Execute all agents in parallel using Go routines
   - Wait for all agents to complete (no early cancellation on first error)
   - Return array of results preserving input order
   - Include both successful and failed agent results in response

2. **Configuration Integration**
   - Leverage existing `pkg/config` system for:
     - Max concurrent agents limit (`runtime.native_tools.call_agents.max_concurrent`)
     - Individual agent timeout (`runtime.native_tools.call_agents.default_timeout`)
     - Global enable/disable flag (`runtime.native_tools.call_agents.enabled`)
   - Follow `@.cursor/rules/global-config.mdc` standards

3. **Resource Management**
   - Use Go 1.25+ `sync.WaitGroup.Go()` for goroutine management
   - Respect context cancellation and timeouts
   - Apply semaphore pattern for max concurrent limit
   - Proper cleanup on context cancellation

4. **Error Handling**
   - Continue executing all agents even if some fail
   - Return partial results with success/failure status per agent
   - Propagate errors with proper codes (builtin.Code*)
   - Maintain observability through builtin telemetry

5. **Testing & Validation**
   - Unit tests for concurrent execution logic
   - Integration tests with mock agent executor
   - Error scenario coverage (timeouts, failures, cancellation)
   - Configuration validation tests

### Out of Scope (Future Enhancements)

- Sequential execution mode (use single `cp__call_agent` calls for this)
- Inter-agent data passing (pipeline pattern)
- Dynamic agent dependency resolution
- Agent execution retries
- Result streaming (future streaming support)
- Agent execution priority/ordering

---

## Goals

### Primary Objectives

1. **Reduce Execution Latency**: Enable parallel agent execution to reduce total workflow time
2. **Maintain System Stability**: Respect resource limits to prevent overload
3. **Preserve Observability**: Maintain full telemetry and error tracking
4. **Follow Project Standards**: Adhere to all coding standards and architecture patterns

### Success Metrics

- Parallel execution of N agents completes in ~O(1) time vs O(N) sequential
- Zero resource leaks under normal and error conditions
- 100% test coverage for concurrent logic paths
- All linter and test checks pass

---

## User Stories

### As a Workflow Author

**Story 1: Parallel Research Gathering**
```yaml
# I want to gather information from multiple research agents simultaneously
tools:
  - id: research_parallel
    name: cp__call_agents
    input:
      agents:
        - agent_id: researcher.academic
          action_id: gather
          with: { topic: "quantum computing" }
        - agent_id: researcher.industry
          action_id: gather
          with: { topic: "quantum computing" }
        - agent_id: researcher.patents
          action_id: gather
          with: { topic: "quantum computing" }
# Result: All three research tasks complete concurrently
```

**Story 2: Multi-Modal Analysis**
```yaml
# I want to analyze content through multiple specialized agents in parallel
tools:
  - id: analyze_content
    name: cp__call_agents
    input:
      agents:
        - agent_id: analyzer.sentiment
          prompt: "Analyze sentiment of the article"
          with: { content: "{{workflow.input.article}}" }
        - agent_id: analyzer.entities
          prompt: "Extract named entities"
          with: { content: "{{workflow.input.article}}" }
        - agent_id: analyzer.summary
          prompt: "Generate executive summary"
          with: { content: "{{workflow.input.article}}" }
# Result: Sentiment, entities, and summary generated concurrently
```

**Story 3: Handling Partial Failures**
```yaml
# I want all agent results even if some fail
# agents: [agent_a, agent_b, agent_c]
# Result: If agent_b fails, I still get results from agent_a and agent_c
# with clear indication of agent_b's failure
```

---

## Core Features

### Feature 1: Parallel Agent Execution

**What**: Execute multiple agents concurrently using Go routines
**Why**: Reduce total execution time from O(N) to O(1) for independent agents
**How**: 
- Use `sync.WaitGroup` with Go 1.25+ `.Go()` method
- Apply semaphore pattern for concurrency limit
- Collect results in slice preserving input order

**Requirements**:
1. Accept array of agent execution requests
2. Launch goroutines respecting max concurrent limit
3. Wait for all agents to complete
4. Return ordered results array
5. Handle context cancellation gracefully

### Feature 2: Configuration-Driven Resource Limits

**What**: Use `pkg/config` for all tunable parameters
**Why**: Allow operators to tune for their infrastructure
**How**: 
- Add config fields following `@.cursor/rules/global-config.mdc`
- Read via `config.FromContext(ctx)` (no globals)
- Provide sensible defaults (max_concurrent: 10, timeout: 60s)

**Requirements**:
1. Add `CallAgents` section to `NativeToolsConfig`
2. Register config fields in registry
3. Validate limits (max_concurrent >= 1, timeout > 0)
4. Document in config help text

### Feature 3: Comprehensive Result Reporting

**What**: Return detailed results for each agent execution
**Why**: Users need to know which agents succeeded/failed and why
**How**: Array of result objects with success status and error details

**Requirements**:
1. Preserve input array order in results
2. Include success boolean per agent
3. Include full response for successful executions
4. Include error details for failed executions
5. Include execution metadata (exec_id, duration, etc.)

### Feature 4: Observability & Telemetry

**What**: Maintain full visibility into parallel executions
**Why**: Debug issues and monitor system health
**How**: Integrate with existing builtin telemetry system

**Requirements**:
1. Record invocation metrics per `builtin.RecordInvocation`
2. Log execution start/completion per agent
3. Track parallel execution count
4. Report timeout and error rates
5. Use `logger.FromContext(ctx)` for all logging

---

## High-Level Technical Constraints

### Performance Requirements

- Support up to 50 concurrent agent executions (configurable)
- Individual agent timeout: 60 seconds default, 300 seconds max
- Total operation timeout: context-driven
- Memory: O(N) for N agents (no memory leaks)

### Concurrency Requirements

- Thread-safe result collection
- Proper goroutine cleanup on cancellation
- No goroutine leaks under any error condition
- Respect Go 1.25+ concurrency patterns

### Compatibility Requirements

- Go 1.25.2 baseline (use `sync.WaitGroup.Go()`)
- Compatible with existing `toolenv.AgentExecutor` interface
- No breaking changes to existing `cp__call_agent`
- Follow project architecture patterns

### Security & Resource Protection

- Enforce max concurrent agents limit
- Respect individual and global timeouts
- Prevent resource exhaustion
- Validate all inputs per agent

---

## Non-Goals (Out of Scope)

### Explicitly Excluded

1. **Sequential Execution Mode**: Use multiple single `cp__call_agent` calls
2. **Agent Pipeline/Chaining**: Each agent executes independently
3. **Result Streaming**: Future enhancement when streaming support added
4. **Agent Dependency Resolution**: All agents execute in parallel
5. **Execution Priority**: All agents have equal priority
6. **Retry Logic**: Failures are reported; no automatic retries
7. **Early Cancellation**: All agents run to completion or timeout
8. **Backwards Compatibility**: Greenfield approach per project standards

---

## Backend Requirements

### Language & Stack

- **Language**: Go 1.25.2
- **Concurrency**: Use `sync.WaitGroup.Go()` and `golang.org/x/sync/semaphore`
- **Config**: `pkg/config` with context-based access
- **Logging**: `pkg/logger` with context-based access
- **Testing**: `testify` for assertions and mocks
- **Architecture**: Follow `@.cursor/rules/architecture.mdc`

### Package Structure

```
engine/tool/builtin/callagents/  # New package (note plural)
├── handler.go         # Main handler with parallel execution logic
├── handler_test.go    # Unit tests for handler
├── schema.go          # Input/output JSON schemas
├── types.go           # Request/response types
└── executor.go        # Parallel execution coordinator
```

### Core Implementation Components

#### 1. Input Schema & Types

```go
// Input: array of agent execution configs
type AgentExecutionRequest struct {
    AgentID   string         `json:"agent_id"   mapstructure:"agent_id"`
    ActionID  string         `json:"action_id"  mapstructure:"action_id"`
    Prompt    string         `json:"prompt"     mapstructure:"prompt"`
    With      map[string]any `json:"with"       mapstructure:"with"`
    TimeoutMs int            `json:"timeout_ms" mapstructure:"timeout_ms"`
}

type handlerInput struct {
    Agents []AgentExecutionRequest `json:"agents" mapstructure:"agents"`
}
```

#### 2. Output Schema & Types

```go
// Output: array preserving input order
type AgentExecutionResult struct {
    Success  bool           `json:"success"`
    AgentID  string         `json:"agent_id"`
    ActionID string         `json:"action_id,omitempty"`
    ExecID   string         `json:"exec_id,omitempty"`
    Response map[string]any `json:"response,omitempty"`
    Error    *ErrorDetails  `json:"error,omitempty"`
    Duration int64          `json:"duration_ms"`
}

type ErrorDetails struct {
    Message string `json:"message"`
    Code    string `json:"code"`
}

type handlerOutput struct {
    Results       []AgentExecutionResult `json:"results"`
    TotalCount    int                    `json:"total_count"`
    SuccessCount  int                    `json:"success_count"`
    FailureCount  int                    `json:"failure_count"`
    TotalDuration int64                  `json:"total_duration_ms"`
}
```

#### 3. Configuration Integration

**File**: `pkg/config/native_tools.go`

```go
// Add to NativeToolsConfig
type NativeToolsConfig struct {
    // ... existing fields
    CallAgent  NativeCallAgentConfig  `koanf:"call_agent"  ...`
    CallAgents NativeCallAgentsConfig `koanf:"call_agents" ...` // New
}

type NativeCallAgentsConfig struct {
    Enabled        bool          `koanf:"enabled" ...`
    DefaultTimeout time.Duration `koanf:"default_timeout" ...`
    MaxConcurrent  int           `koanf:"max_concurrent" ...`
}
```

**File**: `pkg/config/definition/schema.go`

```go
func registerRuntimeNativeToolsCallAgentsFields(registry *Registry) {
    registry.Register(&FieldDef{
        Path:    "runtime.native_tools.call_agents.enabled",
        Default: true,
        EnvVar:  "RUNTIME_NATIVE_TOOLS_CALL_AGENTS_ENABLED",
        Type:    reflect.TypeOf(true),
        Help:    "Enable cp__call_agents builtin for parallel execution",
    })
    registry.Register(&FieldDef{
        Path:    "runtime.native_tools.call_agents.default_timeout",
        Default: 60 * time.Second,
        EnvVar:  "RUNTIME_NATIVE_TOOLS_CALL_AGENTS_DEFAULT_TIMEOUT",
        Type:    durationType,
        Help:    "Default timeout per agent in parallel execution",
    })
    registry.Register(&FieldDef{
        Path:    "runtime.native_tools.call_agents.max_concurrent",
        Default: 10,
        EnvVar:  "RUNTIME_NATIVE_TOOLS_CALL_AGENTS_MAX_CONCURRENT",
        Type:    reflect.TypeOf(0),
        Help:    "Maximum concurrent agents in parallel execution",
    })
}
```

#### 4. Parallel Execution Logic

**File**: `engine/tool/builtin/callagents/executor.go`

```go
// Execute agents in parallel with resource limits
func executeAgentsParallel(
    ctx context.Context,
    env toolenv.Environment,
    requests []toolenv.AgentRequest,
    maxConcurrent int,
) []AgentExecutionResult {
    // Use semaphore for concurrency limit
    sem := semaphore.NewWeighted(int64(maxConcurrent))
    results := make([]AgentExecutionResult, len(requests))
    var wg sync.WaitGroup
    
    for i, req := range requests {
        i, req := i, req // Capture loop variables
        wg.Go(func() {
            // Acquire semaphore
            if err := sem.Acquire(ctx, 1); err != nil {
                results[i] = buildErrorResult(req, err)
                return
            }
            defer sem.Release(1)
            
            // Execute agent
            results[i] = executeSingleAgent(ctx, env, req)
        })
    }
    
    wg.Wait()
    return results
}
```

#### 5. Handler Registration

**File**: `engine/tool/builtin/callagents/handler.go`

```go
const toolID = "cp__call_agents"

func init() {
    native.RegisterProvider(Definition)
}

func Definition(env toolenv.Environment) builtin.BuiltinDefinition {
    return builtin.BuiltinDefinition{
        ID:           toolID,
        Description:  "Execute multiple agents in parallel.",
        InputSchema:  &inputSchema,
        OutputSchema: &outputSchema,
        Handler:      newHandler(env),
    }
}
```

### Error Handling & Validation

#### Input Validation

1. **Array validation**: 
   - Minimum 1 agent, maximum from config (default 50)
   - Each agent must have valid agent_id
   - Each agent must have action_id OR prompt
   - Timeout values must be non-negative

2. **Error codes** (reuse from `engine/tool/builtin/errors.go`):
   - `builtin.CodeInvalidArgument`: validation failures
   - `builtin.CodePermissionDenied`: tool disabled in config
   - `builtin.CodeInternal`: unexpected errors
   - `builtin.CodeDeadlineExceeded`: timeout errors

#### Error Response Pattern

```go
// Per-agent error in results
{
    "success": false,
    "agent_id": "researcher.academic",
    "error": {
        "message": "agent execution timeout",
        "code": "DEADLINE_EXCEEDED"
    },
    "duration_ms": 60000
}
```

### Resource Management

#### Goroutine Management

```go
// Use Go 1.25+ WaitGroup.Go() pattern
var wg sync.WaitGroup
for _, req := range requests {
    req := req // Capture
    wg.Go(func() {
        defer func() {
            if r := recover(); r != nil {
                log.Error("agent execution panic", "error", r)
            }
        }()
        // Execute agent
    })
}
wg.Wait()
```

#### Context Propagation

```go
// Respect context cancellation
for i, req := range requests {
    wg.Go(func() {
        select {
        case <-ctx.Done():
            results[i] = buildCancelledResult(req)
            return
        default:
            results[i] = executeSingleAgent(ctx, env, req)
        }
    })
}
```

#### Memory Management

- Pre-allocate result slice: `make([]Result, len(requests))`
- Use index-based writes (thread-safe for non-overlapping indices)
- No unbounded channels or buffers
- Proper cleanup on early return

### Logging & Observability

```go
func processRequest(ctx context.Context, env toolenv.Environment, payload map[string]any) (core.Output, string, int, string, error) {
    log := logger.FromContext(ctx)
    cfg := config.FromContext(ctx)
    
    log.Info("starting parallel agent execution",
        "agent_count", len(input.Agents),
        "max_concurrent", cfg.Runtime.NativeTools.CallAgents.MaxConcurrent,
    )
    
    start := time.Now()
    results := executeAgentsParallel(ctx, env, requests, cfg.Runtime.NativeTools.CallAgents.MaxConcurrent)
    duration := time.Since(start)
    
    successCount := countSuccessful(results)
    log.Info("parallel agent execution complete",
        "total", len(results),
        "success", successCount,
        "failed", len(results)-successCount,
        "duration_ms", duration.Milliseconds(),
    )
    
    return buildOutput(results), builtin.StatusSuccess, 0, "", nil
}
```

---

## Testing Requirements

### Unit Tests

**File**: `engine/tool/builtin/callagents/handler_test.go`

```go
func TestCallAgents_ParallelExecution(t *testing.T) {
    t.Run("Should execute multiple agents in parallel", func(t *testing.T) {
        // Test concurrent execution reduces total time
    })
    
    t.Run("Should respect max concurrent limit", func(t *testing.T) {
        // Test semaphore enforcement
    })
    
    t.Run("Should preserve result order", func(t *testing.T) {
        // Test results match input order
    })
    
    t.Run("Should include all results even with failures", func(t *testing.T) {
        // Test partial failure handling
    })
    
    t.Run("Should handle context cancellation", func(t *testing.T) {
        // Test graceful shutdown
    })
    
    t.Run("Should validate input array size", func(t *testing.T) {
        // Test limits enforcement
    })
}
```

### Integration Tests

**File**: `test/integration/tool/call_agents_integration_test.go`

```go
func TestCallAgentsIntegration(t *testing.T) {
    t.Run("Should execute agents via LLM service", func(t *testing.T) {
        // Full integration with mock agent executor
    })
    
    t.Run("Should respect configuration limits", func(t *testing.T) {
        // Test config integration
    })
}
```

### Test Coverage Requirements

- Unit test coverage: ≥ 90% for concurrent logic
- Integration test coverage: happy path + error scenarios
- Concurrent execution correctness (race detector)
- Memory leak detection (no goroutine leaks)

---

## Request Examples

### Example 1: Parallel Research

```json
{
  "agents": [
    {
      "agent_id": "researcher.academic",
      "action_id": "gather_research",
      "with": {
        "topic": "quantum computing",
        "depth": "comprehensive"
      }
    },
    {
      "agent_id": "researcher.industry",
      "action_id": "gather_research",
      "with": {
        "topic": "quantum computing",
        "depth": "comprehensive"
      }
    },
    {
      "agent_id": "researcher.patents",
      "action_id": "search_patents",
      "with": {
        "keywords": ["quantum", "computing", "qubit"]
      }
    }
  ]
}
```

**Response**:
```json
{
  "results": [
    {
      "success": true,
      "agent_id": "researcher.academic",
      "action_id": "gather_research",
      "exec_id": "exec_abc123",
      "response": {
        "papers": [...],
        "summary": "..."
      },
      "duration_ms": 2345
    },
    {
      "success": true,
      "agent_id": "researcher.industry",
      "action_id": "gather_research",
      "exec_id": "exec_def456",
      "response": {
        "reports": [...],
        "summary": "..."
      },
      "duration_ms": 3012
    },
    {
      "success": false,
      "agent_id": "researcher.patents",
      "error": {
        "message": "patent search service unavailable",
        "code": "UNAVAILABLE"
      },
      "duration_ms": 5000
    }
  ],
  "total_count": 3,
  "success_count": 2,
  "failure_count": 1,
  "total_duration_ms": 3012
}
```

### Example 2: Multi-Modal Analysis

```json
{
  "agents": [
    {
      "agent_id": "analyzer.sentiment",
      "prompt": "Analyze the sentiment and tone",
      "with": {
        "content": "{{article_content}}"
      }
    },
    {
      "agent_id": "analyzer.entities",
      "prompt": "Extract named entities",
      "with": {
        "content": "{{article_content}}"
      }
    },
    {
      "agent_id": "analyzer.summary",
      "action_id": "summarize",
      "with": {
        "content": "{{article_content}}",
        "max_length": 250
      },
      "timeout_ms": 30000
    }
  ]
}
```

### Example 3: Workflow YAML Usage

```yaml
# In workflow definition
tasks:
  - id: parallel_research
    type: native
    tool: cp__call_agents
    input:
      agents:
        - agent_id: researcher.web
          prompt: "Research latest trends"
          with:
            query: "{{workflow.input.topic}}"
        - agent_id: researcher.academic
          action_id: search_papers
          with:
            topic: "{{workflow.input.topic}}"
            years: [2023, 2024, 2025]
        - agent_id: researcher.news
          prompt: "Find recent news articles"
          with:
            topic: "{{workflow.input.topic}}"
```

---

## Acceptance Criteria

### Functional Requirements

- [ ] `cp__call_agents` builtin registered and discoverable
- [ ] Accepts array of agent execution requests
- [ ] Executes all agents in parallel using goroutines
- [ ] Respects `max_concurrent` configuration limit
- [ ] Respects individual agent timeouts
- [ ] Returns results array preserving input order
- [ ] Includes both successful and failed results
- [ ] Handles context cancellation gracefully
- [ ] No goroutine leaks under any condition

### Configuration Requirements

- [ ] Config fields registered in `pkg/config/definition/schema.go`
- [ ] Config struct added to `NativeToolsConfig`
- [ ] Config builder updated in `buildRuntimeConfig`
- [ ] Environment variables work correctly
- [ ] Config validation prevents invalid values
- [ ] `compozy config show` displays new fields

### Quality Requirements

- [ ] All unit tests pass (`make test`)
- [ ] All integration tests pass
- [ ] Linter passes (`make lint`)
- [ ] Race detector passes (`go test -race`)
- [ ] No goroutine leaks detected
- [ ] Code follows `@.cursor/rules/go-coding-standards.mdc`
- [ ] Functions under 50 lines
- [ ] Proper error handling with context

### Documentation Requirements

- [ ] Inline code documentation (godoc)
- [ ] Usage examples in documentation
- [ ] Configuration options documented
- [ ] Error codes documented
- [ ] Integration test serves as usage example

---

## Suggested Implementation Steps

### Phase 1: Configuration Setup

1. Add `NativeCallAgentsConfig` to `pkg/config/native_tools.go`
2. Register config fields in `pkg/config/definition/schema.go`
3. Update config builder in `pkg/config/config.go`
4. Add config tests in `pkg/config/config_test.go`
5. Verify: `compozy config show` displays new fields

### Phase 2: Core Implementation

1. Create `engine/tool/builtin/callagents/` package
2. Define input/output schemas in `schema.go`
3. Define types in `types.go`
4. Implement parallel executor in `executor.go`:
   - Semaphore-based concurrency control
   - Result collection with order preservation
   - Error handling per agent
5. Implement handler in `handler.go`:
   - Input validation
   - Config access via context
   - Telemetry integration
6. Register builtin in `init()`

### Phase 3: Testing

1. Write unit tests in `handler_test.go`:
   - Parallel execution correctness
   - Result order preservation
   - Concurrent limit enforcement
   - Context cancellation
   - Error handling
2. Write integration tests in `test/integration/tool/call_agents_integration_test.go`
3. Run race detector: `go test -race ./engine/tool/builtin/callagents/...`
4. Verify no goroutine leaks

### Phase 4: Integration

1. Update `engine/tool/builtin/imports/imports.go` to include new package
2. Add to system prompt template if needed
3. Create example workflow using `cp__call_agents`
4. Update documentation

### Phase 5: Validation

1. Run full test suite: `make test`
2. Run linter: `make lint`
3. Manual smoke test with real agents
4. Performance test: verify O(1) vs O(N) execution time
5. Load test: verify max_concurrent enforcement

---

## Best Practices

### Concurrency Patterns

**DO**:
- Use `sync.WaitGroup.Go()` (Go 1.25+) for goroutine management
- Use `golang.org/x/sync/semaphore` for concurrency limits
- Pre-allocate result slices with known size
- Use index-based writes for thread-safe result collection
- Handle panics in goroutines with defer/recover
- Respect context cancellation in all goroutines

**DON'T**:
- Use unbounded goroutines
- Forget to call `wg.Done()` or proper cleanup
- Share mutable state without synchronization
- Ignore context cancellation
- Create channel bottlenecks
- Leak goroutines on early return

### Error Handling

**DO**:
- Continue execution on individual agent failures
- Return partial results with error details
- Use proper error codes from `builtin` package
- Log errors with context
- Validate all inputs before execution
- Handle timeouts gracefully

**DON'T**:
- Stop execution on first error
- Lose successful results when some fail
- Use generic error messages
- Panic on errors
- Ignore validation errors
- Block indefinitely on errors

### Resource Management

**DO**:
- Enforce max concurrent limits
- Release semaphore in defer
- Clean up on context cancellation
- Use timeouts for all agent calls
- Monitor goroutine count
- Test for memory leaks

**DON'T**:
- Allow unlimited concurrency
- Hold resources longer than needed
- Ignore resource cleanup
- Run without timeouts
- Create unbounded buffers
- Forget to close resources

### Configuration

**DO**:
- Use `config.FromContext(ctx)` exclusively
- Provide sensible defaults
- Validate configuration values
- Document environment variables
- Follow naming conventions
- Test configuration edge cases

**DON'T**:
- Use global configuration
- Hard-code limits
- Skip validation
- Use magic numbers
- Forget environment variable mapping
- Make configuration optional where limits matter

### Testing

**DO**:
- Test concurrent execution correctness
- Test with race detector
- Test context cancellation
- Test timeout scenarios
- Test max concurrent enforcement
- Test result order preservation
- Test partial failure scenarios

**DON'T**:
- Skip concurrency tests
- Ignore race conditions
- Use sleeps for synchronization
- Test only happy paths
- Skip integration tests
- Forget edge cases

---

## Should Not

### Implementation Anti-Patterns

- **DON'T** change the existing `cp__call_agent` (singular) implementation
- **DON'T** use global variables for state or configuration
- **DON'T** pass logger or config as function parameters (use context)
- **DON'T** create functions longer than 50 lines
- **DON'T** use `context.Background()` in runtime code paths
- **DON'T** add backwards compatibility code (greenfield approach)
- **DON'T** use channels when simple index-based collection works
- **DON'T** implement sequential mode (out of scope)
- **DON'T** add inter-agent communication (out of scope)
- **DON'T** implement retry logic (out of scope)

### Testing Anti-Patterns

- **DON'T** use testify suite patterns (use direct `*testing.T`)
- **DON'T** skip race detector testing
- **DON'T** use arbitrary sleeps for timing
- **DON'T** test trivial getters/setters
- **DON'T** duplicate test coverage unnecessarily
- **DON'T** create weak assertions (`assert.Error` without details)

### Configuration Anti-Patterns

- **DON'T** access configuration globally
- **DON'T** hard-code resource limits
- **DON'T** skip environment variable registration
- **DON'T** forget CLI flag categorization
- **DON'T** omit validation for limits
- **DON'T** use magic numbers for timeouts/limits

---

## References

### Project Standards (MUST READ)

- **Go Coding Standards**: `@.cursor/rules/go-coding-standards.mdc`
- **Architecture Patterns**: `@.cursor/rules/architecture.mdc`
- **Testing Requirements**: `@.cursor/rules/test-standards.mdc`
- **Global Config**: `@.cursor/rules/global-config.mdc`
- **Logger Config**: `@.cursor/rules/logger-config.mdc`
- **Magic Numbers**: `@.cursor/rules/magic-numbers.mdc`
- **No Line Breaks**: `@.cursor/rules/no-linebreaks.mdc`

### Related Code

- **Current Implementation**: `engine/tool/builtin/callagent/handler.go`
- **Agent Executor**: `engine/agent/exec/runner.go`
- **Tool Environment**: `engine/runtime/toolenv/interface.go`
- **Config Structure**: `pkg/config/native_tools.go`
- **Builtin Registry**: `engine/tool/builtin/registry.go`
- **Integration Tests**: `test/integration/tool/call_agent_integration_test.go`

### Go Documentation

- **Go 1.25 sync.WaitGroup.Go**: https://pkg.go.dev/sync#WaitGroup.Go
- **Semaphore**: https://pkg.go.dev/golang.org/x/sync/semaphore
- **Context Package**: https://pkg.go.dev/context
- **Testing Package**: https://pkg.go.dev/testing

### External References

- **Concurrency Patterns**: https://go.dev/blog/pipelines
- **Error Handling**: https://go.dev/blog/error-handling-and-go
- **Effective Go**: https://go.dev/doc/effective_go

---

## Relevant Files

### Implementation Files (New)

- `engine/tool/builtin/callagents/handler.go`
- `engine/tool/builtin/callagents/handler_test.go`
- `engine/tool/builtin/callagents/schema.go`
- `engine/tool/builtin/callagents/types.go`
- `engine/tool/builtin/callagents/executor.go`
- `engine/tool/builtin/callagents/executor_test.go`

### Configuration Files (Modified)

- `pkg/config/native_tools.go`
- `pkg/config/definition/schema.go`
- `pkg/config/config.go`
- `pkg/config/config_test.go`
- `pkg/config/provider.go`

### Test Files (New)

- `test/integration/tool/call_agents_integration_test.go`

### Documentation Files (New)

- `docs/content/docs/core/tools/call-agents.mdx`
- `engine/tool/builtin/callagents/README.md`

---

## Dependent Files

### Existing Dependencies

- `engine/tool/builtin/callagent/handler.go` (reference implementation)
- `engine/runtime/toolenv/interface.go` (AgentExecutor interface)
- `engine/agent/exec/runner.go` (ExecuteAgent implementation)
- `engine/tool/builtin/definition.go` (builtin definition types)
- `engine/tool/builtin/errors.go` (error codes and helpers)
- `engine/tool/builtin/telemetry.go` (observability helpers)
- `engine/tool/native/registry.go` (registration mechanism)
- `pkg/logger/logger.go` (context-based logging)
- `pkg/config/manager.go` (configuration management)

---

## Output

In addition to the implementation, provide:

1. **Summary Document**: Brief implementation summary covering:
   - Architecture decisions made
   - Concurrency patterns used
   - Configuration options added
   - Testing approach and coverage
   - Performance characteristics

2. **Usage Examples**: Practical examples showing:
   - Simple parallel execution (2-3 agents)
   - Complex multi-modal analysis (5+ agents)
   - Handling partial failures
   - Configuration tuning
   - YAML workflow integration

3. **Migration Guide**: For users of existing `cp__call_agent`:
   - When to use `cp__call_agent` vs `cp__call_agents`
   - Converting sequential calls to parallel
   - Configuration recommendations
   - Performance expectations

4. **Troubleshooting Guide**: Common issues and solutions:
   - Timeout tuning
   - Concurrency limit adjustment
   - Memory optimization
   - Error debugging

---

<perplexity>
Before implementing, use Perplexity and Context7 to research:

1. **Go Concurrency Best Practices**:
   - Query: "Go 1.25+ sync.WaitGroup.Go method best practices and patterns for parallel task execution"
   - Query: "golang.org/x/sync/semaphore usage patterns for limiting concurrent goroutines"

2. **Error Aggregation Patterns**:
   - Query: "Go patterns for collecting errors from multiple goroutines while preserving order"

3. **Context Cancellation in Parallel Operations**:
   - Query: "Go context cancellation patterns for parallel goroutine execution with cleanup"

4. **Testing Concurrent Code**:
   - Query: "Go race detector and testing patterns for concurrent code with goroutines"

5. **Libraries (if needed)**:
   - Use Context7 to check: `golang.org/x/sync` for semaphore usage examples
   - Use Context7 to check: `github.com/stretchr/testify` for testing patterns

**Note**: When using Perplexity, provide detailed questions rather than keyword searches. For Context7, use two-step process: resolve library ID first, then fetch documentation.
</perplexity>

---

<greenfield>
**YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach. We don't need to care about backwards compatibility since the project is in alpha. Supporting old and new stuff just introduces more complexity. **NEVER sacrifice quality because of backwards compatibility.**

This means:
- Feel free to add new config fields without migration paths
- Create new packages without worrying about old patterns
- Use latest Go 1.25+ features without fallbacks
- Optimize for the best design, not compatibility
- Keep the codebase clean and focused
</greenfield>

---

<critical>
**MANDATORY PRE-IMPLEMENTATION CHECKLIST:**

- [ ] Read ALL referenced rule files in `.cursor/rules/`
- [ ] Understand existing `cp__call_agent` implementation completely
- [ ] Review Go 1.25+ `sync.WaitGroup.Go()` documentation
- [ ] Research `golang.org/x/sync/semaphore` usage patterns
- [ ] Understand context cancellation propagation
- [ ] Review project config management patterns
- [ ] Check existing builtin telemetry integration
- [ ] Verify test patterns and requirements

**DURING IMPLEMENTATION:**

- [ ] Use `config.FromContext(ctx)` - NEVER global config
- [ ] Use `logger.FromContext(ctx)` - NEVER pass logger as parameter
- [ ] Keep all functions under 50 lines
- [ ] Follow no-linebreaks formatting rules
- [ ] Use `t.Context()` in tests - NEVER `context.Background()`
- [ ] Run `make lint` and `make test` before considering complete
- [ ] Verify with race detector: `go test -race`
- [ ] Check for goroutine leaks

**COMPLETION CRITERIA:**

- [ ] `make lint` passes with zero warnings
- [ ] `make test` passes all tests
- [ ] Race detector passes: `go test -race ./engine/tool/builtin/callagents/...`
- [ ] Integration tests pass
- [ ] Configuration appears in `compozy config show`
- [ ] No goroutine leaks detected
- [ ] Code review against all mandatory standards
- [ ] Documentation complete and accurate

**YOU CAN ONLY mark this task complete if ALL criteria above pass.**
</critical>
