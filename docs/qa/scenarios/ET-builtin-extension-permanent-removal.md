---
id: ET-builtin-extension-permanent-removal
area: ET
title: Permanently remove and restore a built-in extension
persona: Vera
journey: J-extension-policy-admin
expected: An operator removes the bundled spec-cycle built-in permanently. The removal stays after a daemon restart, the removed list names it, and restore brings it back at its bundled version, enabled, with no restart. The same operations work over CLI, UDS and HTTP, the web Remove dialog and Removed built-ins section, and the agent native tools. Permanent removal is not offered to agents.
entry_points: compozy extension remove spec-cycle --permanent --global; compozy extension removed -o json; compozy extension restore spec-cycle --global; DELETE /api/extensions/spec-cycle?permanent=true (UDS); GET /api/extensions/removed-builtins (HTTP); POST /api/extensions/spec-cycle/restore; compozy__extensions_restore; compozy__extensions_removed_list; web Marketplace installed → spec-cycle → Actions → Remove… → Remove permanently; Removed built-ins → Restore spec-cycle
qa_status: pass
bug_ids:
fix_status: none
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-11-18-builtin-extension-permanent-removal.md
last_report: docs/qa/reports/2026-11-18-builtin-extension-permanent-removal.md
overlaps: ET-bundled-extension-trust
---

Operator journey for permanent removal of a bundled built-in. A plain `remove` is undone at the next
daemon boot. `--permanent` writes a tombstone in the same transaction as the removal, and boot skips a
tombstoned name until `restore` clears it.

Walks recorded on isolated labs: E2E-001, E2E-002 and E2E-004 on 2026-11-18; E2E-003 on 2026-11-19 (see
the report for the manifests and teardown):

- E2E-001 (CLI) — pass. `remove --permanent --global` → daemon stop and start → `extension list` omits
  `spec-cycle` → `extension removed` lists it → `restore --global` → `extension list -o json` shows
  `source: bundled`, `enabled: true`.
- E2E-002 (UDS and HTTP) — pass. UDS `DELETE /api/extensions/spec-cycle?permanent=true` returns 200 with the
  `extension` envelope; HTTP `GET /api/extensions/removed-builtins` lists it; HTTP
  `POST /api/extensions/spec-cycle/restore` returns `status: restored`. The payload shape follows the
  existing `ExtensionRemoveResponse` envelope, not the flat example in `_dx.md` (report F1).
- E2E-003 (agent) — pass (2026-11-19, lab 2, after the operator's manual login in the lab HOME). Agent
  `qa-restore-agent` (`claude`, `claude-haiku-5-5`, `approve-all`) in its session: `compozy__extensions_removed_list`
  lists `spec-cycle` and matches `extension removed -o json`; `compozy__extensions_restore` for `spec-cycle` returns
  `status: restored`, `source: bundled`, `version: 0.6.0`, `enabled: true`; the next
  `compozy__extensions_removed_list` returns `{"removed":[]}`, matching the CLI. `compozy__tool_search` for
  `permanent` returns no removal tool. The session's only extension-removal tool is `compozy__extensions_remove`,
  whose input is `name` and `owner` with `additionalProperties: false` and no `permanent` flag (report Session 3).
- E2E-004 (web) — pass. `web/e2e/__tests__/builtin-extension-removal.spec.ts` runs the dialog, the
  permanent choice with type-to-confirm, the Removed built-ins restore, and the installed list.

Refusals owned by the unit and integration suites in `_tests.md` (re-run by their owning tasks, not
re-walked in this pass): a marketplace or
local row is refused with `extension_not_builtin` (422); an unknown name is `extension_not_found` (404); a
built-in that still has other installations returns 409; restore of a non-removed built-in returns
`extension_builtin_not_removed` (409); restore over an operator copy returns `extension_exists` (409).
