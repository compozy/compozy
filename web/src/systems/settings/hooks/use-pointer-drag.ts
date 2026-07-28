import { useRef, useState, type PointerEvent as ReactPointerEvent } from "react";

export interface PointerDragSpec<TState> {
  /** Captured once at pointerdown; return null to refuse the gesture. */
  onStart: (event: ReactPointerEvent<HTMLElement>) => TState | null;
  onMove: (event: PointerEvent, state: TState) => void;
  onEnd?: (state: TState) => void;
}

export interface PointerDragModel {
  dragging: boolean;
  onPointerDown: (event: ReactPointerEvent<HTMLElement>) => void;
}

/**
 * One pointer-capture gesture for every draggable affordance on this page. The
 * element keeps the pointer for the whole drag, so leaving the canvas — or
 * releasing over another window — still ends the gesture cleanly instead of
 * stranding a half-moved divider.
 */
export function usePointerDrag<TState>(spec: PointerDragSpec<TState>): PointerDragModel {
  const [dragging, setDragging] = useState(false);
  const specRef = useRef(spec);
  specRef.current = spec;

  const onPointerDown = (event: ReactPointerEvent<HTMLElement>) => {
    if (event.button !== 0) return;
    const state = specRef.current.onStart(event);
    if (state === null) return;
    event.preventDefault();
    event.stopPropagation();

    const target = event.currentTarget;
    target.setPointerCapture(event.pointerId);
    setDragging(true);

    const handleMove = (moveEvent: PointerEvent) => {
      specRef.current.onMove(moveEvent, state);
    };
    const finish = () => {
      target.removeEventListener("pointermove", handleMove);
      target.removeEventListener("pointerup", finish);
      target.removeEventListener("pointercancel", finish);
      target.removeEventListener("lostpointercapture", finish);
      setDragging(false);
      specRef.current.onEnd?.(state);
    };

    target.addEventListener("pointermove", handleMove);
    target.addEventListener("pointerup", finish);
    target.addEventListener("pointercancel", finish);
    target.addEventListener("lostpointercapture", finish);
  };

  return { dragging, onPointerDown };
}

/**
 * The keyboard half of a drag affordance. Every handle is also a `role="slider"`,
 * so the layout is editable without a pointer at all.
 */
export function arrowKeyStep(
  event: { key: string; shiftKey: boolean },
  options: { coarse?: number; fine?: number } = {}
): number | null {
  const fine = options.fine ?? 1;
  const coarse = options.coarse ?? fine * 10;
  const magnitude = event.shiftKey ? coarse : fine;
  if (event.key === "ArrowRight" || event.key === "ArrowUp") return magnitude;
  if (event.key === "ArrowLeft" || event.key === "ArrowDown") return -magnitude;
  return null;
}
