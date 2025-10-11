# Compozy LLM Orchestrator: Research-Driven Improvements (Architecture, Methods, Plan)

## Executive Summary

This document synthesizes recent research and best practices to improve Compozy’s agentic orchestration in `engine/llm`. It maps our current architecture, pinpoints issues, and proposes concrete upgrades aligned with hierarchical multi-agent orchestration, context engineering, robust tool use, and evaluation. The plan is designed to be incremental, testable, and measurable.

Top priorities:

- Provider abstraction: eliminate provider-specific hacks.
- Prompt system upgrade: template-driven, dynamic examples, structured-output enforcement.
- Loop evolution: progress detection, dynamic restart; stronger memory/context controls.

## Current Architecture Overview (engine/llm)

Core modules and responsibilities:

- `orchestrator/orchestrator.go`: Builds and wires orchestrator components (memory manager, request builder, tool executor, response handler, LLM invoker, FSM loop, telemetry recorder).
- `orchestrator/state_machine.go`: Finite State Machine (FSM) states/events for the conversation loop.
- `orchestrator/loop.go`: OnEnter\* handlers implementing the loop: await LLM → evaluate response → process tools → update budgets → handle completion → finalize.
- `orchestrator/request_builder.go`: Builds system+messages+tool definitions and call options; validates conversation.
- `orchestrator/response_handler.go`: Parses responses, enforces structured output, retries finalization with feedback.
- `orchestrator/tool_executor.go`: Executes tool calls, validates args, records telemetry; updates budgets.
- `orchestrator/llm_invoker.go`: Invoke with retry/timeout and jittered exponential backoff.
- `prompt_builder.go`: Minimal prompt assembly and structured-output hints.
- `adapter/providers.go`: Provider factory mapping to underlying LLM SDKs (OpenAI, Anthropic, Google, etc.).
- `telemetry/*`: Structured events, context usage, tool logs, content capture controls.

Execution flow (high level):

1. Request assembled → 2) LLM call → 3) If tools requested, execute tools → 4) Update budgets/progress → 5) Continue loop or finalize → 6) Store memory, emit telemetry.

## Dependencies & Execution Flow (FSM)

- States: `init` → `await_llm` → `evaluate_response` → `process_tools` → `update_budgets` → (`await_llm` | `terminate_error`) OR `handle_completion` → (`await_llm` | `finalize`).
- Key middleware hooks: before/after LLM request, before/after tool execution, before finalize.
- Telemetry points: request snapshot, response snapshot (with tool_calls count), context usage threshold warnings, tool logs, finalize.

## Current Issues and Risks

1. Tight provider coupling and cross-cutting hacks

- Evidence: Provider-specific logic handled in core loop (e.g., special JSON handling for Groq in `orchestrator/loop.go`), and a central switch-based provider factory in `adapter/providers.go`.
- Impact: Higher cost to add models; brittle when trying routing/fallback; scatters provider knowledge.

2. Rudimentary prompt management

- Evidence: `prompt_builder.go` uses string concatenation; no templates; limited dynamic examples; structured-output guidance but no provider-optimized prompts.
- Impact: Hard to iterate on prompting patterns; cannot easily A/B test; reduces quality/consistency.

3. Monolithic mutable LoopContext

- Evidence: `LoopContext` aggregates request, LLM request/response, tool results, budgets, counters; mutated across states.
- Impact: Harder to reason about state transitions; raises accidental side-effect risk; non-ideal for testing.

4. Progress/no-progress and budget checks are present but narrow

- Evidence: `update_budgets` supports budget exceed and simple progress fingerprint; no adaptive restart policy.
- Impact: Stuck loops may need richer signals; lacks just-in-time compaction or dynamic restart triggers.

5. Structured-output enforcement is reactive

- Evidence: `response_handler.go` validates output, can retry with feedback; JSON extraction helpers exist.
- Impact: Better proactive schema priming and provider-native JSON mode could reduce retries and latency.

6. Tool ergonomics could be stronger

