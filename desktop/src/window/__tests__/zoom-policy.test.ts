import { describe, expect, it } from "vitest";

import { clampZoomLevel, nextZoomLevel, zoomDirectionForInput } from "../zoom-policy";

// Invariant: all zoom entry points share the same bounded integer ladder.
describe("zoom policy", () => {
  it("Should clamp at both bounds and reset exactly", () => {
    expect(clampZoomLevel(10)).toBe(3);
    expect(clampZoomLevel(-10)).toBe(-3);
    expect(nextZoomLevel(3, "in")).toBe(3);
    expect(nextZoomLevel(-3, "out")).toBe(-3);
    expect(nextZoomLevel(2, "reset")).toBe(0);
  });

  it.each([
    ["darwin", { meta: true, control: false, key: "0" }, "reset"],
    ["darwin", { meta: true, control: false, key: "=" }, "in"],
    ["darwin", { meta: true, control: false, key: "-" }, "out"],
    ["darwin", { meta: false, control: true, key: "0" }, null],
    ["darwin", { meta: false, control: true, key: "=" }, null],
    ["linux", { meta: false, control: true, key: "0" }, "reset"],
    ["linux", { meta: false, control: true, key: "+" }, "in"],
    ["win32", { meta: true, control: false, key: "0" }, null],
  ] as const)("Should map %s %o to the zoom direction %s", (platform, keys, expected) => {
    expect(zoomDirectionForInput({ type: "keyDown", ...keys }, platform)).toBe(expected);
  });

  it("Should ignore key releases so one press zooms once", () => {
    expect(
      zoomDirectionForInput({ type: "keyUp", meta: true, control: false, key: "=" }, "darwin")
    ).toBeNull();
  });
});
