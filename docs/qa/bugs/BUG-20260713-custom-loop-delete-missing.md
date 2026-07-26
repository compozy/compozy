# BUG-20260713-custom-loop-delete-missing: Workspace Loops have no reachable Delete action

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-06 delete the custom fork without removing its built-in source
- **Scenarios:** LP-delete-custom-loop
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md

## Summary

After publishing workspace-owned `reviews-watch` v1, Bruno could configure, edit, run, and inspect it but could not delete it. The detail page has no overflow/destructive action, the builder has no Delete action, and Configure exposes only Reset to defaults, Cancel, and Save. The underlying Web delete adapter and mutation exist but are not wired to any user surface.

## Reproduction

1. Open workspace-owned `reviews-watch` v1 from the catalog.
2. Inspect every detail action and open Configure.
3. Open the builder and inspect its toolbar.
4. Try to remove only the workspace shadow.

**Expected:** A destructive-action modal requires intentional confirmation, deletes only the workspace-owned definition, returns to the catalog, and reveals the bundled read-only source after refresh.
**Actual:** No Delete action or confirmation modal is reachable.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-loop-delete-action-missing.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-loop-delete-restores-readonly-catalog.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-loop-delete-restores-readonly-detail.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-loop-goalless-delete-restored-bundled.dom.txt`
- Source trace: `web/src/systems/loops/hooks/use-loop-actions.ts` exports `useDeleteLoop`, but no Loop route/component consumes it.

## Fix

- **Root cause:** The delete mutation is implemented but is not wired to a workspace-owned Loop UI action or confirmation lifecycle.
- **Fix:** Wire a workspace-only `Delete loop` action into the Loop detail, use the shared `@agh/ui` typed-name `ConfirmDialog`, execute the existing mutation only after exact confirmation, and evict/navigate only after success so failures preserve the current detail cache.
- **Fix commit:** pending final task commit
- **Regression test:** Canonical Loop detail/action suites cover workspace-only visibility, typed-name gating, cancel, one-shot confirmation, success-only cache eviction/navigation, and failure preservation. The worker's focused Loop lane passed 49/49 tests; scoped format/lint and React Doctor 100/100 also passed.

## Verification

- Same-persona in-app-browser replay passed twice on 2026-07-13. An incorrect name kept the destructive button disabled, Cancel preserved the workspace v2 shadow, and exact-name confirmation removed it. Fresh catalog/detail reads exposed the bundled read-only v0 source with `Fork & edit` and no Delete action.
- The strict goal-less replay repeated exact-name confirmation after real run `looprun-c0e322b615e43c12`; navigation returned to `/loops` and a fresh catalog read again showed bundled read-only `reviews-watch` v0 while preserving its run history.
