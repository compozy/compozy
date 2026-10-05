---
id: ET-web-theme-toggle-persists
area: ET
title: Switch between light and dark from the rail and keep the choice across reloads
persona: Bruno
journey: J-operate-desktop-shell
expected: A browser with no stored choice opens dark even on a light OS; the rail-foot toggle flips the painted theme in place (sun while dark, moon while light, its accessible name saying what it switches to) and stores an explicit light or dark; the Appearance pane offers Light, Dark and System and reflects the stored choice; a reload paints the stored theme before the first frame with no dark-to-light flash; another open tab adopts the change without a reload.
entry_points: rail-foot theme toggle; Settings > Appearance > Theme; browser reload; second browser tab on the same origin
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-web-theme-system-follows-os; ET-web-light-theme-readability; APP-theme-native-no-flash
---

Added 2026-09-30 for the shell-rail theme (D3/D4). The preference is client-local: localStorage key
`compozy.theme` (`light` | `dark` | `system`, default `dark`), never a daemon or config.toml key. The
pre-paint boot script is the same-origin `/theme-boot.js` (the daemon CSP is `script-src 'self'`).

Walk: clear site data and load the desktop (dark); toggle from the rail foot (light, moon, name "Switch to
dark mode"); reload and watch the first frame (light, no flash); open Appearance (Light selected); choose
Dark there and confirm a second tab repaints without reloading. `<html>` carries `data-theme` and the
`dark` class together; `<meta name="theme-color">` follows the chrome color.

Development evidence (not a QA run): `web/src/systems/theme/__tests__/theme-runtime.test.ts` covers the
store, cross-tab sync and boot-script parity; a Playwright pass on Vite dev confirmed boot paint for every
stored value and reload persistence.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.
