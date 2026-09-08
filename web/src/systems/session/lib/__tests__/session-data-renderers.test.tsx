import { render, screen } from "@testing-library/react";
import { type ComponentProps } from "react";
import { describe, expect, it } from "vitest";

import type {
  AgentEventPayload,
  CompozyPermissionData,
  SessionInteractionRecord,
} from "../../types";
import { CompozyEventDataRenderer, CompozyPermissionDataRenderer } from "../session-data-renderers";
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
    <CompozyEventDataRenderer
      {...({ data } as unknown as ComponentProps<typeof CompozyEventDataRenderer>)}
    />
  );
}

function renderWithContext(
  data: AgentEventPayload,
  expiredInteractions?: ReadonlyMap<string, SessionInteractionRecord>
) {
  render(
    <SessionRuntimeRenderProvider
      expiredInteractions={expiredInteractions}
      sessionId={SESSION_ID}
      workspaceId={WORKSPACE_ID}
    >
      <EventRenderer data={data} />
    </SessionRuntimeRenderProvider>
  );
}

function PermissionRenderer({ data }: { data: CompozyPermissionData }) {
  return (
    <CompozyPermissionDataRenderer
      {...({ data } as unknown as ComponentProps<typeof CompozyPermissionDataRenderer>)}
    />
  );
}

function renderPermissionWithContext(
  data: CompozyPermissionData,
  expiredInteractions?: ReadonlyMap<string, SessionInteractionRecord>
) {
  render(
    <SessionRuntimeRenderProvider
      expiredInteractions={expiredInteractions}
      sessionId={SESSION_ID}
      workspaceId={WORKSPACE_ID}
    >
      <PermissionRenderer data={data} />
    </SessionRuntimeRenderProvider>
  );
}

function clarifyPending(): AgentEventPayload {
  const resolved = clarifyResolved();
  const raw = resolved.raw as Record<string, unknown>;
  return { ...resolved, raw: { ...raw, status: "pending", answer: null } };
}

function restartExpired(
  overrides: Partial<SessionInteractionRecord> = {}
): SessionInteractionRecord {
  return {
    interaction_id: "int-1",
    kind: "permission",
    provider_request_id: "req-1",
    turn_id: "turn-001",
    status: "canceled",
    created_at: "2026-09-05T10:00:00Z",
    resolved_at: "2026-09-05T10:02:00Z",
    resolution: "failed-by-restart",
    resolved_by: "system",
    ...overrides,
  };
}

