// Suite: session-badge
// Invariant: one exported badge dictionary covers the daemon's twelve-token
// vocabulary exhaustively, is the only state-glyph/tone source for every
// attention surface, spells out the state word wherever a glyph is shared so no
// state reads as colour or shape alone, mirrors the daemon's attention classes,
// draws `waiting` from the daemon's `waiting_on`, and narrows unrecognized tokens
// to the honesty state.
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
import { sessionAttentionReason } from "../session-attention-reason";

/** Asks waiting on the operator: one attention glyph, told apart by their words. */
const WAITING_ON_YOU: SessionBadgeToken[] = [
  "waiting-for-input",
  "waiting-for-auth",
  "needs-attention",
];
/** The daemon's needs-you class (`session.ClassForBadge`). */
const NEEDS_YOU: SessionBadgeToken[] = [...WAITING_ON_YOU, "failed"];

describe("session badge dictionary", () => {
  it("Should cover the daemon's twelve-token vocabulary exhaustively (UT-050)", () => {
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
        "waiting",
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
    // Needs-you is the accent-orange attention dot (never the warning amber),
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

  it("Should draw waiting from the daemon's waiting_on with its word, never a new glyph (UT-075)", () => {
    const sessions = sessionBadgeSignal("waiting", "sessions");
    expect(sessions).toMatchObject({
      state: "delegated",
      label: "waiting",
      displayLabel: "Waiting on sessions",
      attention: "none",
      speaks: true,
    });
    const monitor = sessionBadgeSignal("waiting", "monitor");
    expect(monitor).toMatchObject({
      state: "monitoring",
      label: "waiting",
      displayLabel: "Monitoring",
      attention: "none",
      speaks: true,
    });
    // A missing reason keeps the sessions mark; waiting_on never alters another badge.
    expect(sessionBadgeSignal("waiting").state).toBe("delegated");
    expect(sessionBadgeSignal("done", "monitor").state).toBe("done");
    expect(isFinishedBadge("waiting")).toBe(false);
    expect(isNeedsYouBadge("waiting")).toBe(false);
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

describe("needs-attention reason (#760)", () => {
  const supervision = (
    sources: { kind: SourceKind; state: string; error?: string; ref?: string }[]
  ) => ({ quiet_warning: null, work_signals: [], sources });
  type SourceKind = "task_lease" | "awaiting_reply" | "loop_run" | "tool_running";

  it("Should name the cause the payload carries, explicit blockers first", () => {
    const reason = sessionAttentionReason(
      {
        attention: undefined,
        state: "active",
        supervision: supervision([
          { kind: "loop_run", state: "unknown", error: "source is not configured" },
          { kind: "task_lease", state: "stale", error: "task_lease evidence is stale" },
          { kind: "tool_running", state: "present" },
          { kind: "awaiting_reply", state: "attention", error: "Waiting on the reviewer" },
        ]),
      },
      "needs-attention"
    );
    expect(reason?.summary).toBe("Waiting on the reviewer (+2)");
    expect(reason?.detail.split("\n")).toEqual([
      "Waiting on the reviewer",
      "Task lease went stale: task_lease evidence is stale",
      "Can't check Loop run: source is not configured",
    ]);
  });

  it("Should name an unverified stop and stay silent for other badges or no cause", () => {
    const stop = {
      attention: "stop_verification_failed",
      state: "stopping" as const,
      supervision: null,
    };
    expect(sessionAttentionReason(stop, "needs-attention")?.summary).toBe("Stop not confirmed");
    expect(sessionAttentionReason(stop, "running")).toBeNull();
    expect(
      sessionAttentionReason({ state: "active", supervision: null }, "needs-attention")
    ).toBeNull();
  });

  it("Should name a child turn that ended without delivering (#1065)", () => {
    const early = { attention: "ended_early", state: "active" as const, supervision: null };
    const reason = sessionAttentionReason(early, "needs-attention");
    expect(reason?.summary).toBe("Stopped before delivering");
    expect(reason?.detail).toBe(
      "Stopped before delivering: no pull request, and nothing left running to resume it"
    );
    expect(sessionAttentionReason(early, "running")).toBeNull();
  });

  it("Should say a session is back from Later on its return date, whatever the badge (#1076)", () => {
    const back = {
      state: "stopped" as const,
      supervision: null,
      returned_from_later_at: "2026-10-30T09:00:00Z",
    };
    expect(sessionAttentionReason(back, "needs-attention")?.summary).toBe("Back from Later");
    expect(sessionAttentionReason(back, "idle")?.summary).toBe("Back from Later");
    expect(sessionAttentionReason({ ...back, returned_from_later_at: null }, "idle")).toBeNull();
  });
});
