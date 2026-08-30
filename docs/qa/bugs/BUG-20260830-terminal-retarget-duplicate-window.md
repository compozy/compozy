# BUG-20260830-terminal-retarget-duplicate-window: Creating a terminal can duplicate its window

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-operate-desktop-shell, open Terminal and create the first terminal
- **Scenarios:** ET-web-window-routing-lifecycle
- **Found:** 2026-08-30 · **Report:** docs/qa/reports/2026-08-28-integrated-terminal-rebase.md

## Summary

Creating the first terminal could leave the original Terminal window open and add a second window
for the same terminal. Both windows attached as viewers, so the desktop showed two copies of one
terminal and no longer had one semantic window identity.

## Reproduction

- **Charter:** packaged Desktop E2E-013 · **Tour:** Feature Tour
- **Environment:** GitHub Actions Linux desktop package / real daemon / current Web bundle / en-US

1. Complete onboarding and select a project workspace.
2. Open Terminal from the Dock.
3. Select `Open a terminal` in the empty state.
4. Type a command in the terminal.

**Expected:** The existing Terminal window retargets to the created terminal and remains the only
window for that terminal.

**Actual:** Two Terminal windows displayed the same terminal id and reported two viewers.

## Evidence

- Exact-head CI run `33314396335`, Desktop job `99265064419`.
- The failure artifact resolved `getByTestId("terminal-window").getByRole("log")` to windows
  `w-ad6733e98671e6cca461c53895` and `w-47cf901507916f7c2d017734ff`, both showing terminal
  `term-bc534e75b0ab`.
- The packaged Desktop scenario passed three consecutive local repetitions after the fix.

## Fix

- **Root cause:** A pending window retarget projected the destination route immediately but kept the
  old `instanceKey`. Concurrent route reconciliation therefore saw a routed terminal with no matching
  semantic instance and opened another window.
- **Production fix:** Window route intents now carry the destination `instanceKey` when the command is
  a retarget. Route-only navigation continues to preserve the existing instance identity.
- **Fix commit:** pending; included in the exact-head CI remediation commit
- **Regression suite:** `web/src/systems/os/hooks/__tests__/window-manager-runtime.test.ts` proves a
  pending retarget projects route and instance identity together, while route-only navigation leaves
  the instance unchanged.

## Verification

- **Unit evidence:** The canonical window-manager runtime suite passes 32/32 through the repository
  Turbo graph.
- **Desktop evidence:** Packaged Desktop E2E-013 passes 3/3, including terminal creation, input,
  clipboard, refit, and IME behavior.
- **Result:** One existing window retargeted to the new terminal in every repetition.
