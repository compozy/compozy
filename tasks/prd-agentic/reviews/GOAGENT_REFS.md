### GO Agent References → Improvements for `engine/llm`

This document consolidates research from `ai-docs/goagent/repo.txt` and Compozy’s current `engine/llm` to propose concrete, high‑impact improvements to our agentic orchestration. It serves as context, technical spec, and an implementation plan.

---

## Scope and Goals

- Use insights from GoAgent’s architecture to strengthen Compozy’s `engine/llm` agentic loop across robustness, extensibility, and structured outputs.
- Preserve our existing clean architecture and telemetry while adding missing controls: schema validation before tools, per‑tool limits, middleware hooks, dynamic prompts, and stronger finalization for structured output.

---

## Sources and References

- Primary external reference: `ai-docs/goagent/repo.txt` (extracted from Vitalii Honchar’s Go Agent library)
- Compozy code references (selected):
  - `engine/llm/orchestrator/orchestrator.go`
  - `engine/llm/orchestrator/loop.go`
  - `engine/llm/orchestrator/request_builder.go`
  - `engine/llm/orchestrator/response_handler.go`
  - `engine/llm/orchestrator/tool_executor.go`
  - `engine/llm/adapter/*` (providers, response formats)
  - `engine/llm/prompt_builder.go`
  - `engine/schema/schema.go`
  - `pkg/schemagen/*`

Research performed with Zen MCP (Gemini 2.5 Pro) across the above files and goagent reference.

---

## Summary of GoAgent Patterns Relevant to Us

- LLM Abstraction and Structured Output
  - Interface exposes `Call` and `CallWithStructuredOutput` with a typed helper to unmarshal JSON into generics.
  - OpenAI adapter sets `response_format` with strict JSON Schema when available; otherwise prompt‑based enforcement.

- Tooling System
  - Typed tools: parameters schema and typed results (generics), with automatic JSON schema generation using `invopop/jsonschema`.
  - Per‑tool usage limits and a uniform error result envelope.

- Agent Orchestration
  - ReAct‑style system prompt with explicit THINK/ACT/OBSERVE protocol and dynamic state (tools, usage, limits) rendered each iteration.
  - Middleware chain to intercept and modify behavior around LLM/tool phases.
  - Explicit finalization step for structured output (append dedicated prompt, then parse/validate JSON).

---

## Current `engine/llm` Architecture (Essentials)

- Orchestrator
  - `orchestrator.New(cfg)` composes: MemoryManager, RequestBuilder, ToolExecutor, ResponseHandler, LLMInvoker, Conversation loop, Telemetry recorder.
  - FSM‑driven loop (`loop.go`) with budgets: max iterations, per‑tool error budgets, progress tracking, and telemetry.

- Request Building and Prompts
  - `request_builder.go` builds prompt, tools, call options; decides native JSON Schema vs. prompt‑based schema enforcement; handles provider quirks (e.g., JSON mode off for Gemini when tools present; Groq JSON shim tool).
  - `prompt_builder.go` supports schema‑enhanced prompting when native structured output isn’t available.

- Adapters and Providers
  - `adapter/` uses LangChainGo models; supports OpenAI, Anthropic, Google (Gemini), Groq, Ollama, etc.
  - Structures for `OutputFormat` (default or JSON Schema); model caching by schema fingerprint; `response_format` for OpenAI.

- Telemetry and Memory
  - Rich run/iteration events; tool call/result snapshots; memory store of responses and failure episodes.

---

## Gaps vs. GoAgent (What to Add)

1. Pre‑Validate Tool Arguments Against Schema (before execution)

- Problem: Tool arg validation is not uniformly enforced at the orchestrator tool boundary.
- Solution: In `orchestrator/tool_executor.go`, validate `call.Arguments` using the tool’s declared JSON Schema (when available) via `engine/schema`. On failure, return a structured `ToolResult` error payload and record telemetry.

2. Per‑Tool Invocation Caps (configurable)

- Problem: We guard error budgets and progress stalls but don’t cap successful repeated calls per tool.
- Solution: Add per‑tool max invocations to `settings` and track in `loop_state`; enforce in `tool_executor.Execute` or `UpdateBudgets` with a budget‑exceeded error.

3. Agent Middleware Pipeline

- Problem: No public extension points for cross‑cutting concerns (RBAC, safety filters, custom logging, caching hints).
- Solution: Define middleware interfaces (e.g., `BeforeLLMRequest`, `AfterLLMResponse`, `BeforeToolExecution`, `AfterToolExecution`) and invoke them around existing loop transitions. Keep opt‑in and context‑first.

4. Dynamic System Prompt Augmentation per Iteration

- Problem: The system prompt is mostly static; the LLM lacks explicit view of current budgets/usage.
- Solution: Before each LLM call, augment the system prompt with current tool usage, error budgets, progress hints. Guard dynamic state with a feature flag. Always include a ReAct THINK/ACT/OBSERVE header.

5. Stronger Structured Output Finalization

- Problem: When native JSON Schema mode isn’t active or tools are involved, final JSON can be brittle.
- Solution: On final turn, append a deterministic “finalize outputs as JSON only” instruction and validate. If invalid, retry a small number of times with corrective instruction (budgeted, logged).

6. Uniform Tool Error Envelope

- Problem: Error result payloads can vary between tools.
- Solution: Standardize `ToolResult` error JSON shape across all tools: `{ success:false, error:{ code, message, details? }, remediation_hint? }`. Ensure adapters pass JSONContent for structured errors.

