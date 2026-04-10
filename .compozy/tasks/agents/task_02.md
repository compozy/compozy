---
status: pending
title: Runtime resolution and canonical system prompt assembly
type: backend
complexity: high
dependencies:
  - task_01
---

# Task 02: Runtime resolution and canonical system prompt assembly

## Overview
Integrate resolved agents into Compozy's execution configuration model by merging runtime precedence and assembling the final canonical system prompt for agent-backed runs. This task turns agent definitions from static files into executable runtime inputs that can be used consistently by CLI execution and nested agent runs.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
- NOTE: No `_prd.md` exists for this feature. Technical requirements are derived from `_techspec.md` and the accepted ADRs.
</critical>

<requirements>
- MUST resolve effective runtime settings for agent-backed executions in the exact precedence order defined by the TechSpec: explicit CLI flags, `AGENT.md` defaults, workspace config defaults, then built-in runtime defaults.
- MUST use explicit-flag detection semantics compatible with Cobra's `cmd.Flags().Changed(...)` so zero-value flags do not accidentally override agent defaults.
- MUST assemble the system prompt in the canonical order defined by the TechSpec: built-in framing, agent metadata block, compact discovery catalog, then the agent prompt body.
- MUST keep the discovery catalog intentionally compact and limited to progressive-discovery fields only.
- MUST ensure the selected agent's metadata and prompt body are injected once and only once into the final system prompt.
- MUST keep prompt assembly reusable across direct `exec --agent` runs and nested child-agent runs.
- MUST preserve the existing non-agent execution path when no agent is selected.
</requirements>

## Subtasks
- [ ] 02.1 Add runtime-resolution helpers that merge selected-agent defaults with workspace config and built-in runtime defaults.
- [ ] 02.2 Introduce an agent prompt assembler that emits the canonical ordered output defined by the TechSpec.
- [ ] 02.3 Build compact discovery-catalog generation from the resolved agent registry for progressive agent discovery.
- [ ] 02.4 Integrate the assembled prompt and resolved runtime into the shared execution-preparation path without regressing non-agent runs.
- [ ] 02.5 Add focused tests for precedence ordering, zero-value flag behavior, and canonical prompt assembly output.

## Implementation Details
See TechSpec "Runtime precedence", "Discovery catalog", and "System prompt assembly template" for the exact ordering and field set this task must implement.

This task should not attach MCP servers or implement nested execution. Its job is to produce the resolved runtime and system prompt inputs that the ACP layer can consume later. Preserve the current no-agent behavior and avoid duplicating prompt composition logic across execution modes.

### Relevant Files
- `internal/core/model/runtime_config.go` — Existing built-in runtime defaults that agent-backed runs must layer on top of.
- `internal/core/prompt/common.go` — Existing shared prompt addendum seam to extend or complement for agent-backed prompts.
- `internal/core/prompt/prompt_test.go` — Existing prompt expectations that should be expanded rather than bypassed.
- `internal/core/plan/prepare.go` — Existing prompt-building flow that may need to consume agent-aware resolution.
- `internal/core/run/internal/acpshared/command_io.go` — Shared ACP execution-preparation seam where the assembled system prompt will be applied.
- `internal/core/agents/` — Registry output from task 01 that this task must consume.

### Dependent Files
- `internal/cli/commands.go` — Later `exec --agent` CLI wiring will depend on explicit-flag precedence behavior defined here.
- `internal/core/agent/client.go` — Later session creation will consume resolved runtime settings and preassembled prompts.
- `internal/core/run/executor/execution_acp_test.go` — Existing execution prompt tests may need updates once agent-aware prompt assembly is added.

### Related ADRs
- [ADR-002: Assemble Agent System Prompts from Resolved Metadata with Explicit Override Precedence](adrs/adr-002.md) — Primary ADR for prompt order and precedence.
- [ADR-004: Keep V1 Agent Capability Scope Minimal](adrs/adr-004.md) — Constrains discovery-catalog shape and avoids extra prompt providers.

## Deliverables
- Agent-aware runtime-resolution helpers with explicit-flag precedence support.
- Canonical system prompt assembler for selected agents.
- Compact discovery-catalog generation for available sibling agents.
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration tests for agent-backed prompt composition in the shared execution path **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] Explicit CLI `--model` wins over a different `model` value defined in `AGENT.md`.
  - [ ] An unset CLI flag does not override an agent default just because the Cobra value is the language zero value.
  - [ ] The assembled system prompt contains built-in framing before agent metadata, metadata before discovery, and discovery before the agent body.
  - [ ] The discovery catalog omits unsupported rich fields and contains only the compact progressive-discovery data.
  - [ ] When no agent is selected, prompt assembly falls back to the existing non-agent behavior.
- Integration tests:
  - [ ] An agent-backed execution request reaches the shared ACP path with the resolved model, reasoning effort, access mode, and system prompt expected by precedence rules.
  - [ ] A workspace config default still applies when the selected `AGENT.md` omits that runtime field.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- Agent-backed executions produce a deterministic system prompt whose order matches the TechSpec.
- Runtime precedence behaves exactly as documented, including explicit-flag handling for zero values.
- Non-agent execution continues to work unchanged when no agent is selected.
