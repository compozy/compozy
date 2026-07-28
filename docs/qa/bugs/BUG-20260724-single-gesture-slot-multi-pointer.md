# BUG-20260724-single-gesture-slot-multi-pointer: simultaneous pointer drags share one gesture slot

- **Status:** suspected
- **Impact (user-side):** preview/commit corruption only under multi-touch or simultaneous mice
- **Persona Affected:** Bruno
- **Journey Step:** —
- **Scenarios:** ET-window-manager-layout-gestures
- **Found:** 2026-07-24 (fix-window-management code sweep)
- **Origin:** static trace, `web/src/systems/os/stores/window-manager-store.ts` (`gesture` is a single slot) + `web/src/systems/os/hooks/use-os-window.ts` (`handleDrag` reads the global gesture regardless of source window)

## Summary

The presentation store holds exactly one `gesture` session. Two concurrent pointer drags (touch + trackpad, or two touch points on different windows) overwrite each other's preview and race the finish decision. Desktop-first surface makes this low priority; confirm with a two-pointer trace before designing per-pointer sessions.
