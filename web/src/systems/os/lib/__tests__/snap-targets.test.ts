// Suite: pure progressive snap-target resolution
// Invariant: WM-WEB-004 — targets are contained in the captured work area; overshoot clamps
// to the nearest edge and keeps it armed; sides/corners, progressive ratios, structural
// occupied drops, swap-modifier drops, zoom, Dock reservation, and hysteresis resolve once.
// Boundary IN: pointer + captured work area + structural candidates + swap modifier state.
// Boundary OUT: gesture lifecycle and daemon command execution.
import { describe, expect, it } from "vitest";

import {
  createTileSnapTarget,
  DEFAULT_SNAP_TARGET_CONFIG,
  resolveSnapTarget,
  snapTargetIsContained,
  type OccupiedSnapCandidate,
  type ResolveSnapTargetInput,
} from "../snap-targets";

const AREA = { x: 10, y: 20, w: 1200, h: 800 };

const CANDIDATE: OccupiedSnapCandidate = {
  windowId: "window:target",
  nodeId: "leaf:target",
  rect: { x: 300, y: 220, w: 400, h: 300 },
  z: 4,
  parentAxis: "horizontal",
};

function resolve(input: Omit<ResolveSnapTargetInput, "config">) {
  return resolveSnapTarget({ ...input, config: DEFAULT_SNAP_TARGET_CONFIG });
}

