// Suite: shared seam drag lifecycle
// Invariant: one captured pointer owns a seam drag; it commits at most one resize, fenced to the
// revision and split of the seam it is based on, and a cancelled drag commits nothing. A re-based
// drag keeps the seam on screen under the pointer and commits against the new seam only.
// Owning layer: the shared seam drag interaction store. Boundary OUT: pointer transport, the seam
// preview and command I/O (useOsSplitSeam in the OS interaction hooks suite).
import { describe, expect, it } from "vitest";

import type { ProjectedSeam } from "../../lib/window-manager-types";
import { seamDragAtLimit, seamDragCommit, seamDragLogic } from "../seam-drag-store";

const SEAM: ProjectedSeam = {
  id: "split:root:0",
  splitId: "split:root",
  boundaryIndex: 0,
  orientation: "vertical",
  rect: { x: 496, y: 0, w: 8, h: 700 },
  value: 500,
  minValue: 300,
  maxValue: 700,
  axisSpan: 1000,
  leadingWeight: 0.5,
  trailingWeight: 0.5,
  leadingWindowIds: ["window:left"],
  trailingWindowIds: ["window:right"],
};

function startedStore() {
  const store = seamDragLogic.createStore();
  store.trigger.dragStarted({
    pointerId: 7,
    coordinate: 100,
    seam: SEAM,
    revision: 12,
    contextKey: "binding|desktop/tab|area",
  });
  return store;
}

describe("seamDragLogic", () => {
  it("Should follow only the captured pointer", () => {
    const store = startedStore();

    store.trigger.pointerMoved({ pointerId: 7, coordinate: 160 });
    store.trigger.pointerMoved({ pointerId: 8, coordinate: 400 });
    store.trigger.dragEnded({ pointerId: 8 });

    expect(store.getSnapshot().context).toMatchObject({ phase: "dragging", deltaPx: 60 });
    store.trigger.dragEnded({ pointerId: 7 });
    expect(store.getSnapshot().context).toMatchObject({ phase: "idle", seam: null });
  });

  it("Should commit one resize fenced to the press revision and the seam's split", () => {
    const store = startedStore();
    store.trigger.pointerMoved({ pointerId: 7, coordinate: 160 });

    expect(seamDragCommit(store.getSnapshot().context)).toEqual({
      splitId: "split:root",
      boundaryIndex: 0,
      delta: expect.closeTo(0.06),
      fence: { expectedRevision: 12, rebase: { splitId: "split:root", boundaryIndex: 0 } },
    });
  });

  it("Should commit nothing for a drag that moved nothing", () => {
    const store = startedStore();
    store.trigger.pointerMoved({ pointerId: 7, coordinate: 100 });

    expect(seamDragCommit(store.getSnapshot().context)).toBeNull();
  });

  it("Should stop at a member's minimum and report it", () => {
    const store = startedStore();
    store.trigger.pointerMoved({ pointerId: 7, coordinate: -150 });

    expect(seamDragAtLimit(store.getSnapshot().context)).toBe(true);
    // The commit is clamped to the minimum: 200px of travel in a 1000px span.
    expect(seamDragCommit(store.getSnapshot().context)?.delta).toBeCloseTo(-0.2);
  });

  it("Should re-base on a moved seam: the pointer keeps its place and the new bounds clamp", () => {
    const store = startedStore();
    store.trigger.pointerMoved({ pointerId: 7, coordinate: 160 });
    // The split moved to 60/40 under the drag; its trailing minimum now stops the seam at 650.
    const moved: ProjectedSeam = {
      ...SEAM,
      value: 600,
      maxValue: 650,
      leadingWeight: 0.6,
      trailingWeight: 0.4,
    };

    store.trigger.rebased({ seam: moved, revision: 13 });

    // The seam was shown at 500 + 60 = 560: from 600 that is -40, and the pointer still owns it.
    expect(store.getSnapshot().context).toMatchObject({ deltaPx: -40, revision: 13, seam: moved });
    store.trigger.pointerMoved({ pointerId: 7, coordinate: 400 });
    // 500 + 300 = 800 on screen is past 650: the commit stops where the new bounds do.
    expect(seamDragAtLimit(store.getSnapshot().context)).toBe(true);
    expect(seamDragCommit(store.getSnapshot().context)).toEqual({
      splitId: "split:root",
      boundaryIndex: 0,
      delta: expect.closeTo(0.05),
      fence: { expectedRevision: 13, rebase: { splitId: "split:root", boundaryIndex: 0 } },
    });
  });

  it.each(["escape", "context-changed"] as const)(
    "Should end a drag cancelled by %s with nothing to commit",
    reason => {
      const store = startedStore();
      store.trigger.pointerMoved({ pointerId: 7, coordinate: 160 });

      store.trigger.dragCancelled({ reason });

      expect(store.getSnapshot().context).toMatchObject({
        phase: "idle",
        cancelReason: reason,
      });
      expect(seamDragCommit(store.getSnapshot().context)).toBeNull();
      store.trigger.dragEnded({ pointerId: 7 });
      expect(store.getSnapshot().context.phase).toBe("idle");
    }
  );
});
