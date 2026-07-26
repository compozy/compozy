# L-011 — "Fraco" test coverage is the most repeated pushback on generated `_tasks.md`

**Class:** Spec authoring
**Date discovered:** 2026-04-17 (recurring through 2026-04-26)
**Evidence sources:** 4 distinct Codex sessions with verbatim quotes

## Context

After running `$cy-create-tasks`, Pedro reads the generated `_tasks.md` and almost always pushes back on weak test plans. The pushback uses two specific BR-PT escalation markers — "fraco" (weak) and "leviano" (lazy). Verbatim quotes:

- 2026-04-17 13:10: _"olhando as tasks geradas, me parece que o numero de tests unitarios e integration tb estão bem fracos, ou é impressão minha"_
- 2026-04-17 18:02: _"o nivel de test unit e integration ficou fraco, melhore isso"_
- 2026-04-18 23:48: _"aprovado, na hora de criar as tasks, você deve não ser leviano nos units e integration tests quando criar a lista deles"_
- 2026-04-26: _"também é importante que cada task tenha uma boa cobertura de tests feitas corretamente, não ser leviano ou colocar apenas 1 ou 2 tests, mas fazer da forma correta $no-workarounds"_

This is the single most consistent correction Pedro issues against `cy-create-tasks` output.

## Root cause

LLMs default to a "good-enough" test density: 1-2 unit tests per behavior, sometimes a single "integration smoke" entry. AGH's behavior count per task is materially higher (lease invariants, concurrency stress, failure-path cleanup, contract drift, security redaction). A lazy default produces tasks that pass `make verify` but leak issues into review rounds — exactly the pattern that drives 40%+ of CodeRabbit issues. → `lessons/L-002` for the test-shape side.

## Rule

> When generating `_tasks.md`, count behaviors documented in the TechSpec (lease invariants, error paths, concurrency cases, security cases, observability events) and plan tests proportional to that count. Reject lists with 1-2 tests for many behaviors. Use `agh-test-conventions` to enforce shape; this lesson governs density.

This lesson governs density only after a test is justified. It does not authorize tests per task, per file, or per implementation detail. Every proposed test must first name the invariant, owning layer, and canonical suite (`consolidate-test-suites`). "No new automated test" is valid when an existing suite, lint rule, codegen check, typecheck, build, visual QA, or documented manual evidence already owns the invariant.

## Operationalization

Before approving a generated `_tasks.md`:

1. For each task, count the behaviors named in the TechSpec section it implements.
2. For each behavior, name the invariant, owning layer, and existing canonical suite before adding test cases.
3. Cross-check the proposed test count: at minimum 1 happy-path + 1 failure-path + 1 concurrency case (when relevant) + 1 contract/redaction case (when wire-affecting). Aim higher when the task is `critical` complexity.
4. If the task body says only "unit tests for the new functions", expand to enumerated assertions or replace it with a no-new-test rationale.
5. For QA-gating tasks (the trailing `qa-execution`): test cases enumerate every public surface touched, not a single smoke pass.

## Anti-patterns

- "Tests will be added during implementation."
- "Smoke test the new endpoint."
- "Cover the happy path."
- "Add unit tests for `Foo()`."
- "Add unit and integration tests because every task needs tests."
- "Pin every CSS/prose/generated literal so it cannot drift."

These are pre-rejected — they will produce a "fraco" pushback.

## Source

- `~/.codex/sessions/2026/04/17/...` (multiple turns 13:10, 18:02)
- `~/.codex/sessions/2026/04/18/...` (turn 23:48)
- `~/.codex/sessions/2026/04/26/...` (autonomy `_tasks.md` review)
- `../analysis/analysis_codex_sessions.md` §Recurring Theme 5
- `docs/_memory/_synthesis.md` Top-level Finding 3
