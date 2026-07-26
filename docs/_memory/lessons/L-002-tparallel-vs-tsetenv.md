# L-002 — `t.Parallel()` is incompatible with `t.Setenv`

**Class:** Testing
**Date discovered:** 2026-04-09 (PR 10 CodeRabbit cycle); reinforced 2026-04-15+
**Evidence sources:** 3 analyses concur

## Context

CodeRabbit and other reviewers repeatedly flag missing `t.Parallel()` calls in AGH tests. In several cases, the tests use `t.Setenv` to inject configuration; adding `t.Parallel()` to those tests _breaks them_ because Go's testing package explicitly forbids the combination — `t.Setenv` panics if any parent or sibling test is parallel.

`TestHooksConcurrentRebuildAndDispatch` is the canonical example: it stayed serial under `make verify` because adding `t.Parallel()` amplified an unrelated flake (a timing-sensitive concurrent-rebuild assertion).

## Root cause

Reviewers (especially LLM-based reviewers) apply "always add `t.Parallel()`" as a default. Go's runtime contract is stricter: tests that mutate process-global state via `t.Setenv` or that depend on shared mutable state are correctly serial.

## Rule

> Independent subtests MUST call `t.Parallel()`. Tests that use `t.Setenv` (directly or transitively) MUST NOT call `t.Parallel()`. Reject reviewer suggestions to add `t.Parallel()` to env-mutating tests as INVALID — with concrete evidence (the `t.Setenv` callsite).

## Triage protocol

When a reviewer flags missing `t.Parallel()`:

1. Search the test for `t.Setenv` (or any helper that calls it).
2. If found, mark INVALID with rationale: "`t.Setenv` is incompatible with `t.Parallel()` per Go testing contract; keeping serial."
3. If not found AND the test does not depend on shared state, add `t.Parallel()`.
4. If shared state is suspected, document the ownership rationale rather than adding parallelism.

## Source

- `.codex/ledger/2026-04-06-MEMORY-pr5-coderabbit.md`
- `.codex/ledger/2026-04-09-MEMORY-pr10-coderabbit.md` and `pr10-review-fix.md`
- Multiple Hermes round-2 review issues
- `../analysis/analysis_codex_ledger.md`, `../analysis/analysis_global_runs.md`, `../analysis/analysis_compozy_tasks.md`
