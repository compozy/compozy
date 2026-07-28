import { describe, expect, it } from "vitest";

import {
  compileCron,
  compressWeekdays,
  cronNext,
  dateToLocalInput,
  decodeCron,
  defaultAtLocal,
  formatAbsoluteUtc,
  formatClock,
  formatRelative,
  fullCronModel,
  humanCron,
  isValidCron,
  localInputToDate,
  parseCron,
  parseDuration,
  toRfc3339,
} from "../cron-engine";

// Wed 2026-06-03 08:30:00 UTC — a deterministic anchor for every time-based case.
const NOW = Date.UTC(2026, 5, 3, 8, 30, 0);

describe("cron-engine parseCron / isValidCron", () => {
  it("Should accept a well-formed 5-field expression", () => {
    expect(isValidCron("0 9 * * 1-5")).toBe(true);
    expect(parseCron("*/15 * * * *")).not.toBeNull();
  });

  it("Should reject seconds fields, macros, and out-of-range values", () => {
    expect(isValidCron("0 0 9 * * 1-5")).toBe(false); // 6 fields (seconds)
    expect(isValidCron("@daily")).toBe(false);
    expect(isValidCron("0 24 * * *")).toBe(false); // hour 24 out of range
    expect(isValidCron("60 * * * *")).toBe(false); // minute 60 out of range
    expect(isValidCron("0 9 * * 8")).toBe(false); // weekday 8 out of range
    expect(isValidCron("not a cron")).toBe(false);
  });
});

describe("cron-engine cronNext", () => {
  it("Should return strictly-increasing future weekday fires at the requested time", () => {
    const runs = cronNext("0 9 * * 1-5", 5, NOW);
    expect(runs).not.toBeNull();
    expect(runs).toHaveLength(5);
    for (let i = 0; i < runs!.length; i += 1) {
      const date = runs![i];
      expect(date.getTime()).toBeGreaterThan(NOW);
      expect(date.getUTCHours()).toBe(9);
      expect(date.getUTCMinutes()).toBe(0);
      // Mon–Fri only (1..5).
      expect(date.getUTCDay()).toBeGreaterThanOrEqual(1);
      expect(date.getUTCDay()).toBeLessThanOrEqual(5);
      if (i > 0) {
        expect(date.getTime()).toBeGreaterThan(runs![i - 1].getTime());
      }
    }
    // First weekday fire after Wed 08:30 is the same Wed at 09:00.
    expect(runs![0].toISOString()).toBe("2026-06-03T09:00:00.000Z");
  });

  it("Should space minute-interval fires evenly", () => {
    const runs = cronNext("*/15 * * * *", 3, NOW);
    expect(runs).not.toBeNull();
    expect(runs!.map(date => date.toISOString())).toEqual([
      "2026-06-03T08:45:00.000Z",
      "2026-06-03T09:00:00.000Z",
      "2026-06-03T09:15:00.000Z",
    ]);
  });

  it("Should return null for an invalid expression", () => {
    expect(cronNext("nope", 5, NOW)).toBeNull();
  });
});

describe("cron-engine humanCron", () => {
  it("Should describe common rhythms in plain language", () => {
    expect(humanCron("0 9 * * 1-5")).toBe("every weekday at 09:00");
    expect(humanCron("0 9 * * *")).toBe("every day at 09:00");
    expect(humanCron("0 0 * * *")).toBe("every day at midnight");
    expect(humanCron("0 * * * *")).toBe("every hour, on the hour");
    expect(humanCron("30 * * * *")).toBe("every hour at :30");
    expect(humanCron("*/15 * * * *")).toBe("every 15 minutes");
    expect(humanCron("0 8 * * 0,6")).toBe("weekends at 08:00");
    expect(humanCron("0 8 * * 1")).toBe("every Monday at 08:00");
    expect(humanCron("0 9 15 * *")).toBe("on day 15 of every month at 09:00");
  });

  it("Should return null for shapes it cannot phrase", () => {
    expect(humanCron("5 9 3 6 *")).toBeNull(); // specific month — unsupported phrasing
    expect(humanCron("bad")).toBeNull();
  });
});

