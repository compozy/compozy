// Suite: useSessionLiveTail
// Invariants: the infinite transcript cache owns cursor/fences, preserves loaded history, and
// retains readonly array identity while the exact message sequence is unchanged; Goal snapshot
// frames invalidate only the exact Goal cache without mutating transcript pages.
// Boundary IN: REST transcript pages, transcript SSE frames, Goal frames, and terminal frames.
// Boundary OUT: real HTTP transport and final thread visuals.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { primarySessionFixture, sessionTranscriptFixture } from "../../mocks/fixtures";
import { sessionKeys } from "../../lib/query-keys";
import type { SessionTranscriptData } from "../../lib/session-transcript-query";
import type {
  NormalizedSessionTranscriptResponse,
  SessionMessage,
  SessionPayload,
  SessionState,
  SessionTranscriptPage,
} from "../../types";
import {
  getSessionDebugCounters,
  resetSessionDebugTelemetry,
  SESSION_DEBUG_EVENTS,
} from "../../lib/session-observability";
import { useSessionLiveTail } from "../use-session-live-tail";
import type { SessionStreamEventSource } from "../use-session-live-tail";

interface StreamCursor {
  afterSequence?: number;
  epoch?: number;
  generation?: number;
}

vi.mock("../../adapters/session-api", () => ({
  buildSessionStreamUrl: (workspaceId: string, id: string, cursor: StreamCursor = {}) => {
    const path = `/api/workspaces/${encodeURIComponent(workspaceId)}/sessions/${encodeURIComponent(id)}/stream`;
    const params = new URLSearchParams({ frames: "transcript" });
    if (cursor.afterSequence !== undefined && cursor.afterSequence > 0) {
      params.set("after_sequence", String(cursor.afterSequence));
    }
    if (cursor.epoch !== undefined) params.set("epoch", String(cursor.epoch));
    if (cursor.generation !== undefined) params.set("generation", String(cursor.generation));
    return `${path}?${params.toString()}`;
  },
  fetchSession: vi.fn(),
  fetchSessionEvents: vi.fn(),
  fetchSessionHistory: vi.fn(),
  fetchSessionLedger: vi.fn(),
  fetchSessionRecap: vi.fn(),
  fetchSessionTranscript: vi.fn(),
  fetchSessions: vi.fn(),
  SessionApiError: class SessionApiError extends Error {},
  SessionLedgerUnavailableError: class SessionLedgerUnavailableError extends Error {},
  SessionNotFoundError: class SessionNotFoundError extends Error {},
}));

import { fetchSession, fetchSessionTranscript } from "../../adapters/session-api";

function fixtureWorkspaceId(): string {
  const workspaceId = primarySessionFixture.workspace_id;
  if (!workspaceId) throw new Error("primary session fixture must include workspace_id");
  return workspaceId;
}

const WORKSPACE_ID = fixtureWorkspaceId();
const SESSION_ID = primarySessionFixture.id;
const STREAM_URL = `/api/workspaces/${WORKSPACE_ID}/sessions/${SESSION_ID}/stream?frames=transcript`;

class FakeSessionEventSource implements SessionStreamEventSource {
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

  emit(type: string, payload: unknown, lastEventId = "") {
    const event = { data: JSON.stringify(payload), lastEventId } as MessageEvent;
    for (const listener of this.listeners.get(type) ?? []) {
      if (typeof listener === "function") listener(event);
      else listener.handleEvent(event);
    }
  }

  close() {
    this.closed = true;
  }
}

function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
}

