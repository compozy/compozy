import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { type ComponentProps } from "react";
import { describe, expect, it } from "vitest";

import type { TerminalEngine } from "@compozy/ui";

import { stubEngineLoader } from "@/systems/terminal/components/__tests__/terminal-window-harness";
import { terminalReplayFailedCopy } from "@/systems/terminal/lib/terminal-copy";
import type { AgentEventPayload } from "../../types";
import { SessionAgentReportedBlock } from "../../components/session-agent-reported-block";
import { CompozyEventDataRenderer } from "../session-data-renderers";
import { SessionRuntimeRenderProvider } from "../session-runtime-render-context";

function stubThrowingEngineLoader(error: Error) {
  return async (): Promise<TerminalEngine> => {
    const engine = await stubEngineLoader();
    return {
      ...engine,
      createTerminal: options => {
        const terminal = engine.createTerminal(options);
        return Object.assign(terminal, {
          write: () => {
            throw error;
          },
        });
      },
    };
  };
}

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
    expect(screen.getByText("bun test")).toBeInTheDocument();
    expect(screen.queryByText("Command output")).not.toBeInTheDocument();
    expect(screen.getByRole("log", { name: "bun test — reported by the agent" })).toHaveAttribute(
      "data-readonly",
      "true"
    );
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
    for (const control of ["Take control", "Open terminal", "Record", "Kill"]) {
      expect(screen.queryByRole("button", { name: control })).not.toBeInTheDocument();
    }
  });

  it("Should route an agent-reported terminal event through the reported block", () => {
    renderWithContext(reportedTerminalEvent);

    expect(
      screen.getByTestId("session-agent-reported-block-reported-terminal-1")
    ).toBeInTheDocument();
  });

  it("Should collapse long agent-reported output behind a line count control", async () => {
    const user = userEvent.setup();
    const lines = Array.from({ length: 20 }, (_, index) => `line ${index + 1}`);
    render(
      <SessionAgentReportedBlock
        data={{
          ...reportedTerminalEvent,
          text: `${lines.join("\n")}\n`,
        }}
        engineLoader={() => new Promise(() => undefined)}
      />
    );

    const more = screen.getByRole("button", { name: /show \d+ more lines/ });
    expect(more).toHaveAttribute("aria-expanded", "false");
    expect(
      screen.getByRole("log", { name: "bun test — reported by the agent — collapsed" })
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Open terminal" })).not.toBeInTheDocument();

    await user.click(more);
    expect(more).toHaveAttribute("aria-expanded", "true");
    expect(more).toHaveTextContent("show fewer lines");
    expect(
      screen.getByRole("log", { name: "bun test — reported by the agent" })
    ).toBeInTheDocument();
  });

  it("Should keep an untitled report as the provenance mark only", () => {
    render(
      <SessionAgentReportedBlock
        data={{ ...reportedTerminalEvent, title: "" }}
        engineLoader={() => new Promise(() => undefined)}
      />
    );

    expect(screen.getByText("reported by agent")).toBeInTheDocument();
    expect(screen.queryByText("Command output")).not.toBeInTheDocument();
    expect(screen.getByRole("log", { name: "Reported by the agent" })).toBeInTheDocument();
  });

  it("Should state the reported total when the specimen is truncated", () => {
    render(
      <SessionAgentReportedBlock
        data={{
          ...reportedTerminalEvent,
          text: "Last output lines remain visible.\n",
          reported_terminal: {
            id: "reported-terminal-1",
            cwd: "/workspace",
            total_bytes: 163_750,
            truncated: true,
            exit_code: 0,
          },
        }}
        engineLoader={() => new Promise(() => undefined)}
      />
    );

    const note = screen.getByTestId("session-agent-reported-truncated");
    expect(note.tagName).toBe("DIV");
    expect(note).toHaveTextContent("truncated");
    expect(note).toHaveTextContent(`${new Intl.NumberFormat().format(163_750)} bytes`);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
    expect(screen.queryByText(/omitted/)).not.toBeInTheDocument();
  });

  it("Should keep the line clamp independent of a truncated byte bound", async () => {
    const user = userEvent.setup();
    const lines = Array.from({ length: 20 }, (_, index) => `line ${index + 1}`);
    render(
      <SessionAgentReportedBlock
        data={{
          ...reportedTerminalEvent,
          text: `${lines.join("\n")}\n`,
          reported_terminal: {
            id: "reported-terminal-1",
            cwd: "/workspace",
            total_bytes: 163_750,
            truncated: true,
            exit_code: 0,
          },
        }}
        engineLoader={() => new Promise(() => undefined)}
      />
    );

    const more = screen.getByRole("button", { name: /show \d+ more lines/ });
    expect(more).toHaveAttribute("aria-expanded", "false");
    expect(screen.getByTestId("session-agent-reported-truncated")).toHaveTextContent(
      `${new Intl.NumberFormat().format(163_750)} bytes`
    );
    expect(screen.queryByRole("button", { name: /bytes/ })).not.toBeInTheDocument();

    await user.click(more);
    expect(more).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByTestId("session-agent-reported-truncated")).toBeInTheDocument();
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

  it("Should show a write failure on the reported block and not reject the replay", async () => {
    const unhandled: unknown[] = [];
    const onUnhandled = (event: PromiseRejectionEvent) => {
      unhandled.push(event.reason);
    };
    window.addEventListener("unhandledrejection", onUnhandled);
    render(
      <SessionAgentReportedBlock
        data={reportedTerminalEvent}
        engineLoader={stubThrowingEngineLoader(new Error("emulator parse failed"))}
      />
    );

    await waitFor(() => {
      expect(screen.getByRole("status")).toHaveTextContent(terminalReplayFailedCopy());
    });
    expect(unhandled).toEqual([]);
    window.removeEventListener("unhandledrejection", onUnhandled);
  });
});
