# BUG-20260822-empty-briefing-artifacts-null: Empty briefing artifacts crash the run page

- **Status:** verified
- **Impact (user-side):** Blocks-Verification
- **Severity:** High · **Priority:** P0
- **Persona Affected:** Loop operator
- **Journey Step:** Open a live run before it has produced an artifact
- **Scenarios:** LP-web-run-default-read-briefing; LP-web-run-operator-register
- **Found:** 2026-08-22 · **Report:** docs/qa/reports/2026-08-21-loop-task-legibility.md
- **Origin:** task_07 release-grade QA

## Summary

The public briefing encoded an empty artifact collection as `null`. The web contract treats
`artifacts` as a stable array and mapped it while rendering a live run, so the whole Loops window
failed with `Cannot read properties of null (reading 'map')`.

## Reproduction

- **Invariant:** Public briefing collection fields serialize as arrays, including when empty.
- **Owning layer:** Loop briefing projection.
- **Canonical suite:** `TestBriefingContract` in `internal/loop/briefing_test.go`.

## Fix

The projection now copies artifacts into a non-nil empty slice, preserving `[]` on the wire.

## Verification

The canonical briefing contract test passed, and the fresh daemon-served browser lane passed with
227 tests, 3 declared conditional skips, and 0 failures. The live empty-artifact run page rendered
without an exception.
