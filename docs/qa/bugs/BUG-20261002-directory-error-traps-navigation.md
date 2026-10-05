# BUG-20261002-directory-error-traps-navigation: An unreadable folder disables every way back

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Dora
- **Journey Step:** J-operate-workspace-context, project registration
- **Scenarios:** MS-web-workspace-add-directory-browser
- **Found:** 2026-10-02 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Dora enters a folder without read permission while adding a project. The error is visible, but
Home and Up become disabled and Locations disappears. Closing and reopening the dialog retains
the inaccessible folder; she must reload the page and repeat the browse from the beginning.

## Reproduction

- **Charter:** CH-untested-071-operate-workspace-context-dora · **Tour:** Back-Button Tour
- **Environment:** isolated macOS lab; desktop, 1512 × 862, wifi-fast, en-US.

1. Open the project menu and choose Add project.
2. Browse into the lab's readable `research-notes` directory, then its mode-000 `restricted-notes` child.
3. After the permission error appears, try Up and Home.
4. Close and reopen the dialog. Reload the page and repeat once from the real entry point.

**Expected:** The permission error remains visible while known parent, home, and root destinations
remain usable. Browsing and abandoning the dialog create no workspace.

**Actual:** Both navigation buttons are disabled and the Locations row disappears, including after
the clean page retry. No workspace was submitted; the independent UDS catalog remains unchanged.

## Evidence

- `docs/qa/evidence/2026-10-02-untested/workspace-browser-home-recovery-inspection.json`
- `docs/qa/evidence/2026-10-02-untested/workspace-browser-parent-fresh-page-retry.json`
- `docs/qa/evidence/2026-10-02-untested/workspace-browser-permission-clean-retry.png`
- `docs/qa/evidence/2026-10-02-untested/workspace-browser-after-browse-catalog.json`
- Recording: `/Users/pedronauck/.config/browser-harness/agent-workspace/recordings/compozy-workspace-browser-20261002` (closed, 26 frames).

## Fix

- **Root cause:** Both browser consumers derive every navigation anchor from the current directory
  query's successful response. A new failed query has no data, discarding anchors already learned
  from the parent listing. The query error itself is correct.
- **Repair scope:** Retain known navigation anchors at the shared browser's navigation event;
  preserve the failed path and error, without reusing another directory's contents as its result.
- **Fix commit:** `ebfb89518`.
- **Regression test:** Existing `directory-browser.test.tsx`, through the real setup/browser hooks
  and a mocked filesystem adapter boundary; real permission-failure replay is also required.

## Verification

- **Retested:** 2026-10-02, Dora through a fresh production-page load and the shared onboarding browser.
- **Result:** A real mode-000 directory still reports permission denied, with Up, Home, and Locations
  available. Each destination successfully leaves the failure. Onboarding independently preserves
  the same recovery controls. The owned fixture was restored to mode 0755 after both walks.
- **Coverage:** POSIX and Windows navigation cases fail before repair and pass afterward in the
  existing suite. All 693 Web suites / 6,900 tests, typecheck, and production build pass. React Doctor
  reports 100/100. `make gate` passes every affected lane; lint reports zero warnings and errors.
  The first validation exposed an invalid test selector option, corrected without changing its
  behavioral assertions; a harness prop-spread warning was also repaired.
- **Evidence:** `directory-recovery-{red,green,web-validation-final,react-doctor,gate}.log`,
  `directory-recovery-real-replay.json`, `workspace-browser-permission-fixed.png`, and
  `workspace-browser-onboarding-recovery.json` under the dated evidence directory.
- **Remaining journey:** Project registration and refresh also succeed, but the adjacent Skip
  action retained the previous project scope. That distinct handoff finding keeps the complete
  scenario pending; it does not invalidate the confirmed directory-navigation repair.
