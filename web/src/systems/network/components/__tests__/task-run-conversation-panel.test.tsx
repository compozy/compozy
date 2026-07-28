import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";

import type { NetworkConversationMessage, TaskRunNetworkUsage } from "../../types";
import { TaskRunConversationPanel } from "../task-run-conversation-panel";

const usage: TaskRunNetworkUsage = {
  workspace_id: "ws-fixture",
  details: [],
  total: {
    wake_count: 4,
    reserved_wake_count: 1,
    actual_wake_count: 2,
    unavailable_wake_count: 1,
    charged_wall_time: "1s",
    input_tokens: 10,
    output_tokens: 4,
  },
};

function message(id: string, text: string): NetworkConversationMessage {
  return {
    message_id: id,
    channel: "coord-run-1",
    surface: "thread",
    thread_id: "thread_agent_channel",
    kind: "say",
    direction: "sent",
    peer_from: "sess-1",
    body: { text },
    text,
    timestamp: "2026-07-15T08:00:00Z",
  };
}

describe("TaskRunConversationPanel", () => {
  it("Should explain empty conversation silence with truthful run usage", () => {
    render(
      <TaskRunConversationPanel boundsLabel="Bounds: 0/4 wakes" messages={[]} usage={usage} />
    );
    expect(screen.getByTestId("tasks-run-conversation-empty")).toHaveTextContent(
      /Silence is normal/i
    );
    expect(screen.getByTestId("tasks-run-usage-summary")).toHaveTextContent(
      "Run usage: 2 actual · 1 reserved · 1 unavailable · 10 charged in / 4 charged out"
    );
  });

  it("Should render durable messages and keep the run view interactive while paginating", () => {
    const onLoadMore = vi.fn();
    render(
      <TaskRunConversationPanel
        hasMoreMessages
        messages={[message("msg-1", "First update"), message("msg-2", "Second update")]}
        onLoadMore={onLoadMore}
        usage={usage}
      />
    );
    fireEvent.click(screen.getByRole("button", { name: "Load older messages" }));
    expect(onLoadMore).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId("tasks-run-conversation-summary")).toHaveTextContent("2 messages");
    expect(screen.getByRole("log", { name: "Run coordination messages" })).toHaveTextContent(
      "Second update"
    );
  });

  it("Should disclose SSE reconnects without hiding paginated history", () => {
    render(
      <TaskRunConversationPanel
        messages={[message("msg-1", "Durable update")]}
        streamError={new Error("stream disconnected")}
        usage={usage}
      />
    );
    expect(screen.getByTestId("tasks-run-conversation-stream-status")).toHaveTextContent(
      /paginated refresh remains active/i
    );
    expect(screen.getByRole("log")).toHaveTextContent("Durable update");
  });
});
