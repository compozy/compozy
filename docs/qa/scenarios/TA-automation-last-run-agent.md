---
id: TA-automation-last-run-agent
area: TA
title: Agents read each automation's last run from the list calls
persona: Ada
journey: J-24
expected: "Every job and trigger item from `compozy automation jobs|triggers -o json`, `GET /api/automation/{jobs,triggers}` over HTTP and UDS, `compozy__automation_{jobs,triggers}_{list,get}` and the single-item get routes carries `last_run` {id, status, started_at, ended_at when ended, skip_reason for a skipped fire stored as canceled}, equal to the first run of `…/{id}/runs?limit=1`; an automation that never ran omits the key. The CLI human table shows a Last Run column (failed 7h ago; — when it never ran; skipped for an overlap and missed for misfire_grace_exceeded, never canceled); `-o toon` adds last_run_status and last_run_started_at. `--target agent|loop|task` (and the `target` query/tool input) filters both lists server-side with honest totals; `task` matches jobs with a task target and no trigger. Every other verb, route, argument and output is unchanged, and the Web listing row shows the same truth."
entry_points: `compozy automation jobs|triggers`; `GET /api/automation/jobs`; `GET /api/automation/triggers`; `compozy__automation_jobs_list`; `compozy__automation_triggers_list`
qa_status: partial
bug_ids: BUG-20261103-target-instance-id-accepted
fix_status: open
retest_status: pending
fix_commits:
evidence: .compozy/tasks/automations/screens/; .compozy/tasks/automations/reports/q1-qa-walk.md (Round 2); docs/qa/reports/artifacts/2026-11-03-stays-on-hardening-qa/ (2026-11-03 stays-on QA walk)
last_report: docs/qa/reports/2026-11-03-stays-on-hardening-qa.md
overlaps: TA-web-automations-listing; TA-schedule-catchup-overlap
---

New in the Automations spec (task 07; US-034, US-035; ADR-003). Produce one completed, one failed, one `self_overlap` skip and one never-run automation of each kind, then compare the four surfaces against `…/runs?limit=1` for the same ids and against the Web row text. Check a workspace-scoped read never returns another project's runs in `last_run`.

## Stays-on event delivery

Create a workspace webhook trigger with `--target-instance`, sign one delivery and inspect its
completed run: `session_id` is empty and metadata identifies the stored instance event and check
status. Run the check with a provider advertising read-only mode, including an attempted edit.
The edit is refused, a proposal retains trigger/run/event source IDs, and the main has no new user
message. Repeat with `ext.acme.ticket`; it uses the same dispatcher. Disable checks and verify the
run explains why no check reads the event. A missing target fails the run while keeping the trigger
enabled. Verify pause retains events, retention counts omissions, and a successful check consumes
only its bounded batch.

Backend evidence: `daemon.TestInstanceWebhookCheckIntegration` (signed HTTP ingress, real ACP,
SQLite and public proposal read), `automation.TestDispatchInstanceEvents`, and the canonical
`globaldb.TestInstanceEventDelivery` / `TestInstanceWakeTransactions` suites. Results are recorded
in `docs/qa/reports/2026-11-03-stays-on-hardening-backend.md`; Web presentation remains separate.

QA verification 2026-11-03 (stays-on event delivery, soh/qa 583415ef5, isolated acpmock lab): Partial. A workspace webhook trigger with `--target-instance inbox` accepted one signed delivery (HTTP 200). The completed run had `session_id: null` and metadata naming `instance_event_id`, instance, trigger and `instance_event_check_scheduled: true`. The read-only check produced proposal `arpt-…` whose sources carry `event_id`, `trigger_id` and `run_id`, and the main gained no user message (`webhook-run.json`, `inbox-reports.json`). With checks disabled, the run reported `instance_event_reason: checks_disabled`. Finding: `--target-instance <instance-id>` first demands `--prompt` with an error naming `--agent`; once given a prompt it stores the ID as `agent_name`, and every delivery returns 404 `trigger_target_instance_not_found` (BUG-20261103-target-instance-id-accepted). Not walked: an `ext.*` event, a failed run for a missing target, retention counts, and evidence of the refused edit.