function createWrapper(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

function sessionWithState(state: SessionState): SessionPayload {
  return { ...primarySessionFixture, state };
}

function transcriptResponse(
  messages: SessionMessage[],
  options: {
    epoch?: number;
    generation?: number;
    firstSequence?: number;
    hasOlder?: boolean;
    limit?: number;
    nextBeforeSequence?: number;
  } = {}
): NormalizedSessionTranscriptResponse {
  const firstSequence = options.firstSequence ?? 1;
  const entries = messages.map((message, index) => ({
    message,
    sequence: firstSequence + index,
    start_sequence: firstSequence + index,
  }));
  const maxSequence = entries.at(-1)?.sequence ?? 0;
  return {
    entries,
    epoch: options.epoch ?? 1,
    generation: options.generation ?? 1,
    has_older: options.hasOlder ?? false,
    limit: options.limit ?? Math.max(messages.length, 200),
    max_sequence: maxSequence,
    ...(options.nextBeforeSequence === undefined
      ? {}
      : { next_before_sequence: options.nextBeforeSequence }),
  };
}

function transcriptPage(
  messages: SessionMessage[],
  options: Parameters<typeof transcriptResponse>[1] = {}
): SessionTranscriptPage {
  const response = transcriptResponse(messages, options);
  return { ...response, cursor: response.max_sequence };
}

function seededTranscriptData(...pages: SessionTranscriptPage[]): SessionTranscriptData {
  return { pages, pageParams: pages.map(() => undefined) };
}

function textMessage(id: string, text: string): SessionMessage {
  return { id, role: "assistant", parts: [{ type: "text", text }] };
}

function clarifyMessage(id: string): SessionMessage {
  return {
    id,
    role: "assistant",
    parts: [
      {
        type: "data-agh-event",
        data: {
          type: "clarify",
          session_id: SESSION_ID,
          turn_id: "clarify:req-1",
          raw: {
            status: "pending",
            request: {
              request_id: "req-1",
              workspace_id: WORKSPACE_ID,
              session_id: SESSION_ID,
              agent_name: primarySessionFixture.agent_name,
              question: "Which path?",
              choices: ["Fast", "Safe"],
              asked_at: "2026-07-16T00:00:00Z",
              deadline: "2026-07-16T00:05:00Z",
            },
            at: "2026-07-16T00:00:00Z",
          },
        },
      },
    ] as unknown as SessionMessage["parts"],
  };
}

function clarifyDeltaFrame(message: SessionMessage) {
  return {
    cursor: 2,
    entries: [{ message, sequence: 2, start_sequence: 2 }],
    epoch: 1,
    generation: 1,
    has_more: false,
    max_sequence: 2,
    session_id: SESSION_ID,
  };
}

function renderLiveTail(
  options: {
    queryClient?: QueryClient;
    sources?: FakeSessionEventSource[];
  } = {}
) {
  const queryClient = options.queryClient ?? createQueryClient();
  const sources = options.sources ?? [];
  const eventSourceFactory = (url: string) => {
    const source = new FakeSessionEventSource(url);
    sources.push(source);
    return source;
  };
  const rendered = renderHook(
    () =>
      useSessionLiveTail({
        workspaceId: WORKSPACE_ID,
        sessionId: SESSION_ID,
        eventSourceFactory,
      }),
    { wrapper: createWrapper(queryClient) }
  );
  return { ...rendered, queryClient, sources };
}

function seedActiveSession(queryClient: QueryClient) {
  queryClient.setQueryData(
    sessionKeys.detail(WORKSPACE_ID, SESSION_ID),
    sessionWithState("active")
  );
}

describe("useSessionLiveTail", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    resetSessionDebugTelemetry();
    vi.mocked(fetchSession).mockResolvedValue(sessionWithState("active"));
    vi.mocked(fetchSessionTranscript).mockResolvedValue(transcriptResponse([]));
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
    resetSessionDebugTelemetry();
  });

  it("Should open the stream even when the initial transcript request fails", async () => {
    const transcriptError = new Error("transcript endpoint returned 500");
    vi.mocked(fetchSessionTranscript).mockRejectedValue(transcriptError);

    const { result, sources } = renderLiveTail();

    await waitFor(() => expect(sources).toHaveLength(1));
    await waitFor(() => expect(result.current.isError).toBe(true));

    expect(sources[0]?.url).toBe(STREAM_URL);
    expect(result.current.error).toBe(transcriptError);
    expect(result.current.messages).toEqual([]);
    expect(getSessionDebugCounters()[SESSION_DEBUG_EVENTS.transcriptFetchFailed]).toBe(1);
  });

  it("Should recover a failed live transcript through the bounded polling refetch", async () => {
    vi.useFakeTimers();
    vi.mocked(fetchSessionTranscript)
      .mockRejectedValueOnce(new Error("transcript endpoint returned 500"))
      .mockResolvedValueOnce(transcriptResponse([sessionTranscriptFixture[0]!]));
    const queryClient = createQueryClient();
    seedActiveSession(queryClient);

    const { result } = renderLiveTail({ queryClient });

    await act(async () => {
      await vi.waitFor(() => expect(result.current.isError).toBe(true));
      await vi.advanceTimersByTimeAsync(5_000);
      await vi.waitFor(() => expect(result.current.messages).toHaveLength(1));
    });
    expect(fetchSessionTranscript).toHaveBeenCalledTimes(2);
  });

  it("Should append older pages and preserve them across a same-fence snapshot", async () => {
    vi.mocked(fetchSessionTranscript)
      .mockResolvedValueOnce(
        transcriptResponse([sessionTranscriptFixture[1]!], {
          firstSequence: 20,
          hasOlder: true,
          nextBeforeSequence: 20,
        })
      )
      .mockResolvedValueOnce(
        transcriptResponse([sessionTranscriptFixture[0]!], { firstSequence: 1 })
      );
    const queryClient = createQueryClient();
    seedActiveSession(queryClient);
    const { result, sources } = renderLiveTail({ queryClient });

    await waitFor(() => expect(result.current.messages).toHaveLength(1));
    act(() => result.current.loadOlder());
    await waitFor(() => expect(result.current.messages).toHaveLength(2));
    expect(fetchSessionTranscript).toHaveBeenNthCalledWith(
      2,
      WORKSPACE_ID,
      SESSION_ID,
      { before_sequence: 20 },
      expect.any(AbortSignal)
    );

    act(() => {
      sources[0]?.emit(
        "transcript_snapshot",
        {
          entries: [
            {
              message: sessionTranscriptFixture[2]!,
              sequence: 30,
              start_sequence: 30,
            },
          ],
          epoch: 1,
          generation: 1,
          has_older: true,
          max_sequence: 30,
          next_before_sequence: 20,
          reset: false,
          session_id: SESSION_ID,
        },
        "30"
      );
    });

    await waitFor(() => expect(result.current.messages).toHaveLength(3));
    expect(result.current.messages.map(message => message.id)).toEqual([
      sessionTranscriptFixture[0]?.id,
      sessionTranscriptFixture[1]?.id,
      sessionTranscriptFixture[2]?.id,
    ]);
    expect(
      queryClient.getQueryData<SessionTranscriptData>(
        sessionKeys.transcript(WORKSPACE_ID, SESSION_ID)
      )?.pages
    ).toHaveLength(2);
  });

  it("Should not poll immutable older pages while the healthy SSE stream is open", async () => {
    vi.useFakeTimers();
    vi.mocked(fetchSessionTranscript)
      .mockResolvedValueOnce(
        transcriptResponse([sessionTranscriptFixture[1]!], {
          firstSequence: 20,
          hasOlder: true,
          nextBeforeSequence: 20,
        })
      )
      .mockResolvedValueOnce(
        transcriptResponse([sessionTranscriptFixture[0]!], { firstSequence: 1 })
      );
    const queryClient = createQueryClient();
    seedActiveSession(queryClient);
    const { result } = renderLiveTail({ queryClient });

    await act(async () => {
      await vi.waitFor(() => expect(result.current.messages).toHaveLength(1));
      result.current.loadOlder();
      await vi.waitFor(() => expect(result.current.messages).toHaveLength(2));
    });
    vi.mocked(fetchSessionTranscript).mockClear();

    await act(async () => vi.advanceTimersByTimeAsync(20_000));

    expect(fetchSessionTranscript).not.toHaveBeenCalled();
    expect(result.current.messages).toHaveLength(2);
  });

  it("Should replace every loaded page only for an explicit reset snapshot", async () => {
    vi.useFakeTimers();
    const queryClient = createQueryClient();
    seedActiveSession(queryClient);
    queryClient.setQueryData(
      sessionKeys.transcript(WORKSPACE_ID, SESSION_ID),
      seededTranscriptData(
        transcriptPage([sessionTranscriptFixture[1]!], {
          firstSequence: 20,
          hasOlder: true,
          nextBeforeSequence: 20,
        }),
        transcriptPage([sessionTranscriptFixture[0]!], { firstSequence: 1 })
      )
    );
    const { result, sources } = renderLiveTail({ queryClient });
    await act(async () => {
      await vi.waitFor(() => expect(sources).toHaveLength(1));
    });

    act(() => {
      sources[0]?.emit(
        "transcript_snapshot",
        {
          entries: [
            {
              message: sessionTranscriptFixture[2]!,
              sequence: 5,
              start_sequence: 5,
            },
          ],
          epoch: 2,
          generation: 1,
          has_older: false,
          max_sequence: 5,
          reason: "epoch_mismatch",
          reset: true,
          session_id: SESSION_ID,
        },
        "5"
      );
    });
    await act(async () => {
      await vi.waitFor(() =>
        expect(result.current.messages.map(message => message.id)).toEqual([
          sessionTranscriptFixture[2]?.id,
        ])
      );
    });

    expect(
      queryClient.getQueryData<SessionTranscriptData>(
        sessionKeys.transcript(WORKSPACE_ID, SESSION_ID)
      )?.pages
    ).toHaveLength(1);
    act(() => sources[0]?.onerror?.(new Event("error")));
    await act(async () => vi.advanceTimersByTimeAsync(250));
    expect(sources[1]?.url).toBe(`${STREAM_URL}&after_sequence=5&epoch=2&generation=1`);
  });

  it("Should order and deduplicate delta batches by stable start sequence", async () => {
    vi.mocked(fetchSessionTranscript).mockResolvedValue(
      transcriptResponse([sessionTranscriptFixture[0]!, sessionTranscriptFixture[1]!], {
        firstSequence: 4,
      })
    );
    const revisedSecond = {
      ...sessionTranscriptFixture[1]!,
      parts: [{ type: "text", text: "Revised launch summary.", state: "done" }],
    } as SessionMessage;
    const { result, sources } = renderLiveTail();
    await waitFor(() => expect(result.current.messages).toHaveLength(2));

    act(() => {
      sources[0]?.emit(
        "transcript_delta",
        {
          cursor: 8,
          entries: [
            { message: sessionTranscriptFixture[2]!, sequence: 7, start_sequence: 7 },
            { message: revisedSecond, sequence: 8, start_sequence: 5 },
          ],
          epoch: 1,
          generation: 1,
          has_more: false,
          max_sequence: 8,
          session_id: SESSION_ID,
        },
        "8"
      );
    });

    await waitFor(() => expect(result.current.messages).toHaveLength(3));
    expect(result.current.messages.map(message => message.id)).toEqual([
      sessionTranscriptFixture[0]?.id,
      sessionTranscriptFixture[1]?.id,
      sessionTranscriptFixture[2]?.id,
    ]);
    expect(JSON.stringify(result.current.messages[1])).toContain("Revised launch summary");
  });

  it("Should keep every page bounded while rolling live overflow into loaded history", async () => {
    const messages = Array.from({ length: 6 }, (_, index) =>
      textMessage(`bounded-${index + 1}`, `Bounded message ${index + 1}`)
    );
    const queryClient = createQueryClient();
    seedActiveSession(queryClient);
    queryClient.setQueryData(
      sessionKeys.transcript(WORKSPACE_ID, SESSION_ID),
      seededTranscriptData(
        transcriptPage(messages.slice(2, 4), {
          firstSequence: 3,
          hasOlder: true,
          nextBeforeSequence: 3,
          limit: 2,
        }),
        transcriptPage(messages.slice(0, 2), { firstSequence: 1, limit: 2 })
      )
    );
    const { result, sources } = renderLiveTail({ queryClient });
    await waitFor(() => expect(result.current.messages).toHaveLength(4));

    act(() => {
      sources[0]?.emit(
        "transcript_delta",
        {
          cursor: 6,
          entries: [
            { message: messages[4]!, sequence: 5, start_sequence: 5 },
            { message: messages[5]!, sequence: 6, start_sequence: 6 },
          ],
          epoch: 1,
          generation: 1,
          has_more: false,
          max_sequence: 6,
          session_id: SESSION_ID,
        },
        "6"
      );
    });
    await waitFor(() => expect(result.current.messages).toHaveLength(6));

    let data = queryClient.getQueryData<SessionTranscriptData>(
      sessionKeys.transcript(WORKSPACE_ID, SESSION_ID)
    );
    expect(data?.pages.map(page => page.entries.length)).toEqual([2, 2, 2]);
    expect(data?.pages[0]?.cursor).toBe(6);
    expect(result.current.messages.map(message => message.id)).toEqual(
      messages.map(message => message.id)
    );

    const revisedFourth = textMessage("bounded-4", "Bounded message 4 revised");
    act(() => {
      sources[0]?.emit(
        "transcript_delta",
        {
          cursor: 8,
          entries: [{ message: revisedFourth, sequence: 8, start_sequence: 4 }],
          epoch: 1,
          generation: 1,
          has_more: false,
          max_sequence: 8,
          session_id: SESSION_ID,
        },
        "8"
      );
    });
    await waitFor(() =>
      expect(JSON.stringify(result.current.messages)).toContain("Bounded message 4 revised")
    );

    data = queryClient.getQueryData<SessionTranscriptData>(
      sessionKeys.transcript(WORKSPACE_ID, SESSION_ID)
    );
    expect(data?.pages.every(page => page.entries.length <= page.limit)).toBe(true);
    expect(data?.pages[0]?.cursor).toBe(8);
    expect(result.current.messages).toHaveLength(6);
  });

  it("Should advance the cache cursor for an empty delta and reconnect from cache fences", async () => {
    vi.useFakeTimers();
    const queryClient = createQueryClient();
    seedActiveSession(queryClient);
    const { result, sources } = renderLiveTail({ queryClient });
    await act(async () => {
      await vi.waitFor(() => expect(result.current.status).toBe("success"));
    });

    act(() => {
      sources[0]?.emit(
        "transcript_delta",
        {
          cursor: 12,
          entries: [],
          epoch: 1,
          generation: 1,
          has_more: false,
          max_sequence: 12,
          session_id: SESSION_ID,
        },
        "12"
      );
    });
    await act(async () => {
      await vi.waitFor(() => {
        const data = queryClient.getQueryData<SessionTranscriptData>(
          sessionKeys.transcript(WORKSPACE_ID, SESSION_ID)
        );
        expect(data?.pages[0]?.cursor).toBe(12);
      });
    });

    act(() => sources[0]?.onerror?.(new Event("error")));
    await act(async () => vi.advanceTimersByTimeAsync(250));
    expect(sources[1]?.url).toBe(`${STREAM_URL}&after_sequence=12&epoch=1&generation=1`);
  });

  it("Should read an externally advanced cache cursor when reconnecting", async () => {
    vi.useFakeTimers();
    const queryClient = createQueryClient();
    seedActiveSession(queryClient);
    queryClient.setQueryData(
      sessionKeys.transcript(WORKSPACE_ID, SESSION_ID),
      seededTranscriptData(transcriptPage([], { epoch: 4, generation: 7 }))
    );
    const { sources } = renderLiveTail({ queryClient });
    await act(async () => {
      await vi.waitFor(() => expect(sources).toHaveLength(1));
    });
    queryClient.setQueryData<SessionTranscriptData>(
      sessionKeys.transcript(WORKSPACE_ID, SESSION_ID),
      existing => {
        const head = existing?.pages[0];
        return existing && head
          ? { ...existing, pages: [{ ...head, cursor: 44 }, ...existing.pages.slice(1)] }
          : existing;
      }
    );

    act(() => sources[0]?.onerror?.(new Event("error")));
    await act(async () => vi.advanceTimersByTimeAsync(250));
    expect(sources[1]?.url).toBe(`${STREAM_URL}&after_sequence=44&epoch=4&generation=7`);
  });

  it("Should invalidate Goal without advancing the transcript cursor or replacing pages", async () => {
    vi.useFakeTimers();
    const queryClient = createQueryClient();
    seedActiveSession(queryClient);
    const transcriptKey = sessionKeys.transcript(WORKSPACE_ID, SESSION_ID);
    const goalKey = sessionKeys.goal(WORKSPACE_ID, SESSION_ID);
    queryClient.setQueryData(
      transcriptKey,
      seededTranscriptData(
        transcriptPage([textMessage("tail-3", "Tail 3"), textMessage("tail-4", "Tail 4")], {
          firstSequence: 3,
          hasOlder: true,
          limit: 2,
          nextBeforeSequence: 3,
        }),
        transcriptPage([textMessage("older-1", "Older 1"), textMessage("older-2", "Older 2")], {
          firstSequence: 1,
          limit: 2,
        })
      )
    );
    queryClient.setQueryData(goalKey, null);

    const { result, sources } = renderLiveTail({ queryClient });
    await act(async () => {
      await vi.waitFor(() => {
        expect(sources).toHaveLength(1);
        expect(result.current.status).toBe("success");
      });
    });

    const transcriptBefore = queryClient.getQueryData<SessionTranscriptData>(transcriptKey);
    const invalidateQueries = vi.spyOn(queryClient, "invalidateQueries");

    act(() => {
      sources[0]?.emit(
        "goal_snapshot_changed",
        {
          bound_session_id: SESSION_ID,
          cause: "status",
          revision: 7,
          run_id: "run-goal-1",
          session_id: SESSION_ID,
        },
        "99"
      );
    });

    expect(invalidateQueries).toHaveBeenCalledWith({
      queryKey: goalKey,
      exact: true,
    });
    expect(queryClient.getQueryData<SessionTranscriptData>(transcriptKey)).toBe(transcriptBefore);
    expect(
      transcriptBefore?.pages.map(page => page.entries.map(entry => entry.start_sequence))
    ).toEqual([
      [3, 4],
      [1, 2],
    ]);

    act(() => sources[0]?.onerror?.(new Event("error")));
    await act(async () => vi.advanceTimersByTimeAsync(250));

    expect(sources[1]?.url).toBe(`${STREAM_URL}&after_sequence=4&epoch=1&generation=1`);
  });

  it("Should wake the exact clarifications projection on a clarify transcript delta", async () => {
    vi.mocked(fetchSessionTranscript).mockResolvedValue(
      transcriptResponse([textMessage("m1", "hi")])
    );
    const { result, sources, queryClient } = renderLiveTail();
    await waitFor(() => expect(result.current.messages).toHaveLength(1));

    const invalidateQueries = vi.spyOn(queryClient, "invalidateQueries");
    act(() => {
      sources[0]?.emit("transcript_delta", clarifyDeltaFrame(clarifyMessage("clarify-1")), "2");
    });

    expect(invalidateQueries).toHaveBeenCalledWith({
      queryKey: sessionKeys.clarifications(WORKSPACE_ID, SESSION_ID),
      exact: true,
    });
  });

  it("Should not wake the clarifications projection on an ordinary transcript delta", async () => {
    vi.mocked(fetchSessionTranscript).mockResolvedValue(
      transcriptResponse([textMessage("m1", "hi")])
    );
    const { result, sources, queryClient } = renderLiveTail();
    await waitFor(() => expect(result.current.messages).toHaveLength(1));

    const invalidateQueries = vi.spyOn(queryClient, "invalidateQueries");
    act(() => {
      sources[0]?.emit(
        "transcript_delta",
        clarifyDeltaFrame(textMessage("m2", "plain answer")),
        "2"
      );
    });

    expect(invalidateQueries).not.toHaveBeenCalledWith({
      queryKey: sessionKeys.clarifications(WORKSPACE_ID, SESSION_ID),
      exact: true,
    });
  });

  it("Should reject a mismatched delta and reconnect without replacing history", async () => {
    vi.useFakeTimers();
    vi.mocked(fetchSessionTranscript).mockResolvedValue(
      transcriptResponse([sessionTranscriptFixture[0]!])
    );
    const { result, sources } = renderLiveTail();
    await act(async () => {
      await vi.waitFor(() => expect(result.current.messages).toHaveLength(1));
    });

    act(() => {
      sources[0]?.emit(
        "transcript_delta",
        {
          cursor: 9,
          entries: [{ message: sessionTranscriptFixture[1]!, sequence: 9, start_sequence: 9 }],
          epoch: 1,
          generation: 2,
          has_more: false,
          max_sequence: 9,
          session_id: SESSION_ID,
        },
        "9"
      );
    });
    await act(async () => vi.advanceTimersByTimeAsync(250));

    expect(result.current.messages.map(message => message.id)).toEqual([
      sessionTranscriptFixture[0]?.id,
    ]);
    expect(sources[0]?.closed).toBe(true);
    expect(sources[1]?.url).toBe(`${STREAM_URL}&after_sequence=1&epoch=1&generation=1`);
  });

  it("Should preserve unchanged ThreadMessage identities across a delta update", async () => {
    vi.mocked(fetchSessionTranscript).mockResolvedValue(
      transcriptResponse([sessionTranscriptFixture[0]!, sessionTranscriptFixture[1]!])
    );
    const { result, rerender, sources } = renderLiveTail();
    await waitFor(() => expect(result.current.messages).toHaveLength(2));
    const unchanged = result.current.messages;
    const first = result.current.messages[0];
    const second = result.current.messages[1];

    rerender();
    expect(result.current.messages).toBe(unchanged);

    act(() => {
      sources[0]?.emit(
        "transcript_delta",
        {
          cursor: 3,
          entries: [{ message: sessionTranscriptFixture[2]!, sequence: 3, start_sequence: 3 }],
          epoch: 1,
          generation: 1,
          has_more: false,
          max_sequence: 3,
          session_id: SESSION_ID,
        },
        "3"
      );
    });
    await waitFor(() => expect(result.current.messages).toHaveLength(3));

    expect(result.current.messages[0]).toBe(first);
    expect(result.current.messages[1]).toBe(second);
  });

  it("Should back off exponentially without refetching on transport errors", async () => {
    vi.useFakeTimers();
    const queryClient = createQueryClient();
    seedActiveSession(queryClient);
    const { result, sources } = renderLiveTail({ queryClient });
    await act(async () => {
      await vi.waitFor(() => expect(result.current.status).toBe("success"));
    });
    vi.mocked(fetchSessionTranscript).mockClear();

    act(() => sources[0]?.onerror?.(new Event("error")));
    await act(async () => vi.advanceTimersByTimeAsync(250));
    act(() => sources[1]?.onerror?.(new Event("error")));
    await act(async () => vi.advanceTimersByTimeAsync(500));

    expect(sources).toHaveLength(3);
    expect(fetchSessionTranscript).not.toHaveBeenCalled();
  });

  it("Should project terminal failure state and close without reconnecting", async () => {
    vi.useFakeTimers();
    vi.mocked(fetchSession).mockResolvedValue(sessionWithState("stopped"));
    const queryClient = createQueryClient();
    seedActiveSession(queryClient);
    const { result, sources } = renderLiveTail({ queryClient });
    await act(async () => {
      await vi.waitFor(() => expect(result.current.status).toBe("success"));
    });

    act(() => {
      sources[0]?.emit(
        "session_stopped",
        {
          agent_name: primarySessionFixture.agent_name,
          content: {},
          failure: { kind: "process_exit", summary: "provider exited before response" },
          id: "session-stopped-fixture",
          sequence: 9,
          session_id: SESSION_ID,
          spawn_depth: 0,
          stop_detail: "provider exited before response",
          stop_reason: "agent_crashed",
          timestamp: "2026-07-07T12:00:00Z",
          turn_id: "turn-terminal",
          type: "session_stopped",
        },
        "9"
      );
    });
    await act(async () => {
      await vi.waitFor(() =>
        expect(result.current.messages.at(-1)?.id).toBe(`session-stopped-${SESSION_ID}`)
      );
      await vi.advanceTimersByTimeAsync(4_000);
    });

    expect(sources[0]?.closed).toBe(true);
    expect(sources).toHaveLength(1);
    expect(JSON.stringify(result.current.messages.at(-1))).toContain(
      "provider exited before response"
    );
  });

  it("Should not open a stream for a stopped session", async () => {
    const queryClient = createQueryClient();
    queryClient.setQueryData(
      sessionKeys.detail(WORKSPACE_ID, SESSION_ID),
      sessionWithState("stopped")
    );
    vi.mocked(fetchSession).mockResolvedValue(sessionWithState("stopped"));

    const { result, sources } = renderLiveTail({ queryClient });
    await waitFor(() => expect(result.current.status).toBe("success"));
    expect(sources).toHaveLength(0);
  });

  it("Should close the active source on unmount", async () => {
    const { sources, unmount } = renderLiveTail();
    await waitFor(() => expect(sources).toHaveLength(1));

    unmount();

    expect(sources[0]?.closed).toBe(true);
  });
});
