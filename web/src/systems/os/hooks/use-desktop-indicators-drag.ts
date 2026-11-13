import { useSelector } from "@xstate/store-react";

import { osTabDragStore } from "../stores/os-tab-drag-store";
import { useOsTabDragDesktopTarget, useOsTabDragSourceDesktop } from "./use-os-tab-drag";

/**
 * The top-bar indicators' part in a tab drag: the desktop it started on, the desktop it would move
 * to, and what it moves — a whole tab, or one window leaving its split (a window head).
 */
export function useDesktopIndicatorsDrag() {
  const dragSourceDesktopId = useOsTabDragSourceDesktop();
  const dropTargetDesktopId = useOsTabDragDesktopTarget();
  const dragKind = useSelector(osTabDragStore, snapshot =>
    snapshot.context.session?.source.kind === "head" ? "window" : "tab"
  );
  return { dragSourceDesktopId, dropTargetDesktopId, dragKind };
}
