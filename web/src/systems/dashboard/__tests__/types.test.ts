import { describe, expect, it } from "vitest";

import { normalizeHomeUsageWindow } from "../types";

describe("normalizeHomeUsageWindow", () => {
  it("Should keep supported windows and fall back to 30 otherwise", () => {
    expect(normalizeHomeUsageWindow(7)).toBe(7);
    expect(normalizeHomeUsageWindow(90)).toBe(90);
    expect(normalizeHomeUsageWindow(13)).toBe(30);
    expect(normalizeHomeUsageWindow(Number.NaN)).toBe(30);
  });
});
