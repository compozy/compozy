import { SessionPromptRuntimeContext } from "@/systems/session/contexts/session-prompt-runtime-context-value";
import {
  sessionPromptRuntimeInput,
  sessionPromptRuntimeStoreLogic,
  type SessionPromptRuntimeStore,
} from "@/systems/session/stores/session-prompt-runtime-store";
import { StrictMode, useEffect, useLayoutEffect, type ComponentProps } from "react";
import { useAui, type ThreadMessage } from "@assistant-ui/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  act,
  fireEvent,
  render,
  renderHook,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { UIProvider } from "@compozy/ui";
import { agentKeys } from "@/systems/agent";
import { AgentIdentityProvider } from "@/systems/agent/components/agent-identity-provider";
import { SessionMentionChip } from "../session-composer-mention-chip";
import {
  $getRoot,
  $getSelection,
  $isRangeSelection,
  $isTextNode,
  $setSelection,
  getNearestEditorFromDOMNode,
  CONTROLLED_TEXT_INSERTION_COMMAND,
  UNDO_COMMAND,
  type ElementNode,
} from "lexical";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { OverlayContainerContext, toast } from "@compozy/ui";

import { useThinkingGuardElapsed } from "../hooks/use-thinking-guard-elapsed";
import { useSessionComposerState } from "../hooks/use-session-composer-state";

import { resetGatewayStreamAuth } from "@/lib/gateway-stream-auth";
import {
  isSessionRunning,
  SessionApiError,
  SessionBusyInputRefusalError,
  type SessionBusyInputDraft,
  type SessionSendOutcome,
} from "@/systems/session";
import { SessionChatRuntimeProvider } from "@/systems/session/components/session-chat-runtime-provider";
import { primarySessionFixture, sessionTranscriptFixture } from "@/systems/session/mocks/fixtures";
import { SessionTranscriptThreadProvider } from "@/systems/session/lib/session-transcript-thread-context";
import {
  getSessionDebugCounters,
  getSessionDebugEvents,
  resetSessionDebugTelemetry,
  SESSION_DEBUG_EVENTS,
} from "@/systems/session/lib/session-observability";
import { normalizeTranscriptMessages } from "@/systems/session/lib/message-schemas";
import { toReadonlyThreadMessages } from "@/systems/session/lib/session-thread-repository";
import type { SessionFailurePayload, SessionMessage, SessionState } from "@/systems/session/types";
import { SESSION_TRANSPORT_LIVE } from "@/systems/session/lib/session-transport";
import type { SessionTransportState } from "@/systems/session/lib/session-transcript-thread-context-value";
import type { SessionTranscriptThreadStatus } from "@/systems/session/lib/session-transcript-thread-context-value";
import { sessionKeys } from "@/systems/session/lib/query-keys";
import type { SessionTranscriptData } from "@/systems/session/lib/session-transcript-query";

import {
  SessionOriginContext,
  type SessionOriginContextValue,
} from "@/systems/session/contexts/session-origin-context-value";
import {
  SessionForkContext,
  type SessionForkRequest,
} from "@/systems/session/contexts/session-fork-context-value";
import { continuedSessionFixture } from "@/systems/session/mocks/derive-fixtures";
import {
  STEER_WAKE_TURN,
  steeredWakesTranscript,
} from "@/systems/session/components/stories/sessions-stability-steer-fixtures";
import { sessionOriginView } from "@/systems/session/lib/session-origin";
import { SessionThread } from "../session-thread";
import { readCursorSnapshot } from "../session-composer-cursor-snapshot";
import { formatDataPreview } from "../session-message-parts.logic";
import { SessionThinkingRow } from "@/systems/session/components/session-thinking-row";
import {
  clearSessionTerminalQuote,
  composeQuotedPrompt,
  peekSessionTerminalQuote,
  sessionStore,
} from "@/systems/session";
import {
  buildTerminalQuote,
  holdPendingTerminalQuote,
  peekPendingTerminalQuote,
  takePendingTerminalQuote,
} from "@/systems/terminal/parts";
import { discardComposerDraftAttachments, resetSessionPromptRecovery } from "@/systems/session";

beforeEach(() => {
  vi.spyOn(toast, "error").mockReturnValue("toast");
  vi.spyOn(toast, "success").mockReturnValue("toast");
  vi.spyOn(toast, "warning").mockReturnValue("toast");
});

// Spied so the composer suites can count document walks per editor update.
vi.mock("../session-composer-cursor-snapshot", async importOriginal => {
  const actual = await importOriginal<typeof import("../session-composer-cursor-snapshot")>();
  return { ...actual, readCursorSnapshot: vi.fn(actual.readCursorSnapshot) };
});

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

function jsonResponse(body: unknown, init?: ResponseInit) {
  return new Response(JSON.stringify(body), {
    status: init?.status ?? 200,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
    },
  });
}

function pngFile(name = "shot.png") {
  const header = Uint8Array.of(0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a);
  return new File([header, new Uint8Array(24)], name, { type: "image/png" });
}

function pdfFile(name = "report.pdf") {
  return new File(["%PDF-1.7\nattachment"], name, { type: "application/pdf" });
}

function zipFile() {
  return new File([Uint8Array.of(0x50, 0x4b, 0x03, 0x04)], "archive.zip", {
    type: "application/zip",
  });
}

let attachmentUploadSequence = 0;

function nextAttachmentUploadIdentity(): { id: string; sha256: string } {
  attachmentUploadSequence += 1;
  const digest = attachmentUploadSequence.toString(16).padStart(64, "0");
  return { id: `att_${digest}`, sha256: digest };
}

function attachmentUploadPayload({
  id,
  kind,
  mimeType,
  name = "shot.png",
  sha256,
}: {
  id?: string;
  kind?: "file" | "image";
  mimeType?: string;
  name?: string;
  sha256?: string;
} = {}) {
  const identity = nextAttachmentUploadIdentity();
  const isPNG = name.endsWith(".png");
  return {
    attachment: {
      bytes: 32,
      created_at: "2026-08-15T00:00:00Z",
      height: 10,
      id: id ?? identity.id,
      kind: kind ?? (isPNG ? "image" : "file"),
      mime_type: mimeType ?? (isPNG ? "image/png" : "application/pdf"),
      name,
      sha256: sha256 ?? identity.sha256,
      width: 10,
    },
  };
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

interface UploadedFileMetadata {
  name: string;
  type: string;
}

function uploadedFileMetadata(entry: FormDataEntryValue | null): UploadedFileMetadata | null {
  if (entry === null || typeof entry === "string") {
    return null;
  }
  return { name: entry.name, type: entry.type };
}

async function uploadedFileFromRequest(
  input: RequestInfo | URL,
  init?: RequestInit
): Promise<UploadedFileMetadata | null> {
  if (init?.body instanceof FormData) {
    return uploadedFileMetadata(init.body.get("file"));
  }
  if (!(input instanceof Request)) {
    return null;
  }
  const body = await input.clone().text();
  const filename = body.match(/filename="([^"]+)"/)?.[1];
  const type = body.match(/\r?\nContent-Type:\s*([^\r\n]+)/i)?.[1];
  return filename && type ? { name: filename, type } : null;
}

function fixtureWorkspaceId(): string {
  const workspaceId = primarySessionFixture.workspace_id;
  if (!workspaceId) {
    throw new Error("primary session fixture must include workspace_id");
  }
  return workspaceId;
}

const TOOL_ARTIFACT_URI = `compozy://tool-artifacts/art_${"c".repeat(64)}`;

function toolArtifactRef() {
  return {
    uri: TOOL_ARTIFACT_URI,
    name: "tool-result.json",
    mime_type: "application/vnd.compozy.tool-result+json",
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

/** A completed prompt stream, so a delivered turn settles instead of hanging. */
function sseResponse(frames: string[]) {
  const encoder = new TextEncoder();
  return new Response(
    new ReadableStream({
      start(controller) {
        for (const frame of frames) {
          controller.enqueue(encoder.encode(frame));
        }
        controller.close();
      },
    }),
    {
      headers: {
        "Content-Type": "text/event-stream",
        "x-vercel-ai-ui-message-stream": "v1",
      },
    }
  );
}

/** Records every prompt POST body so a duplicated turn is visible, not inferred. */
function createPromptRecordingFetchMock(
  promptCalls: string[],
  options?: Parameters<typeof createFetchMock>[0]
) {
  const base = createFetchMock(options);
  return vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const pathname = getPathname(input);
    // A prompt POST streams, so the client identifies the listener first; a local
    // tier needs no ticket, which is the ordinary same-origin case.
    if (pathname === "/api/status") {
      return new Response(JSON.stringify({}), {
        headers: {
          "Content-Type": "application/json",
          "X-Compozy-Gateway-Tier": "local",
        },
      });
    }
    if (pathname.endsWith("/prompt")) {
      promptCalls.push(String(init?.body ?? ""));
      return sseResponse([]);
    }
    return base(input, init);
  });
}

function createPromptRejectedFetchMock() {
  const base = createFetchMock();
  return vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const pathname = getPathname(input);
    if (pathname === "/api/status") {
      return new Response(JSON.stringify({}), {
        headers: {
          "Content-Type": "application/json",
          "X-Compozy-Gateway-Tier": "local",
        },
      });
    }
    if (pathname.endsWith("/prompt")) {
      return jsonResponse(
        {
          code: "acp_prompt_image_unsupported",
          error: "The selected agent does not accept images",
        },
        { status: 422 }
      );
    }
    return base(input, init);
  });
}

function createFetchMock(options?: {
  attachmentUpload?: Parameters<typeof attachmentUploadPayload>[0];
  beforeAttachmentUpload?: () => Promise<void>;
}) {
  return vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const pathname = getPathname(input);

    if (pathname === "/api/sessions") {
      return jsonResponse({ sessions: [primarySessionFixture] });
    }

    if (
      pathname ===
      `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/${primarySessionFixture.id}`
    ) {
      // The harness session is idle unless a test serves a running detail itself.
      return jsonResponse({ session: { ...primarySessionFixture, badge: "idle" } });
    }

    if (
      pathname ===
      `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/${primarySessionFixture.id}/transcript`
    ) {
      return jsonResponse({ entries: [] });
    }

    if (
      pathname ===
      `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/${primarySessionFixture.id}/clarifications`
    ) {
      return jsonResponse({ clarifications: [] });
    }

    if (
      pathname ===
      `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/${primarySessionFixture.id}/interactions`
    ) {
      return jsonResponse({ interactions: [] });
    }

    if (
      pathname ===
      `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/${primarySessionFixture.id}/prompt/queue`
    ) {
      return jsonResponse({ inputs: [] });
    }

    if (
      pathname ===
      `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/${primarySessionFixture.id}/attachments`
    ) {
      if (options?.beforeAttachmentUpload) {
        await options.beforeAttachmentUpload();
      }
      const uploaded = await uploadedFileFromRequest(input, init);
      if (uploaded === null) {
        return jsonResponse({ error: "file is required" }, { status: 400 });
      }
      if (uploaded.type === "application/zip") {
        return jsonResponse({ error: "unsupported mime type" }, { status: 415 });
      }
      return jsonResponse(
        attachmentUploadPayload(
          options?.attachmentUpload ?? {
            kind: uploaded.type.startsWith("image/") ? "image" : "file",
            mimeType: uploaded.type,
            name: uploaded.name,
          }
        ),
        { status: 201 }
      );
    }

    if (
      pathname.startsWith(
        `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/${primarySessionFixture.id}/attachments/`
      )
    ) {
      return new Response(null, { status: 204 });
    }

    throw new Error(`Unhandled fetch in thread test: ${pathname}`);
  });
}

// The session resource the status row reads while a turn runs: a durable turn
// start five seconds ago and the current tool (S3, US-027).
function runningStatusSession(
  startedAtIso = new Date(Date.now() - 5000).toISOString(),
  currentTool = "Bash"
): NonNullable<ComponentProps<typeof SessionThread>["statusSession"]> {
  return {
    ...primarySessionFixture,
    activity: {
      current_tool: currentTool,
      elapsed_ms: 5000,
      elapsed_seconds: 5,
      idle_seconds: 0,
      iteration_current: 1,
      iteration_max: 1,
      turn_id: "turn-live",
      turn_started_at: startedAtIso,
    },
  };
}

interface ThreadStateOptions {
  messages?: readonly ThreadMessage[];
  status: SessionTranscriptThreadStatus;
  error?: Error | null;
  retry?: () => void;
  isSessionRunning?: boolean;
  statusSession?: ComponentProps<typeof SessionThread>["statusSession"];
  stopCompletionNote?: boolean;
  stopPhase?: ComponentProps<typeof SessionThread>["stopPhase"];
  acpSessionId?: string;
  sessionState?: SessionState;
  failure?: SessionFailurePayload | null;
  durableMessageIds?: string[];
  readOnly?: boolean;
  transport?: SessionTransportState;
  liveDataEnabled?: boolean;
  contextControl?: ComponentProps<typeof SessionThread>["contextControl"];
  subagentControl?: ComponentProps<typeof SessionThread>["subagentControl"];
  origin?: SessionOriginContextValue | null;
  /** The Fork dialog host's request; absent outside a host ("Fork from here" hides). */
  forkRequest?: SessionForkRequest | null;
  subagentNavigation?: ComponentProps<typeof SessionThread>["subagentNavigation"];
}

function threadStateElement(
  queryClient: QueryClient,
  {
    messages = [],
    status,
    error = null,
    retry = vi.fn(),
    isSessionRunning = false,
    statusSession = isSessionRunning ? runningStatusSession() : primarySessionFixture,
    stopCompletionNote = false,
    stopPhase = "idle",
    acpSessionId,
    sessionState,
    failure = null,
    durableMessageIds = [],
    readOnly = false,
    transport,
    liveDataEnabled = true,
    contextControl,
    subagentControl,
    origin = null,
    forkRequest = null,
    subagentNavigation = null,
  }: ThreadStateOptions
) {
  if (durableMessageIds.length > 0) {
    const entries: SessionTranscriptData["pages"][number]["entries"] = durableMessageIds.map(
      (id, index) => ({
        start_sequence: index + 1,
        sequence: index + 1,
        message: { id, role: "user", parts: [{ type: "text", text: id, state: "done" }] },
      })
    );
    const transcriptData: SessionTranscriptData = {
      pageParams: [undefined],
      pages: [
        {
          cursor: entries.length,
          entries,
          epoch: 1,
          generation: 1,
          has_older: false,
          limit: 200,
          max_sequence: entries.length,
        },
      ],
    };
    queryClient.setQueryData<SessionTranscriptData>(
      sessionKeys.transcript(fixtureWorkspaceId(), primarySessionFixture.id),
      transcriptData
    );
  }

  return (
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
          transport={transport}
        >
          <SessionOriginContext value={origin}>
            <SessionForkContext value={forkRequest}>
              <SessionThread
                sessionId={primarySessionFixture.id}
                agentName={primarySessionFixture.agent_name}
                canPrompt
                onCancelPrompt={() => {}}
                isSessionRunning={isSessionRunning}
                statusSession={statusSession}
                stopCompletionNote={stopCompletionNote}
                stopPhase={stopPhase}
                acpSessionId={acpSessionId}
                sessionState={sessionState}
                failure={failure}
                readOnly={readOnly}
                liveDataEnabled={liveDataEnabled}
                contextControl={contextControl}
                subagentControl={subagentControl}
                subagentNavigation={subagentNavigation}
              />
            </SessionForkContext>
          </SessionOriginContext>
        </SessionTranscriptThreadProvider>
      </SessionChatRuntimeProvider>
    </QueryClientProvider>
  );
}

function renderThreadState(
  options: ThreadStateOptions,
  queryClient: QueryClient = createQueryClient()
) {
  const result = render(threadStateElement(queryClient, options));
  return {
    ...result,
    /** Re-renders the same host with changed options (a transcript transition). */
    rerenderWith: (next: Partial<ThreadStateOptions>) =>
      result.rerender(threadStateElement(queryClient, { ...options, ...next })),
  };
}

// The recovery owner is app-scoped: a draft one test leaves pending must not reach the next.
afterEach(() => {
  resetSessionPromptRecovery();
  discardComposerDraftAttachments(primarySessionFixture.id);
});

