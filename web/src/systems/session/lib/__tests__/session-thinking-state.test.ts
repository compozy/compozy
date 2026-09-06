import { describe, expect, it } from "vitest";

import {
  assistantMessageHasContent,
  deriveThinkingState,
  THINKING_FLICKER_GUARD_MS,
  thinkingGuardRemainingMs,
} from "../session-thinking-state";

// Suite: thinking-state derivation (ADR-006 rule 3, US-023, UT-095).
// Invariant: the thinking row exists only while a turn runs without content;
// the reply (and its actions) exists only once content does; a first token
// inside the guard skips the thinking frame; an error before content converts
// the pending reply into its error state.
describe("thinking state", () => {
  const sentAt = 1_000;

  it("Should hold the thinking frame back inside the flicker guard, then show it", () => {
    const base = { running: true, hasContent: false, failed: false, sentAtMs: sentAt };
    expect(deriveThinkingState({ ...base, nowMs: sentAt + 50 })).toBe("pending");
    expect(deriveThinkingState({ ...base, nowMs: sentAt + THINKING_FLICKER_GUARD_MS })).toBe(
      "thinking"
    );
    expect(thinkingGuardRemainingMs(sentAt, sentAt + 100)).toBe(THINKING_FLICKER_GUARD_MS - 100);
    expect(thinkingGuardRemainingMs(sentAt, sentAt + 900)).toBe(0);
    expect(thinkingGuardRemainingMs(null, sentAt)).toBe(0);
  });

  it("Should skip the thinking frame when the first token beats the guard", () => {
    expect(
      deriveThinkingState({
        running: true,
        hasContent: true,
        failed: false,
        sentAtMs: sentAt,
        nowMs: sentAt + 40,
      })
    ).toBe("content");
  });

  it("Should convert a failure before any content into the error state", () => {
    expect(
      deriveThinkingState({
        running: false,
        hasContent: false,
        failed: true,
        sentAtMs: sentAt,
        nowMs: sentAt + 5_000,
      })
    ).toBe("error");
    expect(
      deriveThinkingState({
        running: false,
        hasContent: false,
        failed: false,
        sentAtMs: null,
        nowMs: sentAt,
      })
    ).toBe("none");
  });

  it("Should count only real content — tokens, tools, rich rows — as a reply", () => {
    expect(assistantMessageHasContent([])).toBe(false);
    expect(assistantMessageHasContent([{ type: "text", text: "" }])).toBe(false);
    expect(assistantMessageHasContent([{ type: "text", text: "Hi" }])).toBe(true);
    expect(assistantMessageHasContent([{ type: "reasoning", text: "…" }])).toBe(true);
    expect(
      assistantMessageHasContent([{ type: "tool-call", toolCallId: "t1", toolName: "Bash" }])
    ).toBe(true);
    expect(
      assistantMessageHasContent([
        {
          type: "data-compozy-event",
          data: { type: "prompt_started", text: "Prompt started" },
        },
      ])
    ).toBe(false);
    expect(
      assistantMessageHasContent([
        {
          type: "data-compozy-event",
          data: { type: "error", error: "provider exploded" },
        },
      ])
    ).toBe(true);
    expect(
      assistantMessageHasContent([
        { type: "data-compozy-permission", data: { request_id: "req_1", type: "permission" } },
      ])
    ).toBe(true);
    // A steer marker is content: its provenance line or receipt bubble renders.
    expect(
      assistantMessageHasContent([
        {
          type: "data-compozy-event",
          data: {
            type: "transcript_marker.created",
            marker: { kind: "transcript_marker.prompt_steered", summary: "s", occurred_at: "t" },
          },
        },
      ])
    ).toBe(true);
  });
});
