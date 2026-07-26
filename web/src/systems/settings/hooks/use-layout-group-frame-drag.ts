import type { KeyboardEvent as ReactKeyboardEvent, RefObject } from "react";

import { LAYOUT_GROUP_MIN_SPAN } from "../lib/window-manager-layout-reference";
import { fractionLabel, nearestEdgeDetent } from "../lib/window-manager-layout-detents";
import type {
  WindowManagerLayoutGroup,
  WindowManagerNormalizedRect,
} from "../lib/window-manager-layout-types";
import { usePointerDrag } from "./use-pointer-drag";

export type LayoutGroupEdge = "top" | "right" | "bottom" | "left";

const KEYBOARD_STEP = 0.01;

export interface LayoutGroupFrameDragModel {
  dragging: boolean;
  /** Where this edge sits across the desktop, 0–1. */
  position: number;
  label: string;
  onPointerDown: (event: React.PointerEvent<HTMLElement>) => void;
  onKeyDown: (event: ReactKeyboardEvent<HTMLElement>) => void;
}

function clamp(value: number, low: number, high: number): number {
  return Math.min(Math.max(value, low), high);
}

/**
 * Moves one edge of a group. The clamps are the daemon's own rect rules — an
 * edge cannot leave the 0–1 desktop and cannot cross its opposite edge — so the
 * only invalid state reachable here is overlapping a neighbour, which the canvas
 * flags rather than silently allowing.
 */
export function edgeToFrame(
  frame: WindowManagerNormalizedRect,
  edge: LayoutGroupEdge,
  position: number
): WindowManagerNormalizedRect {
  if (edge === "left") {
    const next = clamp(position, 0, frame.x + frame.w - LAYOUT_GROUP_MIN_SPAN);
    return { ...frame, x: next, w: frame.x + frame.w - next };
  }
  if (edge === "right") {
    const next = clamp(position, frame.x + LAYOUT_GROUP_MIN_SPAN, 1);
    return { ...frame, w: next - frame.x };
  }
  if (edge === "top") {
    const next = clamp(position, 0, frame.y + frame.h - LAYOUT_GROUP_MIN_SPAN);
    return { ...frame, y: next, h: frame.y + frame.h - next };
  }
  const next = clamp(position, frame.y + LAYOUT_GROUP_MIN_SPAN, 1);
  return { ...frame, h: next - frame.y };
}

export function edgePosition(frame: WindowManagerNormalizedRect, edge: LayoutGroupEdge): number {
  if (edge === "left") return frame.x;
  if (edge === "right") return frame.x + frame.w;
  if (edge === "top") return frame.y;
  return frame.y + frame.h;
}

export function useLayoutGroupFrameDrag(
  group: WindowManagerLayoutGroup,
  edge: LayoutGroupEdge,
  canvasRef: RefObject<HTMLElement | null>,
  onChange: (frame: WindowManagerNormalizedRect) => void
): LayoutGroupFrameDragModel {
  const horizontal = edge === "left" || edge === "right";
  const position = edgePosition(group.frame, edge);

  const drag = usePointerDrag<{ frame: WindowManagerNormalizedRect; canvas: DOMRect }>({
    onStart: () => {
      const canvas = canvasRef.current?.getBoundingClientRect();
      if (!canvas || canvas.width === 0 || canvas.height === 0) return null;
      return { frame: group.frame, canvas };
    },
    onMove: (event, state) => {
      const raw = horizontal
        ? (event.clientX - state.canvas.left) / state.canvas.width
        : (event.clientY - state.canvas.top) / state.canvas.height;
      const detent = nearestEdgeDetent(raw);
      onChange(edgeToFrame(state.frame, edge, detent ?? raw));
    },
  });

  const onKeyDown = (event: ReactKeyboardEvent<HTMLElement>) => {
    const forward = event.key === "ArrowRight" || event.key === "ArrowDown";
    const backward = event.key === "ArrowLeft" || event.key === "ArrowUp";
    if (!forward && !backward) return;
    event.preventDefault();
    event.stopPropagation();
    const step = (forward ? KEYBOARD_STEP : -KEYBOARD_STEP) * (event.shiftKey ? 10 : 1);
    onChange(edgeToFrame(group.frame, edge, position + step));
  };

  return {
    dragging: drag.dragging,
    position,
    label: fractionLabel(position),
    onPointerDown: drag.onPointerDown,
    onKeyDown,
  };
}
