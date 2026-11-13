import { useState, type KeyboardEvent as ReactKeyboardEvent } from "react";

import { fractionLabel, nearestDetent } from "../lib/window-manager-layout-detents";
import { moveLayoutSeam } from "../lib/window-manager-layout-tree";
import type {
  WindowManagerLayoutNode,
  WindowManagerLayoutSplit,
} from "../lib/window-manager-layout-types";
import { usePointerDrag } from "./use-pointer-drag";

/** Matches the live shell's keyboard step so both surfaces nudge by the same amount. */
const KEYBOARD_STEP = 0.02;

interface SeamDragState {
  root: WindowManagerLayoutNode;
  /** Where the leading member starts and how long the pair is, along the split axis. */
  start: number;
  span: number;
}

export interface LayoutDraftSeamModel {
  dragging: boolean;
  /** Leading share of the pair, 0–1. */
  ratio: number;
  leadingLabel: string;
  trailingLabel: string;
  snapped: boolean;
  onPointerDown: (event: React.PointerEvent<HTMLElement>) => void;
  onKeyDown: (event: ReactKeyboardEvent<HTMLElement>) => void;
  onDoubleClick: () => void;
}

/**
 * Drags one seam of a split on the *draft*, not the running workspace. Only
 * the two members beside the seam change, and `moveLayoutSeam` keeps the
 * weights positive and summing to 1, so the drag cannot produce a weight vector
 * the daemon rejects. The seam's neighbours in the DOM are the two members, so
 * the pair is measured where it is drawn.
 */
export function useLayoutDraftSeam(
  root: WindowManagerLayoutNode,
  split: WindowManagerLayoutSplit,
  index: number,
  onChange: (next: WindowManagerLayoutNode) => void
): LayoutDraftSeamModel {
  const [snapped, setSnapped] = useState(false);
  const leading = split.weights[index] ?? 0;
  const trailing = split.weights[index + 1] ?? 0;
  const ratio = leading + trailing === 0 ? 0.5 : leading / (leading + trailing);
  const row = split.axis === "horizontal";

  const drag = usePointerDrag<SeamDragState>({
    onStart: event => {
      const before = event.currentTarget.previousElementSibling?.getBoundingClientRect();
      const after = event.currentTarget.nextElementSibling?.getBoundingClientRect();
      if (!before || !after) return null;
      const span = row ? after.right - before.left : after.bottom - before.top;
      if (span <= 0) return null;
      return { root, start: row ? before.left : before.top, span };
    },
    onMove: (event, state) => {
      const pointer = row ? event.clientX : event.clientY;
      const share = (pointer - state.start) / state.span;
      const detent = nearestDetent(share);
      setSnapped(detent !== null);
      onChange(moveLayoutSeam(state.root, split.id, index, detent ?? share));
    },
    onEnd: () => setSnapped(false),
  });

  const onKeyDown = (event: ReactKeyboardEvent<HTMLElement>) => {
    const forward = event.key === "ArrowRight" || event.key === "ArrowDown";
    const backward = event.key === "ArrowLeft" || event.key === "ArrowUp";
    if (!forward && !backward) return;
    event.preventDefault();
    event.stopPropagation();
    onChange(moveLayoutSeam(root, split.id, index, ratio + (forward ? 1 : -1) * KEYBOARD_STEP));
  };

  return {
    dragging: drag.dragging,
    ratio,
    leadingLabel: fractionLabel(ratio),
    trailingLabel: fractionLabel(1 - ratio),
    snapped: drag.dragging && snapped,
    onPointerDown: drag.onPointerDown,
    onKeyDown,
    onDoubleClick: () => onChange(moveLayoutSeam(root, split.id, index, 0.5)),
  };
}
