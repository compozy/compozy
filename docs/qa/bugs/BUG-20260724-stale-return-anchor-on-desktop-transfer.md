# BUG-20260724-stale-return-anchor-on-desktop-transfer: deleting a focus desktop transfers the zoomed window with a stale ReturnAnchor

- **Status:** suspected
- **Impact (user-side):** latent wrong-place restore — next unzoom after a pager delete may target a stale source
- **Persona Affected:** Théo
- **Journey Step:** —
- **Scenarios:** ET-window-manager-layout-gestures; RT-desktop-pager-overview
- **Found:** 2026-07-24 (fix-window-management code sweep)
- **Origin:** static trace, `internal/windowmanager/reducer_desktop_delete.go` (transfer path keeps `window.ReturnAnchor`)

## Summary

Deleting a focus desktop from the Desktops overview transfers its zoomed window to the destination desktop without clearing `window.ReturnAnchor`. The anchor still points at the original zoom source; a later zoom overwrites it, but an unzoom-shaped restore before that could reinstall the window against a source the user no longer expects. Unconfirmed end-to-end — needs a live trace of delete-with-destination on a focus desktop followed by zoom toggling.
