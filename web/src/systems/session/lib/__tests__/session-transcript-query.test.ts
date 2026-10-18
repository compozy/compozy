import { describe, expect, it } from "vitest";

import { flattenTranscriptMessages, type SessionTranscriptData } from "../session-transcript-query";
import type { SessionMessage, SessionTranscriptPage } from "../../types";

function message(id: string, parts: NonNullable<SessionMessage["parts"]>): SessionMessage {
  return { id, role: "assistant", parts };
}

function transcriptData(messages: SessionMessage[]): SessionTranscriptData {
  const entries = messages.map((item, index) => ({
    message: item,
    sequence: index + 1,
    start_sequence: index + 1,
  }));
  const page: SessionTranscriptPage = {
    cursor: entries.length,
    entries,
    epoch: 1,
    generation: 1,
    has_older: false,
    limit: 200,
    max_sequence: entries.length,
  };
  return { pages: [page], pageParams: [undefined] };
}

describe("session transcript presentation", () => {
  // Invariant: provider failure precedence is recomputed across the current
  // page set, including mixed prose, replaced parts, and removed diagnostics.
  // Owning layer: transcript projection; canonical suite: this file.
  it("Should reproject mixed failure content after immutable replacement and history removal", () => {
    const raw = {
      type: "data-compozy-event" as const,
      turnId: "turn-a",
      data: { type: "error", turn_id: "ignored", error: "disconnected" },
    };
    const text = { type: "text" as const, text: "Keep the useful answer" };
    const mixed = message("mixed", [text, raw]);
    const marker = message("marker", [
      {
        type: "data-compozy-event",
        data: {
          type: "transcript_marker.created",
          turn_id: "turn-a",
          marker: { kind: "transcript_marker.provider_failure" },
        },
      },
    ]);
    expect(flattenTranscriptMessages(transcriptData([mixed, marker]))).toEqual([
      { ...mixed, parts: [text] },
      marker,
    ]);
    expect(flattenTranscriptMessages(transcriptData([mixed]))).toEqual([mixed]);
    const replacement = message("mixed", [text]);
    expect(flattenTranscriptMessages(transcriptData([replacement, marker]))).toEqual([
      replacement,
      marker,
    ]);
    expect(flattenTranscriptMessages(transcriptData([]))).toEqual([]);
  });

  it("Should show one durable provider failure across separate transcript messages", () => {
    const rawError = message("error", [
      {
        type: "data-compozy-event",
        data: {
          type: "error",
          turn_id: "turn-failed",
          error: "peer disconnected before response",
        },
      },
    ]);
    const marker = message("marker", [
      {
        type: "data-compozy-event",
        data: {
          type: "transcript_marker.created",
          turn_id: "turn-failed",
          title: "transcript_marker.provider_failure",
          text: "peer disconnected before response",
        },
      },
    ]);

    expect(flattenTranscriptMessages(transcriptData([rawError, marker]))).toEqual([marker]);
    expect(flattenTranscriptMessages(transcriptData([rawError]))).toEqual([rawError]);
  });

  it("Should keep the actionable provider error over the redundant provider_failure marker", () => {
    const providerError = message("error", [
      {
        type: "data-compozy-event",
        data: {
          type: "error",
          turn_id: "turn-auth",
          error: "provider authentication required",
          failure: { kind: "prompt_failure", summary: "provider authentication required" },
          provider_error: {
            code: "provider_auth_required",
            provider: "claude-code",
            next_action: "login",
            guidance: "run provider auth login for this provider",
            occurrence_count: 1,
            first_seen_at: "2026-09-05T14:02:00Z",
            last_seen_at: "2026-09-05T14:02:00Z",
          },
        },
      },
    ]);
    const marker = message("marker", [
      {
        type: "data-compozy-event",
        data: {
          type: "transcript_marker.created",
          turn_id: "turn-auth",
          title: "transcript_marker.provider_failure",
          text: "provider authentication required",
        },
      },
    ]);
    const otherTurnMarker = message("marker-other", [
      {
        type: "data-compozy-event",
        data: {
          type: "transcript_marker.created",
          turn_id: "turn-other",
          title: "transcript_marker.provider_failure",
          text: "peer disconnected before response",
        },
      },
    ]);

    expect(
      flattenTranscriptMessages(transcriptData([providerError, marker, otherTurnMarker]))
    ).toEqual([providerError, otherTurnMarker]);
    expect(flattenTranscriptMessages(transcriptData([providerError]))).toEqual([providerError]);
  });
});
