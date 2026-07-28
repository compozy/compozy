import { describe, expect, it } from "vitest";
import { categoryLabel, formatDate, formatDateCompact, formatReadingTime } from "../format";

describe("blog formatting helpers", () => {
  it("formats release dates in UTC so midnight content dates do not drift by local timezone", () => {
    expect(formatDate("2026-04-30T00:00:00.000Z")).toBe("Apr 30, 2026");
    expect(formatDateCompact("2026-04-30T00:00:00.000Z")).toBe("Apr 30");
  });

  it("keeps reading time and category labels stable for public cards", () => {
    expect(formatReadingTime(0.2)).toBe("1 min");
    expect(formatReadingTime(1.6)).toBe("2 min");
    expect(categoryLabel("runtime")).toBe("Runtime");
  });
});
