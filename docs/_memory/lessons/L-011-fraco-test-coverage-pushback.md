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

LLMs default to a "good-enough" test density: 1-2 unit tests per behavior, sometimes a single "integration smoke" entry. Compozy's behavior count per task is materially higher (lease invariants, concurrency stress, failure-path cleanup, contract drift, security redaction). A lazy default produces tasks that pass `make verify` but leak issues into review rounds — exactly the pattern that drives 40%+ of CodeRabbit issues. → `lessons/L-002` for the test-shape side.

## Rule

> Plan tests from distinct observable invariants and failure modes. Name the owning layer and canonical suite before adding a case; existing tests or stronger gates can satisfy the contract without new coverage.

## Operationalization

- Identify which behavior could fail and which existing test/probe would expose it.
- Add happy, error, concurrency, security, or wire cases when each represents a distinct relevant risk; no minimum test-count quota.
- Replace vague "unit tests for functions" with concrete assertions or a brief explanation of the existing owning evidence.
- QA covers affected public journeys and integration gaps; it reuses valid slice evidence rather than repeating every case.

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
