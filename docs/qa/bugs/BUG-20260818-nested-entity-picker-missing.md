# BUG-20260818-nested-entity-picker-missing: Nested reviewer answer falls back to raw JSON

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Bruno
- **Journey Step:** J-supervise-loop-request, answer the waiting reviewer question
- **Scenarios:** LP-answer-typed-request-entities
- **Found:** 2026-08-18 · **Report:** docs/qa/reports/2026-08-18-typed-loop-inputs.md

## Summary

Bruno opened a request whose nested `assignment.reviewer` field was annotated as an agent. The Web
form showed one raw JSON text box instead of the shared agent picker, so Bruno had to know the JSON
shape and type the agent name by hand.

## Reproduction

- **Charter:** CH-typed-request-entity-answer · **Tour:** Network Tour
- **Environment:** isolated macOS lab / desktop Web / wifi-fast / en-US

1. Start `typed-request-qa` and open its Needs You card.
2. Inspect the control generated from `assignment.reviewer` with `x-compozy-kind: agent`.

**Expected:** The nested field renders the same searchable agent picker used by Loop inputs.
**Actual:** The entire `assignment` object renders as one raw JSON text box.

## Evidence

- `docs/qa/reports/2026-08-18-typed-loop-inputs.md`
- Independent CLI replay proved the daemon already rejects `missing-reviewer` at the exact
  `assignment.reviewer` path without closing the request.

## Fix

- **Root cause:** the Web schema projector inspected only top-level properties; it classified the
  parent object as JSON and never visited the nested vendor annotation.
- **Fix commit:** pending implementation commit
- **Regression test:** `web/src/systems/loops/lib/__tests__/loop-request-payload.test.ts` and the
  canonical request-form case in `web/src/systems/loops/components/__tests__/loop-run-page.test.tsx`.

## Verification

- A fresh run and fresh browser session rendered `assignment.reviewer` as the searchable agent
  picker, selected `reviewer`, submitted once, reached `done`, and left no live request in an
  independent CLI read.
