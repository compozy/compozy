import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useAui, useAuiState, type ThreadMessage } from "@assistant-ui/react";
import { StrictMode, useEffect, useLayoutEffect, useState } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { observeGatewayAccess, type GatewayAccessSignal } from "@/lib/gateway-access-signal";
import { resetGatewayStreamAuth } from "@/lib/gateway-stream-auth";
import { SessionThread } from "@/components/assistant-ui/session-thread";
import { createSessionPromptDispatchStore } from "@/components/assistant-ui/session-prompt-dispatch-store";
import { Toaster } from "@compozy/ui";
import { formatMessageError } from "@/components/assistant-ui/session-thread-error";
import { SessionTranscriptThreadProvider } from "@/systems/session/lib/session-transcript-thread-context";

import { sessionStore } from "@/systems/session/stores/session-store";
import {
  useSessionTranscriptThreadMessages,
  useSessionTranscriptThreadStatus,
} from "@/systems/session/hooks/use-session-transcript-thread-messages";
import { mergeSessionThreadReadModel } from "@/systems/session/lib/session-thread-read-model";
import { toReadonlyThreadMessages } from "@/systems/session/lib/session-thread-repository";
import type { SessionTranscriptData } from "@/systems/session/lib/session-transcript-query";
import type { SessionTranscriptThreadStatus } from "@/systems/session/lib/session-transcript-thread-context-value";
import { primarySessionFixture, sessionTranscriptFixture } from "@/systems/session/mocks/fixtures";
import type { TranscriptMessage } from "@/systems/session/types";

import { SessionChatRuntimeProvider } from "../session-chat-runtime-provider";
import {
  SessionPromptRuntimeContext,
  type SessionPromptRuntimeSnapshot,
} from "../../contexts/session-prompt-runtime-context-value";
import {
  sessionPromptRuntimeInput,
  sessionPromptRuntimeStoreLogic,
} from "../../stores/session-prompt-runtime-store";
import {
  sessionKeys,
  useClearSessionConversation,
  useSessionTranscriptThreadState,
} from "@/systems/session";

const SESSION_STREAM_QUERY = "frames=transcript";

describe("formatMessageError", () => {
  it("extracts provider failure detail from JSON-RPC error envelopes", () => {
    expect(
      formatMessageError(
        '{"code":-32603,"message":"Internal error","data":{"error":"peer disconnected before response"}}'
      )
    ).toBe("peer disconnected before response");
  });

  it("does not produce empty message chrome for blank provider errors", () => {
    expect(formatMessageError("")).toBeNull();
  });

  it("does not render raw JSON when no provider error detail exists", () => {
    expect(formatMessageError('{"type":"abort"}')).toBeNull();
  });
});

