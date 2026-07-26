import { describe, expect, it } from "vitest";

import {
  formatDatePill,
  formatTimelineClock,
  formatTimelineClockWithSeconds,
  formatTimelineIso,
  isSameCalendarDay,
  isWithinSeconds,
  TIMELINE_GROUP_WINDOW_SECONDS,
} from "../format-timestamp";

describe("formatTimelineClock", () => {
  it("Should format a clock time without seconds", () => {
    const clock = formatTimelineClock("2026-04-17T14:32:00Z");
    expect(clock).toMatch(/\d{1,2}:\d{2}/);
    expect(clock).not.toMatch(/\d{2}:\d{2}:\d{2}/);
  });

  it("Should return empty string for invalid input", () => {
    expect(formatTimelineClock(null)).toBe("");
    expect(formatTimelineClock(undefined)).toBe("");
    expect(formatTimelineClock("not-a-date")).toBe("");
  });
});

describe("formatTimelineClockWithSeconds", () => {
  it("Should include seconds in the clock format", () => {
    const clock = formatTimelineClockWithSeconds("2026-04-17T14:32:09Z");
    expect(clock).toMatch(/\d{1,2}:\d{2}:\d{2}/);
  });
});

describe("formatTimelineIso", () => {
  it("Should round-trip an ISO timestamp", () => {
    expect(formatTimelineIso("2026-04-17T14:32:00.000Z")).toBe("2026-04-17T14:32:00.000Z");
  });
});

describe("formatDatePill", () => {
  it("Should label same-day timestamps as TODAY", () => {
    const now = new Date("2026-04-17T15:00:00Z");
    expect(formatDatePill("2026-04-17T08:00:00Z", { now })).toBe("TODAY");
  });

  it("Should label previous day as YESTERDAY", () => {
    const now = new Date("2026-04-17T15:00:00Z");
    expect(formatDatePill("2026-04-16T08:00:00Z", { now })).toBe("YESTERDAY");
  });

  it("Should render the weekday for older same-week dates", () => {
    const now = new Date("2026-04-17T15:00:00Z");
    expect(formatDatePill("2026-04-14T08:00:00Z", { now })).toMatch(/^[A-Z]+$/);
  });

  it("Should prefix the year when crossing a year boundary", () => {
    const now = new Date("2026-04-17T15:00:00Z");
    expect(formatDatePill("2025-12-15T08:00:00Z", { now })).toMatch(/^2025/);
  });

  it("Should include the month for dates older than a week", () => {
    const now = new Date("2026-04-17T15:00:00Z");
    expect(formatDatePill("2026-03-15T08:00:00Z", { now })).toMatch(/·/);
  });

  it("Should return empty string for invalid input", () => {
    expect(formatDatePill(null)).toBe("");
    expect(formatDatePill("not-a-date")).toBe("");
  });
});

describe("isWithinSeconds", () => {
  it("Should return true when current is within the window after previous", () => {
    expect(isWithinSeconds("2026-04-17T14:32:30Z", "2026-04-17T14:32:00Z", 60)).toBe(true);
  });

  it("Should return false when current is past the window", () => {
    expect(isWithinSeconds("2026-04-17T14:33:30Z", "2026-04-17T14:32:00Z", 60)).toBe(false);
  });

  it("Should return false when current precedes previous", () => {
    expect(isWithinSeconds("2026-04-17T14:31:30Z", "2026-04-17T14:32:00Z", 60)).toBe(false);
  });

  it("Should default to a 60s window", () => {
    expect(TIMELINE_GROUP_WINDOW_SECONDS).toBe(60);
  });
});

describe("isSameCalendarDay", () => {
  it("Should detect same day for two timestamps", () => {
    expect(isSameCalendarDay("2026-04-17T08:00:00Z", "2026-04-17T22:00:00Z")).toBe(true);
  });

  it("Should detect different days", () => {
    expect(isSameCalendarDay("2026-04-17T08:00:00Z", "2026-04-18T08:00:00Z")).toBe(false);
  });
});
