# L-007 — E2E harness regressions follow runtime contract changes

**Class:** Testing
**Date discovered:** 2026-04-26 (autonomy task_18 QA)
**Evidence sources:** task_18 BUG-001/002/003 + global_runs

## Context

The autonomy MVP `make verify` passed. Real-scenario QA via `real-scenario-qa` then surfaced three Playwright/E2E regressions:

- **BUG-001** — workspace onboarding race in the web E2E `TC-AUTO-015` test; absent shared workspace-onboarding wait helper.
- **BUG-002** — `acpmock` exact-match canonicalization for situation-augmented prompts. Task 04 added a situation-context augmenter that changed the rendered prompt; the deterministic ACP mock fixture matcher still expected the pre-Task-04 shape.
- **BUG-003** — Tasks browser E2E asserting an empty Agents-panel state; manual-first publish actually rendered an active run.

All three were rooted in tests written against an _older_ runtime contract. None was a production bug.

## Root cause

When a runtime contract changes — a new prompt augmenter, a different fixture canonicalization, a new manual-first UI state — the deterministic test infrastructure (acpmock fixtures, Playwright selectors, browser fixtures) embeds the _old_ contract. `make verify` passes because tests still hit their old expectations. Real-scenario QA exposes the drift.

## Rule

> When a runtime contract changes (prompt augmenter, situation context, fixture format, manual-first UI state), the E2E mock and matchers ship in the same PR. Do not let the test infrastructure encode a stale contract.

## Operationalization

- For ACP fixture work: replace fragile string-matching with structured prompt metadata. acpmock uses typed metadata, not rendered prompt substrings.
- For Playwright E2E: add shared wait helpers (`web/e2e/fixtures/selectors.ts`) for workspace onboarding, session creation, manual-first publish. New runtime states require helper updates in the same PR.
- Real-scenario QA is the canonical regression net. `make verify` is necessary but not sufficient.
- E2E regressions surfaced in the QA pass are NOT production bugs unless they reveal divergent runtime behavior. Fix the test infrastructure, not the runtime.

## Anti-pattern

- Adding a `time.Sleep(2 * time.Second)` to "stabilize" a flaky Playwright spec.
- Loosening an acpmock matcher to substring instead of metadata.
- Skipping E2E in the QA pass because "the unit tests cover it."

## Source

- `.compozy/tasks/autonomous/memory/task_18.md`
- `.compozy/tasks/autonomous/qa/verification-report.md`
- `.compozy/tasks/autonomous/qa/issues/BUG-001.md`, `BUG-002.md`, `BUG-003.md`
- `.codex/plans/2026-04-17-e2e-confidence-hardening.md` — root-cause plan for the structured-metadata switch
- `../analysis/analysis_compozy_tasks.md` (task_18 findings), `../analysis/analysis_global_runs.md` (autonomy QA section)
