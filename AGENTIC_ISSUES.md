# Agentic Issues – Commit 2a6afd4 (2025-10-11)

## 1. Context & Objectives

- PRD scope “prd-agentic-imp” introduces loop restarts, conversation compaction, telemetry upgrades, templated prompts, and a provider registry overhaul under `engine/llm`.
- Overall scope focuses on the `engine/llm/orchestrator` and `engine/llm/adapter` changes shipped in commit `2a6afd4`.
- Objectives:
  - Validate the agentic loop refactor, memory compaction flow, prompt refresh logic, and provider capability plumbing.
  - Confirm that the changes are net improvements, surface regressions, and outline the validation path required for production readiness.

## 2. Strengths Observed

- Prompt generation now flows through reusable templates with dynamic context rendering, improving consistency (`engine/llm/prompt_builder.go`, `engine/llm/system_prompt_renderer.go`).
- Loop state structuring decouples serialisable budgets/progress from runtime handles, making telemetry snapshots richer (`engine/llm/orchestrator/loop_state.go`).
- Config surface mirrors new orchestration controls and telemetry thresholds, closing historical gaps between YAML schema and runtime (`engine/llm/config.go`, `pkg/config/config.go`).

## 3. Priority-Grouped Findings

### 3.1 Critical

1. **Prompt refresh corrupts memory transcripts**
   - Files: `engine/llm/orchestrator/loop.go:33`, `engine/llm/orchestrator/memory.go:137`.
   - Issue: `refreshUserPrompt` still overwrites `Messages[0]`, but `memoryManager.Inject` now prepends persisted transcripts. The first message is no longer the live user prompt, so the loop corrupts memory entries and keeps using stale prompts after each iteration.
   - Actions:
     - Use `loopCtx.baseMessageCount` (or scan for the first `RoleUser` message) before mutating the prompt.
     - Reject refreshes if no user message exists.
     - Add regression coverage for conversations that include memory injection.

2. **Shared mutable state during loop restarts**
   - File: `engine/llm/orchestrator/loop.go:570-580`.
   - Issue: `restartLoop` copies base conversation slices without deep-cloning nested fields (`ToolCalls`, `Parts`). Subsequent mutations leak into prior iterations, producing non-deterministic behaviour and corrupted telemetry under retries.
   - Actions:
     - Introduce deterministic deep-copy helpers on `llmadapter.Message`, `ToolCall`, and `ToolResult`.
     - Apply deep copies wherever the base history is preserved (restart paths, memory compaction persistence, telemetry snapshots).
     - Add regression tests that mutate tool payloads across restarts.

### 3.2 High

1. **Context usage denominator wrong—compaction never triggers**
   - File: `engine/llm/orchestrator/loop.go:820`.
   - Issue: `computeContextUsage` divides by the completion `MaxTokens` cap instead of the provider’s true context window. Usage percentages remain near zero, so compaction thresholds, telemetry warnings, and the new compaction pipeline never fire.
   - Actions:
     - Extend `llmadapter.ProviderCapabilities` (and factory wiring) to surface provider context window size.
     - Populate `LoopContext` with this value and compute usage percentages against it, falling back to completion caps only when the window is unknown.

2. **Global default provider registry impedes DI and structured logging**
   - Files: `engine/llm/adapter/providers.go:73-125`, `engine/llm/adapter/factory.go:12-44`.
   - Issue: `defaultProviderRegistry` is populated via init-time side effects and consumed implicitly by `NewDefaultFactory`, emitting `log.Printf`. This hides dependencies, complicates testing, and pollutes logs.
   - Actions:
     - Move toward explicit registry injection and context-aware logging (`logger.FromContext`).
     - Expose constructors such as `NewDefaultFactoryWithRegistry`.
     - Register providers during application bootstrap; avoid implicit globals.

3. **Regression coverage gaps for orchestrator restarts and compaction**
   - Issue: Without targeted integration tests, the issues above can reappear unnoticed.
   - Actions: Restore end-to-end orchestration scenarios covering restarts, compaction toggles, dynamic prompt updates, and MCP/provider fallbacks, aligning with PRD expectations.

4. **Telemetry & compaction guardrails absent when context window unknown**
   - File: `engine/llm/orchestrator/loop.go:820`.
   - Issue: When the real context limit cannot be determined, `percent` stays at zero, silently disabling telemetry/compaction.
   - Actions: Emit telemetry/log warnings when limits are absent and consider failing fast for providers requiring compaction.

### 3.3 Medium

1. **Template fallback cost and hidden errors**
   - File: `engine/llm/orchestrator/request_builder.go:401-431`.
   - Issue: `composeSystemPromptFallback` reparses templates from `embed.FS` on every failure, hiding root errors and adding CPU overhead.
   - Actions: Cache the fallback template at init and emit structured error context when the primary render fails.

