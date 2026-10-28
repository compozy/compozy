import { notificationKeys } from "@/systems/notifications";
import {
  QueryClient,
  QueryClientProvider,
  QueryObserver,
  useInfiniteQuery,
  useQuery,
} from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { resetProfileViews, setProfileView } from "@/systems/profiles";

import { primarySessionFixture } from "../../mocks/fixtures";
import { sessionKeys } from "../../lib/query-keys";
import { sessionDetailOptions, sessionTranscriptOptions } from "../../lib/query-options";
import type { SessionTranscriptData } from "../../lib/session-transcript-query";
import type {
  NormalizedSessionTranscriptResponse,
  SessionMessage,
  SessionPayload,
  SessionState,
} from "../../types";
import {
  useSessionCatalogStreams,
  type SessionCatalogEventSource,
} from "../use-session-catalog-streams";
import { sessionCatalogStreamsLogic } from "../session-catalog-streams-store";

vi.mock("../../adapters/session-api", async importOriginal => ({
  ...(await importOriginal<typeof import("../../adapters/session-api")>()),
  fetchSession: vi.fn(),
  fetchSessionTranscript: vi.fn(),
}));

import { fetchSession, fetchSessionTranscript } from "../../adapters/session-api";

const WORKSPACE_ID = primarySessionFixture.workspace_id ?? "ws_primary";
const SESSION_ID = primarySessionFixture.id;
const POST_STOP_MARKER_KIND = "transcript_marker.post_stop";

function sessionWithState(state: SessionState): SessionPayload {
  return { ...primarySessionFixture, state };
}

function textMessage(id: string, text: string): SessionMessage {
  return { id, role: "assistant", parts: [{ type: "text", text }] };
}

function postStopMarkerMessage(id: string): SessionMessage {
  return {
    id,
    role: "assistant",
    parts: [
      {
        type: "data-compozy-event",
        data: {
          type: "transcript_marker.created",
          session_id: SESSION_ID,
          turn_id: "turn-late",
          text: "Late agent output discarded after the session stopped.",
          title: POST_STOP_MARKER_KIND,
          raw: {
            kind: POST_STOP_MARKER_KIND,
            occurred_at: "2026-09-05T12:00:00Z",
            summary: "Late agent output discarded after the session stopped.",
            evidence: { event_type: "agent_message" },
          },
        },
      },
    ] as unknown as SessionMessage["parts"],
  };
}

function transcriptResponse(messages: SessionMessage[]): NormalizedSessionTranscriptResponse {
  const entries = messages.map((message, index) => ({
    message,
    sequence: index + 1,
    start_sequence: index + 1,
  }));
  return {
    entries,
    epoch: 1,
    generation: 1,
    has_older: false,
    limit: 200,
    max_sequence: entries.length,
  };
}

function seededTranscript(messages: SessionMessage[]): SessionTranscriptData {
  const response = transcriptResponse(messages);
  return { pages: [{ ...response, cursor: response.max_sequence }], pageParams: [undefined] };
}

function transcriptHasPostStopMarker(data: SessionTranscriptData | undefined): boolean {
  return (data?.pages[0]?.entries ?? []).some(entry =>
    JSON.stringify(entry.message).includes(POST_STOP_MARKER_KIND)
  );
}

