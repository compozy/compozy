# BUG-20261003-profile-unavailable-identity-write: A reserved profile still accepts identity changes

- **Status:** verified
- **Fix commit:** pending
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-operate-profiles, inspect and recover an unavailable lifecycle owner
- **Scenarios:** ET-profile-operations-recovery
- **Found:** 2026-10-03 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

In CH-profile-lifecycle-plan-recovery, Interrupt Tour, isolated profile-recovery lab on b915570a8:

1. Create recovery-notes and retain a conflicting imported recovery-guides directory.
2. Rename recovery-notes to recovery-guides. The catalog changes, while its filesystem step fails.
3. Select recovery-guides; the CLI refuses profile_unavailable and names the reserving operation.
4. Update recovery-guides to color #527b67. The CLI succeeds and the catalog retains the change.

Expected: a pending or failed lifecycle operation reserves the profile until recovery completes.
Identity mutation must observe that same reservation. Actual: identity updates bypass it, and
their audit is then rejected by the existing unavailable-owner event write guard.

## Evidence

docs/qa/evidence/2026-10-02-untested/profile-recovery-ada-{unavailable,selection-refusal,
identity-refusal,failed-ops-cli,final-events,ended}.json. The identity-refusal receipt's name
describes the attempted branch; its exit 0 and changed color are the failing observation.
The walk ended before source inspection. Restart and explicit retry otherwise preserve both
the original notes and the conflicting import.

## Diagnosis and intended repair

UpdateIdentity reads and updates the profile inside the write transaction without invoking
the existing lifecycle availability guard. Check that guard in the same transaction before
changing identity. Retain the existing ability to edit an archived profile when it has no
pending operation; this repair concerns operation reservations.

Invariant, owner, canonical suite: identity changes cannot commit for an owner reserved by an
unfinished lifecycle operation; after completion, they can. The profile manager owns this
invariant in TestManagerSelectionResolutionAndAvailability in internal/profile/manager_test.go.
Extend its existing unavailable-owner case and verify unchanged identity on refusal plus success
after recovery. No duplicate HTTP/CLI assertion suite or standalone test file is needed.

## Repair and verification

UpdateIdentity now checks the existing availability guard inside its write transaction, before
changing identity. It intentionally permits archived metadata edits when no operation reserves
the profile. The canonical regression fails before the repair and passes with unchanged color
on refusal plus a successful edit after completion.

Fresh Ada replay proves CLI and HTTP refuse the failed owner with profile_unavailable, including
HTTP 409; independent reads retain its original identity. Correcting the actual directory conflict
and explicitly retrying permits the edit. Archived and active canaries retain their intended audit
ownership. The final source passes the focused race cohort, test-shape checks, production build
and all affected make gate lanes. Receipts: profile-recovery-{red,final-green,final-build,gate}.log
and profile-recovery-final-ada-*.json. The walk and recording are ended.
