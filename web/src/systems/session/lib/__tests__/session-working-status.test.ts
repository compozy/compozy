import { describe, expect, it } from "vitest";

import {
  agentCountLabel,
  deriveWorkingStatus,
  formatFrozenDuration,
  formatWorkingElapsed,
  stopAttributionDetail,
} from "../session-working-status";

// Suite: working-status derivation (S3, US-027, UT-109, UT-110).
// Invariant: the status row reads "Working for {elapsed} · {activity}" from the
// daemon's durable turn start and its current tool / decision / children, stays
// working while children run, and freezes into a truthful sentence after a stop
// or failure. Elapsed derives from `turn_started_at`, never a browser stopwatch.
const NOW = Date.parse("2026-09-06T12:02:14Z");

function session(overrides: Partial<Parameters<typeof deriveWorkingStatus>[0]["session"]> = {}) {
  return {
    activity: {
      current_tool: "Bash",
      elapsed_ms: 0,
      elapsed_seconds: 0,
      idle_seconds: 0,
      iteration_current: 1,
      iteration_max: 1,
      turn_id: "turn_1",
      turn_started_at: "2026-09-06T12:00:00Z",
    },
    pending_interactions: [],
    supervision: null,
    ...overrides,
  } as Parameters<typeof deriveWorkingStatus>[0]["session"];
}

describe("working status", () => {
  it("Should derive Working for {elapsed} · Running {tool} from the session payload", () => {
    expect(
      deriveWorkingStatus({
        session: session(),
        running: true,
        thinking: false,
        lastTurn: null,
        nowMs: NOW,
      })
    ).toEqual({
      kind: "working",
      startedAtMs: Date.parse("2026-09-06T12:00:00Z"),
      elapsed: "2m 14s",
      activity: "Running shell",
      agentCount: 0,
    });
  });

  it("Should count parallel tools honestly and prefer a pending decision", () => {
    const parallel = session({
      supervision: {
        quiet_warning: null,
        sources: [],
        work_signals: [
          { kind: "tool_running", since: "2026-09-06T12:01:00Z" },
          { kind: "tool_running", since: "2026-09-06T12:01:01Z" },
          { kind: "tool_running", since: "2026-09-06T12:01:02Z" },
        ],
      },
    });
    const status = deriveWorkingStatus({
      session: parallel,
      running: true,
      thinking: false,
      lastTurn: null,
      nowMs: NOW,
    });
    expect(status).toMatchObject({ kind: "working", activity: "Running 3 tools" });

    const waiting = session({
      pending_interactions: [{ kind: "permission" }] as never,
    });
    expect(
      deriveWorkingStatus({
        session: waiting,
        running: true,
        thinking: false,
        lastTurn: null,
        nowMs: NOW,
      })
    ).toMatchObject({ activity: "Waiting for your decision" });
  });

  it("Should stay working with the children count while spawned agents run", () => {
    const children = session({
      activity: { ...session().activity!, current_tool: undefined },
      supervision: {
        quiet_warning: null,
        sources: [],
        work_signals: [
          { kind: "active_child", since: "2026-09-06T12:01:00Z", ref: "sess_child_1" },
          { kind: "active_child", since: "2026-09-06T12:01:30Z", ref: "sess_child_2" },
        ],
      },
    });
    expect(
      deriveWorkingStatus({
        session: children,
        running: true,
        thinking: false,
        lastTurn: null,
        nowMs: NOW,
      })
    ).toMatchObject({ kind: "working", activity: null, agentCount: 2 });
    expect(agentCountLabel(1)).toBe("1 agent running");
    expect(agentCountLabel(2)).toBe("2 agents running");
  });

  it("Should read Thinking before content and freeze into a stop or failure after the turn", () => {
    expect(
      deriveWorkingStatus({
        session: session(),
        running: true,
        thinking: true,
        lastTurn: null,
        nowMs: NOW,
      })
    ).toEqual({ kind: "thinking" });
    const stopped = {
      startedAtMs: Date.parse("2026-09-06T12:00:00Z"),
      endedAtMs: Date.parse("2026-09-06T12:01:40Z"),
      cause: "stopped" as const,
      failureCause: null,
    };
    expect(
      deriveWorkingStatus({
        session: session(),
        running: false,
        thinking: false,
        lastTurn: stopped,
        nowMs: NOW,
      })
    ).toEqual({ kind: "stopped", duration: "1m 40s", byYou: true, detail: null });
    // Only the operator's own stop reads "by you"; the daemon's stops say what they were.
    expect(
      deriveWorkingStatus({
        session: session(),
        running: false,
        thinking: false,
        lastTurn: { ...stopped, stop: { kind: "inactivity", noWorkMs: 40 * 60_000 } },
        nowMs: NOW,
      })
    ).toEqual({
      kind: "stopped",
      duration: "1m 40s",
      byYou: false,
      detail: "no work for 40 minutes",
    });
    expect(stopAttributionDetail({ kind: "escalated" })).toBe(
      "the agent didn't answer the stop, so it was closed for you"
    );
    expect(stopAttributionDetail({ kind: "inactivity", noWorkMs: null })).toBe(
      "no work signal from the agent"
    );
    expect(stopAttributionDetail({ kind: "other", detail: "daemon shutdown" })).toBe(
      "daemon shutdown"
    );
    expect(stopAttributionDetail({ kind: "user" })).toBeNull();
    expect(
      deriveWorkingStatus({
        session: session(),
        running: false,
        thinking: false,
        lastTurn: { ...stopped, cause: "failed", failureCause: "rate limited" },
        nowMs: NOW,
      })
    ).toEqual({ kind: "failed", duration: "1m 40s", cause: "rate limited" });
    expect(
      deriveWorkingStatus({
        session: session(),
        running: false,
        thinking: false,
        lastTurn: { ...stopped, cause: "completed" },
        nowMs: NOW,
      })
    ).toEqual({ kind: "hidden" });
  });

  it("Should derive elapsed from the durable turn start so a remount recomputes instead of resetting", () => {
    const started = Date.parse("2026-09-06T12:00:00Z");
    expect(formatWorkingElapsed(started, started + 12 * 60_000 + 4_000)).toBe("12m 4s");
    expect(formatWorkingElapsed(started, started + 3_600_000 + 120_000)).toBe("1h 2m");
    expect(formatWorkingElapsed(started, started - 5_000)).toBe("0s");
    expect(formatFrozenDuration(0)).toBe("1s");
    // The same payload read at two different instants yields the same start.
    const first = deriveWorkingStatus({
      session: session(),
      running: true,
      thinking: false,
      lastTurn: null,
      nowMs: NOW,
    });
    const later = deriveWorkingStatus({
      session: session(),
      running: true,
      thinking: false,
      lastTurn: null,
      nowMs: NOW + 9_000,
    });
    expect(
      first.kind === "working" &&
        later.kind === "working" &&
        first.startedAtMs === later.startedAtMs
    ).toBe(true);
    expect(later).toMatchObject({ elapsed: "2m 23s" });
  });
});