function createQueryClient(): QueryClient {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

/** A mounted session page: the detail read plus the transcript the live tail owns while live. */
function useMountedSessionPage(factory: (url: string) => SessionCatalogEventSource) {
  useSessionCatalogStreams({ eventSourceFactory: factory });
  const detail = useQuery(sessionDetailOptions(WORKSPACE_ID, SESSION_ID));
  const transcript = useInfiniteQuery(sessionTranscriptOptions(WORKSPACE_ID, SESSION_ID));
  return { detailState: detail.data?.state, transcript: transcript.data };
}

class FakeCatalogEventSource implements SessionCatalogEventSource {
  readonly listeners = new Map<string, Set<EventListenerOrEventListenerObject>>();
  closed = false;

  constructor(readonly url: string) {}

  addEventListener(type: string, listener: EventListenerOrEventListenerObject) {
    const listeners = this.listeners.get(type) ?? new Set<EventListenerOrEventListenerObject>();
    listeners.add(listener);
    this.listeners.set(type, listeners);
  }

  removeEventListener(type: string, listener: EventListenerOrEventListenerObject) {
    this.listeners.get(type)?.delete(listener);
  }

  emit(type: string, payload?: unknown) {
    const event =
      payload === undefined
        ? new Event(type)
        : new MessageEvent(type, { data: JSON.stringify(payload) });
    for (const listener of this.listeners.get(type) ?? []) {
      if (typeof listener === "function") listener(event);
      else listener.handleEvent(event);
    }
  }

  close() {
    this.closed = true;
  }
}

function wrapper(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useSessionCatalogStreams", () => {
  beforeEach(() => vi.clearAllMocks());
  afterEach(() => {
    vi.useRealTimers();
    resetProfileViews();
  });

  it("Should tolerate brief reconnects and report sustained loss until recovery", () => {
    vi.useFakeTimers();
    const client = createQueryClient();
    const sources: FakeCatalogEventSource[] = [];
    const factory = (url: string) => {
      const source = new FakeCatalogEventSource(url);
      sources.push(source);
      return source;
    };
    const { result, rerender, unmount } = renderHook(
      () => useSessionCatalogStreams({ eventSourceFactory: factory }),
      { wrapper: wrapper(client) }
    );
    expect(result.current).toBe("connecting");
    act(() => sources[0]!.emit("open"));
    act(() => sources[0]!.emit("error"));
    rerender();
    expect(sources).toHaveLength(1);
    expect(result.current).toBe("live");
    act(() => vi.advanceTimersByTime(9_999));
    act(() => sources[0]!.emit("open"));
    act(() => vi.advanceTimersByTime(10_000));
    expect(result.current).toBe("live");

    act(() => sources[0]!.emit("error"));
    act(() => vi.advanceTimersByTime(5_000));
    act(() => sources[0]!.emit("error"));
    act(() => vi.advanceTimersByTime(5_000));
    expect(result.current).toBe("stale");
    act(() => sources[0]!.emit("open"));
    expect(result.current).toBe("live");
    act(() => sources[0]!.emit("error"));
    unmount();
    act(() => vi.advanceTimersByTime(10_000));
    expect(sources[0]!.closed).toBe(true);
    client.clear();
  });

  it("Should bound initial connection grace and reset it for a new scope", () => {
    vi.useFakeTimers();
    const client = createQueryClient();
    const sources: FakeCatalogEventSource[] = [];
    const factory = (url: string) => {
      const source = new FakeCatalogEventSource(url);
      sources.push(source);
      return source;
    };
    const { result, unmount } = renderHook(
      () => useSessionCatalogStreams({ eventSourceFactory: factory }),
      { wrapper: wrapper(client) }
    );
    act(() => vi.advanceTimersByTime(10_000));
    expect(result.current).toBe("stale");
    act(() => setProfileView({ kind: "profile", profile: "other" }));
    expect(result.current).toBe("connecting");
    act(() => sources[0]!.emit("error"));
    act(() => sources[1]!.emit("open"));
    act(() => vi.advanceTimersByTime(10_000));
    expect(result.current).toBe("live");
    unmount();
    client.clear();
  });

  it("Should keep late source status events behind the current connection generation", () => {
    const store = sessionCatalogStreamsLogic.createStore();
    const [connecting] = store.transition(store.getInitialSnapshot(), {
      type: "connectionRequested",
      connect: () => () => {},
    });
    const [reconnecting] = store.transition(connecting, {
      type: "connectionRequested",
      connect: () => () => {},
    });
    const [afterLateFailure] = store.transition(reconnecting, {
      type: "connectionStale",
      generation: connecting.context.generation,
    });

    expect(afterLateFailure.context.status).toBe("connecting");
  });

  it("Should own one server-scoped aggregate source and reconcile global sessions", () => {
    vi.useFakeTimers();
    const queryClient = new QueryClient();
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    const betaDetail = sessionKeys.detail("ws_beta", "sess_beta");
    const globalDetail = sessionKeys.detail("", "sess_global");
    const untouched = [
      sessionKeys.detail("ws_other", "sess_beta"),
      sessionKeys.goal("ws_beta", "sess_beta"),
      sessionKeys.transcript("ws_beta", "sess_beta"),
      sessionKeys.transcript("", "sess_global"),
    ];
    for (const key of [betaDetail, globalDetail, ...untouched])
      queryClient.setQueryData(key, { state: "active" });
    const sources: FakeCatalogEventSource[] = [];
    const factory = (url: string) => {
      const source = new FakeCatalogEventSource(url);
      sources.push(source);
      return source;
    };
    const { unmount } = renderHook(
      () => useSessionCatalogStreams({ eventSourceFactory: factory }),
      { wrapper: wrapper(queryClient) }
    );

    // Both axes ride the URL: workspaces widened, profile scoped. The scope is
    // never omitted, so the daemon can never answer an unscoped subscription.
    expect(sources.map(source => source.url)).toEqual([
      "/api/sessions/catalog-stream?all_workspaces=true&profile=default",
    ]);

    act(() => {
      sources[0]?.emit("session_catalog_changed", {
        kind: "deleted",
        workspace_id: "ws_beta",
        session_id: "sess_beta",
      });
    });
    expect(invalidate).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: sessionKeys.workspaceLists("ws_beta") }),
      { cancelRefetch: false }
    );
    expect(invalidate).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: sessionKeys.workspaceLists("") }),
      { cancelRefetch: false }
    );
    expect(invalidate).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: sessionKeys.attentionSummary() }),
      { cancelRefetch: false }
    );
    expect(invalidate).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: notificationKeys.attentionRoot() }),
      { cancelRefetch: false }
    );
    expect(queryClient.getQueryState(betaDetail)?.isInvalidated).toBe(true);
    expect(queryClient.getQueryState(globalDetail)?.isInvalidated).toBe(false);
    for (const key of untouched) expect(queryClient.getQueryState(key)?.isInvalidated).toBe(false);
    invalidate.mockClear();
    act(() => {
      sources[0]?.emit("session_catalog_changed", {
        kind: "upserted",
        workspace_id: "",
        session_id: "sess_global",
      });
    });
    act(() => vi.advanceTimersByTime(5_000));
    expect(invalidate).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: sessionKeys.workspaceLists("") }),
      { cancelRefetch: false }
    );
    expect(queryClient.getQueryState(globalDetail)?.isInvalidated).toBe(true);
    for (const key of untouched) expect(queryClient.getQueryState(key)?.isInvalidated).toBe(false);
    // A subagent transition upserts its parent: a cold subagent list re-reads (trailing window).
    expect(invalidate).toHaveBeenCalledWith(
      { queryKey: sessionKeys.subagents("", "sess_global") },
      { cancelRefetch: false }
    );

    act(() => sources[0]?.emit("open"));
    expect(invalidate).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: sessionKeys.workspaceLists("") }),
      { cancelRefetch: false }
    );
    expect(invalidate).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: sessionKeys.attentionSummary() }),
      { cancelRefetch: false }
    );
    expect(invalidate).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: notificationKeys.attentionRoot() }),
      { cancelRefetch: false }
    );

    unmount();
    expect(sources[0]?.closed).toBe(true);
    expect([...sources[0]!.listeners.values()].every(listeners => listeners.size === 0)).toBe(true);
  });

  it("Should reconcile open detail and by-id reads from the daemon PR cache workspace wake", async () => {
    const queryClient = createQueryClient();
    const before = { ...primarySessionFixture, state: "stopped", branch: "topic" };
    const after = {
      ...before,
      pull_request: { ...before.pull_request, number: 42, state: "merged" },
    };
    const transcriptKey = sessionKeys.transcript(WORKSPACE_ID, SESSION_ID);
    queryClient.setQueryData(transcriptKey, seededTranscript([textMessage("m1", "Stopped")]));
    const read = vi.fn().mockResolvedValue(after);
    const keys = [
      sessionKeys.detail(WORKSPACE_ID, SESSION_ID),
      sessionKeys.byId(SESSION_ID, "default"),
    ];
    const unsubscribe = keys.map(queryKey =>
      new QueryObserver(queryClient, {
        queryKey,
        initialData: before,
        staleTime: Infinity,
        queryFn: read,
      }).subscribe(() => {})
    );
    const foreign = sessionKeys.byId("foreign", "default");
    queryClient.setQueryData(foreign, { ...before, id: "foreign", workspace_id: "other" });
    let source: FakeCatalogEventSource;
    const factory = (url: string) => (source = new FakeCatalogEventSource(url));
    const { unmount } = renderHook(
      () => useSessionCatalogStreams({ eventSourceFactory: factory }),
      { wrapper: wrapper(queryClient) }
    );
    act(() =>
      source.emit("session_catalog_changed", {
        kind: "layout",
        profile_id: "default",
        profile_name: "",
        workspace_id: WORKSPACE_ID,
        session_id: "",
      })
    );
    await waitFor(() => {
      for (const key of keys) expect(queryClient.getQueryData(key)).toEqual(after);
    });
    expect(read).toHaveBeenCalledTimes(2);
    expect(queryClient.getQueryState(transcriptKey)?.isInvalidated).toBe(false);
    expect(queryClient.getQueryState(foreign)?.isInvalidated).toBe(false);
    unmount();
    for (const stop of unsubscribe) stop();
  });

  it("Should reconcile the workspace lists on a layout change without touching a session", () => {
    vi.useFakeTimers();
    const queryClient = createQueryClient();
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    let source: FakeCatalogEventSource;
    const factory = (url: string) => (source = new FakeCatalogEventSource(url));
    renderHook(() => useSessionCatalogStreams({ eventSourceFactory: factory }), {
      wrapper: ({ children }: { children: ReactNode }) => (
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      ),
    });
    act(() => {
      source.emit("session_catalog_changed", {
        kind: "layout",
        profile_id: "default",
        profile_name: "default",
        workspace_id: "ws_alpha",
        session_id: "",
      });
    });
    act(() => vi.advanceTimersByTime(5_000));
    expect(invalidate).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: sessionKeys.workspaceLists("") }),
      { cancelRefetch: false }
    );
    expect(invalidate).not.toHaveBeenCalledWith({ queryKey: sessionKeys.byIdRoot("") });
  });

  it("Should bound catalog reads under continuous lifecycle activity and stop pending wakes on unmount", () => {
    vi.useFakeTimers();
    const queryClient = createQueryClient();
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    let source: FakeCatalogEventSource;
    const factory = (url: string) => (source = new FakeCatalogEventSource(url));
    const { unmount } = renderHook(
      () => useSessionCatalogStreams({ eventSourceFactory: factory }),
      {
        wrapper: ({ children }: { children: ReactNode }) => (
          <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
        ),
      }
    );
    for (let second = 0; second < 60; second++) {
      act(() => {
        for (let frame = 0; frame < 20; frame++)
          source.emit("session_catalog_changed", {
            kind: "upserted",
            workspace_id: "ws_busy",
            session_id: "sess_busy",
          });
        vi.advanceTimersByTime(1_000);
      });
    }
    const reads = invalidate.mock.calls.filter(
      ([filters]) =>
        JSON.stringify(filters?.queryKey) === JSON.stringify(sessionKeys.workspaceLists(""))
    );
    expect(reads.length).toBeLessThanOrEqual(13);
    expect(reads.length).toBeGreaterThan(1);
    // M9: 1,200 parent upserts (progress bursts) collapse to one subagent-list read per window.
    const subagentReads = invalidate.mock.calls.filter(
      ([filters]) =>
        JSON.stringify(filters?.queryKey) ===
        JSON.stringify(sessionKeys.subagents("ws_busy", "sess_busy"))
    );
    expect(subagentReads.length).toBeLessThanOrEqual(60);
    expect(subagentReads.length).toBeGreaterThan(1);
    unmount();
    const before = invalidate.mock.calls.length;
    act(() => vi.advanceTimersByTime(60_000));
    expect(invalidate.mock.calls).toHaveLength(before);
  });

  it("Should defer catalog reads while unfocused and reconcile once focus returns", () => {
    vi.useFakeTimers();
    const registrations = vi.spyOn(window, "addEventListener");
    const queryClient = createQueryClient();
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    let source: FakeCatalogEventSource;
    const factory = (url: string) => (source = new FakeCatalogEventSource(url));
    const { unmount } = renderHook(
      () => useSessionCatalogStreams({ eventSourceFactory: factory }),
      { wrapper: wrapper(queryClient) }
    );
    // Exercise the owned DOM subscription at its I/O boundary. Dispatching to
    // every library listener also invokes SWR's import-time native timer with
    // the FocusEvent as its delay (swr@2.5.1), unrelated to this query owner.
    const focus = registrations.mock.calls.find(([type]) => type === "focus")?.[1] as EventListener;
    const blur = registrations.mock.calls.find(([type]) => type === "blur")?.[1] as EventListener;
    act(() => blur(new Event("blur")));
    act(() => {
      for (let frame = 0; frame < 100; frame++)
        source.emit("session_catalog_changed", {
          kind: "upserted",
          workspace_id: "ws_busy",
          session_id: "sess_busy",
        });
      vi.advanceTimersByTime(60_000);
    });
    const catalogReads = () =>
      invalidate.mock.calls.filter(
        ([filters]) =>
          JSON.stringify(filters?.queryKey) === JSON.stringify(sessionKeys.workspaceLists(""))
      ).length;
    expect(catalogReads()).toBe(0);
    act(() => focus(new Event("focus")));
    registrations.mockRestore();
    expect(catalogReads()).toBe(1);
    unmount();
  });

  it.each([false, true])(
    "Should finish session reads during catalog bursts and retain the final wake (already fetching: %s)",
    async alreadyFetching => {
      const queryClient = createQueryClient();
      const reads: Array<{ signal: AbortSignal; resolve: (value: SessionPayload) => void }> = [];
      const keys = [
        sessionKeys.detail(WORKSPACE_ID, SESSION_ID),
        sessionKeys.byId(SESSION_ID, "default"),
      ];
      const unsubscribe = keys.map(queryKey =>
        new QueryObserver(queryClient, {
          queryKey,
          initialData: sessionWithState("active"),
          staleTime: Infinity,
          queryFn: ({ signal }) =>
            new Promise<SessionPayload>(resolve => reads.push({ signal, resolve })),
        }).subscribe(() => {})
      );
      let source: FakeCatalogEventSource;
      const factory = (url: string) => (source = new FakeCatalogEventSource(url));
      const { unmount } = renderHook(
        () => useSessionCatalogStreams({ eventSourceFactory: factory }),
        { wrapper: wrapper(queryClient) }
      );
      if (alreadyFetching) {
        for (const queryKey of keys) void queryClient.refetchQueries({ queryKey, exact: true });
      }
      act(() => {
        for (let frame = 0; frame < 200; frame++)
          source.emit("session_catalog_changed", {
            kind: "upserted",
            workspace_id: WORKSPACE_ID,
            session_id: SESSION_ID,
          });
      });
      expect({
        started: reads.length,
        aborted: reads.filter(read => read.signal.aborted).length,
      }).toEqual({ started: 2, aborted: 0 });
      await act(async () => {
        for (const read of reads.slice(0, 2)) read.resolve(sessionWithState("active"));
      });
      await waitFor(() => expect(reads).toHaveLength(4));
      await act(async () => {
        for (const read of reads.slice(2)) read.resolve(sessionWithState("stopped"));
      });
      await waitFor(() => {
        for (const key of keys)
          expect(queryClient.getQueryData<SessionPayload>(key)?.state).toBe("stopped");
      });
      expect(reads.every(read => !read.signal.aborted)).toBe(true);
      unmount();
      for (const stop of unsubscribe) stop();
      queryClient.clear();
    }
  );

  it("Should retain an attention wake until an inactive session prefetch settles", async () => {
    const queryClient = createQueryClient();
    const queryKey = sessionKeys.byId(SESSION_ID, "default");
    let resolveRead: (value: SessionPayload) => void = () => {};
    const read = vi.fn(
      () =>
        new Promise<SessionPayload>(resolve => {
          resolveRead = resolve;
        })
    );
    const options = { queryKey, queryFn: read, staleTime: Infinity };
    const prefetch = queryClient.fetchQuery(options);
    let source: FakeCatalogEventSource;
    const factory = (url: string) => (source = new FakeCatalogEventSource(url));
    const { unmount } = renderHook(
      () => useSessionCatalogStreams({ eventSourceFactory: factory }),
      { wrapper: wrapper(queryClient) }
    );
    act(() => {
      source.emit("session_attention_changed", {
        session_id: SESSION_ID,
        workspace_id: WORKSPACE_ID,
        from: "running",
        to: "done",
        class: "finished",
        at: "2026-10-17T12:00:00Z",
      });
    });
    expect(read).toHaveBeenCalledTimes(1);
    await act(async () => {
      resolveRead(sessionWithState("active"));
      await prefetch;
    });
    await waitFor(() => expect(queryClient.getQueryState(queryKey)?.isInvalidated).toBe(true));
    read.mockResolvedValue(sessionWithState("stopped"));
    const unsubscribe = new QueryObserver(queryClient, options).subscribe(() => {});
    await waitFor(() =>
      expect(queryClient.getQueryData<SessionPayload>(queryKey)?.state).toBe("stopped")
    );
    expect(read).toHaveBeenCalledTimes(2);
    unmount();
    unsubscribe();
    queryClient.clear();
  });

  it("Should preserve a final catalog wake after an in-flight snapshot finishes", async () => {
    vi.useFakeTimers();
    const queryClient = createQueryClient();
    let resolveRead: (value: string[]) => void = () => {};
    const read = vi.fn(
      () =>
        new Promise<string[]>(resolve => {
          resolveRead = resolve;
        })
    );
    let source: FakeCatalogEventSource;
    const factory = (url: string) => (source = new FakeCatalogEventSource(url));
    const { unmount } = renderHook(
      () => {
        useSessionCatalogStreams({ eventSourceFactory: factory });
        useQuery({
          queryKey: sessionKeys.workspaceLists(""),
          queryFn: read,
          initialData: [],
          staleTime: Infinity,
          retry: false,
        });
      },
      { wrapper: wrapper(queryClient) }
    );
    act(() => source.emit("open"));
    expect(read).toHaveBeenCalledTimes(1);
    act(() => {
      source.emit("session_catalog_changed", {
        kind: "upserted",
        workspace_id: "ws_busy",
        session_id: "sess_busy",
      });
      vi.advanceTimersByTime(5_000);
    });
    expect(read).toHaveBeenCalledTimes(1);
    await act(async () => {
      resolveRead(["old-snapshot"]);
      await Promise.resolve();
    });
    act(() => vi.advanceTimersByTime(5_000));
    expect(read).toHaveBeenCalledTimes(2);
    await act(async () => {
      resolveRead(["reconciled"]);
      await Promise.resolve();
    });
    unmount();
  });

  it("Should re-read an open monitor list when the catalog stream reconnects", async () => {
    const queryClient = createQueryClient();
    const read = vi.fn(async () => ({ monitors: [] }));
    let source: FakeCatalogEventSource;
    const factory = (url: string) => (source = new FakeCatalogEventSource(url));
    const { unmount } = renderHook(
      () => {
        useSessionCatalogStreams({ eventSourceFactory: factory });
        useQuery({
          queryKey: sessionKeys.monitors("ws_mon", "sess_mon"),
          queryFn: read,
          staleTime: Infinity,
          retry: false,
        });
      },
      { wrapper: wrapper(queryClient) }
    );
    await waitFor(() => expect(read).toHaveBeenCalledTimes(1));
    act(() => source.emit("open"));
    await waitFor(() => expect(read).toHaveBeenCalledTimes(2));
    unmount();
  });

  it("Should keep catalog errors visible without activity bypassing thirty-second backoff", async () => {
    vi.useFakeTimers();
    const queryClient = createQueryClient();
    const read = vi.fn(async () => {
      throw new Error("catalog unavailable");
    });
    let source: FakeCatalogEventSource;
    const factory = (url: string) => (source = new FakeCatalogEventSource(url));
    const { result, unmount } = renderHook(
      () => {
        useSessionCatalogStreams({ eventSourceFactory: factory });
        return useQuery({
          queryKey: sessionKeys.workspaceLists(""),
          queryFn: read,
          initialData: [],
          staleTime: Infinity,
          retry: false,
        });
      },
      { wrapper: wrapper(queryClient) }
    );
    await act(async () => {
      source.emit("open");
      await Promise.resolve();
    });
    expect(read).toHaveBeenCalledTimes(1);
    for (let second = 0; second < 29; second++) {
      await act(async () => {
        source.emit("session_catalog_changed", {
          kind: "upserted",
          workspace_id: "ws_busy",
          session_id: "sess_busy",
        });
        vi.advanceTimersByTime(1_000);
        await Promise.resolve();
      });
    }
    expect(read).toHaveBeenCalledTimes(1);
    expect(result.current.error?.message).toBe("catalog unavailable");
    await act(async () => {
      vi.advanceTimersByTime(1_000);
      await Promise.resolve();
    });
    expect(read).toHaveBeenCalledTimes(2);
    unmount();
  });

  it("Should retry only blocked catalogs until a new wake includes healthy siblings", async () => {
    vi.useFakeTimers();
    const queryClient = createQueryClient();
    const failing = vi.fn(async () => {
      throw new Error("catalog unavailable");
    });
    const healthy = vi.fn(async () => []);
    let source: FakeCatalogEventSource;
    const factory = (url: string) => (source = new FakeCatalogEventSource(url));
    const { unmount } = renderHook(
      () => {
        useSessionCatalogStreams({ eventSourceFactory: factory });
        useQuery({
          queryKey: [...sessionKeys.workspaceLists(""), "failing"],
          queryFn: failing,
          initialData: [],
          staleTime: Infinity,
          retry: false,
        });
        useQuery({
          queryKey: [...sessionKeys.workspaceLists(""), "healthy"],
          queryFn: healthy,
          initialData: [],
          staleTime: Infinity,
          retry: false,
        });
      },
      { wrapper: wrapper(queryClient) }
    );
    await act(async () => {
      source.emit("open");
      await Promise.resolve();
    });
    expect(failing).toHaveBeenCalledTimes(1);
    expect(healthy).toHaveBeenCalledTimes(1);
    await act(async () => {
      source.emit("session_catalog_changed", {
        kind: "upserted",
        workspace_id: "ws_busy",
        session_id: "sess_busy",
      });
      vi.advanceTimersByTime(5_000);
      await Promise.resolve();
    });
    expect(healthy).toHaveBeenCalledTimes(2);
    for (let window = 0; window < 5; window++) {
      await act(async () => {
        vi.advanceTimersByTime(5_000);
        await Promise.resolve();
      });
      expect(healthy).toHaveBeenCalledTimes(2);
    }
    expect(failing).toHaveBeenCalledTimes(2);
    await act(async () => {
      source.emit("open");
      vi.advanceTimersByTime(5_000);
      await Promise.resolve();
    });
    expect(healthy).toHaveBeenCalledTimes(3);
    expect(failing).toHaveBeenCalledTimes(2);
    unmount();
  });

  it("Should reopen under the new profile and stop the old socket on a switch", () => {
    const queryClient = new QueryClient();
    const sources: FakeCatalogEventSource[] = [];
    const factory = (url: string) => {
      const source = new FakeCatalogEventSource(url);
      sources.push(source);
      return source;
    };
    const { unmount } = renderHook(
      () => useSessionCatalogStreams({ eventSourceFactory: factory }),
      { wrapper: wrapper(queryClient) }
    );
    expect(sources).toHaveLength(1);

    act(() => setProfileView({ kind: "profile", profile: "marketing" }));

    // A second socket opens under the new scope, and the first one is closed —
    // leaving a profile's scope stops its stream writes (US-010.EC-2).
    expect(sources).toHaveLength(2);
    expect(sources[1]?.url).toBe(
      "/api/sessions/catalog-stream?all_workspaces=true&profile=marketing"
    );
    expect(sources[0]?.closed).toBe(true);
    expect([...sources[0]!.listeners.values()].every(listeners => listeners.size === 0)).toBe(true);

    act(() => setProfileView({ kind: "aggregate" }));
    expect(sources[2]?.url).toBe(
      "/api/sessions/catalog-stream?all_workspaces=true&all_profiles=true"
    );
    expect(sources[1]?.closed).toBe(true);

    unmount();
  });

  it("Should route each named attention event to its handler exactly once (UT-078, UT-083)", () => {
    const queryClient = new QueryClient();
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    const sources: FakeCatalogEventSource[] = [];
    const factory = (url: string) => {
      const source = new FakeCatalogEventSource(url);
      sources.push(source);
      return source;
    };
    const onAttentionEdge = vi.fn();
    const onOperatorNotification = vi.fn();
    const alpha = { id: "ws_alpha" };
    const { unmount } = renderHook(
      () =>
        useSessionCatalogStreams({
          eventSourceFactory: factory,
          onAttentionEdge,
          onOperatorNotification,
        }),
      { wrapper: wrapper(queryClient) }
    );

    const edge = {
      session_id: "sess_alpha",
      workspace_id: alpha.id,
      from: "running",
      to: "waiting-for-input",
      class: "needs-you",
      at: "2026-07-13T12:04:00Z",
    };
    const notification = {
      notification_id: "ntf_1",
      session_id: "sess_alpha",
      workspace_id: alpha.id,
      title: "Deps audit done",
      body: "3 findings",
      at: "2026-07-13T12:05:00Z",
    };

    act(() => {
      sources[0]?.emit("session_attention_changed", edge);
      sources[0]?.emit("operator_notification", notification);
    });

    expect(onAttentionEdge).toHaveBeenCalledExactlyOnceWith(edge);
    expect(onOperatorNotification).toHaveBeenCalledExactlyOnceWith(notification);
    expect(invalidate).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: sessionKeys.workspaceLists(alpha.id) }),
      { cancelRefetch: false }
    );
    expect(invalidate).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: sessionKeys.workspaceLists("") }),
      { cancelRefetch: false }
    );
    expect(invalidate).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: sessionKeys.attentionSummary() }),
      { cancelRefetch: false }
    );
    expect(invalidate).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: notificationKeys.attentionRoot() }),
      { cancelRefetch: false }
    );

    // The server owns authorization; the client routes every valid frame it receives.
    act(() => {
      sources[0]?.emit("session_attention_changed", { ...edge, workspace_id: "ws_foreign" });
      sources[0]?.emit("operator_notification", { ...notification, workspace_id: "ws_foreign" });
    });
    expect(onAttentionEdge).toHaveBeenCalledTimes(2);
    expect(onOperatorNotification).toHaveBeenCalledTimes(2);

    // A malformed frame is dropped rather than delivered half-populated.
    act(() => {
      sources[0]?.emit("session_attention_changed", { session_id: "sess_alpha" });
      sources[0]?.emit("operator_notification", { notification_id: "ntf_2" });
    });
    expect(onAttentionEdge).toHaveBeenCalledTimes(2);
    expect(onOperatorNotification).toHaveBeenCalledTimes(2);

    unmount();
    expect(sources[0]?.listeners.get("session_attention_changed")?.size ?? 0).toBe(0);
    expect(sources[0]?.listeners.get("operator_notification")?.size ?? 0).toBe(0);
  });

  it("Should keep handler identity out of the connection lifetime (UT-078)", () => {
    // A re-rendering notifier must never tear down and reopen the stream: that
    // would drop edges and restart the generation fence on every render.
    const queryClient = new QueryClient();
    const sources: FakeCatalogEventSource[] = [];
    const factory = (url: string) => {
      const source = new FakeCatalogEventSource(url);
      sources.push(source);
      return source;
    };
    const alpha = { id: "ws_alpha" };
    const first = vi.fn();
    const second = vi.fn();
    const { rerender } = renderHook(
      ({ handler }: { handler: () => void }) =>
        useSessionCatalogStreams({
          eventSourceFactory: factory,
          onAttentionEdge: handler,
        }),
      { initialProps: { handler: first }, wrapper: wrapper(queryClient) }
    );

    rerender({ handler: second });

    expect(sources).toHaveLength(1);
    act(() => {
      sources[0]?.emit("session_attention_changed", {
        session_id: "sess_alpha",
        workspace_id: alpha.id,
        from: "running",
        to: "failed",
        class: "needs-you",
        at: "2026-07-13T12:06:00Z",
      });
    });
    expect(first).not.toHaveBeenCalled();
    expect(second).toHaveBeenCalledTimes(1);
  });

  it.each([{ worktree_id: "wt_new" }, { workspace_path: "/workspace/new-root" }])(
    "Should refresh mounted files only when the catalog changes the checkout (%j)",
    async location => {
      // Invariant: externally changed CWD reaches the open mention picker without
      // navigation, while other session/workspace file caches remain untouched.
      // Owner: session catalog reconciliation; canonical stream suite.
      const queryClient = createQueryClient();
      const filesKey = sessionKeys.fileSearch(WORKSPACE_ID, SESSION_ID, "", 50);
      const otherKey = sessionKeys.fileSearch(WORKSPACE_ID, "sess_other", "", 50);
      const foreignKey = sessionKeys.fileSearch("ws_other", SESSION_ID, "", 50);
      queryClient.setQueryData(filesKey, ["root.txt"]);
      queryClient.setQueryData(otherKey, ["other.txt"]);
      queryClient.setQueryData(foreignKey, ["foreign.txt"]);
      queryClient.setQueryData(
        sessionKeys.detail(WORKSPACE_ID, SESSION_ID),
        sessionWithState("active")
      );
      vi.mocked(fetchSession).mockResolvedValue(sessionWithState("active"));
      const readFiles = vi.fn().mockResolvedValue(["checkout.txt"]);
      const sources: FakeCatalogEventSource[] = [];
      const factory = (url: string) => {
        const source = new FakeCatalogEventSource(url);
        sources.push(source);
        return source;
      };
      const { result } = renderHook(
        () => {
          useSessionCatalogStreams({ eventSourceFactory: factory });
          useQuery(sessionDetailOptions(WORKSPACE_ID, SESSION_ID));
          return useQuery({ queryKey: filesKey, queryFn: readFiles, staleTime: Infinity });
        },
        { wrapper: wrapper(queryClient) }
      );
      expect(result.current.data).toEqual(["root.txt"]);
      act(() => {
        sources[0]?.emit("session_catalog_changed", {
          kind: "upserted",
          workspace_id: WORKSPACE_ID,
          session_id: SESSION_ID,
        });
      });
      await waitFor(() => expect(fetchSession).toHaveBeenCalledTimes(1));
      await waitFor(() =>
        expect(
          queryClient.isFetching({ queryKey: sessionKeys.detail(WORKSPACE_ID, SESSION_ID) })
        ).toBe(0)
      );
      expect(readFiles).not.toHaveBeenCalled();
      vi.mocked(fetchSession).mockResolvedValue({ ...sessionWithState("active"), ...location });
      act(() => {
        sources[0]?.emit("session_catalog_changed", {
          kind: "upserted",
          workspace_id: WORKSPACE_ID,
          session_id: SESSION_ID,
        });
      });
      await waitFor(() => expect(result.current.data).toEqual(["checkout.txt"]));
      expect(readFiles).toHaveBeenCalledTimes(1);
      expect(queryClient.getQueryState(otherKey)?.isInvalidated).toBe(false);
      expect(queryClient.getQueryState(foreignKey)?.isInvalidated).toBe(false);
      expect(fetchSessionTranscript).not.toHaveBeenCalled();
    }
  );

  it.each([false, true])(
    "Should wake a stopped session's mounted transcript after detail settles (upsert then layout during prior read: %s)",
    async interleaved => {
      // Invariant: the post-stop discard marker reaches an already-mounted transcript
      // without navigation, remount, reconnect, or polling — the catalog upsert re-reads
      // the detail and re-reads the transcript only once that read says the session is
      // no longer live. Cached live state that is already stale must never decide it.
      const queryClient = createQueryClient();
      const detailKey = sessionKeys.detail(WORKSPACE_ID, SESSION_ID);
      const transcriptKey = sessionKeys.transcript(WORKSPACE_ID, SESSION_ID);
      const foreignTranscriptKey = sessionKeys.transcript(WORKSPACE_ID, "sess_other");
      const otherWorkspaceTranscriptKey = sessionKeys.transcript("ws_other", SESSION_ID);
      queryClient.setQueryData(detailKey, sessionWithState("active"));
      queryClient.setQueryData(transcriptKey, seededTranscript([textMessage("m1", "Working…")]));
      queryClient.setQueryData(
        foreignTranscriptKey,
        seededTranscript([textMessage("f1", "Other")])
      );
      queryClient.setQueryData(
        otherWorkspaceTranscriptKey,
        seededTranscript([textMessage("w1", "Elsewhere")])
      );
      const foreignBefore = queryClient.getQueryData(foreignTranscriptKey);
      const otherWorkspaceBefore = queryClient.getQueryData(otherWorkspaceTranscriptKey);
      let resolveDetail: ((session: SessionPayload) => void) | undefined;
      vi.mocked(fetchSession).mockImplementation(
        () =>
          new Promise<SessionPayload>(resolve => {
            resolveDetail = resolve;
          })
      );
      vi.mocked(fetchSessionTranscript).mockResolvedValue(
        transcriptResponse([textMessage("m1", "Working…"), postStopMarkerMessage("late-1")])
      );
      const sources: FakeCatalogEventSource[] = [];
      const factory = (url: string) => {
        const source = new FakeCatalogEventSource(url);
        sources.push(source);
        return source;
      };
      const { result } = renderHook(() => useMountedSessionPage(factory), {
        wrapper: wrapper(queryClient),
      });
      expect(result.current.detailState).toBe("active");
      if (interleaved) {
        void queryClient.refetchQueries({ queryKey: detailKey, exact: true });
        await waitFor(() => expect(fetchSession).toHaveBeenCalledTimes(1));
      }

      act(() => {
        sources[0]?.emit("session_catalog_changed", {
          kind: "upserted",
          workspace_id: WORKSPACE_ID,
          session_id: SESSION_ID,
        });
        if (interleaved)
          sources[0]?.emit("session_catalog_changed", {
            kind: "layout",
            profile_id: "default",
            workspace_id: WORKSPACE_ID,
            session_id: "",
          });
      });

      await waitFor(() => expect(fetchSession).toHaveBeenCalledTimes(1));
      // The wake landed while the cache still said active: no transcript read yet, and
      // no fabricated stopped state either.
      expect(fetchSessionTranscript).not.toHaveBeenCalled();
      expect(queryClient.getQueryData<SessionPayload>(detailKey)?.state).toBe("active");

      if (interleaved) vi.mocked(fetchSession).mockResolvedValue(sessionWithState("stopped"));
      await act(async () => {
        resolveDetail?.(sessionWithState("stopped"));
      });

      await waitFor(() =>
        expect(transcriptHasPostStopMarker(result.current.transcript)).toBe(true)
      );
      expect(result.current.detailState).toBe("stopped");
      expect(fetchSession).toHaveBeenCalledTimes(interleaved ? 2 : 1);
      expect(fetchSessionTranscript).toHaveBeenCalledTimes(1);
      expect(fetchSessionTranscript).toHaveBeenCalledWith(
        WORKSPACE_ID,
        SESSION_ID,
        {},
        expect.any(AbortSignal)
      );
      expect(sources).toHaveLength(1);
      expect(queryClient.getQueryData(foreignTranscriptKey)).toBe(foreignBefore);
      expect(queryClient.getQueryData(otherWorkspaceTranscriptKey)).toBe(otherWorkspaceBefore);
    }
  );

  it("Should finish shared reads but discard old profile followups after the stream closes", async () => {
    const queryClient = createQueryClient();
    const detailKey = sessionKeys.detail(WORKSPACE_ID, SESSION_ID);
    queryClient.setQueryData(detailKey, sessionWithState("active"));
    queryClient.setQueryData(
      sessionKeys.transcript(WORKSPACE_ID, SESSION_ID),
      seededTranscript([textMessage("m1", "Working…")])
    );
    let resolveRead: (value: SessionPayload) => void = () => {};
    vi.mocked(fetchSession).mockImplementation(
      () =>
        new Promise<SessionPayload>(resolve => {
          resolveRead = resolve;
        })
    );
    vi.mocked(fetchSessionTranscript).mockResolvedValue(
      transcriptResponse([textMessage("m1", "Working…"), postStopMarkerMessage("late-1")])
    );
    const sources: FakeCatalogEventSource[] = [];
    const factory = (url: string) => {
      const source = new FakeCatalogEventSource(url);
      sources.push(source);
      return source;
    };
    const { unmount } = renderHook(() => useMountedSessionPage(factory), {
      wrapper: wrapper(queryClient),
    });
    const wake = { kind: "upserted", workspace_id: WORKSPACE_ID, session_id: SESSION_ID };
    act(() => {
      sources[0]!.emit("session_catalog_changed", wake);
      sources[0]!.emit("session_catalog_changed", wake);
      setProfileView({ kind: "profile", profile: "marketing" });
    });
    expect(sources).toHaveLength(2);
    expect(sources[0]!.closed).toBe(true);
    expect(fetchSession).toHaveBeenCalledTimes(1);
    const signal = vi.mocked(fetchSession).mock.calls[0]![2];
    expect(signal?.aborted).toBe(false);
    await act(async () => resolveRead(sessionWithState("stopped")));
    expect(queryClient.getQueryData<SessionPayload>(detailKey)?.state).toBe("stopped");
    expect(fetchSession).toHaveBeenCalledTimes(1);
    expect(fetchSessionTranscript).not.toHaveBeenCalled();

    vi.mocked(fetchSession).mockResolvedValue(sessionWithState("stopped"));
    act(() => sources[1]!.emit("session_catalog_changed", wake));
    await waitFor(() => expect(fetchSessionTranscript).toHaveBeenCalledTimes(1));
    expect(fetchSession).toHaveBeenCalledTimes(2);
    unmount();
    queryClient.clear();
  });

  it("Should leave a live session's transcript to the live tail on a catalog wake", async () => {
    const queryClient = createQueryClient();
    const detailKey = sessionKeys.detail(WORKSPACE_ID, SESSION_ID);
    const transcriptKey = sessionKeys.transcript(WORKSPACE_ID, SESSION_ID);
    queryClient.setQueryData(detailKey, sessionWithState("active"));
    queryClient.setQueryData(transcriptKey, seededTranscript([textMessage("m1", "Working…")]));
    const transcriptBefore = queryClient.getQueryData(transcriptKey);
    vi.mocked(fetchSession).mockResolvedValue(sessionWithState("active"));
    vi.mocked(fetchSessionTranscript).mockResolvedValue(
      transcriptResponse([postStopMarkerMessage("never")])
    );
    const sources: FakeCatalogEventSource[] = [];
    const factory = (url: string) => {
      const source = new FakeCatalogEventSource(url);
      sources.push(source);
      return source;
    };
    renderHook(() => useMountedSessionPage(factory), { wrapper: wrapper(queryClient) });

    act(() => {
      sources[0]?.emit("session_catalog_changed", {
        kind: "upserted",
        workspace_id: WORKSPACE_ID,
        session_id: SESSION_ID,
      });
    });
    await waitFor(() => expect(fetchSession).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(queryClient.isFetching()).toBe(0));

    expect(fetchSessionTranscript).not.toHaveBeenCalled();
    expect(queryClient.getQueryData(transcriptKey)).toBe(transcriptBefore);

    // A deletion re-reads the detail but never wakes the transcript, even once the
    // authoritative read is no longer live.
    vi.mocked(fetchSession).mockResolvedValue(sessionWithState("stopped"));
    act(() => {
      sources[0]?.emit("session_catalog_changed", {
        kind: "deleted",
        workspace_id: WORKSPACE_ID,
        session_id: SESSION_ID,
      });
    });
    await waitFor(() => expect(fetchSession).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(queryClient.isFetching()).toBe(0));

    expect(fetchSessionTranscript).not.toHaveBeenCalled();
  });

  it("Should keep the shell alive when global source construction fails", () => {
    const queryClient = new QueryClient();
    const factory = vi.fn(() => {
      throw new Error("EventSource unavailable");
    });

    renderHook(
      () =>
        useSessionCatalogStreams({
          eventSourceFactory: factory,
        }),
      { wrapper: wrapper(queryClient) }
    );

    expect(factory).toHaveBeenCalledTimes(1);
  });
});
