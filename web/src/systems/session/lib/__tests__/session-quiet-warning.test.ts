import { describe, expect, it } from "vitest";

import {
  formatQuietDuration,
  formatQuietDurationWords,
  sessionQuietWarning,
  sessionQuietWarningClock,
  sessionQuietWarningFacts,
} from "../session-quiet-warning";

// Suite: quiet-warning read model (US-014.AC-1, US-014.AC-3, US-014.EC-2).
// Invariant: the open session warns only while the daemon's
// `supervision.quiet_warning` is present, derives elapsed/remaining from the
// daemon's instants, and never promises a stop the daemon did not schedule.
// Boundary IN: the session resource `supervision` block and the browser clock.
// Boundary OUT: notice and status-row rendering.
const QUIET_SINCE = "2026-09-06T10:00:00Z";
const WARNED_AT = "2026-09-06T10:30:00Z";
const STOP_AT = "2026-09-06T10:40:00Z";

const scheduled = {
  state: "active" as const,
  supervision: {
    quiet_warning: { quiet_since: QUIET_SINCE, stop_at: STOP_AT, warned_at: WARNED_AT },
    sources: [],
    work_signals: [],
  },
};

describe("session quiet-warning read model", () => {
  it("Should surface a warning only while the daemon reports one on a live session", () => {
    expect(sessionQuietWarning({ state: "active", supervision: null })).toBeNull();
    expect(
      sessionQuietWarning({
        state: "active",
        supervision: { quiet_warning: null, sources: [], work_signals: [] },
      })
    ).toBeNull();
    expect(sessionQuietWarning({ ...scheduled, state: "stopped" })).toBeNull();
    expect(sessionQuietWarning(scheduled)).toEqual({
      quietSinceMs: Date.parse(QUIET_SINCE),
      stopAtMs: Date.parse(STOP_AT),
      warnedAtMs: Date.parse(WARNED_AT),
    });
  });

  it("Should keep the warning without a deadline when automatic stop is off", () => {
    const warning = sessionQuietWarning({
      ...scheduled,
      supervision: {
        ...scheduled.supervision,
        quiet_warning: { ...scheduled.supervision.quiet_warning, stop_at: null },
      },
    });
    expect(warning?.stopAtMs).toBeNull();
    expect(sessionQuietWarningFacts(warning!)).toEqual({ quietFor: "30 minutes", stopsIn: null });
    expect(sessionQuietWarningClock(warning!, Date.parse(WARNED_AT) + 60_000)).toEqual({
      quietFor: "31m",
      stopDue: false,
      stopsIn: null,
    });
  });

  it("Should state the warned durations as facts and count the live episode from the daemon instants", () => {
    const warning = sessionQuietWarning(scheduled)!;
    expect(sessionQuietWarningFacts(warning)).toEqual({
      quietFor: "30 minutes",
      stopsIn: "10 minutes",
    });
    expect(sessionQuietWarningClock(warning, Date.parse(WARNED_AT) + 61_000)).toEqual({
      quietFor: "31m",
      stopDue: false,
      stopsIn: "8m",
    });
    expect(sessionQuietWarningClock(warning, Date.parse(STOP_AT) + 5_000)).toEqual({
      quietFor: "40m",
      stopDue: true,
      stopsIn: null,
    });
  });

  it("Should drop a warning whose instants cannot be read", () => {
    expect(
      sessionQuietWarning({
        ...scheduled,
        supervision: {
          ...scheduled.supervision,
          quiet_warning: { quiet_since: "not-a-date", stop_at: null, warned_at: WARNED_AT },
        },
      })
    ).toBeNull();
  });

  it("Should format quiet durations at minute granularity and in words", () => {
    expect(formatQuietDuration(45_000)).toBe("45s");
    expect(formatQuietDuration(31 * 60_000 + 4_000)).toBe("31m");
    expect(formatQuietDuration(65 * 60_000)).toBe("1h 5m");
    expect(formatQuietDuration(-5_000)).toBe("0s");
    expect(formatQuietDurationWords(1_000)).toBe("1 second");
    expect(formatQuietDurationWords(60_000)).toBe("1 minute");
    expect(formatQuietDurationWords(30 * 60_000)).toBe("30 minutes");
    expect(formatQuietDurationWords(70 * 60_000)).toBe("1 hour 10 minutes");
    expect(formatQuietDurationWords(2 * 3_600_000)).toBe("2 hours");
  });
});
