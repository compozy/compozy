import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ThreadMessage } from "@assistant-ui/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("sonner", () => ({
  toast: { error: vi.fn() },
}));

vi.mock("../../adapters/session-api", async importOriginal => ({
  ...(await importOriginal<typeof import("../../adapters/session-api")>()),
  approveSession: vi.fn(),
}));

vi.mock("../../adapters/session-clarification-api", async importOriginal => ({
  ...(await importOriginal<typeof import("../../adapters/session-clarification-api")>()),
  fetchSessionClarifications: vi.fn(),
  answerSessionClarification: vi.fn(),
}));

import { approveSession } from "../../adapters/session-api";
import {
  answerSessionClarification,
  fetchSessionClarifications,
} from "../../adapters/session-clarification-api";
import { SessionTranscriptThreadProvider } from "../../lib/session-transcript-thread-context";
import type { ClarificationPending } from "../../types";
import { SessionDecisionDock } from "../session-decision-dock";

const WORKSPACE_ID = "ws_alpha";
const SESSION_ID = "sess-001";

function permissionMessage(
  requestId: string,
  overrides: Record<string, unknown> = {}
): ThreadMessage {
  return {
    id: `message-${requestId}`,
    role: "assistant",
    content: [
      {
        type: "data-compozy-permission",
        data: {
          type: "permission",
          session_id: SESSION_ID,
          turn_id: "turn-001",
          request_id: requestId,
          title: "Bash",
          action: "execute",
          resource: "rm -rf /tmp/test",
          raw: { tool_input: { command: "rm -rf /tmp/test" } },
          ...overrides,
        },
      },
    ],
  } as unknown as ThreadMessage;
}

function clarification(overrides: Partial<ClarificationPending> = {}): ClarificationPending {
  return {
    agent_name: "frontend-qa",
    session_id: SESSION_ID,
    request_id: "clarify-1",
    question: "Where should the marker stories live?",
    choices: ["packages/ui (shared primitive)", "web/src/systems/session"],
    asked_at: "2026-07-19T20:00:00Z",
    deadline: "2026-07-19T20:05:00Z",
    ...overrides,
  };
}

function renderDock({
  messages = [] as readonly ThreadMessage[],
  clarifications = [] as ClarificationPending[],
} = {}) {
  vi.mocked(fetchSessionClarifications).mockResolvedValue(clarifications);
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <SessionTranscriptThreadProvider
        messages={messages}
        status="success"
        error={null}
        retry={vi.fn()}
      >
        <SessionDecisionDock sessionId={SESSION_ID} workspaceId={WORKSPACE_ID} />
      </SessionTranscriptThreadProvider>
    </QueryClientProvider>
  );
}

