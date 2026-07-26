import { describe, expect, it } from "vitest";

import { FORMAT_TIME_FALLBACK, formatUptimeSeconds } from "../format-time";

describe("format-time re-export", () => {
  it("Should re-export the @agh/ui sentinel string", () => {
    expect(FORMAT_TIME_FALLBACK).toBe("—");
  });
});

describe("formatUptimeSeconds", () => {
  it("Should return an em-dash for invalid input", () => {
    expect(formatUptimeSeconds(undefined)).toBe("—");
    expect(formatUptimeSeconds(null)).toBe("—");
    expect(formatUptimeSeconds(-12)).toBe("—");
    expect(formatUptimeSeconds(Number.NaN)).toBe("—");
  });

  it("Should format sub-minute durations as seconds", () => {
    expect(formatUptimeSeconds(0)).toBe("0s");
    expect(formatUptimeSeconds(45)).toBe("45s");
  });

  it("Should format minutes hours and days with their largest two units", () => {
    expect(formatUptimeSeconds(60)).toBe("1m");
    expect(formatUptimeSeconds(125)).toBe("2m 5s");
    expect(formatUptimeSeconds(3_600)).toBe("1h");
    expect(formatUptimeSeconds(3_600 + 1_800)).toBe("1h 30m");
    expect(formatUptimeSeconds(86_400)).toBe("1d");
    expect(formatUptimeSeconds(86_400 + 3_600 * 5)).toBe("1d 5h");
  });
});
