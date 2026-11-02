---
id: RT-upgrade-spec-cycle-hard-cut
area: RT
title: Upgrade a v0.4.8 home through the spec-cycle hard cut
persona: Bruno
journey: J-deliver-through-agents
expected: A real v0.4.8 home with references to removed resources starts normally after upgrading, sessions open, affected automations fail in isolation with a clear error, .compozy/tasks files stay byte-identical, and nothing needs deleting.
entry_points: compozy daemon start; compozy doctor; compozy session list; compozy automation; compozy loop list; compozy extension list
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-upgrade-memory-removal-home; ET-052; TA-web-automations-upgrade
---

Spec QA-004, covering US-006, US-007 and US-010.AC-2.

Prepare the home with v0.4.8 itself: a session on `review_fixer`, an automation job and a trigger aimed
at `implement-tasks`, a stored `loop_configure` override for that Loop, a stays-on instance of a removed
agent, an active Loop run, an installed Batuta, a same-name workspace Loop, and `.compozy/tasks` content
with `task_*.md`, `reviews-NNN/` and `memory/`. Hash the task files first.

Upgrade without any manual step and confirm:

1. The daemon starts, and unrelated features work.
2. Removed Loops, agents, tools and skills are gone from listings.
3. `review_fixer` history stays readable, and resuming it fails with a clear error.
4. The automation records stay unchanged, and each run fails alone with a clear error.
5. The override row stays and is never read. The active run continues from its persisted snapshot.
6. The same-name workspace Loop keeps working.
7. The stays-on instance pauses as `definition_missing`.
8. Batuta still loads, and only `batuta-deliver` fails when run.
9. Task file hashes are unchanged.
