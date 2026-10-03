# BUG-20261003-profile-archive-event-rejected: Profile lifecycle events disappear when their subject is unavailable

- **Status:** verified
- **Fix commit:** 4760da89f; fe8a644b1
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-operate-profiles, retire an empty draft profile while its browser stays open
- **Scenarios:** ET-profile-web-settings-lifecycle-dialogs; ET-profile-switcher-restore; ET-profile-operations-recovery
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

## Regressed — 2026-10-03, unavailable operation owner

Ada's CH-profile-lifecycle-plan-recovery Interrupt Tour on b915570a8 creates recovery-notes
beside a preserved, conflicting recovery-guides directory. Rename commits its catalog change
and records a failed rename_profile step. CLI, HTTP and UDS agree on operation
op_01M411M8MPXFFNCCF3V73BZXCZ, but the public all-profile log has no
profile.lifecycle_op_failed event, including after restart. Failed operations correctly remain
failed until explicit retry; moving the preserved import aside and retrying completes without
losing content. The session ends before source inspection.

The daemon log confirms that the event insert is rejected by profile_unavailable. The original
archive/delete special case does not include failure or recovery events, whose subject can still
be reserved, archived, or deleted. This is the same audit-owner defect under another lifecycle
transition; the already-verified archive behavior remains intact.

Evidence: profile-recovery-ada-{conflict,failed-ops-cli,failed-ops-http,failed-ops-uds,
events-after-restart,final-events,ended}.json in this cycle's evidence directory.

Invariant, owner, canonical suite: lifecycle failure/recovery audits remain durable with their
original subject and operation even when that subject cannot accept ordinary writes. The daemon
event adapter owns this invariant in TestDaemonProfileEventRecorder,
internal/daemon/boot_profiles_test.go. Extend its real SQLite failure case and existing terminal
event-owner coverage; retain the ordinary unavailable-owner write guard. No new test file,
schema, wire shape, polling, or validation bypass is needed.

## Re-found — 2026-10-03, archived identity audit

The first repair replay verifies durable failure audit and pending-owner identity refusal.
An adjacent compatibility check edits recovery-guides after archive: the color persists, but
profile.identity_updated is absent, and the daemon records profile_archived on its audit insert.
Archived metadata edits are intentionally retained. Carry the subject's known state internally
to the existing audit-owner decision, without exposing a new wire field or changing active-owner
event ownership. Extend the same real SQLite archive case in TestDaemonProfileEventRecorder to
cover this observable; no second suite or new file owns it.

Evidence: profile-recovery-fixed-ada-{archived-identity,archived-identity-read,
archive-canary-events,ended}.json. The five-frame browser recording is stopped before diagnosis.

## Recovery extension verified

The final Ada replay preserves the failed operation's subject, ID and error in the public audit.
Archived identity edits also retain a durable operator-owned event, while active identity events
keep their own owner. The internal subject state is absent from the JSON payload. Ordinary writes
under unavailable or archived owners remain refused.

Real SIGKILL interruptions after rename and during deletion recover on boot. The rename preserves
all 2,048 authored configuration files byte-for-byte; deletion finishes removing its remaining tree.
A further clean restart retains exactly one recovery event per operation, with stable event IDs
and original subjects, including the deleted subject. CLI, HTTP and UDS readbacks agree.

Evidence: profile-recovery-final-ada-*.json, profile-recovery-{rename,delete}-crash-proof.json,
and the inspected profile-recovery-final-settings.png. The six-frame recording is stopped before
source review. The existing recorder/availability/recovery/lifecycle race cohort passes, as do
both test-shape checks and every affected make gate lane (zero Go lint issues). Gate receipts:
profile-recovery-{gate,gate-status}.log. Archive interruption and the separate blank-desktop bug
remain outstanding; they do not reopen this verified audit-owner repair.
