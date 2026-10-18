# BUG-20261003-profile-archive-resource-automations: Archive omits enabled resource automations

- **Status:** verified
- **Fix commit:** b4ed8ca18
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Sol; Ada
- **Journey Step:** J-operate-profiles, archive and restore a profile with enabled automation
- **Scenarios:** ET-profile-web-settings-lifecycle-dialogs; ET-profile-cli-lifecycle
- **Found:** 2026-10-03 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Archiving publication-drafts promises to pause its automations, but the enabled Monthly release
digest job is omitted from the plan and remains enabled and registered in the scheduler. Restoring
the profile reports that it had no paused automations. No job execution was attempted or observed.

## Reproduction

- **Charter:** CH-profile-settings-dialog-plans · **Tour:** Back-Button Tour
- **Environment:** desktop 1512×862, wifi-fast, en-US, isolated daemon 53876, build a15b2ea62

1. Create an enabled workspace job under publication-drafts using the public automation CLI.
2. Open Settings → Profiles with the keyboard and preview Archive for that profile.
3. Compare the dialog with HTTP and UDS archive-plan reads, then confirm Archive.
4. Read the profile over UDS and list jobs through `automation jobs --all-profiles --json`.
5. Restore the profile, read the job again, and reload Settings.

**Expected:** The plan and result enumerate the enabled job. Archive durably disables it and
removes its scheduler registration; restoring the profile leaves it paused for explicit reactivation.
**Actual:** The plan and result return null automation lists. The profile is archived, but job
job-71be9464ae04c54a remains enabled and scheduler-registered with next_run 2026-11-01T09:00:00Z.
Unarchive returns an empty paused list and the job remains enabled without operator reactivation.

## Evidence

All paths below are relative to docs/qa/evidence/2026-10-02-untested/.

- profile-archive-sol-release-plan-independent.json; HTTP and UDS agree on the omission.
- profile-archive-sol-job-before.json; independent enabled job and scheduler state.
- profile-archive-sol-confirmed.json; the Web request quotes its plan revision and returns HTTP200.
- profile-archive-sol-profile-after.json; the profile is archived.
- profile-archive-sol-all-jobs-after.json; the aggregate CLI still reports enabled and registered.
- profile-archive-sol-unarchive-confirmed.json and profile-archive-sol-job-unarchived.json.
- profile-archive-enabled-job-missing.png and profile-unarchive-missing-paused-job.png, inspected.
- profile-archive-sol-ended.json; recording profile-archive-sol stopped with 114 frames.

The explicit archived-owner job read is correctly refused with profile_archived. The aggregate
read supplies the independent state evidence. No source or database reads occurred during the
persona walk. The separate book-notes running-session blocker correctly names Source index and
disables Archive. An incomplete AX selector timed out before complete accessibility inspection;
that driver mistake is not a product failure.

## Investigation

The profile manager's plan and pause transaction read and update legacy automation_jobs and
automation_triggers. The daemon's current automation definitions use the resource kernel instead.
The repair must include durable pause state, live scheduling/trigger reconciliation, exact plan
validation, owner isolation, and preservation of pauses across unarchive and restart.

## Fix

Archive reads both canonical resource definitions and legacy rows, with source-appropriate enabled
overrides. The profile's immediate SQLite transaction pauses dynamic definitions, updates their
resource versions, and records operational overrides for managed definitions. It retains unrelated
owners and previously disabled automations. The existing lifecycle journal synchronizes the live
automation owner after commit and before reporting completion; boot reads the durable paused state.

An injected runtime synchronization failure exposed an idempotent-archive shortcut that returned
success while finalization was incomplete. Availability is now checked before that shortcut;
`profile ops retry` completes the journaled synchronization without replaying the mutation.

Owning invariant and suite: internal/automation/resource_test.go, TestProfileArchiveResourceAutomations.
Real resource stores, SQLite and schedulers prove plan membership, stale-plan refusal, owner isolation,
managed-definition preservation, paused state after restore/restart, and explicit reactivation. An
I/O-boundary failure proves error propagation, durable recovery reporting and explicit retry.

## Verification

The initial regression fails for dynamic, config and package sources because the plan is empty;
profile-archive-automations-red.log retains those failures. The failure-path extension also fails
before its guard repair; profile-archive-automations-failure-red.log records the false success.
Focused race-enabled profile, automation-resource and daemon suites pass in
profile-archive-automations-race.log. The convention checker reports the same eight pre-existing
inline-case findings as HEAD; the added cases follow the canonical shape. Both receipts are retained.
An initial test constant-name compilation mistake is preserved separately and is not a product bug.

Fresh Sol replay proves the exact two-item preview, quoted HTTP mutation, disabled public job and
trigger catalogs, and paused restore controls. Escape and reload before confirmation preserve state.
Explicit keyboard reactivation targets publication-drafts despite book-notes being selected; toggling
the trigger back off leaves the job active independently. Recording profile-archive-fixed-sol stops
with 165 frames. The inspected before/after screenshots and profile-archive-fixed-sol-* receipts
retain the visual, transport and independent-read evidence.

Ada then archives the reactivated job through CLI: the plan excludes the already paused trigger,
repeat archive is idempotent, and unarchive retains the exact audit list. A real daemon restart leaves
both definitions disabled with no next run. Both Source index sessions are stopped. See
profile-archive-fixed-cli-*.json. No scheduled or triggered model generation is claimed.

Two reactivation-driver mistakes are separated from product behavior: the harness string Space did
not send a native space character, and successful reactivation changes the label from Reactivate to
Pause. Native key events, the current AX label, HTTP200 and independent CLI reads prove success.

Delivery receipts are profile-archive-automations-gate-ready.log and the final delivery-proof JSON.
The initial gates retain formatter drift and two lint findings: a long SQL-table assignment and
one new branch exceeding the existing finalizer's complexity limit. Formatting, splitting that
assignment, and extracting the unchanged directory-creation step resolve them without suppression.
Vet also catches a missing error-format argument during that extraction; the original path argument
is restored before the focused profile rerun. All intermediate failed receipts are retained. The
strict lab audit initially reports missing provider-surface and final-report/gate records; a fresh
native executable probe supplies bounded provider evidence, without claiming generation. The full
Settings and CLI lifecycle charters remain pending beyond these verified archive/restore legs.

Commit b4ed8ca18 retains current-pass records for every affected gate lane. The final source
build and owned daemon restart keep both automations disabled; the strict targeted evidence audit
passes without blockers or warnings. See profile-archive-automations-delivery-proof.json.