function jsonResponse(body: unknown, init?: ResponseInit) {
  return new Response(JSON.stringify(body), {
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

function transcriptEntries(messages: TranscriptMessage[], firstSequence = 1) {
  return messages.map((message, index) => ({
    message,
    sequence: firstSequence + index,
    start_sequence: firstSequence + index,
  }));
}

function transcriptPayload(
  messages: TranscriptMessage[],
  options: {
    firstSequence?: number;
    hasOlder?: boolean;
    nextBeforeSequence?: number;
  } = {}
) {
  const entries = transcriptEntries(messages, options.firstSequence);
  return {
    entries,
    epoch: 1,
    generation: 1,
    has_older: options.hasOlder ?? false,
    limit: 200,
    max_sequence: entries.at(-1)?.sequence ?? 0,
    ...(options.nextBeforeSequence === undefined
      ? {}
      : { next_before_sequence: options.nextBeforeSequence }),
  };
}

function transcriptCache(messages: TranscriptMessage[]): SessionTranscriptData {
  const payload = transcriptPayload(messages);
  return {
    pages: [{ ...payload, cursor: payload.max_sequence }],
    pageParams: [undefined],
  };
}

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

function openSseResponse(frames: string[]) {
  const encoder = new TextEncoder();
  let streamController!: ReadableStreamDefaultController<Uint8Array>;
  const response = new Response(
    new ReadableStream({
      start(controller) {
        streamController = controller;
        for (const frame of frames) {
          controller.enqueue(encoder.encode(frame));
        }
      },
    }),
    {
      headers: {
        "Content-Type": "text/event-stream",
        "x-vercel-ai-ui-message-stream": "v1",
      },
    }
  );
  return {
    response,
    close(framesToFinish: string[]) {
      for (const frame of framesToFinish) {
        streamController.enqueue(encoder.encode(frame));
      }
      streamController.close();
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

function getRequestURL(input: RequestInfo | URL): URL {
  if (typeof input === "string") return new URL(input, "http://localhost");
  if (input instanceof URL) return input;
  return new URL(input.url, "http://localhost");
}

function getLastUserMessageID(init?: RequestInit): string {
  if (typeof init?.body !== "string") return "";
  const body: unknown = JSON.parse(init.body);
  if (typeof body !== "object" || body === null || !("messages" in body)) return "";
  const messages = body.messages;
  if (!Array.isArray(messages)) return "";
  for (const message of [...messages].reverse()) {
    if (
      typeof message === "object" &&
      message !== null &&
      "role" in message &&
      message.role === "user" &&
      "id" in message &&
      typeof message.id === "string"
    ) {
      return message.id;
    }
  }
  return "";
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

function createDeferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((promiseResolve, promiseReject) => {
    resolve = promiseResolve;
    reject = promiseReject;
  });
  return { promise, reject, resolve };
}

function abortableResponse(response: Promise<Response>, signal?: AbortSignal | null) {
  if (!signal) return response;
  if (signal.aborted) {
    return Promise.reject(new DOMException("The operation was aborted", "AbortError"));
  }

  return new Promise<Response>((resolve, reject) => {
    const handleAbort = () => {
      reject(new DOMException("The operation was aborted", "AbortError"));
    };
    signal.addEventListener("abort", handleAbort, { once: true });
    void response.then(
      result => {
        signal.removeEventListener("abort", handleAbort);
        resolve(result);
      },
      error => {
        signal.removeEventListener("abort", handleAbort);
        reject(error);
      }
    );
  });
}

class FakeSessionEventSource {
  readonly listeners = new Map<string, Set<EventListenerOrEventListenerObject>>();
  closed = false;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;

  constructor(readonly url: string) {}

  addEventListener(type: string, listener: EventListenerOrEventListenerObject) {
    const listeners = this.listeners.get(type) ?? new Set<EventListenerOrEventListenerObject>();
    listeners.add(listener);
    this.listeners.set(type, listeners);
  }

  removeEventListener(type: string, listener: EventListenerOrEventListenerObject) {
    this.listeners.get(type)?.delete(listener);
  }

  close() {
    this.closed = true;
  }

  dispatch(type: string, payload: unknown, lastEventId: string) {
    const event = new MessageEvent(type, { data: JSON.stringify(payload) });
    Object.defineProperty(event, "lastEventId", { value: lastEventId });
    if (type === "message") {
      this.onmessage?.(event);
    }
    for (const listener of this.listeners.get(type) ?? []) {
      if (typeof listener === "function") {
        listener(event);
      } else {
        listener.handleEvent(event);
      }
    }
  }
}

function TranscriptStateProbe() {
  const state = useSessionTranscriptThreadState();

  return (
    <div
      data-testid="transcript-state-probe"
      data-status={state.status}
      data-is-error={String(state.isError)}
      data-is-pending={String(state.isPending)}
      data-error-message={state.error?.message ?? ""}
    >
      {state.messages.length}
    </div>
  );
}

const TranscriptMessagesSubscriptionProbe = vi.fn(function TranscriptMessagesSubscriptionProbe() {
  const messages = useSessionTranscriptThreadMessages();
  return <output data-testid="transcript-messages-subscription-probe">{messages.length}</output>;
});

const TranscriptStatusSubscriptionProbe = vi.fn(function TranscriptStatusSubscriptionProbe() {
  const status = useSessionTranscriptThreadStatus();
  return <output data-testid="transcript-status-subscription-probe">{status}</output>;
});

interface RuntimeTranscriptCommit {
  projectedIds: string[];
  runtimeIds: string[];
}

function RuntimeTranscriptCoherenceProbe({
  onCommit,
}: {
  onCommit: (sample: RuntimeTranscriptCommit) => void;
}) {
  const runtimeMessages = useAuiState(state => state.thread.messages);
  const projectedMessages = useSessionTranscriptThreadMessages();

  useLayoutEffect(() => {
    onCommit({
      projectedIds: projectedMessages.map(message => message.id),
      runtimeIds: runtimeMessages.map(message => message.id),
    });
  }, [onCommit, projectedMessages, runtimeMessages]);

  return null;
}

function ClearConversationButton() {
  const aui = useAui();
  const clearMutation = useClearSessionConversation({ workspaceId: fixtureWorkspaceId() });

  return (
    <button
      type="button"
      disabled={clearMutation.isPending}
      onClick={() => {
        clearMutation.mutate(primarySessionFixture.id, {
          onSuccess: () => aui.thread.reset(),
        });
      }}
    >
      Clear test conversation
    </button>
  );
}

function RetryLatestAssistantMessageButton() {
  const aui = useAui();
  const latestAssistantMessageId = useAuiState(state => {
    for (let index = state.thread.messages.length - 1; index >= 0; index -= 1) {
      const message = state.thread.messages[index];
      if (message?.role === "assistant") {
        return message.id;
      }
    }
    return null;
  });

  if (latestAssistantMessageId === null) {
    return null;
  }

  return (
    <button
      type="button"
      onClick={async () => {
        await aui.thread.message({ id: latestAssistantMessageId }).reload();
      }}
    >
      Retry latest assistant message
    </button>
  );
}

// The composer input is a Lexical contenteditable, so authored text lives in the
// runtime composer rather than a textarea `value`. The probe exposes the same `aui`
// handle the composer uses, so tests can author and read canonical prompt text.
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

function renderSessionThread(
  options: {
    eventSourceFactory?: (url: string) => FakeSessionEventSource;
    includeClearAction?: boolean;
    includeRuntimeRetryControl?: boolean;
    includeToaster?: boolean;
    queryClient?: QueryClient;
    includeTranscriptStateProbe?: boolean;
    includeDecisionDock?: boolean;
    isSessionRunning?: boolean;
    liveTailEnabled?: boolean;
    onCancelPrompt?: () => void;
    onRuntimeTranscriptCommit?: (sample: RuntimeTranscriptCommit) => void;
    runtimeSnapshot?: SessionPromptRuntimeSnapshot;
    strictMode?: boolean;
  } = {}
) {
  const queryClient = options.queryClient ?? createQueryClient();

  const renderTree = () => {
    const sessionRuntime = (
      <QueryClientProvider client={queryClient}>
        <SessionChatRuntimeProvider
          sessionId={primarySessionFixture.id}
          workspaceId={fixtureWorkspaceId()}
          eventSourceFactory={options.eventSourceFactory}
          liveTailEnabled={options.liveTailEnabled}
        >
          <ComposerAuiProbe />
          {options.onRuntimeTranscriptCommit ? (
            <RuntimeTranscriptCoherenceProbe onCommit={options.onRuntimeTranscriptCommit} />
          ) : null}
          {options.includeTranscriptStateProbe ? <TranscriptStateProbe /> : null}
          {options.includeClearAction ? <ClearConversationButton /> : null}
          {options.includeRuntimeRetryControl ? <RetryLatestAssistantMessageButton /> : null}
          <SessionThread
            sessionId={primarySessionFixture.id}
            agentName={primarySessionFixture.agent_name}
            workspaceId={options.includeDecisionDock ? fixtureWorkspaceId() : undefined}
            sessionState={options.includeDecisionDock ? "active" : undefined}
            canPrompt
            isSessionRunning={options.isSessionRunning}
            onCancelPrompt={options.onCancelPrompt ?? (() => {})}
          />
          {options.includeToaster ? <Toaster duration={500} /> : null}
        </SessionChatRuntimeProvider>
      </QueryClientProvider>
    );
    const tree = options.runtimeSnapshot
      ? withRuntimeSnapshot(sessionRuntime, options.runtimeSnapshot)
      : sessionRuntime;
    return options.strictMode ? <StrictMode>{tree}</StrictMode> : tree;
  };
  const utils = render(renderTree());
  return {
    ...utils,
    queryClient,
    rerenderSessionThread: () => utils.rerender(renderTree()),
  };
}

function withRuntimeSnapshot(
  children: React.ReactNode,
  runtimeSnapshot: SessionPromptRuntimeSnapshot
) {
  const store = sessionPromptRuntimeStoreLogic.createStore(
    sessionPromptRuntimeInput({
      agentName: primarySessionFixture.agent_name,
      canPrompt: true,
      effectiveRuntime: primarySessionFixture.runtime.effective,
      selectedRuntime: primarySessionFixture.runtime.selected,
      selectionRevision: primarySessionFixture.runtime.selection_revision,
      sessionId: primarySessionFixture.id,
      workspaceId: primarySessionFixture.workspace_id,
    })
  );
  store.trigger.runtimeSelected({
    value: {
      provider: runtimeSnapshot.provider,
      model: runtimeSnapshot.model ?? "",
      reasoning_effort: runtimeSnapshot.reasoning_effort ?? "",
    },
  });
  if (runtimeSnapshot.speed === "fast") {
    store.trigger.speedSelected({ speed: "fast" });
  }
  return <SessionPromptRuntimeContext value={store}>{children}</SessionPromptRuntimeContext>;
}

function countTranscriptFetches(fetchMock: ReturnType<typeof vi.fn>): number {
  return fetchMock.mock.calls.filter(([input]) => {
    return (
      getPathname(input as RequestInfo | URL) ===
      `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/${primarySessionFixture.id}/transcript`
    );
  }).length;
}

function countPromptFetches(fetchMock: ReturnType<typeof vi.fn>): number {
  return fetchMock.mock.calls.filter(([input]) => {
    return (
      getPathname(input as RequestInfo | URL) ===
      `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/${primarySessionFixture.id}/prompt`
    );
  }).length;
}

function fixtureWorkspaceId(): string {
  const workspaceId = primarySessionFixture.workspace_id;
  if (!workspaceId) {
    throw new Error("primary session fixture must include workspace_id");
  }
  return workspaceId;
}

describe("SessionChatRuntimeProvider", () => {
  // Invariant: a replacement prompt request cancels the previous in-flight request,
  // so stale transport work cannot outlive the current prompt. Owning layer: chat-runtime
  // provider integration. Canonical suite: this file.
  it("Should abort a superseded prompt request", () => {
    const store = createSessionPromptDispatchStore();
    const first = new AbortController();
    const second = new AbortController();

    store.trigger.requestStarted({ controller: first });
    store.trigger.requestStarted({ controller: second });

    expect(first.signal.aborted).toBe(true);
    expect(store.getSnapshot().context).toMatchObject({
      controller: second,
      generation: 1,
      pending: true,
    });
  });

  // Invariant: message subscribers ignore status and pagination-only transcript
  // synchronization, while status subscribers ignore message-only synchronization.
  // Owning layer: provider/thread integration. Canonical suite: this file.
  it("isolates transcript subscribers from unrelated synchronized projection fields", async () => {
    const initialMessages = toReadonlyThreadMessages(sessionTranscriptFixture.slice(0, 1));
    const nextMessages = toReadonlyThreadMessages(sessionTranscriptFixture.slice(0, 2));
    TranscriptMessagesSubscriptionProbe.mockClear();
    TranscriptStatusSubscriptionProbe.mockClear();
    const messageCommits = TranscriptMessagesSubscriptionProbe;
    const statusCommits = TranscriptStatusSubscriptionProbe;
    const retry = vi.fn();

    function TranscriptSubscriptionHarness({
      hasOlder,
      isFetchingOlder,
      messages,
      status,
    }: {
      hasOlder: boolean;
      isFetchingOlder: boolean;
      messages: readonly ThreadMessage[];
      status: SessionTranscriptThreadStatus;
    }) {
      const [children] = useState(() => (
        <>
          <TranscriptMessagesSubscriptionProbe />
          <TranscriptStatusSubscriptionProbe />
        </>
      ));

      return (
        <SessionTranscriptThreadProvider
          messages={messages}
          status={status}
          error={null}
          hasOlder={hasOlder}
          isFetchingOlder={isFetchingOlder}
          retry={retry}
        >
          {children}
        </SessionTranscriptThreadProvider>
      );
    }

    const view = render(
      <TranscriptSubscriptionHarness
        messages={initialMessages}
        status="success"
        hasOlder={false}
        isFetchingOlder={false}
      />
    );

    expect(messageCommits).toHaveBeenCalledTimes(1);
    expect(statusCommits).toHaveBeenCalledTimes(1);

    view.rerender(
      <TranscriptSubscriptionHarness
        messages={initialMessages}
        status="pending"
        hasOlder={false}
        isFetchingOlder={false}
      />
    );
    await waitFor(() => expect(statusCommits).toHaveBeenCalledTimes(2));
    expect(messageCommits).toHaveBeenCalledTimes(1);

    view.rerender(
      <TranscriptSubscriptionHarness
        messages={initialMessages}
        status="pending"
        hasOlder
        isFetchingOlder
      />
    );
    await waitFor(() => {
      expect(screen.getByTestId("transcript-status-subscription-probe")).toHaveTextContent(
        "pending"
      );
    });
    expect(messageCommits).toHaveBeenCalledTimes(1);
    expect(statusCommits).toHaveBeenCalledTimes(2);

    view.rerender(
      <TranscriptSubscriptionHarness
        messages={nextMessages}
        status="pending"
        hasOlder
        isFetchingOlder
      />
    );
    await waitFor(() => expect(messageCommits).toHaveBeenCalledTimes(2));
    expect(statusCommits).toHaveBeenCalledTimes(2);
  });

  it("Should align viewport and composer on the shared thread content rail", async () => {
    renderSessionThread();

    await waitFor(() => {
      expect(screen.getByTestId("chat-view")).toBeInTheDocument();
    });

    const chatView = screen.getByTestId("chat-view");
    const viewportRail = within(chatView).getByTestId("thread-content-rail");
    expect(viewportRail).toHaveClass("px-4", "w-full");
    expect(viewportRail).not.toHaveClass("mx-auto");
    expect(viewportRail.className).not.toContain("max-w-");

    const composerShell = screen.getByTestId("composer-shell");
    const composerRail = within(composerShell).getByTestId("thread-content-rail");
    expect(composerRail).toHaveClass("px-4", "w-full");
    expect(composerRail).not.toHaveClass("mx-auto");
    expect(composerRail.className).not.toContain("max-w-");
  });

  let transcriptMessages = sessionTranscriptFixture.slice(0, 2);
  let sessionDetailResponse = primarySessionFixture;
  let transcriptEpoch = 1;
  let transcriptGeneration = 1;
  let transcriptFetchShouldFail = false;
  let transcriptResponsePromise: Promise<Response> | null = null;
  let clearResponsePromise: Promise<Response> | null = null;
  let promptResponsePromise: Promise<Response> | null = null;
  let olderTranscriptResponsePromise: Promise<Response> | null = null;
  let transcriptFirstSequence = 1;
  let transcriptHasOlder = false;
  let transcriptNextBeforeSequence: number | undefined;
  let olderTranscriptMessages: TranscriptMessage[] = [];
  let goalCommandFailureReason: "goal_objective_required" | "goal_objective_too_large" | null =
    null;
  let lastPromptMessageID = "";
  let promptResponseSequence = 0;
  let streamTickets: string[] = [];
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    composerAui = null;
    sessionStore.trigger.sessionInteractionRemoved({ sessionId: primarySessionFixture.id });
    transcriptMessages = sessionTranscriptFixture.slice(0, 2);
    sessionDetailResponse = primarySessionFixture;
    transcriptEpoch = 1;
    transcriptGeneration = 1;
    transcriptFetchShouldFail = false;
    transcriptResponsePromise = null;
    clearResponsePromise = null;
    promptResponsePromise = null;
    olderTranscriptResponsePromise = null;
    transcriptFirstSequence = 1;
    transcriptHasOlder = false;
    transcriptNextBeforeSequence = undefined;
    olderTranscriptMessages = [];
    goalCommandFailureReason = null;
    lastPromptMessageID = "";
    promptResponseSequence = 0;
    streamTickets = [];
    fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const requestURL = getRequestURL(input);
      const pathname = requestURL.pathname;

      if (pathname === "/api/status") {
        return jsonResponse(
          {},
          {
            headers: {
              "Content-Type": "application/json",
              "X-Compozy-Gateway-Tier": streamTickets.length > 0 ? "private" : "local",
            },
          }
        );
      }

      if (pathname === "/api/gateway/stream-tickets") {
        const ticket = streamTickets.shift();
        if (ticket) {
          return jsonResponse(
            { ticket, expires_at: "2026-08-07T00:00:00Z" },
            {
              status: 201,
              headers: {
                "Content-Type": "application/json",
                "X-Compozy-Gateway-Tier": "private",
              },
            }
          );
        }
        throw new Error("Remote stream ticket requested without a queued test ticket");
      }

      if (pathname === "/api/sessions") {
        return jsonResponse({
          sessions: [primarySessionFixture],
          page: { has_more: false, limit: 50, total: 1 },
        });
      }

      if (
        pathname ===
        `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/${primarySessionFixture.id}`
      ) {
        return jsonResponse({ session: sessionDetailResponse });
      }

      if (
        pathname ===
        `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/${primarySessionFixture.id}/transcript`
      ) {
        if (transcriptResponsePromise) {
          return transcriptResponsePromise;
        }
        if (transcriptFetchShouldFail) {
          return jsonResponse({ error: "recorder temporarily unavailable" }, { status: 500 });
        }
        if (requestURL.searchParams.has("before_sequence")) {
          if (olderTranscriptResponsePromise) return olderTranscriptResponsePromise;
          return jsonResponse(transcriptPayload(olderTranscriptMessages));
        }
        return jsonResponse({
          ...transcriptPayload(transcriptMessages, {
            firstSequence: transcriptFirstSequence,
            hasOlder: transcriptHasOlder,
            nextBeforeSequence: transcriptNextBeforeSequence,
          }),
          epoch: transcriptEpoch,
          generation: transcriptGeneration,
        });
      }

      if (
        pathname ===
        `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/${primarySessionFixture.id}/clarifications`
      ) {
        return jsonResponse({ clarifications: [] });
      }

      if (
        pathname ===
        `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/${primarySessionFixture.id}/goal`
      ) {
        return jsonResponse({ goal: null });
      }

      if (
        pathname ===
        `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/${primarySessionFixture.id}/clear`
      ) {
        if (clearResponsePromise) return clearResponsePromise;
        return jsonResponse({ session: sessionDetailResponse });
      }

      if (
        pathname ===
        `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/${primarySessionFixture.id}/attachments`
      ) {
        return jsonResponse(
          {
            attachment: {
              bytes: 32,
              created_at: "2026-08-15T00:00:00Z",
              height: 10,
              id: `att_${"a".repeat(64)}`,
              kind: "file",
              mime_type: "application/pdf",
              name: "flatten-spec.pdf",
              sha256: "b".repeat(64),
              width: 0,
            },
          },
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

      if (
        pathname ===
        `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/${primarySessionFixture.id}/prompt`
      ) {
        lastPromptMessageID = getLastUserMessageID(init);
        if (promptResponsePromise) {
          return abortableResponse(promptResponsePromise, init?.signal);
        }
        if (goalCommandFailureReason) {
          return jsonResponse(
            {
              prompt: {
                goal: {
                  outcome: "error",
                  reason_code: goalCommandFailureReason,
                  replaced_run_id: null,
                  snapshot: null,
                },
                idempotency_key: "idempotency-goal-error",
                message_id: lastPromptMessageID,
                replayed: false,
                status: "rejected",
              },
            },
            { status: 422 }
          );
        }
        promptResponseSequence += 1;
        const runtimeMessageID = `turn-runtime-${String(promptResponseSequence).padStart(3, "0")}`;
        const runtimeTextID = `${runtimeMessageID}-text-1`;
        return sseResponse([
          `data: {"type":"start","messageId":"${runtimeMessageID}"}\n\n`,
          `data: {"type":"text-start","id":"${runtimeTextID}"}\n\n`,
          `data: {"type":"text-delta","id":"${runtimeTextID}","delta":"Live runtime answer before transcript reconciliation."}\n\n`,
          `data: {"type":"text-end","id":"${runtimeTextID}"}\n\n`,
          'data: {"type":"finish","finishReason":"stop"}\n\n',
          "data: [DONE]\n\n",
        ]);
      }

      throw new Error(`Unhandled fetch in test: ${pathname}`);
    });

    vi.stubGlobal("fetch", fetchMock);
    resetGatewayStreamAuth();
  });

  afterEach(() => {
    vi.useRealTimers();
    resetGatewayStreamAuth();
    vi.unstubAllGlobals();
    act(() => {
      sessionStore.trigger.sessionInteractionRemoved({ sessionId: primarySessionFixture.id });
    });
  });

  it("Should unmount a running readonly thread safely under StrictMode", async () => {
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => undefined);
    try {
      const view = renderSessionThread({ isSessionRunning: true, strictMode: true });

      expect(
        await screen.findByText("Summarize the launch blockers before the 18:30 UTC cutover.")
      ).toBeInTheDocument();
      expect(() => view.unmount()).not.toThrow();
      expect(
        consoleError.mock.calls.some(call =>
          call.some(value =>
            String(value).includes("Tried to unmount a fiber that is already unmounted")
          )
        )
      ).toBe(false);
    } finally {
      consoleError.mockRestore();
    }
  });

  it("Should still render from cache after fake timers advance beyond the old 5-minute gcTime default", async () => {
    vi.useFakeTimers();
    const queryClient = createQueryClient();
    const workspaceId = fixtureWorkspaceId();
    const sessionId = primarySessionFixture.id;
    queryClient.setQueryData(sessionKeys.detail(workspaceId, sessionId), primarySessionFixture);
    const cachedTranscript = transcriptCache(transcriptMessages);
    queryClient.setQueryData(sessionKeys.transcript(workspaceId, sessionId), cachedTranscript);

    const firstRender = renderSessionThread({ queryClient });

    await act(async () => {
      await vi.waitFor(() => {
        expect(
          screen.getByText("Summarize the launch blockers before the 18:30 UTC cutover.")
        ).toBeInTheDocument();
        expect(screen.getByText("Launch readiness snapshot")).toBeInTheDocument();
      });
    });

    firstRender.unmount();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(5 * 60 * 1_000 + 1);
    });

    expect(queryClient.getQueryData(sessionKeys.transcript(workspaceId, sessionId))).toEqual(
      cachedTranscript
    );

    renderSessionThread({ queryClient });

    await act(async () => {
      await vi.waitFor(() => {
        expect(screen.queryByTestId("thread-transcript-skeleton")).not.toBeInTheDocument();
        expect(screen.queryByText("Start a conversation.")).not.toBeInTheDocument();
        expect(
          screen.getByText("Summarize the launch blockers before the 18:30 UTC cutover.")
        ).toBeInTheDocument();
        expect(screen.getByText("Launch readiness snapshot")).toBeInTheDocument();
      });
    });
  }, 10_000);

  it("renders transcript rows when the runtime thread is transiently empty", async () => {
    transcriptMessages = sessionTranscriptFixture.slice(0, 3);

    renderSessionThread();

    await waitFor(() => {
      expect(
        screen.getByText("Summarize the launch blockers before the 18:30 UTC cutover.")
      ).toBeInTheDocument();
      expect(screen.getByText("Launch readiness snapshot")).toBeInTheDocument();
      expect(screen.getByTestId("tool-call-row")).toBeInTheDocument();
    });

    expect(screen.queryByText("Start a conversation.")).not.toBeInTheDocument();
  }, 10_000);

  it("Should show the skeleton on cold provider mount until the transcript resolves", async () => {
    const transcriptResponse = createDeferred<Response>();
    transcriptResponsePromise = transcriptResponse.promise;

    renderSessionThread();

    await waitFor(() => {
      expect(countTranscriptFetches(fetchMock)).toBe(1);
    });
    expect(screen.getByTestId("thread-transcript-skeleton")).toBeInTheDocument();
    expect(screen.queryByText("Start a conversation.")).not.toBeInTheDocument();

    transcriptResponse.resolve(jsonResponse(transcriptPayload(transcriptMessages)));

    expect(
      await screen.findByText("Summarize the launch blockers before the 18:30 UTC cutover.")
    ).toBeInTheDocument();
    expect(screen.getByText("Launch readiness snapshot")).toBeInTheDocument();
    expect(screen.queryByTestId("thread-transcript-skeleton")).not.toBeInTheDocument();
    expect(screen.queryByText("Start a conversation.")).not.toBeInTheDocument();
  }, 10_000);

  it("Should issue one transcript fetch per cold provider mount after the transcript settles", async () => {
    renderSessionThread();

    await waitFor(() => {
      expect(screen.getByText("Launch readiness snapshot")).toBeInTheDocument();
    });

    expect(countTranscriptFetches(fetchMock)).toBe(1);
  }, 10_000);

  it("Should keep the live EventSource closed when liveTailEnabled is false", async () => {
    const eventSourceFactory = vi.fn((url: string) => new FakeSessionEventSource(url));

    renderSessionThread({ eventSourceFactory, liveTailEnabled: false });

    expect(await screen.findByText("Launch readiness snapshot")).toBeInTheDocument();
    expect(eventSourceFactory).not.toHaveBeenCalled();
  });

  it("Should preserve a visible message anchor when older history and a live delta arrive together", async () => {
    const user = userEvent.setup();
    transcriptMessages = [sessionTranscriptFixture[1]!];
    transcriptFirstSequence = 20;
    transcriptHasOlder = true;
    transcriptNextBeforeSequence = 20;
    olderTranscriptMessages = [sessionTranscriptFixture[0]!];
    const olderResponse = createDeferred<Response>();
    olderTranscriptResponsePromise = olderResponse.promise;
    const sources: FakeSessionEventSource[] = [];
    let olderResolved = false;
    const anchorMessageId = sessionTranscriptFixture[1]!.id;

    renderSessionThread({
      eventSourceFactory: url => {
        const source = new FakeSessionEventSource(url);
        sources.push(source);
        return source;
      },
    });

    const loadOlder = await screen.findByRole("button", { name: "Load older messages" });
    const viewport = screen.getByTestId("chat-view");
    Object.defineProperty(viewport, "clientHeight", { configurable: true, value: 500 });
    Object.defineProperty(viewport, "scrollHeight", {
      configurable: true,
      get: () => (olderResolved ? 1_500 : 1_000),
    });
    Object.defineProperty(viewport, "scrollTo", {
      configurable: true,
      value: (options: ScrollToOptions) => {
        viewport.scrollTop = options.top ?? viewport.scrollTop;
      },
    });
    viewport.scrollTop = 320;
    fireEvent.wheel(viewport, { deltaY: -1 });
    fireEvent.scroll(viewport);
    const anchorRow = viewport.querySelector(`[data-message-id="${anchorMessageId}"]`);
    expect(anchorRow).toBeInstanceOf(HTMLElement);

    await user.click(loadOlder);
    const liveMessage: TranscriptMessage = {
      id: "anchor-live-delta",
      role: "assistant",
      parts: [{ type: "text", text: "Live delta while history loads.", state: "done" }],
    };
    act(() => {
      sources[0]?.dispatch(
        "transcript_delta",
        {
          cursor: 21,
          entries: [{ message: liveMessage, sequence: 21, start_sequence: 21 }],
          epoch: 1,
          generation: 1,
          has_more: false,
          max_sequence: 21,
          session_id: primarySessionFixture.id,
        },
        "21"
      );
    });
    expect(await screen.findByText("Live delta while history loads.")).toBeInTheDocument();

    olderResolved = true;
    olderResponse.resolve(jsonResponse(transcriptPayload(olderTranscriptMessages)));

    expect(
      await screen.findByText("Summarize the launch blockers before the 18:30 UTC cutover.")
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(viewport.querySelector(`[data-message-id="${anchorMessageId}"]`)).toBe(anchorRow);
      expect(viewport.scrollTop).toBeGreaterThanOrEqual(320);
    });
    expect(screen.queryByRole("button", { name: "Load older messages" })).not.toBeInTheDocument();
  }, 10_000);

  it("Should fail loudly when workspaceId is empty", () => {
    const queryClient = createQueryClient();
    expect(() =>
      render(
        <QueryClientProvider client={queryClient}>
          <SessionChatRuntimeProvider sessionId={primarySessionFixture.id} workspaceId=" ">
            <SessionThread
              sessionId={primarySessionFixture.id}
              agentName={primarySessionFixture.agent_name}
              canPrompt
              onCancelPrompt={() => {}}
            />
          </SessionChatRuntimeProvider>
        </QueryClientProvider>
      )
    ).toThrow("SessionChatRuntimeProvider requires a non-empty workspaceId");
  });

  it("Should mint a fresh gateway ticket for every remote prompt", async () => {
    streamTickets = ["prompt-ticket-one", "prompt-ticket-two"];
    const user = userEvent.setup();
    renderSessionThread({ eventSourceFactory: url => new FakeSessionEventSource(url) });

    await screen.findByTestId("composer-input");
    await setComposerText("First remote prompt");
    await user.click(screen.getByTestId("composer-send-button"));
    await waitFor(() => expect(countPromptFetches(fetchMock)).toBe(1));

    await setComposerText("Second remote prompt");
    await waitFor(() => expect(screen.getByTestId("composer-send-button")).toBeEnabled());
    await user.click(screen.getByTestId("composer-send-button"));
    await waitFor(() => expect(countPromptFetches(fetchMock)).toBe(2));

    const promptTickets = fetchMock.mock.calls
      .map(([input]) => getRequestURL(input as RequestInfo | URL))
      .filter(url => url.pathname.endsWith("/prompt"))
      .map(url => url.searchParams.get("ticket"));
    expect(promptTickets).toEqual(["prompt-ticket-one", "prompt-ticket-two"]);
  });

  it("Should report a revoked device response from the prompt transport", async () => {
    streamTickets = ["prompt-ticket"];
    promptResponsePromise = Promise.resolve(
      jsonResponse(
        { error: "Device revoked", code: "gateway_device_revoked" },
        {
          status: 401,
          headers: {
            "Content-Type": "application/json",
            "X-Compozy-Gateway-Tier": "private",
          },
        }
      )
    );
    const signals: GatewayAccessSignal[] = [];
    const unsubscribe = observeGatewayAccess(signal => signals.push(signal));
    const user = userEvent.setup();

    try {
      renderSessionThread({ eventSourceFactory: url => new FakeSessionEventSource(url) });
      await screen.findByTestId("composer-input");
      await setComposerText("Prompt from a revoked device");
      await user.click(screen.getByTestId("composer-send-button"));

      await waitFor(() => expect(signals).toEqual(["revoked"]));
    } finally {
      unsubscribe();
    }
  });

  it("Should preserve the first Goal transport and local cancellation across StrictMode replay", async () => {
    const prompt =
      '/goal Reply in exactly one sentence: "CompozyOS keeps agent work local-first and durable."';
    const promptResponse = createDeferred<Response>();
    const onCancelPrompt = vi.fn();
    const sources: FakeSessionEventSource[] = [];
    const user = userEvent.setup();
    transcriptMessages = [
      {
        id: "session-create-hook-start",
        role: "assistant",
        parts: [{ type: "data-compozy-event", data: { type: "hook.dispatch.start" } }],
      },
      {
        id: "session-create-hook-complete",
        role: "assistant",
        parts: [{ type: "data-compozy-event", data: { type: "hook.dispatch.complete" } }],
      },
    ];
    transcriptEpoch = 0;
    transcriptGeneration = 0;
    promptResponsePromise = promptResponse.promise;

    const view = renderSessionThread({
      eventSourceFactory: url => {
        const source = new FakeSessionEventSource(url);
        sources.push(source);
        return source;
      },
      onCancelPrompt,
      strictMode: true,
    });

    try {
      const composer = await screen.findByTestId("composer-input");
      await waitFor(() => {
        expect(screen.getAllByTestId("thread-message-row")).toHaveLength(2);
        // The Lexical composer opts out of interaction via `inert`, not `disabled`.
        expect(composer).not.toHaveAttribute("inert");
        expect(sources).toHaveLength(2);
        expect(sources[0]?.closed).toBe(true);
        expect(sources[1]?.closed).toBe(false);
      });
      expect(countPromptFetches(fetchMock)).toBe(0);

      await setComposerText(prompt);
      await user.click(screen.getByTestId("composer-send-button"));

      await waitFor(() => expect(countPromptFetches(fetchMock)).toBe(1));
      const promptRequest = fetchMock.mock.calls.find(([input]) =>
        getPathname(input as RequestInfo | URL).endsWith("/prompt")
      );
      expect((promptRequest?.[1] as RequestInit | undefined)?.signal?.aborted).toBe(false);
      expect(await screen.findByText(prompt)).toBeInTheDocument();
      expect(await screen.findByLabelText("Working")).toBeInTheDocument();

      await user.click(screen.getByTestId("composer-stop-button"));

      expect(onCancelPrompt).toHaveBeenCalledTimes(1);
      await waitFor(() => {
        expect((promptRequest?.[1] as RequestInit | undefined)?.signal?.aborted).toBe(true);
        expect(screen.queryByTestId("composer-stop-button")).not.toBeInTheDocument();
        const idleComposer = screen.getByTestId("composer-input");
        expect(idleComposer).not.toHaveAttribute("inert");
        expect(composerText()).toBe("");
        expect(screen.getByTestId("composer-send-button")).toBeDisabled();
      });
    } finally {
      promptResponse.resolve(
        sseResponse([
          'data: {"type":"start","messageId":"turn-runtime-strict-001"}\n\n',
          'data: {"type":"finish","finishReason":"stop"}\n\n',
          "data: [DONE]\n\n",
        ])
      );
      view.unmount();
    }
  });

  // Invariant: retrying an existing user turn after a provider rerender keeps
  // its original idempotency key. Owning layer: session chat runtime provider
  // integration. Canonical suite: this provider suite.
  it("retains the prompt idempotency key when a rerendered provider retries the response", async () => {
    const user = userEvent.setup();
    const { rerenderSessionThread } = renderSessionThread({ includeRuntimeRetryControl: true });

    await screen.findByTestId("composer-input");
    await setComposerText("Retry this answer after the provider rerenders");
    await user.click(screen.getByTestId("composer-send-button"));

    await waitFor(() => expect(countPromptFetches(fetchMock)).toBe(1));
    const firstPromptRequest = fetchMock.mock.calls.find(([input]) => {
      return (
        getPathname(input as RequestInfo | URL) ===
        `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/${primarySessionFixture.id}/prompt`
      );
    });
    const firstPromptBody = JSON.parse(String(firstPromptRequest?.[1]?.body)) as {
      idempotency_key: string;
      message_id: string;
    };

    rerenderSessionThread();
    await user.click(await screen.findByRole("button", { name: "Retry latest assistant message" }));

    await waitFor(() => expect(countPromptFetches(fetchMock)).toBe(2));
    const promptRequests = fetchMock.mock.calls.filter(([input]) => {
      return (
        getPathname(input as RequestInfo | URL) ===
        `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/${primarySessionFixture.id}/prompt`
      );
    });
    const retryPromptRequest = promptRequests[1];
    const retryPromptBody = JSON.parse(String(retryPromptRequest?.[1]?.body)) as {
      idempotency_key: string;
      message_id: string;
    };

    expect(retryPromptBody.message_id).toBe(firstPromptBody.message_id);
    expect(retryPromptBody.idempotency_key).toBe(firstPromptBody.idempotency_key);
  });

  it.each([
    ["goal_objective_required", "/goal", "Add an objective after /goal, then try again."],
    [
      "goal_objective_too_large",
      `/goal ${"x".repeat(5_000)}`,
      "Shorten the Goal objective, then try again.",
    ],
  ] as const)(
    "Should render actionable guidance for %s while keeping its reason code internal",
    async (reasonCode, prompt, guidance) => {
      goalCommandFailureReason = reasonCode;
      const user = userEvent.setup();
      renderSessionThread();

      const composer = await screen.findByTestId("composer-input");
      await setComposerText(prompt);
      await user.click(screen.getByTestId("composer-send-button"));

      await waitFor(() => expect(countPromptFetches(fetchMock)).toBe(1));
      expect(
        sessionStore.getSnapshot().context.goalFeedback[primarySessionFixture.id]?.result
          .reason_code
      ).toBe(reasonCode);
      expect(await screen.findByRole("alert")).toHaveTextContent(guidance);
      expect(screen.queryByText(reasonCode)).not.toBeInTheDocument();
      expect(composer).not.toHaveAttribute("inert");
      await setComposerText("Retry draft");
      expect(composerText()).toBe("Retry draft");
      expect(screen.getByRole("alert")).toHaveTextContent(guidance);
      expect(screen.getByTestId("composer-send-button")).toBeEnabled();
      expect(countPromptFetches(fetchMock)).toBe(1);

      goalCommandFailureReason = null;
      await user.click(screen.getByTestId("composer-send-button"));
      await waitFor(() => expect(countPromptFetches(fetchMock)).toBe(2));
      await waitFor(() => expect(screen.queryByRole("alert")).not.toBeInTheDocument());
      expect(
        sessionStore.getSnapshot().context.goalFeedback[primarySessionFixture.id]?.result
          .reason_code
      ).toBe(reasonCode);
    }
  );

  it("keeps locally sent prompts visible through transcript reconciliation", async () => {
    const user = userEvent.setup();
    const { queryClient } = renderSessionThread();

    await waitFor(() => {
      expect(screen.getByText("Launch readiness snapshot")).toBeInTheDocument();
    });

    await setComposerText("Continue from the reattached thread");
    await user.click(screen.getByTestId("composer-send-button"));

    await waitFor(() => {
      expect(screen.getByText("Continue from the reattached thread")).toBeInTheDocument();
      expect(
        screen.getByText("Live runtime answer before transcript reconciliation.")
      ).toBeInTheDocument();
    });
    expect(lastPromptMessageID).not.toBe("");

    transcriptMessages = [
      ...sessionTranscriptFixture.slice(0, 2),
      {
        id: lastPromptMessageID,
        role: "user",
        parts: [
          {
            type: "text",
            text: "Continue from the reattached thread",
            state: "done",
          },
        ],
      },
    ];
    await queryClient.invalidateQueries({
      queryKey: sessionKeys.transcript(fixtureWorkspaceId(), primarySessionFixture.id),
    });

    await waitFor(() => {
      expect(screen.getAllByText("Continue from the reattached thread")).toHaveLength(1);
      expect(
        screen.getByText("Live runtime answer before transcript reconciliation.")
      ).toBeInTheDocument();
    });

    transcriptMessages = [
      ...transcriptMessages,
      {
        id: "turn-runtime-001",
        role: "assistant",
        parts: [
          {
            type: "text",
            text: "Durable transcript answer after reconciliation.",
            state: "done",
          },
        ],
      },
    ];
    await queryClient.invalidateQueries({
      queryKey: sessionKeys.transcript(fixtureWorkspaceId(), primarySessionFixture.id),
    });

    await waitFor(() => {
      expect(screen.getAllByText("Continue from the reattached thread")).toHaveLength(1);
      expect(
        screen.getByText("Durable transcript answer after reconciliation.")
      ).toBeInTheDocument();
    });
    const reconciledRows = screen.getAllByTestId("thread-message-row");
    const promptIndex = reconciledRows.findIndex(row =>
      row.textContent?.includes("Continue from the reattached thread")
    );
    const answerIndex = reconciledRows.findIndex(row =>
      row.textContent?.includes("Durable transcript answer after reconciliation.")
    );
    expect(promptIndex).toBeGreaterThanOrEqual(0);
    expect(answerIndex).toBeGreaterThan(promptIndex);

    expect(
      screen.queryByText("Live runtime answer before transcript reconciliation.")
    ).not.toBeInTheDocument();
    expect(
      transcriptMessages.some(message => JSON.stringify(message).includes("Live runtime answer"))
    ).toBe(false);
    expect(
      fetchMock.mock.calls.some(([input]) => {
        return (
          getPathname(input as RequestInfo | URL) ===
          `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/${primarySessionFixture.id}/prompt`
        );
      })
    ).toBe(true);
  }, 10_000);

  it("Should project a newly completed runtime message in its first committed frame", async () => {
    transcriptMessages = [];
    const commits: RuntimeTranscriptCommit[] = [];
    const user = userEvent.setup();
    renderSessionThread({ onRuntimeTranscriptCommit: sample => commits.push(sample) });

    await screen.findByTestId("composer-input");
    await setComposerText("Finish in one turn");
    await user.click(screen.getByTestId("composer-send-button"));
    expect(
      await screen.findByText("Live runtime answer before transcript reconciliation.")
    ).toBeInTheDocument();

    const completedRuntimeCommits = commits.filter(sample =>
      sample.runtimeIds.includes("turn-runtime-001")
    );
    expect(completedRuntimeCommits.length).toBeGreaterThan(0);
    expect(
      completedRuntimeCommits.every(sample => sample.projectedIds.includes("turn-runtime-001"))
    ).toBe(true);
  });

  // Invariant: the assistant transport binds the selected runtime, canonical
  // authored text, and durable identities without discarding assistant-ui history.
  // Owning layer: session chat transport integration. Canonical suite: this provider suite.
  it("includes the selected runtime in the prompt transport body", async () => {
    const runtime = {
      model: "gpt-5.4",
      provider: "codex",
      reasoning_effort: "high" as const,
      speed: "fast" as const,
    };
    const user = userEvent.setup();
    renderSessionThread({ runtimeSnapshot: runtime });

    await screen.findByTestId("composer-input");
    await setComposerText("Use the selected runtime for this prompt");
    await user.click(screen.getByTestId("composer-send-button"));

    await waitFor(() => expect(countPromptFetches(fetchMock)).toBe(1));
    const promptCall = fetchMock.mock.calls.find(([input]) => {
      return (
        getPathname(input as RequestInfo | URL) ===
        `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/${primarySessionFixture.id}/prompt`
      );
    });
    const init = promptCall?.[1] as RequestInit | undefined;
    const body = JSON.parse(String(init?.body)) as {
      idempotency_key?: string;
      message_id?: string;
      message?: unknown;
      messageId?: unknown;
      messages?: Array<{ role?: string }>;
      runtime?: unknown;
    };
    expect(body.runtime).toEqual(runtime);
    expect(body.message).toBe("Use the selected runtime for this prompt");
    expect(body.messages?.at(-1)).toEqual(expect.objectContaining({ role: "user" }));
    expect(body.message_id).toBe(getLastUserMessageID(init));
    expect(typeof body.idempotency_key).toBe("string");
    expect(body.messageId).toBeUndefined();
  });

  it("Should dock a live permission while the prompt stream remains open", async () => {
    transcriptMessages = [];
    const openPrompt = openSseResponse([
      'data: {"type":"start","messageId":"turn-runtime-permission-001"}\n\n',
      'data: {"type":"data-compozy-permission","id":"permission-live-001","data":{"type":"permission","session_id":"session_001","turn_id":"turn-runtime-permission-001","request_id":"permission-live-001","title":"Edit file","action":"session/request_permission","resource":"pending.txt","raw":{"options":[{"decision":"allow-once","option_id":"allow-once"},{"decision":"reject-once","option_id":"reject-once"}],"tool_input":{"path":"pending.txt"}}}}\n\n',
    ]);
    promptResponsePromise = Promise.resolve(openPrompt.response);
    const user = userEvent.setup();
    const view = renderSessionThread({ includeDecisionDock: true });

    try {
      await screen.findByTestId("composer-input");
      await setComposerText("Request permission");
      await user.click(screen.getByTestId("composer-send-button"));

      expect(await screen.findByTestId("permission-dock")).toBeInTheDocument();
      expect(screen.getByTestId("permission-dock-title")).toHaveTextContent("Edit file");
    } finally {
      openPrompt.close(['data: {"type":"finish","finishReason":"stop"}\n\n', "data: [DONE]\n\n"]);
      view.unmount();
    }
  });

  it("Should hide a completed runtime tail while an authoritative Clear is pending", async () => {
    transcriptMessages = [];
    const clearResponse = createDeferred<Response>();
    clearResponsePromise = clearResponse.promise;
    const user = userEvent.setup();
    renderSessionThread({ includeClearAction: true, includeTranscriptStateProbe: true });

    await screen.findByTestId("composer-input");
    const prompt = "Clear me";
    await setComposerText(prompt);
    await user.click(screen.getByTestId("composer-send-button"));

    expect(await screen.findByText(prompt)).toBeInTheDocument();
    expect(
      await screen.findByText("Live runtime answer before transcript reconciliation.")
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Clear test conversation" }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Clear test conversation" })).toBeDisabled();
    });
    expect(screen.queryByText(prompt)).not.toBeInTheDocument();
    expect(
      screen.queryByText("Live runtime answer before transcript reconciliation.")
    ).not.toBeInTheDocument();

    clearResponse.resolve(jsonResponse({ session: sessionDetailResponse }));
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Clear test conversation" })).toBeEnabled();
    });
    expect(screen.queryByText(prompt)).not.toBeInTheDocument();
  }, 10_000);

  it("keeps distinct runtime and durable message ids visible even when metadata matches", () => {
    const runtimeMessages = toReadonlyThreadMessages([
      {
        id: "client_temp_assistant_001",
        role: "assistant",
        metadata: { turn_id: "turn_promote_001" },
        parts: [
          {
            type: "text",
            text: "Promoted assistant response.",
            state: "done",
          },
        ],
      } as TranscriptMessage,
    ]);
    const transcriptMessages = toReadonlyThreadMessages([
      {
        id: "server_assistant_001",
        role: "assistant",
        metadata: { turn_id: "turn_promote_001", message_id: "client_temp_assistant_001" },
        parts: [
          {
            type: "text",
            text: "Promoted assistant response.",
            state: "done",
          },
        ],
      } as TranscriptMessage,
    ]);

    const merged = mergeSessionThreadReadModel({ transcriptMessages, runtimeMessages });

    expect(merged.map(message => message.id)).toEqual([
      "server_assistant_001",
      "client_temp_assistant_001",
    ]);
  });

  it("Should reconcile a user message only when its durable id exactly matches", () => {
    const runtimeMessages = toReadonlyThreadMessages([
      {
        id: "client_user_001",
        role: "user",
        parts: [{ type: "text", text: "One authored prompt.", state: "done" }],
      } as TranscriptMessage,
    ]);
    const transcriptMessages = toReadonlyThreadMessages([
      {
        id: "client_user_001",
        role: "user",
        parts: [{ type: "text", text: "One authored prompt.", state: "done" }],
      } as TranscriptMessage,
      {
        id: "server_assistant_001",
        role: "assistant",
        parts: [{ type: "text", text: "One durable response.", state: "done" }],
      } as TranscriptMessage,
    ]);

    const merged = mergeSessionThreadReadModel({ transcriptMessages, runtimeMessages });

    expect(merged.map(message => message.id)).toEqual(["client_user_001", "server_assistant_001"]);
  });

  it("Should let an empty authoritative transcript replace a stale runtime tail", () => {
    const runtimeMessages = toReadonlyThreadMessages([
      {
        id: "stale_runtime_user_001",
        role: "user",
        parts: [{ type: "text", text: "Old prompt before clear", state: "done" }],
      } as TranscriptMessage,
    ]);

    const merged = mergeSessionThreadReadModel({
      transcriptMessages: [],
      runtimeMessages,
      includeRuntimeTail: false,
    });

    expect(merged).toEqual([]);
  });

  it("Should let same-count authoritative transcript content replace stale runtime content", () => {
    const runtimeMessages = toReadonlyThreadMessages([
      {
        id: "stale_runtime_assistant_001",
        role: "assistant",
        parts: [{ type: "text", text: "Old assistant text", state: "done" }],
      } as TranscriptMessage,
    ]);
    const transcriptMessages = toReadonlyThreadMessages([
      {
        id: "server_assistant_replacement_001",
        role: "assistant",
        parts: [{ type: "text", text: "Replacement assistant text", state: "done" }],
      } as TranscriptMessage,
    ]);

    const merged = mergeSessionThreadReadModel({
      transcriptMessages,
      runtimeMessages,
      includeRuntimeTail: false,
    });

    expect(merged.map(message => message.id)).toEqual(["server_assistant_replacement_001"]);
  });

  it("renders runtime progress events as activity notices instead of assistant text", async () => {
    transcriptMessages = [
      ...sessionTranscriptFixture.slice(0, 1),
      {
        id: "transcript_runtime_001",
        role: "assistant",
        parts: [
          {
            type: "data-compozy-event",
            data: {
              type: "runtime_progress",
              text: "Still working",
              runtime: {
                turn_id: "turn_001",
                current_tool: "Bash",
                elapsed_ms: 610_000,
                elapsed_seconds: 610,
                idle_seconds: 30,
              },
            },
          },
        ],
      },
    ];

    renderSessionThread();

    await waitFor(() => {
      expect(screen.getByTestId("runtime-activity-notice")).toBeInTheDocument();
    });

    expect(screen.getByTestId("runtime-activity-notice")).toHaveTextContent("Still working");
    expect(screen.getByTestId("runtime-activity-detail")).toHaveTextContent("Using Bash");
  }, 10_000);

  it("renders persisted session error events as failure notices", async () => {
    transcriptMessages = [
      ...sessionTranscriptFixture.slice(0, 1),
      {
        id: "transcript_error_001",
        role: "assistant",
        parts: [
          {
            type: "text",
            text: "Partial response before failure.",
            state: "done",
          },
          {
            type: "data-compozy-event",
            data: {
              type: "error",
              error:
                '{"code":-32603,"message":"Internal error","data":{"error":"peer disconnected before response"}}',
              failure: {
                kind: "process_exit",
                summary: "peer disconnected before response",
              },
            },
          },
        ],
      },
    ];

    renderSessionThread();

    await waitFor(() => {
      expect(screen.getByTestId("session-error-notice")).toBeInTheDocument();
    });

    expect(screen.getByText("Partial response before failure.")).toBeInTheDocument();
    expect(screen.getByTestId("session-error-notice")).toHaveTextContent("Session failed");
    expect(screen.getByTestId("session-error-detail")).toHaveTextContent(
      "peer disconnected before response"
    );
  }, 10_000);

  it("does not render empty error chrome for incomplete message status without a detail", async () => {
    transcriptMessages = [
      ...sessionTranscriptFixture.slice(0, 1),
      {
        id: "transcript_empty_error_status_001",
        role: "assistant",
        status: {
          type: "incomplete",
          reason: "error",
          error: "",
        },
        parts: [
          {
            type: "data-compozy-event",
            data: {
              type: "error",
            },
          },
        ],
      } as unknown as TranscriptMessage,
    ];

    renderSessionThread();

    await waitFor(() => {
      expect(
        screen.getByText("Summarize the launch blockers before the 18:30 UTC cutover.")
      ).toBeInTheDocument();
    });

    expect(screen.queryByTestId("session-message-error")).not.toBeInTheDocument();
    expect(screen.queryByTestId("session-error-notice")).not.toBeInTheDocument();
  }, 10_000);

  it("keeps pending permissions out of the transcript and renders receipts for resolved ones", async () => {
    transcriptMessages = [
      ...sessionTranscriptFixture.slice(0, 1),
      {
        id: "transcript_permission_001",
        role: "assistant",
        parts: [
          {
            type: "data-compozy-permission",
            data: {
              type: "permission",
              request_id: "turn_001:perm_pending",
              title: "Edit pending file",
              resource: "pending.txt",
              action: "session/request_permission",
              raw: { path: "pending.txt" },
            },
          },
          {
            type: "data-compozy-permission",
            data: {
              type: "permission",
              request_id: "turn_001:perm_resolved",
              title: "Edit resolved file",
              resource: "resolved.txt",
              action: "session/request_permission",
              decision: "reject-always",
              raw: { path: "resolved.txt" },
            },
          },
        ],
      },
    ];

    renderSessionThread();

    // The resolved request keeps only its one-line receipt in the transcript.
    await waitFor(() => {
      expect(screen.getByTestId("permission-rejected-notice")).toBeInTheDocument();
    });
    expect(screen.getByTestId("permission-rejected-notice")).toHaveTextContent(
      "Edit resolved file"
    );
    // The pending ask renders nothing in the transcript — it docks on the
    // composer instead (owned by session-decision-dock.test.tsx).
    expect(screen.queryByText(/pending\.txt/)).not.toBeInTheDocument();
    expect(screen.queryByTestId("permission-prompt")).not.toBeInTheDocument();
  }, 10_000);

  it("routes a clarify data event to the clarification receipt, not the activity notice", async () => {
    transcriptMessages = [
      ...sessionTranscriptFixture.slice(0, 1),
      {
        id: "transcript_clarify_001",
        role: "assistant",
        parts: [
          {
            type: "data-compozy-event",
            data: {
              type: "clarify",
              session_id: primarySessionFixture.id,
              turn_id: "clarify:req-1",
              raw: {
                status: "resolved",
                request: {
                  request_id: "req-1",
                  workspace_id: primarySessionFixture.workspace_id,
                  session_id: primarySessionFixture.id,
                  agent_name: primarySessionFixture.agent_name,
                  question: "Which environment should deploy?",
                  choices: ["staging", "production"],
                  asked_at: "2026-07-16T00:00:00Z",
                  deadline: "2026-07-16T00:05:00Z",
                },
                answer: { choice: 1, text: "", fallback: false },
                at: "2026-07-16T00:00:10Z",
              },
            },
          },
        ],
      },
    ];

    renderSessionThread();

    await waitFor(() => {
      expect(screen.getByTestId("clarification-receipt")).toBeInTheDocument();
    });
    expect(screen.getByTestId("clarification-receipt")).toHaveAttribute("data-status", "resolved");
    expect(screen.getByTestId("clarification-receipt-answer")).toHaveTextContent("production");
    expect(screen.queryByTestId("runtime-activity-notice")).not.toBeInTheDocument();
  }, 10_000);

  it("renders mixed text, reasoning, and unregistered tool parts inline in order", async () => {
    transcriptMessages = [
      ...sessionTranscriptFixture.slice(0, 1),
      {
        id: "transcript_mixed_parts_001",
        role: "assistant",
        parts: [
          {
            type: "text",
            text: "Before search.",
            state: "done",
          },
          {
            type: "reasoning",
            text: "Need the current launch note before answering.",
            state: "streaming",
          },
          {
            type: "tool-WebSearch",
            toolCallId: "tool_web_001",
            state: "output-available",
            input: {
              query: "launch note",
            },
            output: {
              type: "tool_result",
              title: "WebSearch",
              raw: {
                content: "Launch note found.",
              },
            },
          },
          {
            type: "text",
            text: "After search.",
            state: "done",
          },
        ],
      },
    ];

    renderSessionThread();

    await waitFor(() => {
      expect(screen.getByTestId("tool-call-row")).toBeInTheDocument();
    });

    // The reasoning part is still streaming, so the flattened ThinkingBlock renders
    // it auto-open inline — no toggle needed to read it in order.
    const chat = screen.getByTestId("chat-view");
    const chatText = chat.textContent ?? "";
    const beforeIndex = chatText.indexOf("Before search.");
    const reasoningIndex = chatText.indexOf("Need the current launch note before answering.");
    // The row heading is the visible tense-aware verb ("Searched web"), not the raw tool id.
    const toolIndex = chatText.indexOf("Searched web");
    const afterIndex = chatText.indexOf("After search.");

    expect(beforeIndex).toBeGreaterThanOrEqual(0);
    expect(reasoningIndex).toBeGreaterThan(beforeIndex);
    expect(toolIndex).toBeGreaterThan(reasoningIndex);
    expect(afterIndex).toBeGreaterThan(toolIndex);
    expect(within(chat).getByTestId("tool-call-row")).toHaveTextContent("Searched web");
  }, 10_000);

  it("keeps unregistered data parts inside the settled turn fold instead of dropping them", async () => {
    const user = userEvent.setup();
    transcriptMessages = [
      ...sessionTranscriptFixture.slice(0, 1),
      {
        id: "transcript_unknown_data_001",
        role: "assistant",
        parts: [
          {
            type: "text",
            text: "Before data.",
            state: "done",
          },
          {
            type: "data-provider-note",
            data: {
              title: "Provider note",
              detail: "Unregistered data event",
            },
          },
          {
            type: "text",
            text: "After data.",
            state: "done",
          },
        ],
      },
    ];

    renderSessionThread();

    // The settled turn keeps authored text visible and folds the unregistered data
    // at its chronological position instead of dropping or moving it.
    const fold = await screen.findByRole("button", { name: "Worked" });
    expect(screen.getByText("After data.")).toBeInTheDocument();
    expect(screen.queryByTestId("session-data-part")).not.toBeInTheDocument();

    await user.click(fold);

    await waitFor(() => {
      expect(screen.getByTestId("session-data-part")).toBeInTheDocument();
    });

    const chat = screen.getByTestId("chat-view");
    const chatText = chat.textContent ?? "";
    const beforeIndex = chatText.indexOf("Before data.");
    const dataIndex = chatText.indexOf("provider-note");
    const afterIndex = chatText.indexOf("After data.");

    expect(beforeIndex).toBeGreaterThanOrEqual(0);
    expect(dataIndex).toBeGreaterThan(beforeIndex);
    expect(afterIndex).toBeGreaterThan(dataIndex);
    // The marker line names the raw event as mono meta — no payload preview card.
    expect(within(chat).getByTestId("session-data-part")).toHaveTextContent("data-provider-note");
  }, 10_000);

  it("reconciles durable transcript after a live session stream event", async () => {
    transcriptMessages = sessionTranscriptFixture.slice(0, 1);
    const sources: FakeSessionEventSource[] = [];

    renderSessionThread({
      eventSourceFactory: url => {
        const source = new FakeSessionEventSource(url);
        sources.push(source);
        return source;
      },
    });

    await waitFor(() => {
      expect(
        screen.getByText("Summarize the launch blockers before the 18:30 UTC cutover.")
      ).toBeInTheDocument();
    });
    await waitFor(() => {
      expect(sources[0]?.url).toBe(
        `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/${primarySessionFixture.id}/stream?${SESSION_STREAM_QUERY}`
      );
    });

    const liveMessage: TranscriptMessage = {
      id: "transcript_assistant_live_tail_001",
      role: "assistant",
      parts: [{ type: "text", text: "Live reattached answer.", state: "done" }],
    };
    sources[0]?.dispatch(
      "transcript_delta",
      {
        session_id: primarySessionFixture.id,
        epoch: 1,
        generation: 1,
        cursor: 2,
        entries: [{ message: liveMessage, sequence: 2, start_sequence: 2 }],
        has_more: false,
        max_sequence: 2,
      },
      "2"
    );

    await waitFor(() => {
      expect(screen.getByText("Live reattached answer.")).toBeInTheDocument();
    });
  }, 10_000);

  it("merges a same-fence durable snapshot without an extra transcript request", async () => {
    transcriptMessages = sessionTranscriptFixture.slice(0, 1);
    const sources: FakeSessionEventSource[] = [];

    renderSessionThread({
      eventSourceFactory: url => {
        const source = new FakeSessionEventSource(url);
        sources.push(source);
        return source;
      },
    });

    await waitFor(() => {
      expect(sources).toHaveLength(1);
    });
    await waitFor(() => {
      expect(sources[0]?.listeners.get("transcript_snapshot")?.size).toBeGreaterThan(0);
      expect(
        screen.getByText("Summarize the launch blockers before the 18:30 UTC cutover.")
      ).toBeInTheDocument();
    });

    const recoveredMessage: TranscriptMessage = {
      id: "transcript_gap_recovery_001",
      role: "assistant",
      parts: [{ type: "text", text: "Recovered from the durable transcript.", state: "done" }],
    };
    await act(async () => {
      sources[0]?.dispatch(
        "transcript_snapshot",
        {
          session_id: primarySessionFixture.id,
          epoch: 1,
          generation: 1,
          entries: [
            { message: sessionTranscriptFixture[0]!, sequence: 1, start_sequence: 1 },
            { message: recoveredMessage, sequence: 4, start_sequence: 4 },
          ],
          has_older: false,
          max_sequence: 4,
          reset: false,
        },
        "4"
      );
    });

    await waitFor(() => {
      expect(screen.getByText("Recovered from the durable transcript.")).toBeInTheDocument();
    });

    expect(countTranscriptFetches(fetchMock)).toBe(1);
  }, 10_000);

  it("Should close the provider stream on terminal frames and reopen only after the session becomes live", async () => {
    vi.useFakeTimers();
    transcriptMessages = sessionTranscriptFixture.slice(0, 1);
    const sources: FakeSessionEventSource[] = [];
    const queryClient = createQueryClient();
    const workspaceId = fixtureWorkspaceId();
    const sessionId = primarySessionFixture.id;
    queryClient.setQueryData(sessionKeys.detail(workspaceId, sessionId), primarySessionFixture);

    renderSessionThread({
      queryClient,
      eventSourceFactory: url => {
        const source = new FakeSessionEventSource(url);
        sources.push(source);
        return source;
      },
    });

    await act(async () => {
      await vi.waitFor(() => {
        expect(sources).toHaveLength(1);
      });
    });

    sessionDetailResponse = { ...primarySessionFixture, state: "stopped" };
    act(() => {
      sources[0]?.dispatch(
        "session_stopped",
        {
          id: "session-stopped-fixture",
          session_id: sessionId,
          sequence: 9,
          type: "session_stopped",
          timestamp: "2026-07-07T12:00:00Z",
          turn_id: "turn-terminal",
          agent_name: primarySessionFixture.agent_name,
          content: {},
          spawn_depth: 0,
        },
        "9"
      );
    });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(4_000);
    });

    expect(sources[0]?.closed).toBe(true);
    expect(sources).toHaveLength(1);

    act(() => {
      queryClient.setQueryData(sessionKeys.detail(workspaceId, sessionId), primarySessionFixture);
    });

    await act(async () => {
      await vi.waitFor(() => {
        expect(sources).toHaveLength(2);
      });
    });
    expect(sources[1]?.url).toBe(
      `/api/workspaces/${workspaceId}/sessions/${sessionId}/stream?${SESSION_STREAM_QUERY}&after_sequence=1&epoch=1&generation=1`
    );
  }, 10_000);

  it("Should expose transcript fetch failure through provider context while attempting the stream", async () => {
    transcriptFetchShouldFail = true;
    const sources: FakeSessionEventSource[] = [];

    renderSessionThread({
      includeTranscriptStateProbe: true,
      eventSourceFactory: url => {
        const source = new FakeSessionEventSource(url);
        sources.push(source);
        return source;
      },
    });

    await waitFor(() => {
      expect(sources).toHaveLength(1);
    });
    await waitFor(() => {
      expect(screen.getByTestId("transcript-state-probe")).toHaveAttribute("data-status", "error");
    });

    const state = screen.getByTestId("transcript-state-probe");
    expect(sources[0]?.url).toBe(
      `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/${primarySessionFixture.id}/stream?${SESSION_STREAM_QUERY}`
    );
    expect(state).toHaveAttribute("data-is-error", "true");
    expect(state).toHaveTextContent("0");
    expect(state.getAttribute("data-error-message")).toContain("recorder temporarily unavailable");
  }, 10_000);

  it("Should expose large transcript histories in order through a bounded DOM window", async () => {
    transcriptMessages = Array.from({ length: 80 }, (_, index) => ({
      id: `transcript_large_${index}`,
      role: index % 2 === 0 ? "user" : "assistant",
      parts: [{ type: "text", text: `Large transcript message ${index}`, state: "done" }],
    })) as TranscriptMessage[];

    renderSessionThread({ includeTranscriptStateProbe: true });

    await waitFor(() => {
      expect(screen.getByTestId("transcript-state-probe")).toHaveTextContent("80");
      const renderedRows = screen.getAllByTestId("thread-message-row");
      expect(renderedRows.length).toBeGreaterThan(0);
      expect(renderedRows.length).toBeLessThan(80);
    });

    const renderedRows = screen.getAllByTestId("thread-message-row");
    const rowIndexes = renderedRows.map(row => Number(row.getAttribute("data-message-index")));
    expect(rowIndexes).toEqual([...rowIndexes].sort((left, right) => left - right));
    for (const [position, row] of renderedRows.entries()) {
      expect(
        within(row).getByText(`Large transcript message ${rowIndexes[position]}`)
      ).toBeVisible();
    }
  }, 10_000);
});
