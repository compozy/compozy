---
id: TA-agent-knowledge-refresh-on-wake
area: TA
title: Refresh changed workspace knowledge on a later worker wake
persona: Bruno
journey: J-refresh-agent-knowledge
expected: An active task-role worker observes changed workspace knowledge on its next eligible wake and acts on the new signal without a second operator prompt.
entry_points: workspace knowledge; task-role wake; hosted native task lease; Network channel
qa_status: pass
bug_ids: BUG-20260729-agent-knowledge-refresh-missed
fix_status: fixed
retest_status: pass
fix_commits: pending final whole-diff commit
evidence: /home/pedronauck/dev/qa-labs/compozy-knowledge-refresh-on-wake-20260803-025914-822792-lab/qa-artifacts/qa/knowledge-refresh-evidence.json
last_report: docs/qa/reports/2026-08-02-knowledge-refresh-on-wake.md
overlaps: TA-task-role-session-activation
---

Task 11 changed the Data Scientist's event-volume knowledge from `first_save: 7812` to
`first_save: 0`. The session processed three later review wake turns but did not re-read or report
the new value within the five-minute recovery window. No follow-up operator prompt was sent.

This scenario owns knowledge freshness across turns. TA-task-role-session-activation continues to
own activation, native claim, and single-run execution.

## Implemented correction

The daemon now reopens bounded regular Markdown files under the session workspace's `knowledge/`
tree for every eligible user, Network, and synthetic turn. The current bytes and a revision digest
travel in the live prompt context; symbolic links are excluded and cannot escape the workspace.

The canonical integration regression mutates one knowledge file between two synthetic wakes and
proves that the second wake contains the new bytes, omits the old bytes, and excludes an
outside-workspace symbolic-link target.

## Verified — 2026-08-03

In a fresh isolated lab, a live Codex-backed session first reported
`CURRENT_CANDIDATE_MS=410`. The workspace Markdown then changed to `500 ms`; one documented
Heartbeat wake produced `CURRENT_CANDIDATE_MS=500` 19.594 seconds after the write. Session events
and recap independently agreed, session health returned to idle/healthy, and the event history
contained one user turn plus one synthetic reentry with no second operator prompt. The strict
release-grade auditor remains blocked by its intentionally broader actor/channel/surface minimums
and the deferred final gate; those do not change this scenario's observed behavior verdict.