describe("resolveSnapTarget", () => {
  it.each([
    { name: "left", point: { x: AREA.x - 40, y: 400 }, edge: "left" },
    { name: "right", point: { x: AREA.x + AREA.w + 40, y: 400 }, edge: "right" },
  ])("Should keep the $name edge armed when the pointer overshoots it", ({ point, edge }) => {
    const target = resolve({ point, workArea: AREA });
    expect(target).toMatchObject({ kind: "tile", edge });
  });

  it("Should resolve zoom when the pointer overshoots the top-center band", () => {
    expect(resolve({ point: { x: 600, y: AREA.y - 40 }, workArea: AREA })).toMatchObject({
      kind: "zoom",
    });
  });

  it("Should keep the reserved bottom-center approach strip above the Dock blocked on overshoot", () => {
    expect(resolve({ point: { x: 600, y: AREA.y + AREA.h + 40 }, workArea: AREA })).toBeNull();
  });

  it("Should resolve a whole-window swap over a candidate while the modifier is held", () => {
    const target = resolve({
      point: { x: 500, y: 370 },
      workArea: AREA,
      candidates: [CANDIDATE],
      swapModifierActive: true,
    });
    expect(target).toEqual({
      kind: "swap",
      id: "swap:leaf:target",
      targetWindowId: "window:target",
      targetNodeId: "leaf:target",
      rect: { x: 300, y: 220, w: 400, h: 300 },
    });
  });

  it("Should keep structural occupied drops when the swap modifier is released", () => {
    const target = resolve({
      point: { x: 500, y: 370 },
      workArea: AREA,
      candidates: [CANDIDATE],
      swapModifierActive: false,
    });
    expect(target?.kind).toBe("stack");
  });

  it.each([
    { cycleStep: 0, ratio: 0.5, width: 600 },
    { cycleStep: 1, ratio: 2 / 3, width: 800 },
    { cycleStep: 2, ratio: 1 / 3, width: 400 },
    { cycleStep: 3, ratio: 0.5, width: 600 },
  ])("Should cycle a side through ratio $ratio", ({ cycleStep, ratio, width }) => {
    const target = resolve({
      point: { x: AREA.x + 1, y: 500 },
      workArea: AREA,
      cycleStep,
    });

    expect(target?.kind).toBe("tile");
    if (target?.kind !== "tile") return;
    expect(target.edge).toBe("left");
    expect(target.ratio).toBeCloseTo(ratio, 5);
    expect(target.rect).toEqual({ x: 10, y: 20, w: width, h: 800 });
  });

  it.each([
    { point: { x: 11, y: 21 }, edge: "top-left", rect: { x: 10, y: 20, w: 600, h: 400 } },
    { point: { x: 1209, y: 21 }, edge: "top-right", rect: { x: 610, y: 20, w: 600, h: 400 } },
    { point: { x: 11, y: 819 }, edge: "bottom-left", rect: { x: 10, y: 420, w: 600, h: 400 } },
    { point: { x: 1209, y: 819 }, edge: "bottom-right", rect: { x: 610, y: 420, w: 600, h: 400 } },
  ])("Should resolve the $edge quarter from either adjacent band", ({ point, edge, rect }) => {
    const target = resolve({ point, workArea: AREA });

    expect(target?.kind).toBe("tile");
    if (target?.kind !== "tile") return;
    expect(target.edge).toBe(edge);
    expect(target.rect).toEqual(rect);
    expect(snapTargetIsContained(target, AREA)).toBe(true);
  });

  it("Should partition odd work areas on one shared pixel edge", () => {
    const oddArea = { x: 10, y: 20, w: 1201, h: 801 };
    const left = createTileSnapTarget(oddArea, "left", [0.5]);
    const right = createTileSnapTarget(oddArea, "right", [0.5]);
    const top = createTileSnapTarget(oddArea, "top", [0.5]);
    const bottom = createTileSnapTarget(oddArea, "bottom", [0.5]);

    expect(left.rect.x + left.rect.w).toBe(right.rect.x);
    expect(top.rect.y + top.rect.h).toBe(bottom.rect.y);
  });

  it("Should block the reserved bottom-center approach strip above the Dock and map top-center to zoom", () => {
    const zoom = resolve({ point: { x: 600, y: 21 }, workArea: AREA });
    const dock = resolve({ point: { x: 600, y: 819 }, workArea: AREA });

    expect(zoom).toEqual({ kind: "zoom", id: "zoom", rect: AREA });
    expect(dock).toBeNull();
  });

  it("Should retain an active side through exit slack and release immediately after", () => {
    const currentTarget = resolve({ point: { x: 11, y: 500 }, workArea: AREA });
    const boundary =
      AREA.x + DEFAULT_SNAP_TARGET_CONFIG.edgeBand + DEFAULT_SNAP_TARGET_CONFIG.exitSlack;

    expect(resolve({ point: { x: boundary, y: 500 }, workArea: AREA, currentTarget })).toEqual(
      currentTarget
    );
    expect(
      resolve({ point: { x: boundary + 1, y: 500 }, workArea: AREA, currentTarget })
    ).toBeNull();
  });

  it("Should resolve before insertion, orthogonal side split, and center stack", () => {
    const before = resolve({
      point: { x: CANDIDATE.rect.x + 1, y: 370 },
      workArea: AREA,
      candidates: [CANDIDATE],
    });
    const side = resolve({
      point: { x: 500, y: CANDIDATE.rect.y + 1 },
      workArea: AREA,
      candidates: [CANDIDATE],
    });
    const stack = resolve({
      point: { x: 500, y: 370 },
      workArea: AREA,
      candidates: [CANDIDATE],
    });

    expect(before?.kind).toBe("insert");
    if (before?.kind === "insert") expect(before.relation).toBe("before");
    expect(side?.kind).toBe("split");
    if (side?.kind === "split") expect(side.side).toBe("top");
    expect(stack?.kind).toBe("stack");
    for (const target of [before, side, stack]) {
      expect(target === null ? false : snapTargetIsContained(target, AREA)).toBe(true);
    }
  });

  it("Should consume live repeat ratios instead of a browser-owned cycle", () => {
    const first = resolveSnapTarget({
      point: { x: 11, y: 500 },
      workArea: AREA,
      cycleStep: 0,
      config: { ...DEFAULT_SNAP_TARGET_CONFIG, repeatRatios: [0.75, 0.25] },
    });
    const second = resolveSnapTarget({
      point: { x: 11, y: 500 },
      workArea: AREA,
      cycleStep: 1,
      config: { ...DEFAULT_SNAP_TARGET_CONFIG, repeatRatios: [0.75, 0.25] },
    });

    expect(first?.rect.w).toBe(900);
    expect(second?.rect.w).toBe(300);
  });

  it("Should activate from the physical edge while previewing inside the configured outer gaps", () => {
    const target = resolveSnapTarget({
      point: { x: AREA.x + 1, y: 500 },
      workArea: AREA,
      config: {
        ...DEFAULT_SNAP_TARGET_CONFIG,
        outerGaps: { top: 7, right: 10, bottom: 13, left: 10 },
      },
    });

    expect(target).toMatchObject({
      kind: "tile",
      edge: "left",
      rect: { x: 20, y: 27, w: 590, h: 780 },
    });
  });

  it("Should let none fall through to occupied resolution and let reserved block it", () => {
    const point = { x: 500, y: AREA.y + 1 };
    const edgeCandidate = {
      ...CANDIDATE,
      rect: { x: 300, y: AREA.y, w: 400, h: 300 },
    };
    const none = resolveSnapTarget({
      point,
      workArea: AREA,
      candidates: [edgeCandidate],
      config: { ...DEFAULT_SNAP_TARGET_CONFIG, topCenter: "none" },
    });
    const reserved = resolveSnapTarget({
      point,
      workArea: AREA,
      candidates: [edgeCandidate],
      config: { ...DEFAULT_SNAP_TARGET_CONFIG, topCenter: "reserved" },
    });

    expect(none?.kind).toBe("split");
    expect(reserved).toBeNull();
  });

  it("Should resolve zoom as the edge-center special target", () => {
    const target = resolveSnapTarget({
      point: { x: 600, y: 21 },
      workArea: AREA,
      config: { ...DEFAULT_SNAP_TARGET_CONFIG, topCenter: "zoom" },
    });

    expect(target?.kind).toBe("zoom");
  });
});
