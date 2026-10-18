---
id: MS-web-session-simple-advanced-launch
area: MS
title: Start session separates launch details from the prompt composer
persona: Dora
journey: J-17
expected: Opening Start session without an explicit agent preselects the active workspace's default agent; starting from a worktree uses that worktree's workspace default, while an explicit agent choice wins. The dialog shows agent selection, with workspace and optional name in Advanced; it contains neither a first-message composer nor a runtime selector. Launch creates one durable session at the selected workspace root, activates its returned owner workspace, and navigates to its composer. Choosing another workspace clears only workspace-scoped launch selections. The session composer owns the "Next prompt" RuntimeSelector and its catalog state; the header carries the only close control.
entry_points: web desktop shell → Start session (dock, command palette, agent catalog, agent detail, dashboard)
qa_status: skipped
bug_ids: BUG-20260730-session-create-window-intent; BUG-20260827-session-create-first-message-regression; BUG-20260827-unbound-session-fast-inheritance
fix_status: fixed
retest_status:
fix_commits:
evidence: .compozy/tasks/modals-redesign/evidence/visual/task_02/VC-01; .compozy/tasks/modals-redesign/evidence/visual/task_02/VC-02; .compozy/tasks/modals-redesign/evidence/visual/task_02/VC-09;/Users/pedronauck/dev/qa-labs/compozy-ms-wave2-current-20260730-061842-796290-lab/qa-logs/qa;docs/qa/evidence/2026-07-30-session-runtime-selector/01-create-simple.png;docs/qa/evidence/2026-07-30-session-runtime-selector/02-create-advanced.png;docs/qa/evidence/2026-07-30-session-runtime-selector/04-session-open-after-create.png;/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/web-session-create-no-first-message.png;/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/web-session-create-advanced-no-first-message.png;/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/web-session-first-prompt-grok45-fast-pass.png
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: MS-web-entity-modal-shell
---

Opening Start session without an explicit agent preselects the active workspace's default agent; starting from a worktree uses that worktree's workspace default, while an explicit agent choice wins. The dialog shows agent selection, with workspace and optional name in Advanced; it contains neither a first-message composer nor a runtime selector. Launch creates one durable session at the selected workspace root, activates its returned owner workspace, and navigates to its composer. Choosing another workspace clears only workspace-scoped launch selections. The session composer owns the "Next prompt" RuntimeSelector and its catalog state; the header carries the only close control.

Walk each listed public entry point, then reload and read the stored result independently. Exercise rejection and recovery with the same workspace and profile to confirm that unrelated state remains intact.

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.
