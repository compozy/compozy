---
id: RT-worktree-web-exit-ladder
area: RT
title: Run the assisted exit from the worktree context
persona: Ada
journey: J-worktree-management
expected: The worktree context shows a five-field status strip that renders unknown facts as unknown, hides ahead/behind without a known upstream, and hides the PR cell with no forge. The exit control renders the daemon's plan verbatim: its primary position, its per-row blocked reasons, and a global pause that blocks every row with the same literal. A status read failure blocks the whole control and is stated in the strip.
entry_points: S1 Workspace menu -> worktree nest -> Context; S5 status chip
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/cli-worktree-dirty-exit-remove.jsonl; web/e2e/__tests__/worktrees.spec.ts
last_report: docs/qa/reports/2026-08-13-worktree-support.md
overlaps: RT-worktree-exit-commit-scope; RT-worktree-api-surface-parity
---

QA impact: Task 07 adds the worktree detail context, status strip, and daemon-rendered exit control.

2026-08-14 layout: the detail host is compact (`sm` 560), unframed, and uses the token stack
(`gap-4 px-5 py-4`). The header path truncates with a tooltip. Ladder verbs are unchanged.

2026-08-15 entry move: the Command-Tab Workspaces overview drops the Context row action; the
worktree context opens from the menubar Workspace menu nest (or the status chip). The context
surface itself is unchanged.
