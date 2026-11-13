import { useSelector } from "@xstate/store-react";

import { osTabDragStore } from "../stores/os-tab-drag-store";
import { useOsTabDragDesktopTarget, useOsTabDragSourceDesktop } from "./use-os-tab-drag";

/** What a tab drag moves: a whole tab, or one window leaving its split (a window head). */
export function useOsTabDragKind(): "tab" | "window" {
  return useSelector(osTabDragStore, (snapshot): "tab" | "window" =>
    snapshot.context.session?.source.kind === "head" ? "window" : "tab"
  );
}

/**
 * The top-bar indicators' part in a tab drag: the desktop it started on, the desktop it would move
 * to, and what it moves.
 */
export function useDesktopIndicatorsDrag() {
  const dragSourceDesktopId = useOsTabDragSourceDesktop();
  const dropTargetDesktopId = useOsTabDragDesktopTarget();
  const dragKind = useOsTabDragKind();
  return { dragSourceDesktopId, dropTargetDesktopId, dragKind };
}
