import { useEffect, useState, type ComponentProps } from "react";
import type { ThreadMessage } from "@assistant-ui/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { toast } from "sonner";

import { SessionChatRuntimeProvider } from "@/systems/session/components/session-chat-runtime-provider";
import { useSessionStore } from "@/systems/session/hooks/use-session-store";
import { primarySessionFixture, sessionTranscriptFixture } from "@/systems/session/mocks/fixtures";
import { SessionTranscriptThreadProvider } from "@/systems/session/lib/session-transcript-thread-context";
import {
  getSessionDebugCounters,
  getSessionDebugEvents,
  resetSessionDebugTelemetry,
  SESSION_DEBUG_EVENTS,
} from "@/systems/session/lib/session-observability";
import { toReadonlyThreadMessages } from "@/systems/session/lib/session-thread-repository";
import type { SessionFailurePayload, SessionMessage, SessionState } from "@/systems/session/types";
import type { SessionTranscriptThreadStatus } from "@/systems/session/lib/session-transcript-thread-context-value";

import { SessionThread } from "../session-thread";
import { formatDataPreview } from "../session-message-parts.logic";
import { TimelineRowContent } from "../session-timeline-render";
import {
  computeStableSessionRows,
  deriveSessionRows,
  EMPTY_STABLE_SESSION_ROWS,
  type SessionTimelinePart,
  type StableSessionRowsState,
} from "../session-timeline.logic";
import { WorkingIndicator } from "../session-working-row";

vi.mock("sonner", () => ({
  toast: {
    error: vi.fn(),
    success: vi.fn(),
    warning: vi.fn(),
  },
}));

vi.mock("@tanstack/react-router", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    Link: ({ to, params, children, ...props }: Record<string, unknown>) => (
      <a
        href={typeof to === "string" ? to : "#"}
        data-params={JSON.stringify(params)}
        {...(props as Record<string, unknown>)}
      >
        {children as React.ReactNode}
      </a>
    ),
  };
});

// Suite: thread state panes
// Invariant: ThreadEmpty renders only after the transcript fetch succeeds with zero messages.
// Boundary IN: SessionThread state branching and readonly row rendering.
// Boundary OUT: transcript query/refetch wiring, covered by session-chat-runtime-provider.test.tsx.

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), {
    headers: { "Content-Type": "application/json" },
  });
}

function getPathname(input: RequestInfo | URL): string {
  if (typeof input === "string") {
    return new URL(input, "http://localhost").pathname;
  }

  if (input instanceof URL) {
    return input.pathname;
  }

  return new URL(input.url, "http://localhost").pathname;
}

function fixtureWorkspaceId(): string {
  const workspaceId = primarySessionFixture.workspace_id;
  if (!workspaceId) {
    throw new Error("primary session fixture must include workspace_id");
  }
  return workspaceId;
}

const TOOL_ARTIFACT_URI = `agh://tool-artifacts/art_${"c".repeat(64)}`;

function toolArtifactRef() {
  return {
    uri: TOOL_ARTIFACT_URI,
    name: "tool-result.json",
    mime_type: "application/vnd.agh.tool-result+json",
    bytes: 4_096,
    sha256: "c".repeat(64),
  };
}

function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
      mutations: {
        retry: false,
      },
    },
  });
}

function createFetchMock() {
  return vi.fn(async (input: RequestInfo | URL) => {
    const pathname = getPathname(input);

    if (pathname === "/api/sessions") {
      return jsonResponse({ sessions: [primarySessionFixture] });
    }

    if (
      pathname ===
      `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/${primarySessionFixture.id}`
    ) {
      return jsonResponse({ session: primarySessionFixture });
    }

    if (
      pathname ===
      `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/${primarySessionFixture.id}/transcript`
    ) {
      return jsonResponse({ entries: [] });
    }

    throw new Error(`Unhandled fetch in thread test: ${pathname}`);
  });
}

function renderThreadState({
  messages = [],
  status,
  error = null,
  retry = vi.fn(),
  isSessionRunning = false,
  acpSessionId,
  sessionState,
  failure = null,
}: {
  messages?: readonly ThreadMessage[];
  status: SessionTranscriptThreadStatus;
  error?: Error | null;
  retry?: () => void;
  isSessionRunning?: boolean;
  acpSessionId?: string;
  sessionState?: SessionState;
  failure?: SessionFailurePayload | null;
}) {
  const queryClient = createQueryClient();

  render(
    <QueryClientProvider client={queryClient}>
      <SessionChatRuntimeProvider
        sessionId={primarySessionFixture.id}
        workspaceId={fixtureWorkspaceId()}
      >
        <SessionTranscriptThreadProvider
          messages={messages}
          status={status}
          error={error}
          retry={retry}
        >
          <SessionThread
            sessionId={primarySessionFixture.id}
            agentName={primarySessionFixture.agent_name}
            canPrompt
            onCancelPrompt={() => {}}
            isSessionRunning={isSessionRunning}
            acpSessionId={acpSessionId}
            sessionState={sessionState}
            failure={failure}
          />
        </SessionTranscriptThreadProvider>
      </SessionChatRuntimeProvider>
    </QueryClientProvider>
  );
}

