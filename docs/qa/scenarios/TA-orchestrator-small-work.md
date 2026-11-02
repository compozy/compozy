---
id: TA-orchestrator-small-work
area: TA
title: Hand small work to the stays-on orchestrator
persona: Bruno
journey: J-deliver-through-agents
expected: On a real provider, a stays-on orchestrator records a work log entry before dispatch, runs the item in its own worktree, closes it with evidence, and a stale entry wakes heartbeat triage.
entry_points: compozy agent instance create orchestrator; compozy session prompt; .compozy/work-log.md; compozy__agent_heartbeat_status
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: TA-spec-conductor-delivery; MS-web-stays-on-reports
---

Spec QA-002, covering US-002, US-003, US-005.AC-2 and US-008.

Send one bug report to the main session. Confirm the `[ ]` entry exists before the child session does,
the entry moves to `[~]` with session and worktree, and closes as `[x]` with evidence.

Then check the edges: a duplicate request reports the open entry, a spec-sized request gets a
recommendation instead of an executor, and learned preferences go to agent memory and not to the work
log. Repeat a dispatch after killing the orchestrator before the `[~]` update, and confirm no second
executor starts.

Add a `[ ]` entry older than 30 minutes. The next heartbeat digest carries `work:<id>` and triage
reports or proposes. With the digest unchanged, the following heartbeat makes no model call.

Run this scenario before `cy-orquestrate-issue` is removed, because stays-on triage has not yet run on
real providers.
