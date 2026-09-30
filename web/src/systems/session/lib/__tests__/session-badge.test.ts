// Suite: session-badge
// Invariant: one exported badge dictionary covers the daemon's eleven-token
// vocabulary exhaustively, is the only state-glyph/tone source for every
// attention surface, spells out the state word wherever a glyph is shared so no
// state reads as colour or shape alone, mirrors the daemon's attention classes,
// and narrows unrecognized tokens to the honesty state.
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

/** Asks waiting on the operator: one attention glyph, told apart by their words. */
const WAITING_ON_YOU: SessionBadgeToken[] = [
  "waiting-for-input",
  "waiting-for-auth",
  "needs-attention",
];
/** The daemon's needs-you class (`session.ClassForBadge`). */
const NEEDS_YOU: SessionBadgeToken[] = [...WAITING_ON_YOU, "failed"];

describe("session badge dictionary", () => {
  it("Should cover the daemon's eleven-token vocabulary exhaustively (UT-050)", () => {
    // The Go authority is internal/session/badge.go; a token added there must
    // land here or `satisfies Record<SessionBadgeToken, …>` stops compiling.
    expect([...SESSION_BADGES].sort()).toEqual(
      [
        "done",
        "failed",
        "hung",
        "idle",
        "needs-attention",
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
    // Every badge carries a state glyph, a plain state word, and the exact CLI
    // token. Badges that share a glyph must speak their word wherever the mark
    // shows, so a shared glyph is always disambiguated on a second channel.
    for (const badge of SESSION_BADGES) {
      const signal = SESSION_BADGE_SIGNAL[badge];
      expect(signal.state).toBeTruthy();
      expect(signal.label).toBe(badge);
      expect(signal.displayLabel).toMatch(/^[A-Z][a-z -]+$/);
    }
    expect(SESSION_BADGE_SIGNAL["waiting-for-input"].displayLabel).toBe("Needs your answer");
    expect(SESSION_BADGE_SIGNAL.hung.displayLabel).toBe("Stuck");
    const byState = new Map<string, SessionBadgeToken[]>();
    for (const badge of SESSION_BADGES) {
      const state = SESSION_BADGE_SIGNAL[badge].state;
      byState.set(state, [...(byState.get(state) ?? []), badge]);
    }
    for (const [state, badges] of byState) {
      const silent = badges.filter(badge => !SESSION_BADGE_SIGNAL[badge].speaks);
      // At most one badge per glyph may rest on the glyph alone.
      expect(silent.length, `glyph ${state}`).toBeLessThanOrEqual(1);
    }
    // Every needs-you badge names itself in words.
    for (const badge of NEEDS_YOU) expect(SESSION_BADGE_SIGNAL[badge].speaks).toBe(true);
  });

  it("Should map each badge to its canonical state glyph and tone (UT-050)", () => {
    // Needs-you is the Compozy-orange attention dot (never the warning amber),
    // failures and a stuck runtime are the danger ring, work in flight is the
    // mint running ring, and resting states stay neutral.
    for (const badge of WAITING_ON_YOU) {
      expect(SESSION_BADGE_SIGNAL[badge].state).toBe("attention");
      expect(SESSION_BADGE_SIGNAL[badge].tone).toBe("accent");
    }
    for (const badge of ["failed", "hung", "unhealthy"] as const) {
      expect(SESSION_BADGE_SIGNAL[badge].state).toBe("failed");
      expect(SESSION_BADGE_SIGNAL[badge].tone).toBe("danger");
    }
    expect(SESSION_BADGE_SIGNAL.running.state).toBe("running");
    expect(SESSION_BADGE_SIGNAL.done.state).toBe("done");
    expect(SESSION_BADGE_SIGNAL.stopped.state).toBe("stopped");
    expect(SESSION_BADGE_SIGNAL.idle.state).toBe("idle");
    for (const badge of ["running", "done", "stopped", "idle", "unknown"] as const) {
      expect(SESSION_BADGE_SIGNAL[badge].tone).toBe("neutral");
    }
    // The honesty state never fakes liveness: it names itself instead of
    // resting on the idle dot it shares.
    expect(SESSION_BADGE_SIGNAL.unknown.speaks).toBe(true);
    expect(SESSION_BADGE_SIGNAL.stopped.state).not.toBe(SESSION_BADGE_SIGNAL.unknown.state);
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
  it("Should match needs-you on exactly input, auth, failed, and needs-attention (UT-051)", () => {
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
