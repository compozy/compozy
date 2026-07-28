import type { KeyboardEvent as ReactKeyboardEvent } from "react";

import { usePointerDrag } from "./use-pointer-drag";

export interface DragValueSpec {
  value: number;
  min: number;
  max: number;
  axis: "x" | "y";
  /** Pointer pixels → value units. 1 means the guide tracks the pointer exactly. */
  scale?: () => number;
  /** Guides on the far side of a box grow as the pointer moves toward the centre. */
  invert?: boolean;
  onChange: (next: number) => void;
}

export interface DragValueModel {
  dragging: boolean;
  onPointerDown: (event: React.PointerEvent<HTMLElement>) => void;
  onKeyDown: (event: ReactKeyboardEvent<HTMLElement>) => void;
}

function clampRound(value: number, min: number, max: number): number {
  return Math.min(Math.max(Math.round(value), min), max);
}

/**
 * One integer pixel value, dragged where it lives and stepped with the arrow
 * keys. Every guide on the spacing and snapping cards uses this, so the two
 * input paths cannot drift apart — and no geometry value on the page needs a
 * number field to be reachable.
 */
export function useDragValue(spec: DragValueSpec): DragValueModel {
  const drag = usePointerDrag<{ start: number; origin: number; scale: number }>({
    onStart: event => ({
      start: spec.value,
      origin: spec.axis === "x" ? event.clientX : event.clientY,
      scale: spec.scale?.() ?? 1,
    }),
    onMove: (event, state) => {
      const pointer = spec.axis === "x" ? event.clientX : event.clientY;
      const travel = (pointer - state.origin) * state.scale * (spec.invert ? -1 : 1);
      spec.onChange(clampRound(state.start + travel, spec.min, spec.max));
    },
  });

  const onKeyDown = (event: ReactKeyboardEvent<HTMLElement>) => {
    const forward = event.key === "ArrowRight" || event.key === "ArrowUp";
    const backward = event.key === "ArrowLeft" || event.key === "ArrowDown";
    if (!forward && !backward) return;
    event.preventDefault();
    const step = (forward ? 1 : -1) * (event.shiftKey ? 10 : 1);
    spec.onChange(clampRound(spec.value + step, spec.min, spec.max));
  };

  return { dragging: drag.dragging, onPointerDown: drag.onPointerDown, onKeyDown };
}

/** The aria contract every guide shares. */
export function dragValueAria(label: string, value: number, min: number, max: number) {
  return {
    "aria-label": `${label}, ${value} pixels`,
    "aria-valuemax": max,
    "aria-valuemin": min,
    "aria-valuenow": value,
    "aria-valuetext": `${value} pixels`,
    role: "slider" as const,
    tabIndex: 0,
  };
}
