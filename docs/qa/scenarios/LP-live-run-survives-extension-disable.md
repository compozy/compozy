---
id: LP-live-run-survives-extension-disable
area: LP
title: Resume a pinned Loop wait after its extension is disabled
persona: Bruno
journey: J-04
expected: Disabling an extension after a Loop run starts resolves the run's pinned profile and does not make an unrelated wait impossible to resume. The resume reads the run's pinned executed definition and accepts the wait payload while leaving other profiles unchanged; any later unavailable extension action fails through the node lifecycle instead of turning the resume request into an internal server error.
entry_points: compozy --profile <name> extension disable; POST /api/workspaces/:workspace_id/loop-runs/:run_id/nodes/:node_id/resume; compozy loop status
qa_status: untested
bug_ids: BUG-20260803-disabled-extension-blocks-wait-resume; BUG-20261005-loop-wait-example-unknown-event
fix_status: fixed
retest_status: pass
fix_commits: Task 08 checkpoint
evidence: docs/qa/evidence/2026-10-02-untested/loops-pinned-wait-final-http.json; docs/qa/evidence/2026-10-02-untested/loops-pinned-wait-final-uds.json; docs/qa/evidence/2026-10-02-untested/loops-pinned-wait-final-web.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: LP-operator-lifecycle-ui; ET-021
---

story: As an operator, I can finish an already-open wait even if the extension used by a later node is temporarily unavailable.

Task 08 reproduced the original HTTP 500 with `spec-cycle` disabled, repaired the service to load the run's immutable definition snapshot, rebuilt the daemon, and re-walked the same public sequence. Resume returned HTTP 200 while the extension remained disabled; the later extension action entered the ordinary node failure lifecycle.

src: .compozy/tasks/loop-node-lifecycle/task_08.md

QA impact 2026-08-22: reset because extension disable now targets the Loop run's profile instead of a
global extension state.

Walk: start the run under one profile, disable the extension through that profile, and verify resume
uses the run's pinned profile while a second profile's extension state and work remain unchanged.

2026-10-05: CH-pinned-wait-extension-lifecycle completed on the real daemon. The existing
wait resumed with HTTP 200 while spec-cycle was disabled only in resume-editorial. Fresh
CLI/UDS reads retained the successful wait; the unavailable importer entered quarantine.
The studio Profile retained enabled state and a working importer. After restoring the
extension and requeuing only read_tasks, the original Run completed in generation 3 with
the same wait output reference, zero pending tasks and no open waits. Public events show
one wait start/resume. Web and reload preserve the result; both extension inventories
match their pre-walk name/enabled/version baselines. The stale DSL event example found
during setup is repaired and publicly validated; commit/delivery closure is pending.
