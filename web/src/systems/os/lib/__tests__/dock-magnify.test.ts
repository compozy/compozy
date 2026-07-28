// Suite: dock magnification math
// Invariant: OpenDesign falloff maps distance → factor and factor → transform.
// Boundary IN: pointer distance to icon center.
// Boundary OUT: DOM pointer wiring (useDockMagnify / OsDock).
import { describe, expect, it } from "vitest";

import {
  DOCK_MAG_LIFT_PX,
  DOCK_MAG_RADIUS,
  DOCK_MAG_SCALE,
  dockMagnifyFactor,
  dockMagnifyTransform,
} from "../dock-magnify";

describe("dockMagnifyFactor", () => {
  it("Should peak at the icon center and fall linearly to zero at the radius", () => {
    expect(dockMagnifyFactor(0)).toBe(1);
    expect(dockMagnifyFactor(DOCK_MAG_RADIUS / 2)).toBe(0.5);
    expect(dockMagnifyFactor(DOCK_MAG_RADIUS)).toBe(0);
    expect(dockMagnifyFactor(DOCK_MAG_RADIUS + 1)).toBe(0);
  });
});

describe("dockMagnifyTransform", () => {
  it("Should emit the prototype translateY + scale string at full influence", () => {
    expect(dockMagnifyTransform(1)).toBe(
      `translateY(${-DOCK_MAG_LIFT_PX}px) scale(${1 + DOCK_MAG_SCALE})`
    );
    expect(dockMagnifyTransform(0.5)).toBe(
      `translateY(${-DOCK_MAG_LIFT_PX * 0.5}px) scale(${1 + DOCK_MAG_SCALE * 0.5})`
    );
    expect(dockMagnifyTransform(0)).toBe("");
  });
});
