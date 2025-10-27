## status: completed

<task_context>
<domain>sdk/examples</domain>
<type>documentation</type>
<scope>examples</scope>
<complexity>low</complexity>
<dependencies>sdk/memory</dependencies>
</task_context>

# Task 47.0: Example: Memory Conversation (S)

## Overview

Create example demonstrating the complete memory system with all advanced features: flush strategies, privacy scoping, expiration, persistence, and distributed locking.

<critical>
- **ALWAYS READ** @.cursor/rules/go-coding-standards.mdc before start
- **ALWAYS READ** tasks/prd-sdk/05-examples.md (Example 4: Memory Conversation)
- **MUST** demonstrate all memory features (flush, privacy, persistence, etc.)
- **MUST** show memory reference with dynamic keys
</critical>

<requirements>
- Runnable example: sdk/examples/04_memory_conversation.go
- Demonstrates: Memory ConfigBuilder and ReferenceBuilder
- Shows: Summarization flush, privacy scoping, expiration, Redis persistence, distributed locking
- Agent integration with memory reference
- Clear comments on memory management patterns
</requirements>

## Subtasks

- [x] 47.1 Create sdk/examples/04_memory_conversation.go
- [x] 47.2 Build memory config with all features:
  - [x] Summarization flush strategy
  - [x] Privacy scoping (user-isolated)
  - [x] Expiration (24h auto-cleanup)
  - [x] Redis persistence
  - [x] Token counting
  - [x] Distributed locking
- [x] 47.3 Build memory reference with dynamic key template
- [x] 47.4 Create agent with memory integration
- [x] 47.5 Build project with memory resource
- [x] 47.6 Add comments explaining memory features
- [x] 47.7 Update README.md with memory example
- [x] 47.8 Test example runs successfully

## Implementation Details

Per 05-examples.md section 4:

**Advanced memory configuration:**
```go
memConfig, err := memory.New("customer-support").
    WithProvider("openai").
    WithModel("gpt-4o-mini").
    WithMaxTokens(2000).
    WithSummarizationFlush("openai", "gpt-4", 1000).
    WithPrivacy(memory.PrivacyUserScope).
    WithExpiration(24 * time.Hour).
    WithPersistence(memory.PersistenceRedis).
    WithDistributedLocking(true).
    Build(ctx)
```

**Memory reference with dynamic key:**
```go
memRef, err := memory.NewReference("customer-support").
    WithKey("conversation-{{.conversation.id}}-{{.user.id}}").
    Build(ctx)
```

**Agent with memory:**
```go
supportAgent, err := agent.New("support-agent").
    WithMemory(memRef).
    Build(ctx)
```

### Relevant Files

- `sdk/examples/04_memory_conversation.go` - Main example
- `sdk/examples/README.md` - Updated instructions

### Dependent Files

- `sdk/memory/config.go` - ConfigBuilder
- `sdk/memory/reference.go` - ReferenceBuilder
- `sdk/agent/builder.go` - Agent with memory

## Deliverables

- [x] sdk/examples/04_memory_conversation.go (runnable)
- [x] Updated README.md with memory example section
- [x] Comments explaining each memory feature:
  - [x] Summarization flush (keeps memory under token limit)
  - [x] Privacy scoping (user isolation)
  - [x] Expiration (auto-cleanup)
  - [x] Persistence (Redis)
  - [x] Distributed locking (concurrent access safety)
- [x] All memory features demonstrated
- [x] Verified example runs successfully

## Tests

From _tests.md:

- Example validation:
  - [x] Code compiles without errors
  - [x] Example runs with valid Redis connection
  - [x] All memory features configured correctly
  - [x] Dynamic key template validated
  - [x] Agent memory integration works
  - [x] Flush strategy config matches engine expectations
  - [x] Privacy scope enum validated

## Success Criteria

- Example demonstrates all advanced memory features
- Comments explain each feature's purpose
- Dynamic key template shown
- README updated with Redis setup requirements
- Example runs end-to-end successfully
- Code passes `make lint`
