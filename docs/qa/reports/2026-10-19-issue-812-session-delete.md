# Issue #812: deleted session window retirement

Scope: targeted Web/API walk as Théo, desktop power user. Real isolated daemon (installed 0.3.0-beta.35) and this branch's Vite app, 1440 × 900, dark theme. No API mocks, provider prompts, local E2E suites, or global gates. The orchestrator explicitly authorized a single focused Playwright headless walk after CUA timed out.

Charter: keep retained session tabs open, delete individual or selected sessions through Session → Sessions, and keep the empty session surface. Compare a single-profile view with All profiles. Canary: a valid foreign-profile session retains its owner notice. Saved window-manager bindings are checked through the public snapshot API.

| Journey | Status | Evidence |
| --- | --- | --- |
| Single list deletion in default | Pass | [Before](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-812/2026-10-19/profile-single-before.png), [after](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-812/2026-10-19/profile-single-after.png) |
| Bulk list deletion in default | Pass | [Before](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-812/2026-10-19/profile-bulk-before.png), [after](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-812/2026-10-19/profile-bulk-after.png) |
| Single list deletion in All profiles | Pass | [Before](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-812/2026-10-19/aggregate-single-before.png), [after](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-812/2026-10-19/aggregate-single-after.png) |
| Bulk list deletion in All profiles | Pass | [Before](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-812/2026-10-19/aggregate-bulk-before.png), [after](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-812/2026-10-19/aggregate-bulk-after.png) |
| Foreign-profile canary | Pass | [Owner banner](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-812/2026-10-19/foreign-profile.png); scoped API 404, aggregate API 200 |

## Session debriefs

Each target was created through the public API, opened through its public deep link, and deleted from the real Sessions catalog. Every target ended at **No session selected**, without **Couldn't find this session**. Independent by-id HTTP reads returned 404; the daemon's window-manager snapshot retained the tab with its session instance cleared. The bulk default walk also retired several pre-existing background session tabs.

The first walk exposed an SSE race: a by-id 404 could invoke remote retirement while the successful local delete was still retargeting the same window. That produced an unnecessary **Session not found** toast. The fix now uses the mutation cache's per-session pending state through async host reconciliation. All four combinations were re-walked with an additional assertion rejecting that toast.

Foreign history stayed read-only under **This session belongs to personal**, with **Switch to personal**. A new direct link to an already-deleted session still follows the existing route-loader behavior (toast then redirect to the agent page); it does not enter the open-window retirement flow. This unchanged route behavior is outside this fix. Some walks displayed the shell's existing **Layout changed elsewhere. Refreshed.** indicator while restoring shared layout revisions; no missing-session notice remained.

## Verification

- `bunx turbo run build --filter=@compozy/client --only`: local dependency built.
- `bunx turbo run test --filter=./web --only --` followed by the five affected session-window, routing-coordinator, session-action, agent-detail, and sidebar suites: **104 tests passed**.
- `bunx turbo run typecheck lint --filter=./web --only`: passed, zero lint warnings/errors.
- React Doctor diff scan: no diagnostics; remote numeric score unavailable because its score API could not be reached.
- Targeted real QA strict evidence auditor: **PASS**, zero blockers and warnings.
- `git diff --check`: passed.

Turbo's optional shared-cache write emitted an IO permission warning after successful tasks; the tests/typecheck/lint themselves exited zero. Heavy local gates and full E2E suites were explicitly excluded; current-head PR CI owns the delivery gate. No backend production code changed, so the installed real daemon was reused inside the unique lab rather than building another backend.

## Final status

Targeted QA passed. PR CI and both external reviews are tracked in the PR; this QA report alone is not a PR delivery claim. The manifest-driven teardown completed with `TEARDOWN_CLEAN=true` and `TEARDOWN_ALL_CLEAN=true`; no owned lab processes remain.