7. Telemetry and Diagnostics Enhancements

- Add: `tool_schema_id`, `forced_json_mode` (bool), `structured_output_strict` (bool), finalization retry counts; emit explicit reasons for fallbacks.

---

## Architectural Decisions and Design Notes

- Context‑First and Config‑Driven
  - All new toggles (per‑tool caps, dynamic prompts, finalize retries) must be wired via `pkg/config` and read through `config.FromContext(ctx)` (no singletons; no DI for loggers/config).
  - ReAct header is always included in the system prompt (not configurable).

- Budget Model
  - Extend existing budgets with: per‑tool success cap; structured output finalize retries; continue to track error budgets and progress fingerprints.

- Validation Layer
  - Prefer compiled schema validation (`engine/schema.Schema.Compile` then validate raw args map). Fail fast; never execute tools on invalid args.

- Provider Behavior
  - Keep OpenAI `response_format` path and schema‑cache approach; log and fallback when provider rejects structured output or schema too large.
  - For Gemini+tools, continue prompt‑based JSON enforcement; combine with finalization step for reliability.

---

## Proposed Changes (by file)

- `engine/llm/orchestrator/tool_executor.go`
  - Validate tool args against schema before `t.Call`.
  - Enforce per‑tool max invocations (read from settings). Emit budget‑exceeded error as structured JSON.
  - Standardize error envelope and ensure `JSONContent` is populated for LLM parsing.

- `engine/llm/orchestrator/loop_state.go`
  - Add counters for per‑tool successful invocations; track finalization retries.

- `engine/llm/orchestrator/request_builder.go`
  - Add dynamic system prompt augmentation (current usage/budgets) behind a feature flag.
  - Keep provider‑specific JSON mode logic; expose telemetry flags for `ForceJSON` and schema strictness.

- `engine/llm/orchestrator/response_handler.go`
  - Implement explicit finalization retries: on invalid JSON with structured output expected, append corrective prompt and continue loop until retry budget is exhausted.

- `engine/llm/orchestrator/orchestrator.go` and `loop.go`
  - Introduce middleware interfaces and invocation points around LLM request/response and tool execution.

- `engine/llm/prompt_builder.go`
  - Always include ReAct header in the system prompt; provide helper to render dynamic state fragments.

- `engine/schema/schema.go`
  - Add small helper(s) for validating arbitrary `json.RawMessage` against `*Schema` for tool arg checks.

---

## Configuration Additions (draft)

```yaml
llm:
  enable_dynamic_prompt_state: true # include tool usage/budgets in system prompt
  tool_call_caps:
    default: 3 # per‑tool default cap per conversation
    overrides:
      web_search: 5
  finalize_output_retries: 2 # retries for invalid final JSON
```

All config reads must use `config.FromContext(ctx)`.

---

## Risks and Mitigations

- Risk: Over‑constraining the LLM with too many prompt details.
  - Mitigation: Feature‑flag dynamic prompt state; measure via telemetry. ReAct header always included.

- Risk: Increased complexity around tool validation and caps.
  - Mitigation: Keep validation localized in `tool_executor`; use consistent error envelopes for predictability.

- Risk: Provider drift for structured outputs.
  - Mitigation: Preserve current provider gating; add explicit finalization path and clear telemetry on fallbacks.

---

## Rollout Plan (Phased)

Phase 1 – Safety and Reliability (Quick Wins)

- Tool arg pre‑validation and uniform error envelope (executor).
- Finalization retries for structured outputs (response handler).
- Telemetry additions (`forced_json_mode`, schema IDs, retries).
- Enable ReAct header in prompt builder (always on).

Phase 2 – Control and Guidance

- Per‑tool invocation caps via settings + loop state.
- Dynamic prompt augmentation (feature‑flagged).

Phase 3 – Extensibility

- Middleware pipeline with hooks around LLM and tool phases.

---

## Acceptance Criteria

- Invalid tool args never reach tool implementations; executor returns structured error; telemetry records the event.
- Per‑tool caps stop runaway loops; budget‑exceeded errors are structured and surfaced to the LLM.
- Final JSON outputs are reliable across providers due to explicit finalization and limited retries.
- Telemetry dashboards show forced JSON mode, strictness, schema IDs, and finalize retries.
- Feature flags can disable dynamic prompt; ReAct header is always enabled.

---

## Appendix: Notable Code Locations

- Dynamic structured output and prompt:
  - `engine/llm/orchestrator/request_builder.go` (JSON Schema vs. prompt enforcement; Groq JSON shim)
  - `engine/llm/prompt_builder.go` (schema‑enhanced prompts)

- Orchestration and budgets:
  - `engine/llm/orchestrator/loop.go` (FSM, budgets, progress tracking)
  - `engine/llm/orchestrator/tool_executor.go` (concurrency, budgets)

- Provider adapters and response formats:
  - `engine/llm/adapter/*` (OpenAI response_format, schema fingerprint cache)

- Schema infrastructure:
  - `engine/schema/schema.go` and `pkg/schemagen/*`

---

## Implementation Notes

- Keep functions < 50 lines; prefer small helpers with intention‑revealing names.
- No global configuration; always use `config.FromContext(ctx)` and `logger.FromContext(ctx)`.
- Avoid magic numbers; promote tunables to config.
- Add unit tests for:
  - Tool arg validation errors and envelopes
  - Per‑tool cap enforcement
  - Finalization retry behavior (invalid → corrected JSON)
  - Middleware ordering and short‑circuit behavior
