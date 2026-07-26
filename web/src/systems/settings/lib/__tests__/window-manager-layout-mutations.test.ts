// Suite: layout structure conversions
// Invariant: every weight vector the editor emits already sums to 1, so
// `topology.split_weight_sum` is unreachable rather than reported (validate.go:249
// runs on the document as sent); and the orientation the person picks produces the
// geometry the runtime projector actually renders.
// Boundary IN: one layout node + the orientation chosen in the inspector.
// Boundary OUT: selection state, drafts, transport.
import { describe, expect, it } from "vitest";

import { projectLayout, type LayoutDesktop, type LayoutProjectionInput } from "@/systems/os";

import {
  createLayoutNodeIdFactory,
  evenWeights,
  layoutNodeIds,
  layoutOrientationOf,
  toSplit,
  toStack,
  type LayoutOrientation,
} from "../window-manager-layout-mutations";
import type { WindowManagerLayoutNode } from "../window-manager-layout-types";

const DAEMON_WEIGHT_TOLERANCE = 1e-6;

function stackOf(count: number): WindowManagerLayoutNode {
  const windowIds = Array.from({ length: count }, (_, index) => `window:${index + 1}`);
  return { id: "stack:seed", kind: "stack", windowIds, activeId: windowIds[0] ?? "" };
}

function mint() {
  return createLayoutNodeIdFactory([]);
}

function weightSum(node: WindowManagerLayoutNode): number {
  if (node.kind !== "split") throw new Error("expected a split");
  return node.weights.reduce((total, weight) => total + weight, 0);
}

/**
 * Renders the converted node through the same projector the OS shell uses. The
 * settings document type is structurally the runtime topology, so this is an
 * assignment, not a translation — if that ever stops being true, this file fails
 * to compile.
 */
function projectRoot(root: WindowManagerLayoutNode) {
  const desktop: LayoutDesktop = {
    id: "desktop:one",
    name: "One",
    order: 0,
    purpose: "standard",
    focusOwner: null,
    floating: [],
    groups: [{ id: "group:main", frame: { x: 0, y: 0, w: 1, h: 1 }, root }],
  };
  const input: LayoutProjectionInput = {
    revision: 1,
    desktop,
    workArea: { x: 0, y: 0, w: 1000, h: 600 },
    gaps: { inner: 0, top: 0, right: 0, bottom: 0, left: 0 },
  };
  return projectLayout(input);
}

describe("evenWeights", () => {
  it("Should sum to 1 inside the daemon's tolerance for every split size", () => {
    for (let count = 2; count <= 8; count += 1) {
      const total = evenWeights(count).reduce((sum, weight) => sum + weight, 0);
      expect(Math.abs(total - 1)).toBeLessThanOrEqual(DAEMON_WEIGHT_TOLERANCE);
    }
  });

  it("Should return nothing for a split that has no children", () => {
    expect(evenWeights(0)).toEqual([]);
  });
});

describe("toSplit", () => {
  it("Should emit normalized weights instead of one per window", () => {
    for (let count = 2; count <= 8; count += 1) {
      const split = toSplit(stackOf(count), "columns", mint());
      expect(Math.abs(weightSum(split) - 1)).toBeLessThanOrEqual(DAEMON_WEIGHT_TOLERANCE);
    }
  });

  it("Should give every window in the subtree its own leaf", () => {
    const split = toSplit(stackOf(3), "rows", mint());
    if (split.kind !== "split") throw new Error("expected a split");
    expect(split.children.map(child => (child.kind === "leaf" ? child.windowId : null))).toEqual([
      "window:1",
      "window:2",
      "window:3",
    ]);
    expect(split.weights).toHaveLength(3);
  });

  it("Should stack the windows top to bottom when the person picks rows", () => {
    const projection = projectRoot(toSplit(stackOf(2), "rows", mint()));
    const [first, second] = projection.windows;
    expect(first?.rect.x).toBe(second?.rect.x);
    expect(first?.rect.w).toBe(second?.rect.w);
    expect(second?.rect.y).toBeGreaterThan(first?.rect.y ?? 0);
  });

  it("Should place the windows side by side when the person picks columns", () => {
    const projection = projectRoot(toSplit(stackOf(2), "columns", mint()));
    const [first, second] = projection.windows;
    expect(first?.rect.y).toBe(second?.rect.y);
    expect(first?.rect.h).toBe(second?.rect.h);
    expect(second?.rect.x).toBeGreaterThan(first?.rect.x ?? 0);
  });

  it("Should round-trip an axis back to the orientation the inspector shows", () => {
    const orientations: LayoutOrientation[] = ["rows", "columns"];
    for (const orientation of orientations) {
      const split = toSplit(stackOf(2), orientation, mint());
      if (split.kind !== "split") throw new Error("expected a split");
      expect(layoutOrientationOf(split.axis)).toBe(orientation);
    }
  });
});

describe("toStack", () => {
  it("Should collect every window in the subtree and keep the first on top", () => {
    const split = toSplit(stackOf(3), "columns", mint());
    const stack = toStack(split, mint());
    if (stack.kind !== "stack") throw new Error("expected a stack");
    expect(stack.windowIds).toEqual(["window:1", "window:2", "window:3"]);
    expect(stack.activeId).toBe("window:1");
  });
});

describe("createLayoutNodeIdFactory", () => {
  it("Should never mint an id the document already uses", () => {
    const existing = toSplit(stackOf(2), "columns", mint());
    const reused = toSplit(stackOf(2), "rows", createLayoutNodeIdFactory(layoutNodeIds(existing)));
    const overlap = layoutNodeIds(reused).filter(id => layoutNodeIds(existing).includes(id));
    expect(overlap).toEqual([]);
  });

  it("Should mint distinct ids within one conversion", () => {
    const ids = layoutNodeIds(toSplit(stackOf(4), "rows", mint()));
    expect(new Set(ids).size).toBe(ids.length);
  });
});
