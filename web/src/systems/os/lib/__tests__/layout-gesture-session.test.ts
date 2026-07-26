// Suite: pure layout gesture decisions and floating clamp
// Invariant: WM-WEB-005/006 — pointer-up owns the final sample, cancellation never commits,
// stale layouts commit only after unambiguous rebase, and floating commits remain reachable/contained.
// Boundary IN: immutable gesture state machine and floating geometry clamp.
// Boundary OUT: DOM pointer capture wiring and daemon command transport.
import { describe, expect, it } from "vitest";

import {
  beginLayoutGesture,
  cancelLayoutGesture,
  finishLayoutGesture,
  previewLayoutGesture,
  type GestureCancelReason,
} from "../layout-gesture-session";
import { clampFloatingRect } from "../layout-projection";
import type { SnapTarget } from "../snap-targets";

const LEFT_TARGET: SnapTarget = {
  kind: "tile",
  id: "edge:left:0.5",
  edge: "left",
  ratio: 0.5,
  rect: { x: 0, y: 0, w: 500, h: 700 },
};

const RIGHT_TARGET: SnapTarget = {
  kind: "tile",
  id: "edge:right:0.5",
  edge: "right",
  ratio: 0.5,
  rect: { x: 500, y: 0, w: 500, h: 700 },
};

const WORK_AREA = { x: 0, y: 0, w: 1000, h: 700 };

function activeSession() {
  return beginLayoutGesture({
    pointerId: 17,
    point: { x: 320, y: 80 },
    workArea: WORK_AREA,
    layoutRevision: 41,
    source: {
      windowId: "window:dragged",
      nodeId: "leaf:dragged",
      groupId: "group:main",
      moveMode: "window",
    },
  });
}

describe("layout gesture decisions", () => {
  it("Should use the final pointer sample and target instead of the last preview", () => {
    const session = activeSession();
    const previewed = previewLayoutGesture(session, { x: 5, y: 300 }, LEFT_TARGET, WORK_AREA);
    const decision = finishLayoutGesture(previewed, {
      finalPoint: { x: 995, y: 320 },
      finalTarget: RIGHT_TARGET,
      currentRevision: 41,
      currentWorkArea: WORK_AREA,
      clientValid: true,
    });

    expect(decision.kind).toBe("commit");
    if (decision.kind !== "commit") return;
    expect(decision.command.target).toEqual(RIGHT_TARGET);
    expect(decision.command.finalPoint).toEqual({ x: 995, y: 320 });
    expect(session.currentPoint).toEqual({ x: 320, y: 80 });
    expect(session.preview).toBeNull();
  });

  it.each<GestureCancelReason>(["escape", "pointer-cancel", "lost-capture", "manual"])(
    "Should preserve %s cancellation without producing a command",
    reason => {
      const cancelled = cancelLayoutGesture(activeSession(), reason, { x: 400, y: 200 });
      const decision = finishLayoutGesture(cancelled, {
        finalPoint: { x: 995, y: 320 },
        finalTarget: RIGHT_TARGET,
        currentRevision: 41,
        currentWorkArea: WORK_AREA,
        clientValid: true,
      });

      expect(decision).toEqual({
        kind: "cancel",
        reason,
        finalPoint: { x: 400, y: 200 },
        capturedRevision: 41,
        currentRevision: 41,
      });
    }
  );

  it("Should cancel an invalid client and any stale layout", () => {
    const invalidClient = finishLayoutGesture(activeSession(), {
      finalPoint: { x: 995, y: 320 },
      finalTarget: RIGHT_TARGET,
      currentRevision: 41,
      currentWorkArea: WORK_AREA,
      clientValid: false,
    });
    const stale = finishLayoutGesture(activeSession(), {
      finalPoint: { x: 995, y: 320 },
      finalTarget: RIGHT_TARGET,
      currentRevision: 42,
      currentWorkArea: WORK_AREA,
      clientValid: true,
    });

    expect(invalidClient.kind === "cancel" ? invalidClient.reason : null).toBe("invalid-client");
    expect(stale.kind === "cancel" ? stale.reason : null).toBe("stale-layout");
  });

  it("Should cancel when the live work area differs from the pointer-down capture", () => {
    const decision = finishLayoutGesture(activeSession(), {
      finalPoint: { x: 995, y: 320 },
      finalTarget: RIGHT_TARGET,
      currentRevision: 41,
      currentWorkArea: { x: 0, y: 0, w: 900, h: 700 },
      clientValid: true,
    });

    expect(decision).toEqual({
      kind: "cancel",
      reason: "work-area-changed",
      finalPoint: { x: 995, y: 320 },
      capturedRevision: 41,
      currentRevision: 41,
    });
  });

  it("Should own preview target geometry", () => {
    const target: SnapTarget = { ...LEFT_TARGET, rect: { ...LEFT_TARGET.rect } };
    const previewed = previewLayoutGesture(activeSession(), { x: 5, y: 300 }, target, WORK_AREA);

    target.rect.x = 500;

    expect(previewed.status === "active" ? previewed.preview?.rect.x : null).toBe(0);
  });
});

describe("clampFloatingRect", () => {
  it("Should contain an oversized commit and keep its grabbed title-bar point reachable", () => {
    const workArea = { x: 10, y: 20, w: 800, h: 600 };
    const rect = clampFloatingRect({
      proposedRect: { x: -500, y: -400, w: 1200, h: 900 },
      workArea,
      pointer: { x: 805, y: 25 },
      grabOffset: { x: 60, y: 12 },
      titleBarHeight: 32,
    });

    expect(rect).toEqual(workArea);
    expect(rect.x).toBeGreaterThanOrEqual(workArea.x);
    expect(rect.y).toBeGreaterThanOrEqual(workArea.y);
    expect(rect.x + rect.w).toBeLessThanOrEqual(workArea.x + workArea.w);
    expect(rect.y + rect.h).toBeLessThanOrEqual(workArea.y + workArea.h);
  });
});