describe("SessionThread transcript states", () => {
  // Invariant: a supplied context action reaches the composer in its host thread; owner: thread/composer composition.
  it("Should keep the context action in the composer for an inactive session", () => {
    renderThreadState({
      status: "success",
      sessionState: "stopped",
      contextControl: <button type="button">Inspect context</button>,
    });
    expect(
      screen
        .getByRole("button", { name: "Inspect context" })
        .closest('[data-testid="composer-shell"]')
    ).not.toBeNull();
  });

  // Invariant: the subagents control sits after the context ring and before Attach files (#766);
  // owner: composer toolbar composition.
  it("Should place the subagents control between the context action and Attach files", () => {
    renderThreadState({
      status: "success",
      contextControl: <button type="button">Inspect context</button>,
      subagentControl: <button type="button">Subagents</button>,
    });
    const order = [
      screen.getByRole("button", { name: "Inspect context" }),
      screen.getByRole("button", { name: "Subagents" }),
      screen.getByRole("button", { name: "Attach files" }),
    ];
    for (const [before, after] of [
      [order[0]!, order[1]!],
      [order[1]!, order[2]!],
    ] as const) {
      expect(before.compareDocumentPosition(after) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    }
  });

  it("Should preview non-JSON values without returning undefined", () => {
    const value = () => "result";
    expect(formatDataPreview(value)).toBe(String(value));
  });

  beforeEach(() => {
    attachmentUploadSequence = 0;
    resetSessionDebugTelemetry();
    vi.stubGlobal("fetch", createFetchMock());
    vi.mocked(toast.error).mockClear();
    vi.mocked(toast.success).mockClear();
    vi.mocked(toast.warning).mockClear();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    resetSessionDebugTelemetry();
  });

  it("Should render the transcript skeleton on pending without empty-state copy", async () => {
    renderThreadState({ status: "pending" });

    // One busy status names the loading area; the message-shaped placeholders stay silent.
    const region = await screen.findByRole("status");
    expect(region).toHaveTextContent(/loading transcript/i);
    expect(region).toHaveAttribute("aria-busy", "true");
    expect(screen.getByTestId("thread-transcript-skeleton")).toBe(region);
    expect(screen.getAllByRole("status")).toHaveLength(1);
    expect(screen.queryByText(/Start a conversation/i)).not.toBeInTheDocument();
  });

  // Invariant: standalone durable clear records are one visible trace across
  // their operational siblings; removals, history identity and reply survive.
  // Owner: thread rendering, canonical SessionThread transcript-state suite.
  it("Should render standalone queue clears as one trace without losing the reply", async () => {
    const cleared = (id: string, marker: boolean): SessionMessage =>
      ({
        id,
        role: "assistant",
        parts: [
          {
            type: "data-compozy-event",
            data: marker
              ? {
                  type: "transcript_marker.created",
                  raw: {
                    kind: "transcript_marker.queue_cleared",
                    summary: "Queued input removed by explicit clear.",
                    occurred_at: "2026-09-06T14:00:00Z",
                    evidence: { queue_entry_id: id, actor_kind: "user" },
                  },
                }
              : { type: "session.queue_cleared", raw: { queue_entry_id: id } },
          },
        ],
      }) as SessionMessage;
    const transcript = [
      cleared("clear-1", true),
      cleared("event-1", false),
      cleared("clear-2", true),
      cleared("event-2", false),
      {
        id: "removed-1",
        role: "assistant",
        parts: [
          {
            type: "data-compozy-event",
            data: {
              type: "transcript_marker.created",
              raw: {
                kind: "transcript_marker.prompt_dropped",
                summary: "Queued input canceled by operator.",
                occurred_at: "2026-09-06T14:00:01Z",
                evidence: { queue_status: "canceled", mode: "queue" },
              },
            },
          },
        ],
      } as SessionMessage,
      {
        id: "continued",
        role: "assistant",
        parts: [{ type: "text", text: "The reply continues." }],
      } as SessionMessage,
    ];
    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });
    const traces = await screen.findAllByTestId("transcript-marker-notice");
    expect(traces).toHaveLength(2);
    expect(traces[0]).toHaveTextContent("×2");
    expect(traces[1]).toHaveTextContent("You removed a queued follow-up");
    expect(screen.getByText("The reply continues.")).toBeInTheDocument();
    for (const message of transcript) {
      expect(document.querySelector(`[data-message-id="${message.id}"]`)).toBeInTheDocument();
    }
  });

  // Invariant (S18): an observed compaction is a `system` message the thread renders
  // as one visible row per compaction id, and a later projection of the same id
  // updates that row in place — never a second row, never a dropped one.
  // Owner: thread rendering, canonical SessionThread transcript-state suite.
  it("Should render a compaction message as one row and update it in place by id", async () => {
    const compaction = (data: Record<string, unknown>): SessionMessage =>
      ({
        id: "compaction:c1",
        role: "system",
        parts: [
          {
            type: "data-compozy-compaction",
            id: "c1",
            data: {
              kind: "compaction",
              compaction_id: "c1",
              started_at: "2026-10-08T14:02:11Z",
              ...data,
            },
          },
        ],
      }) as SessionMessage;
    const surrounding = (item: SessionMessage) =>
      toReadonlyThreadMessages([
        { id: "ask", role: "user", parts: [{ type: "text", text: "Wrap up." }] } as SessionMessage,
        item,
        {
          id: "after",
          role: "assistant",
          parts: [{ type: "text", text: "Ready for more." }],
        } as SessionMessage,
      ]);

    const { rerenderWith } = renderThreadState({
      status: "success",
      messages: surrounding(compaction({ status: "in_progress" })),
    });
    const row = await screen.findByTestId("session-compaction-item");
    expect(row).toHaveAttribute("data-status", "in_progress");
    expect(row).toHaveTextContent("Compacting context…");
    expect(screen.getByText("Ready for more.")).toBeInTheDocument();

    rerenderWith({
      messages: surrounding(
        compaction({
          status: "completed",
          summary: "Moved invoices to the ledger API.",
          ended_at: "2026-10-08T14:02:39Z",
        })
      ),
    });

    const updated = await screen.findAllByTestId("session-compaction-item");
    expect(updated).toHaveLength(1);
    expect(updated[0]).toHaveAttribute("data-status", "completed");
    expect(updated[0]).toHaveTextContent("Context compacted");
    expect(updated[0]).not.toHaveTextContent("Compacting context…");
    expect(within(updated[0]!).getByRole("button", { name: "Summary" })).toBeInTheDocument();
    expect(
      document.querySelectorAll(
        '[data-testid="thread-message-row"][data-message-id="compaction:c1"]'
      )
    ).toHaveLength(1);
  });

  // Invariant (S18): a requested compaction is followed, in the same turn, by the agent's own
  // reply. The thread shows the compaction row AND that reply; neither hides the other.
  // Owner: thread rendering over the daemon's real projection (user /compact, the system
  // compaction message, the quiet fired event, then the assistant text with its maintenance
  // `prompt_delivery`, whose `spans` is null because `/compact` carries no sections). The
  // transcript is normalized exactly as the live tail does; a rejected frame hides the reply.
  it("Should render the assistant text that follows a compaction item in the same turn", async () => {
    const turn = "turn-955a640065514905";
    const transcript = [
      {
        id: "ask",
        role: "user",
        metadata: { turn_id: "turn-afe4f209a549163f" },
        parts: [{ type: "text", text: "baseline", state: "done" }],
      },
      {
        id: "turn-afe4f209a549163f",
        role: "assistant",
        parts: [{ type: "text", text: "baseline ready", state: "done" }],
      },
      {
        id: "ev-compact",
        role: "user",
        metadata: { turn_id: turn },
        parts: [{ type: "text", text: "/compact", state: "done" }],
      },
      {
        id: "compaction:native-1",
        role: "system",
        parts: [
          {
            type: "data-compozy-compaction",
            id: "native-1",
            data: {
              kind: "compaction",
              compaction_id: "native-1",
              status: "completed",
              summary: "Retain the agreed project constraints.",
              started_at: "2026-10-08T11:50:54.762Z",
              ended_at: "2026-10-08T11:50:57.765Z",
            },
          },
        ],
      },
      {
        id: `${turn}-2`,
        role: "assistant",
        parts: [
          {
            type: "data-compozy-event",
            data: {
              type: "session.compaction_fired",
              session_id: "s",
              turn_id: turn,
              timestamp: "2026-10-08T11:50:54.762Z",
              raw: { compaction_id: "native-1", trigger: "requested" },
            },
          },
        ],
      },
      {
        id: `${turn}-3`,
        role: "assistant",
        parts: [
          {
            type: "text",
            id: `${turn}-3-text-1`,
            text: "Native compaction observed.",
            state: "done",
          },
          {
            type: "data-compozy-event",
            data: {
              type: "prompt_delivery",
              session_id: "s",
              turn_id: turn,
              timestamp: "2026-10-08T11:50:57.821Z",
              delivery: {
                turn_id: turn,
                sent_at: "2026-10-08T11:50:54.762Z",
                estimate: "bytes_div_4",
                spans: null,
              },
            },
          },
        ],
      },
    ] as unknown as SessionMessage[];
    const normalized = await normalizeTranscriptMessages(transcript);
    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(normalized) });

    expect(await screen.findByTestId("session-compaction-item")).toHaveTextContent(
      "Context compacted"
    );
    expect(screen.getByText("Native compaction observed.")).toBeInTheDocument();
  });

  it("Should render a conversation whose turn carries an unnamed or malformed part", async () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const transcript = [
      {
        id: "goal-turn-1",
        role: "assistant",
        parts: [
          { type: "text", id: "goal-turn-1-text-1", text: "Running the gate.", state: "done" },
          {
            // A failed result whose provider never reported the tool: no toolName, no errorText.
            type: "dynamic-tool",
            state: "output-error",
            toolCallId: "exec-orphan",
            input: {},
            output: { status: "failed", raw: { raw_output: { exit_code: 2 } } },
          },
        ],
      },
      {
        id: "goal-turn-2",
        role: "assistant",
        parts: [
          { type: "reasoning", id: "goal-turn-2-reasoning-1", text: 42, state: "done" },
          { type: "text", id: "goal-turn-2-text-1", text: "Gate fixed on main.", state: "done" },
        ],
      },
      {
        // Every part is invalid: the message goes, the conversation stays.
        id: "user-emptied",
        role: "user",
        parts: [{ type: "text", text: 42 }],
      },
    ] as unknown as SessionMessage[];

    const normalized = await normalizeTranscriptMessages(transcript);
    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(normalized) });

    expect(await screen.findByText("Running the gate.")).toBeInTheDocument();
    expect(screen.getByText("Gate fixed on main.")).toBeInTheDocument();
    expect(normalized.map(message => message.id)).toEqual(["goal-turn-1", "goal-turn-2"]);
    expect(normalized[1]?.parts.map(part => part.type)).toEqual(["text"]);
    // A malformed envelope is never mistaken for a message of invalid parts.
    const malformed = { id: "bogus-role", role: "bogus", parts: [{ type: "text", text: "kept" }] };
    await expect(normalizeTranscriptMessages([...transcript, malformed])).rejects.toThrow();
    expect(warn).toHaveBeenCalledWith(
      "Dropped an invalid transcript part",
      expect.objectContaining({ messageId: "goal-turn-2", partIndex: 0, partType: "reasoning" })
    );
    warn.mockRestore();
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
    expect(within(pane).getByText("Couldn't load this conversation")).toBeInTheDocument();
    expect(screen.getByTestId("thread-transcript-error-detail")).toHaveTextContent(
      "recorder temporarily unavailable"
    );
    expect(screen.queryByText(/Start a conversation/i)).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Try again" }));

    expect(retry).toHaveBeenCalledTimes(1);
  });

  it("Should render past a resolved clarification with an unstable structured payload", async () => {
    const transcript: SessionMessage[] = [
      {
        id: "assistant-resolved-clarification",
        role: "assistant",
        parts: [
          {
            type: "data-compozy-event",
            data: {
              type: "clarify",
              session_id: primarySessionFixture.id,
              turn_id: "clarify:req-render-loop",
              raw: {
                status: "resolved",
                request: {
                  request_id: "req-render-loop",
                  workspace_id: primarySessionFixture.workspace_id,
                  session_id: primarySessionFixture.id,
                  agent_name: primarySessionFixture.agent_name,
                  question: "Which environment should deploy?",
                  choices: ["staging", "production"],
                  asked_at: "2026-08-12T12:00:00Z",
                  deadline: "2026-08-12T12:05:00Z",
                },
                answer: { choice: 1, text: "", fallback: false },
                at: "2026-08-12T12:00:10Z",
              },
            },
          },
        ],
      },
      {
        id: "assistant-after-resolved-clarification",
        role: "assistant",
        parts: [
          {
            type: "text",
            text: "Deployment checks completed after the clarification.",
            state: "done",
          },
        ],
      },
    ];

    const messages = toReadonlyThreadMessages(transcript);
    const clarificationMessage = messages[0];
    expect.assert(clarificationMessage);
    const clarificationPart = clarificationMessage.content[0];
    expect.assert(clarificationPart);
    const volatilePart = new Proxy(clarificationPart, {
      get(target, property, receiver) {
        const value = Reflect.get(target, property, receiver);
        return property === "data" ? structuredClone(value) : value;
      },
    });
    const volatileContent = [volatilePart, ...clarificationMessage.content.slice(1)];
    const volatileClarification = new Proxy(clarificationMessage, {
      get(target, property, receiver) {
        if (property === "content") {
          return volatileContent;
        }
        return Reflect.get(target, property, receiver);
      },
    });

    renderThreadState({
      status: "success",
      messages: [volatileClarification, ...messages.slice(1)],
    });

    expect(await screen.findByTestId("clarification-receipt")).toHaveAttribute(
      "data-status",
      "resolved"
    );
    expect(screen.getByTestId("clarification-receipt-answer")).toHaveTextContent("production");
    expect(
      screen.getByText("Deployment checks completed after the clarification.")
    ).toBeInTheDocument();
    expect(screen.queryByTestId("session-thread-error-boundary")).not.toBeInTheDocument();
  });

  it("Should render ThreadEmpty only for success with zero messages", async () => {
    renderThreadState({ status: "success" });

    expect(await screen.findByText(/to get started/i)).toBeInTheDocument();
    expect(screen.queryByTestId("thread-transcript-skeleton")).not.toBeInTheDocument();
    expect(screen.queryByTestId("thread-transcript-error")).not.toBeInTheDocument();
  });

  // Invariant: a transcript that has said nothing yet reads as empty. Hook dispatches and
  // other status-only events render no row, so a fresh session holding only those keeps
  // the generic empty state instead of a blank viewport.
  it("Should keep ThreadEmpty for a fresh session whose transcript holds only status events", async () => {
    const transcript = [
      {
        id: "hook-start",
        role: "assistant",
        parts: [{ type: "data-compozy-event", data: { type: "hook.dispatch.start" } }],
      },
      {
        id: "hook-complete",
        role: "assistant",
        parts: [{ type: "data-compozy-event", data: { type: "hook.dispatch.complete" } }],
      },
    ] as SessionMessage[];
    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

    expect(await screen.findByText(/to get started/i)).toBeInTheDocument();
  });

  // Invariant (UT-071): a derived child marks where its own transcript starts — one divider
  // before its first message, or the divider alone above a compact empty state before any.
  // Owning layer: the thread message list. Canonical suite: this file.
  it("Should render the origin divider once, before the derived child's first message", async () => {
    const user = userEvent.setup();
    const onOpenSource = vi.fn();
    const child = continuedSessionFixture();
    const transcript = [
      { id: "child-first", role: "user", parts: [{ type: "text", text: "Pick up the tests." }] },
      { id: "child-reply", role: "assistant", parts: [{ type: "text", text: "On it." }] },
    ] as SessionMessage[];
    renderThreadState({
      status: "success",
      messages: toReadonlyThreadMessages(transcript),
      origin: {
        origin: sessionOriginView(child, { title: "Refactor flaky manager tests" })!,
        onOpenSource,
      },
    });

    const divider = await screen.findByTestId("session-origin-divider");
    expect(screen.getAllByTestId("session-origin-divider")).toHaveLength(1);
    expect(divider).toHaveAccessibleName("Continued from Refactor flaky manager tests");
    const firstRow = document.querySelector('[data-message-id="child-first"]');
    expect(firstRow).toContainElement(divider);
    expect(screen.queryByText(/to get started/i)).not.toBeInTheDocument();

    await user.click(screen.getByTestId("session-origin-divider-link"));
    expect(onOpenSource).toHaveBeenCalledExactlyOnceWith(child.lineage?.parent_session_id);
  });

  it("Should show the divider alone above the empty state of an unprompted derived child", async () => {
    const child = continuedSessionFixture();
    renderThreadState({
      status: "success",
      origin: { origin: sessionOriginView(child, null)! },
    });

    const empty = await screen.findByTestId("session-thread-empty");
    expect(empty).toHaveTextContent("Nothing said here yet");
    expect(empty).toHaveTextContent(
      "The conversation carried over is sent with your first message."
    );
    const divider = screen.getByTestId("session-origin-divider");
    expect(divider).toHaveAttribute("data-link", "false");
    expect(divider).toHaveTextContent("Continued from Claude");
    expect(screen.queryByTestId("session-origin-divider-link")).not.toBeInTheDocument();
    expect(screen.queryByText(/to get started/i)).not.toBeInTheDocument();
  });

  it("Should keep the empty state of a derived child whose transcript holds only status events", async () => {
    const child = continuedSessionFixture();
    const transcript = [
      {
        id: "hook-start",
        role: "assistant",
        parts: [{ type: "data-compozy-event", data: { type: "hook.dispatch.start" } }],
      },
      {
        id: "hook-complete",
        role: "assistant",
        parts: [{ type: "data-compozy-event", data: { type: "hook.dispatch.complete" } }],
      },
    ] as SessionMessage[];
    renderThreadState({
      status: "success",
      messages: toReadonlyThreadMessages(transcript),
      origin: { origin: sessionOriginView(child, null)! },
    });

    const empty = await screen.findByTestId("session-thread-empty");
    expect(empty).toHaveTextContent("Nothing said here yet");
    expect(screen.getAllByTestId("session-origin-divider")).toHaveLength(1);
  });

  // Invariant: a dead live stream never poses as an empty session (US-018.AC-2).
  // Owning layer: thread state pane + transport notices. Canonical suite: this file.
  it("Should render the sync-failed pane instead of ThreadEmpty when the stream gave up", async () => {
    const user = userEvent.setup();
    const retry = vi.fn();
    renderThreadState({
      status: "success",
      transport: {
        ...SESSION_TRANSPORT_LIVE,
        failure: { at: 9_000, attempts: 6 },
        lastLiveAt: 1_000,
        phase: "failed",
        retry,
      },
    });

    const pane = await screen.findByTestId("thread-transcript-sync-failed");
    expect(pane).toHaveAttribute("role", "alert");
    expect(pane).toHaveTextContent("This conversation didn't sync");
    expect(pane).toHaveTextContent("after 6 tries");
    expect(screen.getAllByRole("button", { name: "Try again" })).toHaveLength(1);
    expect(screen.queryByText(/to get started/i)).not.toBeInTheDocument();
    await user.click(screen.getByTestId("thread-transcript-sync-failed-retry"));
    expect(retry).toHaveBeenCalledTimes(1);
  });

  it("Should keep a loaded conversation and add the stopped-updates marker when the stream gave up", async () => {
    renderThreadState({
      messages: toReadonlyThreadMessages([
        { id: "m-1", role: "assistant", parts: [{ type: "text", text: "Saved reply" }] },
      ] as SessionMessage[]),
      status: "success",
      transport: {
        ...SESSION_TRANSPORT_LIVE,
        failure: { at: 9_000, attempts: 6 },
        lastLiveAt: Date.parse("2026-09-06T14:02:00Z"),
        phase: "failed",
        retry: vi.fn(),
      },
    });

    expect(await screen.findByText("Saved reply")).toBeInTheDocument();
    const marker = screen.getByTestId("session-transport-failure");
    expect(marker).toHaveTextContent("couldn't reconnect after 6 tries");
    expect(screen.queryByTestId("thread-transcript-sync-failed")).not.toBeInTheDocument();
  });

  it("Should prioritize durable starting state and keep the composer disabled", async () => {
    renderThreadState({
      status: "pending",
      sessionState: "starting",
      isSessionRunning: isSessionRunning({
        ...primarySessionFixture,
        state: "starting",
        badge: "running",
      }),
    });

    const pane = await screen.findByTestId("thread-session-starting");
    expect(pane).toHaveAttribute("role", "status");
    expect(pane).toHaveTextContent("Your session is saved");
    expect(pane).not.toHaveTextContent(/runtime/i);
    expect(screen.queryByTestId("thread-transcript-skeleton")).not.toBeInTheDocument();
    // The Lexical composer is inerted (not focusable, not editable) instead of
    // carrying a `disabled` attribute, and renders its placeholder as visible text.
    expect(screen.getByTestId("composer-input")).toHaveAttribute("inert");
    expect(screen.getByTestId("composer-send-button")).toBeDisabled();
    expect(screen.getByTestId("composer-input")).toHaveTextContent("Session is starting…");
    expect(screen.queryByText("Thinking…")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Stop generation" })).not.toBeInTheDocument();
  });

  it("Should keep a session stopped before its first turn open for the next prompt", () => {
    renderThreadState({
      status: "success",
      sessionState: "stopped",
      failure: { kind: "cancellation", summary: "daemon stopped by user" },
    });

    expect(screen.queryByTestId("thread-session-startup-failure")).not.toBeInTheDocument();
    expect(screen.getByTestId("composer-input")).not.toHaveTextContent("Session failed to start");
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
    expect(within(pane).getByRole("button", { name: "Check agent settings" })).toBeInTheDocument();
    expect(screen.queryByTestId("thread-transcript-error")).not.toBeInTheDocument();
    expect(screen.getByTestId("composer-input")).toHaveAttribute("inert");
    expect(screen.getByTestId("composer-send-button")).toBeDisabled();
    expect(screen.getByTestId("composer-input")).toHaveTextContent("Session failed to start");
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
    expect(within(pane).queryByRole("button", { name: "Check agent settings" })).toBeNull();
    expect(screen.getByTestId("composer-input")).toHaveAttribute("inert");
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

  // Invariant: a turn the daemon's watchdog saw go quiet keeps its one working
  // row, which adds "quiet for {d}" from the supervision episode — no notice, no
  // second line; the composer Stop stays the action.
  it("Should keep the working row with a quiet segment while the daemon reports a quiet turn", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-04-17T18:42:00Z"));
    try {
      const transcript = [
        {
          id: "assistant-quiet-turn",
          role: "assistant",
          parts: [
            {
              type: "reasoning",
              text: "Waiting on the build.",
              state: "streaming",
              turn_id: "turn-live",
              timestamp: "2026-04-17T18:00:00Z",
            },
          ] as unknown as SessionMessage["parts"],
        } as SessionMessage,
      ];
      renderThreadState({
        status: "success",
        messages: toReadonlyThreadMessages(transcript),
        isSessionRunning: true,
        statusSession: {
          ...runningStatusSession("2026-04-17T18:00:00Z"),
          supervision: {
            quiet_warning: {
              quiet_since: "2026-04-17T18:30:00Z",
              warned_at: "2026-04-17T18:40:00Z",
              stop_at: "2026-04-17T18:50:00Z",
            },
            sources: [],
            work_signals: [],
          },
        },
      });

      const workingRow = await screen.findByTestId("session-working-row");
      expect(screen.getAllByTestId("session-working-row")).toHaveLength(1);
      expect(screen.getByTestId("session-working-timer")).toHaveTextContent("42m");
      expect(screen.getByTestId("session-working-quiet")).toHaveTextContent("quiet for 12m");
      expect(workingRow).toHaveTextContent(/Working for 42m.*quiet for 12m/);
      expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  // Invariant (S3 / US-027.EC-2, US-014): the frozen sentence after a daemon
  // stop names the daemon's cause from its own records — an inactivity stop
  // reads the supervision episode's actual span and never "by you"; a verified
  // escalated session stop says it was closed for the operator.
  it("Should read a daemon inactivity stop with its actual quiet span, and an escalated close, never as by you", async () => {
    const quietSince = "2026-07-07T11:20:00Z";
    const stoppedAt = "2026-07-07T12:01:40Z";
    const transcript = [
      {
        id: "assistant-inactivity",
        role: "assistant",
        parts: [
          {
            type: "text",
            text: "Waiting on the reviewer.",
            state: "done",
            timestamp: "2026-07-07T12:00:00Z",
          },
          {
            type: "data-compozy-event",
            timestamp: stoppedAt,
            data: {
              type: "session.supervision_stopped",
              timestamp: stoppedAt,
              raw: {
                cause: "inactivity",
                supervision: {
                  quiet_warning: {
                    quiet_since: quietSince,
                    warned_at: "2026-07-07T11:50:00Z",
                    stop_at: stoppedAt,
                  },
                  work_signals: [],
                  sources: [],
                },
              },
            },
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
    ];
    renderThreadState({
      status: "success",
      messages: toReadonlyThreadMessages(transcript),
      statusSession: {
        ...primarySessionFixture,
        state: "stopped",
        stop_cause: "inactivity",
        stop_reason: "timeout",
      },
    });

    const row = await screen.findByTestId("session-stopped-row");
    expect(row).toHaveAttribute("data-stopped-by", "daemon");
    expect(row).toHaveTextContent("Stopped after 1m 40s");
    expect(row).toHaveTextContent("no work for 41 minutes");
    expect(row).not.toHaveTextContent("by you");
  });

  // Invariant (US-027.EC-2, turn scope): the durable turn receipt with
  // `verified && escalated` reads "closed for you" while the session stays
  // active; the stopped turn itself stays open in the transcript.
  it("Should read a verified escalated turn stop as closed for you from the turn receipt", async () => {
    const transcript = [
      {
        id: "turn-forced",
        role: "assistant",
        parts: [
          {
            type: "text",
            text: "Running the suite.",
            state: "done",
            timestamp: "2026-07-07T12:00:00Z",
          },
          {
            type: "data-compozy-event",
            timestamp: "2026-07-07T12:00:14Z",
            data: {
              type: "session.turn_quiesced",
              turn_id: "turn-forced",
              timestamp: "2026-07-07T12:00:14Z",
              raw: {
                scope: "turn",
                turn_id: "turn-forced",
                verified: true,
                escalated: true,
                phase: "forced",
                elapsed_ms: 14_000,
                stop_cause: "user_requested",
              },
            },
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
    ];
    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });
    const row = await screen.findByTestId("session-stopped-row");
    expect(row).toHaveAttribute("data-stopped-by", "daemon");
    expect(row).toHaveTextContent("Stopped after 14s");
    expect(row).toHaveTextContent("the agent didn't answer the stop, so it was closed for you");
  });

  it("Should read a verified escalated session stop as closed for you", async () => {
    const transcript = [
      {
        id: "assistant-escalated",
        role: "assistant",
        parts: [
          {
            type: "text",
            text: "Still running the suite.",
            state: "done",
            timestamp: "2026-07-07T12:00:00Z",
          },
          {
            type: "data-compozy-event",
            timestamp: "2026-07-07T12:00:14Z",
            data: {
              type: "session_stopped",
              stop_reason: "user_canceled",
              timestamp: "2026-07-07T12:00:14Z",
            },
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
    ];
    renderThreadState({
      status: "success",
      messages: toReadonlyThreadMessages(transcript),
      statusSession: {
        ...primarySessionFixture,
        state: "stopped",
        stop_cause: "user_requested",
        escalated: true,
        verified: true,
      },
    });
    const row = await screen.findByTestId("session-stopped-row");
    expect(row).toHaveAttribute("data-stopped-by", "daemon");
    expect(row).toHaveTextContent("Stopped after 14s");
    expect(row).toHaveTextContent("the agent didn't answer the stop, so it was closed for you");
    expect(row).not.toHaveTextContent("by you");
  });

  // Invariant (VC-07, BUG-20260906-injected-guidance-missing-history): steer
  // provenance binds to the daemon's explicit message_id — a loaded message gets
  // its meta line, several same-turn steers keep their own, guidance never
  // dispatched renders exactly one receipt bubble from the marker's authored
  // text (subdued when superseded), and a legacy marker stays a neutral row.
  // Owning layer: thread rendering over the real provenance provider.
  it("Should bind steer provenance to message identities: meta lines, one receipt per undispatched identity, legacy rows", async () => {
    const steered = (id: string, evidence: Record<string, unknown>, timestamp: string) =>
      ({
        id,
        role: "assistant",
        parts: [
          {
            type: "data-compozy-event",
            timestamp,
            data: {
              type: "transcript_marker.created",
              marker: {
                kind: "transcript_marker.prompt_steered",
                summary: "Steer.",
                occurred_at: timestamp,
                evidence,
              },
            },
          },
        ] as unknown as SessionMessage["parts"],
      }) as SessionMessage;
    const transcript: SessionMessage[] = [
      {
        id: "msg_a",
        role: "user",
        parts: [{ type: "text", text: "Only touch the lifecycle tests" }],
      } as SessionMessage,
      {
        id: "msg_b",
        role: "user",
        parts: [{ type: "text", text: "Skip the store package entirely" }],
      } as SessionMessage,
      // Two same-turn steers, recorded in one assistant turn, each naming its own message.
      {
        id: "assistant-two-steers",
        role: "assistant",
        parts: [
          ...steered(
            "x",
            {
              steer_delivery: "interrupt_fallback",
              message_id: "msg_b",
              authored_text: "Skip the store package entirely",
            },
            "2026-07-07T12:00:02Z"
          ).parts!,
          ...steered(
            "y",
            {
              steer_delivery: "injected",
              message_id: "msg_a",
              authored_text: "Only touch the lifecycle tests",
            },
            "2026-07-07T12:00:01Z"
          ).parts!,
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
      // Pending guidance never dispatched, then superseded: one subdued receipt with the latest truth.
      steered(
        "pending",
        {
          steer_delivery: "pending_injection",
          message_id: "msg_c",
          authored_text: "Use the lifecycle channel",
        },
        "2026-07-07T12:00:11Z"
      ),
      {
        id: "superseded",
        role: "assistant",
        parts: [
          {
            type: "data-compozy-event",
            timestamp: "2026-07-07T12:00:21Z",
            data: {
              type: "transcript_marker.created",
              marker: {
                kind: "transcript_marker.prompt_superseded",
                summary: "Superseded.",
                occurred_at: "2026-07-07T12:00:21Z",
                evidence: {
                  message_id: "msg_c",
                  authored_text: "Use the lifecycle channel",
                  replacement_entry_id: "inq_9",
                },
              },
            },
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
      // A legacy marker without identity: a neutral row, attached to nothing.
      steered(
        "legacy",
        { steer_delivery: "injected", queue_entry_id: "inq_1" },
        "2026-07-07T12:00:31Z"
      ),
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

    const metas = await screen.findAllByTestId("user-message-steer-meta");
    const metaByText = (text: string) =>
      screen.getByText(text).closest<HTMLElement>('[data-testid="thread-message-row"]')!;
    expect(
      within(metaByText("Only touch the lifecycle tests")).getByTestId("user-message-steer-meta")
    ).toHaveAttribute("data-steer", "injected");
    expect(
      within(metaByText("Skip the store package entirely")).getByTestId("user-message-steer-meta")
    ).toHaveAttribute("data-steer", "interrupt_fallback");
    // Exactly one receipt for msg_c, subdued, with the latest (superseded) truth and the authored text.
    const receipts = screen.getAllByTestId("user-message-receipt");
    expect(receipts).toHaveLength(1);
    expect(receipts[0]).toHaveAttribute("data-steer", "superseded");
    expect(receipts[0]).toHaveAttribute("data-message-id-receipt", "msg_c");
    expect(within(receipts[0]!).getByTestId("user-message-bubble")).toHaveAttribute(
      "data-subdued",
      "true"
    );
    expect(receipts[0]).toHaveTextContent("Use the lifecycle channel");
    expect(receipts[0]).toHaveTextContent("Superseded by your next steer");
    // The bound identities render no marker rows; the legacy marker keeps its neutral line.
    const legacyRows = screen.getAllByTestId("steer-marker-notice");
    expect(legacyRows).toHaveLength(1);
    expect(legacyRows[0]).toHaveAttribute("data-steer", "injected");
    // Meta lines exist only under the two real messages plus the receipt.
    expect(metas).toHaveLength(3);
  });

  // Invariant (issue #923): a turn the daemon splits around steer markers
  // delivered mid-call reads as one turn — no empty shell for a marker whose row
  // draws nothing, and the turn's pieces keep the inline rhythm with the turn
  // gap only after its last piece. Owning layer: thread rendering over the real
  // provenance provider, fed the sanitized projection of the reported turn.
  it.each([
    [
      "distinct wake identities",
      ["wake_01M5MV5SVJGJVBC5P1278DP0KA", "wake_01M5MV6A9XKQ2RTB7W3HZC4D8E"],
      2,
    ],
    ["identities persisted as [REDACTED]", ["[REDACTED]", "[REDACTED]"], 1],
  ] as const)(
    "Should render a turn steered mid-call by subagent wakes without empty gaps (%s)",
    async (_label, wakeIds, steerRows) => {
      const normalized = await normalizeTranscriptMessages(
        steeredWakesTranscript(wakeIds) as unknown as SessionMessage[]
      );
      renderThreadState({ status: "success", messages: toReadonlyThreadMessages(normalized) });

      await waitFor(() =>
        expect(screen.getAllByTestId("steer-marker-notice")).toHaveLength(steerRows)
      );
      const pieces = screen.getAllByTestId("assistant-message");
      // Every mounted piece draws something: the chunk, the visible steer rows, the continuation.
      expect(pieces).toHaveLength(steerRows + 2);
      for (const piece of pieces) expect(piece).not.toBeEmptyDOMElement();
      for (const piece of pieces.slice(0, -1)) {
        expect(piece).toHaveClass("pb-transcript-inline-gap");
      }
      for (const piece of pieces.slice(1)) expect(piece).not.toHaveClass("pt-1");
      expect(pieces.at(-1)).toHaveClass("pb-transcript-turn-gap");
      // The trailing whitespace-only thought is not a second thought.
      expect(pieces.at(-1)).toHaveTextContent("5 tools · 1 thought · 1 running");
    }
  );

  it("Should join a text-only continuation that names its turn only in message metadata", async () => {
    // The daemon names the turn of a `-N` continuation in its metadata when no
    // event part carries it (a reply that resumes right after the steer).
    const transcript = [
      ...steeredWakesTranscript([
        "wake_01M5MV5SVJGJVBC5P1278DP0KA",
        "wake_01M5MV6A9XKQ2RTB7W3HZC4D8E",
      ]).slice(0, 4),
      {
        id: `${STEER_WAKE_TURN}-2`,
        role: "assistant",
        metadata: { turn_id: STEER_WAKE_TURN },
        parts: [{ type: "text", text: "Resumed the pending work.", state: "done" }],
      },
    ] as unknown as SessionMessage[];
    const normalized = await normalizeTranscriptMessages(transcript);
    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(normalized) });

    await screen.findByText("Resumed the pending work.");
    const pieces = screen.getAllByTestId("assistant-message");
    expect(pieces).toHaveLength(4);
    expect(pieces.at(-2)).toHaveClass("pb-transcript-inline-gap");
    expect(pieces.at(-1)).not.toHaveClass("pt-1");
    expect(pieces.at(-1)).toHaveClass("pb-transcript-turn-gap");
  });

  it("Should hide the pending receipt once the real message is loaded and carry the newer marker's truth", async () => {
    const pendingMarker = {
      id: "pending",
      role: "assistant",
      parts: [
        {
          type: "data-compozy-event",
          timestamp: "2026-07-07T12:00:11Z",
          data: {
            type: "transcript_marker.created",
            marker: {
              kind: "transcript_marker.prompt_steered",
              summary: "Waiting.",
              occurred_at: "2026-07-07T12:00:11Z",
              evidence: {
                steer_delivery: "pending_injection",
                message_id: "msg_p",
                authored_text: "Use the channel",
              },
            },
          },
        },
      ] as unknown as SessionMessage["parts"],
    } as SessionMessage;
    const confirmed: SessionMessage[] = [
      pendingMarker,
      {
        id: "msg_p",
        role: "user",
        parts: [{ type: "text", text: "Use the channel" }],
      } as SessionMessage,
      {
        id: "injected",
        role: "assistant",
        parts: [
          {
            type: "data-compozy-event",
            timestamp: "2026-07-07T12:00:12Z",
            data: {
              type: "transcript_marker.created",
              marker: {
                kind: "transcript_marker.prompt_steered",
                summary: "Delivered.",
                occurred_at: "2026-07-07T12:00:12Z",
                evidence: {
                  steer_delivery: "injected",
                  message_id: "msg_p",
                  authored_text: "Use the channel",
                },
              },
            },
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(confirmed) });

    await screen.findByText("Use the channel");
    expect(screen.queryByTestId("user-message-receipt")).not.toBeInTheDocument();
    expect(screen.queryByTestId("steer-marker-notice")).not.toBeInTheDocument();
    const meta = screen.getByTestId("user-message-steer-meta");
    expect(meta).toHaveAttribute("data-steer", "injected");
    expect(screen.getAllByTestId("user-message-bubble")).toHaveLength(1);
  });

  // Invariant (BUG-20260906-promoted-turn-missing-live, renderer side): a promoted
  // queued input's turn as the public transcript projects it — the replaced turn's
  // fallback/cancel markers and receipt, the authored user message, the accepted
  // marker, then the assistant answer as a lone text part with no metadata and no
  // turn id of its own — renders the answer as its own row, both cold and when the
  // same message id gains its text on settle. Owning layer: transcript repository +
  // thread rows + assistant message; canonical suite: this file. Fixture: the tail
  // of the lab's `navigation-promote-live-final-transcript.json` (sequences 4588–4595).
  it("Should render a promoted turn's settled answer cold and when its message settles in place", async () => {
    const marker = (
      id: string,
      turnId: string,
      kind: string,
      timestamp: string,
      evidence: Record<string, unknown>
    ) =>
      ({
        id,
        role: "assistant",
        parts: [
          {
            type: "data-compozy-event",
            data: {
              type: "transcript_marker.created",
              session_id: "history-operator-session-1",
              turn_id: turnId,
              timestamp,
              title: kind,
              raw: { kind, occurred_at: timestamp, summary: kind, evidence },
            },
          },
        ] as unknown as SessionMessage["parts"],
      }) as SessionMessage;
    const replaced = "turn-2a47a312263fa523";
    const promoted = "turn-a8c57a3749295c38";
    const answerText = "Queued recovery processed once.";
    const transcript: SessionMessage[] = [
      marker(
        "ev-2d10240621c21737",
        replaced,
        "transcript_marker.prompt_steered",
        "2026-09-06T12:30:42.184457Z",
        {
          steer_delivery: "interrupt_fallback",
          mode: "steer",
        }
      ),
      marker(
        "ev-ac4f79d957d200f8",
        replaced,
        "transcript_marker.prompt_cancel",
        "2026-09-06T12:30:42.192997Z",
        {
          source: "cancel_prompt",
        }
      ),
      {
        id: `${replaced}-2`,
        role: "assistant",
        parts: [
          {
            type: "data-compozy-event",
            data: {
              type: "session.turn_quiesced",
              turn_id: replaced,
              timestamp: "2026-09-06T12:30:42.288312Z",
              raw: {
                scope: "turn",
                turn_id: replaced,
                verified: true,
                escalated: false,
                phase: "cooperative",
                elapsed_ms: 91,
                stop_cause: "user_requested",
              },
            },
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
      {
        id: "2cd6cf11-0f30-4278-9e90-a7215a0d73ca",
        role: "user",
        metadata: {
          turn_id: promoted,
          timestamp: "2026-09-06T12:30:42.312938Z",
          message_id: "2cd6cf11-0f30-4278-9e90-a7215a0d73ca",
        },
        parts: [
          {
            type: "text",
            text: "Queued recovery: verify live promotion after cache reconciliation.",
            state: "done",
          },
        ],
      } as SessionMessage,
      marker(
        "ev-17ad14dfea105a15",
        promoted,
        "transcript_marker.prompt_accepted",
        "2026-09-06T12:30:42.328137Z",
        {
          message_id: "2cd6cf11-0f30-4278-9e90-a7215a0d73ca",
          queue_entry_id: "inq-promoted",
          mode: "queue",
        }
      ),
    ];
    const answer = {
      id: promoted,
      role: "assistant",
      parts: [
        { type: "text", id: `${promoted}-text-1`, text: answerText, state: "done" },
      ] as unknown as SessionMessage["parts"],
    } as SessionMessage;
    const streaming = {
      ...answer,
      status: { type: "running" },
      parts: [
        { type: "text", id: `${promoted}-text-1`, text: "", state: "streaming" },
      ] as unknown as SessionMessage["parts"],
    } as SessionMessage;
    const answerRow = () => document.querySelector(`[data-message-id="${promoted}"]`);

    const cold = renderThreadState({
      status: "success",
      messages: toReadonlyThreadMessages([...transcript, answer]),
    });
    await screen.findByText("Queued recovery: verify live promotion after cache reconciliation.");
    expect(answerRow()).toHaveTextContent(answerText);
    cold.unmount();

    const live = renderThreadState({
      status: "success",
      messages: toReadonlyThreadMessages([...transcript, streaming]),
    });
    await screen.findByText("Queued recovery: verify live promotion after cache reconciliation.");
    // ADR-006: no shell before content — the streaming answer draws nothing yet.
    expect(answerRow()?.textContent ?? "").not.toContain(answerText);
    live.rerenderWith({ messages: toReadonlyThreadMessages([...transcript, answer]) });
    await waitFor(() => {
      expect(answerRow()).toHaveTextContent(answerText);
    });
  });

  // Invariant (public API wire shape, task_10 re-walk): the daemon's persisted
  // marker events carry the marker under `raw`; the repository normalizes that
  // boundary once, so a reloaded injected steer binds to its bubble's meta line
  // and a queued prompt shows only the position the daemon recorded.
  // Owning layer: transcript repository + thread host. Canonical suite: this file.
  it("Should bind a reloaded raw-marker steer and a queued prompt through the repository boundary", async () => {
    const wireMarker = (
      id: string,
      kind: string,
      evidence: Record<string, unknown>,
      timestamp: string
    ) =>
      ({
        id,
        role: "assistant",
        parts: [
          {
            type: "data-compozy-event",
            data: {
              type: "transcript_marker.created",
              session_id: "sess-1",
              turn_id: "turn-f3d48e7247b52982",
              timestamp,
              text: "Steering delivered into the live turn.",
              title: kind,
              raw: {
                kind,
                occurred_at: timestamp,
                summary: "Steering delivered into the live turn.",
                evidence,
              },
            },
          },
        ] as unknown as SessionMessage["parts"],
      }) as SessionMessage;
    const transcript: SessionMessage[] = [
      wireMarker(
        "ev-queued",
        "transcript_marker.prompt_queued",
        { queue_entry_id: "inq-d4b7", queue_position: 2, mode: "queue" },
        "2026-09-06T10:25:09.578Z"
      ),
      {
        id: "msg_handbook_guidance_02",
        role: "user",
        metadata: { turn_id: "turn-f3d48e7247b52982", message_id: "msg_handbook_guidance_02" },
        parts: [
          { type: "text", text: "Use the heading Recovery checklist in every handbook chapter." },
        ],
      } as SessionMessage,
      wireMarker(
        "ev-4acc274ce3953dcc",
        "transcript_marker.prompt_steered",
        {
          authored_text: "Use the heading Recovery checklist in every handbook chapter.",
          input_event_id: "ev-76843bf85ba7f174",
          message_id: "msg_handbook_guidance_02",
          mode: "steer",
          queue_entry_id: "inq-bc06",
          queue_status: "sent",
          steer_delivery: "injected",
          target_turn_id: "turn-f3d48e7247b52982",
        },
        "2026-09-06T10:45:41.210Z"
      ),
      {
        id: "msg_release_index_01",
        role: "user",
        metadata: { turn_id: "turn-6bf0", message_id: "msg_release_index_01" },
        parts: [{ type: "text", text: "Now write the index." }],
      } as SessionMessage,
      wireMarker(
        "ev-accepted",
        "transcript_marker.prompt_accepted",
        {
          queue_entry_id: "inq-d4b7",
          message_id: "msg_release_index_01",
          authored_text: "Now write the index.",
          mode: "queue",
        },
        "2026-09-06T10:26:02.105Z"
      ),
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

    const guidance = (
      await screen.findByText("Use the heading Recovery checklist in every handbook chapter.")
    ).closest<HTMLElement>('[data-testid="thread-message-row"]')!;
    const meta = within(guidance).getByTestId("user-message-steer-meta");
    expect(meta).toHaveAttribute("data-steer", "injected");
    expect(meta).toHaveTextContent("Steered — delivered into the live turn");
    const queued = screen
      .getByText("Now write the index.")
      .closest<HTMLElement>('[data-testid="thread-message-row"]')!;
    const queuedMeta = within(queued).getByTestId("user-message-steer-meta");
    expect(queuedMeta).toHaveAttribute("data-steer", "queued");
    expect(queuedMeta).toHaveTextContent("From the queue");
    expect(queuedMeta).toHaveTextContent("was #2");
    // Bound markers render no rows of their own; the original queued marker is not a steer row.
    expect(screen.queryByTestId("steer-marker-notice")).not.toBeInTheDocument();
    expect(screen.queryByTestId("user-message-receipt")).not.toBeInTheDocument();
  });

  // Invariant (task_07 US-022 on the public wire): a settled turn whose tool
  // parts carry no time or turn id of their own — only its `tool_call`/`usage`
  // events do — still folds as one turn behind "Worked for · counts", status
  // events never become rows, and the final reply stays visible.
  it("Should fold a real settled turn whose tools and text carry no time of their own", async () => {
    const turnId = "turn-f3d48e7247b52982";
    const statusEvent = (type: string, timestamp: string, extra: Record<string, unknown> = {}) => ({
      type: "data-compozy-event",
      data: { type, session_id: "sess-1", turn_id: turnId, timestamp, ...extra },
    });
    const writeParts = (index: number, at: string) => [
      statusEvent("tool_call", at, { title: "Preparing file…", tool_call_id: `toolu_${index}` }),
      statusEvent("usage", at, { usage: { turn_id: turnId, context_used: 1000 + index } }),
      {
        type: "tool-Preparing file…",
        state: "output-available",
        toolCallId: `toolu_${index}`,
        title: "Preparing file…",
        input: { content: `# Chapter ${index}`, file_path: `./handbook/chapter-${index}.md` },
        output: { type: "tool_result", title: "Write", raw: { content: "ok" } },
      },
    ];
    const transcript: SessionMessage[] = [
      {
        id: `${turnId}-3`,
        role: "assistant",
        parts: [
          ...writeParts(1, "2026-09-06T10:46:17.140Z"),
          ...writeParts(2, "2026-09-06T10:46:22.567Z"),
          ...writeParts(3, "2026-09-06T10:46:26.931Z"),
          statusEvent("usage", "2026-09-06T10:50:29.140Z", {
            usage: { turn_id: turnId, context_used: 9 },
          }),
          {
            type: "text",
            text: "All chapters carry the Recovery checklist heading.",
            state: "done",
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

    const fold = await screen.findByTestId("turn-fold-row");
    // Duration spans the turn's own recorded event times (first tool_call tick to
    // the closing usage event). The daemon's stale "Preparing file…" title is
    // never read as prose; the calls' input (a file path plus content) is what
    // names them as the three file writes they were.
    expect(fold).toHaveTextContent(/^Worked for 4m 12s · Edited 3 files/);
    // Opened, the fold lists the calls as rows — no second disclosure (VC-05).
    await userEvent.click(fold);
    await waitFor(() => {
      expect(screen.getAllByTestId("tool-call-row")).toHaveLength(3);
    });
    expect(screen.queryByTestId("work-summary-row")).not.toBeInTheDocument();
    await userEvent.click(fold);
    expect(
      screen.getByText("All chapters carry the Recovery checklist heading.")
    ).toBeInTheDocument();
    expect(screen.queryByTestId("runtime-activity-notice")).not.toBeInTheDocument();
    expect(screen.queryByTestId("live-tool")).not.toBeInTheDocument();
    expect(screen.queryAllByTestId("tool-call-row")).toHaveLength(0);
  });

  // Invariant (task_07 US-022 / ADR-009 on the public wire): a turn the operator
  // canceled records its end in later projected messages; the call left without
  // a result reads "stopped", never "Running", the turn stays open under
  // "You stopped after", and the status row reads the stop with the whole turn's span.
  it("Should settle an interrupted turn's pending call from the daemon's later receipt", async () => {
    const turnId = "turn-4f009525b74d74a6";
    const at = (offset: number) =>
      new Date(Date.parse("2026-09-06T10:25:12.931Z") + offset).toISOString();
    const transcript: SessionMessage[] = [
      {
        id: `${turnId}-2`,
        role: "assistant",
        parts: [
          {
            type: "data-compozy-event",
            data: {
              type: "tool_call",
              turn_id: turnId,
              timestamp: at(0),
              title: "Preparing file…",
              tool_call_id: "toolu_done",
            },
          },
          {
            type: "tool-Preparing file…",
            state: "output-available",
            toolCallId: "toolu_done",
            title: "Preparing file…",
            input: { content: "# Sessions", file_path: "drafts/sessions.md" },
            output: { type: "tool_result", title: "Write", raw: { content: "ok" } },
          },
          {
            type: "data-compozy-event",
            data: {
              type: "tool_call",
              turn_id: turnId,
              timestamp: at(30_000),
              title: "Preparing file…",
              tool_call_id: "toolu_cut",
            },
          },
          {
            type: "tool-Preparing file…",
            state: "input-streaming",
            toolCallId: "toolu_cut",
            title: "Preparing file…",
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
      {
        id: "ev-cancel",
        role: "assistant",
        parts: [
          {
            type: "data-compozy-event",
            data: {
              type: "transcript_marker.created",
              turn_id: turnId,
              timestamp: at(46_000),
              title: "transcript_marker.prompt_cancel",
              raw: {
                kind: "transcript_marker.prompt_cancel",
                occurred_at: at(46_000),
                summary: "Prompt canceled by operator.",
                evidence: { source: "cancel_prompt" },
              },
            },
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
      {
        id: `${turnId}-4`,
        role: "assistant",
        parts: [
          {
            type: "data-compozy-event",
            data: {
              type: "session.turn_quiesced",
              turn_id: turnId,
              timestamp: at(46_126),
              raw: {
                scope: "turn",
                turn_id: turnId,
                verified: true,
                escalated: false,
                phase: "cooperative",
                elapsed_ms: 98,
                stop_cause: "user_requested",
              },
            },
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

    // The span reaches the receipt the daemon projected into a later message.
    expect(
      within(await screen.findByTestId("turn-fold-interrupted")).getByTestId("turn-fold-open-label")
    ).toHaveTextContent(/^You stopped after 46s/);
    expect(screen.queryByTestId("live-tool")).not.toBeInTheDocument();
    const rows = screen.getAllByTestId("tool-call-row");
    const cut = rows.find(row => row.querySelector('[data-status="stopped"]'));
    expect(cut).toBeDefined();
    expect(within(cut!).getByTestId("tool-call-state-word")).toHaveTextContent("stopped");
    expect(screen.queryByText(/Running Preparing/)).not.toBeInTheDocument();
    // The fold is the one owner of "You stopped after 46s": no second status line.
    expect(screen.queryByTestId("session-stopped-row")).not.toBeInTheDocument();
  });

  // Invariant (US-009.EC-2): a stop the daemon answered nothing-in-flight reads
  // completed, faintly, and never canceled.
  it("Should read the completion note when the stop arrived after the turn finished", async () => {
    renderThreadState({ status: "success", stopCompletionNote: true });
    const row = await screen.findByTestId("session-stop-completion-row");
    expect(row).toHaveTextContent(/^Completed/);
    expect(row).toHaveTextContent("the stop arrived after the turn finished");
    expect(screen.queryByTestId("session-stopped-row")).not.toBeInTheDocument();
  });

  // ADR-006 rule 3: before any content the status row reads "Thinking…" and no
  // assistant shell exists; the empty-state copy never shows for a live turn.
  it("Should read Thinking with no assistant shell when the successful transcript is empty and active", async () => {
    renderThreadState({ status: "success", isSessionRunning: true });

    expect(await screen.findByRole("status", { name: "Agent is responding" })).toHaveTextContent(
      "Thinking…"
    );
    expect(screen.queryByTestId("assistant-message")).not.toBeInTheDocument();
    expect(screen.queryByTestId("assistant-message-actions")).not.toBeInTheDocument();
    expect(screen.queryByText(/Start a conversation/i)).not.toBeInTheDocument();
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

  // UT-068 (S3, US-007.EC-2/EC-5): "Fork from here" is rewind's twin — present
  // only on a durable user message under a Fork host, mounted before "Rewind to
  // here", and it hands that message (id + text) to the host.
  it("Should offer Fork from here beside Rewind only for a durable user message", async () => {
    const forkRequest = vi.fn<SessionForkRequest>();
    const messages = toReadonlyThreadMessages(sessionTranscriptFixture.slice(0, 2));

    renderThreadState({
      status: "success",
      messages,
      durableMessageIds: ["transcript_user_001"],
      forkRequest,
    });

    const fork = await screen.findByRole("button", { name: "Fork from here" });
    await waitFor(() => expect(fork).toBeEnabled());
    expect(screen.getAllByTestId("user-message-fork")).toHaveLength(1);
    const rewind = screen.getByTestId("user-message-rewind");
    expect(fork.compareDocumentPosition(rewind) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();

    fireEvent.click(fork);
    expect(forkRequest).toHaveBeenCalledExactlyOnceWith(undefined, {
      messageId: "transcript_user_001",
    });
  });

  it("Should not offer Fork from here for an optimistic tail, without a host, or read-only", async () => {
    const messages = toReadonlyThreadMessages(sessionTranscriptFixture.slice(0, 2));
    const view = renderThreadState({ status: "success", messages, forkRequest: vi.fn() });

    expect(await screen.findByTestId("thread-messages")).toBeInTheDocument();
    expect(screen.queryByTestId("user-message-fork")).not.toBeInTheDocument();
    view.rerenderWith({
      durableMessageIds: ["transcript_user_001"],
      forkRequest: null,
    });
    expect(await screen.findByTestId("user-message-rewind")).toBeInTheDocument();
    const rewind = screen.getByRole("button", { name: "Edit message" });
    await waitFor(() => expect(rewind).toBeEnabled());
    expect(screen.getAllByTestId("user-message-rewind")).toHaveLength(1);
    expect(screen.queryByTestId("user-message-fork")).not.toBeInTheDocument();

    view.rerenderWith({
      durableMessageIds: ["transcript_user_001"],
      forkRequest: vi.fn(),
      readOnly: true,
    });
    expect(await screen.findByTestId("thread-messages")).toBeInTheDocument();
    expect(screen.queryByTestId("user-message-fork")).not.toBeInTheDocument();

    view.rerenderWith({
      durableMessageIds: ["transcript_user_001"],
      forkRequest: null,
      readOnly: true,
    });
    expect(await screen.findByTestId("thread-messages")).toBeInTheDocument();
    expect(screen.queryByTestId("user-message-rewind")).not.toBeInTheDocument();
    expect(screen.queryByTestId("composer-input")).not.toBeInTheDocument();
  });

  // A pending rewind anywhere in the workspace moves the transcript under every
  // row: Fork remains blocked, while selecting a local edit makes no mutation.
  it("Should disable Fork but allow selecting a local edit while a rewind is pending", async () => {
    const forkRequest = vi.fn<SessionForkRequest>();
    const messages = toReadonlyThreadMessages(sessionTranscriptFixture.slice(0, 2));
    const queryClient = createQueryClient();
    void queryClient
      .getMutationCache()
      .build(queryClient, {
        mutationKey: sessionKeys.rewindConversation(fixtureWorkspaceId()),
        mutationFn: () => new Promise<never>(() => undefined),
      })
      .execute(undefined);

    renderThreadState(
      {
        status: "success",
        messages,
        durableMessageIds: ["transcript_user_001"],
        forkRequest,
      },
      queryClient
    );

    const fork = await screen.findByTestId("user-message-fork");
    expect(fork).toBeDisabled();
    expect(screen.getByTestId("user-message-rewind")).toBeEnabled();
    fireEvent.click(fork);
    expect(forkRequest).not.toHaveBeenCalled();
  });

  // A turn started outside this page (CLI, another window, before a reload) leaves the
  // local thread idle; Fork stays blocked, but selecting a local edit remains safe.
  it("Should disable Fork but allow selecting a local edit while the daemon reports a running turn", async () => {
    const baseFetch = createFetchMock();
    const detailPath = `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/${primarySessionFixture.id}`;
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) =>
        getPathname(input) === detailPath
          ? jsonResponse({ session: runningStatusSession() })
          : baseFetch(input, init)
      )
    );
    const messages = toReadonlyThreadMessages(sessionTranscriptFixture.slice(0, 2));

    renderThreadState({
      status: "success",
      messages,
      durableMessageIds: ["transcript_user_001"],
      forkRequest: vi.fn<SessionForkRequest>(),
    });

    await waitFor(() => expect(screen.getByTestId("user-message-fork")).toBeDisabled());
    expect(screen.getByTestId("user-message-rewind")).toBeEnabled();
  });

  // Invariant: user prose links permit only safe external protocols and leave
  // code literal. Owner: user-message rendering; canonical suite: SessionThread.
  it("renders safe user links while preserving code and unsafe link labels", async () => {
    const text =
      "https://example.com/path?q=1#part [Email](mailto:hello@example.com) [unsafe](javascript:alert) `https://code.example`";
    renderThreadState({
      status: "success",
      messages: toReadonlyThreadMessages([
        { id: "user-links", role: "user", parts: [{ type: "text", text }] } as SessionMessage,
      ]),
    });
    const bubble = await screen.findByTestId("user-message-bubble");
    const links = within(bubble).getAllByRole("link");
    expect(links).toHaveLength(2);
    expect(links[0]).toHaveAttribute("href", "https://example.com/path?q=1#part");
    expect(links[1]).toHaveAttribute("href", "mailto:hello@example.com");
    for (const link of links) {
      expect(link).toHaveAttribute("rel", "noopener noreferrer");
      expect(link).toHaveAttribute("target", "_blank");
    }
    expect(within(bubble).getByText("https://code.example").tagName).toBe("CODE");
    expect(bubble).toHaveTextContent("unsafe");
  });

  // Invariant (#967): what the operator typed is what the user message shows — each
  // line break, the blank line, a pasted list, and a link — in every variant of the
  // user message. Owner: user-message rendering; canonical suite: SessionThread.
  describe("authored formatting across user-message variants", () => {
    const AUTHORED = "/review first line\nsecond line see https://example.com/a\n\n- alpha\n- beta";
    const steerMarker = (messageId: string) =>
      ({
        id: "ev-steered",
        role: "assistant",
        parts: [
          {
            type: "data-compozy-event",
            data: {
              type: "transcript_marker.created",
              session_id: "sess-1",
              turn_id: "turn-authored",
              timestamp: "2026-10-24T10:00:00.000Z",
              text: "Steering delivered into the live turn.",
              title: "transcript_marker.prompt_steered",
              raw: {
                kind: "transcript_marker.prompt_steered",
                occurred_at: "2026-10-24T10:00:00.000Z",
                summary: "Steering delivered into the live turn.",
                evidence: {
                  authored_text: AUTHORED,
                  message_id: messageId,
                  mode: "steer",
                  queue_status: "sent",
                  steer_delivery: "injected",
                  target_turn_id: "turn-authored",
                },
              },
            },
          },
        ] as unknown as SessionMessage["parts"],
      }) as SessionMessage;
    const variants: {
      name: string;
      metadata: Record<string, unknown>;
      parts?: SessionMessage["parts"];
      after?: (id: string) => SessionMessage[];
      expectChip?: string;
    }[] = [
      { name: "plain text", metadata: {} },
      {
        name: "skill command",
        metadata: {
          skill_invocations: [
            { command_id: "skill:workspace:review", token: "/review", start: 0, end: 7 },
          ],
        },
        expectChip: "session-skill-directive",
      },
      {
        name: "@-mention",
        metadata: {
          mentions: [
            {
              kind: "file",
              token: "@README.md",
              target: "README.md",
              label: "README.md",
              start: AUTHORED.length + 1,
              end: AUTHORED.length + 11,
            },
          ],
        },
        expectChip: "session-mention-chip",
      },
      {
        name: "attachment",
        metadata: {
          attachments: [
            {
              id: "att-notes",
              name: "notes.pdf",
              mime_type: "application/pdf",
              bytes: 4096,
              sha256: "b".repeat(64),
              kind: "file",
            },
          ],
        },
        parts: [
          {
            type: "file",
            mediaType: "application/pdf",
            url: "compozy://session-attachments/att-notes",
            filename: "notes.pdf",
          },
        ] as SessionMessage["parts"],
      },
      { name: "steered entry", metadata: {}, after: id => [steerMarker(id)] },
    ];

    it.each(variants)("keeps authored formatting for $name", async variant => {
      const id = `user-authored-${variant.name.replace(/\W+/g, "-")}`;
      const text = variant.name === "@-mention" ? `${AUTHORED} @README.md` : AUTHORED;
      const transcript = [
        {
          id,
          role: "user",
          metadata: { turn_id: "turn-authored", message_id: id, ...variant.metadata },
          parts: [...(variant.parts ?? []), { type: "text", text, state: "done" }],
        } as SessionMessage,
        ...(variant.after?.(id) ?? []),
      ];

      renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

      const bubble = await screen.findByTestId("user-message-bubble");
      const paragraph = bubble.querySelector("p")!;
      expect(paragraph.querySelectorAll("br")).toHaveLength(1);
      expect(paragraph).toHaveTextContent("/review first line");
      const items = within(bubble).getAllByRole("listitem");
      expect(items.map(item => item.firstChild?.textContent?.trim())).toEqual(["alpha", "beta"]);
      expect(within(paragraph).getByRole("link")).toHaveAttribute("href", "https://example.com/a");
      if (variant.expectChip) expect(within(bubble).getByTestId(variant.expectChip)).toBeVisible();
      if (variant.after) {
        expect(screen.getByTestId("user-message-steer-meta")).toHaveAttribute(
          "data-steer",
          "injected"
        );
      }
    });

    it("keeps authored formatting after a persisted transcript is normalized (reload)", async () => {
      const [message] = await normalizeTranscriptMessages([
        {
          id: "user-authored-reload",
          role: "user",
          metadata: { turn_id: "turn-authored", message_id: "user-authored-reload" },
          parts: [{ type: "text", text: AUTHORED }],
        },
      ]);

      renderThreadState({ status: "success", messages: toReadonlyThreadMessages([message!]) });

      const bubble = await screen.findByTestId("user-message-bubble");
      expect(bubble.querySelector("p")!.querySelectorAll("br")).toHaveLength(1);
      expect(within(bubble).getAllByRole("listitem")).toHaveLength(2);
    });
  });

  it("Should render verified skill invocation tokens from persisted user-message metadata", async () => {
    const transcript = [
      {
        id: "user-skill-invocation",
        role: "user",
        content: "Review /frontend-qa before the release.",
        metadata: {
          skill_invocations: [
            {
              command_id: "skill:workspace:frontend-qa:sha256:abc123",
              token: "/frontend-qa",
              label: "Frontend QA",
              source: "workspace",
              start: 7,
              end: 19,
            },
          ],
        },
        parts: [
          {
            type: "text",
            text: "Review /frontend-qa before the release.",
            state: "done",
          },
        ],
      } as SessionMessage,
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

    const directive = await screen.findByTestId("session-skill-directive");
    expect(directive).toHaveAttribute("data-raw-token", "/frontend-qa");
    expect(directive).toHaveAttribute(
      "data-command-id",
      "skill:workspace:frontend-qa:sha256:abc123"
    );
    expect(directive).toHaveTextContent("/frontend-qa");
    expect(screen.getByTestId("user-message-bubble")).toHaveTextContent(
      "Review /frontend-qa before the release."
    );
  });

  it("Should render only the server-admitted skill occurrence", async () => {
    const transcript = [
      {
        id: "user-repeated-skill-invocation",
        role: "user",
        content: "/review then /review",
        metadata: {
          skill_invocations: [
            {
              command_id: "skill:workspace:review:sha256:def456",
              token: "/review",
              label: "Review",
              source: "workspace",
              start: 13,
              end: 20,
            },
          ],
        },
        parts: [{ type: "text", text: "/review then /review", state: "done" }],
      } as SessionMessage,
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

    expect(await screen.findAllByTestId("session-skill-directive")).toHaveLength(1);
    expect(screen.getByTestId("user-message-bubble")).toHaveTextContent("/review then /review");
  });

  it("Should preserve spacing between adjacent verified directives", async () => {
    const transcript = [
      {
        id: "user-adjacent-skill-invocations",
        role: "user",
        content: "/review /review",
        metadata: {
          skill_invocations: [
            {
              command_id: "skill:workspace:review:sha256:def456",
              token: "/review",
              label: "Review",
              source: "workspace",
              start: 0,
              end: 7,
            },
            {
              command_id: "skill:workspace:review:sha256:def456",
              token: "/review",
              label: "Review",
              source: "workspace",
              start: 8,
              end: 15,
            },
          ],
        },
        parts: [{ type: "text", text: "/review /review", state: "done" }],
      } as SessionMessage,
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

    expect(await screen.findAllByTestId("session-skill-directive")).toHaveLength(2);
    expect(screen.getByTestId("user-message-bubble").textContent).toBe("/review /review");
  });

  // Invariant: only daemon-verified `mentions` render as chips, at their offsets;
  // a session chip opens that session (US-008.AC-1/2, EC-1/3). Owner: user message.
  it("Should render verified mentions as chips and open a mentioned session", async () => {
    const user = userEvent.setup();
    const openSession = vi.fn();
    const text =
      "Compare @internal/acp/client_prompt.go with @session:sess-a under @web/src/ and @nope.md";
    const transcript = [
      {
        id: "user-mentions",
        role: "user",
        content: text,
        metadata: {
          mentions: [
            {
              kind: "file",
              token: "@internal/acp/client_prompt.go",
              target: "internal/acp/client_prompt.go",
              label: "client_prompt.go",
              start: 8,
              end: 38,
            },
            {
              kind: "session",
              token: "@session:sess-a",
              target: "sess-a",
              label: "Fix login bug",
              start: 44,
              end: 59,
            },
            {
              kind: "folder",
              token: "@web/src/",
              target: "web/src/",
              label: "src/",
              start: 66,
              end: 75,
            },
            { kind: "file", token: "@broken", start: 3 },
          ],
        },
        parts: [{ type: "text", text, state: "done" }],
      } as SessionMessage,
    ];

    renderThreadState({
      status: "success",
      messages: toReadonlyThreadMessages(transcript),
      subagentNavigation: { openSession },
    });

    const chips = await screen.findAllByTestId("session-mention-chip");
    expect(chips.map(chip => chip.getAttribute("data-mention-kind"))).toEqual([
      "file",
      "session",
      "folder",
    ]);
    expect(chips[0]).toHaveTextContent("client_prompt.go");
    // A plain span carries no accessible name; screen readers read hidden text instead.
    expect(within(chips[0]!).getByText("File internal/acp/client_prompt.go")).toHaveClass(
      "sr-only"
    );
    expect(chips[0]!.tagName).toBe("SPAN");
    expect(chips[2]).toHaveTextContent("src/");
    // The unresolved token stays text: the UI never claims what the daemon did not verify.
    expect(screen.getByTestId("user-message-bubble")).toHaveTextContent("and @nope.md");

    await user.click(screen.getByRole("button", { name: "Session Fix login bug (sess-a)" }));
    expect(openSession).toHaveBeenCalledWith(
      { sessionId: "sess-a", agentName: "", workspaceId: fixtureWorkspaceId() },
      { newWindow: false }
    );
  });

  it("Should render a message without mention metadata exactly as before", async () => {
    const text = "It started after the @Override handler in @internal/acp/client.go";
    const transcript = [
      {
        id: "user-pre-mentions",
        role: "user",
        content: text,
        parts: [{ type: "text", text, state: "done" }],
      } as SessionMessage,
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

    expect(await screen.findByTestId("user-message-bubble")).toHaveTextContent(text);
    expect(screen.queryByTestId("session-mention-chip")).not.toBeInTheDocument();
  });

  function mentionMessage(id: string, text: string, token: string): SessionMessage {
    const start = text.indexOf(token);
    return {
      id,
      role: "user",
      content: text,
      metadata: {
        mentions: [
          { kind: "file", token, target: "a.go", label: "a.go", start, end: start + token.length },
        ],
      },
      parts: [{ type: "text", text, state: "done" }],
    } as SessionMessage;
  }

  // Invariant (VC-18): a chip sits inside the markdown the author wrote around
  // its token, so emphasis and lists still form. Owner: user message.
  it("Should keep the markdown around a mention chip", async () => {
    const text = "**Read @a.go now**\n\n- check @a.go first";
    const message = mentionMessage("user-mention-markdown", text, "@a.go");
    const secondStart = text.lastIndexOf("@a.go");
    (message.metadata as { mentions: unknown[] }).mentions.push({
      kind: "file",
      token: "@a.go",
      target: "a.go",
      label: "a.go",
      start: secondStart,
      end: secondStart + 5,
    });
    renderThreadState({ status: "success", messages: toReadonlyThreadMessages([message]) });

    const chips = await screen.findAllByTestId("session-mention-chip");
    expect(chips).toHaveLength(2);
    expect(chips[0]!.closest("strong")).toHaveTextContent("Read a.go");
    expect(chips[1]!.closest("li")).toHaveTextContent("check a.go");
    expect(screen.getByTestId("user-message-bubble")).not.toHaveTextContent("**");
  });

  // Invariant (BR-3): the daemon only skips inline and fenced code, so it can
  // verify a token inside an indented block; there it reads as its literal
  // token in code, never a chip or the slot placeholder. Owner: user message.
  it("Should render a verified mention inside indented code as its literal token", async () => {
    const text = "Run this:\n\n    cat @a.go\n";
    renderThreadState({
      status: "success",
      messages: toReadonlyThreadMessages([mentionMessage("user-mention-code", text, "@a.go")]),
    });

    const bubble = await screen.findByTestId("user-message-bubble");
    expect(bubble.querySelector("code")).toHaveTextContent("cat @a.go");
    expect(screen.queryByTestId("session-mention-chip")).not.toBeInTheDocument();
    expect(bubble).not.toHaveTextContent("compozy-directive");
  });

  // Invariant (US-008): copying a selection across a chip yields its raw
  // token, so the pasted text still mentions the same target. Owner: user message.
  it("Should copy a selected mention chip as its raw token", async () => {
    const text = "Read @a.go now";
    renderThreadState({
      status: "success",
      messages: toReadonlyThreadMessages([mentionMessage("user-mention-copy", text, "@a.go")]),
    });

    const chip = await screen.findByTestId("session-mention-chip");
    const paragraph = chip.closest("p")!;
    const range = document.createRange();
    range.selectNodeContents(paragraph);
    const selection = window.getSelection()!;
    selection.removeAllRanges();
    selection.addRange(range);
    const setData = vi.fn();

    fireEvent.copy(paragraph, { clipboardData: { setData } });

    expect(setData).toHaveBeenCalledWith("text/plain", "Read @a.go now");
    selection.removeAllRanges();
  });

  // Invariant (US-008.EC-2, VC-21): a session chip shows the title recorded at
  // send time, even after that session was renamed. Owner: user message.
  it("Should keep the recorded session title after the session is renamed", async () => {
    const base = createFetchMock();
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) =>
        getPathname(input) === "/api/sessions"
          ? jsonResponse({
              sessions: [
                { ...primarySessionFixture, id: "sess-7f9e21", name: "ACP contract — final" },
              ],
              page: { has_more: false, limit: 50 },
            })
          : base(input, init)
      )
    );
    const text = "What did @session:sess-7f9e21 conclude?";
    const transcript = [
      {
        id: "user-renamed-mention",
        role: "user",
        content: text,
        metadata: {
          mentions: [
            {
              kind: "session",
              token: "@session:sess-7f9e21",
              target: "sess-7f9e21",
              label: "Review ACP prompt contract",
              start: 9,
              end: 29,
            },
          ],
        },
        parts: [{ type: "text", text, state: "done" }],
      } as SessionMessage,
    ];

    renderThreadState({
      status: "success",
      messages: toReadonlyThreadMessages(transcript),
      subagentNavigation: { openSession: vi.fn() },
    });

    const chip = await screen.findByRole("button", {
      name: "Session Review ACP prompt contract (sess-7f9e21)",
    });
    expect(chip).toHaveTextContent("Review ACP prompt contract");
    expect(screen.getByTestId("user-message-bubble")).not.toHaveTextContent("ACP contract — final");
  });

  it("Should render the attachment gallery above the user text bubble", async () => {
    const workspaceId = fixtureWorkspaceId();
    const sessionId = primarySessionFixture.id;
    const transcript = [
      {
        id: "user-with-attachments",
        role: "user",
        metadata: {
          attachments: [
            {
              id: "att-image",
              name: "diagram.png",
              mime_type: "image/png",
              bytes: 2048,
              sha256: "a".repeat(64),
              kind: "image",
              width: 1280,
              height: 720,
            },
            {
              id: "att-notes",
              name: "notes.pdf",
              mime_type: "application/pdf",
              bytes: 4096,
              sha256: "b".repeat(64),
              kind: "file",
            },
          ],
        },
        parts: [
          {
            type: "file",
            mediaType: "image/png",
            url: "compozy://session-attachments/att-image",
            filename: "diagram.png",
          },
          {
            type: "file",
            mediaType: "application/pdf",
            url: "compozy://session-attachments/att-notes",
            filename: "notes.pdf",
          },
          {
            type: "file",
            mediaType: "image/png",
            url: "compozy://session-attachments/att-image",
            filename: "diagram.png",
          },
          { type: "text", text: "Flatten both.", state: "done" },
        ],
      } as SessionMessage,
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

    const gallery = await screen.findByTestId("user-message-attachment-gallery");
    const bubble = screen.getByTestId("user-message-bubble");
    expect(bubble).toHaveTextContent("Flatten both.");
    expect(gallery.compareDocumentPosition(bubble) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    const imageFrames = screen.getAllByTestId("user-message-attachment-frame");
    expect(imageFrames).toHaveLength(2);
    expect(within(imageFrames[0]!).getByRole("img")).toHaveAttribute(
      "src",
      `/api/workspaces/${workspaceId}/sessions/${sessionId}/attachments/att-image/bytes`
    );
    expect(imageFrames[0]).not.toHaveAttribute("role");
    expect(screen.getByTestId("user-message-attachment-file-card")).toHaveAttribute(
      "href",
      `/api/workspaces/${workspaceId}/sessions/${sessionId}/attachments/att-notes/bytes`
    );
    expect(screen.getByTestId("user-message-attachment-file-card")).not.toHaveAttribute("role");
    expect(screen.getByTestId("user-message-attachment-file-card")).toHaveTextContent("notes.pdf");
    // Non-image files download, with a separate browsing context if download is unavailable.
    expect(screen.getByTestId("user-message-attachment-file-card")).toHaveAttribute(
      "download",
      "notes.pdf"
    );
    expect(screen.getByTestId("user-message-attachment-file-card")).toHaveAttribute(
      "target",
      "_blank"
    );
    expect(screen.getAllByRole("listitem")).toHaveLength(3);
    expect(within(gallery).getAllByRole("link")).toHaveLength(1);
    expect(within(gallery).getAllByRole("button")).toHaveLength(2);
    const images = screen.getAllByRole("img", { name: "diagram.png" });
    expect(images).toHaveLength(2);
    expect(images[0]).toHaveAttribute("width", "1280");
    expect(images[0]).toHaveAttribute("height", "720");
    expect(images[0]?.getAttribute("src")).not.toMatch(/^data:/);
  });

  // Invariant: image previews preserve the SPA and restore focus after every dismissal.
  // Owner: session attachment interaction, canonical SessionThread suite.
  it.each([
    ["close", false],
    ["escape", false],
    ["backdrop", false],
    ["close", true],
    ["escape", true],
    ["backdrop", true],
  ] as const)(
    "Should dismiss a transcript image with %s (window scoped: %s) and return to its origin",
    async (dismissal, windowScoped) => {
      const user = userEvent.setup();
      const originalURL = window.location.href;
      const transcript = [
        {
          id: "preview-image",
          role: "user",
          parts: [
            {
              type: "file",
              mediaType: "image/png",
              url: "compozy://session-attachments/att-shot",
              filename: "shot.png",
            },
          ],
        } as SessionMessage,
      ];
      const host = document.createElement("div");
      document.body.append(host);
      render(
        <OverlayContainerContext value={windowScoped ? host : null}>
          {threadStateElement(createQueryClient(), {
            status: "success",
            messages: toReadonlyThreadMessages(transcript),
          })}
        </OverlayContainerContext>,
        { container: host }
      );
      const trigger = await screen.findByRole("button", { name: "Preview shot.png" });
      expect(trigger).not.toHaveAttribute("href");
      await user.click(trigger);
      const dialog = await screen.findByRole("dialog", { name: "shot.png" });
      expect(within(dialog).getByRole("img", { name: "shot.png" })).toHaveAttribute(
        "src",
        `/api/workspaces/${fixtureWorkspaceId()}/sessions/${primarySessionFixture.id}/attachments/att-shot/bytes`
      );
      if (dismissal === "close")
        await user.click(within(dialog).getByRole("button", { name: "Close" }));
      else if (dismissal === "escape") await user.keyboard("{Escape}");
      else await user.click(document.querySelector<HTMLElement>('[data-slot="dialog-overlay"]')!);
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
      await waitFor(() => expect(trigger).toHaveFocus());
      expect(window.location.href).toBe(originalURL);
    }
  );

  it("Should skip the empty bubble on an image-only user turn", async () => {
    const transcript = [
      {
        id: "user-image-only",
        role: "user",
        parts: [
          {
            type: "file",
            mediaType: "image/png",
            url: "compozy://session-attachments/att-shot",
            filename: "shot.png",
          },
        ],
      } as SessionMessage,
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

    expect(await screen.findByTestId("user-message-attachment-gallery")).toBeInTheDocument();
    expect(screen.getByTestId("user-message-attachment-frame")).toBeInTheDocument();
    expect(screen.queryByTestId("user-message-bubble")).not.toBeInTheDocument();
    expect(screen.queryByTestId("user-message-actions-copy")).not.toBeInTheDocument();
  });

  it("Should render a local image-only attachment before transcript replacement", async () => {
    const transcript = [
      {
        id: "user-local-image-only",
        role: "user",
        parts: [
          {
            type: "data-compozy-attachment",
            data: {
              bytes: 2048,
              height: 720,
              id: "att-local-shot",
              kind: "image",
              mime_type: "image/png",
              name: "local-shot.png",
              sha256: "c".repeat(64),
              width: 1280,
            },
          },
        ],
      } as SessionMessage,
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

    expect(await screen.findByTestId("user-message-attachment-gallery")).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "local-shot.png" })).toHaveAttribute(
      "src",
      `/api/workspaces/${fixtureWorkspaceId()}/sessions/${primarySessionFixture.id}/attachments/att-local-shot/bytes`
    );
    expect(screen.queryByTestId("user-message-bubble")).not.toBeInTheDocument();
  });

  it("Should leave a text-only user turn without a gallery", async () => {
    const transcript = [
      {
        id: "user-text-only",
        role: "user",
        parts: [{ type: "text", text: "Ship the landing page.", state: "done" }],
      } as SessionMessage,
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

    expect(await screen.findByTestId("user-message-bubble")).toHaveTextContent(
      "Ship the landing page."
    );
    expect(screen.queryByTestId("user-message-attachment-gallery")).not.toBeInTheDocument();
  });

  it("Should copy only the user-turn text when attachments are present", async () => {
    const writeText = vi.fn<(value: string) => Promise<void>>().mockResolvedValue(undefined);
    const clipboardDescriptor = Object.getOwnPropertyDescriptor(navigator, "clipboard");
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });

    const transcript = [
      {
        id: "user-copy-with-attachments",
        role: "user",
        parts: [
          {
            type: "file",
            mediaType: "image/png",
            url: "compozy://session-attachments/att-image",
            filename: "diagram.png",
          },
          { type: "text", text: "Flatten both.", state: "done" },
        ],
      } as SessionMessage,
    ];

    try {
      renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

      const copy = await screen.findByTestId("user-message-actions-copy");
      fireEvent.click(copy);
      await waitFor(() => {
        expect(writeText).toHaveBeenCalledWith("Flatten both.");
      });
      expect(vi.mocked(toast.success)).toHaveBeenCalledWith("Message copied");
    } finally {
      if (clipboardDescriptor) {
        Object.defineProperty(navigator, "clipboard", clipboardDescriptor);
      } else {
        Reflect.deleteProperty(navigator as unknown as { clipboard?: unknown }, "clipboard");
      }
    }
  });

  // US-008.AC-3: copying a message with mention chips copies the raw token text, not the
  // chip labels. Owner: user message actions.
  it("Should copy the raw token text of a message rendered with mention chips", async () => {
    const writeText = vi.fn<(value: string) => Promise<void>>().mockResolvedValue(undefined);
    const clipboardDescriptor = Object.getOwnPropertyDescriptor(navigator, "clipboard");
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
    const text = "Compare @internal/acp/client_prompt.go with @session:sess-a";
    const transcript = [
      {
        id: "user-copy-with-mentions",
        role: "user",
        content: text,
        metadata: {
          mentions: [
            {
              kind: "file",
              token: "@internal/acp/client_prompt.go",
              target: "internal/acp/client_prompt.go",
              label: "client_prompt.go",
              start: 8,
              end: 38,
            },
            {
              kind: "session",
              token: "@session:sess-a",
              target: "sess-a",
              label: "Fix login bug",
              start: 44,
              end: 59,
            },
          ],
        },
        parts: [{ type: "text", text, state: "done" }],
      } as SessionMessage,
    ];

    try {
      renderThreadState({
        status: "success",
        messages: toReadonlyThreadMessages(transcript),
        subagentNavigation: { openSession: vi.fn() },
      });

      expect(await screen.findAllByTestId("session-mention-chip")).toHaveLength(2);
      fireEvent.click(await screen.findByTestId("user-message-actions-copy"));
      await waitFor(() => {
        expect(writeText).toHaveBeenCalledWith(text);
      });
    } finally {
      if (clipboardDescriptor) {
        Object.defineProperty(navigator, "clipboard", clipboardDescriptor);
      } else {
        Reflect.deleteProperty(navigator as unknown as { clipboard?: unknown }, "clipboard");
      }
    }
  });

  it("Should keep the message action visible without hover and report clipboard rejection", async () => {
    const writeText = vi
      .fn<(value: string) => Promise<void>>()
      .mockRejectedValue(new Error("blocked"));
    const clipboardDescriptor = Object.getOwnPropertyDescriptor(navigator, "clipboard");
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
    const transcript = [
      {
        id: "assistant-copy-rejected",
        role: "assistant",
        parts: [
          {
            type: "text",
            text: "The clipboard permission is blocked.",
            state: "done",
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
    ];

    try {
      renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

      const toolbar = await screen.findByTestId("assistant-message-actions");
      const copy = within(toolbar).getByTestId("assistant-message-actions-copy");
      expect(toolbar).toBeVisible();
      expect(copy).toBeVisible();
      expect(toolbar).not.toHaveClass("opacity-0", "pointer-events-none");

      fireEvent.click(copy);

      await waitFor(() => {
        expect(copy).toHaveAttribute("data-copy-state", "failed");
        expect(copy).toHaveAttribute("aria-label", "Copy failed");
        expect(vi.mocked(toast.error)).toHaveBeenCalledWith("Couldn't copy message");
      });
    } finally {
      if (clipboardDescriptor) {
        Object.defineProperty(navigator, "clipboard", clipboardDescriptor);
      } else {
        Reflect.deleteProperty(navigator as unknown as { clipboard?: unknown }, "clipboard");
      }
    }
  });

  // ADR-006 rule 3: the actions row (copy, timestamp) exists only once the
  // reply has text; a streaming turn with reasoning alone reserves nothing.
  it("Should mount no copy row before a streaming answer has text", async () => {
    const transcript = [
      {
        id: "assistant-streaming-reasoning",
        role: "assistant",
        parts: [
          {
            type: "reasoning",
            text: "Checking the tool output before answering.",
            state: "streaming",
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
      {
        id: "assistant-settled-tool-only",
        role: "assistant",
        parts: [
          {
            type: "tool-Bash",
            toolCallId: "tool-settled-only",
            state: "output-available",
            input: { command: "true" },
            output: { type: "tool_result", title: "Bash", raw: { content: "ok" } },
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

    expect(await screen.findByTestId("thinking-block")).toBeInTheDocument();
    expect(screen.queryByTestId("assistant-message-actions")).not.toBeInTheDocument();
  });

  it.each([
    ["goal-work", "Working on goal"],
    ["goal-continuation", "Working on goal"],
    ["goal-compaction", "Tidying up goal notes"],
  ] as const)(
    "Should render persisted %s prompt identity from the transcript",
    async (kind, label) => {
      const transcript = [
        {
          id: `assistant-${kind}`,
          role: "assistant",
          parts: [
            {
              type: "data-compozy-event",
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
      expect(notice).toHaveTextContent("step 3");
      expect(notice).not.toHaveTextContent("generation");
      expect(notice).toHaveAttribute("data-goal-kind", kind);
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
            type: "data-compozy-event",
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
            type: "data-compozy-event",
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

    function GrowingTranscriptThread({
      messages,
    }: {
      messages: ReturnType<typeof toReadonlyThreadMessages>;
    }) {
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

    const initialMessages = toReadonlyThreadMessages(initialTranscript);
    const grownMessages = toReadonlyThreadMessages(grownTranscript);
    const { rerender } = render(<GrowingTranscriptThread messages={initialMessages} />);

    expect(await screen.findByText("Continue delegated task run")).toBeInTheDocument();
    rerender(<GrowingTranscriptThread messages={grownMessages} />);
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

    // A settled run rests as one semantic summary line — no tool rows, no
    // paths, no count eyebrow. The two same-path variants stay distinct files.
    const summaryLabel = await screen.findByTestId("work-summary-label");
    expect(summaryLabel).toHaveTextContent("Read 8 files");
    expect(screen.queryByText(/\/tmp\/file-8\.ts/)).not.toBeInTheDocument();
    expect(screen.queryAllByTestId("tool-call-row")).toHaveLength(0);

    const toggle = screen.getByRole("button", { name: /Read 8 files/ });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    const detailsId = toggle.getAttribute("aria-controls");
    expect(detailsId).toBeTruthy();
    const details = document.getElementById(detailsId ?? "");
    expect(details).toBeInTheDocument();
    expect(details).toHaveAttribute("hidden");
    await user.click(toggle);

    expect(toggle).toHaveAttribute("aria-expanded", "true");
    expect(details).not.toHaveAttribute("hidden");
    expect(await screen.findByText(/\/tmp\/file-1\.ts/)).toBeInTheDocument();
    expect(screen.getAllByTestId("tool-call-row")).toHaveLength(8);

    toggle.focus();
    await user.keyboard("{Enter}");
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryAllByTestId("tool-call-row")).toHaveLength(0);
    expect(screen.queryByText(/\/tmp\/file-1\.ts/)).not.toBeInTheDocument();
  });

  it("Should announce several calls in flight as one running count", async () => {
    const transcript = [
      {
        id: "assistant-parallel",
        role: "assistant",
        parts: [
          {
            type: "tool-Read",
            toolCallId: "tool-read-a",
            state: "input-available",
            turn_id: "turn-parallel",
            timestamp: "2026-07-07T12:00:00Z",
            input: { file_path: "/tmp/a.ts" },
          },
          {
            type: "tool-Read",
            toolCallId: "tool-read-b",
            state: "input-available",
            turn_id: "turn-parallel",
            timestamp: "2026-07-07T12:00:01Z",
            input: { file_path: "/tmp/b.ts" },
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

    const row = await screen.findByTestId("live-tool");
    expect(row).toHaveAttribute("data-parallel", "2");
    // The count sits inside the disclosure button, so a sibling status speaks it.
    expect(within(row).getByRole("status")).toHaveTextContent("Running 2 tools…");
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

    // ADR-006: a call still in flight is the turn's one live row — the verb
    // shimmers, nothing on the right; never the legacy bordered "preparing
    // input" box and never a stacked card.
    const row = await screen.findByTestId("live-tool");
    expect(within(row).getByTestId("live-tool-label")).toHaveTextContent("Reading");
    // The running state is announced, not only shown by the shimmer.
    expect(within(row).getByRole("status")).toHaveTextContent("Reading");
    expect(row.querySelector('[data-slot="tool-call-row-status"]')).toBeNull();
    expect(screen.queryByTestId("tool-call-row")).not.toBeInTheDocument();
    expect(screen.queryByText(/preparing input/i)).not.toBeInTheDocument();
  });

  // Invariant (US-018.EC-2, task_06 VC-05): a paused background window keeps its
  // last frame — the live row and the status row show no motion, the words carry
  // the state, and the status row says when the frame is from. Owning layer:
  // thread motion (live-tool row + status row); canonical suite: this file.
  it("Should still the live row and the status row while the window is paused", async () => {
    const pausedAt = Date.parse("2026-07-07T12:02:00Z");
    const transcript = [
      { id: "user-paused", role: "user", parts: [{ type: "text", text: "Run the retry tests" }] },
      {
        id: "assistant-paused",
        role: "assistant",
        status: { type: "running" },
        parts: [
          {
            type: "tool-Bash",
            toolCallId: "tool-bash-paused",
            state: "input-streaming",
            turnId: "turn-paused",
            timestamp: "2026-07-07T12:00:00Z",
            input: { command: "go test ./internal/store/... -run Retry" },
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
    ];

    renderThreadState({
      status: "success",
      messages: toReadonlyThreadMessages(transcript as SessionMessage[]),
      isSessionRunning: true,
      liveDataEnabled: false,
      transport: {
        ...SESSION_TRANSPORT_LIVE,
        lastLiveAt: pausedAt,
        phase: "disabled",
        retry: () => undefined,
      },
    });

    const row = await screen.findByTestId("live-tool");
    expect(row).toHaveAttribute("data-still", "true");
    const label = within(row).getByTestId("live-tool-label");
    expect(label).toHaveTextContent(/^Running/);
    // The verb carries the motion; the paused window stills it to subtle text.
    const verb = within(label).getByText("Running");
    expect(verb).not.toHaveClass("session-shimmer");
    expect(verb).toHaveClass("text-subtle");
    const working = screen.getByTestId("session-working-row");
    expect(working.querySelector(".session-working-dots")).toBeNull();
    expect(within(working).getByTestId("session-working-as-of")).toHaveTextContent(/as of/);
  });

  it("Should preserve output-error text through the repository and render it on the failed row", async () => {
    const user = userEvent.setup();
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
            errorText: "terminal/create denied before writing workspace marker",
            output: {
              error: "stale nested provider error",
              stderr: "provider diagnostic",
            },
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

    // A tool error the turn absorbed reads as information (ADR-009): subtle ×,
    // the word "failed", and the error text preserved on the row.
    const row = await screen.findByTestId("tool-call-row");
    expect(row.querySelector('[data-slot="tool-call-row"]')?.getAttribute("data-status")).toBe(
      "absorbed"
    );
    expect(
      row.querySelector('[data-slot="tool-call-row-status"]')?.getAttribute("aria-label")
    ).toBe("Failed");
    expect(within(row).getByTestId("tool-call-state-word")).toHaveTextContent("failed");
    expect(row.querySelector('[data-slot="tool-call-row-preview"]')).toHaveTextContent(
      "terminal/create denied before writing workspace marker"
    );
    await user.click(row.querySelector('[data-slot="tool-call-row-trigger"]') as HTMLElement);
    expect(row.querySelector('[data-slot="tool-call-row-error"]')).toHaveTextContent(
      "terminal/create denied before writing workspace marker"
    );
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
            type: "tool-compozy__session_search",
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
            type: "tool-compozy__session_search",
            toolCallId: "tool-acp-artifact",
            state: "output-available",
            turn_id: "turn-acp-artifact",
            timestamp: "2026-07-19T04:31:00Z",
            input: { query: "release evidence" },
            output: {
              type: "tool_result",
              title: "Search sessions",
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
            type: "tool-compozy__session_search",
            toolCallId: "tool-persisted-artifact",
            state: "output-available",
            turn_id: "turn-persisted-artifact",
            timestamp: "2026-07-19T04:32:00Z",
            input: { query: "release evidence" },
            output: {
              type: "tool_result",
              title: "Search sessions",
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
    expect(screen.queryByText(/\/tmp\/launch\.md/)).not.toBeInTheDocument();
    expect(fold).toHaveAttribute("aria-expanded", "false");

    await user.click(fold);

    expect(fold).toHaveAttribute("aria-expanded", "true");
    expect(await screen.findByText(/\/tmp\/launch\.md/)).toBeInTheDocument();
    await user.click(screen.getByTestId("thinking-trigger"));
    // The settled thinking row previews the first line; the body carries the
    // full reasoning markdown once expanded.
    expect(
      within(screen.getByTestId("thinking-content")).getByText("Need to inspect the launch file.")
    ).toBeInTheDocument();

    await user.click(fold);

    expect(fold).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByText(/\/tmp\/launch\.md/)).not.toBeInTheDocument();
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
    expect(screen.getByTestId("changed-files-list")).toHaveAttribute("hidden");

    await user.click(toggle);

    // Expanded: one row per modified file, each with its diff stats.
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    expect(await screen.findByTestId("changed-files-list")).toBeInTheDocument();
    const fileRows = within(rollup).getAllByTestId("changed-file-row");
    expect(fileRows).toHaveLength(2);
    expect(within(fileRows[0]!).getByText("launch.ts")).toBeInTheDocument();
    expect(within(fileRows[1]!).getByText("notes.md")).toBeInTheDocument();
  });

  // Invariant (ADR-009, task_07 VC-06 both cells): a turn whose end this message
  // itself records — the operator's `stop_reason`, or the fallback steer that
  // replaced it — settles the call still awaiting its result: it reads
  // "stopped", never a live row, and the turn folds under its own label.
  // Owning layer: message timeline derivation; canonical suite: this file.
  it("Should settle a pending call when the message's own event ends its turn", async () => {
    const stoppedTurn = [
      {
        id: "assistant-stop-recorded",
        role: "assistant",
        parts: [
          {
            type: "tool-Bash",
            toolCallId: "tool-bash-cut",
            state: "input-available",
            turnId: "turn-cut",
            timestamp: "2026-07-07T12:00:00Z",
            input: { command: "go test ./internal/store/... -count=3" },
          },
          {
            type: "data-compozy-event",
            turnId: "turn-cut",
            timestamp: "2026-07-07T12:01:40Z",
            data: {
              type: "prompt_interrupted",
              turn_id: "turn-cut",
              stop_reason: "cancelled",
              timestamp: "2026-07-07T12:01:40Z",
            },
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
    ];
    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(stoppedTurn) });
    const interrupted = await screen.findByTestId("turn-fold-interrupted");
    expect(interrupted).toHaveTextContent(/You stopped after 1m 40s$/);
    expect(screen.queryByTestId("live-tool")).not.toBeInTheDocument();
    expect(screen.getByTestId("tool-call-state-word")).toHaveTextContent("stopped");
    // The rows keep their place; the label is the line at the end of the turn (VC-06).
    const label = within(interrupted).getByTestId("turn-fold-open-label");
    const row = within(interrupted).getByTestId("tool-call-row");
    expect(row.compareDocumentPosition(label) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("Should fold a turn a fallback steer replaced even with a call still pending", async () => {
    const replacedTurn = [
      {
        id: "assistant-replaced",
        role: "assistant",
        parts: [
          {
            type: "tool-Bash",
            toolCallId: "tool-bash-replaced",
            state: "input-available",
            turnId: "turn-replaced",
            timestamp: "2026-07-07T12:00:00Z",
            input: { command: "go build ./..." },
          },
          {
            type: "data-compozy-event",
            turnId: "turn-replaced",
            timestamp: "2026-07-07T12:00:48Z",
            data: {
              type: "transcript_marker.created",
              turn_id: "turn-replaced",
              timestamp: "2026-07-07T12:00:48Z",
              marker: {
                kind: "transcript_marker.prompt_steered",
                summary: "Steer fell back to interrupt.",
                occurred_at: "2026-07-07T12:00:48Z",
                evidence: { steer_delivery: "interrupt_fallback", mode: "steer" },
              },
            },
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
    ];
    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(replacedTurn) });
    expect(await screen.findByTestId("turn-fold-row")).toHaveTextContent(
      /^Interrupted after 48s · replaced by your steer/
    );
    expect(screen.queryByTestId("live-tool")).not.toBeInTheDocument();
  });

  it.each(["event", "interrupted", "cancelled", "canceled"])(
    "Should keep %s interruption visible with consistent tool detail",
    async interruption => {
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
              state: interruption === "event" ? "output-available" : interruption,
              turn_id: "turn-int",
              timestamp: "2026-07-07T12:00:04Z",
              input: { file_path: "/tmp/interrupted.md" },
              output: { type: "tool_result", title: "Read", raw: { content: "partial" } },
            },
            ...(interruption === "event"
              ? [
                  {
                    type: "data-compozy-event",
                    data: {
                      type: "session-stopped",
                      turn_id: "turn-int",
                      stop_reason: "cancelled",
                      timestamp: "2026-07-07T12:00:07Z",
                    },
                  },
                ]
              : []),
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
      expect(screen.getByText(/\/tmp\/interrupted\.md/)).toBeInTheDocument();
      expect(screen.getByText("Stopped before the summary.")).toBeInTheDocument();
      if (interruption !== "event") {
        expect(screen.getByTestId("tool-call-state-word")).toHaveTextContent("stopped");
        expect(screen.queryByTestId("live-tool")).not.toBeInTheDocument();
      }
    }
  );

  it("Should let the provider diagnostic own a live prompt failure instead of rendering it twice", async () => {
    // The live stream's errorText and the persisted diagnostic event carry the same daemon summary.
    const summary = "provider authentication required";
    const transcript = [
      {
        id: "assistant-provider-auth",
        role: "assistant",
        status: { type: "incomplete", reason: "error", error: summary },
        parts: [
          { type: "text", text: "Partial answer before the auth lapse.", state: "done" },
          {
            type: "data-compozy-event",
            data: {
              type: "error",
              turn_id: "assistant-provider-auth",
              error: summary,
              failure: { kind: "prompt_failure", summary },
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
        ] as unknown as SessionMessage["parts"],
      } as unknown as SessionMessage,
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

    expect(await screen.findByText("Partial answer before the auth lapse.")).toBeInTheDocument();
    const notice = screen.getByTestId("session-error-notice");
    expect(notice).toHaveAttribute("data-provider-error", "provider_auth_required");
    expect(screen.getAllByRole("alert")).toHaveLength(1);
    expect(screen.queryByTestId("session-message-error")).not.toBeInTheDocument();
  });

  it("Should keep an unrelated message error beside an earlier provider diagnostic", async () => {
    const transcript = [
      {
        id: "assistant-provider-then-transport",
        role: "assistant",
        status: { type: "incomplete", reason: "error", error: "connection stalled" },
        parts: [
          {
            type: "data-compozy-event",
            data: {
              type: "error",
              turn_id: "assistant-provider-then-transport",
              error: "provider rate limited",
              failure: { kind: "prompt_failure", summary: "provider rate limited" },
              provider_error: {
                code: "provider_rate_limited",
                provider: "claude-code",
                next_action: "retry",
                guidance: "retry after the provider recovers",
                occurrence_count: 1,
                first_seen_at: "2026-09-05T14:02:00Z",
                last_seen_at: "2026-09-05T14:02:00Z",
              },
            },
          },
        ] as unknown as SessionMessage["parts"],
      } as unknown as SessionMessage,
    ];

    renderThreadState({ status: "success", messages: toReadonlyThreadMessages(transcript) });

    expect(await screen.findByTestId("session-error-notice")).toHaveAttribute(
      "data-provider-error",
      "provider_rate_limited"
    );
    expect(screen.getByTestId("session-message-error")).toHaveTextContent("connection stalled");
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
            type: "data-compozy-event",
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
            type: "data-compozy-event",
            data: { type: "system", title: "CompozyOS Runtime Agent" },
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
            type: "data-compozy-event",
            data: { type: "system", title: "CompozyOS Runtime Agent" },
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
    // Settled, streaming, incomplete, and persisted-failure text remain copyable.
    const assistantToolbars = screen.getAllByTestId("assistant-message-actions");
    expect(assistantToolbars).toHaveLength(6);
    expect(screen.queryByRole("button", { name: "Use as Goal" })).not.toBeInTheDocument();
    expect(screen.getAllByTestId("assistant-message-actions-copy")).toHaveLength(6);
    const successfulToolbar = assistantToolbars[0]!;
    expect(
      within(successfulToolbar).getByTestId("assistant-message-actions-timestamp")
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
      expect(copy.querySelector('[data-slot="copy-idle-icon"]')).not.toBeNull();

      fireEvent.click(copy);

      // The whole markdown source lands on the clipboard, and the icon flips to a check.
      await waitFor(() => {
        expect(writeText).toHaveBeenCalledWith(markdownSource);
      });
      await waitFor(() => {
        expect(copy).toHaveAttribute("data-copied", "true");
      });
      expect(vi.mocked(toast.success)).toHaveBeenCalledWith("Message copied");
      expect(copy.querySelector('[data-slot="copy-success-icon"]')).not.toBeNull();
      expect(copy.querySelector('[data-slot="copy-idle-icon"]')).toBeNull();
    } finally {
      if (clipboardDescriptor) {
        Object.defineProperty(navigator, "clipboard", clipboardDescriptor);
      } else {
        Reflect.deleteProperty(navigator as unknown as { clipboard?: unknown }, "clipboard");
      }
    }
  });

  it("Should render one working row between the transcript and composer while a turn streams", async () => {
    // The active turn retains its timeline shape for folding, but the active work
    // status is a single composer-adjacent row rather than a duplicate in history.
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

    renderThreadState({
      status: "success",
      messages: toReadonlyThreadMessages(transcript),
      isSessionRunning: true,
      statusSession: runningStatusSession(startedAtIso, "Bash"),
    });

    const workingRow = await screen.findByTestId("session-working-row");
    expect(screen.getAllByTestId("session-working-row")).toHaveLength(1);
    expect(screen.getByTestId("chat-view")).not.toContainElement(workingRow);
    // Three stepped 4px dots on the duty cycle; the old spinner row is gone.
    expect(workingRow.querySelector('[data-slot="typing-dots"]')?.children).toHaveLength(3);
    expect(workingRow.querySelector(".animate-spin")).toBeNull();
    // Live "Working for Xs": the timer counts from the daemon's durable turn
    // start (US-027); the running tool is the live row's, never repeated here.
    expect(workingRow).toHaveTextContent(/Working for/);
    const timer = screen.getByTestId("session-working-timer");
    expect(timer.textContent).toMatch(/^\d+s$/);
    expect(screen.queryByTestId("session-working-activity")).not.toBeInTheDocument();
    expect(workingRow).not.toHaveTextContent("Running shell");
  });

  // Invariant: a pending stop states intent without presenting continued work or
  // confirmed death. Owner: composed thread status; canonical suite: this file.
  it("Should show stopping until the pending stop resolves", async () => {
    renderThreadState({
      status: "success",
      messages: [],
      isSessionRunning: true,
      stopPhase: "stopping",
      statusSession: runningStatusSession("2026-09-06T15:00:00Z", "Bash"),
    });
    expect(await screen.findByTestId("session-stopping-row")).toHaveTextContent("Stopping…");
    expect(screen.queryByTestId("session-working-row")).toBeNull();
    expect(screen.queryByTestId("session-stopped-row")).toBeNull();
  });

  it("Should keep a settled tool tail live when the message id differs from its part turn id", async () => {
    const transcript = [
      {
        id: "assistant-live-tools",
        role: "assistant",
        status: { type: "running" },
        parts: [
          {
            type: "tool-Read",
            toolCallId: "live-read-1",
            state: "output-available",
            turn_id: "turn-live-tools",
            timestamp: "2026-07-07T12:00:00Z",
            input: { file_path: "/tmp/live-1.ts" },
            output: { type: "tool_result", title: "Read", raw: { content: "one" } },
          },
          {
            type: "tool-Read",
            toolCallId: "live-read-2",
            state: "output-available",
            turn_id: "turn-live-tools",
            timestamp: "2026-07-07T12:00:01Z",
            input: { file_path: "/tmp/live-2.ts" },
            output: { type: "tool_result", title: "Read", raw: { content: "two" } },
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
    ];

    renderThreadState({
      status: "success",
      messages: toReadonlyThreadMessages(transcript),
      isSessionRunning: true,
    });

    // The live turn's completed tools rest as one group sentence (ADR-006),
    // still marked live so the turn never folds while it streams.
    const group = await screen.findByTestId("work-summary-row");
    expect(group).toHaveAttribute("data-live", "true");
    expect(within(group).getByTestId("work-summary-label")).toHaveTextContent("Read 2 files");
    expect(screen.queryAllByTestId("tool-call-row")).toHaveLength(0);
    expect(screen.getAllByTestId("session-working-row")).toHaveLength(1);
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

      renderThreadState({
        status: "success",
        messages: toReadonlyThreadMessages(transcript),
        isSessionRunning: true,
        statusSession: runningStatusSession("2026-07-07T12:00:00Z"),
      });

      const workingRow = await screen.findByTestId("session-working-row");
      expect(workingRow).toHaveTextContent(/Working for/);
      // Reduced motion removes only the dots; the elapsed runtime fact remains.
      expect(workingRow.querySelector('[data-slot="typing-dots"]')).toBeNull();
      expect(screen.getByTestId("session-working-timer")).toBeInTheDocument();
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
      const renderSpy = vi.fn();
      function Probe() {
        renderSpy();
        return (
          <SessionThinkingRow
            session={runningStatusSession(new Date(startedAt).toISOString())}
            running
            thinking={false}
            lastTurn={null}
          />
        );
      }

      render(<Probe />);
      expect(renderSpy).toHaveBeenCalledTimes(1);
      const timer = screen.getByTestId("session-working-timer");
      expect(timer.textContent).toBe("0s");

      // Advancing the fake clock 3s also advances `Date.now()`, so the interval
      // ticks land at 1s/2s/3s past the anchored start.
      act(() => {
        vi.advanceTimersByTime(3000);
      });

      // The text node advanced to 3s while the React tree never re-rendered.
      expect(timer.textContent).toBe("3s");
      expect(renderSpy).toHaveBeenCalledTimes(1);
    } finally {
      vi.useRealTimers();
    }
  });

  // jsdom leaves scroll metrics at 0, so the live-follow machine cannot observe a
  // gap on its own; mock the viewport geometry to place the reader away from the
  // live edge, then emit the scroll event that represents the changed viewport.
  function primeScrolledAwayViewport(viewport: HTMLElement) {
    Object.defineProperty(viewport, "scrollHeight", { configurable: true, value: 2000 });
    Object.defineProperty(viewport, "clientHeight", { configurable: true, value: 500 });
    viewport.scrollTop = 300;
  }

  it.each([
    ["a wheel", (viewport: HTMLElement) => fireEvent.wheel(viewport, { deltaY: -120 })],
    ["a navigation key", (viewport: HTMLElement) => fireEvent.keyDown(viewport, { key: "PageUp" })],
  ])(
    "Should reveal the scroll-to-bottom pill when the reader scrolls away with %s and return to the live edge on click",
    async (_gesture, scrollAway) => {
      const messages = toReadonlyThreadMessages(sessionTranscriptFixture.slice(0, 2));

      renderThreadState({ status: "success", messages });

      expect(await screen.findByText("Launch readiness snapshot")).toBeInTheDocument();
      expect(screen.getByTestId("thread-messages")).toBeInTheDocument();
      expect(screen.queryByText(/Start a conversation/i)).not.toBeInTheDocument();
      const viewport = screen.getByTestId("chat-view");
      const pill = screen.getByTestId("scroll-to-bottom-pill");
      // Following the live edge: the pill is hidden and non-interactive.
      expect(pill).toHaveAttribute("data-visible", "false");
      expect(pill).toBeDisabled();

      // The reader's gesture hands the viewport over; the scroll it causes keeps it there.
      primeScrolledAwayViewport(viewport);
      scrollAway(viewport);
      fireEvent.scroll(viewport);
      await waitFor(() => {
        expect(pill).toHaveAttribute("data-visible", "true");
        expect(pill).toBeEnabled();
      });
      expect(viewport.scrollTop).toBe(300);

      // Clicking the pill returns to the live edge and hides the affordance again.
      fireEvent.click(pill);
      await waitFor(() => {
        expect(pill).toHaveAttribute("data-visible", "false");
      });
    }
  );

  // Invariant (#907): only a reader gesture detaches live-follow. Opening a long
  // transcript lands on estimated row heights; as rows measure, the virtualizer's
  // own compensation moves the viewport off the end with no gesture. Following
  // must hold and re-pin the true end instead of reading that as the reader.
  it("Should hold the live edge when the viewport leaves it without a reader gesture", async () => {
    const messages = toReadonlyThreadMessages(
      Array.from({ length: 40 }, (_, index) => ({
        id: `settling-${index}`,
        role: index % 2 === 0 ? "user" : "assistant",
        parts: [{ type: "text", text: `Message ${index}`, state: "done" }],
      })) as SessionMessage[]
    );

    renderThreadState({ status: "success", messages });
    expect(await screen.findByText("Message 0")).toBeInTheDocument();

    const viewport = screen.getByTestId("chat-view");
    const pill = screen.getByTestId("scroll-to-bottom-pill");
    primeScrolledAwayViewport(viewport);
    fireEvent.scroll(viewport);

    expect(viewport.scrollTop).toBe(1_500);
    expect(pill).toHaveAttribute("data-visible", "false");

    // Late row measurement grows the content after the first re-pin.
    Object.defineProperty(viewport, "scrollHeight", { configurable: true, value: 3000 });
    fireEvent.scroll(viewport);

    expect(viewport.scrollTop).toBe(2_500);
    expect(pill).toHaveAttribute("data-visible", "false");
  });

  function settlingTranscript(prefix: string, length = 40) {
    return toReadonlyThreadMessages(
      Array.from({ length }, (_, index) => ({
        id: `${prefix}-${index}`,
        role: index % 2 === 0 ? "user" : "assistant",
        parts: [{ type: "text", text: `Message ${index}`, state: "done" }],
      })) as SessionMessage[]
    );
  }

  function primeViewportAtEnd(viewport: HTMLElement) {
    Object.defineProperty(viewport, "scrollHeight", { configurable: true, value: 2000 });
    Object.defineProperty(viewport, "clientHeight", { configurable: true, value: 500 });
    viewport.scrollTop = 1_500;
  }

  function scrollViewportTo(viewport: HTMLElement, scrollTop: number) {
    viewport.scrollTop = scrollTop;
    fireEvent.scroll(viewport);
  }

  // Invariant (#907): opening a long transcript whose mounted rows measure
  // differently from their estimates lands on, and stays at, the true end. jsdom has no
  // layout, so this simulates the browser contract the virtualizer relies on:
  // rows report a measured height, the scroller's height follows the rendered
  // sizer, writes clamp to the scrollable range, and each move dispatches an
  // asynchronous scroll event (no gesture behind any of them).
  it("Should land at the true end when measured rows differ from their estimates", async () => {
    // Real transcripts mix rows far taller than estimated with rows that render
    // nothing (operational markers measure 0px): both move the viewport.
    const measuredRowHeight = (index: number) => (index % 3 === 2 ? 0 : 360);
    const viewportHeight = 500;
    const isViewport = (element: Element) =>
      element instanceof HTMLElement && element.dataset.testid === "chat-view";
    const scrollHeightOf = (viewport: Element) => {
      const sizer = viewport.querySelector<HTMLElement>('[data-testid="thread-messages"]');
      if (!sizer) return viewportHeight;
      const rows = [...sizer.querySelectorAll<HTMLElement>("[data-index]")].reduce(
        (total, row) => total + measuredRowHeight(Number(row.dataset.index)),
        0
      );
      const padding =
        Number.parseFloat(sizer.style.paddingTop || "0") +
        Number.parseFloat(sizer.style.paddingBottom || "0");
      return Math.max(viewportHeight, padding + rows);
    };
    const offsets = new WeakMap<Element, number>();
    const restore: Array<() => void> = [];
    const patch = <T extends object>(target: T, key: string, descriptor: PropertyDescriptor) => {
      const original = Object.getOwnPropertyDescriptor(target, key);
      Object.defineProperty(target, key, { configurable: true, ...descriptor });
      restore.push(() => {
        if (original) Object.defineProperty(target, key, original);
        else delete (target as Record<string, unknown>)[key];
      });
      return original;
    };
    const originalOffsetHeight = Object.getOwnPropertyDescriptor(
      HTMLElement.prototype,
      "offsetHeight"
    );
    patch(HTMLElement.prototype, "offsetHeight", {
      get(this: HTMLElement) {
        if (this.hasAttribute("data-index")) return measuredRowHeight(Number(this.dataset.index));
        return originalOffsetHeight?.get?.call(this) ?? 0;
      },
    });
    const originalScrollHeight = Object.getOwnPropertyDescriptor(Element.prototype, "scrollHeight");
    patch(Element.prototype, "scrollHeight", {
      get(this: Element) {
        return isViewport(this)
          ? scrollHeightOf(this)
          : (originalScrollHeight?.get?.call(this) ?? 0);
      },
    });
    const originalClientHeight = Object.getOwnPropertyDescriptor(Element.prototype, "clientHeight");
    patch(Element.prototype, "clientHeight", {
      get(this: Element) {
        return isViewport(this) ? viewportHeight : (originalClientHeight?.get?.call(this) ?? 0);
      },
    });
    const originalScrollTop = Object.getOwnPropertyDescriptor(Element.prototype, "scrollTop");
    const writeScrollTop = (element: Element, value: number) => {
      const max = Math.max(0, scrollHeightOf(element) - viewportHeight);
      const next = Math.min(max, Math.max(0, value));
      if (next === (offsets.get(element) ?? 0)) return;
      offsets.set(element, next);
      setTimeout(() => element.dispatchEvent(new Event("scroll")), 0);
    };
    patch(Element.prototype, "scrollTop", {
      get(this: Element) {
        return isViewport(this)
          ? (offsets.get(this) ?? 0)
          : (originalScrollTop?.get?.call(this) ?? 0);
      },
      set(this: Element, value: number) {
        if (isViewport(this)) writeScrollTop(this, value);
        else originalScrollTop?.set?.call(this, value);
      },
    });
    // Layout changes notify every observer of its observed boxes, as the
    // browser does after a size change (the setup mock never fires).
    const observers = new Set<{ fire: () => void }>();
    let notifyQueued = false;
    const notifyObservers = () => {
      if (notifyQueued) return;
      notifyQueued = true;
      setTimeout(() => {
        notifyQueued = false;
        for (const observer of observers) observer.fire();
      }, 0);
    };
    class LayoutResizeObserver {
      private readonly targets = new Set<Element>();
      private readonly handle = {
        fire: () => {
          if (this.targets.size === 0) return;
          this.callback(
            [...this.targets].map(target => ({ target }) as ResizeObserverEntry),
            this as unknown as ResizeObserver
          );
        },
      };
      constructor(private readonly callback: ResizeObserverCallback) {
        observers.add(this.handle);
      }
      observe(target: Element) {
        this.targets.add(target);
        notifyObservers();
      }
      unobserve(target: Element) {
        this.targets.delete(target);
      }
      disconnect() {
        this.targets.clear();
        observers.delete(this.handle);
      }
    }
    patch(window, "ResizeObserver", { value: LayoutResizeObserver, writable: true });
    const layoutChanges = new MutationObserver(notifyObservers);
    layoutChanges.observe(document.body, {
      attributes: true,
      attributeFilter: ["style"],
      childList: true,
      subtree: true,
    });
    restore.push(() => layoutChanges.disconnect());
    const originalScrollTo = Element.prototype.scrollTo;
    patch(Element.prototype, "scrollTo", {
      value(this: Element, optionsOrX?: ScrollToOptions | number, y?: number) {
        const top = typeof optionsOrX === "number" ? y : optionsOrX?.top;
        if (isViewport(this)) writeScrollTop(this, top ?? offsets.get(this) ?? 0);
        else Reflect.apply(originalScrollTo, this, [optionsOrX, y]);
      },
      writable: true,
    });

    try {
      renderThreadState({ status: "success", messages: settlingTranscript("measured", 60) });
      expect(await screen.findByText("Message 59")).toBeInTheDocument();
      const viewport = screen.getByTestId("chat-view");
      const distanceFromEnd = () =>
        viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight;

      await waitFor(() => {
        expect(viewport.scrollTop).toBeGreaterThan(0);
        expect(distanceFromEnd()).toBe(0);
      });
      // Let the late measurement and scroll events settle, then check again.
      await act(() => new Promise(resolve => setTimeout(resolve, 100)));
      expect(distanceFromEnd()).toBe(0);
      expect(screen.getByTestId("scroll-to-bottom-pill")).toHaveAttribute("data-visible", "false");
    } finally {
      for (const undo of restore.reverse()) undo();
    }
  });

  // Invariant: a reader gesture that starts at the end keeps the viewport through
  // small deltas inside the 40px band (slow trackpads, the first key repeat, a
  // scrollbar drag); following re-arms only when the reader comes back down.
  // Chromium applies the wheel's scroll before the wheel event runs, so each step
  // moves the viewport, then dispatches the gesture, then the scroll event.
  it("Should let a gesture leave the end in small steps and re-arm only on the way back", async () => {
    renderThreadState({ status: "success", messages: settlingTranscript("small-steps") });
    expect(await screen.findByText("Message 0")).toBeInTheDocument();

    const viewport = screen.getByTestId("chat-view");
    primeViewportAtEnd(viewport);
    fireEvent.scroll(viewport);
    for (const scrollTop of [1_490, 1_480, 1_470]) {
      viewport.scrollTop = scrollTop;
      fireEvent.wheel(viewport, { deltaY: -10 });
      fireEvent.scroll(viewport);
      expect(viewport.scrollTop).toBe(scrollTop);
    }
    expect(screen.getByTestId("scroll-to-bottom-pill")).toHaveAttribute("data-visible", "false");

    // Layout compensation moving the viewport down is not the reader coming back.
    scrollViewportTo(viewport, 1_478);
    expect(viewport.scrollTop).toBe(1_478);

    // Heading back into the band re-arms following, which pins the true end.
    viewport.scrollTop = 1_495;
    fireEvent.wheel(viewport, { deltaY: 25 });
    fireEvent.scroll(viewport);
    expect(viewport.scrollTop).toBe(1_500);
    expect(screen.getByTestId("scroll-to-bottom-pill")).toHaveAttribute("data-visible", "false");
  });

  // Invariant: a scrollbar drag is one held pointer; dragging back down into the
  // band re-arms following even though no wheel or key event says "down".
  it("Should re-arm following when a held pointer drags the viewport back to the end", async () => {
    renderThreadState({ status: "success", messages: settlingTranscript("scrollbar-drag") });
    expect(await screen.findByText("Message 0")).toBeInTheDocument();

    const viewport = screen.getByTestId("chat-view");
    primeViewportAtEnd(viewport);
    fireEvent.pointerDown(viewport);
    scrollViewportTo(viewport, 1_200);
    expect(viewport.scrollTop).toBe(1_200);
    expect(screen.getByTestId("scroll-to-bottom-pill")).toHaveAttribute("data-visible", "true");

    scrollViewportTo(viewport, 1_480);
    expect(viewport.scrollTop).toBe(1_500);
    fireEvent.pointerUp(document);
    expect(screen.getByTestId("scroll-to-bottom-pill")).toHaveAttribute("data-visible", "false");
  });

  // Invariant: pressing transcript content (selecting text, clicking a row
  // control) at the end is not a scroll; following holds until the press
  // actually drags the viewport.
  it("Should keep following when a press on transcript content does not scroll", async () => {
    renderThreadState({ status: "success", messages: settlingTranscript("content-press") });
    expect(await screen.findByText("Message 0")).toBeInTheDocument();

    const viewport = screen.getByTestId("chat-view");
    primeViewportAtEnd(viewport);
    const row = screen.getByText("Message 0");
    fireEvent.pointerDown(row);
    Object.defineProperty(viewport, "scrollHeight", { configurable: true, value: 1800 });
    scrollViewportTo(viewport, 1_300);
    expect(viewport.scrollTop).toBe(1_300);
    fireEvent.pointerUp(document);
    Object.defineProperty(viewport, "scrollHeight", { configurable: true, value: 2000 });

    // Still following: a later non-gesture scroll off the end is pinned back.
    scrollViewportTo(viewport, 300);
    expect(viewport.scrollTop).toBe(1_500);
    expect(screen.getByTestId("scroll-to-bottom-pill")).toHaveAttribute("data-visible", "false");
  });

  // Invariant: a key a focused control consumes (Shift+Space activating "Copy")
  // is not a scroll gesture, so following holds; the keys it does not consume
  // (ArrowUp on that same button) still hand the viewport to the reader.
  it("Should tell keys a focused control consumes from keys that scroll past it", async () => {
    renderThreadState({ status: "success", messages: settlingTranscript("control-key") });
    expect(await screen.findByText("Message 0")).toBeInTheDocument();

    const viewport = screen.getByTestId("chat-view");
    const pill = screen.getByTestId("scroll-to-bottom-pill");
    primeViewportAtEnd(viewport);
    const control = within(viewport).getAllByRole("button")[0]!;
    fireEvent.keyDown(control, { key: " ", shiftKey: true });

    // Still following: a later non-gesture scroll off the end is pinned back.
    scrollViewportTo(viewport, 300);
    expect(viewport.scrollTop).toBe(1_500);
    expect(pill).toHaveAttribute("data-visible", "false");

    fireEvent.keyDown(control, { key: "ArrowUp" });
    scrollViewportTo(viewport, 300);
    expect(viewport.scrollTop).toBe(300);
    expect(pill).toHaveAttribute("data-visible", "true");
  });

  it("Should not apply a stale live-edge reconciliation after the reader scrolls away", async () => {
    const pendingFrames: FrameRequestCallback[] = [];
    const requestFrame = vi.spyOn(window, "requestAnimationFrame").mockImplementation(callback => {
      pendingFrames.push(callback);
      return pendingFrames.length;
    });
    const messages = toReadonlyThreadMessages(
      Array.from({ length: 40 }, (_, index) => ({
        id: `reader-controlled-${index}`,
        role: index % 2 === 0 ? "user" : "assistant",
        parts: [{ type: "text", text: `Message ${index}`, state: "done" }],
      })) as SessionMessage[]
    );

    try {
      renderThreadState({ status: "success", messages });
      expect(await screen.findByText("Message 0")).toBeInTheDocument();

      const viewport = screen.getByTestId("chat-view");
      primeScrolledAwayViewport(viewport);
      fireEvent.wheel(viewport, { deltaY: -120 });
      fireEvent.scroll(viewport);
      const scrollTo = vi.fn((optionsOrX?: ScrollToOptions | number, y?: number) => {
        const top = typeof optionsOrX === "number" ? y : optionsOrX?.top;
        viewport.scrollTop = top ?? viewport.scrollTop;
      });
      viewport.scrollTo = scrollTo;

      const reconciliation = pendingFrames.shift();
      expect(reconciliation).toBeDefined();
      act(() => reconciliation?.(performance.now()));

      expect(scrollTo).not.toHaveBeenCalled();
      expect(viewport.scrollTop).toBe(300);
    } finally {
      requestFrame.mockRestore();
    }
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

    // Collapsed settled run: the summary line is visible; the reader has
    // scrolled to a fixed offset before expanding the group.
    expect(await screen.findByTestId("work-summary-label")).toHaveTextContent("Read 8 files");
    const viewport = screen.getByTestId("chat-view");
    primeScrolledAwayViewport(viewport);

    // Expanding only changes the XState row store. The virtualizer owns height
    // measurement and scroll compensation, so the disclosure has no competing
    // imperative scroll writer.
    fireEvent.click(screen.getByRole("button", { name: /Read 8 files/ }));

    expect(await screen.findByText(/\/tmp\/anchor-1\.ts/)).toBeInTheDocument();
    expect(viewport.scrollTop).toBe(300);
  });

  it.each([false, true])(
    "Should preserve a work disclosure node and focus as entries arrive and settle (mixed: %s)",
    async mixed => {
      const user = userEvent.setup();
      const queryClient = createQueryClient();
      const toolPart = (toolCallId: string, index: number) => ({
        type: "tool-Read",
        // Match the reasoning part's fallback id to exercise cross-kind identity.
        toolCallId: mixed && toolCallId === "b" ? "assistant-anchor-stable:0" : toolCallId,
        state: "output-available",
        turn_id: "turn-anchor-stable",
        timestamp: `2026-07-07T12:00:0${index}Z`,
        input: { file_path: `/tmp/stable-${toolCallId}.ts` },
        output: { type: "tool_result", title: "Read", raw: { content: toolCallId } },
      });
      const initialMessages = toReadonlyThreadMessages([
        {
          id: "assistant-anchor-stable",
          role: "assistant",
          parts: [
            ...(mixed
              ? [
                  {
                    type: "reasoning",
                    id: "stable-thought",
                    text: "Checking **files**",
                    state: "streaming",
                    turn_id: "turn-anchor-stable",
                  },
                ]
              : []),
            ...["b", "c", "d", "e", "f"].map((id, index) => toolPart(id, index)),
          ],
        } as SessionMessage,
      ]);
      const grownMessages = toReadonlyThreadMessages([
        {
          id: "assistant-anchor-stable",
          role: "assistant",
          parts: [
            ...(mixed
              ? [
                  {
                    type: "reasoning",
                    id: "stable-thought",
                    text: "Checked **files**",
                    state: "done",
                    turn_id: "turn-anchor-stable",
                  },
                ]
              : []),
            ...["a", "b", "c", "d", "e", "f"].map((id, index) => toolPart(id, index)),
          ],
        } as SessionMessage,
      ]);
      const tree = (messages: readonly ThreadMessage[]) => (
        <QueryClientProvider client={queryClient}>
          <SessionChatRuntimeProvider
            sessionId={primarySessionFixture.id}
            workspaceId={fixtureWorkspaceId()}
          >
            <SessionTranscriptThreadProvider
              messages={messages}
              status="success"
              error={null}
              retry={() => {}}
            >
              <SessionThread
                sessionId={primarySessionFixture.id}
                agentName={primarySessionFixture.agent_name}
                canPrompt
                onCancelPrompt={() => {}}
                isSessionRunning={false}
              />
            </SessionTranscriptThreadProvider>
          </SessionChatRuntimeProvider>
        </QueryClientProvider>
      );

      const view = render(tree(initialMessages));
      const initialButton = await screen.findByRole("button", {
        name: mixed ? "5 tools · 1 thought · thinking" : "Read 5 files",
      });
      initialButton.focus();
      await user.click(initialButton);
      expect(initialButton).toHaveAttribute("aria-expanded", "true");

      const thought = mixed ? screen.getByTestId("thinking-block") : null;
      const retainedTool = screen
        .getAllByTestId("tool-call-row")
        .find(row => row.textContent?.includes("stable-b.ts"));
      expect(retainedTool).toBeDefined();
      expect(screen.getAllByTestId("tool-call-row")).toHaveLength(5);

      view.rerender(tree(grownMessages));

      const grownButton = await screen.findByRole("button", {
        name: mixed ? "6 tools · 1 thought" : "Read 6 files",
      });
      expect(screen.getAllByTestId("tool-call-row")).toHaveLength(6);
      expect(
        screen.getAllByTestId("tool-call-row").find(row => row.textContent?.includes("stable-b.ts"))
      ).toBe(retainedTool);
      if (mixed) {
        expect(screen.getByTestId("thinking-block")).toBe(thought);
        const thoughtTrigger = within(thought!).getByTestId("thinking-trigger");
        expect(thoughtTrigger).toHaveAttribute("aria-expanded", "true");
        expect(within(thought!).getByRole("region", { name: "Reasoning" })).toHaveTextContent(
          "Checked files"
        );
        expect(
          thought!.compareDocumentPosition(retainedTool!) & Node.DOCUMENT_POSITION_FOLLOWING
        ).toBeTruthy();
      }
      expect(grownButton).toBe(initialButton);
      expect(grownButton).toHaveAttribute("aria-expanded", "true");
      grownButton.focus();
      await user.keyboard("{Enter}");
      expect(grownButton).toHaveAttribute("aria-expanded", "false");
      expect(document.activeElement).toBe(grownButton);
    }
  );
});

// Suite: streaming render-count probe (task 39).
// Invariant: semantic row equality + compiler-stabilized `TimelineRowContent` mean streaming N
// chunk updates change only the live row's visible subtree; settled rows keep their mounted DOM.
// Boundary IN: `sessionRowEqual` + the compiler-managed row renderer.
// Boundary OUT: SSE/query wiring (owned by use-session-live-tail.test.tsx).
describe("SessionThread streaming render-count", () => {
  function streamingTranscript(liveText: string): readonly ThreadMessage[] {
    return toReadonlyThreadMessages([
      {
        id: "assistant-streaming-render-isolation",
        role: "assistant",
        status: { type: "running" },
        parts: [
          {
            type: "text",
            text: "Settled answer",
            state: "done",
            turn_id: "turn-render-isolation",
          },
          // A settled call draws a row between the blocks; adjacent prose would read as one.
          {
            type: "tool-Read",
            toolCallId: "call-render-isolation",
            state: "output-available",
            input: { file_path: "README.md" },
            output: "ok",
            turn_id: "turn-render-isolation",
          },
          {
            type: "text",
            text: liveText,
            state: "streaming",
            turn_id: "turn-render-isolation",
          },
        ] as unknown as SessionMessage["parts"],
      } as SessionMessage,
    ]);
  }

  function renderStreamingTimeline(liveText: string) {
    const queryClient = createQueryClient();
    const tree = (nextLiveText: string) => (
      <QueryClientProvider client={queryClient}>
        <SessionChatRuntimeProvider
          sessionId={primarySessionFixture.id}
          workspaceId={fixtureWorkspaceId()}
        >
          <SessionTranscriptThreadProvider
            messages={streamingTranscript(nextLiveText)}
            status="success"
            error={null}
            retry={vi.fn()}
          >
            <SessionThread
              sessionId={primarySessionFixture.id}
              agentName={primarySessionFixture.agent_name}
              canPrompt
              onCancelPrompt={vi.fn()}
              isSessionRunning
            />
          </SessionTranscriptThreadProvider>
        </SessionChatRuntimeProvider>
      </QueryClientProvider>
    );
    const view = render(tree(liveText));
    return { rerender: (nextLiveText: string) => view.rerender(tree(nextLiveText)) };
  }

  it("Should preserve the settled timeline row while the real stream receives chunks", async () => {
    const view = renderStreamingTimeline("chunk 1");
    const settledElement = await screen.findByText("Settled answer");

    for (const chunk of [
      "chunk 1 chunk 2",
      "chunk 1 chunk 2 chunk 3",
      "chunk 1 chunk 2 chunk 3 chunk 4",
    ]) {
      view.rerender(chunk);

      expect(await screen.findByText(chunk)).toBeInTheDocument();
      expect(screen.getByText("Settled answer")).toBe(settledElement);
    }
  });
});

// Suite: composer draft, running-state semantics + queued-prompt strip (task 35).
// Invariant: Draft text survives the app's StrictMode lifecycle exactly, Enter has one
// defined meaning per phase, only one busy-input submission is active, and queued prompts render
// as real, actionable rows.
// Boundary IN: SessionComposer draft persistence, phase toggle, Enter interception,
// queued-strip wiring.
// Boundary OUT: queue/steer/cancel API orchestration, covered by use-session-page-controls.test.tsx.

// The composer input is a Lexical contenteditable, so the runtime composer — not a
// textarea `value` — is the source of truth for authored text. The probe exposes the
// same `aui` handle the composer uses, so tests drive and read canonical raw text
// (tokens, not the chip labels the contenteditable renders).
let composerAui: ReturnType<typeof useAui> | null = null;

function ComposerAuiProbe() {
  const aui = useAui();
  useEffect(() => {
    composerAui = aui;
    return () => {
      composerAui = null;
    };
  }, [aui]);
  return null;
}

function requireComposerAui() {
  if (!composerAui) {
    throw new Error("composer runtime probe is not mounted");
  }
  return composerAui;
}

/** Typing-equivalent: replaces the whole composer text and parks the cursor at its end. */
async function setComposerText(text: string) {
  const aui = requireComposerAui();
  await act(async () => {
    aui.composer.setText(text);
  });
}

function composerText(): string {
  return requireComposerAui().composer.getState().text;
}

/** Lexical listens for key events on the contenteditable, not on the labelled wrapper. */
async function findComposerEditable(): Promise<HTMLElement> {
  const input = await screen.findByTestId("composer-input");
  const editable = input.querySelector<HTMLElement>('[contenteditable="true"]');
  if (!editable) {
    throw new Error("composer contenteditable is not mounted");
  }
  return editable;
}

/**
 * Moves the caret inside the rendered line. `setComposerText` always parks it at
 * the end, so mid-text trigger scopes need a real DOM selection that Lexical
 * reconciles back into its own editor state.
 */
async function placeComposerCursor(offset: number) {
  const editable = await findComposerEditable();
  const walker = document.createTreeWalker(editable, NodeFilter.SHOW_TEXT);
  let textNode = walker.nextNode();
  let localOffset = offset;
  while (textNode && localOffset > (textNode.textContent?.length ?? 0)) {
    localOffset -= textNode.textContent?.length ?? 0;
    textNode = walker.nextNode();
  }
  if (!textNode) {
    throw new Error("composer line has no rendered text node");
  }
  const cursorNode = textNode;
  await act(async () => {
    editable.focus();
    const selection = document.getSelection();
    const range = document.createRange();
    range.setStart(cursorNode, localOffset);
    range.collapse(true);
    selection?.removeAllRanges();
    selection?.addRange(range);
    document.dispatchEvent(new Event("selectionchange"));
  });
}

/** Mounts the real composer runtime, optionally scoping overlays to its desktop host. */
function renderComposer(
  overrides: Partial<ComponentProps<typeof SessionThread>>,
  transport?: SessionTransportState,
  overlayContainer: HTMLElement | null = null,
  runtimeStore: SessionPromptRuntimeStore | null = null
) {
  const queryClient = createQueryClient();
  return render(
    <OverlayContainerContext value={overlayContainer}>
      <StrictMode>
        <QueryClientProvider client={queryClient}>
          <SessionPromptRuntimeContext value={runtimeStore}>
            <SessionChatRuntimeProvider
              sessionId={primarySessionFixture.id}
              workspaceId={fixtureWorkspaceId()}
            >
              <ComposerAuiProbe />
              <SessionTranscriptThreadProvider
                messages={[]}
                status="success"
                error={null}
                retry={vi.fn()}
                transport={transport}
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
          </SessionPromptRuntimeContext>
        </QueryClientProvider>
      </StrictMode>
    </OverlayContainerContext>
  );
}

describe("SessionThread composer running semantics", () => {
  beforeEach(() => {
    composerAui = null;
    vi.stubGlobal("fetch", createFetchMock());
    vi.mocked(toast.error).mockClear();
    vi.mocked(toast.warning).mockClear();
    sessionStore.trigger.composerDraftDiscarded({ sessionId: primarySessionFixture.id });
    sessionStore.trigger.firstPromptSent({ sessionId: primarySessionFixture.id });
    takePendingTerminalQuote();
    clearSessionTerminalQuote(primarySessionFixture.id);
  });

  afterEach(async () => {
    takePendingTerminalQuote();
    vi.unstubAllGlobals();
    // The listener tier latches in module scope once a stream authorizes.
    resetGatewayStreamAuth();
    // The tree is still mounted here (RTL's own cleanup runs after this hook),
    // so every store reset the quote slot or composer subscribes to is a React
    // update. Discarding a non-empty draft also re-hydrates the Lexical editor,
    // which commits on a microtask — the async act drains it before the hook ends.
    await act(async () => {
      clearSessionTerminalQuote(primarySessionFixture.id);
      sessionStore.trigger.composerDraftDiscarded({ sessionId: primarySessionFixture.id });
      sessionStore.trigger.firstPromptSent({ sessionId: primarySessionFixture.id });
    });
  });

  // Invariant: decorating URL text never changes the authored draft, including
  // subsequent edits. Owner: composer; canonical suite: SessionThread composer.
  it("preserves editable URL text through composer formatting and edits", async () => {
    renderComposer({ isSessionRunning: false });
    await screen.findByTestId("composer-input");
    const text = "Read https://example.com/a_(b)?q=1#hash, then mailto:a@example.com";
    await setComposerText(text);
    const editable = await findComposerEditable();
    await waitFor(() => expect(editable).toHaveTextContent(text));
    expect(composerText()).toBe(text);
    const cursor = text.indexOf("_(b)");
    await placeComposerCursor(cursor);
    const editor = getNearestEditorFromDOMNode(editable);
    if (!editor) throw new Error("composer editor is not mounted");
    const readCursor = () =>
      editor.getEditorState().read(() => {
        const selection = $getSelection();
        return $isRangeSelection(selection) ? readCursorSnapshot(selection)?.cursor : null;
      });
    await act(async () => {
      editor.dispatchCommand(CONTROLLED_TEXT_INSERTION_COMMAND, " ");
    });
    const edited = `${text.slice(0, cursor)} ${text.slice(cursor)}`;
    await waitFor(() => expect(composerText()).toBe(edited));
    expect(readCursor()).toBe(cursor + 1);
    await act(async () => {
      editor.dispatchCommand(UNDO_COMMAND, undefined);
    });
    await waitFor(() => expect(composerText()).toBe(text));
    expect(readCursor()).toBe(cursor);
    await setComposerText(text.replace("https://", "broken://"));
    await waitFor(() => expect(editable).toHaveTextContent("broken://example.com"));
    expect(composerText()).toBe(text.replace("https://", "broken://"));

    // A URL typed one character at a time first matches as a short prefix
    // ("https://e"); the decoration must still cover the whole URL (#967).
    const linkSpans = () =>
      Array.from(
        editable.querySelectorAll<HTMLElement>('span[style*="--compozy-composer-link"]'),
        span => span.textContent
      );
    await setComposerText("see ");
    await placeComposerCursor(4);
    const typed = "see https://example.com/a/b now";
    for (const char of typed.slice(4)) {
      await act(async () => {
        editor.dispatchCommand(CONTROLLED_TEXT_INSERTION_COMMAND, char);
      });
    }
    await waitFor(() => expect(composerText()).toBe(typed));
    expect(linkSpans()).toEqual(["https://example.com/a/b"]);

    const lines = "first https://a.example/x\nthen mailto:b@example.com and https://c.example";
    await setComposerText(lines);
    await waitFor(() => expect(composerText()).toBe(lines));
    expect(linkSpans()).toEqual([
      "https://a.example/x",
      "mailto:b@example.com",
      "https://c.example",
    ]);
  });

  it("Should restore the active session draft after the composer remounts", async () => {
    const firstView = renderComposer({ isSessionRunning: false });

    await screen.findByTestId("composer-input");
    await setComposerText("keep this draft");
    expect(composerText()).toBe("keep this draft");
    await waitFor(() => {
      expect(sessionStore.getSnapshot().context.drafts[primarySessionFixture.id]).toBe(
        "keep this draft"
      );
    });

    firstView.unmount();
    renderComposer({ isSessionRunning: false });

    // Hydration lands in the editor on Lexical's own commit; wait for that
    // commit (the rendered text), not merely for the input to exist.
    await waitFor(() =>
      expect(screen.getByTestId("composer-input")).toHaveTextContent("keep this draft")
    );
    expect(composerText()).toBe("keep this draft");
  });

  it("Should persist an edit committed before saved draft hydration is observed", async () => {
    const sessionId = primarySessionFixture.id;
    sessionStore.trigger.composerDraftChanged({ sessionId, text: "saved draft" });
    function EditingComposer() {
      const aui = useAui();
      const { composerText: text } = useSessionComposerState(sessionId);
      useLayoutEffect(() => {
        aui.composer.setText("new edit during hydration");
      }, [aui]);
      return <output data-testid="hydrating-composer">{text}</output>;
    }
    render(
      <QueryClientProvider client={createQueryClient()}>
        <SessionChatRuntimeProvider sessionId={sessionId} workspaceId={fixtureWorkspaceId()}>
          <EditingComposer />
        </SessionChatRuntimeProvider>
      </QueryClientProvider>
    );
    await waitFor(() => {
      expect(screen.getByTestId("hydrating-composer")).toHaveTextContent(
        "new edit during hydration"
      );
      expect(sessionStore.getSnapshot().context.drafts[sessionId]).toBe(
        "new edit during hydration"
      );
    });
  });

  it("Should insert a standalone command token without submitting the prompt", async () => {
    const user = userEvent.setup();
    const onCommandCatalogOpen = vi.fn();
    renderComposer({
      onCommandCatalogOpen,
      commandCatalog: {
        standaloneSections: [
          {
            id: "built-in",
            label: "Built-in",
            commands: [{ id: "goal", token: "/goal", label: "Goal", lane: "builtin" as const }],
          },
          {
            id: "agent",
            label: "Agent",
            commands: [{ id: "review", token: "/review", label: "Review", lane: "agent" as const }],
          },
          {
            id: "skills",
            label: "Skills",
            commands: [
              {
                id: "frontend-qa",
                token: "/frontend-qa",
                label: "Frontend QA",
                lane: "skill" as const,
                scope: "workspace",
              },
            ],
          },
        ],
        inlineSkills: [
          {
            id: "frontend-qa",
            token: "/frontend-qa",
            label: "Frontend QA",
            lane: "skill" as const,
            scope: "workspace",
          },
        ],
      },
    });

    await screen.findByTestId("composer-input");
    expect(onCommandCatalogOpen).not.toHaveBeenCalled();
    await setComposerText("/");

    const menu = await screen.findByTestId("composer-command-menu");
    expect(onCommandCatalogOpen).toHaveBeenCalledTimes(1);
    // One flat list: category headers are decoration, every command is directly selectable.
    expect(
      within(menu)
        .getAllByTestId("composer-command-section")
        .map(section => section.textContent)
    ).toEqual(["Built-in", "Agent", "Skills"]);
    expect(within(menu).getAllByRole("option")).toHaveLength(3);
    await user.click(within(menu).getByRole("option", { name: /Frontend QA/i }));

    await waitFor(() => {
      expect(composerText()).toBe("/frontend-qa ");
      expect(sessionStore.getSnapshot().context.drafts[primarySessionFixture.id]).toBe(
        "/frontend-qa "
      );
    });
    // Selecting a command closes the menu; the token survives only as an inline chip.
    expect(screen.queryByTestId("composer-command-menu")).not.toBeInTheDocument();
    expect(screen.queryByText("Frontend QA")).not.toBeInTheDocument();
  });

  it("Should label a skill absorbed from another tool and leave native rows unlabeled", async () => {
    renderComposer({
      commandCatalog: {
        standaloneSections: [
          {
            id: "skills",
            label: "Skills",
            commands: [
              {
                id: "review-checklist",
                token: "/review-checklist",
                label: "review-checklist",
                lane: "skill" as const,
                scope: "workspace",
              },
              {
                id: "pdf-tools",
                token: "/pdf-tools",
                label: "pdf-tools",
                lane: "skill" as const,
                scope: "workspace_profile",
                origin: "claude",
              },
            ],
          },
        ],
        inlineSkills: [],
      },
    });

    await screen.findByTestId("composer-input");
    await setComposerText("/");

    const menu = await screen.findByTestId("composer-command-menu");
    const absorbed = within(menu).getByRole("option", { name: /pdf-tools/i });
    expect(
      within(absorbed).getByText("claude", { selector: "[data-slot='composer-command-origin']" })
    ).toBeVisible();
    // Origin is a fact about the source, never a replacement for the tier word.
    expect(absorbed).toHaveTextContent("Workspace profile");

    const native = within(menu).getByRole("option", { name: /review-checklist/i });
    expect(within(native).queryByTestId("composer-command-origin")).not.toBeInTheDocument();
    expect(native.querySelector("[data-slot='composer-command-origin']")).toBeNull();
  });

  it("Should remove the selected action token without deleting an earlier mention", async () => {
    const user = userEvent.setup();
    const onCommandAction = vi.fn((token: string) => token === "/worktree");
    renderComposer({
      onCommandAction,
      commandCatalog: {
        standaloneSections: [
          {
            id: "built-in",
            label: "Built-in",
            commands: [
              {
                id: "worktree",
                token: "/worktree",
                label: "Worktree",
                lane: "builtin" as const,
              },
            ],
          },
        ],
        inlineSkills: [],
      },
    });

    await screen.findByTestId("composer-input");
    await setComposerText("/ keep the existing /worktree mention");
    await placeComposerCursor(1);
    const menu = await screen.findByTestId("composer-command-menu");
    await user.click(within(menu).getByRole("option", { name: /Worktree/i }));

    expect(onCommandAction).toHaveBeenCalledWith("/worktree");
    await waitFor(() => expect(composerText()).toBe(" keep the existing /worktree mention"));
  });

  it("Should preserve hook order when the command menu closes and reopens", async () => {
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {});

    try {
      renderComposer({
        commandCatalog: {
          standaloneSections: [
            {
              id: "skills",
              label: "Skills",
              commands: [
                {
                  id: "frontend-qa",
                  token: "/frontend-qa",
                  label: "Frontend QA",
                  lane: "skill" as const,
                },
              ],
            },
          ],
          inlineSkills: [
            {
              id: "frontend-qa",
              token: "/frontend-qa",
              label: "Frontend QA",
              lane: "skill" as const,
            },
          ],
        },
      });

      const editable = await findComposerEditable();
      await setComposerText("/");
      expect(await screen.findByTestId("composer-command-menu")).toBeInTheDocument();

      await act(async () => {
        fireEvent.keyDown(editable, { key: "Escape" });
      });
      await waitFor(() => {
        expect(screen.queryByTestId("composer-command-menu")).not.toBeInTheDocument();
      });

      await setComposerText("");
      await setComposerText("/");
      expect(await screen.findByTestId("composer-command-menu")).toBeInTheDocument();

      const hookOrderErrors = consoleError.mock.calls
        .flatMap(args => args.map(String))
        .filter(message =>
          /change in the order of Hooks|Rendered (?:more|fewer) hooks than expected/i.test(message)
        );
      expect(hookOrderErrors).toEqual([]);
    } finally {
      consoleError.mockRestore();
    }
  });

  it("Should restrict an inline slash menu to skills", async () => {
    const user = userEvent.setup();
    renderComposer({
      commandCatalog: {
        standaloneSections: [
          {
            id: "built-in",
            label: "Built-in",
            commands: [{ id: "goal", token: "/goal", label: "Goal", lane: "builtin" as const }],
          },
          {
            id: "skills",
            label: "Skills",
            commands: [
              {
                id: "frontend-qa",
                token: "/frontend-qa",
                label: "Frontend QA",
                lane: "skill" as const,
              },
            ],
          },
        ],
        inlineSkills: [
          {
            id: "frontend-qa",
            token: "/frontend-qa",
            label: "Frontend QA",
            lane: "skill" as const,
          },
        ],
      },
    });

    await screen.findByTestId("composer-input");
    // A non-blank prefix before the trigger makes the scope inline — skills only,
    // no standalone controls — and the insertion keeps the trailing prompt text.
    await setComposerText("Review / then");
    await placeComposerCursor("Review /".length);

    const menu = await screen.findByTestId("composer-command-menu");
    expect(
      within(menu)
        .getAllByTestId("composer-command-section")
        .map(section => section.textContent)
    ).toEqual(["Skills"]);
    expect(within(menu).queryByRole("option", { name: /Goal/i })).not.toBeInTheDocument();
    await user.click(within(menu).getByRole("option", { name: /Frontend QA/i }));

    await waitFor(() => {
      expect(composerText()).toBe("Review /frontend-qa then");
    });
  });

  it("Should expose standalone controls after leading whitespace with trailing text", async () => {
    renderComposer({
      commandCatalog: {
        standaloneSections: [
          {
            id: "built-in",
            label: "Built-in",
            commands: [{ id: "goal", token: "/goal", label: "Goal", lane: "builtin" as const }],
          },
          {
            id: "agent",
            label: "Agent",
            commands: [{ id: "review", token: "/review", label: "Review", lane: "agent" as const }],
          },
        ],
        inlineSkills: [
          {
            id: "frontend-qa",
            token: "/frontend-qa",
            label: "Frontend QA",
            lane: "skill" as const,
          },
        ],
      },
    });

    await screen.findByTestId("composer-input");
    // Only whitespace precedes the trigger, so the scope stays standalone — the
    // prompt text that trails the caret must not pull it back to inline.
    await setComposerText("  / remaining prompt");
    await placeComposerCursor(3);

    const menu = await screen.findByTestId("composer-command-menu");
    expect(
      within(menu)
        .getAllByTestId("composer-command-section")
        .map(section => section.textContent)
    ).toEqual(["Built-in", "Agent"]);
    expect(within(menu).getByRole("option", { name: /Goal/i })).toBeInTheDocument();
    expect(within(menu).getByRole("option", { name: /Review/i })).toBeInTheDocument();
  });

  it("Should preserve live composer text when a stale persisted draft arrives", async () => {
    renderComposer({ isSessionRunning: false });

    await screen.findByTestId("composer-input");
    await setComposerText("Live draft");

    act(() => {
      sessionStore.trigger.composerDraftChanged({
        sessionId: primarySessionFixture.id,
        text: "Stale persisted draft",
      });
    });

    expect(composerText()).toBe("Live draft");
  });

  it("Should steer the draft on Enter while running (daemon default) and suppress the runtime send", async () => {
    const onQueuePrompt = vi.fn(() => Promise.resolve());
    const onSteerPrompt = vi.fn(() => Promise.resolve());
    renderComposer({ isSessionRunning: true, allowBusyInput: true, onQueuePrompt, onSteerPrompt });

    const editable = await findComposerEditable();
    await setComposerText("steer this follow-up");
    // While running, assistant-ui's own thread is idle, so a plain Enter would submit
    // to the runtime; our interception must perform the follow-up default instead.
    await act(async () => {
      fireEvent.keyDown(editable, { key: "Enter" });
    });

    await waitFor(() => {
      expect(onSteerPrompt).toHaveBeenCalledWith({
        message: "steer this follow-up",
        attachments: [],
      });
    });
    expect(onSteerPrompt).toHaveBeenCalledTimes(1);
    expect(onQueuePrompt).not.toHaveBeenCalled();
    await waitFor(() => {
      expect(composerText()).toBe("");
    });
    // The runtime send never fired, so no user message entered the thread.
    expect(screen.queryByText("steer this follow-up")).not.toBeInTheDocument();
  });

  it("Should queue on Enter when the daemon default is queue and steer with the modifier (E2E-012 unit shadow)", async () => {
    const onQueuePrompt = vi.fn(() => Promise.resolve());
    const onSteerPrompt = vi.fn(() => Promise.resolve());
    renderComposer({
      busyInputDefaultMode: "queue",
      isSessionRunning: true,
      allowBusyInput: true,
      onQueuePrompt,
      onSteerPrompt,
    });

    const editable = await findComposerEditable();
    expect(screen.getByTestId("composer-enter-hint")).toHaveAttribute("data-enter", "queue");
    expect(screen.getByTestId("composer-enter-hint")).toHaveAttribute("data-modifier", "steer");
    await setComposerText("park this one");
    await act(async () => {
      fireEvent.keyDown(editable, { key: "Enter" });
    });
    await waitFor(() => expect(onQueuePrompt).toHaveBeenCalledOnce());
    expect(onSteerPrompt).not.toHaveBeenCalled();

    await setComposerText("redirect this one");
    await act(async () => {
      fireEvent.keyDown(editable, { key: "Enter", metaKey: true });
    });
    await waitFor(() =>
      expect(onSteerPrompt).toHaveBeenCalledWith({ message: "redirect this one", attachments: [] })
    );
    // The modifier is one-shot: the hint still reads the configured default.
    expect(screen.getByTestId("composer-enter-hint")).toHaveAttribute("data-enter", "queue");
    expect(onQueuePrompt).toHaveBeenCalledOnce();
  });

  it("UT-086: Should do nothing on Enter with an empty draft during a turn", async () => {
    const onQueuePrompt = vi.fn(() => Promise.resolve());
    const onSteerPrompt = vi.fn(() => Promise.resolve());
    renderComposer({ isSessionRunning: true, allowBusyInput: true, onQueuePrompt, onSteerPrompt });

    const editable = await findComposerEditable();
    await act(async () => {
      fireEvent.keyDown(editable, { key: "Enter" });
      fireEvent.keyDown(editable, { key: "Enter", metaKey: true });
    });

    expect(onSteerPrompt).not.toHaveBeenCalled();
    expect(onQueuePrompt).not.toHaveBeenCalled();
    expect(screen.queryByTestId("composer-feedback-note")).not.toBeInTheDocument();
    expect(toast.error).not.toHaveBeenCalled();
  });

  it("Should answer an accepted busy send inline with its disposition", async () => {
    const user = userEvent.setup();
    const onSteerPrompt = vi.fn(() =>
      Promise.resolve({
        disposition: "steering" as const,
        entryId: "inp_4d9",
        idempotencyKey: "idk_1f77",
        messageId: "msg_01k4",
        queuePosition: null,
        replayed: false,
        steerDelivery: "pending_injection" as const,
        turnId: "t_9f2",
      })
    );
    const onQueuePrompt = vi.fn(() =>
      Promise.resolve({
        disposition: "queued" as const,
        entryId: "inp_4d8",
        idempotencyKey: "idk_9b02",
        messageId: "msg_01k3",
        queuePosition: 2,
        replayed: false,
        steerDelivery: null,
        turnId: "t_9f2",
      })
    );
    renderComposer({ isSessionRunning: true, allowBusyInput: true, onQueuePrompt, onSteerPrompt });

    await screen.findByTestId("composer-input");
    await setComposerText("only the lifecycle tests");
    await user.click(screen.getByTestId("composer-steer-button"));

    const steerNote = await screen.findByTestId("composer-feedback-note");
    expect(steerNote).toHaveAttribute("data-kind", "disposition");
    expect(steerNote).toHaveTextContent(
      "Steering — the agent sees it when the current tool finishes"
    );
    // The daemon's delivery word is diagnostic: an attribute, never on screen.
    expect(steerNote).toHaveAttribute("data-detail", "pending_injection");
    expect(steerNote).not.toHaveTextContent("pending_injection");
    await waitFor(() => expect(composerText()).toBe(""));

    await setComposerText("ship it with tests");
    // Typing again retires the previous note before the next send answers.
    expect(screen.queryByTestId("composer-feedback-note")).not.toBeInTheDocument();
    await user.click(screen.getByTestId("composer-queue-button"));

    await waitFor(() =>
      expect(screen.getByTestId("composer-feedback-note")).toHaveTextContent(
        "Queued #2 — runs after the current turn"
      )
    );
    expect(screen.getByTestId("composer-feedback-note")).toHaveAttribute("data-detail", "inp_4d8");
    expect(screen.getByTestId("composer-feedback-note")).not.toHaveTextContent("inp_4d8");
  });

  // A replayed identity keeps its Later reading: the session is still parked, so
  // the generic "nothing was sent twice" replay line must not hide that.
  it("Should keep a replayed Later send naming the parked session", async () => {
    const user = userEvent.setup();
    const onQueuePrompt = vi.fn(() =>
      Promise.resolve({
        disposition: "later" as const,
        entryId: "inp_7a1",
        idempotencyKey: "idk_7a1",
        messageId: "msg_01k5",
        queuePosition: null,
        replayed: true,
        steerDelivery: null,
        turnId: null,
      })
    );
    renderComposer({ isSessionRunning: true, allowBusyInput: true, onQueuePrompt });

    await screen.findByTestId("composer-input");
    await setComposerText("pick this up when it returns");
    await user.click(screen.getByTestId("composer-queue-button"));

    const note = await screen.findByTestId("composer-feedback-note");
    expect(note).toHaveAttribute("data-code", "later");
    expect(note).toHaveTextContent("Queued — this session is in Later");
    expect(note).not.toHaveTextContent("nothing was sent twice");
  });

  it("Should admit only one busy-input submission while a queue request is pending", async () => {
    let resolveQueue: (() => void) | undefined;
    const queuePending = new Promise<void>(resolve => {
      resolveQueue = resolve;
    });
    const onQueuePrompt = vi.fn(() => queuePending);
    renderComposer({ isSessionRunning: true, allowBusyInput: true, onQueuePrompt });

    await screen.findByTestId("composer-input");
    await setComposerText("queue this only once");
    const queueButton = screen.getByTestId("composer-queue-button");

    act(() => {
      fireEvent.click(queueButton);
      fireEvent.click(queueButton);
    });

    await waitFor(() => expect(onQueuePrompt).toHaveBeenCalledOnce());
    resolveQueue?.();
    await waitFor(() => expect(composerText()).toBe(""));
  });

  it("Should consume only the submitted text and files when the operator keeps working during a busy send", async () => {
    const user = userEvent.setup();
    let resolveQueue: ((outcome: SessionSendOutcome) => void) | undefined;
    const onQueuePrompt = vi.fn(
      (_draft: SessionBusyInputDraft) =>
        new Promise<SessionSendOutcome>(resolve => {
          resolveQueue = resolve;
        })
    );
    vi.stubGlobal("fetch", createFetchMock({ attachmentUpload: { name: "sent.png" } }));
    renderComposer({
      isSessionRunning: true,
      allowBusyInput: true,
      onQueuePrompt,
      promptImageCapability: "supported",
    });

    await screen.findByTestId("composer-input");
    await setComposerText("ship it with tests");
    await act(async () => {
      await requireComposerAui().composer.addAttachment(pngFile("sent.png"));
    });
    await waitFor(() =>
      expect(screen.getByTestId("composer-attachment-tile")).toHaveAttribute("data-state", "ready")
    );
    await user.click(screen.getByTestId("composer-queue-button"));
    await waitFor(() => expect(onQueuePrompt).toHaveBeenCalledOnce());
    const sentAttachmentId = onQueuePrompt.mock.calls[0]?.[0].attachments[0]?.id;
    expect(sentAttachmentId).toMatch(/^att_/);

    // The editor stays writable while the daemon answers: keep typing and add a file.
    await setComposerText("ship it with tests and update the changelog");
    await act(async () => {
      await requireComposerAui().composer.addAttachment(pngFile("later.png"));
    });
    await waitFor(() => expect(screen.getAllByTestId("composer-attachment-tile")).toHaveLength(2));

    await act(async () => {
      resolveQueue?.({
        disposition: "queued",
        entryId: "inp_4d8",
        idempotencyKey: "idk_9b02",
        messageId: "msg_01k3",
        queuePosition: 1,
        replayed: false,
        steerDelivery: null,
        turnId: "t_9f2",
      });
    });

    // Only what was sent leaves the field; the newer text and file stay, and the
    // remainder is exact — the separator the operator typed is theirs too.
    await waitFor(() => expect(composerText()).toBe(" and update the changelog"));
    await waitFor(() => expect(screen.getAllByTestId("composer-attachment-tile")).toHaveLength(1));
    expect(screen.getByTestId("composer-attachment-tile")).toHaveTextContent("later.png");
    expect(sessionStore.getSnapshot().context.drafts[primarySessionFixture.id]).toBe(
      " and update the changelog"
    );
    // The disposition still answers the send that just landed.
    expect(screen.getByTestId("composer-feedback-note")).toHaveTextContent("Queued #1");
    expect(onQueuePrompt).toHaveBeenCalledOnce();
  });

  it("Should keep newly typed indented text exactly after an attachment-only busy send is accepted", async () => {
    const user = userEvent.setup();
    let resolveQueue: (() => void) | undefined;
    const onQueuePrompt = vi.fn(
      (_draft: SessionBusyInputDraft) =>
        new Promise<void>(resolve => {
          resolveQueue = resolve;
        })
    );
    vi.stubGlobal("fetch", createFetchMock({ attachmentUpload: { name: "only-file.png" } }));
    renderComposer({
      isSessionRunning: true,
      allowBusyInput: true,
      onQueuePrompt,
      promptImageCapability: "supported",
    });

    await screen.findByTestId("composer-input");
    await act(async () => {
      await requireComposerAui().composer.addAttachment(pngFile("only-file.png"));
    });
    await waitFor(() =>
      expect(screen.getByTestId("composer-attachment-tile")).toHaveAttribute("data-state", "ready")
    );
    expect(composerText()).toBe("");
    await user.click(screen.getByTestId("composer-queue-button"));
    await waitFor(() => expect(onQueuePrompt).toHaveBeenCalledOnce());
    expect(onQueuePrompt.mock.calls[0]?.[0]).toMatchObject({ message: "" });

    // An empty sent prefix means everything typed meanwhile is the operator's,
    // including the indentation they started with.
    const indented = "    - keep the leading indent";
    await setComposerText(indented);
    await act(async () => {
      resolveQueue?.();
    });

    await waitFor(() =>
      expect(screen.queryByTestId("composer-attachment-tile")).not.toBeInTheDocument()
    );
    expect(composerText()).toBe(indented);
    await waitFor(() =>
      expect(sessionStore.getSnapshot().context.drafts[primarySessionFixture.id]).toBe(indented)
    );
    expect(onQueuePrompt).toHaveBeenCalledOnce();
  });

  it("Should leave a rewritten draft untouched when the busy send it replaced is accepted", async () => {
    const user = userEvent.setup();
    let resolveSteer: (() => void) | undefined;
    const onSteerPrompt = vi.fn(
      () =>
        new Promise<void>(resolve => {
          resolveSteer = resolve;
        })
    );
    renderComposer({ isSessionRunning: true, allowBusyInput: true, onSteerPrompt });

    await screen.findByTestId("composer-input");
    await setComposerText("only the lifecycle tests");
    await user.click(screen.getByTestId("composer-steer-button"));
    await waitFor(() => expect(onSteerPrompt).toHaveBeenCalledOnce());

    await setComposerText("actually, the store package too");
    await act(async () => {
      resolveSteer?.();
    });

    await waitFor(() => expect(composerText()).toBe("actually, the store package too"));
    expect(sessionStore.getSnapshot().context.drafts[primarySessionFixture.id]).toBe(
      "actually, the store package too"
    );
  });

  it("Should queue ready attachment refs while also offering steer", async () => {
    const user = userEvent.setup();
    const onQueuePrompt = vi.fn(() => Promise.resolve());
    const onSteerPrompt = vi.fn(() => Promise.resolve());
    const fetch = createFetchMock({ attachmentUpload: { name: "queued.png" } });
    vi.stubGlobal("fetch", fetch);
    renderComposer({
      isSessionRunning: true,
      allowBusyInput: true,
      onQueuePrompt,
      onSteerPrompt,
      promptImageCapability: "supported",
    });

    await screen.findByTestId("composer-input");
    await setComposerText("review this image");
    await act(async () => {
      await requireComposerAui().composer.addAttachment(pngFile("queued.png"));
    });
    await waitFor(() =>
      expect(screen.getByTestId("composer-attachment-tile")).toHaveAttribute("data-state", "ready")
    );
    expect(screen.getByTestId("composer-steer-button")).toBeEnabled();
    await user.click(screen.getByTestId("composer-queue-button"));

    await waitFor(() => expect(onQueuePrompt).toHaveBeenCalledOnce());
    expect(onQueuePrompt).toHaveBeenCalledWith({
      message: "review this image",
      attachments: [
        expect.objectContaining({
          id: expect.stringMatching(/^att_[a-f0-9]{64}$/),
          name: "queued.png",
          mime_type: "image/png",
        }),
      ],
    });
    expect(onSteerPrompt).not.toHaveBeenCalled();
    await waitFor(() => {
      expect(screen.queryByTestId("composer-attachment-tile")).not.toBeInTheDocument();
    });
    expect(
      fetch.mock.calls.some(([input]) =>
        /\/attachments\/att_[a-f0-9]{64}$/.test(getPathname(input))
      )
    ).toBe(false);
  });

  // Invariant (task_03/06 truth, US-007): a send that failed without the daemon's answer
  // is not "Not sent" — the daemon may have accepted it before the answer was lost. The
  // composer says "Not confirmed" (the strip row keeps the identity with Retry) and the
  // field is consumed like an accepted send, so nothing is typed twice; a proven refusal
  // (client gate, daemon 4xx) still reads "Not sent" and restores the draft.
  // Owner: composer busy-input submission/feedback. Canonical suite: this file.
  it("Should state an unacknowledged queue send as not confirmed and hand its text to the strip", async () => {
    const user = userEvent.setup();
    const onQueuePrompt = vi.fn(() => Promise.reject(new TypeError("Failed to fetch")));
    renderComposer({ isSessionRunning: true, allowBusyInput: true, onQueuePrompt });

    await screen.findByTestId("composer-input");
    await setComposerText("queue this follow-up");
    await user.click(screen.getByTestId("composer-queue-button"));

    const note = await screen.findByTestId("composer-feedback-note");
    expect(note).toHaveAttribute("data-kind", "unconfirmed");
    expect(note).toHaveAttribute("data-code", "unconfirmed");
    expect(note).toHaveTextContent(
      "Not confirmed — Failed to fetch. Retry replays the same message; nothing is sent twice."
    );
    expect(note).not.toHaveTextContent("Not sent");
    expect(screen.queryByTestId("composer-feedback-suffix")).not.toBeInTheDocument();
    expect(toast.error).not.toHaveBeenCalled();
    expect(composerText()).toBe("");
  });

  it("Should read a synchronous handler failure as unconfirmed too, never as proof of non-delivery", async () => {
    const user = userEvent.setup();
    const onQueuePrompt = vi.fn(() => {
      throw new Error("queue failed synchronously");
    });
    renderComposer({ isSessionRunning: true, allowBusyInput: true, onQueuePrompt });

    await screen.findByTestId("composer-input");
    await setComposerText("queue this follow-up");
    await user.click(screen.getByTestId("composer-queue-button"));

    await waitFor(() => {
      expect(screen.getByTestId("composer-feedback-note")).toHaveTextContent(
        "Not confirmed — queue failed synchronously."
      );
    });
    expect(screen.getByTestId("composer-feedback-note")).toHaveAttribute(
      "data-kind",
      "unconfirmed"
    );
  });

  it("Should keep a daemon 4xx answer as a proven refusal that restores the draft", async () => {
    const user = userEvent.setup();
    const onQueuePrompt = vi.fn(() =>
      Promise.reject(new SessionApiError("no such session", 404, primarySessionFixture.id))
    );
    renderComposer({ isSessionRunning: true, allowBusyInput: true, onQueuePrompt });

    await screen.findByTestId("composer-input");
    await setComposerText("queue this follow-up");
    await user.click(screen.getByTestId("composer-queue-button"));

    const note = await screen.findByTestId("composer-feedback-note");
    expect(note).toHaveAttribute("data-kind", "refusal");
    expect(note).toHaveTextContent("Not sent — no such session");
    expect(composerText()).toBe("queue this follow-up");
  });

  it("UT-100: Should state the reason and keep text and attachments when a busy send is refused", async () => {
    const user = userEvent.setup();
    const onQueuePrompt = vi.fn(() =>
      Promise.reject(
        new SessionBusyInputRefusalError({ code: "active_turn_mismatch", currentTurnId: "t_9f3" })
      )
    );
    const fetch = createFetchMock({ attachmentUpload: { name: "restored.png" } });
    vi.stubGlobal("fetch", fetch);
    renderComposer({
      isSessionRunning: true,
      allowBusyInput: true,
      onQueuePrompt,
      promptImageCapability: "supported",
    });

    await screen.findByTestId("composer-input");
    await setComposerText("Only touch the lifecycle tests");
    await act(async () => {
      await requireComposerAui().composer.addAttachment(pngFile("restored.png"));
    });
    await waitFor(() =>
      expect(screen.getByTestId("composer-attachment-tile")).toHaveAttribute("data-state", "ready")
    );
    await user.click(screen.getByTestId("composer-queue-button"));

    const note = await screen.findByTestId("composer-feedback-note");
    expect(note).toHaveAttribute("data-kind", "refusal");
    expect(note).toHaveAttribute("data-code", "active_turn_mismatch");
    expect(note).toHaveTextContent(
      "Not sent — the turn changed before this went out. Your draft is back."
    );
    expect(note).toHaveAttribute("data-detail", "active_turn_mismatch");
    expect(note).not.toHaveTextContent("active_turn_mismatch");
    expect(composerText()).toBe("Only touch the lifecycle tests");
    expect(screen.getByTestId("composer-attachment-tile")).toBeInTheDocument();
    expect(toast.error).not.toHaveBeenCalled();
  });

  it("Should steer an image on Enter and clear the acknowledged draft", async () => {
    const onSteerPrompt = vi.fn(() => Promise.resolve());
    const onQueuePrompt = vi.fn(() => Promise.resolve());
    const attachmentIdentity = nextAttachmentUploadIdentity();
    vi.stubGlobal(
      "fetch",
      createFetchMock({ attachmentUpload: { ...attachmentIdentity, name: "files.png" } })
    );
    renderComposer({
      isSessionRunning: true,
      allowBusyInput: true,
      onQueuePrompt,
      onSteerPrompt,
      promptImageCapability: "supported",
    });

    const editable = await findComposerEditable();
    await setComposerText("review this image");
    await act(async () => {
      await requireComposerAui().composer.addAttachment(pngFile("files.png"));
    });
    await waitFor(() =>
      expect(screen.getByTestId("composer-attachment-tile")).toHaveAttribute("data-state", "ready")
    );
    expect(screen.getByTestId("composer-steer-button")).toBeEnabled();
    await act(async () => {
      fireEvent.keyDown(editable, { key: "Enter" });
    });

    await waitFor(() =>
      expect(onSteerPrompt).toHaveBeenCalledWith({
        message: "review this image",
        attachments: [expect.objectContaining({ id: attachmentIdentity.id, name: "files.png" })],
      })
    );
    expect(onQueuePrompt).not.toHaveBeenCalled();
    await waitFor(() => expect(composerText()).toBe(""));
    expect(screen.queryByTestId("composer-attachment-tile")).not.toBeInTheDocument();
    expect(toast.error).not.toHaveBeenCalled();
  });

  it("Should steer the current draft while running and clear it after success", async () => {
    const user = userEvent.setup();
    const onSteerPrompt = vi.fn(() => Promise.resolve());
    renderComposer({ isSessionRunning: true, allowBusyInput: true, onSteerPrompt });

    await screen.findByTestId("composer-input");
    await setComposerText("steer with this constraint");
    await user.click(screen.getByTestId("composer-steer-button"));

    await waitFor(() => {
      expect(onSteerPrompt).toHaveBeenCalledWith({
        message: "steer with this constraint",
        attachments: [],
      });
      expect(composerText()).toBe("");
    });
  });

  it("Should queue with Cmd/Ctrl+Enter as the one-shot opposite of the steer default", async () => {
    const onQueuePrompt = vi.fn(() => Promise.resolve());
    const onSteerPrompt = vi.fn(() => Promise.resolve());
    renderComposer({ isSessionRunning: true, allowBusyInput: true, onQueuePrompt, onSteerPrompt });

    const editable = await findComposerEditable();
    // The contenteditable itself carries the accessible name (screen readers
    // and Playwright role queries target it, not the wrapper div).
    await waitFor(() => {
      expect(editable).toHaveAttribute("aria-label", "Session prompt");
    });
    await setComposerText("park this for later");
    await act(async () => {
      fireEvent.keyDown(editable, { key: "Enter", ctrlKey: true });
    });

    await waitFor(() => {
      expect(onQueuePrompt).toHaveBeenCalledWith({
        message: "park this for later",
        attachments: [],
      });
      expect(composerText()).toBe("");
    });
    expect(onSteerPrompt).not.toHaveBeenCalled();
  });

  it("Should keep steer and interrupt available before the active turn id arrives", async () => {
    // The daemon resolves the live turn at admission when no fence is sent
    // (invariant 6); the composer no longer holds the verbs hostage to the poll.
    renderComposer({
      isSessionRunning: true,
      onInterruptPrompt: vi.fn(),
      onQueuePrompt: vi.fn(),
      onSteerPrompt: vi.fn(),
    });

    await screen.findByTestId("composer-input");
    await setComposerText("the daemon resolves the fence");

    expect(screen.getByTestId("composer-queue-button")).toBeEnabled();
    expect(screen.getByTestId("composer-steer-button")).toBeEnabled();
    expect(screen.getByTestId("composer-interrupt-button")).toBeEnabled();
  });

  it("Should silently preserve the draft when the queue owner is replaced", async () => {
    const user = userEvent.setup();
    const onQueuePrompt = vi.fn(() =>
      Promise.reject(new DOMException("Busy input request was canceled", "AbortError"))
    );
    renderComposer({ isSessionRunning: true, allowBusyInput: true, onQueuePrompt });

    await screen.findByTestId("composer-input");
    await setComposerText("queue this follow-up");
    await user.click(screen.getByTestId("composer-queue-button"));

    await waitFor(() => expect(onQueuePrompt).toHaveBeenCalledOnce());
    expect(toast.error).not.toHaveBeenCalled();
    expect(composerText()).toBe("queue this follow-up");
  });

  // Invariant (US-009.AC-1/EC-1, ADR-004): while a stop is landing the primary
  // control is a guarded "Stopping…" pill that takes no activation; Steer and
  // Interrupt are absent, Queue stays, and Enter queues. Owning layer: composer
  // action row + controller. Canonical suite: this file.
  it("Should render the guarded Stopping… pill, keep only Queue, and queue on Enter while a stop lands", async () => {
    const user = userEvent.setup();
    const onCancelPrompt = vi.fn();
    const onQueuePrompt = vi.fn(() => Promise.resolve());
    const onSteerPrompt = vi.fn(() => Promise.resolve());
    const onInterruptPrompt = vi.fn(() => Promise.resolve());
    renderComposer({
      allowBusyInput: true,
      isSessionRunning: true,
      onCancelPrompt,
      onInterruptPrompt,
      onQueuePrompt,
      onSteerPrompt,
      stopPhase: "stopping",
    });

    const editable = await findComposerEditable();
    const pill = screen.getByTestId("composer-stop-button");
    expect(pill).toHaveAttribute("data-state", "stopping");
    expect(pill).toHaveAttribute("aria-disabled", "true");
    expect(pill).toHaveTextContent("Stopping…");
    await user.click(pill);
    await user.dblClick(pill);
    expect(onCancelPrompt).not.toHaveBeenCalled();

    expect(screen.queryByTestId("composer-steer-button")).not.toBeInTheDocument();
    expect(screen.queryByTestId("composer-interrupt-button")).not.toBeInTheDocument();
    expect(screen.getByTestId("composer-queue-button")).toBeInTheDocument();
    const hint = screen.getByTestId("composer-enter-hint");
    expect(hint).toHaveAttribute("data-enter", "queue");
    expect(hint).not.toHaveAttribute("data-modifier");

    // The field stays editable: the draft is not at risk, and both Enter variants queue it.
    expect(editable).not.toHaveAttribute("inert");
    await setComposerText("after the stop, do this");
    await act(async () => {
      fireEvent.keyDown(editable, { key: "Enter", metaKey: true });
    });
    await waitFor(() =>
      expect(onQueuePrompt).toHaveBeenCalledWith({
        message: "after the stop, do this",
        attachments: [],
      })
    );
    expect(onSteerPrompt).not.toHaveBeenCalled();
    expect(onInterruptPrompt).not.toHaveBeenCalled();
  });

  it("Should issue one cancel for a double-click on Stop", async () => {
    const user = userEvent.setup();
    const onCancelPrompt = vi.fn();
    renderComposer({
      allowBusyInput: true,
      isSessionRunning: true,
      onCancelPrompt,
      onQueuePrompt: vi.fn(),
    });

    const stop = await screen.findByTestId("composer-stop-button");
    expect(stop).toHaveAttribute("data-state", "stop");
    await user.dblClick(stop);
    expect(onCancelPrompt).toHaveBeenCalledOnce();
  });

  it("Should show the accent Send disc while idle and the danger Stop disc while running", async () => {
    const { rerender } = renderComposerRerenderable({ isSessionRunning: false });

    expect(await screen.findByTestId("composer-send-button")).toBeInTheDocument();
    expect(screen.queryByTestId("composer-stop-button")).not.toBeInTheDocument();
    // Idle: the screen-reader ⏎ hint reads "send".
    expect(screen.getByTestId("composer-enter-hint")).toHaveTextContent(/send/);
    expect(screen.getByTestId("composer-enter-hint")).toHaveClass("sr-only");

    rerender({ isSessionRunning: true, allowBusyInput: true, onQueuePrompt: vi.fn() });

    const stop = await screen.findByTestId("composer-stop-button");
    expect(stop).toHaveAttribute("aria-label", "Stop generation");
    expect(screen.queryByTestId("composer-send-button")).not.toBeInTheDocument();
    // Busy: Enter performs the daemon default (steer) and the modifier the opposite.
    const hint = screen.getByTestId("composer-enter-hint");
    expect(hint).toHaveAttribute("data-enter", "steer");
    expect(hint).toHaveAttribute("data-modifier", "queue");
    expect(hint).toHaveTextContent(/steer/);
    expect(hint).toHaveTextContent(/queue/);
    expect(hint).not.toHaveTextContent(/send/);
    // The busy row shows no shortcut badges: the hint stays for assistive tech only (#826).
    expect(hint).toHaveClass("sr-only");
  });

  // Invariant: sending while disconnected fails now and keeps the draft (US-018.AC-3):
  // the press answers with one info note, nothing is consumed, no request leaves.
  // Owning layer: composer busy actions + send button. Canonical suite: this file.
  it("Should refuse a send while disconnected, keep the draft, and say so (US-018.AC-3)", async () => {
    const user = userEvent.setup();
    const onSteerPrompt = vi.fn(() => Promise.resolve());
    const onQueuePrompt = vi.fn(() => Promise.resolve());
    const disconnected: SessionTransportState = {
      ...SESSION_TRANSPORT_LIVE,
      degradedAt: 1_000,
      lastLiveAt: 500,
      phase: "waiting-reconnect",
      reconnectAttempt: 4,
      retry: vi.fn(),
    };
    renderComposer({ isSessionRunning: false, onQueuePrompt, onSteerPrompt }, disconnected);

    await screen.findByTestId("composer-input");
    await setComposerText("Only touch the lifecycle tests");
    const send = screen.getByTestId("composer-send-button");
    expect(send).toHaveAttribute("data-transport", "disconnected");
    expect(send).toBeEnabled();
    await user.click(send);

    const note = await screen.findByTestId("composer-feedback-note");
    expect(note).toHaveAttribute("data-code", "disconnected");
    expect(note).toHaveTextContent("Not sent");
    expect(note).toHaveTextContent("you're disconnected right now");
    expect(screen.queryByTestId("composer-feedback-suffix")).not.toBeInTheDocument();
    expect(composerText()).toBe("Only touch the lifecycle tests");
    expect(onSteerPrompt).not.toHaveBeenCalled();
    expect(onQueuePrompt).not.toHaveBeenCalled();

    // Enter answers the same way; a second press repeats the note, never a silent no-op.
    const editable = await findComposerEditable();
    await act(async () => {
      fireEvent.keyDown(editable, { key: "Enter" });
    });
    expect(await screen.findByTestId("composer-feedback-note")).toHaveAttribute(
      "data-code",
      "disconnected"
    );
    expect(composerText()).toBe("Only touch the lifecycle tests");
    expect(onSteerPrompt).not.toHaveBeenCalled();
  });

  it("Should render queued rows with steer, edit, and remove wired to real entries", async () => {
    const user = userEvent.setup();
    const onSteerQueuedPrompt = vi.fn();
    const onRemoveQueuedPrompt = vi.fn();
    const onReplaceQueuedPrompt = vi.fn().mockResolvedValue(undefined);
    renderComposer({
      isSessionRunning: true,
      allowBusyInput: true,
      onQueuePrompt: vi.fn(() => Promise.resolve()),
      onSteerQueuedPrompt,
      onRemoveQueuedPrompt,
      onReplaceQueuedPrompt,
      queuedPrompts: [
        { id: "inq-1", owner: null, position: 1, text: "Add a regression test." },
        { id: "inq-2", owner: null, position: 2, text: "Then update the docs." },
      ],
    });

    const rows = await screen.findAllByTestId("composer-queued-prompt-row");
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveTextContent("Add a regression test.");

    await user.click(within(rows[0]).getByTestId("composer-queued-steer"));
    expect(onSteerQueuedPrompt).toHaveBeenCalledWith({
      id: "inq-1",
      owner: null,
      position: 1,
      text: "Add a regression test.",
    });

    await user.click(within(rows[1]).getByTestId("composer-queued-remove"));
    expect(onRemoveQueuedPrompt).toHaveBeenCalledWith("inq-2");

    // Edit happens in the row: the preview becomes a field, the durable entry
    // stays until one atomic replacement is acknowledged, the composer is untouched.
    await user.click(within(rows[0]).getByTestId("composer-queued-edit"));
    const editor = await screen.findByTestId("composer-queued-editor");
    expect(editor).toHaveValue("Add a regression test.");
    expect(screen.getByTestId("composer-queued-state")).toHaveTextContent("Editing");
    // While one row is being edited the other rows offer no verbs.
    expect(screen.queryByTestId("composer-queued-steer")).not.toBeInTheDocument();
    expect(composerText()).toBe("");
    await user.clear(editor);
    await user.type(editor, "Add a regression test — and run the e2e lane too");
    await user.click(screen.getByTestId("composer-queued-edit-save"));
    await waitFor(() =>
      expect(onReplaceQueuedPrompt).toHaveBeenCalledWith(
        { id: "inq-1", owner: null, position: 1, text: "Add a regression test." },
        "Add a regression test — and run the e2e lane too"
      )
    );
    expect(onRemoveQueuedPrompt).not.toHaveBeenCalledWith("inq-1");
    expect(composerText()).toBe("");
    await waitFor(() =>
      expect(screen.queryByTestId("composer-queued-editor")).not.toBeInTheDocument()
    );
  });

  it("Should render positions, owner attribution, and no verbs on another actor's row", async () => {
    const onClearQueue = vi
      .fn()
      .mockResolvedValue({ cleared_count: 2, inputs: [], queue_generation: 2 });
    renderComposer({
      isSessionRunning: true,
      allowBusyInput: true,
      onQueuePrompt: vi.fn(() => Promise.resolve()),
      onSteerQueuedPrompt: vi.fn(),
      onRemoveQueuedPrompt: vi.fn(),
      onReplaceQueuedPrompt: vi.fn().mockResolvedValue(undefined),
      onClearQueue,
      queuedPrompts: [
        { id: "inq-1", owner: null, position: 1, status: "queued", text: "# Ship it\nwith tests" },
        {
          id: "inq-2",
          owner: { id: "reviewer", kind: "goal" },
          position: 2,
          status: "queued",
          text: "Also update the changelog",
        },
      ],
    });

    const header = await screen.findByTestId("composer-queue-header");
    expect(within(header).getByTestId("composer-queue-count")).toHaveTextContent("2");
    expect(header).toHaveTextContent(/queued/);
    expect(within(header).queryByTestId("composer-queue-full")).not.toBeInTheDocument();
    const rows = screen.getAllByTestId("composer-queued-prompt-row");
    expect(within(rows[0]!).getByTestId("composer-queued-position")).toHaveTextContent("#1");
    // The preview is one flattened line; the full text stays on the title.
    expect(within(rows[0]!).getByTestId("composer-queued-preview")).toHaveTextContent("Ship it");
    expect(within(rows[0]!).queryByTestId("composer-queued-owner")).not.toBeInTheDocument();
    expect(within(rows[0]!).getByTestId("composer-queued-steer")).toBeEnabled();
    // Another actor's entry is attributed and read-only: verbs absent, never disabled.
    expect(within(rows[1]!).getByTestId("composer-queued-position")).toHaveTextContent("#2");
    expect(within(rows[1]!).getByTestId("composer-queued-owner")).toHaveTextContent("reviewer");
    expect(within(rows[1]!).queryByTestId("composer-queued-steer")).not.toBeInTheDocument();
    expect(within(rows[1]!).queryByTestId("composer-queued-edit")).not.toBeInTheDocument();
    expect(within(rows[1]!).queryByTestId("composer-queued-remove")).not.toBeInTheDocument();
  });

  it("Should confirm an explicit clear in place before calling the clear handler once", async () => {
    const user = userEvent.setup();
    const onClearQueue = vi
      .fn()
      .mockResolvedValue({ cleared_count: 1, inputs: [], queue_generation: 2 });
    renderComposer({
      isSessionRunning: true,
      allowBusyInput: true,
      onQueuePrompt: vi.fn(() => Promise.resolve()),
      onSteerQueuedPrompt: vi.fn(),
      onRemoveQueuedPrompt: vi.fn(),
      onClearQueue,
      queuedPrompts: [{ id: "inq-1", owner: null, position: 1, status: "queued", text: "Ship it" }],
    });

    await user.click(await screen.findByTestId("composer-queue-clear"));
    const confirm = screen.getByTestId("composer-queue-clear-confirm");
    expect(confirm).toHaveTextContent("Remove all 1 queued follow-up?");
    // While confirming, the row verbs are gone: the only question is the whole queue.
    expect(screen.queryByTestId("composer-queued-remove")).not.toBeInTheDocument();
    await user.click(within(confirm).getByTestId("composer-queue-clear-keep"));
    expect(onClearQueue).not.toHaveBeenCalled();
    expect(screen.getByTestId("composer-queue-header")).toBeInTheDocument();

    await user.click(screen.getByTestId("composer-queue-clear"));
    await user.keyboard("{Escape}");
    expect(onClearQueue).not.toHaveBeenCalled();
    expect(screen.getByTestId("composer-queue-header")).toBeInTheDocument();

    await user.click(screen.getByTestId("composer-queue-clear"));
    await user.click(screen.getByTestId("composer-queue-clear-confirm-button"));
    await waitFor(() => expect(onClearQueue).toHaveBeenCalledOnce());
    await waitFor(() => expect(screen.getByTestId("composer-queue-header")).toBeInTheDocument());
  });

  it("Should show an unconfirmed send with Retry replaying its identity and Discard dropping it", async () => {
    const user = userEvent.setup();
    const onRetryUnconfirmedSend = vi.fn().mockResolvedValue({
      disposition: "queued",
      entryId: "inq-9",
      idempotencyKey: "idk-1",
      messageId: "msg-1",
      queuePosition: 2,
      replayed: true,
      steerDelivery: null,
      turnId: "turn-1",
    });
    const onDiscardUnconfirmedSend = vi.fn();
    renderComposer({
      isSessionRunning: true,
      allowBusyInput: true,
      onQueuePrompt: vi.fn(() => Promise.resolve()),
      onSteerQueuedPrompt: vi.fn(),
      onRemoveQueuedPrompt: vi.fn(),
      onRetryUnconfirmedSend,
      onDiscardUnconfirmedSend,
      queuedPrompts: [],
      unconfirmedSends: [
        {
          action: "queue",
          attachments: [],
          expectedTurnId: "turn-1",
          id: "msg-1",
          identity: { idempotencyKey: "idk-1", messageId: "msg-1" },
          phase: "unconfirmed",
          runtime: null,
          text: "Also run the migration equivalence suite",
        },
      ],
    });

    const row = await screen.findByTestId("composer-queued-prompt-row");
    expect(row).toHaveAttribute("data-local", "unconfirmed");
    expect(within(row).getByTestId("composer-queued-position")).toHaveTextContent("—");
    expect(within(row).getByTestId("composer-queued-state")).toHaveTextContent(
      "Not confirmed — retry or discard"
    );
    // The message id stays one step deeper, on hover.
    expect(within(row).getByTestId("composer-queued-state")).not.toHaveTextContent("msg-1");
    expect(within(row).getByTitle("msg-1")).toBeInTheDocument();

    await user.click(within(row).getByTestId("composer-queued-retry"));
    expect(onRetryUnconfirmedSend).toHaveBeenCalledWith("msg-1");
    // The replay's answer lands in the composer note: it had arrived, nothing was sent twice.
    const note = await screen.findByTestId("composer-feedback-note");
    expect(note).toHaveAttribute("data-code", "queued");
    expect(note).toHaveTextContent("Queued #2");
    expect(note).toHaveTextContent("nothing was sent twice");
    expect(note).toHaveAttribute("data-detail", "replayed");

    await user.click(within(row).getByTestId("composer-queued-discard"));
    expect(onDiscardUnconfirmedSend).toHaveBeenCalledWith("msg-1");
  });

  it("Should drop the queue affordances while the daemon reports the queue at cap", async () => {
    renderComposer({
      isSessionRunning: true,
      allowBusyInput: true,
      onQueuePrompt: vi.fn(() => Promise.resolve()),
      onSteerPrompt: vi.fn(() => Promise.resolve()),
      onSteerQueuedPrompt: vi.fn(),
      onRemoveQueuedPrompt: vi.fn(),
      queueCap: 2,
      queuedPrompts: [
        { id: "inq-1", owner: null, position: 1, status: "queued", text: "one" },
        { id: "inq-2", owner: null, position: 2, status: "queued", text: "two" },
      ],
    });

    const header = await screen.findByTestId("composer-queue-header");
    expect(within(header).getByTestId("composer-queue-full")).toHaveTextContent("full");
    expect(screen.queryByTestId("composer-queue-button")).not.toBeInTheDocument();
    expect(screen.getByTestId("composer-steer-button")).toBeInTheDocument();
    const hint = screen.getByTestId("composer-enter-hint");
    expect(hint).toHaveAttribute("data-enter", "steer");
    expect(hint).not.toHaveAttribute("data-modifier");
  });

  it("Should edit only the annotation in the row and keep the envelope on save", async () => {
    const user = userEvent.setup();
    const quote = buildTerminalQuote({
      terminalId: "term-4f21c9a03b7e",
      fromLine: 1,
      lines: ["FAIL"],
    });
    const wire = composeQuotedPrompt("What failed?", quote);
    const onReplaceQueuedPrompt = vi.fn().mockResolvedValue(undefined);
    renderComposer({
      isSessionRunning: true,
      allowBusyInput: true,
      onQueuePrompt: vi.fn(() => Promise.resolve()),
      onReplaceQueuedPrompt,
      onSteerQueuedPrompt: vi.fn(),
      onRemoveQueuedPrompt: vi.fn(),
      queuedPrompts: [{ id: "inq-quote", owner: null, position: 1, text: wire }],
    });

    const row = await screen.findByTestId("composer-queued-prompt-row");
    await user.click(within(row).getByTestId("composer-queued-edit"));
    const editor = await screen.findByTestId("composer-queued-editor");
    // The editor never receives `<terminal_context>`; the envelope rides back on save.
    expect(editor).toHaveValue("What failed?");
    await user.clear(editor);
    await user.type(editor, "Why did it fail?");
    await user.keyboard("{Meta>}{Enter}{/Meta}");
    await waitFor(() =>
      expect(onReplaceQueuedPrompt).toHaveBeenCalledWith(
        { id: "inq-quote", owner: null, position: 1, text: wire },
        composeQuotedPrompt("Why did it fail?", quote)
      )
    );
    expect(peekSessionTerminalQuote(primarySessionFixture.id)).toBeNull();
  });

  it("Should cancel an in-row edit with Escape and keep the entry untouched", async () => {
    const user = userEvent.setup();
    const onReplaceQueuedPrompt = vi.fn().mockResolvedValue(undefined);
    renderComposer({
      isSessionRunning: true,
      allowBusyInput: true,
      onQueuePrompt: vi.fn(() => Promise.resolve()),
      onReplaceQueuedPrompt,
      onSteerQueuedPrompt: vi.fn(),
      onRemoveQueuedPrompt: vi.fn(),
      queuedPrompts: [{ id: "inq-1", owner: null, position: 1, text: "Edit this queued prompt." }],
    });

    const row = await screen.findByTestId("composer-queued-prompt-row");
    await user.click(within(row).getByTestId("composer-queued-edit"));
    const editor = await screen.findByTestId("composer-queued-editor");
    await user.type(editor, " changed");
    await user.keyboard("{Escape}");
    expect(screen.queryByTestId("composer-queued-editor")).not.toBeInTheDocument();
    expect(screen.getByTestId("composer-queued-preview")).toHaveTextContent(
      "Edit this queued prompt."
    );
    expect(onReplaceQueuedPrompt).not.toHaveBeenCalled();
    // Without a replace handler the edit verb is absent, never disabled.
    expect(screen.getByTestId("composer-queued-edit")).toBeInTheDocument();
  });

  it("Should hand an edit refused as entry_dispatching to the composer as a fresh draft", async () => {
    const user = userEvent.setup();
    // Invariant: a refused queued edit preserves text authored while Save was
    // pending. Owner: composer interaction; canonical session-thread suite.
    let rejectReplacement: ((error: Error) => void) | undefined;
    const onReplaceQueuedPrompt = vi.fn(
      () =>
        new Promise<void>((_resolve, reject) => {
          rejectReplacement = reject;
        })
    );
    renderComposer({
      isSessionRunning: true,
      allowBusyInput: true,
      onQueuePrompt: vi.fn(() => Promise.resolve()),
      onReplaceQueuedPrompt,
      onSteerQueuedPrompt: vi.fn(),
      onRemoveQueuedPrompt: vi.fn(),
      queuedPrompts: [{ id: "inq-1", owner: null, position: 1, text: "Ship it with tests" }],
    });

    const row = await screen.findByTestId("composer-queued-prompt-row");
    await user.click(within(row).getByTestId("composer-queued-edit"));
    const editor = await screen.findByTestId("composer-queued-editor");
    await user.type(editor, " — and run the e2e lane too");
    await user.click(screen.getByTestId("composer-queued-edit-save"));

    await setComposerText("Keep my concurrent draft");
    expect(composerText()).toBe("Keep my concurrent draft");
    await act(async () => {
      rejectReplacement?.(
        new SessionApiError("already sending", 409, primarySessionFixture.id, {
          code: "entry_dispatching",
        })
      );
    });

    // The current draft survives ahead of the refused edit.
    await waitFor(() =>
      expect(composerText()).toBe(
        "Keep my concurrent draft\n\nShip it with tests — and run the e2e lane too"
      )
    );
    const note = await screen.findByTestId("composer-feedback-note");
    expect(note).toHaveAttribute("data-code", "entry_dispatching");
    expect(note).toHaveTextContent("That one is already sending");
    expect(screen.queryByTestId("composer-queued-editor")).not.toBeInTheDocument();
    expect(toast.error).not.toHaveBeenCalled();
  });

  it("Should keep the editor open when the replacement never landed", async () => {
    const user = userEvent.setup();
    const onReplaceQueuedPrompt = vi.fn().mockRejectedValue(new TypeError("Failed to fetch"));
    renderComposer({
      isSessionRunning: true,
      allowBusyInput: true,
      onQueuePrompt: vi.fn(() => Promise.resolve()),
      onReplaceQueuedPrompt,
      onSteerQueuedPrompt: vi.fn(),
      onRemoveQueuedPrompt: vi.fn(),
      queuedPrompts: [{ id: "inq-1", owner: null, position: 1, text: "Queued prompt text." }],
    });

    const row = await screen.findByTestId("composer-queued-prompt-row");
    await user.click(within(row).getByTestId("composer-queued-edit"));
    await user.click(screen.getByTestId("composer-queued-edit-save"));
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith("Failed to fetch"));
    expect(screen.getByTestId("composer-queued-editor")).toHaveValue("Queued prompt text.");
    expect(composerText()).toBe("");
  });

  it("Should render dispatching input as immutable while the daemon owns delivery", async () => {
    renderComposer({
      isSessionRunning: true,
      onQueuePrompt: vi.fn(),
      onSteerQueuedPrompt: vi.fn(),
      onRemoveQueuedPrompt: vi.fn(),
      queuedPrompts: [
        {
          id: "inq-dispatching",
          owner: null,
          position: 1,
          status: "dispatching",
          text: "Already leaving the queue.",
        },
      ],
    });

    // Dispatching is a state, not a disappearance: the row says so and offers no
    // verb the runtime would refuse (absent, never disabled).
    const row = await screen.findByTestId("composer-queued-prompt-row");
    expect(row).toHaveAttribute("data-status", "dispatching");
    expect(within(row).getByTestId("composer-queued-state")).toHaveTextContent("Sending…");
    expect(within(row).queryByTestId("composer-queued-steer")).not.toBeInTheDocument();
    expect(within(row).queryByTestId("composer-queued-edit")).not.toBeInTheDocument();
    expect(within(row).queryByTestId("composer-queued-remove")).not.toBeInTheDocument();
  });

  // US-004 AC-2: the message typed in session-create is sent by the session
  // itself, once. `renderComposer` mounts under StrictMode, so the effect is
  // deliberately invoked twice — a second POST here would be a real duplicate
  // turn charged to the operator.
  it("Should send a queued first prompt exactly once", async () => {
    const promptCalls: string[] = [];
    vi.stubGlobal("fetch", createPromptRecordingFetchMock(promptCalls));
    act(() => {
      sessionStore.trigger.firstPromptQueued({
        sessionId: primarySessionFixture.id,
        text: "Investigate the regression",
      });
    });

    renderComposer({ isSessionRunning: false });

    await waitFor(() => expect(promptCalls).toHaveLength(1));
    expect(promptCalls[0]).toContain("Investigate the regression");
    await waitFor(() =>
      expect(
        sessionStore.getSnapshot().context.firstPrompts[primarySessionFixture.id]
      ).toBeUndefined()
    );
    // Settle any trailing effect pass before asserting no second delivery.
    await act(async () => {
      await Promise.resolve();
    });
    expect(promptCalls).toHaveLength(1);
  });

  it("Should upload first-prompt files once and send the selected runtime with them", async () => {
    const promptCalls: string[] = [];
    vi.stubGlobal("fetch", createPromptRecordingFetchMock(promptCalls));
    const runtime = {
      provider: "codex",
      model: "gpt-5.6-sol",
      reasoning_effort: "high",
      speed: "fast" as const,
    };
    const store = sessionPromptRuntimeStoreLogic.createStore(
      sessionPromptRuntimeInput({
        agentName: primarySessionFixture.agent_name,
        sessionId: primarySessionFixture.id,
        workspaceId: fixtureWorkspaceId(),
        canPrompt: true,
        effectiveRuntime: undefined,
        selectedRuntime: undefined,
        selectionRevision: 0,
      })
    );
    act(() =>
      sessionStore.trigger.firstPromptQueued({
        sessionId: primarySessionFixture.id,
        text: "Inspect this file",
        files: [new File(["evidence"], "notes.txt", { type: "text/plain" })],
        runtime,
      })
    );
    renderComposer({ isSessionRunning: false }, undefined, null, store);
    await waitFor(() => expect(promptCalls).toHaveLength(1));
    const body = JSON.parse(promptCalls[0]!);
    expect(body.runtime).toEqual(runtime);
    expect(body.attachments).toHaveLength(1);
    expect(body.attachments[0].id).toMatch(/^att_/);
    await waitFor(() =>
      expect(
        sessionStore.getSnapshot().context.firstPrompts[primarySessionFixture.id]
      ).toBeUndefined()
    );
    expect(promptCalls).toHaveLength(1);
  });

  // Invariant: rejected first-message uploads recover the editable draft without sending.
  // Owner: session composer/runtime handoff; canonical suite: SessionThread.
  it("Should recover the first-prompt draft when attachment upload is refused", async () => {
    const promptCalls: string[] = [];
    vi.stubGlobal("fetch", createPromptRecordingFetchMock(promptCalls));
    act(() =>
      sessionStore.trigger.firstPromptQueued({
        sessionId: primarySessionFixture.id,
        text: "Keep this message",
        files: [new File(["zip"], "archive.zip", { type: "application/zip" })],
      })
    );
    renderComposer({ isSessionRunning: false });
    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        "Couldn't send the first message. It is waiting in the composer."
      )
    );
    expect(promptCalls).toHaveLength(0);
    expect(composerText()).toBe("Keep this message");
    expect(screen.getByTestId("composer-attachment-tile")).toHaveAttribute(
      "data-state",
      "rejected"
    );
    expect(sessionStore.getSnapshot().context.drafts[primarySessionFixture.id]).toBe(
      "Keep this message"
    );
    expect(
      sessionStore.getSnapshot().context.firstPrompts[primarySessionFixture.id]
    ).toBeUndefined();
  });

  // Invariant: removing an uploading first-prompt file relinquishes automatic delivery immediately;
  // neither a manual send nor reopening can replay the original handoff. Owner: SessionThread.
  it.each([false, true])(
    "Should not restore removed first-prompt files after reopening (manual send: %s)",
    async manualSend => {
      let release!: () => void;
      const hold = new Promise<void>(resolve => {
        release = resolve;
      });
      const promptCalls: string[] = [];
      vi.stubGlobal(
        "fetch",
        createPromptRecordingFetchMock(promptCalls, { beforeAttachmentUpload: () => hold })
      );
      act(() =>
        sessionStore.trigger.firstPromptQueued({
          sessionId: primarySessionFixture.id,
          text: "Keep only the edited text",
          files: [new File(["evidence"], "removed.txt", { type: "text/plain" })],
        })
      );
      const view = renderComposer({ isSessionRunning: false });
      const tile = await screen.findByTestId("composer-attachment-tile");
      expect(tile).toHaveAttribute("data-state", "uploading");
      let removal!: Promise<void>;
      act(() => {
        const attachment = requireComposerAui().composer.getState().attachments[0]!;
        removal = requireComposerAui().composer.attachment({ id: attachment.id }).remove();
      });
      expect(
        sessionStore.getSnapshot().context.firstPrompts[primarySessionFixture.id]
      ).toBeUndefined();
      if (manualSend) {
        await act(async () => {
          release();
          await removal;
        });
        expect(promptCalls).toHaveLength(0);
        await userEvent.setup().click(screen.getByTestId("composer-send-button"));
        await waitFor(() => expect(promptCalls).toHaveLength(1));
        expect(JSON.parse(promptCalls[0]!).attachments ?? []).toHaveLength(0);
      }
      view.unmount();
      renderComposer({ isSessionRunning: false });
      await screen.findByTestId("composer-input");
      if (!manualSend)
        await act(async () => {
          release();
          await removal;
        });
      expect(screen.queryByTestId("composer-attachment-tile")).not.toBeInTheDocument();
      expect(composerText()).toBe(manualSend ? "" : "Keep only the edited text");
      expect(promptCalls).toHaveLength(manualSend ? 1 : 0);
      expect(
        sessionStore.getSnapshot().context.firstPrompts[primarySessionFixture.id]
      ).toBeUndefined();
    }
  );

  it("Should consume the first-prompt handoff on a manual composer send during upload", async () => {
    let release!: () => void;
    const hold = new Promise<void>(resolve => {
      release = resolve;
    });
    const promptCalls: string[] = [];
    vi.stubGlobal(
      "fetch",
      createPromptRecordingFetchMock(promptCalls, { beforeAttachmentUpload: () => hold })
    );
    act(() =>
      sessionStore.trigger.firstPromptQueued({
        sessionId: primarySessionFixture.id,
        text: "Send this draft manually",
        files: [new File(["evidence"], "manual.txt", { type: "text/plain" })],
      })
    );
    const view = renderComposer({ isSessionRunning: false });
    await screen.findByTestId("composer-attachment-tile");
    act(() => {
      void requireComposerAui().composer.send();
    });
    await act(async () => release());
    await waitFor(() => expect(promptCalls).toHaveLength(1));
    await waitFor(() =>
      expect(
        sessionStore.getSnapshot().context.firstPrompts[primarySessionFixture.id]
      ).toBeUndefined()
    );
    view.unmount();
    renderComposer({ isSessionRunning: false });
    await screen.findByTestId("composer-input");
    expect(screen.queryByTestId("composer-attachment-tile")).not.toBeInTheDocument();
    expect(composerText()).toBe("");
    expect(promptCalls).toHaveLength(1);
  });

  // Invariant: window teardown during upload preserves first-message files and resumes once.
  // Owner: session runtime/composer lifecycle; canonical suite: SessionThread.
  it("Should resume first-prompt attachments after closing during upload", async () => {
    let release!: () => void;
    const hold = new Promise<void>(resolve => {
      release = resolve;
    });
    const promptCalls: string[] = [];
    vi.stubGlobal(
      "fetch",
      createPromptRecordingFetchMock(promptCalls, {
        beforeAttachmentUpload: () => hold,
      })
    );
    const file = new File(["evidence"], "resume.txt", { type: "text/plain" });
    act(() =>
      sessionStore.trigger.firstPromptQueued({
        sessionId: primarySessionFixture.id,
        text: "Read after reopening",
        files: [file],
      })
    );
    const first = renderComposer({ isSessionRunning: false });
    const tile = await screen.findByTestId("composer-attachment-tile");
    expect(tile).toHaveAttribute("data-state", "uploading");
    first.unmount();
    expect(sessionStore.getSnapshot().context.firstPrompts[primarySessionFixture.id]).toMatchObject(
      {
        claimed: false,
        files: [file],
      }
    );
    renderComposer({ isSessionRunning: false });
    await screen.findByTestId("composer-attachment-tile");
    await act(async () => release());
    await waitFor(() => expect(promptCalls).toHaveLength(1));
    expect(JSON.parse(promptCalls[0]!).attachments).toHaveLength(1);
    expect(promptCalls[0]).toContain("Read after reopening");
    await waitFor(() =>
      expect(
        sessionStore.getSnapshot().context.firstPrompts[primarySessionFixture.id]
      ).toBeUndefined()
    );
  });

  it("Should hold a queued first prompt while the session cannot be prompted", async () => {
    const promptCalls: string[] = [];
    vi.stubGlobal("fetch", createPromptRecordingFetchMock(promptCalls));
    act(() => {
      sessionStore.trigger.firstPromptQueued({
        sessionId: primarySessionFixture.id,
        text: "Not yet",
      });
    });

    renderComposer({ canPrompt: false, isSessionRunning: false });

    await screen.findByTestId("composer-input");
    expect(promptCalls).toHaveLength(0);
    expect(sessionStore.getSnapshot().context.firstPrompts[primarySessionFixture.id]?.text).toBe(
      "Not yet"
    );
  });
});

// Invariant: a process-global pending quote is never consumed by an arbitrary
// mounted thread, including a read-only one. Only create success or an explicit
// chosen-session stage may take it.
// Owning layer: SessionThread mount / quote ownership.
describe("SessionThread pending quote ownership", () => {
  const otherSessionId = "sess-other";

  beforeEach(() => {
    takePendingTerminalQuote();
    clearSessionTerminalQuote(primarySessionFixture.id);
    clearSessionTerminalQuote(otherSessionId);
    vi.stubGlobal("fetch", createFetchMock());
  });

  afterEach(() => {
    takePendingTerminalQuote();
    clearSessionTerminalQuote(primarySessionFixture.id);
    clearSessionTerminalQuote(otherSessionId);
    vi.unstubAllGlobals();
  });

  it("Should not consume a pending quote when two threads mount", async () => {
    const quote = buildTerminalQuote({
      terminalId: "term-4f21c9a03b7e",
      fromLine: 1,
      lines: ["one line"],
    });
    holdPendingTerminalQuote(quote);
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
              onCancelPrompt={() => {}}
            />
            <SessionThread
              sessionId={otherSessionId}
              agentName={primarySessionFixture.agent_name}
              canPrompt
              readOnly
            />
          </SessionTranscriptThreadProvider>
        </SessionChatRuntimeProvider>
      </QueryClientProvider>
    );

    await screen.findByTestId("composer-input");
    expect(peekPendingTerminalQuote()?.text).toBe(quote.text);
    expect(peekSessionTerminalQuote(primarySessionFixture.id)).toBeNull();
    expect(peekSessionTerminalQuote(otherSessionId)).toBeNull();
    expect(screen.queryByTestId("terminal-quote-block")).not.toBeInTheDocument();
  });

  it("Should not consume a pending quote when a read-only thread mounts", async () => {
    const quote = buildTerminalQuote({
      terminalId: "term-4f21c9a03b7e",
      fromLine: 1,
      lines: ["one line"],
    });
    holdPendingTerminalQuote(quote);
    renderThreadState({ status: "success", readOnly: true });

    await screen.findByTestId("chat-view");
    expect(peekPendingTerminalQuote()?.text).toBe(quote.text);
    expect(peekSessionTerminalQuote(primarySessionFixture.id)).toBeNull();
    expect(screen.queryByTestId("terminal-quote-block")).not.toBeInTheDocument();
  });
});

// Invariant: picker, paste, and drop create durable-ready composer tiles, while
// queue/interrupt preserve daemon-owned refs and capability gates stay truthful.
// Owning layer: SessionThread's public composer interaction boundary.
describe("SessionThread composer attachments", () => {
  beforeEach(() => {
    composerAui = null;
    vi.stubGlobal("fetch", createFetchMock());
    vi.mocked(toast.error).mockClear();
    vi.mocked(toast.warning).mockClear();
    sessionStore.trigger.composerDraftDiscarded({ sessionId: primarySessionFixture.id });
    sessionStore.trigger.firstPromptSent({ sessionId: primarySessionFixture.id });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    resetGatewayStreamAuth();
    act(() => {
      sessionStore.trigger.composerDraftDiscarded({ sessionId: primarySessionFixture.id });
      sessionStore.trigger.firstPromptSent({ sessionId: primarySessionFixture.id });
    });
  });

  it("Should turn a pasted image into a draft tile", async () => {
    renderComposer({ promptImageCapability: "supported" });
    await screen.findByTestId("composer-input");
    const editable = await findComposerEditable();
    const file = pngFile();
    fireEvent.paste(editable, {
      clipboardData: {
        files: [file],
        items: [{ kind: "file", type: file.type, getAsFile: () => file }],
        types: ["Files"],
      },
    });

    const tile = await screen.findByTestId("composer-attachment-tile");
    expect(tile).toHaveTextContent("shot.png");
    await waitFor(() => expect(tile).toHaveAttribute("data-state", "ready"));
  });

  it.each([
    ["close", false],
    ["escape", false],
    ["backdrop", false],
    ["close", true],
    ["escape", true],
    ["backdrop", true],
  ] as const)(
    "Should dismiss a composer image with %s (window scoped: %s) and keep the draft",
    async (dismissal, windowScoped) => {
      const previewUrls: string[] = [];
      const MockURL = class extends URL {};
      MockURL.createObjectURL = vi.fn(() => {
        const url = `blob:composer-preview-${previewUrls.length}`;
        previewUrls.push(url);
        return url;
      });
      MockURL.revokeObjectURL = vi.fn();
      vi.stubGlobal("URL", MockURL);
      const user = userEvent.setup();
      const originalURL = window.location.href;
      const host = document.createElement("div");
      document.body.append(host);
      renderComposer({ promptImageCapability: "supported" }, undefined, windowScoped ? host : null);
      await screen.findByTestId("composer-input");
      await act(async () => {
        await requireComposerAui().composer.addAttachment(pngFile());
      });
      const trigger = await screen.findByRole("button", { name: "Preview shot.png" });
      await user.click(trigger);
      const dialog = await screen.findByRole("dialog", { name: "shot.png" });
      const source = within(dialog).getByRole("img", { name: "shot.png" }).getAttribute("src");
      expect(source).toMatch(/^blob:composer-preview-/);
      if (dismissal === "close")
        await user.click(within(dialog).getByRole("button", { name: "Close" }));
      else if (dismissal === "escape") await user.keyboard("{Escape}");
      else await user.click(document.querySelector<HTMLElement>('[data-slot="dialog-overlay"]')!);
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
      await waitFor(() => expect(trigger).toHaveFocus());
      expect(screen.getByTestId("composer-attachment-tile")).toHaveTextContent("shot.png");
      expect(MockURL.revokeObjectURL).toHaveBeenCalledWith(source);
      expect(window.location.href).toBe(originalURL);
    }
  );

  it("Should add an attachment when browser crypto is unavailable", async () => {
    vi.stubGlobal("crypto", undefined);
    renderComposer({ promptImageCapability: "supported" });
    await screen.findByTestId("composer-input");

    await act(async () => {
      await requireComposerAui().composer.addAttachment(pngFile("fallback-id.png"));
    });

    const tile = await screen.findByTestId("composer-attachment-tile");
    expect(tile).toHaveTextContent("fallback-id.png");
    await waitFor(() => expect(tile).toHaveAttribute("data-state", "ready"));
  });

  it("Should prevent disabled file drops without adding an attachment", async () => {
    renderComposer({ canPrompt: false, promptImageCapability: "supported" });
    await screen.findByTestId("composer-input");
    const overlay = screen.getByTestId("composer-drop-overlay");
    const dropRoot = overlay.parentElement;
    if (!dropRoot) {
      throw new Error("composer drop root is not mounted");
    }
    const transfer = {
      dropEffect: "copy",
      files: [pngFile()],
      types: ["Files"],
    };

    expect(fireEvent.dragOver(dropRoot, { dataTransfer: transfer })).toBe(false);
    expect(transfer.dropEffect).toBe("none");
    expect(fireEvent.drop(dropRoot, { dataTransfer: transfer })).toBe(false);
    expect(overlay).toHaveAttribute("aria-hidden", "true");
    expect(screen.queryByTestId("composer-attachment-tile")).not.toBeInTheDocument();
  });

  it("Should add a ready tile when an enabled drop contains a file", async () => {
    renderComposer({ promptImageCapability: "supported" });
    await screen.findByTestId("composer-input");
    const overlay = screen.getByTestId("composer-drop-overlay");
    const dropRoot = overlay.parentElement;
    if (!dropRoot) throw new Error("composer drop root is not mounted");
    const file = pngFile("dropped.png");

    fireEvent.drop(dropRoot, {
      dataTransfer: { dropEffect: "copy", files: [file], types: ["Files"] },
    });

    const tile = await screen.findByTestId("composer-attachment-tile");
    expect(tile).toHaveTextContent("dropped.png");
    await waitFor(() => expect(tile).toHaveAttribute("data-state", "ready"));
  });

  it("Should clear an active file drag when prompting becomes unavailable", async () => {
    const view = renderComposerRerenderable({
      canPrompt: true,
      promptImageCapability: "supported",
    });
    await screen.findByTestId("composer-input");
    const overlay = screen.getByTestId("composer-drop-overlay");
    const dropRoot = overlay.parentElement;
    if (!dropRoot) {
      throw new Error("composer drop root is not mounted");
    }
    const transfer = {
      dropEffect: "copy",
      files: [pngFile()],
      types: ["Files"],
    };

    fireEvent.dragEnter(dropRoot, { dataTransfer: transfer });
    expect(overlay).toHaveAttribute("aria-hidden", "false");

    view.rerender({ canPrompt: false, promptImageCapability: "supported" });
    expect(overlay).toHaveAttribute("aria-hidden", "true");
  });

  it("Should turn a picker file into a draft tile", async () => {
    const user = userEvent.setup();
    renderComposer({ promptImageCapability: "supported" });
    await screen.findByRole("button", { name: "Attach files" });
    const input = screen.getByLabelText("Choose attachments");
    if (!(input instanceof HTMLInputElement)) throw new Error("composer file input is not mounted");
    await user.upload(input, pngFile("picked.png"));
    const tile = await screen.findByTestId("composer-attachment-tile");
    expect(tile).toHaveTextContent("picked.png");
    await waitFor(() => expect(tile).toHaveAttribute("data-state", "ready"));
  });

  it("Should disable send while a tile is uploading", async () => {
    let release!: () => void;
    const hold = new Promise<void>(resolve => {
      release = resolve;
    });
    vi.stubGlobal("fetch", createFetchMock({ beforeAttachmentUpload: () => hold }));
    renderComposer({ promptImageCapability: "supported" });
    await screen.findByTestId("composer-input");
    let add!: Promise<void>;
    act(() => {
      add = requireComposerAui().composer.addAttachment(pngFile());
    });
    const tile = await screen.findByTestId("composer-attachment-tile");
    expect(tile).toHaveAttribute("data-state", "uploading");
    const send = screen.getByTestId("composer-send-button");
    expect(send).toBeDisabled();
    expect(send).toHaveAttribute("title", "Saving shot.png");
    await act(async () => {
      release();
      await add;
    });
    await waitFor(() => expect(tile).toHaveAttribute("data-state", "ready"));
    expect(composerText()).toBe("");
    expect(send).toBeEnabled();
  });

  // Invariant (issue #815): Enter honors the same send blocker as the disabled
  // Send control and answers with its reason; nothing leaves and the draft stays.
  it("Should hold Enter while a tile is uploading and name the reason", async () => {
    let release!: () => void;
    const hold = new Promise<void>(resolve => {
      release = resolve;
    });
    const fetchMock = createFetchMock({ beforeAttachmentUpload: () => hold });
    vi.stubGlobal("fetch", fetchMock);
    const user = userEvent.setup();
    renderComposer({ promptImageCapability: "supported" });
    await screen.findByTestId("composer-input");
    await setComposerText("Ship it with the screenshot");
    let add!: Promise<void>;
    act(() => {
      add = requireComposerAui().composer.addAttachment(pngFile());
    });
    const tile = await screen.findByTestId("composer-attachment-tile");
    expect(tile).toHaveAttribute("data-state", "uploading");

    await user.click(screen.getByRole("textbox", { name: "Session prompt" }));
    await user.keyboard("{Enter}");

    expect(vi.mocked(toast.warning)).toHaveBeenCalledWith("Saving shot.png");
    expect(composerText()).toBe("Ship it with the screenshot");
    expect(fetchMock.mock.calls.some(([input]) => getPathname(input).endsWith("/prompt"))).toBe(
      false
    );
    await act(async () => {
      release();
      await add;
    });
    await waitFor(() => expect(tile).toHaveAttribute("data-state", "ready"));
  });

  it("Should refuse unsupported files in place and keep send disabled", async () => {
    renderComposer({ promptImageCapability: "supported" });
    await screen.findByTestId("composer-input");
    await act(async () => {
      await requireComposerAui().composer.addAttachment(zipFile());
    });
    const tile = await screen.findByTestId("composer-attachment-tile");
    expect(tile).toHaveAttribute("data-state", "rejected");
    expect(tile).toHaveTextContent("PNG, JPEG, WebP, PDF, Markdown, or text");
    const send = screen.getByTestId("composer-send-button");
    expect(send).toBeDisabled();
    expect(send).toHaveAttribute("title", "Remove files that are not supported");
  });

  it("Should show the capability gate for images when the targeted agent cannot accept them", async () => {
    renderComposer({ promptImageCapability: "unsupported" });
    await screen.findByTestId("composer-input");
    await act(async () => {
      await requireComposerAui().composer.addAttachment(pngFile());
    });
    await waitFor(() =>
      expect(screen.getByTestId("composer-attachment-tile")).toHaveAttribute("data-state", "ready")
    );
    expect(screen.getByTestId("composer-attachment-gate")).toHaveTextContent(
      "This agent does not accept images."
    );
    const send = screen.getByTestId("composer-send-button");
    expect(send).toBeDisabled();
    expect(send).toHaveAttribute("title", "This agent does not accept images");
  });

  it("Should show the capability gate for PDF files without embedded context", async () => {
    vi.stubGlobal("fetch", createFetchMock({ attachmentUpload: { name: "report.pdf" } }));
    renderComposer({
      promptEmbeddedContextCapability: "unsupported",
      promptImageCapability: "supported",
    });
    await screen.findByTestId("composer-input");
    await act(async () => {
      await requireComposerAui().composer.addAttachment(pdfFile());
    });
    await waitFor(() =>
      expect(screen.getByTestId("composer-attachment-tile")).toHaveAttribute("data-state", "ready")
    );
    expect(screen.getByTestId("composer-attachment-gate")).toHaveTextContent(
      "This agent does not accept PDF files."
    );
    const send = screen.getByTestId("composer-send-button");
    expect(send).toBeDisabled();
    expect(send).toHaveAttribute("title", "This agent does not accept PDF files");
  });

  it("Should send a ready PDF while embedded-context capability is unknown", async () => {
    const promptCalls: string[] = [];
    const user = userEvent.setup();
    vi.stubGlobal("fetch", createPromptRecordingFetchMock(promptCalls));
    renderComposer({
      promptEmbeddedContextCapability: "unknown",
      promptImageCapability: "supported",
    });

    await screen.findByTestId("composer-input");
    await act(async () => {
      await requireComposerAui().composer.addAttachment(pdfFile());
    });
    await waitFor(() =>
      expect(screen.getByTestId("composer-attachment-tile")).toHaveAttribute("data-state", "ready")
    );
    await user.click(screen.getByTestId("composer-send-button"));

    await waitFor(() => expect(promptCalls).toHaveLength(1));
    expect(promptCalls[0]).toMatch(/att_[a-f0-9]{64}/);
  });

  it("Should send a ready image while the target capability is unknown", async () => {
    const promptCalls: string[] = [];
    const user = userEvent.setup();
    vi.stubGlobal("fetch", createPromptRecordingFetchMock(promptCalls));
    renderComposer({ promptImageCapability: "unknown" });

    await screen.findByTestId("composer-input");
    await act(async () => {
      await requireComposerAui().composer.addAttachment(pngFile());
    });
    await waitFor(() =>
      expect(screen.getByTestId("composer-attachment-tile")).toHaveAttribute("data-state", "ready")
    );
    await user.click(screen.getByTestId("composer-send-button"));

    await waitFor(() => expect(promptCalls).toHaveLength(1));
    expect(promptCalls[0]).toMatch(/att_[a-f0-9]{64}/);
  });

  it("Should preserve uploaded attachment state through a session runtime rerender", async () => {
    const promptCalls: string[] = [];
    const user = userEvent.setup();
    vi.stubGlobal("fetch", createPromptRecordingFetchMock(promptCalls));
    const view = renderComposerRerenderable({ promptImageCapability: "supported" });

    await screen.findByTestId("composer-input");
    await act(async () => {
      await requireComposerAui().composer.addAttachment(pngFile());
    });
    await waitFor(() =>
      expect(screen.getByTestId("composer-attachment-tile")).toHaveAttribute("data-state", "ready")
    );
    view.rerender({ promptImageCapability: "supported" });
    await user.click(screen.getByTestId("composer-send-button"));

    await waitFor(() => expect(promptCalls).toHaveLength(1));
    expect(promptCalls[0]).toMatch(/att_[a-f0-9]{64}/);
  });

  it("Should restore a rejected image and text to the composer", async () => {
    const user = userEvent.setup();
    vi.stubGlobal("fetch", createPromptRejectedFetchMock());
    renderComposer({ promptImageCapability: "unknown" });

    await screen.findByTestId("composer-input");
    await setComposerText("Retry this image");
    await act(async () => {
      await requireComposerAui().composer.addAttachment(pngFile());
    });
    await waitFor(() =>
      expect(screen.getByTestId("composer-attachment-tile")).toHaveAttribute("data-state", "ready")
    );
    await user.click(screen.getByTestId("composer-send-button"));

    await waitFor(() => expect(composerText()).toBe("Retry this image"));
    await waitFor(() =>
      expect(screen.getByTestId("composer-attachment-tile")).toHaveAttribute("data-state", "ready")
    );
  });

  it("Should remove a draft tile and release its image preview", async () => {
    const previewUrls: string[] = [];
    const MockURL = class extends URL {};
    MockURL.createObjectURL = vi.fn(() => {
      const url = `blob:session-attachment-${previewUrls.length}`;
      previewUrls.push(url);
      return url;
    });
    MockURL.revokeObjectURL = vi.fn();
    vi.stubGlobal("URL", MockURL);
    const user = userEvent.setup();
    renderComposer({ promptImageCapability: "supported" });
    await screen.findByTestId("composer-input");
    await act(async () => {
      await requireComposerAui().composer.addAttachment(pngFile());
    });
    await screen.findByTestId("composer-attachment-tile");
    await user.click(screen.getByRole("button", { name: "Remove shot.png" }));
    await waitFor(() => {
      expect(screen.queryByTestId("composer-attachment-tile")).not.toBeInTheDocument();
    });
    expect(previewUrls.length).toBeGreaterThan(0);
    expect(MockURL.revokeObjectURL).toHaveBeenCalledTimes(previewUrls.length);
    for (const url of previewUrls) {
      expect(MockURL.revokeObjectURL).toHaveBeenCalledWith(url);
    }
  });

  it("Should restore unsent attachments in order after the session runtime rebuilds", async () => {
    const fetchMock = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    const MockURL = class extends URL {};
    MockURL.createObjectURL = vi.fn(() => "blob:restored-preview");
    MockURL.revokeObjectURL = vi.fn();
    vi.stubGlobal("URL", MockURL);
    const firstView = renderComposer({ promptImageCapability: "supported" });
    await screen.findByTestId("composer-input");
    await act(async () => {
      await requireComposerAui().composer.addAttachment(pngFile());
      await requireComposerAui().composer.addAttachment(pdfFile());
    });
    await waitFor(() =>
      expect(
        screen.getAllByTestId("composer-attachment-tile").map(tile => tile.dataset.state)
      ).toEqual(["ready", "ready"])
    );
    expect(attachmentRequests(fetchMock)).toEqual({ uploads: 2, deletes: 0 });

    firstView.unmount();
    renderComposer({ promptImageCapability: "supported" });

    await waitFor(() => {
      const tiles = screen.getAllByTestId("composer-attachment-tile");
      expect(tiles.map(tile => tile.textContent)).toEqual([
        expect.stringContaining("shot.png"),
        expect.stringContaining("report.pdf"),
      ]);
      expect(tiles.map(tile => tile.dataset.state)).toEqual(["ready", "ready"]);
    });
    expect(screen.getByRole("button", { name: "Preview shot.png" })).toBeInTheDocument();
    // The rebuilt composer reuses the daemon uploads; nothing is sent or deleted again.
    expect(attachmentRequests(fetchMock)).toEqual({ uploads: 2, deletes: 0 });
  });

  it("Should finish an upload left in flight on the restored composer without uploading again", async () => {
    let releaseUpload!: () => void;
    const uploadGate = new Promise<void>(resolve => {
      releaseUpload = resolve;
    });
    const fetchMock = createFetchMock({ beforeAttachmentUpload: () => uploadGate });
    vi.stubGlobal("fetch", fetchMock);
    const firstView = renderComposer({ promptImageCapability: "supported" });
    await screen.findByTestId("composer-input");
    act(() => {
      void requireComposerAui().composer.addAttachment(pngFile());
    });
    await waitFor(() =>
      expect(screen.getByTestId("composer-attachment-tile")).toHaveAttribute(
        "data-state",
        "uploading"
      )
    );

    firstView.unmount();
    renderComposer({ promptImageCapability: "supported" });
    await screen.findByTestId("composer-attachment-tile");
    await act(async () => releaseUpload());

    await waitFor(() =>
      expect(screen.getByTestId("composer-attachment-tile")).toHaveAttribute("data-state", "ready")
    );
    expect(screen.getAllByTestId("composer-attachment-tile")).toHaveLength(1);
    expect(attachmentRequests(fetchMock)).toEqual({ uploads: 1, deletes: 0 });
  });

  it("Should not restore an attachment removed from the draft", async () => {
    const user = userEvent.setup();
    const fetchMock = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    const firstView = renderComposer({ promptImageCapability: "supported" });
    await screen.findByTestId("composer-input");
    await act(async () => {
      await requireComposerAui().composer.addAttachment(pngFile());
      await requireComposerAui().composer.addAttachment(pdfFile());
    });
    await waitFor(() => expect(screen.getAllByTestId("composer-attachment-tile")).toHaveLength(2));
    await user.click(screen.getByRole("button", { name: "Remove shot.png" }));
    await waitFor(() => expect(screen.getAllByTestId("composer-attachment-tile")).toHaveLength(1));

    firstView.unmount();
    renderComposer({ promptImageCapability: "supported" });

    await waitFor(() =>
      expect(screen.getByTestId("composer-attachment-tile")).toHaveAttribute("data-state", "ready")
    );
    expect(screen.getAllByTestId("composer-attachment-tile")).toHaveLength(1);
    expect(screen.getByTestId("composer-attachment-tile")).toHaveTextContent("report.pdf");
  });

  it("Should not restore attachments that were sent", async () => {
    const promptCalls: string[] = [];
    const user = userEvent.setup();
    vi.stubGlobal("fetch", createPromptRecordingFetchMock(promptCalls));
    const firstView = renderComposer({ promptImageCapability: "supported" });
    await screen.findByTestId("composer-input");
    await act(async () => {
      await requireComposerAui().composer.addAttachment(pngFile());
    });
    await waitFor(() =>
      expect(screen.getByTestId("composer-attachment-tile")).toHaveAttribute("data-state", "ready")
    );
    await user.click(screen.getByTestId("composer-send-button"));
    await waitFor(() => expect(promptCalls).toHaveLength(1));
    await waitFor(() =>
      expect(screen.queryByTestId("composer-attachment-tile")).not.toBeInTheDocument()
    );

    firstView.unmount();
    renderComposer({ promptImageCapability: "supported" });

    await screen.findByTestId("composer-input");
    await act(async () => {});
    expect(screen.queryByTestId("composer-attachment-tile")).not.toBeInTheDocument();
  });

  it("Should summarize queued attachments and steer their complete queue entry", async () => {
    const user = userEvent.setup();
    const onSteerQueuedPrompt = vi.fn();
    renderComposer({
      isSessionRunning: true,
      allowBusyInput: true,
      onQueuePrompt: vi.fn(),
      onRemoveQueuedPrompt: vi.fn(),
      onSteerQueuedPrompt,
      queuedPrompts: [
        {
          id: "inq-att",
          owner: null,
          position: 1,
          text: "Review the screenshots.",
          attachments: {
            fileCount: 1,
            imageCount: 2,
            preview: { kind: "file", mark: "PDF" },
          },
        },
      ],
    });

    const row = await screen.findByTestId("composer-queued-prompt-row");
    expect(within(row).getByTestId("composer-queued-attachment-well")).toHaveTextContent("PDF");
    expect(row).toHaveTextContent("· 2 images · 1 file");
    await user.click(within(row).getByTestId("composer-queued-steer"));
    expect(onSteerQueuedPrompt).toHaveBeenCalledExactlyOnceWith({
      id: "inq-att",
      owner: null,
      position: 1,
      text: "Review the screenshots.",
      attachments: {
        fileCount: 1,
        imageCount: 2,
        preview: { kind: "file", mark: "PDF" },
      },
    });
  });
});

/** Daemon attachment traffic a composer caused: uploads and deletions. */
function attachmentRequests(fetchMock: ReturnType<typeof createFetchMock>) {
  const base = `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/${primarySessionFixture.id}/attachments`;
  const paths = fetchMock.mock.calls.map(([input]) => getPathname(input));
  return {
    uploads: paths.filter(path => path === base).length,
    deletes: paths.filter(path => path.startsWith(`${base}/`)).length,
  };
}

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
        <ComposerAuiProbe />
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

// Invariant: a pending reply leaves the flicker guard even when commit work
// delays its effect. Owner: thread status timer; canonical suite: SessionThread.
describe("SessionThread thinking guard", () => {
  it("Should reveal pending activity when the timer fires before the wall-clock deadline", async () => {
    vi.useFakeTimers();
    const now = vi.spyOn(Date, "now").mockReturnValue(0);
    const view = renderHook(() => useThinkingGuardElapsed(0), { wrapper: StrictMode });
    try {
      now.mockReturnValue(249);
      await act(() => vi.advanceTimersByTimeAsync(250));
      expect(view.result.current).toBe(false);
      now.mockReturnValue(250);
      await act(() => vi.advanceTimersByTimeAsync(1));
      expect(view.result.current).toBe(true);
    } finally {
      view.unmount();
      now.mockRestore();
      vi.useRealTimers();
    }
  });

  it("Should reveal pending activity when the effect starts after the guard elapsed", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(0);
    const view = renderHook(() => {
      const elapsed = useThinkingGuardElapsed(0);
      useLayoutEffect(() => {
        vi.setSystemTime(500);
      }, []);
      return elapsed;
    });
    try {
      await act(() => vi.runOnlyPendingTimersAsync());
      expect(view.result.current).toBe(true);
    } finally {
      view.unmount();
      vi.useRealTimers();
    }
  });
});

// Invariant: runtime projection retains canonical identity plus exact provider title
// (reachable on hover of the plain live line) without losing the timer.
// Owner: session thread composition; canonical suite: session-thread.
it("Should preserve provider titles through the runtime on a plain live line", async () => {
  const title = "Inspect layout\n" + "ação 👩🏽‍💻 ".repeat(100) + "provider-title-tail";
  const transcript = [
    {
      id: "long-title",
      role: "assistant",
      status: { type: "running" },
      parts: [
        {
          type: "tool-Bash",
          title,
          toolCallId: "long-call",
          state: "input-available",
          turnId: "long-turn",
          input: { command: "printf 'input-tail'" },
        },
      ],
    },
  ] as unknown as SessionMessage[];
  renderThreadState({
    status: "success",
    messages: toReadonlyThreadMessages(transcript),
    isSessionRunning: true,
    statusSession: runningStatusSession("2026-09-06T12:00:00Z", title),
  });
  const live = await screen.findByTestId("live-tool-label");
  expect(live).toHaveAccessibleName("Running printf 'input-tail'");
  expect(live).not.toHaveTextContent("provider-title-tail");
  // Plain text, not a popover; the exact provider title stays on hover.
  expect(within(live).queryByRole("button")).toBeNull();
  expect(live).toHaveAttribute("title", title);
  // The Working row no longer repeats the tool: the live row is its one owner.
  expect(screen.queryByRole("button", { name: "Activity details" })).not.toBeInTheDocument();
  expect(screen.getByTestId("session-working-timer")).toBeInTheDocument();
});

// Suite extension: agent-to-agent messages in the thread (agent collaboration `_uiux.md` S1–S3;
// UT-060, UT-066, UT-067). Invariant: a user turn whose metadata carries a session origin renders
// as the session message card, never the operator bubble; a system wake renders only when its
// synthetic kind is `session_reply`; a "Sent to" card resolves its reply state from the reply wake
// whose watch id it names. Owning layer: SessionThreadMessage routing over the readonly thread.
describe("SessionThread session messages", () => {
  const senderId = "sess-7f3a2c11d09e4b58";
  const targetId = "sess-c03f9d61b2e84a07";

  beforeEach(() => {
    vi.stubGlobal("fetch", createFetchMock());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  function clientWithPeers() {
    const queryClient = createQueryClient();
    const peer = (id: string, name: string, agent: string) =>
      queryClient.setQueryData(sessionKeys.detail(fixtureWorkspaceId(), id), {
        ...primarySessionFixture,
        id,
        name,
        agent_name: agent,
      });
    peer(senderId, "Refactor billing", "claude");
    peer(targetId, "Billing reviewer", "codex");
    return queryClient;
  }

  function replyWake(id: string, reason: string, kind = "session_reply"): SessionMessage {
    return {
      id,
      role: "system",
      metadata: {
        turn_id: "turn-wake",
        synthetic: {
          kind,
          wake_event_id: "rw-6e2d81a0",
          child_session_id: targetId,
          child_workspace_id: fixtureWorkspaceId(),
          child_agent_name: "codex",
          reason,
          summary: "Per job, one shared counter.",
          reply_truncated: false,
          hop: 1,
        },
      },
      parts: [
        {
          type: "text",
          text: `Session "Billing reviewer" (${targetId}) replied to your message msg-retry-q: ${reason}.\n---\nWake body text the card never reads.`,
          state: "done",
        },
      ],
    } as SessionMessage;
  }

  it("UT-060: Should render an agent-sent user turn as the session message card", async () => {
    const transcript: SessionMessage[] = [
      {
        id: "msg-retry-q",
        role: "user",
        metadata: {
          turn_id: "turn-q",
          message_id: "msg-retry-q",
          origin: {
            kind: "session",
            session_id: senderId,
            workspace_id: fixtureWorkspaceId(),
            agent_name: "claude",
            title_at_send: "Refactor billing",
            hop: 1,
            notify_on_complete: true,
            reply_watch_id: "rw-6e2d81a0",
          },
        },
        parts: [{ type: "text", text: "Is the retry budget per request or per job?" }],
      } as SessionMessage,
      {
        id: "msg-operator",
        role: "user",
        metadata: { turn_id: "turn-o", message_id: "msg-operator" },
        parts: [{ type: "text", text: "Note that down in the review doc too." }],
      } as SessionMessage,
    ];
    renderThreadState(
      { status: "success", messages: toReadonlyThreadMessages(transcript) },
      clientWithPeers()
    );

    const card = await screen.findByRole("article", { name: "Message from Refactor billing" });
    expect(within(card).getByText("Is the retry budget per request or per job?")).toBeVisible();
    expect(within(card).getByText("Reply requested")).toBeInTheDocument();
    expect(within(card).queryByTestId("user-message-bubble")).not.toBeInTheDocument();
    // The operator's own prompt keeps its bubble.
    expect(screen.getAllByTestId("user-message-bubble")).toHaveLength(1);
  });

  it("UT-066: Should render only session_reply wakes and keep other synthetic kinds hidden", async () => {
    renderThreadState(
      {
        status: "success",
        messages: toReadonlyThreadMessages([
          replyWake("wake-reply", "completed"),
          replyWake("wake-subagent", "completed", "subagent_wake"),
        ]),
      },
      clientWithPeers()
    );

    const card = await screen.findByRole("article", {
      name: "Reply from Billing reviewer, completed",
    });
    expect(within(card).getByText("Per job, one shared counter.")).toBeInTheDocument();
    // The answer is the typed `synthetic.summary`; the wake text is never parsed.
    expect(
      within(card).queryByText(/replied to your message|Wake body text/)
    ).not.toBeInTheDocument();
    expect(screen.getAllByTestId("session-reply-card")).toHaveLength(1);
  });

  it("Should render a fired monitor wake as a divider that names its cause", async () => {
    const monitorWake = {
      id: "wake-monitor",
      role: "system",
      metadata: {
        turn_id: "turn-monitor",
        synthetic: {
          kind: "monitor",
          wake_event_id: "mon-0123456789ab",
          reason: "monitor_exit",
          summary: "Release workflow publishes v0.3.0",
        },
      },
      parts: [
        {
          type: "text",
          text: "[COMPOZY MONITOR - NOT USER INPUT] body the row never reads",
          state: "done",
        },
      ],
    } as SessionMessage;
    renderThreadState(
      { status: "success", messages: toReadonlyThreadMessages([monitorWake]) },
      clientWithPeers()
    );

    const divider = await screen.findByTestId("session-monitor-wake");
    expect(within(divider).getByText("Release workflow publishes v0.3.0")).toBeInTheDocument();
    expect(within(divider).getByText("Process exited")).toBeInTheDocument();
    expect(screen.queryByText(/COMPOZY MONITOR/)).not.toBeInTheDocument();
  });

  it("UT-067: Should resolve the sent card's reply state from the matching reply wake", async () => {
    const sender = (withReply: string | null): SessionMessage[] => [
      {
        id: "msg-sender-turn",
        role: "assistant",
        metadata: { turn_id: "turn-send" },
        parts: [
          {
            type: "tool-compozy__session_prompt",
            toolCallId: "call-send",
            state: "output-available",
            turnId: "turn-send",
            input: { session_id: targetId, message: "Is the retry budget per request or per job?" },
            output: { status: "accepted", reply_watch: { id: "rw-6e2d81a0", state: "armed" } },
          },
          {
            type: "data-compozy-session-message",
            turnId: "turn-send",
            data: {
              tool_call_id: "call-send",
              target_session_id: targetId,
              target_workspace_id: fixtureWorkspaceId(),
              message_id: "msg-retry-q",
              mode: "queue",
              reply_watch_id: "rw-6e2d81a0",
              state: "done",
            },
          },
        ],
      } as unknown as SessionMessage,
      ...(withReply ? [replyWake("wake-reply", withReply)] : []),
    ];

    const view = renderThreadState(
      { status: "success", messages: toReadonlyThreadMessages(sender(null)) },
      clientWithPeers()
    );
    const waiting = await screen.findByRole("article", {
      name: "Sent to Billing reviewer, waiting for reply",
    });
    expect(within(waiting).getByText("Is the retry budget per request or per job?")).toBeVisible();
    expect(screen.queryByText("Used session prompt")).not.toBeInTheDocument();

    view.rerenderWith({ messages: toReadonlyThreadMessages(sender("dropped")) });
    expect(
      await screen.findByRole("article", { name: "Sent to Billing reviewer, dropped" })
    ).toBeInTheDocument();
  });

  it("Should show the message a Codex send carried in its MCP call envelope (#742)", async () => {
    renderThreadState(
      {
        status: "success",
        messages: toReadonlyThreadMessages([
          {
            id: "msg-codex-send",
            role: "assistant",
            metadata: { turn_id: "turn-send" },
            parts: [
              {
                type: "tool-mcp.compozy-hosted-tools.compozy__session_prompt",
                toolCallId: "exec-send",
                state: "output-available",
                turnId: "turn-send",
                input: {
                  server: "compozy-hosted-tools",
                  tool: "compozy__session_prompt",
                  arguments: { session_id: targetId, message: "PR opened and ready for review." },
                },
                output: { status: "queued" },
              },
              {
                type: "data-compozy-session-message",
                turnId: "turn-send",
                data: {
                  tool_call_id: "exec-send",
                  target_session_id: targetId,
                  target_workspace_id: fixtureWorkspaceId(),
                  message_id: "msg-codex",
                  mode: "queue",
                  delivery: "after_turn",
                  state: "done",
                },
              },
            ],
          } as unknown as SessionMessage,
        ]),
      },
      clientWithPeers()
    );
    const card = await screen.findByRole("article", { name: "Sent to Billing reviewer" });
    expect(within(card).getByText("PR opened and ready for review.")).toBeVisible();
  });

  it("Should show the structured denial reason on a failed Sent card", async () => {
    renderThreadState(
      {
        status: "success",
        messages: toReadonlyThreadMessages([
          {
            id: "msg-denied-send",
            role: "assistant",
            metadata: { turn_id: "turn-send" },
            parts: [
              {
                type: "tool-compozy__session_prompt",
                toolCallId: "denied-send",
                state: "output-error",
                turnId: "turn-send",
                input: { session_id: targetId, message: "ping" },
                output: {
                  error: { code: "workspace_access_denied", message: "Workspace access denied" },
                },
              },
              {
                type: "data-compozy-session-message",
                turnId: "turn-send",
                data: {
                  tool_call_id: "denied-send",
                  target_session_id: targetId,
                  target_workspace_id: fixtureWorkspaceId(),
                  message_id: "msg-denied",
                  mode: "queue",
                  state: "done",
                },
              },
            ],
          } as unknown as SessionMessage,
        ]),
      },
      clientWithPeers()
    );
    const card = await screen.findByRole("article", { name: "Could not send to Billing reviewer" });
    expect(within(card).getByText("Workspace access denied")).toBeVisible();
  });

  it("Should draw one Sent to card for a call a queued prompt split across two messages (#742)", async () => {
    // A prompt queued while the call streamed its arguments ends the first message; the call's
    // later events and result land in the continuation, so both messages hold its part.
    const send = (id: string, settled: boolean): SessionMessage =>
      ({
        id,
        role: "assistant",
        metadata: { turn_id: "turn-send" },
        parts: [
          {
            type: "tool-compozy__session_prompt",
            toolCallId: "call-send",
            state: settled ? "output-available" : "input-available",
            turnId: "turn-send",
            // The continuation may hold the result without the arguments the first message carried.
            input: settled
              ? {}
              : { session_id: targetId, message: "Is the retry budget per request or per job?" },
            ...(settled ? { output: { status: "accepted" } } : {}),
          },
          {
            type: "data-compozy-session-message",
            turnId: "turn-send",
            data: {
              tool_call_id: "call-send",
              target_session_id: targetId,
              target_workspace_id: fixtureWorkspaceId(),
              message_id: "msg-retry-q",
              mode: "queue",
              ...(settled ? { reply_watch_id: "rw-6e2d81a0" } : {}),
              state: settled ? "done" : "running",
            },
          },
        ],
      }) as unknown as SessionMessage;

    renderThreadState(
      {
        status: "success",
        messages: toReadonlyThreadMessages([send("turn-send", false), send("turn-send-2", true)]),
      },
      clientWithPeers()
    );
    const card = await screen.findByRole("article", {
      name: "Sent to Billing reviewer, waiting for reply",
    });
    expect(within(card).getByText("Is the retry budget per request or per job?")).toBeVisible();
    expect(screen.getAllByTestId("session-sent-message-row")).toHaveLength(1);
  });

  function inboundMessage(
    id: string,
    text: string,
    { from = senderId, inReplyTo }: { from?: string; inReplyTo?: string } = {}
  ): SessionMessage {
    return {
      id,
      role: "user",
      metadata: {
        turn_id: `turn-${id}`,
        message_id: id,
        origin: {
          kind: "session",
          session_id: from,
          workspace_id: fixtureWorkspaceId(),
          hop: 1,
          ...(inReplyTo ? { in_reply_to: inReplyTo } : {}),
        },
      },
      parts: [{ type: "text", text }],
    } as SessionMessage;
  }

  it("Should fold consecutive messages from one sender behind their latest card (#760)", async () => {
    const user = userEvent.setup();
    renderThreadState(
      {
        status: "success",
        messages: toReadonlyThreadMessages([
          inboundMessage("msg-update-1", "Plan drafted."),
          inboundMessage("msg-update-2", "Tests written."),
          inboundMessage("msg-update-3", "PR opened."),
        ]),
      },
      clientWithPeers()
    );

    const toggle = await screen.findByTestId("session-inbound-group-toggle");
    expect(toggle).toHaveTextContent("Refactor billing · 3 messages");
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(screen.getAllByTestId("session-message-card")).toHaveLength(1);
    expect(screen.getByText("PR opened.")).toBeVisible();
    expect(screen.queryByText("Plan drafted.")).not.toBeInTheDocument();

    await user.click(toggle);
    expect(screen.getAllByTestId("session-message-card")).toHaveLength(3);
    expect(screen.getByTestId("session-inbound-group-toggle")).toHaveAttribute(
      "aria-expanded",
      "true"
    );
  });

  it("Should link an explicit reply to its Sent card and read the send as replied (#760)", async () => {
    renderThreadState(
      {
        status: "success",
        messages: toReadonlyThreadMessages([
          {
            id: "msg-sender-turn",
            role: "assistant",
            metadata: { turn_id: "turn-send" },
            parts: [
              {
                type: "tool-compozy__session_prompt",
                toolCallId: "call-send",
                state: "output-available",
                turnId: "turn-send",
                input: { session_id: targetId, message: "Approve the plan?" },
                output: { status: "accepted", reply_watch: { id: "rw-plan", state: "armed" } },
              },
              {
                type: "data-compozy-session-message",
                turnId: "turn-send",
                data: {
                  tool_call_id: "call-send",
                  target_session_id: targetId,
                  target_workspace_id: fixtureWorkspaceId(),
                  message_id: "msg-plan",
                  mode: "queue",
                  reply_watch_id: "rw-plan",
                  state: "done",
                },
              },
            ],
          } as unknown as SessionMessage,
          inboundMessage("msg-approval", "Approved.", { from: targetId, inReplyTo: "msg-plan" }),
        ]),
      },
      clientWithPeers()
    );

    // The explicit answer reads replied while the armed watch keeps waiting for the turn to settle.
    expect(
      await screen.findByRole("article", { name: "Sent to Billing reviewer, replied" })
    ).toBeInTheDocument();
    const card = await screen.findByRole("article", { name: "Message from Billing reviewer" });
    expect(
      within(card).getByRole("button", { name: "In reply: show the original message" })
    ).toHaveAttribute("data-target-message-id", "msg-sender-turn");
  });

  it("Should read a send as replied when its answer arrived after the watch expired (#760)", async () => {
    renderThreadState(
      {
        status: "success",
        messages: toReadonlyThreadMessages([
          {
            id: "msg-sender-turn",
            role: "assistant",
            metadata: { turn_id: "turn-send" },
            parts: [
              {
                type: "tool-compozy__session_prompt",
                toolCallId: "call-send",
                state: "output-available",
                turnId: "turn-send",
                input: { session_id: targetId, message: "Approve the plan?" },
                output: { status: "accepted", reply_watch: { id: "rw-plan", state: "armed" } },
              },
              {
                type: "data-compozy-session-message",
                turnId: "turn-send",
                data: {
                  tool_call_id: "call-send",
                  target_session_id: targetId,
                  target_workspace_id: fixtureWorkspaceId(),
                  message_id: "msg-plan",
                  mode: "queue",
                  reply_watch_id: "rw-plan",
                  state: "done",
                },
              },
            ],
          } as unknown as SessionMessage,
          {
            id: "msg-expired-wake",
            role: "system",
            metadata: {
              turn_id: "turn-wake",
              synthetic: {
                kind: "session_reply",
                wake_event_id: "rw-plan",
                child_session_id: targetId,
                child_workspace_id: fixtureWorkspaceId(),
                child_agent_name: "codex",
                reason: "expired",
                summary: "",
                reply_truncated: false,
                hop: 1,
              },
            },
            parts: [{ type: "text", text: "No reply arrived in time.", state: "done" }],
          } as SessionMessage,
          inboundMessage("msg-approval", "Approved late.", {
            from: targetId,
            inReplyTo: "msg-plan",
          }),
        ]),
      },
      clientWithPeers()
    );

    expect(
      await screen.findByRole("article", { name: "Sent to Billing reviewer, replied" })
    ).toBeInTheDocument();
  });
});

// Invariant: the `@` trigger lists the session's files and same-workspace
// sessions through the daemon routes, inserts the picked target as its token,
// dismisses per occurrence, and never traps Enter when nothing is pickable
// (US-001..US-004). Owning layer: composer integration (owns `/` today).
describe("SessionThread composer mentions", () => {
  const workspaceId = fixtureWorkspaceId();
  const filesPath = `/api/workspaces/${workspaceId}/sessions/${primarySessionFixture.id}/files`;
  const otherSession = {
    ...primarySessionFixture,
    id: "sess-a",
    name: "Fix login bug",
    agent_name: "codex",
    updated_at: new Date(Date.now() - 4 * 60_000).toISOString(),
  };

  type MentionFileEntry = { path: string; name: string; kind: "file" | "folder" };

  interface MentionFetchOptions {
    files?: MentionFileEntry[];
    /** Answers per query; a pending promise holds that query's response. */
    filesFor?: (query: string) => MentionFileEntry[] | Promise<MentionFileEntry[]>;
    filesStatus?: number;
    sessionsStatus?: number;
    sessions?: (typeof primarySessionFixture)[];
  }

  function requestUrl(input: RequestInfo | URL): URL {
    const href = typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
    return new URL(href, "http://localhost");
  }

  function createMentionFetchMock(
    options: MentionFetchOptions = {},
    fileQueries: URLSearchParams[] = [],
    promptCalls: string[] = []
  ) {
    const base = createPromptRecordingFetchMock(promptCalls);
    return vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = requestUrl(input);
      if (url.pathname === filesPath) {
        fileQueries.push(url.searchParams);
        if (options.filesStatus) {
          return jsonResponse({ error: "boom" }, { status: options.filesStatus });
        }
        const entries = options.filesFor
          ? await options.filesFor(url.searchParams.get("q") ?? "")
          : (options.files ?? []);
        return jsonResponse({ entries, truncated: false });
      }
      if (
        url.pathname === "/api/sessions" &&
        url.searchParams.get("workspace_id") === workspaceId
      ) {
        if (options.sessionsStatus) {
          return jsonResponse({ error: "boom" }, { status: options.sessionsStatus });
        }
        // The catalog honors `limit` the way the daemon does.
        const limit = Number(url.searchParams.get("limit") ?? 50);
        const sessions = options.sessions ?? [primarySessionFixture, otherSession];
        return jsonResponse({
          sessions: sessions.slice(0, limit),
          page: { has_more: sessions.length > limit, limit },
        });
      }
      return base(input, init);
    });
  }

  beforeEach(() => {
    otherSession.updated_at = new Date(Date.now() - 4 * 60_000).toISOString();
    composerAui = null;
    sessionStore.trigger.composerDraftDiscarded({ sessionId: primarySessionFixture.id });
  });

  it("Should list files and other sessions and insert a picked file as its token", async () => {
    const user = userEvent.setup();
    const fileQueries: URLSearchParams[] = [];
    vi.stubGlobal(
      "fetch",
      createMentionFetchMock(
        {
          files: [
            { path: "internal/acp/client_prompt.go", name: "client_prompt.go", kind: "file" },
            { path: "web/src/", name: "src", kind: "folder" },
          ],
        },
        fileQueries
      )
    );
    renderComposer({});

    await screen.findByTestId("composer-input");
    await setComposerText("Review @clie");

    const menu = await screen.findByRole("listbox", { name: "Mentions" });
    const fileRow = await within(menu).findByRole("option", { name: /client_prompt\.go/ });
    expect(within(menu).getByTestId("composer-mention-group-files")).toHaveTextContent("Files");
    // The current session is never offered; the other one shows agent and time.
    const sessions = within(menu).getByTestId("composer-mention-group-sessions");
    await within(sessions).findByRole("option", { name: /Fix login bug/ });
    expect(within(sessions).getAllByRole("option")).toHaveLength(1);
    expect(sessions).toHaveTextContent("Codex · 4m ago");
    // Debounced: the settled query reaches the daemon with the picker's 50-row limit.
    await waitFor(() => expect(fileQueries.at(-1)?.get("q")).toBe("clie"));
    expect(fileQueries.at(-1)?.get("limit")).toBe("50");

    await user.click(fileRow);

    await waitFor(() => expect(composerText()).toBe("Review @internal/acp/client_prompt.go "));
    const chip = await screen.findByTestId("session-mention-chip");
    expect(chip).toHaveAttribute("data-mention-kind", "file");
    expect(chip).toHaveTextContent("client_prompt.go");
    expect(screen.queryByRole("listbox", { name: "Mentions" })).not.toBeInTheDocument();
    expect(
      JSON.parse(window.localStorage.getItem(`compozy.mentions.recent.${workspaceId}`) ?? "[]")
    ).toEqual([{ path: "internal/acp/client_prompt.go", kind: "file" }]);
  });

  it("Should insert a picked session as a session token with its title chip", async () => {
    const user = userEvent.setup();
    vi.stubGlobal("fetch", createMentionFetchMock());
    renderComposer({});

    await screen.findByTestId("composer-input");
    await setComposerText("Ask @");

    const menu = await screen.findByRole("listbox", { name: "Mentions" });
    await user.click(await within(menu).findByRole("option", { name: /Fix login bug/ }));
    expect(within(menu).getByTestId("composer-mention-group-files")).toHaveTextContent(
      "No matching files"
    );

    await waitFor(() => expect(composerText()).toBe("Ask @session:sess-a "));
    const chip = await screen.findByTestId("session-mention-chip");
    expect(chip).toHaveAttribute("data-mention-kind", "session");
    expect(chip).toHaveTextContent("Fix login bug");
  });

  it("Should keep a dismissed @ closed while typing and reopen for a new @", async () => {
    vi.stubGlobal(
      "fetch",
      createMentionFetchMock({
        files: [{ path: "Override.java", name: "Override.java", kind: "file" }],
      })
    );
    renderComposer({});

    const editable = await findComposerEditable();
    await setComposerText("@Over");
    expect(await screen.findByRole("listbox", { name: "Mentions" })).toBeInTheDocument();

    await act(async () => {
      fireEvent.keyDown(editable, { key: "Escape" });
    });
    await waitFor(() =>
      expect(screen.queryByTestId("composer-mention-menu")).not.toBeInTheDocument()
    );

    await setComposerText("@Override");
    await act(async () => {
      await new Promise(resolve => setTimeout(resolve, 200));
    });
    expect(screen.queryByTestId("composer-mention-menu")).not.toBeInTheDocument();

    await setComposerText("@Override @");
    expect(await screen.findByRole("listbox", { name: "Mentions" })).toBeInTheDocument();
  });

  it("Should report each failing source in its own group", async () => {
    vi.stubGlobal("fetch", createMentionFetchMock({ filesStatus: 500, sessionsStatus: 500 }));
    renderComposer({});

    await screen.findByTestId("composer-input");
    await setComposerText("@login");

    await waitFor(() => {
      const menu = screen.getByTestId("composer-mention-menu");
      expect(within(menu).getByTestId("composer-mention-group-files")).toHaveTextContent(
        "Couldn't search files"
      );
      expect(within(menu).getByTestId("composer-mention-group-sessions")).toHaveTextContent(
        "Couldn't load sessions"
      );
    });
  });

  it("Should send the prompt as typed when Enter meets a picker with no rows", async () => {
    const promptCalls: string[] = [];
    vi.stubGlobal("fetch", createMentionFetchMock({ sessions: [] }, [], promptCalls));
    renderComposer({});

    const editable = await findComposerEditable();
    await setComposerText("Check @zzq");
    await waitFor(() => {
      const menu = screen.getByTestId("composer-mention-menu");
      expect(menu).toHaveTextContent("No matching files");
      expect(menu).toHaveTextContent("No matching sessions");
    });

    await act(async () => {
      fireEvent.keyDown(editable, { key: "Enter" });
    });

    await waitFor(() => expect(promptCalls).toHaveLength(1));
    expect(promptCalls[0]).toContain("Check @zzq");
  });

  it("Should leave Enter, ArrowDown and Tab to the IME while a composition is in progress", async () => {
    vi.stubGlobal(
      "fetch",
      createMentionFetchMock({
        files: [
          { path: "README.md", name: "README.md", kind: "file" },
          { path: "docs/readme-notes.md", name: "readme-notes.md", kind: "file" },
        ],
        sessions: [],
      })
    );
    renderComposer({});

    const editable = await findComposerEditable();
    await setComposerText("@read");
    const menu = await screen.findByRole("listbox", { name: "Mentions" });
    const first = await within(menu).findByRole("option", { name: /README\.md/ });
    const second = await within(menu).findByRole("option", { name: /readme-notes\.md/ });
    // The "read" rows have landed: the first one is highlighted.
    await waitFor(() => expect(first).toHaveAttribute("data-highlighted"));

    await act(async () => {
      fireEvent.compositionStart(editable, { data: "" });
      for (const key of ["ArrowDown", "Tab", "Enter"]) {
        fireEvent.keyDown(editable, { key, isComposing: true, keyCode: 229 });
      }
    });

    // The highlight did not move and nothing was picked.
    expect(first).toHaveAttribute("data-highlighted");
    expect(second).not.toHaveAttribute("data-highlighted");
    expect(composerText()).toBe("@read");
    expect(screen.queryByTestId("session-mention-chip")).not.toBeInTheDocument();
  });

  // US-001.AC-5: ArrowUp/ArrowDown move the highlight and wrap at both ends.
  it("Should move the highlight with the arrow keys and wrap at both ends", async () => {
    vi.stubGlobal(
      "fetch",
      createMentionFetchMock({
        files: [
          { path: "README.md", name: "README.md", kind: "file" },
          { path: "docs/readme-notes.md", name: "readme-notes.md", kind: "file" },
          { path: "docs/readme-old.md", name: "readme-old.md", kind: "file" },
        ],
        sessions: [],
      })
    );
    renderComposer({});

    const editable = await findComposerEditable();
    await setComposerText("@read");
    const menu = await screen.findByRole("listbox", { name: "Mentions" });
    const rows = [
      await within(menu).findByRole("option", { name: /README\.md/ }),
      await within(menu).findByRole("option", { name: /readme-notes\.md/ }),
      await within(menu).findByRole("option", { name: /readme-old\.md/ }),
    ];
    const highlighted = () => rows.findIndex(row => row.hasAttribute("data-highlighted"));
    const press = async (key: "ArrowDown" | "ArrowUp") => {
      await act(async () => {
        fireEvent.keyDown(editable, { key });
      });
    };
    // The "read" rows have landed: the first one is highlighted.
    await waitFor(() => expect(highlighted()).toBe(0));

    await press("ArrowDown");
    await waitFor(() => expect(highlighted()).toBe(1));
    await press("ArrowUp");
    await waitFor(() => expect(highlighted()).toBe(0));
    // Wrap at the top: ArrowUp from the first row lands on the last.
    await press("ArrowUp");
    await waitFor(() => expect(highlighted()).toBe(2));
    // Wrap at the bottom: ArrowDown from the last row lands on the first.
    await press("ArrowDown");
    await waitFor(() => expect(highlighted()).toBe(0));
    expect(composerText()).toBe("@read");
  });

  // VC-04 / US-004.AC-3: the first load has nothing to pick, so the listbox
  // stays closed — the loading row shows in status mode and Enter sends.
  it("Should send on Enter while the first file search is still loading", async () => {
    const promptCalls: string[] = [];
    vi.stubGlobal(
      "fetch",
      createMentionFetchMock(
        { filesFor: () => new Promise<MentionFileEntry[]>(() => undefined), sessions: [] },
        [],
        promptCalls
      )
    );
    renderComposer({});

    const editable = await findComposerEditable();
    await setComposerText("Check @clie");
    await waitFor(() =>
      expect(screen.getByTestId("composer-mention-group-files")).toHaveTextContent("Loading files…")
    );
    // The loading row is placeholder rows in the live row's frame, announced by the status surface.
    expect(
      screen
        .getByTestId("composer-mention-group-files")
        .querySelectorAll('[data-slot="skeleton-text"]').length
    ).toBeGreaterThan(0);
    expect(screen.queryByRole("listbox", { name: "Mentions" })).not.toBeInTheDocument();
    expect(screen.getByRole("status", { name: "Mentions" })).toBeInTheDocument();

    await act(async () => {
      fireEvent.keyDown(editable, { key: "Enter" });
    });

    await waitFor(() => expect(promptCalls).toHaveLength(1));
    expect(promptCalls[0]).toContain("Check @clie");
  });

  // UT-044 (US-001.EC-6/EC-7): previous rows stay while a newer query loads,
  // and an older response that lands after a newer one is discarded.
  it("Should keep previous rows while loading and discard a stale late response", async () => {
    const entry = (path: string): MentionFileEntry => ({
      path,
      name: path.slice(path.lastIndexOf("/") + 1),
      kind: "file",
    });
    let releaseStale: (entries: MentionFileEntry[]) => void = () => undefined;
    const fileQueries: URLSearchParams[] = [];
    vi.stubGlobal(
      "fetch",
      createMentionFetchMock(
        {
          sessions: [],
          filesFor: query => {
            if (query === "client_") {
              return new Promise<MentionFileEntry[]>(resolve => {
                releaseStale = resolve;
              });
            }
            if (query === "client_p") return [entry("internal/acp/client_prompt.go")];
            return [entry("internal/acp/client.go"), entry("internal/acp/client_prompt.go")];
          },
        },
        fileQueries
      )
    );
    renderComposer({});

    await screen.findByTestId("composer-input");
    await setComposerText("@client");
    const menu = await screen.findByRole("listbox", { name: "Mentions" });
    await within(menu).findByRole("option", { name: /^client\.go/ });

    // The slow query keeps the "client" rows on screen, with no loading row.
    await setComposerText("@client_");
    await waitFor(() => expect(fileQueries.some(q => q.get("q") === "client_")).toBe(true));
    expect(within(menu).getByRole("option", { name: /^client\.go/ })).toBeInTheDocument();
    expect(menu).not.toHaveTextContent("Loading files…");
    // Those rows answer "client", not "client_": none is highlighted, and Enter neither picks
    // one nor sends the prompt while the current answer is pending.
    expect(menu).toHaveAttribute("aria-busy", "true");
    for (const option of within(menu).getAllByRole("option")) {
      expect(option).toHaveAttribute("aria-selected", "false");
    }
    await act(async () => {
      fireEvent.keyDown(await findComposerEditable(), { key: "Enter" });
    });
    expect(screen.queryByTestId("session-mention-chip")).not.toBeInTheDocument();
    expect(composerText()).toBe("@client_");

    // A newer fast query answers first.
    await setComposerText("@client_p");
    await waitFor(() =>
      expect(within(menu).queryByRole("option", { name: /^client\.go/ })).not.toBeInTheDocument()
    );
    expect(within(menu).getAllByRole("option")).toHaveLength(1);

    // The stale "client_" answer arrives late and never replaces the current rows.
    await act(async () => {
      releaseStale([entry("internal/acp/client_steer.go")]);
      await new Promise(resolve => setTimeout(resolve, 50));
    });
    expect(
      within(menu).queryByRole("option", { name: /client_steer\.go/ })
    ).not.toBeInTheDocument();
    const current = within(menu).getByRole("option", { name: /client_prompt\.go/ });
    // The current query's own row is highlighted, and Enter picks exactly it.
    expect(menu).not.toHaveAttribute("aria-busy");
    expect(current).toHaveAttribute("aria-selected", "true");
    await act(async () => {
      fireEvent.keyDown(await findComposerEditable(), { key: "Enter" });
    });
    await waitFor(() =>
      expect(screen.getByTestId("session-mention-chip")).toHaveAttribute(
        "data-mention-target",
        "internal/acp/client_prompt.go"
      )
    );
  });

  // UT-042: the search waits out the 120 ms debounce before asking the daemon.
  it("Should debounce the file search for 120 ms before requesting it", async () => {
    const fileQueries: URLSearchParams[] = [];
    vi.stubGlobal("fetch", createMentionFetchMock({ sessions: [] }, fileQueries));
    renderComposer({});
    await findComposerEditable();

    vi.useFakeTimers();
    try {
      await setComposerText("@clie");
      const clieRequests = () => fileQueries.filter(q => q.get("q") === "clie").length;
      await act(async () => {
        await vi.advanceTimersByTimeAsync(119);
      });
      expect(clieRequests()).toBe(0);
      await act(async () => {
        await vi.advanceTimersByTimeAsync(1);
      });
      await act(async () => {
        await vi.advanceTimersByTimeAsync(20);
      });
      expect(clieRequests()).toBe(1);
      expect(fileQueries.find(q => q.get("q") === "clie")?.get("limit")).toBe("50");
    } finally {
      vi.useRealTimers();
    }
  });

  // UT-048 (US-003.AC-1/4): at most five sessions, never the current one.
  it("Should list at most five other sessions and never the current one", async () => {
    const sessions: (typeof primarySessionFixture)[] = Array.from({ length: 6 }, (_, index) => ({
      ...otherSession,
      id: `sess-${index + 1}`,
      name: `Session ${index + 1}`,
    }));
    sessions.splice(1, 0, primarySessionFixture);
    vi.stubGlobal("fetch", createMentionFetchMock({ sessions }));
    renderComposer({});

    await screen.findByTestId("composer-input");
    await setComposerText("@");

    // The group remounts when the listbox opens, so read it once rows are listed.
    const listbox = await screen.findByRole("listbox", { name: "Mentions" });
    const group = within(listbox).getByTestId("composer-mention-group-sessions");
    await waitFor(() => expect(within(group).getAllByRole("option")).toHaveLength(5));
    expect(
      within(group)
        .getAllByRole("option")
        .map(option => option.textContent)
    ).toEqual([1, 2, 3, 4, 5].map(index => expect.stringContaining(`Session ${index}`)));
    expect(group).not.toHaveTextContent(primarySessionFixture.name ?? primarySessionFixture.id);
  });

  // UT-047 (US-002.AC-4, VC-16): Backspace right after a chip removes it whole.
  it("Should remove a picked mention chip as a unit with Backspace", async () => {
    const user = userEvent.setup();
    vi.stubGlobal(
      "fetch",
      createMentionFetchMock({
        files: [{ path: "internal/acp/client_prompt.go", name: "client_prompt.go", kind: "file" }],
        sessions: [],
      })
    );
    renderComposer({});

    const editable = await findComposerEditable();
    await setComposerText("Review @clie");
    const menu = await screen.findByRole("listbox", { name: "Mentions" });
    await user.click(await within(menu).findByRole("option", { name: /client_prompt\.go/ }));
    await waitFor(() => expect(composerText()).toBe("Review @internal/acp/client_prompt.go "));

    // Drop the boundary space so the caret sits right after the chip (VC-16).
    // jsdom has no Selection.modify, so a character-wise Backspace over plain
    // text cannot run here; the chip's removal is pure editor-model logic.
    const editor = getNearestEditorFromDOMNode(editable);
    if (!editor) throw new Error("composer editor is not mounted");
    await act(async () => {
      editor.update(
        () => {
          const paragraph = $getRoot().getFirstChildOrThrow<ElementNode>();
          const boundary = paragraph.getLastChild();
          if ($isTextNode(boundary) && boundary.getTextContent() === " ") boundary.remove();
          paragraph.selectEnd();
        },
        { discrete: true }
      );
    });
    await waitFor(() => expect(composerText()).toBe("Review @internal/acp/client_prompt.go"));

    await act(async () => {
      fireEvent.keyDown(editable, { key: "Backspace", code: "Backspace", keyCode: 8 });
    });

    await waitFor(() => expect(composerText()).toBe("Review "));
    expect(screen.queryByTestId("session-mention-chip")).not.toBeInTheDocument();
  });

  // Invariant: the `/` scope and `@` trigger detectors share one text/caret
  // walk per committed editor update, and an update that changes neither the
  // text nor the selection walks nothing — while both triggers keep working.
  it("Should read the composer text once per update for both / and @ detectors", async () => {
    const fileQueries: URLSearchParams[] = [];
    vi.stubGlobal(
      "fetch",
      createMentionFetchMock(
        { files: [{ path: "Override.java", name: "Override.java", kind: "file" }] },
        fileQueries
      )
    );
    renderComposer({
      commandCatalog: {
        standaloneSections: [
          {
            id: "built-in",
            label: "Built-in",
            commands: [{ id: "goal", token: "/goal", label: "Goal", lane: "builtin" as const }],
          },
        ],
        inlineSkills: [
          {
            id: "frontend-qa",
            token: "/frontend-qa",
            label: "Frontend QA",
            lane: "skill" as const,
          },
        ],
      },
    });

    const editable = await findComposerEditable();
    const editor = getNearestEditorFromDOMNode(editable);
    if (!editor) throw new Error("composer editor is not mounted");
    const reads = vi.mocked(readCursorSnapshot);
    // Registered after the composer plugins, so it runs after both detectors and
    // attributes the walks of each committed update to that update's tag.
    const readsByTag = new Map<string, number>();
    const walksPerUpdate: number[] = [];
    let readsSeen = reads.mock.calls.length;
    const unregister = editor.registerUpdateListener(({ tags }) => {
      const walks = reads.mock.calls.length - readsSeen;
      readsSeen = reads.mock.calls.length;
      walksPerUpdate.push(walks);
      for (const tag of tags) readsByTag.set(tag, walks);
    });
    const commit = async (tag: string, fn: () => void) => {
      await act(async () => {
        editor.update(fn, { discrete: true, tag });
      });
      return readsByTag.get(tag);
    };

    await setComposerText("@Ov");
    expect(await screen.findByRole("listbox", { name: "Mentions" })).toBeInTheDocument();

    // One text edit: one walk serves both detectors and the picker follows it.
    const edited = await commit("test-edit", () => {
      const paragraph = $getRoot().getFirstChildOrThrow<ElementNode>();
      const text = paragraph.getLastChild();
      if ($isTextNode(text)) text.setTextContent("@Over");
      paragraph.selectEnd();
    });
    expect(edited).toBe(1);
    await waitFor(() => expect(fileQueries.at(-1)?.get("q")).toBe("Over"));

    // Re-selecting the caret exactly where it is changes nothing: no walk.
    const unchanged = await commit("test-noop", () => {
      const selection = $getSelection();
      if (selection) $setSelection(selection.clone());
    });
    expect(unchanged).toBe(0);

    // A selection-only move re-detects with one walk: the picker searches "O".
    const moved = await commit("test-move", () => {
      const text = $getRoot().getFirstChildOrThrow<ElementNode>().getFirstChild();
      if ($isTextNode(text)) text.select(2, 2);
    });
    expect(moved).toBe(1);
    await waitFor(() => expect(fileQueries.at(-1)?.get("q")).toBe("O"));

    // The `/` detector still scopes from the shared walk: mid-text is inline-only.
    await setComposerText("@Over /");
    const menu = await screen.findByTestId("composer-command-menu");
    expect(within(menu).getByRole("option", { name: /Frontend QA/i })).toBeInTheDocument();
    expect(within(menu).queryByRole("option", { name: /Goal/i })).not.toBeInTheDocument();
    unregister();
    expect(Math.max(...walksPerUpdate)).toBe(1);
  });
});

// Invariant: an agent gone from the catalog keeps its chip, marked unresolved, and a live one wears
// its current label. Owner: the mention chip; canonical suite: this file.
describe("SessionMentionChip agent kind", () => {
  function renderChip(agents: unknown[]) {
    const queryClient = createQueryClient();
    queryClient.setQueryData(agentKeys.list(null, "default"), agents);
    render(
      <QueryClientProvider client={queryClient}>
        <AgentIdentityProvider>
          <UIProvider>
            <SessionMentionChip kind="agent" label="Release Captain" target="release_captain" />
          </UIProvider>
        </AgentIdentityProvider>
      </QueryClientProvider>
    );
    return screen.getByTestId("session-mention-chip");
  }

  it("Should show the avatar and current label of an agent the catalog holds", () => {
    const chip = renderChip([
      { name: "release_captain", label: "Ship Captain", resolved_appearance: null },
    ]);

    expect(chip).toHaveTextContent("Ship Captain");
    expect(chip).not.toHaveAttribute("data-mention-unresolved");
    expect(chip.querySelector('[data-slot="agent-avatar"]')).not.toBeNull();
  });

  it("Should mark a deleted agent's chip unresolved and keep its name", () => {
    const chip = renderChip([]);

    expect(chip).toHaveAttribute("data-mention-unresolved", "true");
    expect(chip).toHaveTextContent("Release Captain");
  });
});
