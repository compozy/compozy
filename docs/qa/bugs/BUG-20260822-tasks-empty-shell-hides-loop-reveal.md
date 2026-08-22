# BUG-20260822-tasks-empty-shell-hides-loop-reveal: Empty Tasks state hides the Loop reveal control

- **Status:** verified
- **Impact (user-side):** Blocks-Operation
- **Severity:** High · **Priority:** P0
- **Persona Affected:** Ada, Loop operator
- **Journey Step:** Reveal Loop records in a workspace with no ordinary work
- **Scenarios:** Tasks calm default and reveal
- **Found:** 2026-08-22 · **Report:** docs/qa/reports/2026-08-21-loop-task-legibility.md
- **Origin:** task_07 release-grade QA

## Summary

When the ordinary Tasks population was empty, the route returned only the onboarding empty state.
The stable list shell disappeared with its Work/Loop filter, making hidden Loop records impossible
to reveal in a Loop-only workspace.

## Reproduction

- **Invariant:** A zero-work Tasks catalog keeps the list shell and reveal control while presenting
  the calm onboarding state.
- **Owning layer:** Tasks route composition.
- **Canonical suite:** `tasks-catalog-location.test.tsx`.

## Fix

The empty state now renders inside the shared Tasks listing surface, preserving the reveal control.

## Verification

- Canonical route-composition unit suite: 3 passed.
- Real daemon-served retained Loop record journey: passed with the Loop-only catalog revealed.

