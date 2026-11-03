---
id: TA-orchestrator-small-work
area: TA
title: Hand small work to the always-on orchestrator
persona: Bruno
journey: J-deliver-through-agents
expected: On a real provider, an always-on orchestrator records a work log entry before dispatch, runs the item in its own worktree, closes it with evidence, and a stale entry wakes heartbeat triage.
entry_points: compozy agent instance create orchestrator; compozy session prompt; .compozy/work-log.md; compozy__agent_heartbeat_status
qa_status: blocked-decision
bug_ids: BUG-20261102-instance-worktree-ceiling; BUG-20261102-triage-limits
fix_status: fixed
retest_status: pass
fix_commits: 5ed24fec1; d732bffc3
evidence: docs/qa/reports/2026-11-02-spec-cycle/QA.md
last_report: docs/qa/reports/2026-11-02-spec-cycle/QA.md
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

Run this scenario before `cy-orquestrate-issue` is removed, because always-on triage has not yet run on
real providers.

QA impact 2026-11-03 (#1199 vocabulary, v0.5.1): the bundled `orchestrator` declares `kind: always_on`; `compozy agent instance create orchestrator` reports it as always-on. Not walked; S4 walks it on the integration head.
