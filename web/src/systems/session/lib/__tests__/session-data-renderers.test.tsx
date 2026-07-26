import { render, screen } from "@testing-library/react";
import { type ComponentProps } from "react";
import { describe, expect, it } from "vitest";

import type { AgentEventPayload } from "../../types";
import { AghEventDataRenderer } from "../session-data-renderers";
import { SessionRuntimeRenderProvider } from "../session-runtime-render-context";

const WORKSPACE_ID = "ws_alpha";
const SESSION_ID = "sess-001";

function clarifyResolved(): AgentEventPayload {
  return {
    type: "clarify",
    session_id: SESSION_ID,
    turn_id: "clarify:req-1",
    raw: {
      status: "resolved",
      request: {
        request_id: "req-1",
        workspace_id: WORKSPACE_ID,
        session_id: SESSION_ID,
        agent_name: "mock",
        question: "Which path?",
        choices: ["Fast", "Safe"],
        asked_at: "2026-07-16T00:00:00Z",
        deadline: "2026-07-16T00:05:00Z",
      },
      answer: { choice: 0, text: "", fallback: false },
      at: "2026-07-16T00:00:10Z",
    },
  };
}

const runtimeEvent: AgentEventPayload = {
  type: "runtime_progress",
  text: "Still working",
  runtime: { turn_id: "turn-1", elapsed_ms: 1_000, elapsed_seconds: 1 },
};

// The renderer only reads `data`; the assistant-ui prop bag is otherwise irrelevant here.
function EventRenderer({ data }: { data: AgentEventPayload }) {
  return (
    <AghEventDataRenderer
      {...({ data } as unknown as ComponentProps<typeof AghEventDataRenderer>)}
    />
  );
}

function renderWithContext(data: AgentEventPayload) {
  render(
    <SessionRuntimeRenderProvider sessionId={SESSION_ID} workspaceId={WORKSPACE_ID}>
      <EventRenderer data={data} />
    </SessionRuntimeRenderProvider>
  );
}

describe("AghEventDataRenderer", () => {
  it("Should route a clarify event to the clarification UI", () => {
    renderWithContext(clarifyResolved());

    expect(screen.getByTestId("clarification-receipt")).toHaveAttribute("data-status", "resolved");
    expect(screen.queryByTestId("runtime-activity-notice")).not.toBeInTheDocument();
  });

  it("Should keep ordinary runtime events on the activity-notice renderer", () => {
    renderWithContext(runtimeEvent);

    expect(screen.getByTestId("runtime-activity-notice")).toHaveTextContent("Still working");
    expect(screen.queryByTestId("clarification-receipt")).not.toBeInTheDocument();
  });

  it("Should render ordinary events without a render-context provider", () => {
    render(<EventRenderer data={runtimeEvent} />);

    expect(screen.getByTestId("runtime-activity-notice")).toBeInTheDocument();
  });
});
