import { describe, expect, it } from "vitest";

import { deriveTurnOutcomes } from "../session-turn-outcomes";

// Suite: turn outcomes across the loaded thread (task_07 US-022, US-027).
// Invariant: a turn's end is read from the daemon's own records wherever they
// were projected — the verified `session.turn_quiesced` receipt outranks the
// operator's cancel/interrupt markers — keyed by the turn id, never by message
// adjacency, and with the recording event's own time. Owning layer: pure lib.
function event(data: Record<string, unknown>) {
  return { type: "data-compozy-event", data };
}

describe("turn outcomes", () => {
  it("Should read a turn's end from a later projected message and prefer the verified receipt", () => {
    const outcomes = deriveTurnOutcomes([
      {
        role: "assistant",
        content: [{ type: "tool-Write", toolCallId: "t1", state: "input-streaming" }],
      },
      {
        role: "assistant",
        content: [
          event({
            type: "transcript_marker.created",
            turn_id: "turn-a",
            timestamp: "2026-09-06T10:25:58.936Z",
            marker: {
              kind: "transcript_marker.prompt_cancel",
              summary: "Prompt canceled by operator.",
              occurred_at: "2026-09-06T10:25:58.936Z",
            },
          }),
        ],
      },
      {
        role: "assistant",
        content: [
          event({
            type: "session.turn_quiesced",
            turn_id: "turn-a",
            timestamp: "2026-09-06T10:25:59.057Z",
            raw: {
              scope: "turn",
              turn_id: "turn-a",
              verified: true,
              escalated: false,
              phase: "cooperative",
              elapsed_ms: 98,
              stop_cause: "user_requested",
            },
          }),
        ],
      },
    ]);
    expect(outcomes.get("turn-a")).toEqual({
      turnId: "turn-a",
      kind: "interrupted",
      endedAtMs: Date.parse("2026-09-06T10:25:59.057Z"),
      verified: true,
    });
  });

  it("Should keep turns apart, read a session stop, and ignore status events", () => {
    const outcomes = deriveTurnOutcomes([
      {
        role: "assistant",
        content: [
          event({
            type: "usage",
            turn_id: "turn-b",
            timestamp: "2026-09-06T10:00:00Z",
            usage: { context_used: 1 },
          }),
          event({
            type: "session_stopped",
            turn_id: "turn-c",
            timestamp: "2026-09-06T10:01:00Z",
            stop_reason: "user_canceled",
          }),
          event({
            type: "transcript_marker.created",
            turn_id: "turn-d",
            timestamp: "2026-09-06T10:02:00Z",
            marker: {
              kind: "transcript_marker.prompt_interrupted",
              summary: "Replaced.",
              occurred_at: "2026-09-06T10:02:00Z",
            },
          }),
        ],
      },
      { role: "user", content: [{ type: "text", text: "next" }] },
    ]);
    expect(outcomes.has("turn-b")).toBe(false);
    expect(outcomes.get("turn-c")).toMatchObject({ kind: "stopped", verified: false });
    expect(outcomes.get("turn-d")).toMatchObject({
      kind: "interrupted",
      endedAtMs: Date.parse("2026-09-06T10:02:00Z"),
    });
    expect(deriveTurnOutcomes([]).size).toBe(0);
  });
});
