# BUG-20260724-arrange-preset-overlap-reject: two-up/quad arrange presets fail whenever a non-participant tiled window remains

- **Status:** open
- **Impact (user-side):** silent-failure — the zoom-menu arrange presets do nothing on desktops with extra tiled windows
- **Persona Affected:** Bruno
- **Journey Step:** —
- **Scenarios:** ET-window-manager-layout-gestures
- **Found:** 2026-07-24 (fix-window-management code sweep)
- **Origin:** static trace, `web/src/systems/os/lib/window-manager-command-builders.ts:96-103` + `internal/windowmanager/reducer_layout_arrange.go` + `internal/windowmanager/validate.go` (`topology.group_overlap`)

## Summary

The zoom-menu arrange presets (`two-up`, quad) always emit `frame={0,0,1,1}`. The reducer removes only the participant windows and appends one full-rect group; any remaining tiled window keeps its sub-frame group, the new group overlaps it, and `ValidateSnapshot` rejects the commit. The command fails with a diagnostic and the layout does not change.

Reproduce: tile 3+ windows so at least one stays out of the preset's participant pick, open the zoom menu, choose "two-up" — nothing happens.

Fix direction (design decision needed): reflow non-participant groups out of the claimed frame (the desktop-delete transfer machinery already places islands without overlap), compute the arrange frame from the participants' current frames, or define preset semantics as desktop-wide. Related unconfirmed symptom in the same validator family: edge-tiling into a half already claimed by another group may hit the same `group_overlap` reject — trace before fixing.
