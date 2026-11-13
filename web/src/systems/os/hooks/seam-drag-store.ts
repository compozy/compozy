import { createStoreLogic } from "@xstate/store";

import type { GestureCancelReason } from "../lib/layout-gesture-cancellation";
import type { WindowManagerCommandFence } from "../lib/os-types";
import { seamWeightDelta } from "../lib/seam-preview";
import type { LayoutRevision, ProjectedSeam } from "../lib/window-manager-types";

/** Matches the daemon's no-op band for `layout.resize`. */
export const SEAM_WEIGHT_EPSILON = 0.000001;

/**
 * One seam drag. Everything the commit needs is captured at press: the seam's geometry, the
 * layout revision the drag was authored against, and the context it was drawn in (binding,
 * shown desktop and tab, work area). A drag whose context changes is cancelled, never committed.
 */
export interface SeamDragState {
  phase: "idle" | "dragging";
  pointerId: number | null;
  /** Pointer coordinate on the seam's axis at press. */
  origin: number;
  deltaPx: number;
  seam: ProjectedSeam | null;
  revision: LayoutRevision | null;
  contextKey: string | null;
  /** Why the last drag ended without a commit; null after a commit or before any drag. */
  cancelReason: GestureCancelReason | null;
}

export type SeamDragEvents = {
  dragStarted: {
    pointerId: number;
    coordinate: number;
    seam: ProjectedSeam;
    revision: LayoutRevision;
    contextKey: string;
  };
  pointerMoved: { pointerId: number; coordinate: number };
  dragEnded: { pointerId: number };
  dragCancelled: { reason: GestureCancelReason };
};

function idleSeamDrag(cancelReason: GestureCancelReason | null): SeamDragState {
  return {
    phase: "idle",
    pointerId: null,
    origin: 0,
    deltaPx: 0,
    seam: null,
    revision: null,
    contextKey: null,
    cancelReason,
  };
}

export const seamDragLogic = createStoreLogic<SeamDragState, SeamDragEvents>({
  context: idleSeamDrag(null),
  on: {
    dragStarted: (state, event) => {
      if (state.phase !== "idle") return;
      return {
        phase: "dragging",
        pointerId: event.pointerId,
        origin: event.coordinate,
        deltaPx: 0,
        seam: event.seam,
        revision: event.revision,
        contextKey: event.contextKey,
        cancelReason: null,
      };
    },
    pointerMoved: (state, event) => {
      if (state.phase !== "dragging" || state.pointerId !== event.pointerId) return;
      const deltaPx = event.coordinate - state.origin;
      return deltaPx === state.deltaPx ? undefined : { ...state, deltaPx };
    },
    dragEnded: (state, event) => {
      if (state.phase !== "dragging" || state.pointerId !== event.pointerId) return;
      return idleSeamDrag(null);
    },
    dragCancelled: (state, event) => {
      if (state.phase !== "dragging") return;
      return idleSeamDrag(event.reason);
    },
  },
});

/** A pixel drag pushed past a member's minimum: the seam stops there and says so. */
export function seamDragAtLimit(state: SeamDragState): boolean {
  if (state.phase !== "dragging" || state.seam === null) return false;
  const value = state.seam.value + state.deltaPx;
  return value < state.seam.minValue || value > state.seam.maxValue;
}

export interface SeamDragCommit {
  splitId: string;
  boundaryIndex: number;
  delta: number;
  /** The revision captured at press, guarded by the seam's own split and boundary. */
  fence: WindowManagerCommandFence;
}

/** The one `layout.resize` a finished drag sends, or null when it moved nothing. */
export function seamDragCommit(state: SeamDragState): SeamDragCommit | null {
  const { seam, revision } = state;
  if (state.phase !== "dragging" || seam === null || revision === null) return null;
  const delta = seamWeightDelta(seam, state.deltaPx);
  if (Math.abs(delta) <= SEAM_WEIGHT_EPSILON) return null;
  const { splitId, boundaryIndex } = seam;
  return {
    splitId,
    boundaryIndex,
    delta,
    fence: { expectedRevision: revision, rebase: { splitId, boundaryIndex } },
  };
}

/** Keyboard step in weight space: 2% per arrow, 10% with Shift. */
export function seamKeyStep(event: { shiftKey: boolean }): number {
  return event.shiftKey ? 0.1 : 0.02;
}
