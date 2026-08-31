---
id: ET-terminal-journal-recording
area: ET
title: Inspect command history and recordings
persona: Dora
journey: J-audit-terminal-work
expected: Journal filters change the server query, approximate boundaries are labeled, filtered misses are distinct from an empty history, and a recording replays from its owning profile with its retention stated.
entry_points: Terminal window head Journal toggle; terminal journal CLI; terminal recording download; terminal selection to active session composer
qa_status: pass
bug_ids: BUG-20260826-terminal-journal-workspace-id; BUG-20260826-terminal-config-set-unsupported
fix_status: fixed
retest_status:
fix_commits: b745ebcbcfe6
evidence: web/e2e/__tests__/terminal.spec.ts E2E-007; docs/qa/reports/2026-08-31-terminal-stabilization.md
last_report: docs/qa/reports/2026-08-31-terminal-stabilization.md
overlaps:
---

reset 2026-08-31: the Journal opens from the window head's toggle (with a back affordance) — the journal tab no longer exists; the prior verdict predates that surface.
Flagged by integrated-terminal task 06. Task 10 owns the real-user walk, evidence, and verdict.

2026-08-30 CI repair flag: a visible agent-owned exec could publish its exit before its exact
journal row became queryable. Reset for E2E-003 after terminal finalization is reordered.

2026-08-30 CI repair re-walk: passed 5/5. E2E-003 observed the exited visible terminal and then
queried the same terminal's `approved_once` row on every isolated run; the owning manager package
also passed completely under `-race`.

Walk:

1. Run exact and estimated commands, then open the Journal from the window head's toggle.
2. Filter by actor, time, terminal, and failed result; clear one filter and then all filters.
3. Verify an empty filtered result does not claim the project has no history.
4. Replay a recording and return to its journal row; compare the CLI and browser record.
5. Select a bounded terminal range and send it to the active session composer; verify the `<terminal_context>` source terminal, line range, and untrusted marker survive sending.

2026-08-30 CI repair re-walk: passed. Current-tree E2E-001 and E2E-020 proved that Bash prompt
integration persists the operator's real command rather than its own prompt hook, both through the CLI
journal and the profile-scoped Web journal. The PTY marker regression passed under `-race`.
