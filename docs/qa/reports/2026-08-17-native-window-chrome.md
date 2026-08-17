# QA Run Report — 2026-08-17 — Native window chrome

- **Scope:** Native Electron window controls integrated into the Compozy menubar on macOS and Linux, with unchanged browser chrome.
- **Cadence tier:** targeted
- **Build:** cbd8d097 + working tree · **Environment:** fresh isolated lab at `http://127.0.0.1:49393`; macOS host; Linux desktop environment unavailable locally
- **Started:** 2026-08-17T19:02:46Z · **Status:** blocked-verify

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Dora | Power User | desktop / wifi-fast / en-US | CH-native-window-chrome-macos, CH-native-window-chrome-linux |

## Flows in Scope

- `J-desktop-attach-daily` — The desktop app remains a native second door to the same runtime (`../journeys/J-desktop-attach-daily.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-native-window-chrome-macos | J-desktop-attach-daily / APP-native-window-controls | Dora | Feature Tour | Pass | | |
| 2 | CH-native-window-chrome-macos | J-desktop-attach-daily / APP-window-geometry-recovery | Dora | Feature Tour | Pass | | |
| 3 | CH-native-window-chrome-macos | J-desktop-attach-daily / APP-quit-contract | Dora | Feature Tour | Pass | | |
| 4 | CH-native-window-chrome-linux | J-desktop-attach-daily / APP-native-window-controls | Dora | Feature Tour | Blocked (needs human verify) | | |
| 5 | CH-native-window-chrome-linux | J-desktop-attach-daily / APP-window-geometry-recovery | Dora | Feature Tour | Blocked (needs human verify) | | |
| 6 | CH-native-window-chrome-linux | J-desktop-attach-daily / APP-quit-contract | Dora | Feature Tour | Blocked (needs human verify) | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-native-window-chrome-macos

- The packaged Electron 43.4.0 product window exposed a visible 44px Window Controls Overlay area narrower than the renderer, while the native controls remained owned by macOS.
- Relaunch preserved maximized state and recovered disconnected-display bounds; quitting the shell left the runtime alive and attachable.
- Local E2E launches used `showInactive()`: boot and product windows rendered without taking focus from the operator's current app.
- The browser canary at `http://127.0.0.1:49393` rendered a 44px System bar and a full-width safe area at `x=0`; `windowControlsOverlay.visible` was false. Evidence: `/Users/pedronauck/dev/qa-labs/compozy-native-window-chrome-20260817-190228-135313-lab/qa-artifacts/qa/evidence/browser-menubar.png`.
- Lens pass: usability, accessibility, perceived performance, compatibility on the macOS host, recovery, and local-build parity showed no changed-surface finding. The browser setup marker was completed through the public onboarding endpoint solely to reach the shell.

### CH-native-window-chrome-linux

- No Linux desktop environment was available in this lab. Electron policy and automated platform coverage are present, but native control side/count, window-manager interaction, geometry, and quit remain human verification items.

## What Was Fixed

No product defect was found. During validation, the desktop E2E harness was changed to present windows with Electron's non-activating `showInactive()` API, and its packaged integration now asserts zero focused Compozy windows. A stale Settings navigation expectation was aligned with the canonical web E2E list after the current Attention section exposed it.

## Paper Cuts

None.

## Runtime Errors Observed

None from the product. The Playwright worker printed its existing `NO_COLOR`/`FORCE_COLOR` process warning; it did not affect the app or verdicts.

## Human Verifications Needed

- Walk `CH-native-window-chrome-linux` on at least one X11 or Wayland desktop environment.
- Confirm the window manager's native button side/count, drag region, minimize/maximize/close behavior, relaunch geometry, and runtime survival after close.

## Decisions for a Human

None.

## Learnings

- Electron's Playwright launcher has no supported `headless` option. `BrowserWindow.show()` activates the app on macOS, while `showInactive()` preserves real rendering without stealing focus and remains compatible with the Xvfb/X11 Linux lane.
- The same CSS safe-area wrapper degrades cleanly in a normal browser because the `titlebar-area-*` fallbacks resolve to `x=0` and `width=100%`.

## Final Status

**Blocked only on Linux human verification.** macOS packaged behavior, browser fallback, geometry, quit survival, desktop unit/typecheck, and all 18 desktop E2E scenarios have passing evidence across the full run plus the focused stale-expectation rerun. The scoped and final repository gates are recorded separately at workstream close.
