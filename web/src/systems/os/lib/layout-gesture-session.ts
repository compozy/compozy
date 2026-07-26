import type { SnapTarget } from "./snap-targets";
import type {
  GroupId,
  LayoutNodeId,
  LayoutRevision,
  PixelPoint,
  PixelRect,
  WindowId,
} from "./window-manager-types";

export type GestureCancelReason =
  | "escape"
  | "pointer-cancel"
  | "lost-capture"
  | "work-area-changed"
  | "invalid-client"
  | "stale-layout"
  | "no-target"
  | "manual";

export interface GestureSource {
  windowId: WindowId;
  nodeId: LayoutNodeId | null;
  groupId: GroupId | null;
  moveMode: "window" | "group";
}

export interface BeginLayoutGestureInput {
  pointerId: number;
  point: PixelPoint;
  workArea: PixelRect;
  layoutRevision: LayoutRevision;
  source: GestureSource;
}

export interface ActiveLayoutGesture {
  status: "active";
  pointerId: number;
  startPoint: PixelPoint;
  currentPoint: PixelPoint;
  workArea: PixelRect;
  layoutRevision: LayoutRevision;
  source: GestureSource;
  preview: SnapTarget | null;
}

export interface CancelledLayoutGesture {
  status: "cancelled";
  pointerId: number;
  startPoint: PixelPoint;
  finalPoint: PixelPoint;
  workArea: PixelRect;
  layoutRevision: LayoutRevision;
  source: GestureSource;
  reason: GestureCancelReason;
}

export type LayoutGestureSession = ActiveLayoutGesture | CancelledLayoutGesture;

export interface FinishLayoutGestureInput {
  finalPoint: PixelPoint;
  finalTarget: SnapTarget | null;
  currentRevision: LayoutRevision;
  currentWorkArea: PixelRect;
  clientValid: boolean;
}

export interface GestureCommandCandidate {
  source: GestureSource;
  target: SnapTarget;
  expectedRevision: LayoutRevision;
  finalPoint: PixelPoint;
}

export type GestureDecision =
  | {
      kind: "commit";
      command: GestureCommandCandidate;
    }
  | {
      kind: "cancel";
      reason: GestureCancelReason;
      finalPoint: PixelPoint;
      capturedRevision: LayoutRevision;
      currentRevision: LayoutRevision;
    };

function copyPoint(point: PixelPoint): PixelPoint {
  return { x: point.x, y: point.y };
}

function copyRect(rect: PixelRect): PixelRect {
  return { x: rect.x, y: rect.y, w: rect.w, h: rect.h };
}

function copySnapTarget(target: SnapTarget): SnapTarget {
  return { ...target, rect: copyRect(target.rect) };
}

function sameRect(left: PixelRect, right: PixelRect): boolean {
  return left.x === right.x && left.y === right.y && left.w === right.w && left.h === right.h;
}

function cancelDecision(
  reason: GestureCancelReason,
  finalPoint: PixelPoint,
  capturedRevision: LayoutRevision,
  currentRevision: LayoutRevision
): GestureDecision {
  return {
    kind: "cancel",
    reason,
    finalPoint: copyPoint(finalPoint),
    capturedRevision,
    currentRevision,
  };
}

/** Captures all geometry and revision inputs at pointer-down. */
export function beginLayoutGesture(input: BeginLayoutGestureInput): ActiveLayoutGesture {
  return {
    status: "active",
    pointerId: input.pointerId,
    startPoint: copyPoint(input.point),
    currentPoint: copyPoint(input.point),
    workArea: copyRect(input.workArea),
    layoutRevision: input.layoutRevision,
    source: { ...input.source },
    preview: null,
  };
}

/** Returns an ephemeral preview state; it has no persistence callback or durable side effect. */
export function previewLayoutGesture(
  session: LayoutGestureSession,
  point: PixelPoint,
  preview: SnapTarget | null,
  currentWorkArea: PixelRect
): LayoutGestureSession {
  if (session.status === "cancelled") return session;
  if (!sameRect(session.workArea, currentWorkArea)) {
    return cancelLayoutGesture(session, "work-area-changed", point);
  }
  return {
    ...session,
    currentPoint: copyPoint(point),
    preview: preview === null ? null : copySnapTarget(preview),
  };
}

export function cancelLayoutGesture(
  session: LayoutGestureSession,
  reason: GestureCancelReason,
  point?: PixelPoint
): CancelledLayoutGesture {
  if (session.status === "cancelled") return session;
  return {
    status: "cancelled",
    pointerId: session.pointerId,
    startPoint: session.startPoint,
    finalPoint: copyPoint(point ?? session.currentPoint),
    workArea: session.workArea,
    layoutRevision: session.layoutRevision,
    source: session.source,
    reason,
  };
}

/**
 * Produces at most one semantic command candidate. The pointer-up sample and
 * target always replace the last preview. A revision change cancels the gesture;
 * the shipped pointer path does not carry an authoritative rebase proof.
 */
export function finishLayoutGesture(
  session: LayoutGestureSession,
  input: FinishLayoutGestureInput
): GestureDecision {
  if (session.status === "cancelled") {
    return cancelDecision(
      session.reason,
      session.finalPoint,
      session.layoutRevision,
      input.currentRevision
    );
  }
  if (!sameRect(session.workArea, input.currentWorkArea)) {
    return cancelDecision(
      "work-area-changed",
      input.finalPoint,
      session.layoutRevision,
      input.currentRevision
    );
  }
  if (!input.clientValid) {
    return cancelDecision(
      "invalid-client",
      input.finalPoint,
      session.layoutRevision,
      input.currentRevision
    );
  }
  if (input.currentRevision === session.layoutRevision) {
    if (input.finalTarget === null) {
      return cancelDecision(
        "no-target",
        input.finalPoint,
        session.layoutRevision,
        input.currentRevision
      );
    }
    return {
      kind: "commit",
      command: {
        source: session.source,
        target: copySnapTarget(input.finalTarget),
        expectedRevision: session.layoutRevision,
        finalPoint: copyPoint(input.finalPoint),
      },
    };
  }
  return cancelDecision(
    "stale-layout",
    input.finalPoint,
    session.layoutRevision,
    input.currentRevision
  );
}