2. **Configuration feedback loop lacks transparency**
   - File: `engine/llm/orchestrator/config.go:90-111`.
   - Issue: `normalizeNumericDefaults` silently clamps `RestartStallThreshold` when it exceeds `NoProgressThreshold`, creating operator confusion.
   - Actions: Emit warnings/telemetry events when clamping occurs; document this behaviour in `schemas/config-llm.json`.

3. **Prompt refresh ignores multimodal parts**
   - File: `engine/llm/orchestrator/loop.go:41-47`.
   - Issue: Only `Content` is refreshed; if the template alters `Parts`, stale attachments linger.
   - Actions: Regenerate the full user message (content plus parts) or provide hooks to update attachments alongside rendered text.

### 3.4 Low

1. **Partial deep clone coverage in provider config**
   - File: `engine/llm/adapter/provider_registry.go:170-205`.
   - Issue: `cloneProviderConfig` only duplicates `StopWords`; future mutable fields risk sharing state.
   - Actions: Expand the helper (and document expectations) to deep-copy all slice/map fields going forward.

2. **Minor memory injection optimisation**
   - File: `engine/llm/orchestrator/memory.go:143-145`.
   - Observation: Combined slice can be built with `append(memoryMessages, base...)`; not urgent but worth tidying during refactors.

## 4. Recommended Remediations (from Review)

1. **Fix prompt refresh targeting (Critical).**  
   Use `loopCtx.baseMessageCount` (or search for the first `RoleUser`) before mutating the prompt. Reject refreshes if no user message exists. Add regression coverage for conversations with memory injection.

2. **Expose and consume real context window (High).**  
   Extend `llmadapter.ProviderCapabilities` to surface provider context window size. Populate `LoopContext` with this value and update `computeContextUsage` to compute percentages against it, falling back to completion caps only if the window is unknown.

3. **Add guardrails & observability (Medium).**  
   Log and emit telemetry when context window metadata is missing so operators can adjust configuration. Consider failing fast during configuration if window size cannot be determined for providers that require compaction.

## 5. Combined Strategic Priorities (from both analyses)

1. Implement deterministic deep-copying for loop restart state (Critical).
2. Fix prompt refresh targeting to preserve memory integrity (Critical).
3. Expose real context window metadata and repair `computeContextUsage` (High).
4. Refactor provider registry toward explicit DI and structured logging (High).
5. Restore orchestration/regression coverage across restarts, compaction, and dynamic prompts (High).

## 6. Investigation & Mitigation Plan

1. **State Integrity Audit**
   - Design deep-copy utilities for `llmadapter.Message`, `ToolCall`, and `ToolResult`.
   - Update `restartLoop`, memory compaction persistence, and telemetry snapshots to use clones.
   - Add regression tests simulating restart scenarios with mutable tool payloads.

2. **Provider Registry Hardening**
   - Introduce DI-friendly constructors and thread them through `engine/llm/service.go`.
   - Register providers during application bootstrap and switch to context-aware logging.
   - Add unit tests ensuring duplicate registration warnings are captured via structured logs.

3. **Context-Window Plumbing & Guardrails**
   - Surface provider context windows via `ProviderCapabilities` and record them on `LoopContext`.
   - Update `computeContextUsage` to prioritise this value; log/alert when absent.
   - Tune compaction thresholds once accurate percentages are produced.

4. **Template Pipeline Resilience**
   - Cache fallback templates and emit detailed telemetry when rendering fails.
   - Extend prompt builder tests to cover dynamic examples and failure guidance interplay, including attachment scenarios.

5. **Config Transparency & Observability**
   - Emit warnings when restart thresholds are clamped; document behaviour in config schemas.
   - Confirm telemetry warning thresholds align with config defaults and PRD guidance.

6. **Clone Utility Review**
   - Expand clone helpers to deep-copy future mutable fields.
   - Document assumptions and enforce via targeted tests or lint rules.

7. **Regression Coverage Restoration**
   - Build E2E scenarios covering restarts, compaction, dynamic prompt refresh, and MCP/provider fallbacks.

## 7. Validation & Testing Checklist

- Add unit/integration tests covering:
  - Memory injection + prompt refresh path (ensuring rendered prompt lands on the correct user message).
  - Context usage calculation using mocked provider capabilities and verifying compaction triggers once thresholds are exceeded.
- Implement fixes, then run `make lint` and `make test`.
- Add focused unit tests for restart cloning, provider route fallback, prompt refresh with memory injection, and context usage calculations.
- Execute refreshed end-to-end scenarios covering restarts, compaction thresholds, dynamic prompt state, and MCP/provider fallbacks once new coverage lands.

