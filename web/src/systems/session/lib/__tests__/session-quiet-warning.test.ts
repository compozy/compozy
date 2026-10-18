import { describe, expect, it } from "vitest";

import { formatQuietDuration, formatQuietDurationWords } from "../session-quiet-warning";

// Suite: quiet-episode duration words.
// Invariant: the live "quiet for {d}" segment reads minute-granular status
// vocabulary and the legacy "no work for {d}" stop sentence reads spelled-out
// words, both truthful under a minute.
// Boundary IN: a duration in milliseconds. Boundary OUT: row rendering.
describe("session quiet durations", () => {
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