- Evidence: Validation and telemetry exist; still rely on free-form descriptions.
- Impact: Model misuse of tools, argument mistakes, or redundant calls can increase cost and iteration count.

## Research-Aligned Improvements

The proposals below are mapped to concrete files and patterns, prioritizing incremental adoption.

### A. Provider Abstraction (High Priority)

- What: Replace `switch` factory with a self-registration `ProviderRegistry`. Encapsulate provider quirks (e.g., Groq JSON tool-call behavior) inside provider adapters.
- Why: Centralizes provider quirks and reduces cross-cutting hacks.
- Where:
  - `engine/llm/adapter/providers.go`: introduce `Provider` interface and `ProviderRegistry` with `Register()`; migrate each provider to its own file; factory resolves by key.
  - `orchestrator/loop.go`: remove provider-specific conditionals; handle via adapter.

### B. Prompt System Upgrade with Templates (High)

- What: Externalize prompts and use templates (Go `text/template` or similar). Support: system sections, dynamic few-shot examples, schema embedding, tool guidance blocks per provider.
- Why: Rapid iteration, A/B tests, provider-optimized prompts, easy evolution.
- Where:
  - `engine/llm/prompt_builder.go`: replace string concat with renderer that loads templates from `engine/llm/orchestrator/prompts/` (already exists for built-ins) and injects per-request data (tools list, schema, budget hints, memory summaries).
- Extensions: Provider-specific prompt variants; runbooks for JSON modes; automatic minimal context rendering from memory and attachments.

### C. Loop Enhancements: Dynamic Restart, Progress & Early-Stopping (High)

- What: Add adaptive restart and richer progress checks based on fingerprints of tools/results and LLM usage signals; optional “dynamic restart” if a new, substantially different plan emerges (inspired by multi-agent orchestration resets).
- Why: Prevent wasted iterations; keep the loop responsive; align with research findings on orchestration efficacy.
- Where:
  - `orchestrator/loop.go` & `state_machine.go`: add `EventDynamicRestart`; rules in `update_budgets` or a dedicated `evaluate_progress` state.
  - Telemetry signals (context usage % over thresholds, repeat call patterns) gate restarts or escalate finalization.

### E. Structured Output: Proactive Enforcement (High)

- What: Prefer provider-native JSON/Schema modes when available; include concise schema primers and validator examples in the prompt; keep finalizer retry but reduce its frequency.
- Why: Cuts retries; improves deterministic parsing; aligns with “tool- and schema-first” guidance.
- Where:
  - `request_builder.go` / `prompt_builder.go`: set `ForceJSON`/provider JSON schema flags earlier; embed short JSON examples from templates.
  - `response_handler.go`: keep `extractJSONObject` as fallback; shorten retry budgets when native JSON mode is active.

### F. Tool Ergonomics & Validation (Medium)

- What: Strengthen tool arg schemas and allow short auto-generated usage examples per tool; add cooldown/penalty for repeated failing tool calls.
- Why: Reduce invalid calls and loops; improve tool selection.
- Where:
  - `tool_executor.go`: add penalties and cooldowns; extend telemetry to surface high-failure tools quickly.
  - `tool_registry.go`: carry richer tool metadata (arg schema, hints, examples); use in prompt templates.

### G. Memory Compaction & Context Controls (Medium)

- What: Compact message history when thresholds are crossed; summarize tool outcomes and decisions; preserve important artifacts separately.
- Why: Avoid context rot; lower costs; keep signal high.
- Where:
  - `orchestrator/loop.go`: when context usage warning fires, auto-inject a summarized state message replacing redundant history; store full history to memory.

### H. Telemetry & Budget Policies (Medium)

- What: Expand budget policy (max iterations, tool budgets, time budgets) configured via `pkg/config`; emit structured events when policies impact path.
- Why: Operational control and clearer observability.
- Where:
  - `llm_invoker.go`, `loop.go`, `tool_executor.go`: move magic numbers/constants to config; extend structured logs and alerts.

