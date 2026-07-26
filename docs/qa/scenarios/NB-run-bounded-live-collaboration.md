---
id: NB-run-bounded-live-collaboration
area: NB
title: Run bounded Live collaboration without duplicate wakes
persona: Ada
journey: J-run-bounded-live-collaboration
expected: An explicitly Live execution durably accepts eligible direct messages and mentions, coalesces one causal burst, prompts the target once with untrusted Network context, settles actual usage or a truthful canceled/deadline outcome, recovers queued wakes after restart, and accumulates without prompting at depth or total-budget ceilings.
entry_points: HTTP/UDS/CLI/native execution start; Network thread and direct send; network usage and conversation reads; daemon restart
qa_status: pass
bug_ids: BUG-20260715-network-usage-workspace-name-empty;BUG-20260715-network-wake-restart-target-stopped;BUG-20260715-taskless-network-wake-run-unreadable
fix_status: fixed
retest_status: pass
fix_commits: pending local diff
evidence: docs/qa/evidence/2026-07-14-network-changes/ch-live-bounds-agent-path.md;/Users/pedronauck/dev/qa-labs/agh-network-live-bounds-20260715-061317-610983-lab/qa-artifacts/qa/teardown.json
last_report: docs/qa/reports/2026-07-14-network-changes.md
overlaps: NB-execution-participation-defaults;NB-020;RT-073
---

Planning flag for the Task 04 Live executor. The next targeted QA cycle should compare a Local control run with one explicit Live run, send a ten-message same-root burst plus a depth-capped reply, interrupt one wake through disable/cancel, restart with one admitted-but-unclaimed wake, and reconcile conversation, task-run, ledger detail, and aggregate usage after each branch.

Taxonomy note: this scenario owns runtime admission, cancellation/restart, exhaustion, usage, and workspace isolation. The browser-visible invitation and conversation panel are settled separately by `NB-coordination-invitation-future-runs` and `NB-run-conversation-bounds-usage`.

QA 2026-07-15: deterministic Live execution passed direct/mention admission, burst coalescing, depth and budget exhaustion, cancellation, replay idempotency, actual-or-unavailable usage, and exactly-once restart recovery. Three defects found during the walk were corrected and retested in the same isolated lab. The wider charter remains blocked on separate agent-surface/manual-extension coverage, but this runtime-owned scenario passed.
