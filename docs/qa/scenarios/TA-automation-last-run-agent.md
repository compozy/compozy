---
id: TA-automation-last-run-agent
area: TA
title: Agents read each automation's last run from the list calls
persona: Ada
journey: J-24
expected: "Every job and trigger item from `compozy automation jobs|triggers -o json`, `GET /api/automation/{jobs,triggers}` over HTTP and UDS, `compozy__automation_{jobs,triggers}_{list,get}` and the single-item get routes carries `last_run` {id, status, started_at, ended_at when ended, skip_reason for a skipped fire stored as canceled}, equal to the first run of `…/{id}/runs?limit=1`; an automation that never ran omits the key. The CLI human table shows a Last Run column (failed 7h ago; — when it never ran; skipped for an overlap and missed for misfire_grace_exceeded, never canceled); `-o toon` adds last_run_status and last_run_started_at. `--target agent|loop|task` (and the `target` query/tool input) filters both lists server-side with honest totals; `task` matches jobs with a task target and no trigger. Every other verb, route, argument and output is unchanged, and the Web listing row shows the same truth."
entry_points: `compozy automation jobs|triggers`; `GET /api/automation/jobs`; `GET /api/automation/triggers`; `compozy__automation_jobs_list`; `compozy__automation_triggers_list`
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: TA-web-automations-listing; TA-schedule-catchup-overlap
---

New in the Automations spec (task 07; US-034, US-035; ADR-003). Produce one completed, one failed, one `self_overlap` skip and one never-run automation of each kind, then compare the four surfaces against `…/runs?limit=1` for the same ids and against the Web row text. Check a workspace-scoped read never returns another project's runs in `last_run`.
