---
id: NB-bridge-restart-recovery
area: NB
title: Recover unfinished bridge delivery after restart
persona: Omar
journey: J-recover-mid-turn-restart
expected: After a daemon restart, every durable active bridge delivery is reconciled within its exact scope and workspace before new prompt or delivery registration side effects; a row with no unmatched send intent receives one visible standard terminal error post without replaying persisted text, while a row whose sent sequence is ahead of its acknowledged sequence is terminalized locally as indeterminate without another provider call; recovery never advances `last_success_at`; a terminal metric-persistence error for one removed or conflicting instance is exposed through redacted health without stopping later healthy-instance metric writes; and the Web detail renders only daemon-owned backlog, failure, drop, authentication-failure, last-success, and active-route values.
entry_points: Daemon restart during public bridge response delivery; bridge channel; delivery health metrics
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /home/pedronauck/dev/qa-labs/agh-hermes-bridge-task-10-20260713-022226-583543-lab/qa-artifacts/qa/notes/bridge-charter-results.json
last_report: docs/qa/reports/2026-07-12-hermes-bridge.md
overlaps: NB-bridge-tool-progress; NB-long-bridge-replies; NB-provider-progress-rendering
---

An operator or teammate sees an unfinished streamed response terminate visibly after a daemon
restart instead of being silently abandoned.

Added by the Hermes bridge Task 06 impact flag. Task 09 assigned it to `J-recover-mid-turn-restart` and `CH-mid-turn-bridge-restart`; Task 10 owns execution. Planning flag only; no QA session ran.

QA 2026-07-13: broker, GlobalDB, boot, fresh-broker, and exact daemon restart evidence produced one visible terminal post, no text replay, durable metrics, and reconciled ownership. The full provider matrix remains in the automation backlog.

Phase D impact flag 2026-07-13: restart reconciliation no longer fabricates a successful-delivery timestamp; terminal persistence errors for deleted or conflicting instances no longer stop metrics for healthy instances and are exposed as redacted health errors. The Web removed fabricated `Events (24h)` and `Success rate` tiles and now renders only daemon-owned values. Status reset to `untested`; historical restart evidence remains intact. No QA retest ran.

Phase D R-001 impact flag 2026-07-13: write-ahead sent intent now prevents a second provider call after an unresolved mutation; restart reconciliation terminalizes that row locally as indeterminate. Reconciliation still posts one visible terminal error when no unmatched intent exists. Status remains `untested`; historical restart evidence remains intact. No QA retest ran.
