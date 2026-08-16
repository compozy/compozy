// Suite: session-badge
// Invariant: one exported badge dictionary covers the daemon's ten-token
// vocabulary exhaustively, is the only tone/glyph source for every attention
// surface, pairs each tone with a distinct shape or glyph so no state reads as
// colour alone, and narrows unrecognized tokens to the honesty state.
// Owning layer: unit (systems/session/lib)
import { describe, expect, it } from "vitest";

import {
  isFinishedBadge,
  isNeedsYouBadge,
  SESSION_BADGE_SIGNAL,
  SESSION_BADGES,
  sessionAttentionClass,
  sessionBadgeSignal,
  toSessionBadge,
  type SessionBadgeToken,
} from "../session-badge";

const NEEDS_YOU: SessionBadgeToken[] = ["waiting-for-input", "waiting-for-auth", "failed"];

describe("session badge dictionary", () => {
  it("Should cover the daemon's ten-token vocabulary exhaustively (UT-050)", () => {
    // The Go authority is internal/session/badge.go; a token added there must
    // land here or `satisfies Record<SessionBadgeToken, …>` stops compiling.
    expect([...SESSION_BADGES].sort()).toEqual(
      [
        "done",
        "failed",
        "hung",
        "idle",
        "running",
        "stopped",
        "unhealthy",
        "unknown",
        "waiting-for-auth",
        "waiting-for-input",
      ].sort()
    );
    expect(Object.keys(SESSION_BADGE_SIGNAL).sort()).toEqual([...SESSION_BADGES].sort());
  });

  it("Should never convey a state by colour alone (UT-050)", () => {
    // Every badge carries a glyph, a shape, and the exact CLI state word, so a
    // shared tone is always disambiguated on a second channel.
    for (const badge of SESSION_BADGES) {
      const signal = SESSION_BADGE_SIGNAL[badge];
      expect(signal.glyph).toBeDefined();
      expect(signal.shape).toBeTruthy();
      expect(signal.label).toBe(badge);
    }
    const needsYouShapes = NEEDS_YOU.map(badge => SESSION_BADGE_SIGNAL[badge].shape);
    const needsYouGlyphs = NEEDS_YOU.map(badge => SESSION_BADGE_SIGNAL[badge].glyph);
    expect(new Set(NEEDS_YOU.map(badge => SESSION_BADGE_SIGNAL[badge].tone))).toEqual(
      new Set(["danger"])
    );
    expect(new Set(needsYouShapes).size).toBe(NEEDS_YOU.length);
    expect(new Set(needsYouGlyphs).size).toBe(NEEDS_YOU.length);
  });

  it("Should ink the locked tone per state (UT-050)", () => {
    expect(SESSION_BADGE_SIGNAL.done.tone).toBe("info");
    expect(SESSION_BADGE_SIGNAL.running.tone).toBe("accent");
    expect(SESSION_BADGE_SIGNAL.running.pulse).toBe(true);
    expect(SESSION_BADGE_SIGNAL.idle.tone).toBe("success");
    expect(SESSION_BADGE_SIGNAL.hung.tone).toBe("warning");
    expect(SESSION_BADGE_SIGNAL.unhealthy.tone).toBe("warning");
    // The honesty state must stay visually distinct from a stopped session.
    expect(SESSION_BADGE_SIGNAL.unknown.shape).toBe("dashed-ring");
    expect(SESSION_BADGE_SIGNAL.stopped.shape).toBe("ring");
  });

  it("Should render an unrecognized token as unknown rather than guessing", () => {
    expect(toSessionBadge("sleeping")).toBe("unknown");
    expect(toSessionBadge("")).toBe("unknown");
    expect(toSessionBadge(null)).toBe("unknown");
    expect(toSessionBadge(undefined)).toBe("unknown");
    expect(sessionBadgeSignal("sleeping").label).toBe("unknown");
  });
});

describe("attention classes", () => {
  it("Should match needs-you on exactly input, auth, and failed (UT-051)", () => {
    for (const badge of SESSION_BADGES) {
      const expected = NEEDS_YOU.includes(badge);
      expect(isNeedsYouBadge(badge)).toBe(expected);
    }
  });

  it("Should keep done in its own finished class so it never inflates needs-you (UT-051)", () => {
    expect(sessionAttentionClass("done")).toBe("finished");
    expect(isFinishedBadge("done")).toBe(true);
    expect(isNeedsYouBadge("done")).toBe(false);
    for (const badge of SESSION_BADGES) {
      if (badge === "done") continue;
      expect(isFinishedBadge(badge)).toBe(false);
    }
  });

  it("Should classify an unrecognized badge as none instead of throwing (UT-051)", () => {
    expect(sessionAttentionClass("sleeping")).toBe("none");
    expect(isNeedsYouBadge("sleeping")).toBe(false);
  });
});
