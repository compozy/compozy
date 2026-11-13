import { createStoreLogic } from "@xstate/store";

import type { DesktopId, PixelPoint } from "../lib/window-manager-types";
import { DRAG_START_PX } from "./os-tab-drag-store";

/** One desktop card being dragged to a new position in the overview (US-014.AC-3). */
export interface DesktopCardDragState {
  phase: "idle" | "pressed" | "dragging";
  desktopId: DesktopId | null;
  /** The card's position in the desktop order when pressed. */
  from: number;
  /** Where the card would land among the desktops. */
  index: number;
  origin: PixelPoint | null;
  /** Where the pointer holds the card, from the card's top-left corner. */
  grab: PixelPoint;
  /** The floating card's top-left, relative to the grid, and its width. */
  left: number;
  top: number;
  width: number;
}

export type DesktopCardDragEvents = {
  pressed: {
    desktopId: DesktopId;
    from: number;
    origin: PixelPoint;
    grab: PixelPoint;
    width: number;
  };
  /** `grid` is the grid's top-left on screen; `overIndex` the position under the pointer, if any. */
  moved: { point: PixelPoint; grid: PixelPoint; overIndex: number | null };
  released: {};
  /** Escape, a lost pointer, a blurred window or a desktop order that changed under the drag. */
  cancelled: {};
};

export type DesktopCardDragEmitted = {
  /** The one reorder a finished drag sends; a drop where it started sends nothing. */
  reordered: { desktopId: DesktopId; index: number };
};

const IDLE: DesktopCardDragState = {
  phase: "idle",
  desktopId: null,
  from: -1,
  index: -1,
  origin: null,
  grab: { x: 0, y: 0 },
  left: 0,
  top: 0,
  width: 0,
};

/**
 * The card drag's gesture state. A press stays a click until the pointer travels the tab drag's
 * start distance; the release emits one reorder; cancel restores the order and emits nothing.
 * DOM hit testing stays in `useDesktopCardDrag`, which feeds resolved positions in.
 */
export const desktopCardDragLogic = createStoreLogic<
  DesktopCardDragState,
  DesktopCardDragEvents,
  DesktopCardDragEmitted
>({
  context: IDLE,
  on: {
    pressed: (state, event) => {
      if (state.phase !== "idle") return undefined;
      return {
        ...IDLE,
        phase: "pressed",
        desktopId: event.desktopId,
        from: event.from,
        index: event.from,
        origin: event.origin,
        grab: event.grab,
        width: event.width,
      };
    },
    moved: (state, event) => {
      if (state.phase === "idle" || state.origin === null) return undefined;
      const travel = Math.hypot(event.point.x - state.origin.x, event.point.y - state.origin.y);
      if (state.phase === "pressed" && travel < DRAG_START_PX) return undefined;
      return {
        ...state,
        phase: "dragging",
        index: event.overIndex ?? state.index,
        left: event.point.x - event.grid.x - state.grab.x,
        top: event.point.y - event.grid.y - state.grab.y,
      };
    },
    released: (state, _event, enqueue) => {
      if (state.phase === "dragging" && state.desktopId !== null && state.index !== state.from) {
        enqueue.emit.reordered({ desktopId: state.desktopId, index: state.index });
      }
      return IDLE;
    },
    cancelled: state => (state.phase === "idle" ? undefined : IDLE),
  },
});
