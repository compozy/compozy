import { describe, expect, it } from "vitest";

import { activeReplyHasContent, lastSettledTurn } from "../session-thread-status.logic";

// Suite: thread status derivation for the S3 row (US-027, US-009.EC-2, US-014.EC-2).
// Invariant: the frozen sentence after a turn comes from the daemon's own
// records — the transcript's stop reasons and events plus the session's stop
// facts — and never labels a daemon stop (inactivity, escalation, failure) as
// the operator's. Durations span the turn's recorded instants; the inactivity
// span is the supervision episode's actual timestamps.
const START = "2026-07-07T12:00:00Z";
const END = "2026-07-07T12:01:40Z";

function assistant(parts: unknown[], status?: unknown) {
  return { id: "a1", role: "assistant", content: parts, status };
}

function event(data: Record<string, unknown>, timestamp = END) {
  return { type: "data-compozy-event", data: { ...data, timestamp }, timestamp };
}

const text = { type: "text", text: "Looking at the store…", state: "done", timestamp: START };

describe("thread status derivation", () => {
  it("Should read a stop reason on the daemon's event as the operator's stop", () => {
    const turn = lastSettledTurn([
      assistant([text, event({ type: "prompt_interrupted", stop_reason: "cancelled" })]),
    ]);
    expect(turn).toEqual({
      startedAtMs: Date.parse(START),
      endedAtMs: Date.parse(END),
      cause: "stopped",
      failureCause: null,
      stop: { kind: "user" },
    });
  });

  it("Should read an inactivity stop with the episode's actual quiet span, never as by you", () => {
    const quietSince = "2026-07-07T11:20:00Z";
    const turn = lastSettledTurn(
      [
        assistant([
          text,
          event({ type: "session_stopped", stop_reason: "timeout" }, "2026-07-07T12:00:30Z"),
          event(
            {
              type: "session.supervision_stopped",
              raw: {
                cause: "inactivity",
                supervision: {
                  quiet_warning: {
                    quiet_since: quietSince,
                    warned_at: "2026-07-07T11:50:00Z",
                    stop_at: END,
                  },
                  work_signals: [],
                  sources: [],
                },
              },
            },
            END
          ),
        ]),
      ],
      { state: "stopped", stop_cause: "inactivity", stop_reason: "timeout" }
    );
    expect(turn).toMatchObject({
      cause: "stopped",
      stop: { kind: "inactivity", noWorkMs: Date.parse(END) - Date.parse(quietSince) },
    });
  });

  it("Should read an escalated and verified session stop as closed for you, and other causes with their detail", () => {
    const messages = [
      assistant([text, event({ type: "session_stopped", stop_reason: "user_canceled" })]),
    ];
    expect(
      lastSettledTurn(messages, {
        state: "stopped",
        stop_cause: "user_requested",
        escalated: true,
        verified: true,
      })
    ).toMatchObject({ cause: "stopped", stop: { kind: "escalated" } });
    // Escalated but unverified is not a verified close: the operator's request stands.
    expect(
      lastSettledTurn(messages, {
        state: "stopped",
        stop_cause: "user_requested",
        escalated: true,
        verified: false,
      })
    ).toMatchObject({ cause: "stopped", stop: { kind: "user" } });
    expect(
      lastSettledTurn(messages, {
        state: "stopped",
        stop_cause: "shutdown",
        stop_reason: "shutdown",
        stop_detail: "daemon shutdown",
      })
    ).toMatchObject({ cause: "stopped", stop: { kind: "other", detail: "daemon shutdown" } });
    expect(
      lastSettledTurn(
        [assistant([text, event({ type: "session_stopped", stop_reason: "agent_crashed" })])],
        {
          state: "stopped",
          stop_reason: "agent_crashed",
        }
      )
    ).toMatchObject({ cause: "failed", failureCause: "agent crashed" });
  });

  // Invariant (US-027.EC-2, turn scope): only the durable `session.turn_quiesced`
  // receipt — persisted after verified quiescence — may read "closed for you";
  // an attempted `session.stop_escalated` never does, and a receipt for another
  // turn is not this turn's fact.
  it("Should read a verified escalated turn receipt as closed for you, never an attempted escalation", () => {
    const receipt = (extra: Record<string, unknown>) =>
      event({
        type: "session.turn_quiesced",
        turn_id: "a1",
        raw: {
          scope: "turn",
          turn_id: "a1",
          verified: true,
          phase: "forced",
          elapsed_ms: 14_000,
          stop_cause: "user_requested",
          ...extra,
        },
      });
    const attempt = event(
      {
        type: "session.stop_escalated",
        turn_id: "a1",
        raw: { scope: "turn", turn_id: "a1", phase: "forced", elapsed_ms: 10_000, cause: 3 },
      },
      "2026-07-07T12:01:30Z"
    );
    const cancel = event(
      { type: "prompt_cancel", stop_reason: "cancelled" },
      "2026-07-07T12:00:30Z"
    );

    expect(
      lastSettledTurn([assistant([text, cancel, attempt, receipt({ escalated: true })])])
    ).toMatchObject({
      cause: "stopped",
      stop: { kind: "escalated" },
    });
    // Cooperative: the receipt says not escalated — the operator's plain stop.
    expect(
      lastSettledTurn([
        assistant([text, cancel, receipt({ escalated: false, phase: "cooperative" })]),
      ])
    ).toMatchObject({
      stop: { kind: "user" },
    });
    // An attempted escalation with no receipt is not a verified close.
    expect(lastSettledTurn([assistant([text, cancel, attempt])])).toMatchObject({
      stop: { kind: "user" },
    });
    // A receipt for another turn (replayed history) is ignored; the latest one for this turn wins.
    expect(
      lastSettledTurn([
        assistant([
          text,
          cancel,
          receipt({ escalated: true, turn_id: "older-turn" }),
          receipt({ escalated: false, phase: "cooperative" }),
        ]),
      ])
    ).toMatchObject({ stop: { kind: "user" } });
    // A receipt whose turn is still running (newer live work) is not read.
    expect(
      lastSettledTurn([assistant([text, receipt({ escalated: true })], { type: "running" })])
    ).toBeNull();
  });

  it("Should span every trailing message the daemon projected for the last turn", () => {
    // The daemon splits one turn into segments: the calls in one message, the
    // receipt in a later single-event message. The duration and the cause read
    // the whole turn, not the last segment alone.
    const segment = {
      id: "turn-x-2",
      role: "assistant",
      content: [
        {
          type: "text",
          text: "Writing drafts…",
          state: "done",
          turnId: "turn-x",
          timestamp: START,
        },
        {
          type: "tool-call",
          toolCallId: "t1",
          toolName: "Write",
          args: {},
          turnId: "turn-x",
          timestamp: "2026-07-07T12:00:30Z",
        },
      ],
    };
    const receipt = {
      id: "turn-x-4",
      role: "assistant",
      content: [
        event({
          type: "session.turn_quiesced",
          turn_id: "turn-x",
          raw: {
            scope: "turn",
            turn_id: "turn-x",
            verified: true,
            escalated: false,
            phase: "cooperative",
            elapsed_ms: 98,
            stop_cause: "user_requested",
          },
        }),
      ],
    };
    expect(lastSettledTurn([segment, receipt])).toMatchObject({
      startedAtMs: Date.parse(START),
      endedAtMs: Date.parse(END),
      cause: "stopped",
      stop: { kind: "user" },
    });
    // A different turn's receipt does not reach back into an unrelated segment.
    const other = {
      ...receipt,
      content: [
        event({
          type: "session.turn_quiesced",
          turn_id: "turn-y",
          raw: { scope: "turn", turn_id: "turn-y", verified: true, escalated: false },
        }),
      ],
    };
    expect(lastSettledTurn([segment, other])?.startedAtMs).toBe(Date.parse(END));
  });

  // Invariant (US-014.EC-2 / US-027.EC-2, session scope): the daemon persists a
  // session stop as a synthetic terminal group after the authored turn
  // (`session.stop_escalated`, `session_stopped`, `session.supervision_stopped`,
  // the timeout marker) under its own turn id. The frozen sentence belongs to the
  // preceding authored turn: its span runs from the operator's recorded send to
  // the stop, the cause is supervision's, and the quiet span is the latest
  // recorded episode's — never a wall clock, never a fabricated span, never a
  // failure. Fixture: the lab's `sess-b36e351dbe08b6f6` public transcript.
  it("Should attribute a persisted session stop to the preceding authored turn with its real span and quiet episode", () => {
    const supervision = (quietWarning: Record<string, unknown> | null, timestamp: string) => ({
      raw: {
        cause: "inactivity",
        supervision: { work_signals: [], sources: [], quiet_warning: quietWarning },
      },
      timestamp,
    });
    const authored = {
      id: "turn-31933c88c2569cc7",
      role: "assistant",
      content: [
        {
          type: "text",
          text: "Waiting for the next recovery signal.",
          state: "done",
          turnId: "turn-31933c88c2569cc7",
          timestamp: "2026-09-06T13:54:19.892384Z",
        },
        event(
          {
            type: "session.supervision_warning",
            turn_id: "turn-31933c88c2569cc7",
            ...supervision(
              {
                quiet_since: "2026-09-06T13:54:22.429136Z",
                warned_at: "2026-09-06T13:54:33.432267Z",
                stop_at: "2026-09-06T13:55:03.432267Z",
              },
              "2026-09-06T13:54:33.432267Z"
            ),
          },
          "2026-09-06T13:54:33.432267Z"
        ),
        {
          type: "text",
          text: " Fresh recovery progress arrived; the previous quiet warning should clear.",
          state: "done",
          turnId: "turn-31933c88c2569cc7",
          timestamp: "2026-09-06T13:54:44.897769Z",
        },
        event(
          {
            type: "session.supervision_warning",
            turn_id: "turn-31933c88c2569cc7",
            ...supervision(
              {
                quiet_since: "2026-09-06T13:54:47.417985Z",
                warned_at: "2026-09-06T13:54:57.42158Z",
                stop_at: "2026-09-06T13:55:27.42158Z",
              },
              "2026-09-06T13:54:57.42158Z"
            ),
          },
          "2026-09-06T13:54:57.42158Z"
        ),
      ],
    };
    const stopGroup = {
      id: "turn-08077e10a145eb4a",
      role: "assistant",
      content: [
        event(
          {
            type: "session.stop_escalated",
            turn_id: "turn-08077e10a145eb4a",
            raw: { scope: "session", phase: "forced", elapsed_ms: 81, cause: 11 },
          },
          "2026-09-06T13:55:28.525346Z"
        ),
        event(
          { type: "session_stopped", turn_id: "turn-08077e10a145eb4a", stop_reason: "timeout" },
          "2026-09-06T13:55:28.539506Z"
        ),
        event(
          {
            type: "session.supervision_stopped",
            turn_id: "turn-08077e10a145eb4a",
            ...supervision(null, "2026-09-06T13:55:28.54977Z"),
          },
          "2026-09-06T13:55:28.54977Z"
        ),
      ],
    };
    const marker = {
      id: "ev-497cf6fef2db95fc",
      role: "assistant",
      content: [
        event(
          {
            type: "transcript_marker.created",
            turn_id: "turn-08077e10a145eb4a",
            marker: {
              kind: "transcript_marker.prompt_timeout",
              summary: "Session timed out.",
              occurred_at: "2026-09-06T13:55:28.55237Z",
              evidence: { event_type: "session_stopped", failure_kind: "", stop_reason: "timeout" },
            },
          },
          "2026-09-06T13:55:28.55237Z"
        ),
      ],
    };
    const prompt = {
      id: "msg_quiet_episode",
      role: "user",
      content: [{ type: "text", text: "Observe the quiet recovery episode." }],
      metadata: {
        custom: {
          turn_id: "turn-31933c88c2569cc7",
          timestamp: "2026-09-06T13:54:19.865928Z",
          message_id: "msg_quiet_episode",
        },
      },
    };
    const facts = {
      state: "stopped" as const,
      stop_reason: "timeout",
      stop_cause: "inactivity",
      stop_detail: "inactivity",
      verified: true,
      escalated: true,
    };
    const turn = lastSettledTurn([prompt, authored, stopGroup, marker], facts);
    expect(turn).toEqual({
      startedAtMs: Date.parse("2026-09-06T13:54:19.865928Z"),
      endedAtMs: Date.parse("2026-09-06T13:55:28.55237Z"),
      cause: "stopped",
      failureCause: null,
      stop: {
        kind: "inactivity",
        noWorkMs:
          Date.parse("2026-09-06T13:55:28.54977Z") - Date.parse("2026-09-06T13:54:47.417985Z"),
      },
    });
    // The resource lagging behind (no stop cause yet, escalated + verified) still reads supervision's cause from the transcript.
    expect(
      lastSettledTurn([prompt, authored, stopGroup, marker], {
        state: "stopped",
        escalated: true,
        verified: true,
      })
    ).toMatchObject({ cause: "stopped", stop: { kind: "inactivity" } });
    // The live tail carries the same stop as an `error` event with the stop detail as its text: not a failure.
    const liveError = {
      id: "session-stopped-sess-b36e351dbe08b6f6",
      role: "assistant",
      content: [
        event(
          { type: "error", stop_reason: "timeout", error: "inactivity" },
          "2026-09-06T13:55:28.539506Z"
        ),
      ],
      status: { type: "incomplete", reason: "error" },
    };
    expect(lastSettledTurn([prompt, authored, stopGroup, marker, liveError], facts)).toMatchObject({
      cause: "stopped",
      stop: { kind: "inactivity" },
      startedAtMs: Date.parse("2026-09-06T13:54:19.865928Z"),
    });
    // A failure the daemon recorded still outranks the stop.
    const crashed = {
      ...stopGroup,
      content: [
        event(
          {
            type: "session_stopped",
            turn_id: "turn-08077e10a145eb4a",
            stop_reason: "agent_crashed",
          },
          "2026-09-06T13:55:28.539506Z"
        ),
      ],
    };
    expect(
      lastSettledTurn([prompt, authored, crashed], { ...facts, stop_reason: "agent_crashed" })
    ).toMatchObject({
      cause: "failed",
      failureCause: "agent crashed",
    });
    // No episode recorded anywhere: the span is not invented.
    const bare = { ...authored, content: [authored.content[0]!] };
    expect(lastSettledTurn([prompt, bare, stopGroup], facts)).toMatchObject({
      stop: { kind: "inactivity", noWorkMs: null },
    });
  });

  it("Should read a completed turn, a running one, and content presence", () => {
    const completed = lastSettledTurn([assistant([text, { ...text, timestamp: END }])]);
    expect(completed?.cause).toBe("completed");
    expect(completed?.stop).toBeUndefined();
    expect(lastSettledTurn([assistant([text], { type: "running" })])).toBeNull();
    expect(lastSettledTurn([{ id: "u", role: "user", content: [text] }])).toBeNull();
    expect(activeReplyHasContent([assistant([text], { type: "running" })])).toBe(true);
    expect(activeReplyHasContent([assistant([], { type: "running" })])).toBe(false);
    // An active session never reads its stale stop facts.
    expect(
      lastSettledTurn([assistant([text, { ...text, timestamp: END }])], {
        state: "active",
        stop_cause: "inactivity",
      })
    ).toMatchObject({ cause: "completed" });
  });
});
