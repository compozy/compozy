# BUG-20260724-placement-cycles-unpruned: progressive snap cycle entries survive window close

- **Status:** suspected
- **Impact (user-side):** cosmetic/memory — per-session map growth and a stale cycle step if a window id is reused
- **Persona Affected:** Bruno
- **Journey Step:** —
- **Scenarios:** ET-window-manager-layout-gestures
- **Found:** 2026-07-24 (fix-window-management code sweep)
- **Origin:** static trace, `web/src/systems/os/stores/window-manager-store.ts` (`placementCycles` pruned only via `trackPlacementTarget(windowId, null)` and bind/unbind)

## Summary

`placementCycles` entries are keyed by window id and removed only when a non-tile target tracks or the client rebinds. Closing a window leaves its entry behind; reopening the same app resumes the old progressive-ratio step instead of starting at one-half. Bounded per session and cosmetic, but worth pruning on window close during the next window-manager pass.
