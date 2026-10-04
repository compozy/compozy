# BUG-20261004-task-owner-response-omissions: Scheduler backlog loses task ownership labels

- **Status:** verified
- **Fix commit:** 7d30fa3e2; 8d630a7f0
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Dora
- **Journey Step:** J-operate-profiles, identify the queued work frozen with an archived profile
- **Scenarios:** ET-profile-lifecycle-race-guards; TA-010; TA-048
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

The global scheduler backlog returns empty task profile_id/profile_name and omits the run's profile,
although fresh task and run detail reads identify publishing-queue. Dora cannot identify the owner
from that queue response. A related task update response preserves profile_id but leaves
profile_name empty; a subsequent dependency/detail response correctly decorates the same task.

## Reproduction

1. Create a task in publishing-queue and enqueue a run while the scheduler is paused.
2. Archive the profile; inspect scheduler backlog --include-paused -o json.
3. Compare the same task through the aggregate UDS detail read and its run through CLI list.
4. Unarchive and repeat backlog: the same run remains, with missing owner labels.
5. Update another task in that profile; compare its returned profile_name with a fresh detail.

**Expected:** Existing task/run owner fields retain the real profile identity across these responses.
**Actual:** The queue has empty owner fields; the task update has an empty owner name.

## Evidence

All receipts are under docs/qa/evidence/2026-10-02-untested/:
- profile-work-race-replay-frozen-backlog.json and frozen-task.json
- profile-work-race-replay-unfrozen-backlog.json and run-history.json
- profile-work-race-replay-update.json and dependency-add.json
- profile-work-race-replay-ended.json

## Fix

The scheduler service omitted the run's inherited profile ID, its compact task mapper omitted
the task profile ID, and backlog/update handlers skipped the existing owner decoration. The repair
uses the same service inheritance and handler decorators already used by detail/run responses.
No storage or contract shape changes are needed.

Existing scheduler_controls_test.go owns run identity; tasks_test.go owns response owner fields.
The new cases reproduce the omissions before repair. The update case uses a non-default profile
and verifies the title and owner together. Race-enabled affected checks pass.

## Verification

Dora creates a fresh task, updates its title and independently reads it over HTTP. Archive freezes
one queued run whose CLI backlog carries matching task/run owner fields. Unarchive exposes the same
run exactly once through the HTTP backlog with the same identity. Cancellation persists through UDS.
Evidence: profile-owner-admission-replay-ended.json, task-owner-projection-red.json and
profile-owner-admission-green.json. The build identity records the current repair diff; its commit
is recorded in the enclosing report after the delivery gate.

## Re-found in start responses — 2026-10-04

During CH-scheduler-drain-recovery, task create correctly returns resume-editorial's full identity.
The subsequent task start response leaves task.profile_name empty and omits the run's display
identity. The task run start response omits even profile_id. A fresh run detail identifies the
same owning profile correctly. Existing backlog and update repairs remain verified; these are
additional response paths with the same missing-owner symptom.

Evidence: scheduler-drain-dora-work-create.json, work-start-task.json, work-start.json and
run-current.json under the cycle evidence directory. The session-profile mismatch is a separate
ownership defect in BUG-20261004-task-session-wrong-profile. Response decoration and persisted
session ownership must be verified independently.

The shared nominal settlement reconstructs a run without reattaching its task's profile ID.
Execution-boundary retries have the same omission. Those two service boundaries now retain the
already-authorized task identity before publishing or returning; start/publish/approve responses
use the existing profile owner decorators. The corresponding daemon/service/handler red checks
and task-start-owner-final-focused.json prove the repair with the race detector. The first handler
fixture omitted the required run status and was corrected before recording the causal red result.
Fresh public replay and the delivery gate remain outstanding.

## Re-found in inspection — 2026-10-04

The fresh Dora replay confirms correct owner identity in direct task start, its same-key retry and
run start. Task inspect still leaves task.profile_name empty while retaining the correct ID.
Evidence: scheduler-drain-dora-replay-worker-inspect.json. Keep this additional read projection
on the same bug; the previously verified backlog/update behavior has not regressed.


Inspection uses the existing task-summary owner decorator before serialization. The handlers move
to a cohesive inspection file to keep the production CRUD file below 500 lines. The existing
tasks_test.go handler suite reproduces both task and run inspection omissions and then passes
with the race detector. No new DTO, persisted field or owner lookup fallback is introduced.

The scheduler-projection-fixed Dora replay confirms the full resume-editorial identity in both
task inspection through CLI and run inspection through UDS. The Web Inspect dialog also opens
for that run. Receipts: scheduler-projection-fixed-dora-{task-inspect,run-inspect,run-inspect-web}.json.
The original natural-completion replay already verified the repaired start responses and session
owners. Delivery gate and commit remain pending for this response batch.

## Delivery closure — 2026-10-04

Commit 8d630a7f0 records the repaired behavior. The original-persona replays above and all selected
local gate lanes pass. scheduler-profile-final-delivery-gate.json, final-delivery-gate-status.json
and commit-proof.json retain current-input verification; the committed tree exactly matches
the checked tree 93e4b5183de281b741b373fc93ab802bc89c27f5. Broader QA and PR/CI remain separate.
