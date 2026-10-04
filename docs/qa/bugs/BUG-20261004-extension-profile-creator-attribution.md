# BUG-20261004-extension-profile-creator-attribution: Bound profiles are attributed to the extension as newly created

- **Status:** open
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
