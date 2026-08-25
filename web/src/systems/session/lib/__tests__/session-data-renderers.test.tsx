import { render, screen } from "@testing-library/react";
import { type ComponentProps } from "react";
import { describe, expect, it } from "vitest";

import type { AgentEventPayload } from "../../types";
import { SessionAgentReportedBlock } from "../../components/session-agent-reported-block";
import { CompozyEventDataRenderer } from "../session-data-renderers";
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

const reportedTerminalEvent: AgentEventPayload = {
  type: "terminal_output",
  origin: "agent_reported",
  session_id: SESSION_ID,
  turn_id: "turn-reported",
  title: "bun test",
  text: "$ bun test\n12 tests passed\n",
  reported_terminal: {
    id: "reported-terminal-1",
    cwd: "/workspace",
    total_bytes: 27,
    exit_code: 0,
  },
};

// The renderer only reads `data`; the assistant-ui prop bag is otherwise irrelevant here.
function EventRenderer({ data }: { data: AgentEventPayload }) {
  return (
    <CompozyEventDataRenderer
      {...({ data } as unknown as ComponentProps<typeof CompozyEventDataRenderer>)}
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

describe("CompozyEventDataRenderer", () => {
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

  it("Should render an explicitly labeled read-only reported block with zero controls", () => {
    render(
      <SessionAgentReportedBlock
        data={reportedTerminalEvent}
        engineLoader={() => new Promise(() => undefined)}
      />
    );

    expect(screen.getByText("reported by agent")).toBeInTheDocument();
    expect(
      screen.getByRole("log", { name: "Command output reported by the agent" })
    ).toHaveAttribute("data-readonly", "true");
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
    for (const control of ["Take control", "Open terminal", "Record", "Kill"]) {
      expect(screen.queryByRole("button", { name: control })).not.toBeInTheDocument();
    }
  });

  it("Should render no terminal chrome for an empty report", () => {
    const { container } = render(
      <SessionAgentReportedBlock
        data={{ ...reportedTerminalEvent, text: "" }}
        engineLoader={() => new Promise(() => undefined)}
      />
    );

    expect(container).toBeEmptyDOMElement();
  });
});
