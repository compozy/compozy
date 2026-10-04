# BUG-20261004-attention-deleted-workspace-stale: Notifications retain a deleted project until reload

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-administer-runtime-settings, retire a muted project
- **Scenarios:** MS-attention-settings-roundtrip
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Dora removes a muted project through the CLI while Notifications remains open. HTTP and UDS
confirm its mute was removed from both profiles, but the open page retains a row displaying the
deleted workspace ID. A fresh reproduction then changes Sound from that stale page: the write
returns 500 and the page shows Internal Server Error. The previous channel value survives.

## Reproduction

- **Charter:** CH-herdr-attention-settings · **Tour:** Multi-Tab Tour
- **Environment:** isolated real daemon, desktop Chrome 1512 × 862, en-US

1. Mute Archive desk in default and resume-editorial; also mute Review desk in resume-editorial.
2. Keep Notifications open under resume-editorial and remove Archive desk through the CLI.
3. Fresh-read both profiles through HTTP/UDS: default is empty, resume-editorial retains only Review desk.
4. Inspect the still-open page: it retains ws_6c24a6f1c0f171bc alongside Review desk.
5. Reload: only Review desk remains.

**Expected:** The active policy view reconciles the removed workspace without keeping its obsolete mute.
**Actual:** Workspace labels refresh, exposing the removed ID, while the mute list remains stale.

## Evidence

Receipts under docs/qa/evidence/2026-10-02-untested/:
- attention-mutes-dora-remove-archive.json
- attention-mutes-dora-default-pruned.json and attention-mutes-dora-editorial-pruned.json
- attention-mutes-dora-pruned-and-restored-web.json: MUTE_BUTTONS_BEFORE_RELOAD includes the deleted ID
- attention-mutes-dora-pruned.png: refreshed policy retains only Review desk
- attention-mutes-dora-ended.json: restoration and 19-frame recording closure

Both temporary registrations are removed after the first walk. Original global values and empty
mute sets are restored. That walk did not attempt a save from the stale view; the separate fresh
reproduction below establishes the save failure.

## Fix

- **Root cause:** Channel controls resubmit the cached mute list even though the typed API can
  preserve it when omitted. Workspace catalog removal does not invalidate the separate attention
  read until its normal refresh. The mute repository also returns a raw foreign-key failure for
  an unknown workspace instead of the existing typed missing-workspace error.
- **Fix commit:** 84f02d6b2.
- **Regression test:** The existing attention page suite now uses real query/mutation/profile
  hooks with adapter and browser-permission I/O mocks. Its original four cases still pass; three
  channel cases fail against a stale deleted mute, and a catalog-removal case fails to reconcile.
  All eight pass after repair. The existing SQLite mute-replacement suite strengthens its
  missing-workspace error assertion while retaining the complete rollback assertion; it fails
  before and passes after repair with the race detector. No new test file is added.

Channel writes now omit workspace mutes, while explicit mute edits retain their existing complete
replacement contract. The canonical attention hook rereads the server when the complete workspace
catalog no longer contains one of its mutes; concurrent observers share an in-flight reread.
The repository checks workspace existence inside its existing write transaction and preserves
the existing typed 404 mapping and full rollback. No migration or API shape change is needed.

## Verification

The original-persona repair replay passes on the rebuilt daemon and Web bundle recorded in
attention-deletion-build-identity.json. The delivery gate passes codegen-check, Go lint, affected
Go suites with the race detector, and all 693 Web files / 6,942 tests. The refreshed gate and hooks
pass for commit 84f02d6b2 and tree c60157930737484aa2f8a0c0be7ae74fa2e3b86c. Evidence:
attention-deletion-final-delivery-gate.json, attention-deletion-staged-gate-status.json, and
attention-deletion-commit-proof.json. The complete parent charter retains separate unverified legs.

The open page removes Archive desk in 3.817 seconds after CLI deletion, retaining Review desk.
A second newly registered Archive desk is muted through Web and removed through CLI. While it
is still visibly present, Sound writes only the three channels and receives 200 in 0.205 seconds;
fresh UDS reads sound=false and only Review desk. Both HTTP/default and UDS/resume-editorial
explicit replacements containing the deleted ID return 404 and preserve the complete prior
channels and each profile's distinct mute list. Refresh preserves the saved policy.

All original values and empty lists are restored through Web and independently confirmed;
all three temporary registrations are removed. attention-repair-dora-ended.json closes the
14-frame recording, and all six screenshots are inspected. One restoration snapshot parser
raised KeyError after Sound was already saved; a fresh UDS read confirmed it before continuing
without repeating the toggle. No source changes occurred during the session.

## Reproduced save failure — 2026-10-04

The first stale-ID snapshot was 55.7 seconds after removal, so it does not establish indefinite
staleness beyond the normal section cadence. The fresh session proves the functional consequence
within that window: Sound sends one PATCH with the removed ID and receives 500; subsequent UDS
still reads sound=true and empty mutes. The visible error is captured in
attention-delete-dora-final-failure.png. The settlement reread removes the stale row.

attention-delete-dora-ended.json closes the eight-frame recording; all four screenshots are
inspected. The initial Sound click only dismissed the native picker and emitted no PATCH; the
recorded actual write occurred after explicit picker dismissal. Original settings and the removed
temporary registration are already restored. No source edit occurred during the session.
