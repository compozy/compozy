// Invariant: parseRgbaColor rejects malformed declarations instead of accepting a valid prefix.
// Owning layer: contrast parser utility boundary.
// Canonical suite: this package-level contrast parser suite (no existing suite owns this helper).

import { describe, expect, it } from "vitest";

import { parseRgbaColor } from "../contrast";

describe("parseRgbaColor", () => {
  it("Should reject trailing text after an rgba declaration", () => {
    expect(parseRgbaColor("rgba(1, 2, 3)junk")).toBeNull();
  });

  it("Should accept surrounding whitespace around a complete declaration", () => {
    expect(parseRgbaColor("  rgba(1, 2, 3, 0.5)  ")).toEqual({
      rgb: { r: 1, g: 2, b: 3 },
      alpha: 0.5,
    });
  });
});
