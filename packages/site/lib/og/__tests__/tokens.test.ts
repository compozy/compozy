import { describe, expect, it } from "vitest";
import { truncate } from "../tokens";

describe("OG token helpers", () => {
  it("keeps truncated strings within the requested max length", () => {
    expect(truncate("abcdefghij", 8)).toBe("abcde...");
    expect(truncate("short", 8)).toBe("short");
  });

  it("handles very small max lengths without overflowing the limit", () => {
    expect(truncate("abcdef", 3)).toBe("...");
    expect(truncate("abcdef", 2)).toBe("..");
    expect(truncate("abcdef", 1)).toBe(".");
    expect(truncate("abcdef", 0)).toBe("");
  });
});
