# BUG-20261004-extension-profile-creator-attribution: Bound profiles are attributed to the extension as newly created

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona:** Dora · **Journey:** J-operate-profiles
- **Scenarios:** ET-profile-lifecycle-race-guards
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

During an overlapping local extension install and operator profile create, both commands succeed.
The operator creates shared-editions as 01M43BB1HTQN8ZMJGZQYA3038K with color #246b73 and
pen-tool. Independent CLI, HTTP and UDS reads retain that identity and one profile. The extension
publishes its placed skill to the same profile, but install, status and HTTP detail report
created_by_extension=true. The extension declaration instead requested a different color and icon.

The false attribution survives a separate detail read. No provider prompt is sent. The persona
walk ends and restores editorial selection before source inspection. The profile and extension
remain available for the repair replay. Earlier install failure under a mismatched build identity
is retained separately; it is not this defect's reproduction.

Root cause: enrichExtensionProfilePayload maps HasDeclaredMarker directly to CreatedByExtension.
The marker intentionally records both creation and binding to make declaration processing
idempotent. Its current stored shape does not distinguish those outcomes. Marker existence is
therefore insufficient creation provenance. A repair must preserve create-once behavior and
historical markers, distinguish actual creators from bindings, and avoid guessing legacy origin.

Owning coverage: the existing real-SQLite profile manager suite owns durable creation/binding
provenance and reopen behavior; the existing daemon extension inventory suite owns its public
projection. Existing-profile identity/default preservation remains required. No production repair
or passing regression is claimed yet.

Evidence under docs/qa/evidence/2026-10-02-untested/:

- profile-race-extension-replay-{preview,before,install,create,catalog}.json
- profile-race-extension-replay-owner-{http,uds}.json
- profile-race-extension-replay-{inventory,status,http-status,ended}.json

Main lab source build: 74e282907-dirty, dev identity, SHA-256
2bba77522e6e2a83045667048d65d5b32f3497caad0bc99d0c2bd170e0ba6e7c. This profile-concurrency replay
does not establish released-version compatibility; the separate versioned legacy lab owns that.

## Repair in progress

The public projection regression fails against marker existence and passes when it requires
confirmed creator provenance for the current profile ID. New creation and binding persist distinct
values; appended Global migration 00126 preserves prior marker rows with unknown origin. Current
profile identity checks prevent an old marker from claiming a manually recreated name. Create-once
planning continues to use marker existence independently.

The same race also exposed an apply-result bug: a create plan was reported as Created even when an
operator created the profile before apply. Its existing managed-install suite fails before repair
and passes after apply consults persisted provenance. This prevents a false extension.profile_created
event for that race. The stored-profile, public projection and migration-preservation checks pass;
full delivery gates and a fresh public replay remain required.

Receipts: extension-creator-projection-red.json, extension-creator-owning-green.json,
extension-creator-apply-race-red.json, extension-creator-apply-green.json, and
extension-creator-migration-preservation-complete.json. Initial codegen and test compile errors are
retained as engineering failures, not product repros; their corrected commands pass.

## Verification — 2026-10-04

A fresh public replay preserves one operator-authored profile during an overlapping install,
reports false creator attribution for that binding, and does not seed extension defaults. A
separate extension-created control reports true. Deleting that empty control and recreating its
name manually reports false for the new profile identity. Legacy unknown markers remain intact
and do not assert a creator. Independent CLI/HTTP/UDS reads and a normal daemon restart preserve
those results. The replay is closed in extension-creator-replay-ended.json.

The delivery gate passes code generation, Go lint with zero issues, race-enabled affected Go
tests (1,736 seconds), and the unchanged cached Web lane. Receipt:
extension-creator-delivery-gate.json. Fix commit: `831436907`. The full
profile race scenario remains untested for its independent claim, trigger, spawn, reservation and
delivery legs; this finding is verified without promoting the entire charter.
