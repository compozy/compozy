# BUG-20260822-tasks-loop-e2e-workspace-leak: Tasks Loop E2E writes definitions into the checkout

- **Status:** verified
- **Impact (user-side):** Blocks-Verification
- **Severity:** Medium · **Priority:** P0
- **Persona Affected:** Release QA operator
- **Journey Step:** Re-run the Tasks Loop browser contract
- **Scenarios:** Tasks calm default, reveal, and retained record journeys
- **Found:** 2026-08-22 · **Report:** docs/qa/reports/2026-08-21-loop-task-legibility.md
- **Origin:** task_07 release-grade QA

## Summary

The browser fixture resolved the repository root as its writable workspace. Publishing the test
Loop left a definition in the shared checkout, and the next fresh runtime failed with a duplicate
definition conflict.

## Reproduction

- **Invariant:** A browser E2E workspace is isolated and repeatable and never writes runtime state
  into the shared checkout.
- **Owning layer:** Tasks Playwright runtime fixture.
- **Canonical suite:** Loop record legibility group in `web/e2e/__tests__/tasks.spec.ts`.

## Fix

The group now creates one worker-scoped temporary workspace, seeds it into each isolated runtime,
and removes it after the group. The known QA residue was removed without touching other files.

## Verification

The retained Loop record journey reruns from a fresh isolated workspace without a 409 conflict.

