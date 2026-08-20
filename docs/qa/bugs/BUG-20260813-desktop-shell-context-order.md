# BUG-20260813-desktop-shell-context-order: Desktop boot reads shell context before providing it

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Every Web operator
- **Journey Step:** Open the CompozyOS desktop
- **Scenarios:** RT-worktree-web-nested-navigation; MS-web-workspace-add-directory-browser; ET-palette-registry-driven-root; ET-web-desktop-shell-lifecycle
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

## Re-found and fixed (2026-08-20)

The command-palette registry introduced another shell-context consumer in `DesktopChrome`, above
the provider owned by the same component. Opening the daemon-served desktop hit the root error
boundary with the original `useOsShell` provider error before any palette journey could begin.

- **Report:** `docs/qa/reports/2026-08-20-command-palette.md`
- **Root cause:** `useCmdPaletteRegistry` reads Window Manager topology but was called before
  `OsShellContext.Provider` mounted.
- **Fix commit:** `531b9f5`
- **Regression test:** documented daemon-served Playwright replay in
  `web/e2e/__tests__/agent-categories.spec.ts`; the journey failed at desktop boot before the fix and
  passed from a fresh worker afterward. The shared provider topology has no meaningful isolated
  component contract beyond this real shell mount.
- **Retest:** The focused daemon-served journey rendered `os-desktop`, opened Agents, navigated the
  fleet, and completed in 6.9 seconds. Full charter verification remains in this run.

## Re-found and fixed (2026-08-20, command-palette delivery)

`feat: complete command palette delivery` added live palette client-context reads
(`useFocusedSessionId` / `useDesktop`) inside `useDesktopChrome`, which is the hook that *creates*
the shell handle. Opening `/` under `make dev` hit the original provider error again.

- **Root cause:** the chrome hook asked `OsShellContext` for the projection atom it already owns.
- **Fix:** read `manager.projectionAtom` with `useAtom` and keep context consumers below the
  provider. Regression: `web/src/systems/os/hooks/__tests__/use-desktop-chrome.test.tsx`.
