# BUG-20261004-task-owner-response-omissions: Scheduler backlog loses task ownership labels

- **Status:** verified
- **Fix commit:** 7d30fa3e2
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Dora
- **Journey Step:** J-operate-profiles, identify the queued work frozen with an archived profile
- **Scenarios:** ET-profile-lifecycle-race-guards; TA-010
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
