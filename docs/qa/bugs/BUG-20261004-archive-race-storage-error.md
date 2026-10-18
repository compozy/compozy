# BUG-20261004-archive-race-storage-error: An archived automation owner produces an internal storage error

- **Status:** verified
- **Fix commit:** 7d30fa3e2
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Dora
- **Journey Step:** J-operate-profiles, archive concurrently with a manual automation trigger
- **Scenarios:** ET-profile-lifecycle-race-guards
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Archiving publishing-queue correctly prevents a concurrently triggered automation from creating its
session. The CLI and saved failed-run history report nested storage internals ending in
constraint failed: profile_archived (1811), without the ordinary structured profile guidance.
The safety boundary holds; the operator-facing diagnostic loses the domain reason and action.

## Reproduction

1. Create an enabled future job owned by an active, idle profile.
2. In concurrent terminals, run profile archive and automation jobs trigger for that profile.
3. Observe the ordering where archive commits after trigger admission but before session insertion.
4. Read the archived profile, disabled job, failed attempt and session catalog independently.

**Expected:** A typed archived-profile refusal with actionable lifecycle guidance.
**Actual:** CLI exit 69 and persisted history expose the raw SQLite constraint chain.

## Evidence

All receipts are under docs/qa/evidence/2026-10-02-untested/:
- profile-work-race-replay-trigger.json and trigger-archive.json
- profile-work-race-replay-trigger-owner.json, trigger-job.json and trigger-history.json
- profile-work-race-replay-trigger-session-read.json and final-history.json
- profile-work-race-replay-ended.json

## Fix

Session registration exposed two known SQLite trigger failures as unclassified storage errors.
The repository now recognizes their exact trigger code/message and returns a typed profile
admission refusal. Shared HTTP/UDS responses reuse the existing conflict payload; failed automation
history stores the refusal and recovery action. The database guards, transaction ordering and
dispatch policy are unchanged. Unknown SQLite failures keep their original diagnostics.

The existing global_db_session_test.go exercises archived/unavailable owners through ordinary and
identity-bound registration against real SQLite; profiles_errors_test.go owns HTTP classification;
dispatch_test.go owns durable history and no session/prompt on refused admission. All cases fail
before repair and pass with the race detector afterward. No new test file or migration is added.

## Verification

Dora's first simultaneous trigger loses before admission, leaving no run. A second overlap starts
archive 10ms later and reaches the insertion guard: CLI returns profile_archived with recovery
guidance, and independent UDS/CLI history shows one failed attempt without raw SQLite/wrapper text
or a created session. Unarchive retains the disabled job. A new active-owner session succeeds,
blocks archive, and stops cleanly. Evidence: profile-owner-admission-replay-ended.json,
profile-admission-red.json and profile-owner-admission-green.json. The build identity pins the
repair diff; its commit is recorded in the report after the delivery gate.
