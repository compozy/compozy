import { useState } from "react";

import { resetPaletteExecutionEntry } from "../stores/cmd-palette-execution-store";
import { windowManagerStore } from "../stores/window-manager-store";
import { resetPaletteViewStack } from "./use-os-palette-view-stack";
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
    // Every palette open and every dismissal returns to the root, so "reopening
    // starts at root" holds for Esc, a click outside, an action that closes the
    // palette, and another overlay stealing it — without each of those paths
    // having to know the stack exists (Business Rule 32).
    // The argument and confirmation steps reset alongside the view stack: one
    // place decides that a fresh palette starts clean, whatever dismissed it.
    if (overlay === "palette" || open) {
      resetPaletteViewStack();
      resetPaletteExecutionEntry();
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
