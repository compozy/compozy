# BUG-20260829-command-palette-portal-remains-mounted: Closing the command palette can leave an invisible blocker

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-operate-desktop-shell, open a desktop app from the command palette
- **Scenarios:** ET-web-command-palette-shortcuts
- **Found:** 2026-08-29 · **Report:** docs/qa/reports/2026-08-28-integrated-terminal-rebase.md

## Summary

Selecting `Open Terminal` focused the Terminal window but could leave the command-palette portal
mounted above the desktop. The transparent overlay kept pointer ownership, so the newly opened app
looked unresponsive even though its window state was correct.

## Reproduction

- **Charter:** CH-terminal-profile-fence · **Tour:** Feature Tour
- **Environment:** isolated Compozy QA lab / current source Web bundle / Chromium / en-US

1. Open the desktop command palette.
2. Select `Open Terminal`.
3. Try to use the Terminal empty state.

**Expected:** The palette and its overlay unmount, Terminal gains focus, and its controls are usable.

**Actual:** Terminal gained focus but the palette portal remained mounted and blocked pointer input.

## Evidence

- The browser accessibility tree showed the Terminal window focused while the palette dialog still
  owned the foreground.
- DOM inspection after the action found one palette and one overlay node still mounted.
- The defect reproduced only after the lab daemon was restarted against the current `web/dist`; the
  earlier embedded bundle mismatch was ruled out first.

## Fix in the working tree

- **Root cause:** The shared Dialog combined Base UI portal ownership with Motion exit nodes. Closing
  changed the dialog state, but the exit lifecycle never completed, so the portal was never removed.
- **Production fix:** Render the shared Dialog portal only while its root is open. Remove the obsolete
  exit-motion helpers and keep the existing component-context error contract in a focused hook.
- **Regression suites:** `packages/ui/src/components/__tests__/dialog.test.tsx` and
  `web/e2e/__tests__/os-shell.spec.ts` (`E2E-008`).

## Verification

- **Unit evidence:** The canonical Dialog suite passes 23/23 and now proves the overlay is absent
  after Escape.
- **Build evidence:** `make web-build` passes with the current bundle.
- **Browser evidence:** A trusted Chromium click opened and focused Terminal; after settlement, the
  palette count and overlay count were both zero.
- **Isolated scenario evidence:** The targeted Web walk recorded the same trusted Chromium action in
  `/Users/pedronauck/.config/browser-harness/agent-workspace/recordings/integrated-terminal-profile-retest`;
  the strict QA evidence audit passed with zero blockers and warnings.
