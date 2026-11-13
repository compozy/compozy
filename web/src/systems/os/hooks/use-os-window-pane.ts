import { useDeferredValue, useState, type MouseEvent, type PointerEvent } from "react";

import { useDesktop } from "./use-desktop";
import { useOsTabDragStart, useOsTabPreviewed, useOsWindowDragged } from "./use-os-tab-drag";
import { useOsWindowMember } from "./use-os-window-member";
import { useDeskBoundToActingProfile } from "./use-window-manager-store";

/** Everything one window pane renders from: its member handles, drag state, and visibility. */
export function useOsWindowPane(windowId: string) {
  const member = useOsWindowMember(windowId);
  const { coordinator, win } = member;
  const [overlayHost, setOverlayHost] = useState<HTMLDivElement | null>(null);
  // Mount layout-heavy bodies, including app skeletons, after the pane commits.
  const contentHost = useDeferredValue(overlayHost);
  const deskBound = useDeskBoundToActingProfile();
  const { startHeadDrag } = useOsTabDragStart();
  const dragged = useOsWindowDragged(windowId);
  // A drag previews another tab's content without activating it: its windows show too.
  const previewed = useOsTabPreviewed(win?.tabId ?? null);
  const projectedShown = useDesktop(
    state =>
      state.projections[win?.tabId ?? ""]?.windows.find(entry => entry.windowId === windowId)
        ?.shown ?? true
  );
  const visible = win ? win.shown || (previewed && projectedShown) : false;

  // Navigation intent is classified at the click, not inferred from paths
  // (ADR-011): a link inside the body drills in (`push`); a breadcrumb selects
  // a parent, and the coordinator checks whether it matches the tab's stack.
  const classifyNavigation = (event: MouseEvent<HTMLElement>) => {
    const element = event.target instanceof Element ? event.target : null;
    if (!element) return;
    if (element.closest('[data-slot="topbar-back"], [data-slot="topbar-crumb"]')) {
      coordinator.noteNavigateMode("pop");
      return;
    }
    if (element.closest("a[href]") && element.closest('[data-slot="os-window-body"]')) {
      coordinator.noteNavigateMode("push");
    }
  };

  return {
    member,
    contentHost,
    setOverlayHost,
    deskBound,
    dragged,
    visible,
    classifyNavigation,
    startHeadDrag: (event: PointerEvent<HTMLElement>) => startHeadDrag(windowId, event),
  };
}
