---
id: ET-web-session-pr-chip
area: ET
title: Read a session's pull request in its window topbar and act on it from the chip
persona: Bruno
journey:
expected: A session on a non-default branch shows a chip first in its window topbar status zone, where the agent, provider and time used to be (the origin link, archived word, unrestricted badge, worktree chip and transport chip keep their places). Without a branch nothing renders. With a branch but no PR the chip is the branch name with one lookup marker — pending, unavailable (tooltip names the cause, e.g. "GitHub not connected") or "Couldn't match a PR for this branch" — and a confirmed "no PR" shows the bare branch. With a PR it reads the PR shape, `#number`, Open/Draft/Merged/Closed and at most two signals; color appears only for failing checks, changes requested, conflicts and "Ready to merge", which shows only when every fact is known and fresh. Stale or failed-lookup facts keep their words in muted ink with a clock marker and a tooltip naming the cause and when they were last checked; the tooltip lists the checks breakdown, review, mergeability and up to five failing checks. The chip menu leads with the managed worktree's exit action (Commit & push / Push / Create PR through the existing exit dialogs and progress), else "Fix failing checks", else "Address review comments", else Open PR; then Open PR, Refresh (with the last check time) and any remaining "Ask the agent" prompt. Unavailable actions are absent; a session the operator cannot control gets Open PR and Refresh only. A prompt is one normal user message naming the PR number, URL and head (plus the failing checks), queued behind a running turn and resuming a stopped session. Refresh spins the chip and lands fresh facts, or keeps the cached facts and toasts the cause. The chip re-reads every 60 s while the document is visible.
entry_points: web session window topbar (session-pull-request-chip, session-pull-request-menu); GET /api/sessions/{id} (`branch`, `pull_request`, `pull_request_lookup`); POST /api/sessions/{id}/pull-request/refresh; worktree exit plan + WorktreeCommitDialog / WorktreePrDialog
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-route-chrome-topbar; ET-web-sessions-board-toggle; RT-web-attention-toast-delivery
---

Introduced by the Sessions Kanban spec (`.compozy/tasks/sessions-kanban`, US-004..US-008, E2E-001, E2E-003, visual contract VC-01..VC-18).

Walk (real GitHub credential, disposable repository): open a session working on a fresh feature branch; the topbar shows `⎇ <branch>` with the pending marker, then the bare branch once the lookup confirms no PR. Ask the agent to open a draft PR; after the turn settles the chip reads `#N · Draft` with the pending-checks spinner. Let CI fail: the chip shows "N failing" in danger and its menu leads with "Fix failing checks"; send it and confirm one user message naming `#N`, the URL, the head SHA and the failing checks. Mark the PR ready and approve it with green checks: the chip shows "Ready to merge" only after a fresh read. Disconnect GitHub and press Refresh: cached facts stay, a toast names the cause, and the chip gains the stale clock. In a managed worktree with uncommitted changes the menu leads with the exit plan's action and runs it through the existing commit/PR dialogs. A spawned (non-controllable) session's menu has Open PR and Refresh only.

Owning automated coverage: E2E-001 and E2E-003 (`web/e2e/__tests__/session-pull-request-chip.spec.ts`, store-seeded cache rows), `lib/__tests__/pull-request.test.ts` (UT-060..UT-064, UT-073), `use-session-topbar-slot.test.tsx` (chip placement, menu primary, non-controllable menu), `query-options.test.ts` (UT-072). The real-forge refresh and exit-action legs are not covered by the seeded browser walk.