describe("SessionThread transcript states", () => {
  it("Should preview non-JSON values without returning undefined", () => {
    const value = () => "result";
    expect(formatDataPreview(value)).toBe(String(value));
  });

  beforeEach(() => {
    resetSessionDebugTelemetry();
    vi.stubGlobal("fetch", createFetchMock());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    resetSessionDebugTelemetry();
  });

  it("Should render the transcript skeleton on pending without empty-state copy", async () => {
    renderThreadState({ status: "pending" });

    expect(
      await screen.findByRole("status", {
        name: /loading transcript/i,
      })
    ).toBeInTheDocument();
    expect(screen.getByTestId("thread-transcript-skeleton")).toBeInTheDocument();
    expect(screen.queryByText(/Start a conversation/i)).not.toBeInTheDocument();
  });

  it("Should render a retryable transcript error pane and call retry", async () => {
    const user = userEvent.setup();
    const retry = vi.fn();

    renderThreadState({
      status: "error",
      error: new Error("recorder temporarily unavailable"),
      retry,
    });

    const pane = await screen.findByTestId("thread-transcript-error");
    expect(pane).toHaveAttribute("role", "alert");
    expect(within(pane).getByText("Transcript unavailable")).toBeInTheDocument();
    expect(screen.getByTestId("thread-transcript-error-detail")).toHaveTextContent(
      "recorder temporarily unavailable"
    );
    expect(screen.queryByText(/Start a conversation/i)).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /retry transcript/i }));

    expect(retry).toHaveBeenCalledTimes(1);
  });

  it("Should render ThreadEmpty only for success with zero messages", async () => {
    renderThreadState({ status: "success" });

    expect(await screen.findByText(/Start a conversation/i)).toBeInTheDocument();
    expect(screen.queryByTestId("thread-transcript-skeleton")).not.toBeInTheDocument();
    expect(screen.queryByTestId("thread-transcript-error")).not.toBeInTheDocument();
  });

  it("Should prioritize durable starting state and keep the composer disabled", async () => {
    renderThreadState({ status: "pending", sessionState: "starting" });

    const pane = await screen.findByTestId("thread-session-starting");
    expect(pane).toHaveAttribute("role", "status");
    expect(pane).toHaveTextContent("The session is saved");
    expect(screen.queryByTestId("thread-transcript-skeleton")).not.toBeInTheDocument();
    expect(screen.getByTestId("composer-textarea")).toBeDisabled();
    expect(screen.getByTestId("composer-send-button")).toBeDisabled();
    expect(screen.getByTestId("composer-textarea")).toHaveAttribute(
      "placeholder",
      "Session is starting…"
    );
  });

  it("Should surface durable startup failure and link to the agent runtime settings", async () => {
    renderThreadState({
      status: "error",
      error: new Error("transcript unavailable"),
      sessionState: "stopped",
      failure: {
        kind: "provider_auth_failure",
        summary: "The configured model is unavailable.",
      },
    });

    const pane = await screen.findByTestId("thread-session-startup-failure");
    expect(pane).toHaveAttribute("role", "alert");
    expect(pane).toHaveTextContent("The configured model is unavailable.");
    expect(within(pane).getByRole("button", { name: "Review agent runtime" })).toBeInTheDocument();
    expect(screen.queryByTestId("thread-transcript-error")).not.toBeInTheDocument();
    expect(screen.getByTestId("composer-textarea")).toBeDisabled();
    expect(screen.getByTestId("composer-send-button")).toBeDisabled();
    expect(screen.getByTestId("composer-textarea")).toHaveAttribute(
      "placeholder",
      "Session failed to start"
    );
  });

  it("Should surface a preactivation process failure without an unrelated runtime action", async () => {
    renderThreadState({
      status: "error",
      error: new Error("transcript unavailable"),
      sessionState: "stopped",
      failure: {
        kind: "process_exit",
        summary: "The provider process exited before activation.",
      },
    });

    const pane = await screen.findByTestId("thread-session-startup-failure");
    expect(pane).toHaveTextContent("The provider process exited before activation.");
    expect(within(pane).queryByRole("button", { name: "Review agent runtime" })).toBeNull();
    expect(screen.getByTestId("composer-textarea")).toBeDisabled();
  });

  it("Should keep post-activation failures in the transcript lifecycle", async () => {
    renderThreadState({
      status: "error",
      error: new Error("transcript unavailable"),
      acpSessionId: "acp-existing",
      sessionState: "stopped",
      failure: {
        kind: "process_exit",
        summary: "The active provider process exited.",
      },
    });

    expect(await screen.findByTestId("thread-transcript-error")).toBeInTheDocument();
    expect(screen.queryByTestId("thread-session-startup-failure")).not.toBeInTheDocument();
  });

  it("Should record a debug event when ThreadEmpty renders while the session is active", async () => {
    renderThreadState({ status: "success", isSessionRunning: true });

    expect(await screen.findByText(/Start a conversation/i)).toBeInTheDocument();
    await waitFor(() => {
      expect(getSessionDebugCounters()[SESSION_DEBUG_EVENTS.threadEmptyWhileActive]).toBe(1);
    });
    expect(getSessionDebugEvents()).toContainEqual(
      expect.objectContaining({
        agent_name: primarySessionFixture.agent_name,
        event: SESSION_DEBUG_EVENTS.threadEmptyWhileActive,
        message_count: 0,
        session_id: primarySessionFixture.id,
        transcript_status: "success",
      })
    );
  });

  it("Should render rows for success with transcript messages", async () => {
    const messages = toReadonlyThreadMessages(sessionTranscriptFixture.slice(0, 2));

    renderThreadState({ status: "success", messages });

    expect(await screen.findByText("Launch readiness snapshot")).toBeInTheDocument();
    expect(screen.getByTestId("thread-messages")).toBeInTheDocument();
    expect(screen.queryByText(/Start a conversation/i)).not.toBeInTheDocument();
  });

  it.each([
    ["goal-work", "Goal work"],
    ["goal-continuation", "Goal continuation"],
    ["goal-compaction", "Goal compaction"],
  ] as const)(
    "Should render persisted %s prompt identity from the transcript",
    async (kind, label) => {
      const transcript = [
        {
          id: `assistant-${kind}`,
          role: "assistant",
          parts: [
            {
              type: "data-agh-event",
              data: {
                type: "prompt_started",
                goal: {
                  kind,
                  run_id: "run_1",
                  node_id: "goal",
                  generation: 2,
                  item_index: 0,
                  turn: 3,
                  prompt_attempt: 1,
                  prompt_id: `prompt-${kind}`,
                },
              },
            },
            { type: "text", text: `${label} response`, state: "done" },
          ] as unknown as SessionMessage["parts"],
        } as SessionMessage,
      ];

      renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

      const notice = await screen.findByTestId("goal-prompt-meta");
      expect(notice).toHaveTextContent(label);
      expect(notice).toHaveTextContent("goal · generation 2 · turn 3");
      expect(within(notice).getByRole("link", { name: "Open run" })).toBeInTheDocument();
    }
  );

  it("Should not render beyond the readonly provider's committed message count when transcript grows after reconnect", async () => {
    const initialTranscript = [
      {
        id: "turn-hook-started",
        role: "assistant",
        parts: [
          {
            type: "data-agh-event",
            data: {
              type: "session_started",
              turn_id: "turn-hook-started",
              timestamp: "2026-07-08T21:37:00Z",
            },
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
      {
        id: "turn-hook-prompt",
        role: "assistant",
        parts: [
          {
            type: "data-agh-event",
            data: {
              type: "prompt_started",
              turn_id: "turn-hook-prompt",
              timestamp: "2026-07-08T21:37:01Z",
            },
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
      {
        id: "user-after-hooks",
        role: "user",
        parts: [{ type: "text", text: "Continue delegated task run", state: "done" }],
      } as unknown as SessionMessage,
    ];
    const grownTranscript = [
      ...initialTranscript,
      {
        id: "assistant-after-reconnect",
        role: "assistant",
        parts: [
          {
            type: "text",
            text: "Continuing after reconnect.",
            state: "streaming",
            turn_id: "turn-after-reconnect",
            timestamp: "2026-07-08T21:37:02Z",
          },
        ] as unknown as SessionMessage["parts"],
        status: { type: "running" },
      } as SessionMessage,
    ];

    function GrowingTranscriptThread() {
      const [messages, setMessages] = useState(() => toReadonlyThreadMessages(initialTranscript));
      useEffect(() => {
        setMessages(toReadonlyThreadMessages(grownTranscript));
      }, []);

      return (
        <QueryClientProvider client={createQueryClient()}>
          <SessionChatRuntimeProvider
            sessionId={primarySessionFixture.id}
            workspaceId={fixtureWorkspaceId()}
          >
            <SessionTranscriptThreadProvider
              messages={messages}
              status="success"
              error={null}
              retry={vi.fn()}
            >
              <SessionThread
                sessionId={primarySessionFixture.id}
                agentName={primarySessionFixture.agent_name}
                canPrompt
                onCancelPrompt={() => {}}
                isSessionRunning
              />
            </SessionTranscriptThreadProvider>
          </SessionChatRuntimeProvider>
        </QueryClientProvider>
      );
    }

    render(<GrowingTranscriptThread />);

    expect(await screen.findByText("Continue delegated task run")).toBeInTheDocument();
    expect(await screen.findByText("Continuing after reconnect.")).toBeInTheDocument();
    expect(screen.getByTestId("thread-messages")).toBeInTheDocument();
  });

  it("Should render grouped tool clusters behind a previous-calls toggle", async () => {
    const user = userEvent.setup();
    const transcript = [
      {
        id: "assistant-tools",
        role: "assistant",
        parts: Array.from({ length: 8 }, (_, index) => ({
          type: "tool-Read",
          toolCallId: `tool-read-${index + 1}`,
          state: "output-available",
          turn_id: "turn-tools",
          timestamp: `2026-07-07T12:00:0${index}Z`,
          input: { file_path: `/tmp/file-${index + 1}.ts` },
          output: {
            type: "tool_result",
            title: "Read",
            raw: { content: `file-${index + 1}` },
          },
        })) as unknown as SessionMessage["parts"],
      } as SessionMessage,
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

    // Collapsed settled cluster: only the latest call renders, under an "N tool
    // calls" group label, with the rest behind the previous-calls toggle.
    expect(await screen.findByText("/tmp/file-8.ts")).toBeInTheDocument();
    expect(screen.getByText("8 tool calls")).toBeInTheDocument();
    expect(screen.queryByText("/tmp/file-1.ts")).not.toBeInTheDocument();
    expect(screen.getAllByTestId("tool-call-row")).toHaveLength(1);

    const toggle = screen.getByRole("button", { name: "+7 previous tool calls" });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    await user.click(toggle);

    expect(await screen.findByText("/tmp/file-1.ts")).toBeInTheDocument();
    expect(screen.getAllByTestId("tool-call-row")).toHaveLength(8);
    const openToggle = screen.getByRole("button", { name: "Show fewer tool calls" });
    expect(openToggle).toHaveAttribute("aria-expanded", "true");

    await user.click(openToggle);
    expect(screen.getAllByTestId("tool-call-row")).toHaveLength(1);
    expect(screen.queryByText("/tmp/file-1.ts")).not.toBeInTheDocument();
  });

  it("Should render an empty-args tool mid-stream as the pending row state (not a bordered box)", async () => {
    const transcript = [
      {
        id: "assistant-pending",
        role: "assistant",
        parts: [
          {
            type: "tool-Read",
            toolCallId: "tool-read-pending",
            state: "input-streaming",
            turn_id: "turn-pending",
            timestamp: "2026-07-07T12:00:00Z",
            input: {},
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

    const row = await screen.findByTestId("tool-call-row");
    expect(row.querySelector('[data-slot="tool-call-row"]')?.getAttribute("data-status")).toBe(
      "pending"
    );
    // Pending is glyph-less and never the legacy bordered "preparing input" box.
    expect(row.querySelector('[data-slot="tool-call-row-status"]')).toBeNull();
    expect(screen.queryByText(/preparing input/i)).not.toBeInTheDocument();
  });

  it("Should render an error part as the failed row state (not a custom danger box)", async () => {
    const transcript = [
      {
        id: "assistant-error",
        role: "assistant",
        parts: [
          {
            type: "tool-Bash",
            toolCallId: "tool-bash-error",
            state: "output-error",
            turn_id: "turn-error",
            timestamp: "2026-07-07T12:00:00Z",
            input: { command: "deploy" },
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

    const row = await screen.findByTestId("tool-call-row");
    expect(row.querySelector('[data-slot="tool-call-row"]')?.getAttribute("data-status")).toBe(
      "failed"
    );
    expect(
      row.querySelector('[data-slot="tool-call-row-status"]')?.getAttribute("aria-label")
    ).toBe("Error");
  });

  // Suite: oversized tool-result normalization.
  // Invariant: direct native ToolResult and ACP event wrappers preserve the same bounded preview,
  // truncated flag, and retained artifact reference before the SessionToolCallRow renders them.
  it("Should preserve a direct native truncated ToolResult for the artifact viewer", async () => {
    const user = userEvent.setup();
    const transcript = [
      {
        id: "assistant-native-artifact",
        role: "assistant",
        parts: [
          {
            type: "tool-agh__memory_recall",
            toolCallId: "tool-native-artifact",
            state: "output-available",
            turn_id: "turn-native-artifact",
            timestamp: "2026-07-19T04:30:00Z",
            input: { query: "release evidence" },
            output: {
              preview: "native bounded preview",
              truncated: true,
              artifacts: [toolArtifactRef()],
            },
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });
    const row = await screen.findByTestId("tool-call-row");
    const trigger = row.querySelector<HTMLElement>('[data-slot="tool-call-row-trigger"]');
    expect(trigger).not.toBeNull();
    await user.click(trigger as HTMLElement);

    expect(screen.getByText("native bounded preview")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Open full result" })).toBeInTheDocument();
  });

  it("Should preserve an ACP-wrapped truncated ToolResult for the artifact viewer", async () => {
    const user = userEvent.setup();
    const transcript = [
      {
        id: "assistant-acp-artifact",
        role: "assistant",
        parts: [
          {
            type: "tool-agh__memory_recall",
            toolCallId: "tool-acp-artifact",
            state: "output-available",
            turn_id: "turn-acp-artifact",
            timestamp: "2026-07-19T04:31:00Z",
            input: { query: "release evidence" },
            output: {
              type: "tool_result",
              title: "Recall memory",
              raw: {
                preview: "ACP bounded preview",
                truncated: true,
                artifacts: [toolArtifactRef()],
              },
            },
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });
    const row = await screen.findByTestId("tool-call-row");
    const trigger = row.querySelector<HTMLElement>('[data-slot="tool-call-row-trigger"]');
    expect(trigger).not.toBeNull();
    await user.click(trigger as HTMLElement);

    expect(screen.getByText("ACP bounded preview")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Open full result" })).toBeInTheDocument();
  });

  it("Should preserve a persisted canonical ToolResult for the artifact viewer", async () => {
    const user = userEvent.setup();
    const transcript = [
      {
        id: "assistant-persisted-artifact",
        role: "assistant",
        parts: [
          {
            type: "tool-agh__memory_recall",
            toolCallId: "tool-persisted-artifact",
            state: "output-available",
            turn_id: "turn-persisted-artifact",
            timestamp: "2026-07-19T04:32:00Z",
            input: { query: "release evidence" },
            output: {
              type: "tool_result",
              title: "Recall memory",
              raw: {
                content: "persisted bounded preview",
                raw_output: {
                  preview: "persisted bounded preview",
                  truncated: true,
                  artifacts: [toolArtifactRef()],
                },
              },
            },
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });
    const row = await screen.findByTestId("tool-call-row");
    const trigger = row.querySelector<HTMLElement>('[data-slot="tool-call-row-trigger"]');
    expect(trigger).not.toBeNull();
    await user.click(trigger as HTMLElement);

    expect(screen.getByText("persisted bounded preview")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Open full result" })).toBeInTheDocument();
  });

  it("Should fold settled turn work while keeping the terminal assistant text visible", async () => {
    const user = userEvent.setup();
    const transcript = [
      {
        id: "assistant-fold",
        role: "assistant",
        parts: [
          {
            type: "reasoning",
            text: "Need to inspect the launch file.",
            state: "done",
            turn_id: "turn-fold",
            timestamp: "2026-07-07T12:00:00Z",
          },
          {
            type: "tool-Read",
            toolCallId: "tool-read-fold",
            state: "output-available",
            turn_id: "turn-fold",
            timestamp: "2026-07-07T12:00:03Z",
            input: { file_path: "/tmp/launch.md" },
            output: {
              type: "tool_result",
              title: "Read",
              raw: { content: "Launch notes" },
            },
          },
          {
            type: "text",
            text: "Launch notes are ready.",
            state: "done",
            turn_id: "turn-fold",
            timestamp: "2026-07-07T12:00:05Z",
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

    expect(await screen.findByText("Launch notes are ready.")).toBeInTheDocument();
    const fold = screen.getByRole("button", { name: "Worked for 5s" });
    expect(screen.queryByText("/tmp/launch.md")).not.toBeInTheDocument();
    expect(fold).toHaveAttribute("aria-expanded", "false");

    await user.click(fold);

    expect(fold).toHaveAttribute("aria-expanded", "true");
    expect(await screen.findByText("/tmp/launch.md")).toBeInTheDocument();
    await user.click(screen.getByTestId("thinking-trigger"));
    expect(screen.getByText("Need to inspect the launch file.")).toBeInTheDocument();

    await user.click(fold);

    expect(fold).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByText("/tmp/launch.md")).not.toBeInTheDocument();
    expect(screen.queryByText("Need to inspect the launch file.")).not.toBeInTheDocument();
  });

  it("Should render the changed-files roll-up collapsed and expand it to per-file rows", async () => {
    const user = userEvent.setup();
    const transcript = [
      {
        id: "assistant-edits",
        role: "assistant",
        parts: [
          {
            type: "tool-Edit",
            toolCallId: "tool-edit-1",
            state: "output-available",
            turn_id: "turn-edits",
            timestamp: "2026-07-07T12:00:00Z",
            input: {
              file_path: "/app/src/launch.ts",
              old_string: "const status = 'pending'",
              new_string: "const status = 'ready'",
            },
            output: { type: "tool_result", title: "Edit", raw: { content: "ok" } },
          },
          {
            type: "tool-Write",
            toolCallId: "tool-write-1",
            state: "output-available",
            turn_id: "turn-edits",
            timestamp: "2026-07-07T12:00:02Z",
            input: { file_path: "/app/src/notes.md", content: "line1\nline2\nline3" },
            output: { type: "tool_result", title: "Write", raw: { content: "ok" } },
          },
          {
            type: "text",
            text: "Applied the edits.",
            state: "done",
            turn_id: "turn-edits",
            timestamp: "2026-07-07T12:00:04Z",
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

    // Collapsed: the audit summary line renders with no per-file list.
    const rollup = await screen.findByTestId("changed-files-row");
    expect(within(rollup).getByText("Edited 2 files")).toBeInTheDocument();
    const toggle = within(rollup).getByRole("button", { name: /Edited 2 files/ });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByTestId("changed-files-list")).not.toBeInTheDocument();

    await user.click(toggle);

    // Expanded: one row per modified file, each with its diff stats.
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    expect(await screen.findByTestId("changed-files-list")).toBeInTheDocument();
    const fileRows = within(rollup).getAllByTestId("changed-file-row");
    expect(fileRows).toHaveLength(2);
    expect(within(fileRows[0]!).getByText("launch.ts")).toBeInTheDocument();
    expect(within(fileRows[1]!).getByText("notes.md")).toBeInTheDocument();
  });

  it("Should keep an interrupted turn expanded and label the interruption", async () => {
    // A `stop_reason` data event is the runtime's operator-stop signal; the turn
    // must fold behind a "You stopped after Xs" label yet stay expanded (work
    // visible without a click), so the operator keeps their place.
    const transcript = [
      {
        id: "assistant-interrupted",
        role: "assistant",
        parts: [
          {
            type: "reasoning",
            text: "Reviewing the launch file.",
            state: "done",
            turn_id: "turn-int",
            timestamp: "2026-07-07T12:00:00Z",
          },
          {
            type: "tool-Read",
            toolCallId: "tool-read-int",
            state: "output-available",
            turn_id: "turn-int",
            timestamp: "2026-07-07T12:00:04Z",
            input: { file_path: "/tmp/interrupted.md" },
            output: { type: "tool_result", title: "Read", raw: { content: "partial" } },
          },
          {
            type: "data-agh-event",
            data: {
              type: "session-stopped",
              turn_id: "turn-int",
              stop_reason: "cancelled",
              timestamp: "2026-07-07T12:00:07Z",
            },
          },
          {
            type: "text",
            text: "Stopped before the summary.",
            state: "done",
            turn_id: "turn-int",
            timestamp: "2026-07-07T12:00:07Z",
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

    // The interruption is labeled and the fold has no collapse toggle.
    expect(await screen.findByText("You stopped after 7s")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /You stopped after/i })).not.toBeInTheDocument();
    // Work stays expanded without a click, and the terminal message is visible.
    expect(screen.getByText("/tmp/interrupted.md")).toBeInTheDocument();
    expect(screen.getByText("Stopped before the summary.")).toBeInTheDocument();
  });

  it("Should expose Goal only on settled successful assistant text while preserving copy", async () => {
    // Copy remains available for non-streaming partial/error output, but Goal prefill
    // requires an independently settled successful assistant response.
    const transcript = [
      {
        id: "user-first",
        role: "user",
        parts: [{ type: "text", text: "First question", state: "done" }],
      } as unknown as SessionMessage,
      {
        id: "assistant-settled",
        role: "assistant",
        parts: [
          {
            type: "text",
            text: "The launch note is ready and the checks are green.",
            state: "done",
            turn_id: "turn-settled",
            timestamp: "2026-07-07T12:00:05Z",
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
      {
        id: "user-second",
        role: "user",
        parts: [{ type: "text", text: "Second question", state: "done" }],
      } as unknown as SessionMessage,
      {
        id: "assistant-streaming",
        role: "assistant",
        parts: [
          {
            type: "text",
            text: "Still drafting the answer",
            state: "streaming",
            turn_id: "turn-streaming",
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
      {
        id: "assistant-incomplete",
        role: "assistant",
        status: { type: "incomplete", reason: "error", error: "provider stopped" },
        parts: [
          {
            type: "text",
            text: "Incomplete answer that remains copyable.",
            state: "done",
            turn_id: "turn-incomplete",
          },
        ] as unknown as SessionMessage["parts"],
      } as unknown as SessionMessage,
      {
        id: "assistant-persisted-failure",
        role: "assistant",
        parts: [
          {
            type: "text",
            text: "Persisted partial answer before failure.",
            state: "done",
            turn_id: "turn-persisted-failure",
          },
          {
            type: "data-agh-event",
            data: {
              type: "error",
              failure: { kind: "process_exit", summary: "provider exited" },
            },
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
      {
        id: "turn-9546159e1f16b3f2",
        role: "assistant",
        parts: [
          {
            type: "data-agh-event",
            data: { type: "system", title: "AGH Runtime Agent" },
          },
          {
            type: "text",
            text: "\n\nError: RetriableError: Connection stalled",
            state: "done",
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
      {
        id: "assistant-runtime-success",
        role: "assistant",
        parts: [
          {
            type: "data-agh-event",
            data: { type: "system", title: "AGH Runtime Agent" },
          },
          {
            type: "text",
            text: "Runtime check completed successfully.",
            state: "done",
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

    expect(
      await screen.findByText("The launch note is ready and the checks are green.")
    ).toBeInTheDocument();
    // The settled, incomplete, and persisted-failure text remain copyable. The
    // streaming turn renders no toolbar, and only the two successful turns expose Goal.
    const assistantToolbars = screen.getAllByTestId("assistant-message-actions");
    expect(assistantToolbars).toHaveLength(5);
    expect(screen.getAllByRole("button", { name: "Use as Goal" })).toHaveLength(2);
    expect(screen.getAllByTestId("assistant-message-actions-copy")).toHaveLength(5);
    const successfulToolbar = assistantToolbars[0]!;
    expect(
      within(successfulToolbar).getByRole("button", { name: "Use as Goal" })
    ).toBeInTheDocument();
    expect(
      within(successfulToolbar).getByTestId("assistant-message-actions-timestamp")
    ).toBeInTheDocument();
    expect(within(assistantToolbars[1]!).queryByRole("button", { name: "Use as Goal" })).toBeNull();
    expect(within(assistantToolbars[2]!).queryByRole("button", { name: "Use as Goal" })).toBeNull();
    expect(within(assistantToolbars[3]!).queryByRole("button", { name: "Use as Goal" })).toBeNull();
    expect(
      within(assistantToolbars[4]!).getByRole("button", { name: "Use as Goal" })
    ).toBeInTheDocument();
    // Both user prompts expose the copy affordance.
    expect(screen.getAllByTestId("user-message-actions")).toHaveLength(2);
  });

  it("Should copy the settled assistant message's markdown source and flip the copy icon to check", async () => {
    const writeText = vi.fn<(value: string) => Promise<void>>().mockResolvedValue(undefined);
    const clipboardDescriptor = Object.getOwnPropertyDescriptor(navigator, "clipboard");
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });

    const markdownSource = ["## Launch summary", "", "The launch note is ready."].join("\n");
    const transcript = [
      {
        id: "assistant-copy",
        role: "assistant",
        parts: [
          {
            type: "text",
            text: markdownSource,
            state: "done",
            turn_id: "turn-copy",
            timestamp: "2026-07-07T12:00:05Z",
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
    ];

    try {
      renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

      const copy = await screen.findByTestId("assistant-message-actions-copy");
      expect(copy.querySelector("svg.lucide-copy")).not.toBeNull();

      fireEvent.click(copy);

      // The whole markdown source lands on the clipboard, and the icon flips to a check.
      await waitFor(() => {
        expect(writeText).toHaveBeenCalledWith(markdownSource);
      });
      await waitFor(() => {
        expect(copy).toHaveAttribute("data-copied", "true");
      });
      expect(copy.querySelector("svg.lucide-check")).not.toBeNull();
      expect(copy.querySelector("svg.lucide-copy")).toBeNull();
    } finally {
      if (clipboardDescriptor) {
        Object.defineProperty(navigator, "clipboard", clipboardDescriptor);
      } else {
        Reflect.deleteProperty(navigator as unknown as { clipboard?: unknown }, "clipboard");
      }
    }
  });

  it("Should render the working row with typing dots and a live timer while a turn streams, and drop it once settled", async () => {
    // A streaming reasoning part marks the live turn; the settled turn carries only
    // a done text part. The working row must appear once (live turn), never on the
    // settled turn, and it must be the typing-dots + timer, not the old spinner.
    const startedAtIso = new Date(Date.now() - 5000).toISOString();
    const transcript = [
      {
        id: "assistant-settled-working",
        role: "assistant",
        parts: [
          {
            type: "text",
            text: "All launch checks are green.",
            state: "done",
            turn_id: "turn-settled-working",
            timestamp: "2026-07-07T11:59:00Z",
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
      {
        id: "assistant-live-working",
        role: "assistant",
        parts: [
          {
            type: "reasoning",
            text: "Reviewing the launch checklist before answering.",
            state: "streaming",
            turn_id: "turn-live-working",
            timestamp: startedAtIso,
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

    const workingRow = await screen.findByTestId("session-working-row");
    // Only the streaming turn carries the indicator; the settled turn drops it.
    expect(screen.getAllByTestId("session-working-row")).toHaveLength(1);
    // Typing dots wire the `typing-bounce` keyframe; the old spinner row is gone.
    expect(workingRow.querySelector('[data-slot="typing-dots"]')).not.toBeNull();
    expect(workingRow.querySelector(".animate-spin")).toBeNull();
    // Live "Working for Xs" tabular-nums timer, counting from the turn start.
    expect(workingRow).toHaveTextContent(/Working for/);
    const timer = screen.getByTestId("session-working-timer");
    expect(timer.className).toContain("tabular-nums");
    expect(timer.textContent).toMatch(/^\d+s$/);
  });

  it("Should degrade the working row to a static label under prefers-reduced-motion with no animation classes", async () => {
    const originalMatchMedia = window.matchMedia;
    window.matchMedia = ((query: string) => ({
      matches: query.includes("prefers-reduced-motion"),
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    })) as unknown as typeof window.matchMedia;

    try {
      const transcript = [
        {
          id: "assistant-reduced-motion",
          role: "assistant",
          parts: [
            {
              type: "reasoning",
              text: "Working through the remaining checks.",
              state: "streaming",
              turn_id: "turn-reduced-motion",
              timestamp: "2026-07-07T12:00:00Z",
            },
          ] as unknown as SessionMessage["parts"],
        } as SessionMessage,
      ];

      renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

      const workingRow = await screen.findByTestId("session-working-row");
      expect(workingRow).toHaveTextContent("Working…");
      // Static label only: no typing dots, no live timer, no animation classes.
      expect(workingRow.querySelector('[data-slot="typing-dots"]')).toBeNull();
      expect(screen.queryByTestId("session-working-timer")).not.toBeInTheDocument();
      expect(workingRow.innerHTML).not.toMatch(/animate-|typing-bounce/);
    } finally {
      window.matchMedia = originalMatchMedia;
    }
  });

  it("Should advance the working timer text without re-rendering the row per second", () => {
    // The self-ticking timer mutates its own text node, so the React tree must not
    // commit once per second while a turn streams.
    vi.useFakeTimers();
    try {
      vi.setSystemTime(new Date("2026-07-07T12:00:00Z"));
      const startedAt = Date.parse("2026-07-07T12:00:00Z");
      let renders = 0;
      function Probe() {
        renders += 1;
        return <WorkingIndicator startedAt={startedAt} reducedMotion={false} />;
      }

      render(<Probe />);
      expect(renders).toBe(1);
      const timer = screen.getByTestId("session-working-timer");
      expect(timer.textContent).toBe("0s");

      // Advancing the fake clock 3s also advances `Date.now()`, so the interval
      // ticks land at 1s/2s/3s past the anchored start.
      act(() => {
        vi.advanceTimersByTime(3000);
      });

      // The text node advanced to 3s while the React tree never re-rendered.
      expect(timer.textContent).toBe("3s");
      expect(renders).toBe(1);
    } finally {
      vi.useRealTimers();
    }
  });

  // jsdom leaves scroll metrics at 0, so the live-follow machine cannot observe a
  // gap on its own; mock the viewport geometry to place the reader away from the
  // live edge, then drive the same wheel/scroll gestures the hook listens for.
  function primeScrolledAwayViewport(viewport: HTMLElement) {
    Object.defineProperty(viewport, "scrollHeight", { configurable: true, value: 2000 });
    Object.defineProperty(viewport, "clientHeight", { configurable: true, value: 500 });
    viewport.scrollTop = 300;
  }

  it("Should reveal the scroll-to-bottom pill when the reader scrolls away and return to the live edge on click", async () => {
    const messages = toReadonlyThreadMessages(sessionTranscriptFixture.slice(0, 2));

    renderThreadState({ status: "success", messages });

    expect(await screen.findByText("Launch readiness snapshot")).toBeInTheDocument();
    const viewport = screen.getByTestId("chat-view");
    const pill = screen.getByTestId("scroll-to-bottom-pill");
    // Following the live edge: the pill is hidden and non-interactive.
    expect(pill).toHaveAttribute("data-visible", "false");

    // A manual wheel gesture opts out of live-follow; the machine reads the gap and
    // reveals the pill.
    primeScrolledAwayViewport(viewport);
    fireEvent.wheel(viewport);
    await waitFor(() => {
      expect(pill).toHaveAttribute("data-visible", "true");
    });

    // Clicking the pill returns to the live edge and hides the affordance again.
    fireEvent.click(pill);
    await waitFor(() => {
      expect(pill).toHaveAttribute("data-visible", "false");
    });
  });

  it("Should preserve the anchor row position when expanding a work-group disclosure", async () => {
    const transcript = [
      {
        id: "assistant-anchor-tools",
        role: "assistant",
        parts: Array.from({ length: 8 }, (_, index) => ({
          type: "tool-Read",
          toolCallId: `anchor-tool-${index + 1}`,
          state: "output-available",
          turn_id: "turn-anchor",
          timestamp: `2026-07-07T12:00:0${index}Z`,
          input: { file_path: `/tmp/anchor-${index + 1}.ts` },
          output: { type: "tool_result", title: "Read", raw: { content: `anchor-${index + 1}` } },
        })) as unknown as SessionMessage["parts"],
      } as SessionMessage,
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

    // Collapsed cluster: the latest call is visible; the reader has scrolled to a
    // fixed offset before expanding the group.
    expect(await screen.findByText("/tmp/anchor-8.ts")).toBeInTheDocument();
    const viewport = screen.getByTestId("chat-view");
    primeScrolledAwayViewport(viewport);

    // Expanding reveals the hidden calls without yanking the reader's position: the
    // anchor-preserving toggle corrects scrollTop by the height delta (0 under the
    // static jsdom geometry), so the reading offset is unchanged.
    fireEvent.click(screen.getByRole("button", { name: "+7 previous tool calls" }));

    expect(await screen.findByText("/tmp/anchor-1.ts")).toBeInTheDocument();
    expect(viewport.scrollTop).toBe(300);
  });
});

// Suite: streaming render-count probe (task 39).
// Invariant: derive-layer structural sharing + compiler-stabilized `TimelineRowContent` mean
// streaming N chunk updates change only the live row's visible subtree; settled
// rows keep both their data reference and mounted DOM subtree.
// Boundary IN: `computeStableSessionRows` row identity + the compiler-managed row renderer.
// Boundary OUT: SSE/query wiring (owned by use-session-live-tail.test.tsx).
describe("SessionThread streaming render-count", () => {
  function textPart(id: string, value: string, state: string, turnId: string): SessionTimelinePart {
    return { kind: "text", id, text: value, turnId, timestamp: "2026-07-07T12:00:00Z", state };
  }

  function StableTimeline({
    liveText,
    stateRef,
  }: {
    liveText: string;
    stateRef: { current: StableSessionRowsState };
  }) {
    // Mirror the production hook: re-derive fresh rows, then structurally share
    // against the prior pass so unchanged rows keep their reference.
    const derived = deriveSessionRows([
      textPart("settled", "Settled answer", "done", "turn-settled"),
      textPart("live", liveText, "streaming", "turn-live"),
    ]);
    const stable = computeStableSessionRows(derived, stateRef.current);
    stateRef.current = stable;
    return (
      <>
        {stable.result.map(row => (
          <div key={row.id} data-session-row-id={row.id}>
            <TimelineRowContent row={row} />
          </div>
        ))}
      </>
    );
  }

  it("Should preserve the settled row subtree while the live row changes across streaming chunks", () => {
    const stateRef: { current: StableSessionRowsState } = { current: EMPTY_STABLE_SESSION_ROWS };

    const { container, rerender } = render(
      <StableTimeline liveText="chunk 1" stateRef={stateRef} />
    );
    const settledRowAtMount = stateRef.current.result[0];
    const settledElement = container.querySelector<HTMLElement>(
      '[data-session-row-id="text:settled"]'
    );
    const liveElement = container.querySelector<HTMLElement>('[data-session-row-id="text:live"]');
    if (!settledElement || !liveElement) {
      throw new Error("Expected settled and live timeline rows to render");
    }
    const settledSubtreeAtMount = settledElement.firstElementChild;

    for (const chunk of [
      "chunk 1 chunk 2",
      "chunk 1 chunk 2 chunk 3",
      "chunk 1 chunk 2 chunk 3 chunk 4",
    ]) {
      rerender(<StableTimeline liveText={chunk} stateRef={stateRef} />);
      const nextSettledElement = container.querySelector('[data-session-row-id="text:settled"]');
      const nextLiveElement = container.querySelector('[data-session-row-id="text:live"]');

      expect(nextSettledElement).toBe(settledElement);
      expect(nextSettledElement?.firstElementChild).toBe(settledSubtreeAtMount);
      expect(nextLiveElement).toHaveTextContent(chunk);
    }

    expect(stateRef.current.result[0]).toBe(settledRowAtMount);
  });
});

// Suite: composer running-state semantics + queued-prompt strip (task 35).
// Invariant: Enter has one defined meaning per phase, the primary button reflects
// the phase, and queued prompts render as real, actionable rows.
// Boundary IN: SessionComposer phase toggle, Enter interception, queued-strip wiring.
// Boundary OUT: queue/steer/cancel API orchestration, covered by use-session-page-controls.test.tsx.

function renderComposer(overrides: Partial<ComponentProps<typeof SessionThread>>) {
  const queryClient = createQueryClient();
  render(
    <QueryClientProvider client={queryClient}>
      <SessionChatRuntimeProvider
        sessionId={primarySessionFixture.id}
        workspaceId={fixtureWorkspaceId()}
      >
        <SessionTranscriptThreadProvider
          messages={[]}
          status="success"
          error={null}
          retry={vi.fn()}
        >
          <SessionThread
            sessionId={primarySessionFixture.id}
            agentName={primarySessionFixture.agent_name}
            canPrompt
            onCancelPrompt={vi.fn()}
            {...overrides}
          />
        </SessionTranscriptThreadProvider>
      </SessionChatRuntimeProvider>
    </QueryClientProvider>
  );
}

describe("SessionThread composer running semantics", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", createFetchMock());
    vi.mocked(toast.error).mockClear();
    vi.mocked(toast.warning).mockClear();
    useSessionStore.getState().clearDraft(primarySessionFixture.id);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    useSessionStore.getState().clearDraft(primarySessionFixture.id);
  });

  it("Should queue the draft on Enter while running and suppress the runtime send", async () => {
    const user = userEvent.setup();
    const onQueuePrompt = vi.fn(() => Promise.resolve());
    renderComposer({ isSessionRunning: true, allowBusyInput: true, onQueuePrompt });

    const textarea = await screen.findByTestId("composer-textarea");
    await user.type(textarea, "queue this follow-up");
    // While running, assistant-ui's own thread is idle, so a plain Enter would submit
    // to the runtime; our interception must queue instead and clear the draft.
    await user.type(textarea, "{Enter}");

    await waitFor(() => {
      expect(onQueuePrompt).toHaveBeenCalledWith("queue this follow-up");
    });
    expect(onQueuePrompt).toHaveBeenCalledTimes(1);
    await waitFor(() => {
      expect((textarea as HTMLTextAreaElement).value).toBe("");
    });
    // The runtime send never fired, so no user message entered the thread.
    expect(screen.queryByText("queue this follow-up")).not.toBeInTheDocument();
  });

  it("Should show an error toast and preserve the draft when queue fails", async () => {
    const user = userEvent.setup();
    const onQueuePrompt = vi.fn(() => Promise.reject(new Error("queue failed")));
    renderComposer({ isSessionRunning: true, allowBusyInput: true, onQueuePrompt });

    const textarea = await screen.findByTestId("composer-textarea");
    await user.type(textarea, "queue this follow-up");
    await user.click(screen.getByTestId("composer-queue-button"));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("queue failed");
    });
    expect((textarea as HTMLTextAreaElement).value).toBe("queue this follow-up");
  });

  it("Should show the accent Send disc while idle and the danger Stop disc while running", async () => {
    const { rerender } = renderComposerRerenderable({ isSessionRunning: false });

    expect(await screen.findByTestId("composer-send-button")).toBeInTheDocument();
    expect(screen.queryByTestId("composer-stop-button")).not.toBeInTheDocument();
    expect(screen.queryByTestId("composer-enter-hint")).not.toBeInTheDocument();

    rerender({ isSessionRunning: true, allowBusyInput: true, onQueuePrompt: vi.fn() });

    const stop = await screen.findByTestId("composer-stop-button");
    expect(stop).toHaveAttribute("aria-label", "Stop generation");
    expect(screen.queryByTestId("composer-send-button")).not.toBeInTheDocument();
    expect(screen.getByTestId("composer-enter-hint")).toHaveTextContent(/Enter/);
  });

  it("Should render queued rows with steer, edit, and remove wired to real entries", async () => {
    const user = userEvent.setup();
    const onSteerQueuedPrompt = vi.fn();
    const onRemoveQueuedPrompt = vi.fn();
    renderComposer({
      isSessionRunning: true,
      allowBusyInput: true,
      onQueuePrompt: vi.fn(() => Promise.resolve()),
      onSteerQueuedPrompt,
      onRemoveQueuedPrompt,
      queuedPrompts: [
        { id: "inq-1", text: "Add a regression test." },
        { id: "inq-2", text: "Then update the docs." },
      ],
    });

    const rows = await screen.findAllByTestId("composer-queued-prompt-row");
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveTextContent("Add a regression test.");

    await user.click(within(rows[0]).getByTestId("composer-queued-steer"));
    expect(onSteerQueuedPrompt).toHaveBeenCalledWith({
      id: "inq-1",
      text: "Add a regression test.",
    });

    await user.click(within(rows[1]).getByTestId("composer-queued-remove"));
    expect(onRemoveQueuedPrompt).toHaveBeenCalledWith("inq-2");

    // Edit moves the queued text back into the composer and drops the queued entry.
    await user.click(within(rows[0]).getByTestId("composer-queued-edit"));
    const textarea = screen.getByTestId("composer-textarea") as HTMLTextAreaElement;
    await waitFor(() => {
      expect(textarea.value).toBe("Add a regression test.");
    });
    expect(onRemoveQueuedPrompt).toHaveBeenCalledWith("inq-1");
  });

  it("Should not overwrite an existing draft when editing a queued prompt", async () => {
    const user = userEvent.setup();
    const onRemoveQueuedPrompt = vi.fn();
    renderComposer({
      isSessionRunning: true,
      allowBusyInput: true,
      onQueuePrompt: vi.fn(() => Promise.resolve()),
      onSteerQueuedPrompt: vi.fn(),
      onRemoveQueuedPrompt,
      queuedPrompts: [{ id: "inq-1", text: "Queued prompt text." }],
    });

    const textarea = await screen.findByTestId("composer-textarea");
    await user.type(textarea, "Existing draft");
    const row = await screen.findByTestId("composer-queued-prompt-row");
    await user.click(within(row).getByTestId("composer-queued-edit"));

    expect((textarea as HTMLTextAreaElement).value).toBe("Existing draft");
    expect(onRemoveQueuedPrompt).not.toHaveBeenCalled();
    expect(toast.warning).toHaveBeenCalledWith(
      "Send or clear the current draft before editing a queued prompt."
    );
  });
});

function renderComposerRerenderable(overrides: Partial<ComponentProps<typeof SessionThread>>) {
  const queryClient = createQueryClient();
  const baseProps = {
    sessionId: primarySessionFixture.id,
    agentName: primarySessionFixture.agent_name,
    canPrompt: true,
    onCancelPrompt: vi.fn(),
  } satisfies Partial<ComponentProps<typeof SessionThread>>;

  const tree = (props: Partial<ComponentProps<typeof SessionThread>>) => (
    <QueryClientProvider client={queryClient}>
      <SessionChatRuntimeProvider
        sessionId={primarySessionFixture.id}
        workspaceId={fixtureWorkspaceId()}
      >
        <SessionTranscriptThreadProvider
          messages={[]}
          status="success"
          error={null}
          retry={vi.fn()}
        >
          <SessionThread {...baseProps} {...props} />
        </SessionTranscriptThreadProvider>
      </SessionChatRuntimeProvider>
    </QueryClientProvider>
  );

  const view = render(tree(overrides));
  return {
    rerender: (props: Partial<ComponentProps<typeof SessionThread>>) => view.rerender(tree(props)),
  };
}
