# BUG-20261003-profile-project-restoration: Switching projects carries the previous profile

- **Status:** verified
- **Fix commit:** pending
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Sol
- **Journey Step:** J-operate-profiles, switch to another project
- **Scenarios:** ET-profile-switcher-restore
- **Found:** 2026-10-03 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

CH-profile-settings-dialog-plans, Back-Button Tour, keyboard, desktop 1512×862,
en-US, wifi-fast, production Web/daemon at port 50727, commit 74744060b.

1. In Studio Operations, choose navigation-notes from the dock switcher.
2. Open Projects through the command palette's Workspace picker action.
3. Use the keyboard to select Editorial, which has no remembered profile selection.
4. Editorial opens but the switcher still announces Profile: navigation-notes after 12 seconds.
5. Run profile current from Editorial: the CLI reports default, source default.
6. Reload the browser. Editorial now shows Profile: default.

Expected: entering a different project restores that project's remembered profile or default.
Actual: the browser carries the prior project's local profile until reload. No work is created
under the unintended identity during this walk.

## Evidence

In docs/qa/evidence/2026-10-02-untested/: profile-switcher-restore-sol-project-focus.json
records the actual keyboard switch, editorial-observed.json retains the failed 12-second
assertion, editorial-mismatch.json records the accessible profile identity, and
editorial-independent-current.json independently resolves default. The selection read has
no Editorial slot. reload-divergence.json restores default and stops the 59-frame recording.
Screenshots: profile-workspace-identity-carried.png and profile-workspace-identity-after-reload.png.
The ended receipt records that no source was read during the walk. An earlier Projects
option-driver assumption is retained separately and is not a product failure.

Dedup: archived fallback provenance and unavailable-profile stream recovery concern lifecycle
transitions, not selection restoration between two available projects.

## Investigation

useActiveProfileView carries the local view on every lens-key transition. The destination's
remembered choice cannot win because viewEntered deliberately preserves an existing local
answer. The carry was introduced to preserve the profile axis while changing workspace
breadth (workspace ↔ Global); applying it to distinct projects violates per-project restoration.
The existing E2E-013 in web/e2e/__tests__/profiles.spec.ts owns project entry and restoration.
The fix must retain breadth-toggle continuity and external-selection independence.

The repair is within the authorized fix scope: local Web selection lifetime, no schema,
wire or product-policy change. Regression proof and fresh Sol replay remain pending.

## First repair replay — incomplete

Limiting carry to workspace/Global changes and releasing a departed project's local view fixes
first entry into Editorial. E2E-013/015/020 pass that bounded version; the nine existing hook/store
checks pass and React Doctor remains 100/100. A fresh Sol replay then exposes stale re-entry:
while Editorial is active, CLI remembers studio for Studio Operations; returning still shows
navigation-notes. Reload resolves studio. The bug remains open.

Receipts: profile-project-restore-{red,green,unit,doctor}.log and
profile-project-restore-fixed-sol-*. The recording stops at 82 frames, before investigation.
profile-project-return-stale-selection.png captures the mismatch. Driver label and native
Select All assumptions are retained separately; an early probe ran before Studio entry and
is not counted as client-independence proof. Aggregate was not reached in this public replay.

The existing E2E-013 now also changes the remembered choice while its project is inactive,
then returns without reload. The entry hook captures cached selection data before its stale
query finishes refetching; its first-entry rule then ignores the fresh response.

## Final repair and verification

The selection hook distinguishes distinct-project transitions from workspace/Global breadth
changes. It releases a departed project's local view so returning can use the remembered
selection, including after an aggregate view. Entry does not pin data while the selection is
fetching. The owning selection query revalidates on entry: its former 30-second freshness
window could capture an old value before the change event arrived. Current-client local views
still win over external changes; no server selection is rewritten by breadth changes.

The intermediate fetch guard passes the human-paced 157-frame replay but fails the fast
E2E-013 return. Its trace receives default while the browser remains pinned to marketing;
profile-project-return-diagnosis.json indexes the public response evidence. The final
revalidation policy passes E2E-013/015/020 without adding waits or weakening assertions.
Final root-Turbo build/lint pass, with zero lint warnings/errors, and React Doctor scans
47 changed files at 100/100. Build import/chunk warnings remain separately recorded.

Fresh Sol replay, profile-project-entry-final-sol (235 frames), confirms external CLI selection
updates the Settings map while the open client retains research. Project re-entry applies
studio; All profiles does not survive leaving and returning, and reload agrees. Global
remembers editorial independently from Studio's selection, while breadth toggles preserve
the current view. Back after an explicit switch leaves no lifecycle dialog, retains focus
on the profile trigger, and survives reload. CLI/UDS independently confirm the final slots.
The screenshots of Settings, Global and Back are inspected; no product source is read
during the walk. Driver heading and reload-context assumptions are recorded separately.

Regression suite: web/e2e/__tests__/profiles.spec.ts, E2E-013. Existing breadth continuity
and two-client checks retain their own invariants. Evidence: profile-project-entry-final-*.
Spoken VoiceOver output remains externally unverified; this does not qualify as a full
accessible-charter pass. Delivery gate and commit evidence are recorded in the dated report.
