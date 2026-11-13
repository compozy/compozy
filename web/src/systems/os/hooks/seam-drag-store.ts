import { createStoreLogic } from "@xstate/store";

import type { GestureCancelReason } from "../lib/layout-gesture-cancellation";
import type { WindowManagerCommandFence } from "../lib/os-types";
import { seamWeightDelta } from "../lib/seam-preview";
import type { LayoutRevision, ProjectedSeam } from "../lib/window-manager-types";

/** Matches the daemon's no-op band for `layout.resize`. */
export const SEAM_WEIGHT_EPSILON = 0.000001;

/**
 * The shift a drag frame requesting `requestedPx` pixels shows: held to the seam's bounds and to
 * the shifts its split still lays out side by side at (`admitSeamShift`), read from the layout the
 * drag is based on when the frame is made.
 */
export type SeamShiftAdmission = (requestedPx: number) => number;

/**
 * One seam drag: the seam's geometry and layout revision it is currently based on, and the
 * context it was drawn in (binding, shown desktop and tab, work area). The press bases it on the
 * authoritative seam; every change of that seam's layout under the drag either re-bases the whole
 * gesture (`rebased`, so preview, limit and commit always share one seam) or cancels it, and the
 * release re-checks the latest layout once more before it commits. A drag whose context changes
 * is cancelled, never committed.
 */
export interface SeamDragState {
  phase: "idle" | "dragging";
  pointerId: number | null;
  /** The pointer coordinate on the seam's axis that leaves `seam` where it is laid out. */
  origin: number;
  /** The shift the pointer asks for. */
  deltaPx: number;
  /**
   * The shift the drag shows, reports at its limit and commits: `deltaPx` as the admission the
   * last pointer move, re-base or re-admission carried lets it be. 0 shows the seam where it is.
   */
  shownPx: number;
  /**
   * Whether the frame on screen is at the seam's limit (`seamDragAtLimit`). A pointer move only
   * records the shift; the frame that shows it is presented on the next animation frame, and the
   * limit state changes with it, never a frame before the seam reaches the clamp.
   */
  limitShown: boolean;
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
  /**
   * The seam's layout changed under the drag: `seam` is its latest unpreviewed layout at `revision`. The
   * pointer is re-anchored so the seam on screen stays under it.
   */
  rebased: { seam: ProjectedSeam; revision: LayoutRevision; admit: SeamShiftAdmission };
  pointerMoved: { pointerId: number; coordinate: number; admit: SeamShiftAdmission };
  /**
   * The layout under the drag changed without moving its seam: the shift is admitted again, in
   * the frame that publishes that layout.
   */
  readmitted: { admit: SeamShiftAdmission };
  /** The drag's current shift is now on screen: the limit state follows it. */
  framePresented: {};
  dragEnded: { pointerId: number };
  dragCancelled: { reason: GestureCancelReason };
};

function idleSeamDrag(cancelReason: GestureCancelReason | null): SeamDragState {
  return {
    phase: "idle",
    pointerId: null,
    origin: 0,
    deltaPx: 0,
    shownPx: 0,
    limitShown: false,
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
        shownPx: 0,
        limitShown: false,
        seam: event.seam,
        revision: event.revision,
        contextKey: event.contextKey,
        cancelReason: null,
      };
    },
    rebased: (state, event) => {
      if (state.phase !== "dragging" || state.seam === null) return;
      const shift = event.seam.value - state.seam.value;
      const deltaPx = state.deltaPx - shift;
      return {
        ...state,
        origin: state.origin + shift,
        deltaPx,
        shownPx: event.admit(deltaPx),
        seam: event.seam,
        revision: event.revision,
      };
    },
    pointerMoved: (state, event) => {
      if (state.phase !== "dragging" || state.pointerId !== event.pointerId) return;
      const deltaPx = event.coordinate - state.origin;
      const shownPx = event.admit(deltaPx);
      if (deltaPx === state.deltaPx && shownPx === state.shownPx) return;
      return { ...state, deltaPx, shownPx };
    },
    readmitted: (state, event) => {
      if (state.phase !== "dragging") return;
      const shownPx = event.admit(state.deltaPx);
      const limitShown = atLimit(state.deltaPx, shownPx);
      if (shownPx === state.shownPx && limitShown === state.limitShown) return;
      return { ...state, shownPx, limitShown };
    },
    framePresented: state => {
      if (state.phase !== "dragging") return;
      const limitShown = seamDragAtLimit(state);
      return limitShown === state.limitShown ? undefined : { ...state, limitShown };
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

/**
 * How far a frame must be held back from the pointer before the seam reports its limit. A frame
 * held back by less only drops a fraction of a pixel (a high-density pointer on a split whose
 * shares tie), which is not the seam refusing to follow; it still shows and commits that frame.
 */
export const SEAM_LIMIT_HOLDBACK_PX = 1;

/**
 * A pixel drag pushed past where its split still lays out side by side: the seam stops at the
 * admitted shift and says so once that holds it back by `SEAM_LIMIT_HOLDBACK_PX` or more.
 */
export function seamDragAtLimit(state: SeamDragState): boolean {
  if (state.phase !== "dragging" || state.seam === null) return false;
  return atLimit(state.deltaPx, state.shownPx);
}

function atLimit(deltaPx: number, shownPx: number): boolean {
  return Math.abs(deltaPx - shownPx) >= SEAM_LIMIT_HOLDBACK_PX;
}

export interface SeamDragCommit {
  splitId: string;
  boundaryIndex: number;
  delta: number;
  /** The revision the drag is based on when it commits, guarded by the seam's split and boundary. */
  fence: WindowManagerCommandFence;
}

/** The one `layout.resize` a finished drag sends — the shift it shows — or null for none. */
export function seamDragCommit(state: SeamDragState): SeamDragCommit | null {
  const { seam, revision } = state;
  if (state.phase !== "dragging" || seam === null || revision === null) return null;
  const delta = seamWeightDelta(seam, state.shownPx);
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
