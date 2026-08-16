import { describe, expect, it } from "vitest";

import { clampZoomLevel, nextZoomLevel } from "../zoom-policy";

// Invariant: all zoom entry points share the same bounded integer ladder.
describe("zoom policy", () => {
  it("Should clamp at both bounds and reset exactly", () => {
    expect(clampZoomLevel(10)).toBe(3);
    expect(clampZoomLevel(-10)).toBe(-3);
    expect(nextZoomLevel(3, "in")).toBe(3);
    expect(nextZoomLevel(-3, "out")).toBe(-3);
    expect(nextZoomLevel(2, "reset")).toBe(0);
  });
});
