import { describe, expect, it } from "vitest";

import {
  FORMAT_TIME_FALLBACK,
  formatAbsoluteTime,
  formatDuration,
  formatRelativeTime,
} from "../format-time";

const NOW = new Date("2026-05-11T12:00:00Z").getTime();

describe("formatRelativeTime", () => {
  it("Should return 'just now' for sub-30s deltas", () => {
    expect(formatRelativeTime(new Date(NOW - 1_000).toISOString(), NOW)).toBe("just now");
    expect(formatRelativeTime(new Date(NOW - 29_000).toISOString(), NOW)).toBe("just now");
  });

  it("Should render seconds, minutes, hours, and days in the past", () => {
    expect(formatRelativeTime(new Date(NOW - 45_000).toISOString(), NOW)).toBe("45s ago");
    expect(formatRelativeTime(new Date(NOW - 5 * 60_000).toISOString(), NOW)).toBe("5m ago");
    expect(formatRelativeTime(new Date(NOW - 3 * 3_600_000).toISOString(), NOW)).toBe("3h ago");
    expect(formatRelativeTime(new Date(NOW - 2 * 86_400_000).toISOString(), NOW)).toBe("2d ago");
  });

  it("Should render future deltas with the `in N` prefix", () => {
    expect(formatRelativeTime(new Date(NOW + 90_000).toISOString(), NOW)).toBe("in 1m");
  });

  it("Should fall back to absolute format past 5 weeks", () => {
    const past = new Date(NOW - 6 * 7 * 86_400_000).toISOString();
    const result = formatRelativeTime(past, NOW);
    expect(result).not.toMatch(/ago$/);
    expect(result).toMatch(/2026/);
  });

  it("Should return the sentinel for invalid ISO strings", () => {
    expect(formatRelativeTime("not-iso", NOW)).toBe(FORMAT_TIME_FALLBACK);
  });
});

describe("formatAbsoluteTime", () => {
  it("Should render a locale-aware absolute timestamp", () => {
    expect(formatAbsoluteTime(new Date(NOW).toISOString())).toMatch(/2026/);
  });

  it("Should return the sentinel for invalid ISO strings", () => {
    expect(formatAbsoluteTime("bad")).toBe(FORMAT_TIME_FALLBACK);
  });
});

describe("formatDuration", () => {
  it("Should return 0s for non-positive durations", () => {
    expect(formatDuration(0)).toBe("0s");
    expect(formatDuration(-100)).toBe("0s");
    expect(formatDuration(Number.NaN)).toBe("0s");
  });

  it("Should compose the two largest duration units including days", () => {
    expect(formatDuration(125_000)).toBe("2m 5s");
    expect(formatDuration(120_000)).toBe("2m");
    expect(formatDuration(3_600_000)).toBe("1h");
    expect(formatDuration(3_725_000)).toBe("1h 2m");
    expect(formatDuration(86_400_000)).toBe("1d");
    expect(formatDuration(86_700_000)).toBe("1d 5m");
    expect(formatDuration(90_061_000)).toBe("1d 1h");
  });
});
