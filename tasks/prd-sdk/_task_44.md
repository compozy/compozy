## status: completed

<task_context>
<domain>sdk/examples</domain>
<type>documentation</type>
<scope>examples</scope>
<complexity>low</complexity>
<dependencies>sdk/project,sdk/workflow,sdk/agent,sdk/task</dependencies>
</task_context>

# Task 44.0: Example: Simple Workflow (S)

## Overview

Create the first runnable SDK example demonstrating basic workflow construction with context-first patterns. This serves as the "hello world" example for SDK users.

<critical>
- **ALWAYS READ** @.cursor/rules/go-coding-standards.mdc before start
- **ALWAYS READ** tasks/prd-sdk/05-examples.md (Example 1: Simple Workflow)
- **MUST** use context-first pattern: `Build(ctx)` everywhere
- **MUST** demonstrate proper error handling with BuildError
- **NEVER** use `context.Background()` - create proper context with logger/config
</critical>

<requirements>
- Runnable Go program in sdk/examples/01_simple_workflow.go
- Demonstrates: Project, Model, Workflow, Agent, BasicTask
- Context-first pattern throughout (logger/config from context)
- Proper error handling with BuildError
- Clear comments explaining SDK usage patterns
- README.md with run instructions
</requirements>

## Subtasks

- [x] 44.1 Create sdk/examples/01_simple_workflow.go
- [x] 44.2 Implement context setup (logger + config)
- [x] 44.3 Build model configuration
- [x] 44.4 Build agent with action and output schema
- [x] 44.5 Build workflow with basic task
- [x] 44.6 Build project configuration
- [x] 44.7 Add error handling examples
- [x] 44.8 Create README.md with run instructions
- [x] 44.9 Test example runs successfully

## Implementation Details

Per 05-examples.md section 1:

**Key components:**
- Context with logger and config attached
- Model builder with OpenAI GPT-4
- Agent with instructions and action
- Output schema with validation
- Basic task executing agent action
- Workflow orchestrating task
- Project containing all resources

**Pattern to demonstrate:**
```go
ctx := context.Background()
log := logger.New()
ctx = logger.WithLogger(ctx, log)
ctx = config.WithConfig(ctx, config.Load())

model, err := model.New("openai", "gpt-4").
    WithAPIKey(os.Getenv("OPENAI_API_KEY")).
    WithDefault(true).
    Build(ctx)  // ✅ Context required
```

### Relevant Files

- `sdk/examples/01_simple_workflow.go` - Main example
- `sdk/examples/README.md` - Instructions

### Dependent Files

- `sdk/project/builder.go` - Project builder
- `sdk/workflow/builder.go` - Workflow builder
- `sdk/agent/builder.go` - Agent builder
- `sdk/model/builder.go` - Model builder
- `sdk/task/basic.go` - BasicTask builder

## Deliverables

- [x] sdk/examples/01_simple_workflow.go (runnable)
- [x] sdk/examples/README.md with setup/run instructions
- [x] Clear comments explaining SDK patterns
- [x] Error handling examples
- [x] Verified example runs successfully

## Tests

From _tests.md:

- Example validation:
  - [x] Code compiles without errors
  - [x] Example runs successfully with valid env vars
  - [x] Error handling works (missing API key, invalid config)
  - [x] Output matches expected format
  - [x] All context-first patterns followed
  - [x] No global config or logger usage

## Success Criteria

- Example code is clean, well-commented, and runnable
- Demonstrates context-first pattern correctly
- Error handling shows BuildError aggregation
- README provides clear instructions
- Example runs end-to-end successfully
- Code passes `make lint`
