# BUG-20261003-profile-rename-repositories-unchecked: Rename leaves every repository folder declined by default

- **Status:** verified
- **Fix commit:** a15b2ea62
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Sol
- **Journey Step:** J-operate-profiles, preview and confirm a profile rename
- **Scenarios:** ET-profile-web-settings-lifecycle-dialogs
- **Found:** 2026-10-03 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Sol sees both repository folders offered for renaming, but both are already declined. Confirming
without noticing would leave their content under the old name, contrary to the planned default.
The machine folder and repository paths themselves match the public plan.

## Reproduction

- **Charter:** CH-profile-settings-dialog-plans · **Tour:** Back-Button Tour
- **Environment:** desktop 1512×862, wifi-fast, en-US, isolated daemon 53876, build b4ab86b39

1. Open Settings → Profiles with the keyboard.
2. Choose Rename for release-drafts, whose folders are committed in editorial-desk and reference-library.
3. Type publication-drafts and wait for the two repository offers.
4. Inspect the checkboxes without changing either one.

**Expected:** Both repository offers start checked; the operator can decline each explicitly.
**Actual:** Both are unchecked. Escape restores focus and leaves release-drafts unchanged.

## Evidence

All paths below are relative to docs/qa/evidence/2026-10-02-untested/.

- profile-rename-repositories-unchecked.png, inspected screenshot of both unchecked offers.
- profile-lifecycle-sol-rename-preview-read.json, complete AX checked=false states.
- profile-lifecycle-sol-plan-independent.json, public plan with both repository candidates.
- profile-lifecycle-sol-rename-cancel.json, Escape focus restoration.
- profile-lifecycle-sol-original-after-cancel.json and profile-lifecycle-sol-newname-after-cancel.json,
  independent UDS reads: original id survives and proposed name returns profile_not_found.
- profile-lifecycle-sol-ended.json; recording profile-lifecycle-sol stopped with 71 frames.

The first selector wait used an incomplete accessible name and timed out. That driver error is
separate from the complete AX and screenshot evidence. No source or database reads occurred in persona.

## Fix

Root cause: useProfileLifecycle initializes acceptedRepos to an empty array and changes it only
when a checkbox is toggled. Neither plan arrival nor its candidate list supplies the initial selection.
The bounded repair retains explicit declines and derives accepted ids from the current plan,
so query refreshes preserve user choices and cannot submit a removed candidate.

Owning regression: existing profile Playwright E2E-016. The browser lifecycle composition owns
initial selection, decline persistence while editing the name, and the resulting accepted-only
rename. Real repository and daemon behavior provide independent persistence evidence.

## Verification

The expanded E2E-016 fails before the repair at its first checked repository offer, matching the
real keyboard walk. Evidence: profile-rename-repositories-e2e-before.log and the preserved
profile-rename-repositories-e2e-before-artifacts/ directory. The repair changes only the existing
transient state and lifecycle composition; it adds no synchronization effect or second plan cache.
The unchanged browser regression passes after the repair, alongside E2E-017 (2 cases, 18.9 seconds).
Root Turbo build/typecheck passes. React Doctor initially flags repeated array membership inside
the candidate loop; using the existing Set pattern resolves it without suppression, yielding
100/100 across 39 changed files. Receipts: profile-rename-repositories-e2e-after.log,
profile-rename-repositories-build-final.log and profile-rename-repositories-react-doctor-final.log.

## Fresh Sol verification

Both offers start checked. Declining reference-library survives changing publication-drafts to
publication-notes. Escape restores the Rename button and UDS reports no publication-notes profile.
Reopening starts with an empty name and resets both offers to checked.

Sol declines reference-library again and reviews the publication-drafts plan. A terminal color edit
changes the plan revision from d02d463f… to 19876ca2…. The first Web confirmation quotes the old
revision and only editorial-desk; HTTP returns 409 profile_plan_stale. The dialog displays an inline
review request and fetches the new plan, retaining the declined folder. Independent UDS reads prove
no partial rename. A second explicit confirmation quotes the new revision and succeeds.

The public response marks editorial-desk renamed and reference-library not_selected. UDS preserves
profile id 01M40GH5Z6GTP9DZA29ZMXE4GE and the updated color under publication-drafts. Filesystem and
Git reads retain the exact release notes: only editorial-desk and the personal folder move. The
repository move remains an uncommitted change for the operator. Reload retains the renamed profile.

Receipts: profile-rename-fixed-sol-*.json; screenshots profile-rename-stale-plan-reasked.png and
profile-rename-fixed-persisted.png. Recording profile-rename-fixed-sol is stopped (131 frames).
No source or database reads occurred in persona. The later project menu opens a worktree submenu;
Global remains active, so the dormant-content hint was not reached and is not judged here.
Spoken screen-reader output and the full lifecycle charter remain Pending.

The first delivery gate rejects filter().map() under the repository's stricter single-pass lint
rule. The derivation now uses one for-of pass with the same Set membership and accepted ids;
no rule is suppressed. Final gate and rebuilt E2E evidence follow this mechanical adjustment.

The single-pass derivation initially pushes the dialog host over the React complexity threshold
(93/100). It now lives in the existing lifecycle view model beside its plan query; the host only
consumes acceptedRepos for both rendering and submission. This keeps server-derived state at its
existing owner. Final proof is recorded in profile-rename-repositories-model-*.log.

Final delivery evidence: profile-rename-repositories-delivery-proof.json (commit a15b2ea62).