describe("SessionDecisionDock", () => {
  beforeEach(() => {
    // Clear call HISTORY too — module-mock vi.fn instances keep their recorded
    // calls across tests otherwise, chaining one test's calls into the next.
    vi.clearAllMocks();
    vi.mocked(approveSession).mockResolvedValue(undefined);
    vi.mocked(answerSessionClarification).mockResolvedValue(
      {} as Awaited<ReturnType<typeof answerSessionClarification>>
    );
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("Should render nothing when no decision is pending", async () => {
    const { container } = renderDock();
    await waitFor(() => expect(fetchSessionClarifications).toHaveBeenCalled());
    expect(container).toBeEmptyDOMElement();
  });

  it("Should dock a pending permission with its subject on the code wash", () => {
    renderDock({ messages: [permissionMessage("req-1")] });

    expect(screen.getByTestId("permission-dock")).toBeInTheDocument();
    expect(screen.getByTestId("permission-dock-eyebrow")).toHaveTextContent("Permission");
    expect(screen.getByTestId("permission-dock-title")).toHaveTextContent("Bash");
    expect(screen.getByTestId("permission-dock-subject")).toHaveTextContent("rm -rf /tmp/test");
    expect(screen.getByTestId("permission-allow-once")).toBeInTheDocument();
    expect(screen.getByTestId("permission-allow-always")).toBeInTheDocument();
    expect(screen.getByTestId("permission-reject-once")).toBeInTheDocument();
  });

  it("Should not dock a permission the transcript already resolved", () => {
    const { container } = renderDock({
      messages: [permissionMessage("req-1", { decision: "allow-once" })],
    });
    expect(container.querySelector('[data-testid="permission-dock"]')).toBeNull();
  });

  it("Should decide on digit keys 1–4, with key 4 firing while the menu is closed", async () => {
    renderDock({ messages: [permissionMessage("req-1")] });

    fireEvent.keyDown(document.body, { key: "4" });
    await waitFor(() => {
      expect(approveSession).toHaveBeenCalledWith(WORKSPACE_ID, SESSION_ID, {
        request_id: "req-1",
        turn_id: "turn-001",
        decision: "reject-always",
      });
    });
  });

  it("Should ignore digit shortcuts while an input owns focus", async () => {
    renderDock({ messages: [permissionMessage("req-1")] });
    const textarea = document.createElement("textarea");
    document.body.appendChild(textarea);
    textarea.focus();

    fireEvent.keyDown(textarea, { key: "1" });

    await new Promise(resolve => setTimeout(resolve, 10));
    expect(approveSession).not.toHaveBeenCalled();
    textarea.remove();
  });

  it("Should gate buttons and keys to the decisions the runtime offers", async () => {
    renderDock({
      messages: [
        permissionMessage("req-1", {
          raw: {
            options: [
              { decision: "allow-once", option_id: "allow-once" },
              { decision: "reject-once", option_id: "reject-once" },
            ],
            tool_input: { command: "touch blocked.txt" },
          },
        }),
      ],
    });

    expect(screen.getByTestId("permission-allow-once")).toBeInTheDocument();
    expect(screen.queryByTestId("permission-allow-always")).not.toBeInTheDocument();
    expect(screen.getByTestId("permission-reject-once")).toBeInTheDocument();
    expect(screen.queryByTestId("permission-reject-menu-trigger")).not.toBeInTheDocument();

    fireEvent.keyDown(document.body, { key: "2" });
    fireEvent.keyDown(document.body, { key: "4" });
    await new Promise(resolve => setTimeout(resolve, 10));
    expect(approveSession).not.toHaveBeenCalled();
  });

  it("Should keep reject-always reachable through the reject split menu", async () => {
    const user = userEvent.setup();
    renderDock({ messages: [permissionMessage("req-1")] });

    expect(screen.queryByTestId("permission-reject-menu")).not.toBeInTheDocument();
    await user.click(screen.getByTestId("permission-reject-menu-trigger"));
    await user.click(screen.getByTestId("permission-reject-always"));

    await waitFor(() => {
      expect(approveSession).toHaveBeenCalledWith(
        WORKSPACE_ID,
        SESSION_ID,
        expect.objectContaining({ decision: "reject-always" })
      );
    });
  });

  it("Should count queued decisions and surface the next one after resolving", async () => {
    const user = userEvent.setup();
    renderDock({
      messages: [permissionMessage("req-1"), permissionMessage("req-2", { title: "Write" })],
    });

    expect(screen.getByTestId("permission-dock-count")).toHaveTextContent("1/2");
    await user.click(screen.getByTestId("permission-allow-once"));

    await waitFor(() => {
      expect(screen.getByTestId("permission-dock-title")).toHaveTextContent("Write");
    });
    expect(screen.queryByTestId("permission-dock-count")).not.toBeInTheDocument();
  });

  it("Should dock a bounded clarification and answer choices on digit keys", async () => {
    renderDock({ clarifications: [clarification()] });

    expect(await screen.findByTestId("clarification-dock")).toBeInTheDocument();
    expect(screen.getByTestId("clarification-dock-question")).toHaveTextContent(
      "Where should the marker stories live?"
    );
    expect(screen.getAllByTestId("clarification-dock-choice")).toHaveLength(2);
    expect(screen.getByTestId("clarification-dock-deadline").textContent).toMatch(/^times out /);

    fireEvent.keyDown(document.body, { key: "2" });
    await waitFor(() => {
      expect(answerSessionClarification).toHaveBeenCalledWith(
        WORKSPACE_ID,
        SESSION_ID,
        "clarify-1",
        {
          choice_index: 1,
        }
      );
    });
  });

  it("Should render the free-text form when the question ships no choices and submit on Enter", async () => {
    const user = userEvent.setup();
    renderDock({ clarifications: [clarification({ choices: [] })] });

    const input = await screen.findByTestId("clarification-dock-text-input");
    await user.type(input, "Ship it as canary{Shift>}{Enter}{/Shift} then promote");
    expect(answerSessionClarification).not.toHaveBeenCalled();

    await user.type(input, "{Enter}");
    await waitFor(() => {
      expect(answerSessionClarification).toHaveBeenCalledWith(
        WORKSPACE_ID,
        SESSION_ID,
        "clarify-1",
        {
          text: "Ship it as canary\n then promote",
        }
      );
    });
  });

  it("Should prioritize a pending permission over pending clarifications", async () => {
    renderDock({
      messages: [permissionMessage("req-1")],
      clarifications: [clarification()],
    });

    expect(screen.getByTestId("permission-dock")).toBeInTheDocument();
    expect(screen.queryByTestId("clarification-dock")).not.toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByTestId("permission-dock-count")).toHaveTextContent("1/2");
    });
  });
});
