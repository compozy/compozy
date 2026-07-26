import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useAui } from "@assistant-ui/react";
import { StrictMode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { SessionThread } from "@/components/assistant-ui/session-thread";
import { Toaster } from "@agh/ui";
import { formatMessageError } from "@/components/assistant-ui/session-thread-error";
import {
  sessionKeys,
  useClearSessionConversation,
  useSessionTranscriptThreadState,
} from "@/systems/session";
import { useSessionStore } from "@/systems/session/hooks/use-session-store";
import { mergeSessionThreadReadModel } from "@/systems/session/lib/session-thread-read-model";
import { toReadonlyThreadMessages } from "@/systems/session/lib/session-thread-repository";
import type { SessionTranscriptData } from "@/systems/session/lib/session-transcript-query";
import { primarySessionFixture, sessionTranscriptFixture } from "@/systems/session/mocks/fixtures";
import type { TranscriptMessage } from "@/systems/session/types";

import { SessionChatRuntimeProvider } from "../session-chat-runtime-provider";

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

function testRect(top: number, height = 40): DOMRect {
  return {
    bottom: top + height,
    height,
    left: 0,
    right: 800,
    top,
    width: 800,
    x: 0,
    y: top,
    toJSON: () => ({}),
  };
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

function ClearConversationButton() {
  const aui = useAui();
  const clearMutation = useClearSessionConversation({ workspaceId: fixtureWorkspaceId() });

  return (
    <button
      type="button"
      disabled={clearMutation.isPending}
      onClick={() => {
        clearMutation.mutate(primarySessionFixture.id, {
          onSuccess: () => aui.thread().reset(),
        });
      }}
    >
      Clear test conversation
    </button>
  );
}

function renderSessionThread(
  options: {
    eventSourceFactory?: (url: string) => FakeSessionEventSource;
    includeClearAction?: boolean;
    includeToaster?: boolean;
    queryClient?: QueryClient;
    includeTranscriptStateProbe?: boolean;
    onCancelPrompt?: () => void;
    strictMode?: boolean;
  } = {}
) {
  const queryClient = options.queryClient ?? createQueryClient();

  const tree = (
    <QueryClientProvider client={queryClient}>
      <SessionChatRuntimeProvider
        sessionId={primarySessionFixture.id}
        workspaceId={fixtureWorkspaceId()}
        eventSourceFactory={options.eventSourceFactory}
      >
        {options.includeTranscriptStateProbe ? <TranscriptStateProbe /> : null}
        {options.includeClearAction ? <ClearConversationButton /> : null}
        <SessionThread
          sessionId={primarySessionFixture.id}
          agentName={primarySessionFixture.agent_name}
          canPrompt
          onCancelPrompt={options.onCancelPrompt ?? (() => {})}
        />
        {options.includeToaster ? <Toaster duration={500} /> : null}
      </SessionChatRuntimeProvider>
    </QueryClientProvider>
  );
  const utils = render(options.strictMode ? <StrictMode>{tree}</StrictMode> : tree);
  return { ...utils, queryClient };
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
  let lastPromptClientMessageID = "";
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    useSessionStore.getState().clearDraft(primarySessionFixture.id);
    useSessionStore.setState({ goalResults: {}, goalResultCommands: {}, goalErrorVisible: {} });
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
    lastPromptClientMessageID = "";
    fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const requestURL = getRequestURL(input);
      const pathname = requestURL.pathname;

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
        `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/${primarySessionFixture.id}/clear`
      ) {
        if (clearResponsePromise) return clearResponsePromise;
        return jsonResponse({ session: sessionDetailResponse });
      }

      if (
        pathname ===
        `/api/workspaces/${primarySessionFixture.workspace_id}/sessions/${primarySessionFixture.id}/prompt`
      ) {
        lastPromptClientMessageID = getLastUserMessageID(init);
        if (promptResponsePromise) {
          return abortableResponse(promptResponsePromise, init?.signal);
        }
        if (goalCommandFailureReason) {
          return jsonResponse(
            {
              outcome: "error",
              reason_code: goalCommandFailureReason,
              replaced_run_id: null,
              snapshot: null,
            },
            { status: 422 }
          );
        }
        return sseResponse([
          'data: {"type":"start","messageId":"turn-runtime-001"}\n\n',
          'data: {"type":"text-start","id":"turn-runtime-001-text-1"}\n\n',
          'data: {"type":"text-delta","id":"turn-runtime-001-text-1","delta":"Live runtime answer before transcript reconciliation."}\n\n',
          'data: {"type":"text-end","id":"turn-runtime-001-text-1"}\n\n',
          'data: {"type":"finish","finishReason":"stop"}\n\n',
          "data: [DONE]\n\n",
        ]);
      }

      throw new Error(`Unhandled fetch in test: ${pathname}`);
    });

    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    useSessionStore.getState().clearDraft(primarySessionFixture.id);
    useSessionStore.setState({ goalResults: {}, goalResultCommands: {}, goalErrorVisible: {} });
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
    let viewportElement: HTMLElement | null = null;
    let olderResolved = false;
    const anchorMessageId = sessionTranscriptFixture[1]!.id;
    vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(
      function (this: HTMLElement) {
        if (this === viewportElement) return testRect(0, 600);
        if (this.dataset.messageId === anchorMessageId) {
          return testRect(olderResolved ? 360 : 100);
        }
        return testRect(700);
      }
    );

    renderSessionThread({
      eventSourceFactory: url => {
        const source = new FakeSessionEventSource(url);
        sources.push(source);
        return source;
      },
    });

    const loadOlder = await screen.findByRole("button", { name: "Load older messages" });
    const viewport = screen.getByTestId("chat-view");
    viewportElement = viewport;
    Object.defineProperty(viewport, "scrollHeight", {
      configurable: true,
      get: () => (olderResolved ? 1_500 : 1_000),
    });
    viewport.scrollTop = 320;

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
    await waitFor(() => expect(viewport.scrollTop).toBe(580));
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

  it("Should preserve the first Goal transport and local cancellation across StrictMode replay", async () => {
    const prompt =
      '/goal Reply in exactly one sentence: "AGH keeps agent work local-first and durable."';
    const promptResponse = createDeferred<Response>();
    const onCancelPrompt = vi.fn();
    const sources: FakeSessionEventSource[] = [];
    const user = userEvent.setup();
    transcriptMessages = [
      {
        id: "session-create-hook-start",
        role: "assistant",
        parts: [{ type: "data-agh-event", data: { type: "hook.dispatch.start" } }],
      },
      {
        id: "session-create-hook-complete",
        role: "assistant",
        parts: [{ type: "data-agh-event", data: { type: "hook.dispatch.complete" } }],
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
      const composer = await screen.findByRole("textbox", { name: "Session prompt" });
      await waitFor(() => {
        expect(screen.getAllByTestId("thread-message-row")).toHaveLength(2);
        expect(composer).toBeEnabled();
        expect(sources).toHaveLength(2);
        expect(sources[0]?.closed).toBe(true);
        expect(sources[1]?.closed).toBe(false);
      });
      expect(countPromptFetches(fetchMock)).toBe(0);

      fireEvent.change(composer, { target: { value: prompt } });
      await user.click(screen.getByTestId("composer-send-button"));

      expect(await screen.findByText(prompt)).toBeInTheDocument();
      expect(await screen.findByLabelText("Working")).toBeInTheDocument();
      await waitFor(() => expect(countPromptFetches(fetchMock)).toBe(1));

      await user.click(screen.getByTestId("composer-stop-button"));

      expect(onCancelPrompt).toHaveBeenCalledTimes(1);
      await waitFor(() => {
        expect(screen.queryByTestId("composer-stop-button")).not.toBeInTheDocument();
        expect(screen.getByTestId("composer-send-button")).toBeEnabled();
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

      const composer = await screen.findByRole("textbox", { name: "Session prompt" });
      fireEvent.change(composer, { target: { value: prompt } });
      await user.click(screen.getByTestId("composer-send-button"));

      await waitFor(() => expect(countPromptFetches(fetchMock)).toBe(1));
      expect(useSessionStore.getState().goalResults[primarySessionFixture.id]?.reason_code).toBe(
        reasonCode
      );
      expect(await screen.findByRole("alert")).toHaveTextContent(guidance);
      expect(screen.queryByText(reasonCode)).not.toBeInTheDocument();
      expect(composer).toBeEnabled();
      await user.type(composer, "Retry draft");
      expect(composer).toHaveValue("Retry draft");
      expect(screen.getByRole("alert")).toHaveTextContent(guidance);
      expect(screen.getByTestId("composer-send-button")).toBeEnabled();
      expect(countPromptFetches(fetchMock)).toBe(1);

      goalCommandFailureReason = null;
      await user.click(screen.getByTestId("composer-send-button"));
      await waitFor(() => expect(countPromptFetches(fetchMock)).toBe(2));
      await waitFor(() => expect(screen.queryByRole("alert")).not.toBeInTheDocument());
      expect(useSessionStore.getState().goalResults[primarySessionFixture.id]?.reason_code).toBe(
        reasonCode
      );
    }
  );

  it("keeps locally sent prompts visible through transcript reconciliation", async () => {
    const user = userEvent.setup();
    const { queryClient } = renderSessionThread();

    await waitFor(() => {
      expect(screen.getByText("Launch readiness snapshot")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByTestId("composer-textarea"), {
      target: { value: "Continue from the reattached thread" },
    });
    await user.click(screen.getByTestId("composer-send-button"));

    await waitFor(() => {
      expect(screen.getByText("Continue from the reattached thread")).toBeInTheDocument();
      expect(
        screen.getByText("Live runtime answer before transcript reconciliation.")
      ).toBeInTheDocument();
    });
    expect(lastPromptClientMessageID).not.toBe("");

    transcriptMessages = [
      ...sessionTranscriptFixture.slice(0, 2),
      {
        id: "transcript_user_after_send_001",
        role: "user",
        metadata: { client_message_id: lastPromptClientMessageID },
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

  it("Should hide a completed runtime tail while an authoritative Clear is pending", async () => {
    transcriptMessages = [];
    const clearResponse = createDeferred<Response>();
    clearResponsePromise = clearResponse.promise;
    const user = userEvent.setup();
    renderSessionThread({ includeClearAction: true, includeTranscriptStateProbe: true });

    const composer = await screen.findByRole("textbox", { name: "Session prompt" });
    const prompt = "Clear me";
    await user.type(composer, prompt);
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

  it("promotes an optimistic runtime message to the server identity without a duplicate row", () => {
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
        metadata: {
          turn_id: "turn_promote_001",
          client_temp_id: "client_temp_assistant_001",
        },
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

    expect(merged.map(message => message.id)).toEqual(["server_assistant_001"]);
  });

  it("Should reconcile a user message by client identity without requiring a turn id", () => {
    const runtimeMessages = toReadonlyThreadMessages([
      {
        id: "client_user_001",
        role: "user",
        parts: [{ type: "text", text: "One authored prompt.", state: "done" }],
      } as TranscriptMessage,
    ]);
    const transcriptMessages = toReadonlyThreadMessages([
      {
        id: "server_user_001",
        role: "user",
        metadata: { client_message_id: "client_user_001" },
        parts: [{ type: "text", text: "One authored prompt.", state: "done" }],
      } as TranscriptMessage,
      {
        id: "server_assistant_001",
        role: "assistant",
        parts: [{ type: "text", text: "One durable response.", state: "done" }],
      } as TranscriptMessage,
    ]);

    const merged = mergeSessionThreadReadModel({ transcriptMessages, runtimeMessages });

    expect(merged.map(message => message.id)).toEqual(["server_user_001", "server_assistant_001"]);
  });

  it("Should expose one focused and cancellable Goal draft without submitting the response", async () => {
    const response =
      "Validate that one event creates one automation run and one terminal loop result.";
    transcriptMessages = [
      {
        id: "assistant-earlier-goal",
        role: "assistant",
        parts: [
          {
            type: "text",
            text: "Confirm the trigger is disabled before changing its configuration.",
            state: "done",
          },
        ],
      },
      {
        id: "assistant-draft-goal",
        role: "assistant",
        parts: [
          {
            type: "text",
            text: response,
            state: "done",
          },
        ],
      },
    ];
    const user = userEvent.setup();
    renderSessionThread({ includeToaster: true });

    const actions = await screen.findAllByRole("button", { name: "Use as Goal" });
    expect(actions).toHaveLength(2);
    await user.click(actions[1]!);
    const composer = screen.getByRole("textbox", { name: "Session prompt" });
    expect(composer).toHaveValue(`/goal ${response}`);
    expect(composer).toHaveFocus();
    expect(screen.getByRole("status")).toHaveTextContent("Goal command draft");
    expect(countPromptFetches(fetchMock)).toBe(0);

    await user.click(screen.getByRole("button", { name: "Discard Goal command" }));
    expect(composer).toHaveValue("");
    expect(screen.queryByText("Goal command draft")).not.toBeInTheDocument();
    expect(countPromptFetches(fetchMock)).toBe(0);

    await user.type(composer, "Keep this operator draft.");
    await user.click(screen.getAllByRole("button", { name: "Use as Goal" })[1]!);
    expect(composer).toHaveValue("Keep this operator draft.");
    expect(composer).toHaveFocus();
    expect(
      screen.getByText("Send or discard the current draft before prefilling a Goal command.")
    ).toBeVisible();
    expect(countPromptFetches(fetchMock)).toBe(0);
    await user.clear(composer);
    expect(composer).toHaveValue("");
  });

  it("Should activate Use as Goal from the keyboard without duplicate submission", async () => {
    transcriptMessages = [
      {
        id: "assistant-earlier-keyboard-goal",
        role: "assistant",
        parts: [
          {
            type: "text",
            text: "Inspect the existing run before retrying it.",
            state: "done",
          },
        ],
      },
      {
        id: "assistant-keyboard-goal",
        role: "assistant",
        parts: [
          {
            type: "text",
            text: "Expand the release checks and cite each result.",
            state: "done",
          },
        ],
      },
    ];
    const user = userEvent.setup();
    renderSessionThread();

    const actions = await screen.findAllByRole("button", { name: "Use as Goal" });
    expect(actions).toHaveLength(2);
    const action = actions[1]!;
    action.focus();
    await user.keyboard("{Enter}");

    expect(screen.getByRole("textbox", { name: "Session prompt" })).toHaveValue(
      "/goal Expand the release checks and cite each result."
    );
    expect(countPromptFetches(fetchMock)).toBe(0);
    await user.click(screen.getByRole("button", { name: "Discard Goal command" }));
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
            type: "data-agh-event",
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
            type: "data-agh-event",
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
            type: "data-agh-event",
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

  it("renders only unresolved permission events as interactive prompts", async () => {
    transcriptMessages = [
      ...sessionTranscriptFixture.slice(0, 1),
      {
        id: "transcript_permission_001",
        role: "assistant",
        parts: [
          {
            type: "data-agh-permission",
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
            type: "data-agh-permission",
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

    await waitFor(() => {
      expect(screen.getByTestId("permission-prompt")).toBeInTheDocument();
    });

    expect(screen.getAllByTestId("permission-prompt")).toHaveLength(1);
    expect(screen.getByTestId("permission-prompt")).toHaveTextContent("pending.txt");
  }, 10_000);

  it("routes a clarify data event to the clarification receipt, not the activity notice", async () => {
    transcriptMessages = [
      ...sessionTranscriptFixture.slice(0, 1),
      {
        id: "transcript_clarify_001",
        role: "assistant",
        parts: [
          {
            type: "data-agh-event",
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

    // The settled turn folds its preamble + data behind a "Worked" disclosure and
    // keeps the terminal answer visible; the data part is folded away, not dropped.
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
    expect(within(chat).getByTestId("session-data-part")).toHaveTextContent("Provider note");
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

  it("Should render large transcript histories in message order", async () => {
    transcriptMessages = Array.from({ length: 80 }, (_, index) => ({
      id: `transcript_large_${index}`,
      role: index % 2 === 0 ? "user" : "assistant",
      parts: [{ type: "text", text: `Large transcript message ${index}`, state: "done" }],
    })) as TranscriptMessage[];

    renderSessionThread();

    await waitFor(() => {
      expect(screen.getAllByTestId("thread-message-row")).toHaveLength(80);
    });

    const rowIndexes = screen
      .getAllByTestId("thread-message-row")
      .map(row => Number(row.getAttribute("data-message-index")));
    expect(rowIndexes).toEqual(Array.from({ length: 80 }, (_, index) => index));
    expect(screen.getByText("Large transcript message 0")).toBeInTheDocument();
    expect(screen.getByText("Large transcript message 79")).toBeInTheDocument();
  }, 10_000);
});
