# BUG-20260713-missing-workspace-persists: A removed local folder remains as a ghost workspace

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-prune-missing-workspace, steps 2-4
- **Scenarios:** RT-missing-workspace-pruned
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** AGH-47

## Summary

Bruno registered the lab-owned `ghost-prune-probe` folder through the Add workspace modal, refreshed it successfully, and then removed that folder outside AGH. The active dashboard changed to `Unable to load dashboard` with `workspace root directory no longer exists`, but AGH did not reconcile the registration. Switching to a valid workspace recovered normal use, yet the ghost remained in the switcher, the dashboard workspace count, and the public workspace list after another full refresh.

## Reproduction

- **Charter:** CH-prune-missing-workspace · **Tour:** Interrupt Tour
- **Environment:** desktop / wifi-fast / en-US; isolated daemon at `http://127.0.0.1:58941`; in-app browser.

1. Create a disposable local folder under the isolated QA lab.
2. Open Add workspace, register its absolute path, and confirm it becomes the active workspace.
3. Refresh the dashboard and confirm the workspace is listed and usable.
4. Remove only that disposable folder through the filesystem.
5. Refresh while the removed workspace is active.
6. Switch to another valid workspace and refresh again.
7. Read `GET /api/workspaces` and the removed workspace by ID.

**Expected:** The next reconciliation prunes the missing registration, selects a valid fallback workspace, removes the ghost from all public catalogs, and makes an old selection recover without manual deletion.
**Actual:** The active workspace fails with `workspace root directory no longer exists`; manual switching recovers the dashboard, but `ghost-prune-probe` remains in the switcher, the count remains four, `GET /api/workspaces` retains `ws_73db983811b21119`, and its direct read returns HTTP 410.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-prune-workspace-registered.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-prune-after-folder-removal.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-prune-ghost-persists-after-fallback-refresh.dom.txt`
- Public registration: `ws_73db983811b21119`; direct read after removal: HTTP 410 with `workspace root directory no longer exists`.
- `agh workspace list --json` against the isolated UDS retained the same ghost, matching Web and HTTP rather than reconciling it.

## Fix

- **Root cause:** Workspace reads detected a missing canonical root, but the authoritative list path returned the persisted registration without reconciling it. Every catalog surface therefore retained the ghost until an explicit delete.
- **Correction:** `Resolver.List` now reconciles registrations before returning them. A canonically missing root is unregistered through the owning resolver path, while transient resolution failures remain fail-closed and do not delete state. Concurrent list calls converge on the same deletion. The session manager now stages every stopped session directory before the catalog cascade; database failure restores the directories, while a committed unregister removes the tombstones and publishes exact catalog deletions.
- **Fix commit:** pending
- **Regression test:** Canonical workspace resolver and resolver integration suites cover missing-root pruning, transient-error preservation, concurrent reconciliation, and list/read behavior.

## Verification

- **Retested:** 2026-07-13T10:38Z → 2026-07-13T10:40Z in the original isolated lab, after rebuilding and restarting the daemon with the fixed source.
- The first fresh Web workspace-list read pruned `ws_73db983811b21119`; the workspace rail showed only the three valid registrations and the active Task route remained usable.
- `GET /api/workspaces` and `agh workspace list --json` over the isolated UDS returned the same three valid workspaces; direct read of the former ID returned HTTP 404.
- A second complete daemon restart retained the reconciled catalog, proving the removal was persisted rather than hidden in one Web cache.
- Fixed evidence: `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/rt-missing-workspace-pruned-first-ui.dom.txt`.
- **Final ownership retest:** On 2026-07-14, the UI registered `workspace-prune-final`, created and stopped real Cursor/Grok session `sess-0f2fc3f71bf6b69e`, and then reconciled after the folder was removed. The workspace disappeared from the switcher without an error, `agh3` became the valid fallback, UDS listed only the three healthy registrations, and the staged session directory was gone.
