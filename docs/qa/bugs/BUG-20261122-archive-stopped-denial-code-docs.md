# BUG-20261122-archive-stopped-denial-code-docs: Archive stopped docs promise the wrong denial code for all_workspaces

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Théo
- **Journey Step:** J-archive-session-without-deleting, "Archive every stopped thread (#1924)" step 5
- **Scenarios:** RT-session-archive-catalog
- **Found:** 2026-11-22 · **Report:** docs/qa/reports/2026-11-22-archive-stopped-1940.md

## Summary

An agent whose workspace access is restricted calls `compozy__session_archive_stopped` with
`all_workspaces: true`. The bundled `compozy` skill reference (`native-tools.md`) and the RT scenario told
agents to expect `capability_denied` with reason `needs_grant`; the daemon returns `tool_denied` with
`reason_codes: ["workspace_access_denied"]`, the same code every other native workspace denial uses. An agent
branching on the documented code would miss the denial.

## Reproduction

- **Environment:** isolated lab daemon built from `db3f92bd5`, mock ACP agent with `approve-reads`

1. Create an agent session in a project (its permission policy is limited to that project's path).
2. Have it call `compozy__session_archive_stopped` with `{"all_workspaces": true, "dry_run": true}`.

**Expected (docs):** `capability_denied`, reason `needs_grant`.
**Actual:** `{"code": "tool_denied", "reason_codes": ["workspace_access_denied"]}`; nothing archived.

## Evidence

- `/home/jilles/dev/qa-labs/compozy-archive-stopped-1940-20261122-185609-503208-lab/qa-artifacts/evidence/09-tool-all-denied.json`

## Fix

- **Root cause:** the docs described the internal `workspaceaccess.DenialNeedsGrant` decision instead of the
  public error it maps to (`NewWorkspaceAccessDeniedError`); `TestNativeSessionArchiveStoppedTool` already pins
  the wire as `ErrToolDenied` + `ReasonWorkspaceAccessDenied`.
- **Fix commit:** `12e4be26d` (`fix: align the Archive stopped denial docs and tool summary with the wire`).
- **Regression test:** none added — doc-only; the existing native tool test owns the wire shape.

## Verification

- **Retested:** 2026-11-22 by reading the corrected reference against the captured denial payload · **Report:**
  docs/qa/reports/2026-11-22-archive-stopped-1940.md
