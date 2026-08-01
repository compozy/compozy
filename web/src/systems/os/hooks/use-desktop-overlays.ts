import { useState } from "react";

import { windowManagerStore } from "../stores/window-manager-store";
import { useWindowManagerOverlay, useWindowPaletteIntent } from "./use-window-manager-store";

/**
 * One id per shell overlay, including one per menubar menu. Sharing a single
 * slot is what keeps them mutually exclusive: opening the palette closes a
 * menu, and hover-switching between menubar menus resolves through the
 * race-safe functional update below regardless of which side fires first.
 */
export type DesktopOverlay =
  | "compozy-menu"
  | "workspace-menu"
  | "session-menu"
  | "go-menu"
  | "window-menu"
  | "help-menu"
  | "bell"
  | "palette"
  | "workspaces"
  | "desktops"
  | "sessions"
  | "shortcuts"
  | "about";

type LocalDesktopOverlay = Exclude<DesktopOverlay, "desktops">;

/**
 * One owner for shell overlays; Desktops Overview lives in the WM interaction
 * store. A live palette intent (new-tab picker, deck `+`) IS the palette being
 * open — derived, so "intent set but picker closed" cannot exist (US-005);
 * dismissing the palette consumes the intent and the empty tab stays put.
 */
export function useDesktopOverlays() {
  const [localOverlay, setLocalOverlay] = useState<LocalDesktopOverlay | null>(null);
  const windowManagerOverlay = useWindowManagerOverlay();
  const paletteIntent = useWindowPaletteIntent();
  const activeOverlay: DesktopOverlay | null =
    windowManagerOverlay?.kind === "desktops-overview"
      ? "desktops"
      : paletteIntent !== null
        ? "palette"
        : localOverlay;

  const setOverlayOpen = (overlay: DesktopOverlay, open: boolean) => {
    if (paletteIntent !== null && (!open || overlay !== "palette")) {
      windowManagerStore.trigger.paletteIntentCleared();
    }
    if (overlay === "desktops") {
      setLocalOverlay(null);
      if (open) {
        windowManagerStore.trigger.overlayOpened({ overlay: { kind: "desktops-overview" } });
      } else {
        windowManagerStore.trigger.overlayClosed();
      }
      return;
    }
    if (open) windowManagerStore.trigger.overlayClosed();
    setLocalOverlay(current => {
      if (open) return overlay;
      // An intent-opened palette may shadow an earlier local overlay; closing
      // it must not reveal that stale surface underneath.
      if (overlay === "palette") return null;
      return current === overlay ? null : current;
    });
  };

  const toggleOverlay = (overlay: DesktopOverlay) => {
    setOverlayOpen(overlay, activeOverlay !== overlay);
  };

  return { activeOverlay, setOverlayOpen, toggleOverlay };
}
