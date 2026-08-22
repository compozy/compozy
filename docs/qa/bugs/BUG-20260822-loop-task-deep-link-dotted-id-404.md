# BUG-20260822-loop-task-deep-link-dotted-id-404: Loop task deep links return 404 for canonical dotted IDs

- **Status:** verified
- **Impact (user-side):** Blocks-Operation
- **Severity:** High · **Priority:** P0
- **Persona Affected:** Ada, Loop operator
- **Journey Step:** Open a Loop task record directly from Tasks
- **Scenarios:** Tasks reveal and retained Loop record journey
- **Found:** 2026-08-22 · **Report:** docs/qa/reports/2026-08-21-loop-task-legibility.md
- **Origin:** task_07 release-grade QA

## Summary

Canonical Loop task IDs contain dots. The daemon's static fallback treated every last path segment
containing a dot as a missing asset, so a valid `/tasks/<loop-task-id>` browser navigation returned
plain `404 page not found` instead of the SPA shell.

## Reproduction

- **Invariant:** An HTML navigation to a valid SPA route remains a deep link even when its opaque ID
  contains dots.
- **Owning layer:** HTTP static route fallback.
- **Canonical suite:** `internal/api/httpapi/static_test.go`.

**Expected:** The daemon serves `index.html` for the Tasks deep link.
**Actual:** The daemon returns HTTP 404 because the task ID looks like a filename.

## Fix

The fallback now honors HTML navigation requests before applying the extension heuristic. Existing
assets still resolve first, and missing asset/API routes remain 404.

## Verification

- Canonical static-route suite: 9 passed.
- Real daemon-served retained Loop record Playwright journey: 1 passed in 49.2 seconds.

