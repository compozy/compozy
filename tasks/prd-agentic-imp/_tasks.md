# Agentic Improvements Implementation Task Summary

## Relevant Files

### Core Implementation Files

- `engine/llm/adapter/providers.go` - Replace factory switch with capability-driven registry
- `engine/llm/orchestrator/loop.go` - Integrate new loop context, progress controls, and provider abstraction
- `engine/llm/prompt_builder.go` - Introduce template engine and structured-output priming
- `engine/llm/tool_registry.go` - Expand tool metadata (schemas, cooldowns)
- `engine/llm/telemetry/*` - Capture richer agent metrics and thresholds
- `pkg/config/*` - Surface tunables for retries, budgets, thresholds

### Integration Points

- `engine/llm/service.go` - Orchestrator wiring and provider selection
- `engine/llm/memory_*` - Memory compaction hooks for progress and summaries
- `cli/helpers/*` - CLI/runtime configuration for new tunables

### Documentation Files

- `tasks/prd-agentic/reviews/AGENTIC_IMPROVEMENTS.md` - Source PRD/tech spec details

## Tasks

- [x] 1.0 Provider Registry & Capability Layer (L)
- [x] 2.0 Prompt Template & Structured Output System (M)
- [ ] 3.0 Loop Context Modernization & Progress Engine (L)
- [ ] 4.0 Tool Schema, Cooldown & Ergonomics Upgrade (M)
- [ ] 5.0 Telemetry & Config Tunables Expansion (M)

Notes on sizing:

- S = Small (≤ half-day)
- M = Medium (1–2 days)
- L = Large (3+ days)

## Task Design Rules

- Each parent task is a closed deliverable: independently shippable and reviewable
- Do not split one deliverable across multiple parent tasks; avoid cross-task coupling
- Each parent task must include unit/integration test subtasks derived from PRD test strategy
- Each generated `/_task_<num>.md` must contain explicit Deliverables and Tests sections

## Execution Plan

- Critical Path: 1.0 → 3.0 → 4.0 → 5.0
- Parallel Track A (after 1.0): 2.0 Prompt Template & Structured Output System
- Parallel Track B (after 3.0): 4.0 Tool Schema/Cooldown with 5.0 Telemetry & Config Tunables

Notes

- All runtime code MUST use `logger.FromContext(ctx)` and `config.FromContext(ctx)`
- Run `make fmt && make lint && make test` before marking any task as completed

## Batch Plan (Grouped Commits)

- [x] Batch 1 — Provider Foundations: 1.0
- [x] Batch 2 — Prompt System Upgrade: 2.0
- [ ] Batch 3 — Loop & Tools Core: 3.0, 4.0
- [ ] Batch 4 — Observability & Tunables: 5.0
