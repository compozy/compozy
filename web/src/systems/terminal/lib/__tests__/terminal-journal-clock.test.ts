import { describe, expect, it } from "vitest";

import { formatTerminalJournalClock } from "../terminal-journal-clock";

/**
 * Canonical suite for the journal clock (lib).
 *
 * Invariant: the formatter is a zero-padded 24h clock from UTC numeric fields,
 * never a locale time string.
 */
describe("formatTerminalJournalClock", () => {
  it("Should format 12:44 from UTC hour and minute fields", () => {
    expect(formatTerminalJournalClock("2026-08-25T12:44:00Z")).toBe("12:44");
  });

  it("Should format 12:44:09 when seconds are requested", () => {
    expect(formatTerminalJournalClock("2026-08-25T12:44:09Z", { seconds: true })).toBe("12:44:09");
  });
});
