import { type MouseEvent, useEffect, useRef, useState } from "react";

import { type Layout, usePanelRef } from "@compozy/ui";

import { SESSION_SIDEBAR_DEFAULT_WIDTH } from "../lib/session-sidebar-width";
import { useSessionSidebarState } from "./use-session-sidebar-state";

/** Layout key of the rail panel inside its group. */
export const SESSION_SIDEBAR_PANEL_ID = "sessions-rail";

/** Outlasts `--duration-shell-slow` so a toggle that never transitions still settles. */
const TOGGLE_SETTLE_MS = 400;

/** Half the panel library's fine-pointer resize target: where a drag on the handle starts. */
const HANDLE_HIT_SLOP_PX = 5;

/**
 * The rail's width from a committed layout. The library reports percentages
 * of the group minus its handle; reading the panel's own size instead returns
 * the previous width for a keyboard step, which commits before the DOM does.
 */
function railPixels(
  layout: Layout,
  group: HTMLDivElement | null,
  handle: HTMLDivElement | null
): number | null {
  const percent = layout[SESSION_SIDEBAR_PANEL_ID];
  if (percent === undefined || group === null) return null;
  return (percent / 100) * (group.offsetWidth - (handle?.offsetWidth ?? 0));
}

/**
 * Whether a double-click landed on the resize handle. The pane's content can
 * paint over the 1px handle, so the click often targets a neighbor; the
 * pointer counts when it is within the same hit target a drag uses.
 */
function isHandleDoubleClick(handle: HTMLDivElement | null, event: MouseEvent): boolean {
  if (handle === null) return false;
  if (event.target instanceof Node && handle.contains(event.target)) return true;
  const rect = handle.getBoundingClientRect();
  const slop = Math.max(rect.width / 2, HANDLE_HIT_SLOP_PX);
  return Math.abs(event.clientX - (rect.left + rect.width / 2)) <= slop;
}

/**
 * Animate panel growth for one open/close toggle only. Drags, keyboard steps
 * and window resizes must track the pointer and frame, so the transition is
 * scoped to the toggle and dropped once the panels settle.
 */
function animateToggle(group: HTMLDivElement | null): void {
  if (group === null) return;
  group.dataset.toggling = "";
  const settle = (event?: TransitionEvent) => {
    if (event !== undefined && (event.target as Element | null)?.parentElement !== group) return;
    window.clearTimeout(timer);
    group.removeEventListener("transitionend", settle);
    delete group.dataset.toggling;
  };
  const timer = window.setTimeout(settle, TOGGLE_SETTLE_MS);
  group.addEventListener("transitionend", settle);
}

/**
 * Binds one session window's resizable rail to the shared rail preference:
 * the topbar toggle opens and collapses the panel, operator resizes persist
 * the width, dragging past the floor closes the rail, and a double-click on
 * the handle returns to the default width.
 */
export function useSessionSidebarLayout() {
  const { open, setOpen, width, setWidth, resetWidth } = useSessionSidebarState();
  const panelRef = usePanelRef();
  const groupRef = useRef<HTMLDivElement>(null);
  const handleRef = useRef<HTMLDivElement>(null);
  // Mount geometry only: later widths reach the panel through its API, so a
  // drag in another session window never moves this one.
  const [mountSize] = useState(() => (open ? width : 0));

  // The topbar toggle and ⌘B drive `open`; the panel follows.
  useEffect(() => {
    const panel = panelRef.current;
    if (panel === null || panel.isCollapsed() !== open) return;
    animateToggle(groupRef.current);
    if (open) {
      panel.resize(width);
    } else {
      panel.collapse();
    }
  }, [open, width, panelRef]);

  return {
    open,
    panelRef,
    groupRef,
    handleRef,
    mountSize,
    onLayoutChanged: (layout: Layout, meta: { isUserInteraction: boolean }) => {
      const panel = panelRef.current;
      if (!meta.isUserInteraction || panel === null) return;
      // Dragging past the rail's floor or Enter on the handle collapses it.
      if (panel.isCollapsed()) {
        setOpen(false);
        return;
      }
      const nextWidth = railPixels(layout, groupRef.current, handleRef.current);
      if (nextWidth !== null) setWidth(nextWidth);
    },
    onDoubleClick: (event: MouseEvent) => {
      if (!open || !isHandleDoubleClick(handleRef.current, event)) return;
      panelRef.current?.resize(SESSION_SIDEBAR_DEFAULT_WIDTH);
      resetWidth();
    },
  };
}
