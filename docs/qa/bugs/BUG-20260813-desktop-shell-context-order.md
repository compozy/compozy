# BUG-20260813-desktop-shell-context-order: Desktop boot reads shell context before providing it

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Every Web operator
- **Journey Step:** Open the CompozyOS desktop
- **Scenarios:** RT-worktree-web-nested-navigation; MS-web-workspace-add-directory-browser
- **Found:** 2026-08-13 · **Report:** docs/qa/reports/2026-08-13-worktree-support.md
- **Origin:** Task 10 release QA

## Summary

Opening `/` rendered the root error boundary instead of the desktop. `useDesktopShellModel` read the
window-manager projection through `useDesktop` before `DesktopShell` mounted its own
`OsShellContext`, so every browser journey was blocked.

## Reproduction

1. Start the Web app against a healthy isolated daemon.
2. Open `/` in a browser.
3. Observe `Unable to render this route` with
   `useOsShell requires an <OsShellContext.Provider> above (DesktopShell)`.

## Fix

- Move focused-window worktree selection and stale-scope pruning into `DesktopShellBody`, below the
  provider owned by `DesktopShell`.
- Keep workspace and worktree catalog reads in `useDesktopShellModel`; they do not depend on the OS
  shell context.
- Regression suite: `web/src/systems/os/hooks/__tests__/use-desktop-shell-model.test.tsx`.
- **Fix commit:** `8ec45d75`

## Verification

- The focused Web test, typecheck, and zero-warning lint pass through Turborepo.
- Browser replay renders `data-testid="os-desktop"` and the onboarding panel instead of the root
  error boundary.
- Evidence:
  `/Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/`.
