# BUG-20261003-profile-delete-orphans-automations: Deleting an apparently empty profile strands its automations

- **Status:** verified
- **Fix commit:** 74744060b
- **Impact (user-side):** Data-Loss
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Sol; Ada
- **Journey Step:** J-operate-profiles, delete only an empty archived profile
- **Scenarios:** ET-profile-web-settings-lifecycle-dialogs; ET-profile-cli-lifecycle
- **Found:** 2026-10-03 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

CH-profile-settings-dialog-plans, Back-Button Tour, desktop 1512×862, en-US, wifi-fast,
isolated production Web/daemon at port 53876, build b4ed8ca18.

1. Keep the paused Monthly release digest job and Release notes review trigger owned by
   publication-drafts. Public CLI and HTTP reads show both definitions.
2. Open Settings → Profiles by keyboard. The profile is labeled Empty.
3. Archive it, expand Archived, and open Delete. The dialog lists only Profile identity and settings.
4. Cancel with Escape, reopen and confirm Delete profile.
5. Reload Settings and read the owner through UDS. It is absent with profile_not_found (404).
6. List jobs and triggers with --all-profiles. Both commands fail with an automation profile owner
   not found error, naming the removed stable owner ID.

Expected: disabled and enabled automation definitions count as owned work. A nonempty profile
cannot be deleted; its existing definitions and the aggregate catalog remain accessible.
Actual: deleting the owner succeeds visibly and persists. The definitions remain but their owner
does not, breaking both aggregate automation catalogs. No scheduled or triggered execution occurred.

## Evidence

In docs/qa/evidence/2026-10-02-untested/: profile-delete-navigation-sol-* captures the public
catalog, job and trigger reads, archived plan, keyboard confirmation, independent 404 and both
CLI errors. profile-delete-omits-existing-automations.png and
profile-deleted-automation-owner-absent.png were inspected. Recording profile-delete-navigation-sol
is stopped with 168 frames. No source/database reads occurred during the walk.

The transport capture used an overly strict URL suffix filter and missed the mutation query;
no mutation HTTP status is claimed. UI disappearance, fresh UDS and CLI failures prove the result.

## Investigation

Both profile ownership counters read only legacy automation tables. Canonical resource definitions
are omitted from the Settings work count and the transactional deletion guard. The archive pause
repair is independently verified; deletion needs the same canonical ownership truth without
double-counting definitions that also retain a legacy row.

Owning invariant: canonical and legacy automation definitions count once under their actual owner,
regardless of enabled state; deletion is refused until that work is removed. Owning layer and suite:
profile/resource composition in internal/automation/resource_test.go, with real SQLite/resource stores.

## Repair and verification

The two existing profile counters now read canonical automation resource owners and legacy
definitions that are not shadowed by the same resource identity. Enabled state does not alter
ownership. The transactional delete guard and public work counts reuse those counters.
No resources are deleted to make a profile appear empty, and no catalog error is suppressed.

TestProfileDeleteResourceAutomations in the existing automation resource suite fails with zero
items before the fix for dynamic, config and package sources. It then proves single/list counts,
legacy deduplication, enabled/paused ownership, active/archived deletion refusal, retained records,
and successful deletion only after the owned definitions are removed. Neighboring owners survive.
The focused race-enabled automation/profile suites pass; the convention checker retains exactly
the eight pre-existing findings. Receipts: profile-delete-ownership-{red,race}.log.

Fresh Sol replay resumes the main lab of the same active QA goal on the rebuilt binary. Settings
shows two items and omits Delete for the archived owner. Public CLI deletion refuses both active
and archived states with profile_owns_work. A separate empty archived profile can be canceled and
then deleted with its exact plan revision, returning HTTP200; UDS 404 and reload confirm removal.
Both aggregate automation catalogs remain healthy and retain the paused job/trigger with their
owner. Recording profile-delete-fixed-sol stops with 85 frames. Receipts: profile-delete-fixed-sol-*;
inspected screenshots: profile-delete-owned-work-protected.png, profile-delete-empty-enumeration.png,
and profile-delete-protection-persisted.png. The old broken lab is preserved on disk and torn down
with clean=true, per profile-dialogs-terminal-teardown.json. Full lifecycle charters remain Pending.
