import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { RunFeedEvent } from "../lib/event-store";
import type { RunTranscript } from "../types";
import { RunTranscriptPanel } from "./run-transcript-panel";

function buildLiveEvent(seq: number, update: Record<string, unknown>): RunFeedEvent {
  return {
    id: `event-${seq}`,
    seq,
    kind: "session.update",
    runId: "run-1",
    timestamp: "2026-01-01T00:00:00Z",
    payload: { update },
    receivedAt: seq,
  };
}

describe("RunTranscriptPanel", () => {
  it("Should render backend error text when a failed tool has no structured output", async () => {
    const transcript: RunTranscript = {
      run_id: "run-1",
      messages: [
        {
          id: "msg-1",
          role: "assistant",
          parts: [
            {
              type: "dynamic-tool",
              toolCallId: "tool-1",
              toolName: "Bash",
              state: "output-error",
              input: { command: "make verify" },
              errorText: "verification failed",
            },
          ],
        },
      ],
    };

    render(<RunTranscriptPanel transcript={transcript} />);

    expect(await screen.findByTestId("run-transcript-tool-tool-1")).toHaveTextContent("failed");
    expect(screen.getByTestId("run-transcript-tool-tool-1")).toHaveTextContent(
      "verification failed"
    );
  });

  it("Should replace earlier live tool messages with later updates for the same call", async () => {
    const liveEvents = [
      buildLiveEvent(1, {
        kind: "tool_call_started",
        tool_call_id: "tool-1",
        tool_call_state: "in_progress",
        blocks: [
          {
            type: "tool_use",
            name: "Bash",
            input: { command: "echo ok" },
          },
        ],
      }),
      buildLiveEvent(2, {
        kind: "tool_call_updated",
        tool_call_id: "tool-1",
        tool_call_state: "completed",
        blocks: [{ type: "tool_result", content: "ok" }],
      }),
    ];

    render(<RunTranscriptPanel liveEvents={liveEvents} />);

    const toolCard = await screen.findByTestId("run-transcript-tool-tool-1");
    expect(toolCard).toHaveTextContent("done");
    expect(toolCard).not.toHaveTextContent("running");
  });
});
