# BUG-20261003-profile-archive-event-rejected: Archiving a profile drops the event that sweeps open views

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-operate-profiles, retire an empty draft profile while its browser stays open
- **Scenarios:** ET-profile-web-settings-lifecycle-dialogs; ET-profile-switcher-restore
- **Found:** 2026-10-03 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

The isolated browser displays release-drafts while the CLI archives it. Archive succeeds, but the
visible profile trigger and menu retain the profile. Reload restores the effective default.
A fresh delivery-review profile reproduces the archive symptom after a successful daemon restart
and WebSocket 101 reconnect. The same connection receives a profile.identity_updated control event
for another profile, excluding connection loss as the cause. No source or database is read during
the persona walks.

## Diagnosis and repair

After the walk, the daemon log reports profile lifecycle event recording failed, with the SQLite
profile_archived constraint. The manager commits archive and then the recorder tries to insert its
audit under the now-unavailable subject. The existing delete audit already uses the permanent
operator owner. Apply that same boundary rule to archive and retain the affected profile and
operation in the payload. Do not relax the archived-owner write guard or add browser polling.

Invariant, owner, canonical suite: TestDaemonProfileEventRecorder in internal/daemon/boot_profiles_test.go
uses real SQLite to prove the archive event remains readable with its subject and operation, while
ordinary archived-owner event writes still fail profile_archived. Before repair, it reports zero
archive events rather than one (profile-archive-event-red.log).

## Evidence and verification

Cycle receipts: approval-owner-visibility-recheck.json, approval-owner-stale-switcher.json,
approval-owner-postload-readback.json, profile-stream-replay-*.json, profile-archive-event-diagnosis.json.
Both persona walks are ended and all their recordings stopped. The fresh public replay passes: UDS reads the durable archive event with its original subject and operation; the named WebSocket frame reaches the browser, which sweeps to default without reload.
The separate delete replay still fails despite a durable event. It is tracked as
BUG-20261003-profile-delete-live-stream-owner; this repair covers the archive recording defect.

The recorder suite passes under race detection (2.940s); the convention checker and build pass.
The adjacent profile package selector matched no cases and supplies no additional test claim.
Fresh receipts: profile-archive-fixed-*.json. Both temporary profiles were removed, browser and CLI
are restored to studio, and recording profile-archive-fixed is stopped.

Archive repair delivery gate: every affected make gate lane passed; go-lint reported zero issues.
Receipt: docs/qa/evidence/2026-10-02-untested/profile-archive-event-gate.log.
