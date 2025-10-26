## status: pending

<task_context>
<domain>v2/examples</domain>
<type>documentation</type>
<scope>examples</scope>
<complexity>low</complexity>
<dependencies>v2/project,v2/workflow,v2/agent,v2/task</dependencies>
</task_context>

# Task 44.0: Example: Simple Workflow (S)

## Overview

Create the first runnable SDK example demonstrating basic workflow construction with context-first patterns. This serves as the "hello world" example for SDK users.

<critical>
- **ALWAYS READ** @.cursor/rules/go-coding-standards.mdc before start
- **ALWAYS READ** tasks/prd-modules/05-examples.md (Example 1: Simple Workflow)
- **MUST** use context-first pattern: `Build(ctx)` everywhere
- **MUST** demonstrate proper error handling with BuildError
- **NEVER** use `context.Background()` - create proper context with logger/config
</critical>

<requirements>
- Runnable Go program in v2/examples/01_simple_workflow.go
- Demonstrates: Project, Model, Workflow, Agent, BasicTask
- Context-first pattern throughout (logger/config from context)
- Proper error handling with BuildError
- Clear comments explaining SDK usage patterns
- README.md with run instructions
</requirements>

## Subtasks

- [ ] 44.1 Create v2/examples/01_simple_workflow.go
- [ ] 44.2 Implement context setup (logger + config)
- [ ] 44.3 Build model configuration
- [ ] 44.4 Build agent with action and output schema
- [ ] 44.5 Build workflow with basic task
- [ ] 44.6 Build project configuration
- [ ] 44.7 Add error handling examples
- [ ] 44.8 Create README.md with run instructions
- [ ] 44.9 Test example runs successfully

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

- `v2/examples/01_simple_workflow.go` - Main example
- `v2/examples/README.md` - Instructions

### Dependent Files

- `v2/project/builder.go` - Project builder
- `v2/workflow/builder.go` - Workflow builder
- `v2/agent/builder.go` - Agent builder
- `v2/model/builder.go` - Model builder
- `v2/task/basic.go` - BasicTask builder

## Deliverables

- [ ] v2/examples/01_simple_workflow.go (runnable)
- [ ] v2/examples/README.md with setup/run instructions
- [ ] Clear comments explaining SDK patterns
- [ ] Error handling examples
- [ ] Verified example runs successfully

## Tests

From _tests.md:

- Example validation:
  - [ ] Code compiles without errors
  - [ ] Example runs successfully with valid env vars
  - [ ] Error handling works (missing API key, invalid config)
  - [ ] Output matches expected format
  - [ ] All context-first patterns followed
  - [ ] No global config or logger usage

## Success Criteria

- Example code is clean, well-commented, and runnable
- Demonstrates context-first pattern correctly
- Error handling shows BuildError aggregation
- README provides clear instructions
- Example runs end-to-end successfully
- Code passes `make lint`
