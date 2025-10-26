## status: pending

<task_context>
<domain>v2/examples</domain>
<type>documentation</type>
<scope>examples</scope>
<complexity>low</complexity>
<dependencies>v2/memory</dependencies>
</task_context>

# Task 47.0: Example: Memory Conversation (S)

## Overview

Create example demonstrating the complete memory system with all advanced features: flush strategies, privacy scoping, expiration, persistence, and distributed locking.

<critical>
- **ALWAYS READ** @.cursor/rules/go-coding-standards.mdc before start
- **ALWAYS READ** tasks/prd-modules/05-examples.md (Example 4: Memory Conversation)
- **MUST** demonstrate all memory features (flush, privacy, persistence, etc.)
- **MUST** show memory reference with dynamic keys
</critical>

<requirements>
- Runnable example: v2/examples/04_memory_conversation.go
- Demonstrates: Memory ConfigBuilder and ReferenceBuilder
- Shows: Summarization flush, privacy scoping, expiration, Redis persistence, distributed locking
- Agent integration with memory reference
- Clear comments on memory management patterns
</requirements>

## Subtasks

- [ ] 47.1 Create v2/examples/04_memory_conversation.go
- [ ] 47.2 Build memory config with all features:
  - [ ] Summarization flush strategy
  - [ ] Privacy scoping (user-isolated)
  - [ ] Expiration (24h auto-cleanup)
  - [ ] Redis persistence
  - [ ] Token counting
  - [ ] Distributed locking
- [ ] 47.3 Build memory reference with dynamic key template
- [ ] 47.4 Create agent with memory integration
- [ ] 47.5 Build project with memory resource
- [ ] 47.6 Add comments explaining memory features
- [ ] 47.7 Update README.md with memory example
- [ ] 47.8 Test example runs successfully

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

- `v2/examples/04_memory_conversation.go` - Main example
- `v2/examples/README.md` - Updated instructions

### Dependent Files

- `v2/memory/config.go` - ConfigBuilder
- `v2/memory/reference.go` - ReferenceBuilder
- `v2/agent/builder.go` - Agent with memory

## Deliverables

- [ ] v2/examples/04_memory_conversation.go (runnable)
- [ ] Updated README.md with memory example section
- [ ] Comments explaining each memory feature:
  - Summarization flush (keeps memory under token limit)
  - Privacy scoping (user isolation)
  - Expiration (auto-cleanup)
  - Persistence (Redis)
  - Distributed locking (concurrent access safety)
- [ ] All memory features demonstrated
- [ ] Verified example runs successfully

## Tests

From _tests.md:

- Example validation:
  - [ ] Code compiles without errors
  - [ ] Example runs with valid Redis connection
  - [ ] All memory features configured correctly
  - [ ] Dynamic key template validated
  - [ ] Agent memory integration works
  - [ ] Flush strategy config matches engine expectations
  - [ ] Privacy scope enum validated

## Success Criteria

- Example demonstrates all advanced memory features
- Comments explain each feature's purpose
- Dynamic key template shown
- README updated with Redis setup requirements
- Example runs end-to-end successfully
- Code passes `make lint`
