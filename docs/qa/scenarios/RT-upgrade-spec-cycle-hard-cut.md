---
id: RT-upgrade-spec-cycle-hard-cut
area: RT
title: Upgrade a v0.4.8 home through the spec-cycle hard cut
persona: Bruno
journey: J-deliver-through-agents
expected: A real v0.4.8 home with references to removed resources starts normally after upgrading, sessions open, affected automations fail in isolation with a clear error, .compozy/tasks files stay byte-identical, and nothing needs deleting.
entry_points: compozy daemon start; compozy doctor; compozy session list; compozy automation; compozy loop list; compozy extension list
qa_status: partial
bug_ids: BUG-20261102-migration-172-autoindex
fix_status: fixed
retest_status: pass
fix_commits: 2fa8edf3c
evidence: docs/qa/reports/2026-11-02-spec-cycle/QA.md
last_report: docs/qa/reports/2026-11-02-spec-cycle/QA.md
overlaps: RT-upgrade-memory-removal-home; ET-052; TA-web-automations-upgrade
---

Spec QA-004, covering US-006, US-007 and US-010.AC-2.

Prepare the home with v0.4.8 itself: a session on `review_fixer`, an automation job and a trigger aimed
at `implement-tasks`, a stored `loop_configure` override for that Loop, an always-on instance of a removed
agent, an active Loop run, an installed Batuta, a same-name workspace Loop, and `.compozy/tasks` content
with `task_*.md`, `reviews-NNN/` and `memory/`. Include saved scheduled and triggered custom Loops
whose retired tool node uses `review: {decisions: [respond]}` or
`result_contract: {failure_field: passed}`. Hash the task files first.

Upgrade without any manual step and confirm:

1. The daemon starts, and unrelated features work.
2. Removed Loops, agents, tools and skills are gone from listings.
3. `review_fixer` history stays readable, and resuming it names the unavailable agent. Prompting it returns HTTP 404, never HTTP 500.
4. The automation records stay unchanged, and each run fails alone with a clear error.
5. The override row stays and is never read. The active run fails at its first removed step with a clear error, its history stays, and other runs complete. Also rerun from a removed tool node: the run must settle as failed without reattempting or parking in quarantine; its failure names the missing ToolID.
6. The same-name workspace Loop keeps working.
7. The always-on instance pauses as `definition_missing`. The Agents page neither lists nor counts it, and its sidebar menu offers no agent page to open; Archive stays available.
8. Batuta still loads, and only `batuta-deliver` fails when run.
9. Task file hashes are unchanged.

In a separate copy, add a malformed contract with no `failure_field` to a saved custom Loop.
Validation must reject that independent error rather than treating it as a removed dependency.