## 8. Open Questions & Follow-Ups

- Confirm whether provider SDKs already expose context window sizes; if not, determine canonical defaults per provider.
- Evaluate whether automatic “agent completion hints” should be excluded from memory persistence to avoid noise (not a regression but worth review).

## 9. Alignment Notes

- Critical and high-severity issues were independently identified and confirmed by both manual analysis and expert (Zen MCP) reviews.
- Medium and low-severity observations combine personal findings (template fallback, config clamp, clone helpers) with additional expert suggestions (context guardrails, prompt attachments).
- All recommendations respect existing architectural standards—context-first logging, dependency injection, and deterministic memory handling.

---

## Appendix A – Source Content: Agentic Review Plan

```
# Agentic Review Plan – Commit 2a6afd4 (2025-10-11)

## Scope
- Focus on the `engine/llm/orchestrator` and `engine/llm/adapter` changes shipped in commit `2a6afd4`.
- Validate the agentic loop refactor, memory compaction flow, prompt refresh logic, and provider capability plumbing.
- Assess whether the new functionality constitutes a net improvement or introduces regressions.

## Summary Of Key Findings
- **Critical** – `engine/llm/orchestrator/loop.go:33` + `engine/llm/orchestrator/memory.go:137`
  The new `refreshUserPrompt` still overwrites `Messages[0]`, but `memoryManager.Inject` now prepends persisted transcripts. The first message is no longer the live user prompt, so the loop corrupts memory entries and keeps using stale prompts after each iteration.
- **High** – `engine/llm/orchestrator/loop.go:820`
  `computeContextUsage` divides by the agent `MaxTokens` completion cap instead of the provider context window. Usage percentages remain near zero, so compaction thresholds, telemetry warnings, and the new compaction pipeline never fire.
- **Medium** – Telemetry/compaction thresholds (same location) silently skip work when the true context limit is unknown; no guardrails alert operators.

## Recommended Remediations
1. **Fix prompt refresh targeting (Critical).**
   Use `loopCtx.baseMessageCount` (or search the message slice for the first `RoleUser`) before mutating the prompt. Reject refreshes if no user message exists. Add regression coverage for conversations with memory injection.
2. **Expose and consume real context window (High).**
   Extend `llmadapter.ProviderCapabilities` (and factory wiring) to surface provider context window size. Populate `LoopContext` with this value and update `computeContextUsage` to compute percentages against it, falling back to completion caps only if the window is unknown.
3. **Add guardrails & observability (Medium).**
   Log and emit telemetry when context window metadata is missing so operators can adjust configuration. Consider failing fast during configuration if window size cannot be determined for providers that require compaction.

## Validation Plan
- Add unit/integration tests covering:
  - Memory injection + prompt refresh path (ensures rendered prompt lands on the correct user message).
  - Context usage calculation using mocked provider capabilities and verifying compaction triggers once thresholds are exceeded.
- Re-run `make lint` and `make test` after applying fixes.

## Open Questions / Follow-Ups
- Confirm whether provider SDKs already expose context window sizes; if not, determine canonical defaults per provider.
- Evaluate whether automatic “agent completion hints” should be excluded from memory persistence to avoid noise (not a regression but worth review).
```

## Appendix B – Source Content: Agentic Implementation Review Plan

