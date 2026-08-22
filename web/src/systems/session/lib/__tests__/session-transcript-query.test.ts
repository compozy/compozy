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
});
