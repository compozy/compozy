Context Document: Memory Compaction & Loop Resilience Enhancements

1. Current State of the Orchestrator Loop

The loop refactor introduced a serialisable loopState (engine/llm/orchestrator/loop_state.go) with modular components for iteration tracking, budget enforcement, progress fingerprints, and memory metadata.
Restart logic (loop.go:541-575) now rebuilds the prompt template, resets counters, and replays the FSM via EventRestartLoop.
Progress detection fingerprints (progress.go) and configurable thresholds (config.go) are in place, while tests (loop_progress_test.go, loop_fsm_callbacks_test.go, fingerprint_progress_test.go) validate the FSM and restart pathways.
Memory compaction is surfaced through memoryManager.Compact (memory.go:184-200) but remains a stub; state flags in loop_state.go merely mark intent. 2. Code Review Summary (Gemini 2.5 Pro + O3)

Compaction stub risk: The hook executes but does nothing, leading to surprising behaviour if compaction is “enabled” for operators.
Retry/cooldown bookkeeping: When compaction fails, LoopState.Memory.CompactionSuggested remains true and LastCompactionIteration stays at zero, so the loop retries every iteration.
Restart residues: restartLoop clears prompt state but does not reset compaction flags, so a freshly restarted loop may immediately attempt compaction again.
Secondary notes: fingerprinting of raw tool text is sensitive to harmless whitespace; restart configuration silently clamps when RestartAfterStall > NoProgressThreshold. 3. Research & Reference Material

Anthropic, “Effective Context Engineering for AI Agents” (ai-docs/agentic/03-anthropic-context-engineering.txt) emphasises compaction as the first lever for long-horizon agents, recommending high-fidelity summarisation, careful tuning to avoid losing critical details, and aggressive clearing of stale tool outputs.
Perplexity survey (“LLM agent memory compaction strategies context window management”) corroborates Anthropic’s approach and highlights complementary tactics: structured note-taking, external memory stores, selective offloading of heavy artifacts, sub-agent isolation, and KV-cache level optimisations for long-context workloads. 4. Issue Deep Dive & Recommended Approach

4.1 Implementing Memory Compaction

Problem: memoryManager.Compact currently logs intent and returns nil. Coupled with the orchestration flags, this creates the illusion of compaction without delivering any safety benefit.
Desired behaviour: Safely condense conversation history and tool responses into a summary message (or series of notes) once usage crosses ContextCompactionThreshold.
Proposed design:
Introduce a compaction pipeline that:
Collects the last N user/assistant/tool messages from loopCtx.LLMRequest.Messages.
Builds a compaction prompt using Anthropic’s guidance—prioritise recall first, then trim—to produce a high-fidelity summary.
Clears or archives raw tool payloads; retains structured metadata or short redacted snippets (aligning with Anthropic’s “tool result clearing” recommendation).
Writes the summary back into:
The active request (replace long history with: “Summary (iteration X): …”).
External memory (e.g., call memory.StoreAsync with a compacted transcript).
Provide guardrails:
Feature flag: cfg.EnableContextCompaction with explicit default false until the routine is battle-tested.
Validation: if compaction prompt fails (LLM error) or memory.StoreAsync rejects, return a typed error for the loop to handle (see §4.2).
Files touched: memory.go, possibly introduce engine/llm/orchestrator/compaction.go, update loop.go for integration, extend tests under loop_progress_test.go.
4.2 Retry & Cooldown Bookkeeping

Problem: On failure, the loop retries compaction every iteration because LastCompactionIteration remains zero and CompactionSuggested never clears.
Proposed changes:
Extend memoryManager.Compact to return a sentinel error (e.g., ErrCompactionIncomplete). In tryCompactMemory, catch this error and:
Increment a failure counter (Memory.CompactionFailures).
Record the iteration (LastCompactionIteration = loopCtx.Iteration).
Honour compactionCooldown even after failures (i.e., do not attempt again until iteration ≥ LastCompactionIteration + cooldown).
Add metric/logging to surface repeated failures (e.g., warn after k consecutive unsuccessful attempts).
Update tests to validate:
Compaction attempts respect cooldown after failure (loop_progress_test.go).
Failure counters reset on success.
4.3 Restart-State Hygiene

Problem: restartLoop resets prompt guidance but leaves Memory.CompactionSuggested, PendingCompactionPercent, and CompactionFailures untouched.
Recommendation:
During restart, call a new helper (state.resetMemoryCompaction()), clearing CompactionSuggested, PendingCompactionPercent, CompactionFailures, and optionally LastCompactionThreshold.
Rationale: after summarising the trace, the loop should re-evaluate context usage afresh rather than assuming compaction is still necessary.
Add regression tests ensuring a restart yields CompactionSuggested == false. 5. Complementary Enhancements

Fingerprint normalisation: Before hashing raw tool results (progress.go), normalise strings (trim, collapse whitespace, optionally lower-case) to avoid misinterpreting cosmetic changes as progress.
Configuration ergonomics: Log a warning when RestartAfterStall is clamped to NoProgressThreshold; consider replacing the boolean flag + integer with a single sentinel-based field in the future.
Concurrency guard: Compaction and asynchronous memory storage should reuse a limited worker pool to avoid launching unbounded goroutines; track metrics for compaction latency/throughput. 6. Implementation Roadmap

Design Review: Draft the compaction prompt template, data flow, and failure handling. Validate requirements against Anthropic’s recommendations on precision vs. recall.
Core Compaction Module: Implement summarisation routine, integrate with memoryManager.Compact, and wire into tryCompactMemory.
Resilience Work: Add cooldown bookkeeping, restart-state reset, and logging/metrics.
Testing: Extend the orchestrator test suite with scenarios covering successful compaction, compaction errors, restart hygiene, and no-progress detection with normalised fingerprints.
Rollout Plan: Ship behind a feature flag, collect telemetry on compaction success rates, and tune thresholds per environment. 7. Relevant Files for Changes

engine/llm/orchestrator/memory.go
engine/llm/orchestrator/loop.go
engine/llm/orchestrator/loop_state.go
engine/llm/orchestrator/progress.go
engine/llm/orchestrator/config.go
engine/llm/orchestrator/loop_progress_test.go
engine/llm/orchestrator/loop_fsm_callbacks_test.go
Potential new helper modules under engine/llm/orchestrator/. 8. Open Questions

Should compaction summaries be stored solely in external memory or also preserved in long-term storage (e.g., task repository) for auditing?
Do we need different compaction strategies per agent/action type (e.g., summarise tool outputs differently from chat transcripts)?
How should operators configure compaction thresholds—one global value, or per agent via agent.ActionConfig? 9. Sources & Further Reading

ai-docs/agentic/03-anthropic-context-engineering.txt — Anthropic, Effective Context Engineering for AI Agents (compaction, structured note-taking, tool result clearing).
Perplexity research summary — LLM agent memory compaction strategies context window management (summarisation, offloading, multi-agent context isolation, KV-cache optimisations).
This document consolidates the current implementation status, external best practices, and a concrete plan to deliver reliable compaction, cooldown handling, and restart hygiene in the orchestrator loop.
