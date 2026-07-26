---
id: RT-pressure-context-compaction
area: RT
title: Compact completed context without losing session evidence
persona: Théo
journey: J-11
expected: At configured context pressure, AGH summarizes only complete prior turns into the workspace checkpoint before archiving their event rows from degraded replay. History retains the archived rows, repeated coverage is idempotent, failed summary or archive work preserves replayable events, and a successful provider load remains unchanged.
entry_points: daemon session prompts; agh session events; agh session history; degraded session reactivation; config CLI/native tools
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-session-context-rebuild; MS-workspace-checkpoint-continuity
---

Drive a session above the configured context-pressure threshold after at least one complete turn.
Confirm the checkpoint covers that exact prior-turn sequence range before the rows become archived,
history still returns the rows, and degraded replay omits their raw fact while preserving it through
the checkpoint. Interrupt once after coverage but before archive, then retry and confirm provider
summary work is not duplicated. Repeat with a successful ACP session load and confirm AGH does not
inject replay context.

QA impact 2026-07-15: new session compaction, archive projection, lifecycle event, and config
behavior. Planning flag only; no QA session ran in this implementation slice.

Phase C planning 2026-07-19: persona normalized to Théo; settles US-004 (D3, ADR-003, B-301)
together with MS-workspace-checkpoint-continuity, MS-atomic-memory-batch, and
RT-session-lifecycle-affordances.

Forensic evidence contract (SD-006) — each item cites timestamp, exact command, observed output:

- Usage-pressure trigger log line and the archived-row query proving rows stay queryable.
- The kill-before-clean-session-end + resume run reconstructing archived facts from the injected
  checkpoint summary (no silent loss, no re-inflation).
- The idempotent-retry run after an interrupt between coverage and archive (no duplicated summary
  work), and a `pressure_threshold = 0` run with zero hook dispatch.

src: .compozy/tasks/hermes-comparison/_user_stories.md#us-004-compaction-under-pressure-crash-safe