const pendingTerminalPermission: CompozyPermissionData = {
  type: "permission",
  session_id: SESSION_ID,
  turn_id: "turn-001",
  request_id: "req-1",
  title: "compozy__terminal_exec",
  timestamp: "2026-09-05T10:00:00Z",
  raw: {
    tool_id: "compozy__terminal_exec",
    tool_input: { command: "bun", args: ["test"], cwd: "~/dev/atlas-api", risk: "ordinary" },
  },
};

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

  it("Should route a provider-diagnostic error event to the provider notice", () => {
    renderWithContext({
      type: "error",
      session_id: SESSION_ID,
      turn_id: "turn-auth",
      error: "provider authentication required",
      failure: { kind: "prompt_failure", summary: "provider authentication required" },
      provider_error: {
        code: "provider_auth_required",
        provider: "claude-code",
        next_action: "login",
        guidance: "run provider auth login for this provider",
        occurrence_count: 2,
        first_seen_at: "2026-09-05T14:02:00Z",
        last_seen_at: "2026-09-05T14:05:00Z",
      },
    });

    const notice = screen.getByTestId("session-error-notice");
    expect(notice).toHaveAttribute("data-provider-error", "provider_auth_required");
    expect(screen.getByTestId("provider-error-subject")).toHaveTextContent(
      "claude-code needs sign-in"
    );
    expect(screen.getByTestId("provider-error-occurrence")).toHaveTextContent(/^2 times since/);
    expect(screen.queryByTestId("clarification-receipt")).not.toBeInTheDocument();
  });

  it("Should keep command-style events off any terminal rendering path", () => {
    renderWithContext({
      type: "terminal_output",
      session_id: SESSION_ID,
      turn_id: "turn-legacy",
      title: "bun test",
      text: "$ bun test\n12 tests passed\n",
    });

    expect(screen.queryByText("reported by agent")).not.toBeInTheDocument();
    expect(screen.queryByRole("log")).not.toBeInTheDocument();
  });

  it("Should render a pending clarify ask the daemon expired at a restart as a receipt", () => {
    renderWithContext(
      clarifyPending(),
      new Map([["req-1", restartExpired({ kind: "clarify", turn_id: "clarify:req-1" })]])
    );

    const receipt = screen.getByTestId("clarification-receipt");
    expect(receipt).toHaveAttribute("data-status", "canceled");
    expect(receipt).toHaveAttribute("data-cause", "restart");
    expect(receipt).toHaveTextContent(
      "Question not answered — CompozyOS restarted before you answered · Which path?"
    );
  });

  it("Should keep a pending clarify ask silent while nothing expired it", () => {
    const { container } = render(
      <SessionRuntimeRenderProvider sessionId={SESSION_ID} workspaceId={WORKSPACE_ID}>
        <EventRenderer data={clarifyPending()} />
      </SessionRuntimeRenderProvider>
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("Should not read a plain canceled clarify row as a restart expiry", () => {
    const { container } = render(
      <SessionRuntimeRenderProvider
        expiredInteractions={
          new Map([["req-1", restartExpired({ kind: "clarify", resolution: "", resolved_by: "" })]])
        }
        sessionId={SESSION_ID}
        workspaceId={WORKSPACE_ID}
      >
        <EventRenderer data={clarifyPending()} />
      </SessionRuntimeRenderProvider>
    );

    expect(container).toBeEmptyDOMElement();
  });
});

describe("CompozyPermissionDataRenderer", () => {
  it("Should replace the waiting line with an expired receipt once the daemon expired the ask", () => {
    renderPermissionWithContext(pendingTerminalPermission, new Map([["req-1", restartExpired()]]));

    const receipt = screen.getByTestId("permission-expired-receipt");
    expect(receipt).toHaveAttribute("data-tone", "neutral");
    expect(receipt).toHaveAttribute("data-cause", "restart");
    expect(receipt).toHaveAttribute("data-resolution", "failed-by-restart");
    expect(receipt).toHaveAttribute("data-resolved-by", "system");
    expect(receipt).toHaveTextContent(
      "Not decided · CompozyOS restarted before you answered — bun test did not run · asked"
    );
    expect(receipt.querySelector("time")).toHaveAttribute("dateTime", "2026-09-05T10:00:00Z");
    expect(screen.queryByTestId("permission-waiting-line")).not.toBeInTheDocument();
  });

  it("Should read a canceled row without the restart resolution as a plain cancellation", () => {
    renderPermissionWithContext(
      pendingTerminalPermission,
      new Map([
        ["req-1", restartExpired({ resolution: "operator-cleared", resolved_by: "operator" })],
      ])
    );

    const receipt = screen.getByTestId("permission-expired-receipt");
    expect(receipt).toHaveAttribute("data-tone", "neutral");
    expect(receipt).toHaveAttribute("data-cause", "canceled");
    expect(receipt).toHaveAttribute("data-resolution", "operator-cleared");
    expect(receipt).toHaveTextContent(
      "Not decided · the request was canceled before you answered — bun test did not run"
    );
    expect(receipt).not.toHaveTextContent("restarted");
    expect(screen.queryByTestId("permission-waiting-line")).not.toBeInTheDocument();
  });

  it("Should keep the waiting line while the ask is still open", () => {
    renderPermissionWithContext(pendingTerminalPermission, new Map());

    expect(screen.getByTestId("permission-waiting-line")).toBeInTheDocument();
    expect(screen.queryByTestId("permission-expired-receipt")).not.toBeInTheDocument();
  });

  it("Should name the tool and subject for an expired generic ask", () => {
    renderPermissionWithContext(
      {
        ...pendingTerminalPermission,
        title: "Bash",
        resource: "rm -rf /tmp/test",
        raw: { tool_input: { command: "rm -rf /tmp/test" } },
      },
      new Map([["req-1", restartExpired()]])
    );

    expect(screen.getByTestId("permission-expired-receipt")).toHaveTextContent(
      "Not decided · CompozyOS restarted before you answered · Bash — rm -rf /tmp/test"
    );
  });

  it("Should let a recorded decision win over a stale expiry row", () => {
    renderPermissionWithContext(
      { ...pendingTerminalPermission, decision: "allow-once" },
      new Map([["req-1", restartExpired()]])
    );

    expect(screen.getByTestId("permission-allowed-receipt")).toBeInTheDocument();
    expect(screen.queryByTestId("permission-expired-receipt")).not.toBeInTheDocument();
  });
});
