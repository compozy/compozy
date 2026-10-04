# BUG-20261004-archive-race-storage-error: An archived automation owner produces an internal storage error

- **Status:** open
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

Initial tracing follows the rejected session registration through accepted-start persistence to the
automation run error. The existing database ownership guard must remain authoritative; classify its
error at the owning boundary instead of retrying or bypassing it. No production repair yet.
