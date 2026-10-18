// Suite: editor schedule builder (UT-012 vectors)
// Invariant: quick picks, days and the clock compile to the cron the daemon stores and read back
// as the same plain phrase; a shape the day picker can't show reports no days.
import { describe, expect, it } from "vitest";

import {
  SCHEDULE_QUICK_PICKS,
  scheduleDayTime,
  setScheduleTime,
  toggleScheduleDay,
} from "../automation-form-schedule";
import { describeSchedule } from "../automation-sentence";

const ALL_DAYS = [0, 1, 2, 3, 4, 5, 6];

describe("schedule builder", () => {
  it.each([
    ["Weekdays 9am", "0 9 * * 1-5", "Every weekday at 09:00 UTC", [1, 2, 3, 4, 5]],
    ["Every day 9am", "0 9 * * *", "Every day at 09:00 UTC", ALL_DAYS],
    ["Mondays 8am", "0 8 * * 1", "Every Monday at 08:00 UTC", [1]],
    ["Every hour", "0 * * * *", "Every hour, on the hour", null],
    ["Midnight", "0 0 * * *", "Every day at midnight UTC", ALL_DAYS],
  ])("quick pick %s compiles to %s and reads %s", (label, expr, phrase, days) => {
    expect(SCHEDULE_QUICK_PICKS.find(pick => pick.label === label)?.expr).toBe(expr);
    expect(describeSchedule({ mode: "cron", expr })).toBe(phrase);
    expect(scheduleDayTime(expr)?.days ?? null).toEqual(days);
  });

  it.each([
    ["adds a day to an hourly shape at 09:00", "0 * * * *", 1, false, "0 9 * * 1", false],
    ["drops Sunday from every day", "0 9 * * *", 0, false, "0 9 * * 1-6", false],
    ["completes the week as every day", "0 9 * * 0-5", 6, false, "0 9 * * *", false],
    ["clears the last day without losing the time", "0 8 * * 1", 1, false, "0 8 * * 1", true],
    ["picks a day again after clearing", "0 8 * * 1", 5, true, "0 8 * * 5", false],
  ])("toggleScheduleDay %s", (_name, expr, day, cleared, nextExpr, nextCleared) => {
    expect(toggleScheduleDay(expr, day, cleared)).toEqual({ expr: nextExpr, cleared: nextCleared });
  });

  it.each([
    ["keeps the days", "0 9 * * 1-5", "07:30", "30 7 * * 1-5"],
    ["turns an hourly shape into every day", "0 * * * *", "10:00", "0 10 * * *"],
    ["rejects an impossible clock", "0 9 * * *", "25:00", null],
  ])("setScheduleTime %s", (_name, expr, value, next) => {
    expect(setScheduleTime(expr, value)).toBe(next);
  });

  it("reads intervals and one-shots in plain words", () => {
    expect(describeSchedule({ mode: "every", interval: "30m" })).toBe("Every 30 minutes");
    expect(describeSchedule({ mode: "at", time: "2026-10-08T09:00:00Z" })).toBe(
      "Once on Thu Oct 8 at 09:00 UTC"
    );
  });
});
