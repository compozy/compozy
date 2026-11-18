import { useEffect, useEffectEvent, useState } from "react";

import {
  DESKTOP_OVERFLOW_ATTRIBUTE,
  type DesktopOverflowDirection,
} from "../lib/desktop-indicator-controls";
import type { PixelPoint } from "../lib/window-manager-types";
import { osTabDragStore } from "../stores/os-tab-drag-store";

/** The overflow control of `container` under `point`, if any. */
export function overflowDirectionAt(
  container: ParentNode,
  point: PixelPoint
): DesktopOverflowDirection | null {
  const controls = container.querySelectorAll<HTMLElement>(`[${DESKTOP_OVERFLOW_ATTRIBUTE}]`);
  for (const control of controls) {
    const rect = control.getBoundingClientRect();
    const inside =
      point.x >= rect.left &&
      point.x <= rect.right &&
      point.y >= rect.top &&
      point.y <= rect.bottom;
    const direction = control.getAttribute(DESKTOP_OVERFLOW_ATTRIBUTE);
    if (inside && (direction === "earlier" || direction === "later")) return direction;
  }
  return null;
}

/**
 * The overflow control a dragged tab rests on (US-016.EC-2). The dragged tab holds pointer capture,
 * so the controls never see the pointer themselves: the drag's own pointer is hit-tested against
 * them. Entering one calls `onEnter` once, which opens the overview as a drop surface.
 */
export function useDesktopOverflowDragHover(
  container: HTMLElement | null,
  onEnter: (direction: DesktopOverflowDirection) => void
): DesktopOverflowDirection | null {
  const [hovered, setHovered] = useState<DesktopOverflowDirection | null>(null);
  const enter = useEffectEvent(onEnter);

  useEffect(() => {
    if (container === null) return;
    let current: DesktopOverflowDirection | null = null;
    const subscription = osTabDragStore.subscribe(snapshot => {
      const { phase, pointer } = snapshot.context;
      const next =
        phase === "dragging" && pointer !== null ? overflowDirectionAt(container, pointer) : null;
      if (next === current) return;
      current = next;
      setHovered(next);
      if (next !== null) enter(next);
    });
    return () => subscription.unsubscribe();
  }, [container]);

  return hovered;
}