```
# Agentic Implementation Review Plan (commit 2a6afd4)

## Context

- PRD scope “prd-agentic-imp” introduces loop restarts, conversation compaction, telemetry upgrades, templated prompts, and a provider registry overhaul under `engine/llm`.
- Objective: confirm these changes are net improvements, surface regressions, and outline the validation path required for production readiness.

## Strengths Observed

- Prompt generation now flows through reusable templates with dynamic context rendering, improving consistency (`engine/llm/prompt_builder.go`, `engine/llm/system_prompt_renderer.go`).
- Loop state structuring decouples serialisable budgets/progress from runtime handles, making telemetry snapshots richer (`engine/llm/orchestrator/loop_state.go`).
- Config surface mirrors new orchestration controls and telemetry thresholds, closing historical gaps between YAML schema and runtime (`engine/llm/config.go`, `pkg/config/config.go`).

## Strategic Risks & Findings

### Critical – Shared Mutable State During Loop Restarts

- **Issue**: `restartLoop` copies base conversation slices without deep-cloning nested fields (e.g., `ToolCalls`, `Parts`), so subsequent mutations can leak into prior iterations. (`engine/llm/orchestrator/loop.go:570-580`)
- **Impact**: Non-deterministic agent behaviour, corrupted telemetry, and hard-to-debug regressions under retries.
- **Status**: Independently identified and confirmed by expert review; requires immediate mitigation with explicit deep-copy helpers on `llmadapter.Message`.

### High – Global Default Provider Registry

- **Issue**: `defaultProviderRegistry` is populated via init-style logic (`engine/llm/adapter/providers.go:73-125`) and consumed implicitly by `NewDefaultFactory`, obstructing dependency injection and polluting logs with `log.Printf`. (`engine/llm/adapter/factory.go:12-44`, `engine/llm/adapter/providers.go:100-107`)
- **Impact**: Difficult unit testing, hidden init-order coupling, inconsistent logging in production.
- **Status**: Alignment between personal findings and expert call-out; move toward explicit registry injection and structured logging.

### Medium – Template Fallback Cost & Error Transparency

- **Issue**: `composeSystemPromptFallback` reparses templates from `embed.FS` on every failure (`engine/llm/orchestrator/request_builder.go:401-431`), hiding root errors and adding avoidable CPU overhead.
- **Impact**: Elevated latency under repeated failures; makes diagnosing template issues harder.
- **Status**: Expert recommendation validated; cache compiled fallback template and emit structured error context.

### Medium – Configuration Feedback Loop

- **Issue**: `normalizeNumericDefaults` silently clamps `RestartStallThreshold` when greater than `NoProgressThreshold` but does not surface this to operators (`engine/llm/orchestrator/config.go:90-111`).
- **Impact**: Operators may think higher thresholds are active; misalignment between intent and runtime behaviour.
- **Status**: Personal finding; plan to emit telemetry/log notice when clamping occurs.

### Low – Partial Clone Implementations

- **Issue**: `cloneProviderConfig` deep-copies `StopWords` only; future mutable fields risk shared state (`engine/llm/adapter/provider_registry.go:170-205`).
- **Impact**: Currently low but scales poorly with future config extensions.
- **Status**: Both analyses note; document requirement and extend clone coverage.

## Top 3 Strategic Priorities

1. **Implement deterministic deep-copying for loop restart state** (Critical). Introduce `llmadapter.Message.DeepCopy()` and use it wherever base histories are preserved.
2. **Refactor provider registry to explicit DI and structured logging** (High). Expose registry/factory via constructors, replace `log.Printf` with project logger.
3. **Restore end-to-end orchestration regression coverage** (High). Build scenarios covering restarts, compaction, and dynamic prompt updates to match PRD expectations.

## Investigation & Mitigation Plan

1. **State Integrity Audit**
   - Design deep-copy utilities for `llmadapter.Message`, `ToolCall`, and `ToolResult`.
   - Update `restartLoop`, `MemoryManager.Compact`, and telemetry snapshots to use copies.
   - Add regression tests simulating restart with mutable tool call payloads.

2. **Provider Registry Hardening**
   - Introduce `llmadapter.NewDefaultFactoryWithRegistry(reg *Registry)` public constructor and thread through `engine/llm/service.go`.
   - Register providers in application bootstrap; replace global logging with `logger.FromContext`.
   - Add unit tests ensuring duplicate registration warnings are captured via structured logs.

3. **Regression Coverage Restoration**
   - Recreate multi-iteration orchestration test exercising restart + compaction toggles.
   - Ensure telemetry thresholds and dynamic examples are verified in integration tests.
   - Validate MCP and fallback provider routes using mocks to hit new code paths.

4. **Template Pipeline Resilience**
   - Cache system prompt fallback template at init; surface render errors via telemetry/logger.
   - Extend prompt builder tests to cover dynamic examples + failure guidance interplay.

5. **Config Transparency & Observability**
   - Emit warning event when restart thresholds are clamped; document behaviour in config schema (`schemas/config-llm.json`).
   - Confirm telemetry warning thresholds align with config defaults and PRD guidance.

6. **Clone Utility Review**
   - Expand `cloneProviderConfig` and related helpers to deep-copy future slice/map fields.
   - Document expectations inside the helper and enforce via lint/test.

## Validation Checklist

- Run `make lint` and `make test` (already executed; see CLI output below when available).
- Add focused unit tests for restart cloning, provider route fallback, and template error telemetry.
- Execute refreshed e2e scenario once new coverage lands.

## Alignment with Expert Review

- Findings above merge personal analysis with expert recommendations. Items marked Critical/High were corroborated by the expert output; Medium/Low entries extend the expert feedback with additional local observations (template clamping, test coverage gap).
- Any divergences were resolved by cross-verifying code paths and ensuring recommendations respect existing architectural patterns (context-first logging, DI-first constructors).
```