## Implementation Plan (Phased)

Phase 1 (Safety & Velocity)

- Introduce `ProviderRegistry`; migrate Groq JSON handling into adapter; add minimal routing hook.
- Externalize prompts to templates; keep current fields; no behavior change yet.
- Promote magic numbers (retries, timeouts, thresholds) to config (`pkg/config`).

Phase 2 (Quality & Cost)

- Enable provider-native JSON/schema modes by default when schema present.
- Add compact/summarize step on context threshold events.
- Add tool arg schema examples and cooldown on high failure rate.

Phase 3 (Capability)

- Add dynamic restart and richer progress checks.
- Optional consensus/voting mode as a feature flag for high-stake actions.

Phase 4 (Evaluation & Hardening)

- Add tests (unit + integration) for: routing fallback, JSON mode, compact summaries, restarts, tool cooldowns.
- Use `testgen` to propose new cases; run `codereview` and `precommit` gates.

## Relevant Files (targets for edits)

- Provider Abstraction: `engine/llm/adapter/providers.go` (+ new per-provider files)
- Prompt Templates/Builder: `engine/llm/prompt_builder.go`, `engine/llm/orchestrator/prompts/*`
- FSM Enhancements: `engine/llm/orchestrator/state_machine.go`, `engine/llm/orchestrator/loop.go`
- Structured Output: `engine/llm/orchestrator/request_builder.go`, `engine/llm/orchestrator/response_handler.go`
- Tools: `engine/llm/tool_registry.go`, `engine/llm/orchestrator/tool_executor.go`
- Telemetry/Budgets: `engine/llm/orchestrator/llm_invoker.go`, `engine/llm/orchestrator/loop.go`, `engine/llm/telemetry/*`
- Config: `pkg/config/*` (new tunables for retries, timeouts, budgets, thresholds)

## Open Technical Questions

- Consensus cost vs. accuracy: default off, behind per-action flags?
- Memory compaction strategies: fixed templates vs. model-generated summaries guarded by validators?
- Routing policy source: static config vs. learned scores from telemetry?

## Test Strategy (new cases)

- Routing fallback: provider A failure → provider B success.
- Native JSON mode: schema present → zero finalization retries; invalid JSON triggers fallback extraction.
- Compact on threshold: crossing usage% triggers compaction/summarization.
- Dynamic restart: repeated no-progress fingerprints lead to restart; confirms reduction in wasted iterations.
- Tool cooldown: repeated invalid arg calls lead to penalty; loop exits earlier.

## Sources & References

- AgentOrchestra: A Hierarchical Multi-Agent Framework for General-Purpose Task Solving (arXiv:2506.12508)
  - `https://arxiv.org/html/2506.12508v1`
- Beyond the Strongest LLM: Multi-Turn Multi-Agent Orchestration vs. Single LLMs (arXiv:2509.23537)
  - `https://arxiv.org/html/2509.23537v1`
- An Agentic Flow for Finite State Machine Extraction using Prompt Chaining (arXiv:2507.11222)
  - `https://arxiv.org/html/2507.11222v1`
- Anthropic: Building effective agents
  - `https://www.anthropic.com/research/building-effective-agents`
- Anthropic: Effective context engineering for AI agents
  - `https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents`
- Anthropic: Building agents with the Claude Agent SDK
  - `https://www.anthropic.com/engineering/building-agents-with-the-claude-agent-sdk`
- Anthropic: How we built our multi-agent research system
  - `https://www.anthropic.com/engineering/multi-agent-research-system`

## Appendix: Mapping Issues → Recommendations

- Provider hacks in loop → ProviderRegistry; adapter encapsulation
- Prompt concat → templated prompts; provider variants
- No-progress loops → dynamic restart + richer progress heuristics
- Finalization retries → native JSON schema + smaller retry budgets
- Tool misuse → arg schemas, examples, cooldown penalties
- Context rot → usage-gated compaction & summaries
- Magic numbers → `pkg/config` tunables + telemetry
