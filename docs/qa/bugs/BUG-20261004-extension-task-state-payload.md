# BUG-20261004-extension-task-state-payload: Extension task responses lose persisted state

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Dora
- **Journey Step:** J-complete-task-tree, confirm the task produced by an extension command
- **Scenarios:** TA-001
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

Approve Studio Capture's editorial command under resume-editorial. Its real SDK subprocess calls
tasks/create with draft=true and leaves wake_creator at the documented default. The approval
completes and returns the Host API result. That result omits draft and reports wake_creator=false.
An independent task catalog and direct task detail report draft=true and wake_creator=true for the
same task. The refreshed Web detail also labels it Draft. The task is saved correctly; the response
can mislead an extension about whether it must publish the task or expect creator notification.

Evidence under docs/qa/evidence/2026-10-02-untested/: palette-decision-replay3-approved-status.json,
palette-decision-replay3-approved-drafts.json, palette-decision-replay3-task-direct-read.json,
palette-decision-replay3-task-refreshed.png and palette-decision-replay3-ended-summary.json.

## Diagnosis

The extension Host API task mapper omits Draft and WakeCreator. Its detail-summary mapper also
omits WakeCreator. The HTTP mapper already derives draft from the canonical status and copies the
persisted creator setting. Repair these fields at the Host API response boundary. A create return value can predate the append cursor; it must not be confused with a fresh read.
The first repair replay found that tasks/get also drops the persisted latest_event_seq (0 versus
28 over HTTP/CLI). The same private mapper omits current-run, direct/inherited pause, blocked
reasons and needs-attention metadata. Restore that existing public state at the response boundary.

Invariant: create and subsequent Host API detail responses retain the saved draft and creator
notification state, including the default and explicit false. Owning layer: extension Host API
task serialization. Canonical suite: TestHostAPIHandlerTasksCreateUsesTrustedExtensionIdentity in
internal/extension/host_api_test.go, using the existing real SQLite task service.

## Expanded repair evidence

The first focused repair replay preserves draft/wake_creator but fails on the fresh-read cursor.
See extension-task-state-replay-ended-summary.json. The existing real SQLite list/detail suite
then reproduces the missing inherited/direct pauses and block reasons. The existing direct-run
suite checks current_run_id after a real start; an initial assertion against a merely queued run
was an invalid fixture assumption and is retained in green.json, superseded by green2.json. Creation
returns are not compared with a later event cursor. Cursor checks belong to subsequent reads.
The existing task redaction suite owns safe rendering of the restored human-readable metadata.

## Fix and verification

The Host API mapper now carries persisted draft, creator notification, event cursor, current run,
direct/inherited pause, block reasons and attention metadata. Human-readable state is redacted
like the HTTP boundary. The existing handler/serialization regressions pass with the race detector.
A real SDK subprocess replay agrees with HTTP/UDS at cursors 30 (paused), 42 (attention) and 47
(canceled with cleared flags). Fresh Web/deep-link observations are recorded separately from those
response claims. Evidence: extension-task-full-state-replay-ended-summary.json and green2.json
under the report evidence prefix. The wider TA-001 creation charter remains Pending.
