# BUG-20260713-loop-fork-internal-error: Fork & edit fails before a custom Loop can be created

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Bruno
- **Journey Step:** J-06 fork and edit, step 1
- **Scenarios:** LP-021; LP-toggle-loop-goal; LP-delete-custom-loop
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** n/a

## Summary

Bruno clicked `Fork & edit` on the bundled read-only `software-delivery` Loop. The request copied the definition into the workspace and then failed its response projection with a generic `Internal Server Error`, leaving hidden partial state. After the first fix, an extension-backed `reviews-watch` fork opened successfully in the builder, but its first valid Publish failed with `expected_version is required` because the new workspace definition was still at version zero. The complete fork → edit → publish lifecycle therefore remains blocked.

## Reproduction

- **Charter:** CH-007 · **Tour:** Multi-Tab Tour
- **Environment:** desktop / wifi-fast / en-US; isolated daemon at `http://127.0.0.1:58941`; in-app browser.

1. Open the Loops catalog in the isolated workspace.
2. Open the read-only bundled `software-delivery` Loop.
3. Click `Fork & edit`.
4. Observe the page and refresh the Loops catalog independently.

**Expected:** AGH atomically creates a workspace-owned fork and opens it in the builder so the graph/contract can be edited and published under CAS without mutating the bundled source.
**Actual (initial):** `POST /api/workspaces/:workspace_id/loops` returned HTTP 500 in 3 ms; the page stayed on the read-only Loop and exposed only `Internal Server Error`.
**Actual (first-fix replay):** `reviews-watch` forked into `/loops/reviews-watch/editor`; a valid Watch spec edit passed validation, but Publish rejected the v0 CAS with `loop: validation failed: expected_version is required`.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-007-fork-internal-server-error.png`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-007-loop-fork-fixed-builder.dom.txt`
- `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-108e1613c829/runtime/logs/agh.log`: `2026-07-13T02:11:03.297661-03:00`, `POST /api/workspaces/:workspace_id/loops`, status 500.
- The failed initial POST left `.agh/loops/software-delivery/loop.yaml` with an mtime matching the request even though the client received 500, proving non-atomic partial state.
- First-fix replay: the builder rendered `Published v0`, `Validation 0 issues`, and an enabled Publish action immediately before the version-zero CAS rejection.

## Fix

- **Root cause:** The fork handler copied the bundled definition and then projected the response with a plain Loop linter that lacked the runtime Tool schema registry. The copied extension action `ext__dev_cycle__coderabbit_fetch_unresolved` was therefore rejected as `unknown_action_kind` after the filesystem mutation. The first fix made the operation schema-aware and added compensating filesystem/catalog rollback. The browser replay then exposed a second boundary defect: the editor displayed authoritative `loop.version` but froze optional `definition.meta.version`, which is omitted at zero, so the first Publish sent `expected_version: null`. The Web now normalizes the editable definition from `loop.version` on hydration and after Publish.
- **Fix commit:** pending
- **Regression test:** `internal/daemon` canonical Loop resource suite covers extension-backed fork success, rollback on sync and response-projection failures, absent-CAS rejection, and explicit v0 acceptance. The canonical Web LoopEditor suite proves a realistic v0 response sends `expected_version: 0` and advances to v1.

## Verification

- Same-persona replay passed extension-backed fork, builder hydration, deliberate broken-reference validation/recovery, persisted Watch spec update, v0 → v1 Publish, dry-run, real run `looprun-7e6dbcacdf292853`, operator Stop, fresh catalog/detail reads, and reload persistence. Goal toggle and custom deletion are now independently blocked by BUG-20260713-loop-contract-goal-not-editable and BUG-20260713-custom-loop-delete-missing.
