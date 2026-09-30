import { notificationKeys } from "@/systems/notifications";
import {
  QueryClient,
  QueryClientProvider,
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
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: sessionKeys.detail("ws_beta", "sess_beta"),
      exact: true,
    });
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
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: sessionKeys.detail("", "sess_global"),
      exact: true,
    });

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

    act(() => setProfileView({ scope: "global" }, { kind: "profile", profile: "marketing" }));

    // A second socket opens under the new scope, and the first one is closed —
    // leaving a profile's scope stops its stream writes (US-010.EC-2).
    expect(sources).toHaveLength(2);
    expect(sources[1]?.url).toBe(
      "/api/sessions/catalog-stream?all_workspaces=true&profile=marketing"
    );
    expect(sources[0]?.closed).toBe(true);
    expect([...sources[0]!.listeners.values()].every(listeners => listeners.size === 0)).toBe(true);

    act(() => setProfileView({ scope: "global" }, { kind: "aggregate" }));
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

  it("Should wake a stopped session's mounted transcript only after the authoritative detail read settles", async () => {
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
    queryClient.setQueryData(foreignTranscriptKey, seededTranscript([textMessage("f1", "Other")]));
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

    act(() => {
      sources[0]?.emit("session_catalog_changed", {
        kind: "upserted",
        workspace_id: WORKSPACE_ID,
        session_id: SESSION_ID,
      });
    });

    await waitFor(() => expect(fetchSession).toHaveBeenCalledTimes(1));
    // The wake landed while the cache still said active: no transcript read yet, and
    // no fabricated stopped state either.
    expect(fetchSessionTranscript).not.toHaveBeenCalled();
    expect(queryClient.getQueryData<SessionPayload>(detailKey)?.state).toBe("active");

    await act(async () => {
      resolveDetail?.(sessionWithState("stopped"));
    });

    await waitFor(() => expect(transcriptHasPostStopMarker(result.current.transcript)).toBe(true));
    expect(result.current.detailState).toBe("stopped");
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
