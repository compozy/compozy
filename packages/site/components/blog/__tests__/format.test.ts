import { describe, expect, it } from "vitest";
import { categoryLabel } from "../format";

describe("blog formatting helpers", () => {
  it("keeps category labels stable for public cards", () => {
    expect(categoryLabel("runtime")).toBe("Runtime");
  });
});
