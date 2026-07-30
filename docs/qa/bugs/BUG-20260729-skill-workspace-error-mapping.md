# BUG-20260729-skill-workspace-error-mapping: Missing skill workspace returned an internal error

- **Status:** open
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Bruno
- **Journey Step:** J-agent-marketplace-parity, resolve the installed skill catalog for a workspace
- **Scenarios:** ET-001
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-28-untested-full.md
- **Origin:** Fresh isolated two-workspace skills replay

## Summary

Resolving the skills list for a nonexistent workspace returned HTTP 500 even though the workspace resolver preserved `ErrWorkspaceNotFound`. Operators and agents therefore received an internal-error classification instead of a stable not-found response.

## Reproduction

1. Start the isolated daemon with a registered workspace and skills registry.
2. Request `GET /api/skills?workspace=does-not-exist` over the operator UDS.
3. Observe the status and error body.

**Expected:** HTTP 404 with the preserved workspace-not-found diagnostic.
**Actual before the fix:** HTTP 500 with `workspace: lookup workspace by name "does-not-exist": workspace not found`.

## Evidence

- Before/after public-surface replay: `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/011-skill-scope-content`.
- The canonical regression failed before the production change with `status = 500, want 404`, then passed under `-race`.

## Fix

- **Root cause:** `StatusForSkillError` mapped only skill-domain sentinels. Workspace resolution errors flow through every scoped skill handler, but the mapper fell through to 500 even when `errors.Is` still matched a workspace sentinel.
- **Correction:** The shared skill error mapper now preserves workspace not-found, missing-root, and resolver-unavailable semantics as 404, 410, and 503 respectively.
- **Fix commit:** pending completion gate
- **Regression test:** `TestListSkills/Should preserve workspace not found status` in the canonical `internal/api/core/skills_test.go` suite.

## Verification

- `CGO_ENABLED=1 go test -race ./internal/api/core -run 'TestListSkills/Should_preserve_workspace_not_found_status' -count=1` passes.
- The rebuilt isolated daemon returns 404 for the original UDS request; the surrounding skill scope, content, trust, and tombstone assertions remain green.
