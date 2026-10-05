# BUG-20261004-automation-offline-pagination-silent: Load more gives no feedback while offline

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Bruno
- **Journey Step:** J-24, continue an already loaded automation catalog after losing connectivity
- **Scenarios:** ET-web-jobs-triggers-catalog
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Bruno clicks Load more jobs after losing connectivity. The saved rows remain, but the control
looks unchanged and available; nothing explains why no further jobs appear.

## Reproduction

- **Charter:** CH-automation-catalog-recovery · **Tour:** Interrupt Tour
- **Environment:** desktop Chrome, en-US, real isolated daemon on 55651, build 78133b4f.

1. Open Jobs and load the first 100 of 122 visible definitions.
2. Take the browser offline, then activate Load more jobs.
3. Observe the unchanged control and rows for 12 seconds.
4. Restore connectivity; the pending page completes.

**Expected:** keep loaded rows and identify the request as waiting for connectivity; repeated
activation must not pretend that another request can start.
**Actual:** no error, waiting label or disabled control is exposed.

## Evidence

- docs/qa/evidence/2026-10-02-untested/catalog-recovery-bruno-offline-pagination-bound.json
- docs/qa/evidence/2026-10-02-untested/catalog-offline-pagination-bound.png

The earlier cross-invocation emulation attempt reset online and is explicitly not evidence.
The corrected receipt records navigator.onLine=false before and after the bounded observation.

## Fix

- **Root cause:** the catalog projects isFetchingNextPage but drops Query's paused state.
  An offline request is paused, not fetching, so the button renders its idle branch.
- **Owning layer:** shared AutomationCatalogShell pagination presentation and its Jobs/Triggers
  view-model inputs. Reuse the existing Button and Query state; do not fabricate an error.
- **Regression test:** existing automation-catalog.test.tsx; paused pagination retains content,
  exposes its connection wait and refuses another activation.
- **Fix commit:** 3f53932aa35300c32e92bb6a84d469e1d89f367c.

## Verification

The owning catalog suite fails before the repair and passes all 13 tests afterward.
Jobs and Triggers both retain their first 50 rows while offline, expose the disabled
Waiting for connection control within 0.48 seconds, and continue to 100 rows on reconnect.
The original rows remain present. Both screenshots were inspected. The runtime and browser
connection are restored, and the fresh Bruno recording is stopped.

Receipts: catalog-offline-pagination-{red,green}.json/.log;
catalog-recovery-replay-jobs-offline-observed.json;
catalog-recovery-replay-triggers-offline.json; catalog-recovery-replay-bruno-ended.json.
Checkpoints: catalog-recovery-{jobs,triggers}-offline-fixed.png.
All paths are under docs/qa/evidence/2026-10-02-untested/.

The first replay probe assumed 100 initial rows and stopped before enabling offline mode;
the corrected probe measures the actual loaded page. It is retained as a driver error.
The required gate passes all affected lanes, including 693 Web files / 6,915 tests.
catalog-recovery-commit-proof.json confirms the committed tree equals the checked staged tree.
