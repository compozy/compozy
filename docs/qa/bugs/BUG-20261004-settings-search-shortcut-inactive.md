# BUG-20261004-settings-search-shortcut-inactive: Settings search ignores its advertised shortcut

- **Status:** open
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Dora
- **Journey Step:** J-administer-runtime-settings, find a setting from the keyboard
- **Scenarios:** MS-web-settings-takeover-redesign
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Dora sees `/` beside Search settings, presses it after opening Settings or reading a section,
and nothing happens. Clicking the search field works, including the three retired section names.
The promised keyboard entry requires an extra pointer action.

## Reproduction

- **Charter:** CH-untested-041-administer-runtime-settings-dora · **Tour:** Back-Button Tour
- **Environment:** isolated real daemon, Chrome 1512 × 862, en-US

1. Open Settings from the app, with its desktop sidebar visible.
2. Press `/` before focusing a field. The advertised search remains unfocused.
3. Open Notifications, click the neutral text Changes apply immediately, and press `/` again.
4. Observe for three seconds. Focus remains on the desktop container; no dialog is open.
5. Click Search settings and type gateway, attention or observability. Each alias finds its section.

**Expected:** In the active Settings window, `/` focuses the visible search unless a field,
dialog or another keyboard action owns the keystroke.
**Actual:** The shortcut only listens inside the Settings content subtree, which does not own
the browser's focus after entry or a neutral content click.

## Evidence

Receipts under docs/qa/evidence/2026-10-02-untested/:

- settings-navigation-dora-fresh-entry.json and settings-navigation-dora-back-shortcut-responsive.json:
  fresh entry, physical Slash key, focus and independent alias navigation observations.
- settings-navigation-dora-body-shortcut.png: the visible shortcut after the clean failed attempt.
- settings-navigation-dora-ended.json: closed 43-frame persona session; four screenshots inspected.
- settings-shortcut-engineering-focus-path-valid.json: post-session DOM diagnosis locates focus
  at #app-content, an ancestor of the Settings listener; the search is visible at 181 × 24 pixels.

The earlier header-click attempt opened Full title and is not a clean reproduction. Search typing
before route focus settled also is not a search failure. A diagnostic script quoting error ran no
browser actions; its corrected receipt is separately retained.

## Fix

- **Root cause:** Settings registers a React bubbling handler on its own content wrapper. The
  focusable desktop ancestor receives neutral-content clicks, so those keyboard events never
  descend into Settings. Its search ref and ordinary search behavior work.
- **Fix commit:** Pending.
- **Regression test:** The existing settings-window-nav.test.tsx mounts the real Settings window,
  Query client and window-manager runtime. Three focus assertions fail before repair. All nine
  navigation cases pass after repair; the adjacent Marketplace suite also passes (72 tests total).

The repair reuses the existing document-level listing-search shortcut, enabled only for the
active, visible Settings window, and preserves editable/dialog/handled-key ownership. No API,
schema or persistence contract changes are needed.

## Verification

Dora's fresh Tasks-to-Settings entry focuses search in 0.003 seconds. The neutral-content case
also focuses search; typing attention/ retains the slash. Full title keeps keyboard ownership
until Escape dismisses it. The observability alias reaches Diagnostics and browser Back returns
to Notifications. The Marketplace canary retains slash focus, ordinary slash typing and Escape
clear. Its owned window is closed. Settings reload has an empty search and a working shortcut.

Both profile policies retain true/true/false channels and empty mutes. The 24-frame recording is
closed and all four screenshots are inspected; no source changes occurred during the replay.
Evidence: settings-shortcut-repair-dora-ended.json and settings-shortcut-build-identity.json.
The served index exactly matches the rebuilt bundle. Root Turborepo typecheck/build pass;
React Doctor stays at 93/100 with the same three pre-existing complexity warnings. The build
retains its unrelated CSS/chunk/dynamic-import warnings; it is not warning-free delivery evidence.

The first post-fix test fixture reused a presentation revision, so the real runtime correctly
ignored its simulated focus change. Advancing fixture revisions repairs the input without
weakening an assertion. Browser setup rejected an already-active locale override; locale and
timezone were read before continuing. A premature reopen and a post-Escape key preceded focus
settlement; semantic waits resolved both. Those driver attempts remain in their own receipts.
Delivery gate and commit are pending; the full Settings scenario has other unfinished legs.
