# BUG-20261004-static-web-rewrite-stale-cache: Rebuilt local Web bundle leaves a blank page

- **Status:** fixed — verified
- **Impact (user-side):** Blocked
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-24, enter the automation editor after a local Web rebuild
- **Scenarios:** TA-web-automation-preview-toggle
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

Serve a production Web bundle using COMPOZY_WEB_DIST_DIR and load it in Chrome. Rebuild that
directory without restarting the daemon, then open a fresh document at the same origin.
Chrome revalidates its cached HTML, but the daemon returns 304 using its unchanged start time.
The retained HTML names a removed hashed script, which returns 404 and leaves the app blank.
The existing HTTP suite already promises that local bundle rewrites are visible without restart.

**Expected:** a changed local file invalidates its earlier Last-Modified validator; an unchanged
file retains ordinary conditional 304 responses.
**Actual:** every static response uses the daemon start time, even after the file changes.

## Evidence

Under docs/qa/evidence/2026-10-02-untested/:

- automation-preview-fixed-dora-entry-ended.json and automation-preview-fixed-entry.png
- automation-preview-targets-dora-entry.json and automation-preview-targets-entry-ended.json
- automation-preview-target-bundle-diagnostic.json: cached entrypoint references a removed script
- automation-preview-target-html-cache-proof.json: current HTML differs, but the prior validator gets 304
- automation-static-cache-red.json: root, deep-link and asset conditional requests all reproduce 304

Both interrupted persona sessions ended before diagnosis. A cache-bypassing reload recovered
the first document but did not repair the underlying HTTP behavior.

## Fix

- **Root cause:** serveAsset supplies Handlers.StartedAt to http.ServeContent for every file.
- **Owning layer:** HTTP static content serving, shared by direct assets and SPA entrypoints.
- **Repair:** stat the opened file and use its modification time in both content branches.
  Zero-time embedded assets keep the existing daemon-start fallback. File cleanup, CSP and
  standard conditional/range handling remain owned by the existing handler.
- **Regression invariant:** TestStaticRoutesObserveLocalWebDistRewrite in static_test.go covers
  unchanged 304 and changed 200 responses for root HTML, a deep link and a JavaScript asset.
- **Fix commit:** a2917318c96359b507b0e0a0fc06ad410f7dd648.

## Verification

The regression fails before repair and the static suite passes afterward with the race detector.
The test-shape checker retains the same three untouched legacy findings as HEAD; the changed
suite has none. Same-daemon HTTP requests prove unchanged 304, changed 200, then unchanged 304
for real copied bundle files, plus changed HTML on a deep link. Fresh Chrome entry and ordinary
reload recover without a cache bypass. The required gate passes. Evidence: automation-static-live-revalidation.json,
automation-preview-final-dora-entry.json, automation-preview-final-dora-ended.json and
automation-static-gate-replay.json. Fix SHA: a2917318c96359b507b0e0a0fc06ad410f7dd648.