describe("cron-engine decode / compile round-trips", () => {
  it("Should decode known expressions into builder models", () => {
    expect(decodeCron("*/10 * * * *")).toEqual({ frequency: "minutes", everyMinutes: 10 });
    expect(decodeCron("15 * * * *")).toEqual({ frequency: "hourly", hourlyMinute: 15 });
    expect(decodeCron("0 9 * * *")).toEqual({ frequency: "daily", hour: 9, minute: 0 });
    expect(decodeCron("30 8 * * 1-5")).toEqual({
      frequency: "weekly",
      hour: 8,
      minute: 30,
      weekdays: [1, 2, 3, 4, 5],
    });
    expect(decodeCron("0 9 15 * *")).toEqual({
      frequency: "monthly",
      hour: 9,
      minute: 0,
      monthDay: 15,
    });
    expect(decodeCron("5 9 3 6 *")).toBeNull();
  });

  it("Should compile builder models back to expressions", () => {
    expect(compileCron(fullCronModel({ frequency: "minutes", everyMinutes: 10 }))).toBe(
      "*/10 * * * *"
    );
    expect(compileCron(fullCronModel({ frequency: "daily", hour: 9, minute: 0 }))).toBe(
      "0 9 * * *"
    );
    expect(
      compileCron(
        fullCronModel({ frequency: "weekly", hour: 8, minute: 30, weekdays: [1, 2, 3, 4, 5] })
      )
    ).toBe("30 8 * * 1-5");
    expect(
      compileCron(fullCronModel({ frequency: "monthly", hour: 9, minute: 0, monthDay: 15 }))
    ).toBe("0 9 15 * *");
    expect(compileCron(fullCronModel({ frequency: "custom" }))).toBeNull();
  });

  it("Should compress weekday lists into compact cron tokens", () => {
    expect(compressWeekdays([1, 2, 3, 4, 5])).toBe("1-5");
    expect(compressWeekdays([0, 6])).toBe("0,6");
    expect(compressWeekdays([1, 3, 5])).toBe("1,3,5");
    expect(compressWeekdays([0, 1, 2, 3, 4, 5, 6])).toBe("*");
  });
});

describe("cron-engine duration + at-time helpers", () => {
  it("Should parse positive Go-style durations and reject the rest", () => {
    expect(parseDuration("30m")).toBe(30 * 60_000);
    expect(parseDuration("1h")).toBe(3_600_000);
    expect(parseDuration("2h30m")).toBe(2 * 3_600_000 + 30 * 60_000);
    expect(parseDuration("45s")).toBe(45_000);
    expect(parseDuration("0m")).toBeNull();
    expect(parseDuration("")).toBeNull();
    expect(parseDuration("5 minutes")).toBeNull();
  });

  it("Should round-trip a UTC datetime-local value", () => {
    const value = "2026-06-04T09:00";
    const date = localInputToDate(value);
    expect(date).not.toBeNull();
    expect(date!.toISOString()).toBe("2026-06-04T09:00:00.000Z");
    expect(dateToLocalInput(date!)).toBe(value);
    expect(localInputToDate("garbage")).toBeNull();
  });

  it("Should derive a next-round-hour default one day out", () => {
    expect(defaultAtLocal(NOW)).toBe("2026-06-04T08:00");
  });

  it("Should format RFC3339 without milliseconds", () => {
    expect(toRfc3339(new Date(Date.UTC(2026, 5, 4, 9, 0, 0)))).toBe("2026-06-04T09:00:00Z");
    expect(toRfc3339(null)).toBe("");
  });
});

describe("cron-engine formatters", () => {
  it("Should format clock + absolute UTC labels", () => {
    expect(formatClock(9, 5)).toBe("09:05");
    expect(formatAbsoluteUtc(new Date(Date.UTC(2026, 5, 3, 9, 0, 0)))).toBe("Wed Jun 3, 09:00");
  });

  it("Should format relative offsets", () => {
    expect(formatRelative(new Date(NOW - 1000), NOW)).toBe("now");
    expect(formatRelative(new Date(NOW + 30_000), NOW)).toBe("in 30s");
    expect(formatRelative(new Date(NOW + 45 * 60_000), NOW)).toBe("in 45 min");
    expect(formatRelative(new Date(NOW + (2 * 3_600_000 + 30 * 60_000)), NOW)).toBe("in 2h 30m");
    expect(formatRelative(new Date(NOW + 3 * 3_600_000), NOW)).toBe("in 3h");
    expect(formatRelative(new Date(NOW + 25 * 3_600_000), NOW)).toBe("in 1d 1h");
  });
});
