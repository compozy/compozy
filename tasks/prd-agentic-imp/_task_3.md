## markdown

## status: pending

<task_context>
<domain>engine/llm/orchestrator</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>external_apis</dependencies>
</task_context>

# Task 3.0: Loop Context Modernization & Progress Engine

## Overview

Modularize the conversation loop context, enhance progress detection, and introduce adaptive restart and memory compaction controls to reduce stalled iterations.

<critical>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</critical>

<requirements>
- Decompose `LoopContext` into serializable state components (iteration, history, budgets, memory).
- Implement enhanced progress fingerprinting, dynamic restart policy, and configurable thresholds.
- Wire budget checks to new state structs and ensure deterministic FSM transitions.
- Maintain async memory writes while introducing compaction trigger hooks.
</requirements>

## Subtasks

- [ ] 3.1 Define new state structs (LoopState, BudgetManager, MemoryTracker) and refactor FSM interactions.
- [ ] 3.2 Implement progress fingerprinting with restart triggers and integrate with FSM events.
- [ ] 3.3 Add context-usage threshold detection and surface compaction callbacks to memory manager.
- [ ] 3.4 Update orchestrator unit/integration tests for FSM transitions covering restarts and budget exits.
- [ ] 3.5 Document state serialization strategy for potential persistence follow-up.

## Implementation Details

- PRD “Loop evolution” section requires dynamic restart, stronger memory/context controls, and richer budget checks.
- Ensure all new structs remain under 50 lines and expose JSON tags for future persistence.
- Avoid regressions by maintaining existing telemetry event contracts until Task 5.0 extends them.

### Relevant Files

- `engine/llm/orchestrator/state_machine.go`
- `engine/llm/orchestrator/loop.go`
- `engine/llm/orchestrator/loop_state.go` (new)
- `engine/llm/memory_integration.go`

### Dependent Files

- `engine/llm/orchestrator/response_handler.go`
- `engine/llm/orchestrator/tool_executor.go`

## Deliverables

- Refactored loop context with modular state components and restart logic.
- Updated FSM handling covering new events and budget enforcement.
- Documentation (inline or ADR note) detailing restart triggers and compaction hooks.

## Tests

- Unit/integration tests mapped from PRD test strategy:
  - [ ] Dynamic restart: repeated no-progress fingerprints trigger restart and reduce wasted iterations.
  - [ ] Context threshold: crossing usage% triggers compaction/summarize step.
  - [ ] Budget enforcement: limits halt loop with deterministic FSM exit.

## Success Criteria

- FSM successfully cycles through new restart path without deadlocks.
- Compaction hook fires exactly once per threshold breach.
- No regression in existing orchestrator tests; new coverage added for restart scenarios.
- `make fmt && make lint && make test` pass.
