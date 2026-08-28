import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import type { StreamEventSource } from "@/lib/ticketed-event-source";

import { terminalKeys } from "../../lib/query-keys";
import type { TerminalRecordingMap } from "../../lib/terminal-recording-state";
import { DEV_SERVER_TERMINAL, PSQL_TERMINAL } from "../../mocks/terminal-fixtures";
import type { TerminalInfo } from "../../types";
import { useTerminalCatalogStream } from "../use-terminal-catalog-stream";

/**
 * Canonical suite for the live terminal catalog (part of UT-113).
 *
 * Invariant: exactly one catalog subscription exists per
 * `(queryClient, workspace, profile)`; extra subscribers share that source,
 * switching scope closes the previous source before opening the next, and a
 * frame that arrives from a closed source never reaches the cache that replaced
 * it. Recording started/stopped write the scoped recordings map; a matching
 * snapshot or reconnect open clears only that stream's profile, and the
 * immediately following in-scope recording_started events rehydrate it.
 * The store's own rebinding reducer is covered where the store lives — this
 * file owns the subscription's lifetime.
 */

const WORKSPACE = "ws-atlas";

/** A source a test can inspect and speak through. */
function createFakeSource() {
  const listeners = new Map<string, Set<EventListener>>();
  let closed = false;
  return {
    get closed() {
      return closed;
    },
    listenerCount: () => [...listeners.values()].reduce((total, set) => total + set.size, 0),
    emit(type: string, payload: unknown) {
      const event = new MessageEvent(type, { data: JSON.stringify(payload) });
      for (const listener of listeners.get(type) ?? []) {
        listener(event);
      }
    },
    emitRaw(type: string, data: string) {
      const event = new MessageEvent(type, { data });
      for (const listener of listeners.get(type) ?? []) listener(event);
    },
    queue(type: string, payload: unknown) {
      const queued = [...(listeners.get(type) ?? [])];
      const event = new MessageEvent(type, { data: JSON.stringify(payload) });
      return () => queued.forEach(listener => listener(event));
    },
    source: {
      addEventListener: (type: string, listener: EventListener) => {
        const set = listeners.get(type) ?? new Set<EventListener>();
        set.add(listener);
        listeners.set(type, set);
      },
      removeEventListener: (type: string, listener: EventListener) => {
        listeners.get(type)?.delete(listener);
      },
      close: () => {
        closed = true;
      },
      onmessage: null,
      onerror: null,
      onopen: null,
    },
  };
}

function renderStream(initialProfile: string, initialWorkspaceId = WORKSPACE) {
  const opened: { url: string; fake: ReturnType<typeof createFakeSource> }[] = [];
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  const view = renderHook(
    ({ profileKey, workspaceId }: { profileKey: string; workspaceId: string }) =>
      useTerminalCatalogStream({
        workspaceId,
        profileKey,
        eventSourceFactory: (url: string) => {
          const fake = createFakeSource();
          opened.push({ url, fake });
          return fake.source as unknown as StreamEventSource;
        },
      }),
    { initialProps: { profileKey: initialProfile, workspaceId: initialWorkspaceId }, wrapper }
  );
  return { ...view, client, opened };
}

function renderAggregateStream() {
  const opened: { url: string; fake: ReturnType<typeof createFakeSource> }[] = [];
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  renderHook(
    () =>
      useTerminalCatalogStream({
        workspaceId: WORKSPACE,
        profileKey: "@all",
        allProfiles: true,
        profiles: ["work", "personal"],
        eventSourceFactory: url => {
          const fake = createFakeSource();
          opened.push({ url, fake });
          return fake.source as unknown as StreamEventSource;
        },
      }),
    {
      wrapper: ({ children }) => (
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
      ),
    }
  );
  return { opened, client };
}

function catalog(client: QueryClient, profileKey: string): TerminalInfo[] | undefined {
  return client.getQueryData(terminalKeys.catalog({ workspaceId: WORKSPACE, profileKey }));
}

function recordings(client: QueryClient, profileKey: string): TerminalRecordingMap | undefined {
  return client.getQueryData(terminalKeys.recordings({ workspaceId: WORKSPACE, profileKey }));
}

const RECORDING_AT = "2026-08-25T12:00:00.000Z";

function recordingStarted(overrides: Record<string, unknown> = {}) {
  return {
    event: "terminal.recording_started",
    timestamp: RECORDING_AT,
    workspace_id: WORKSPACE,
    profile_id: "01JB4Z2K9QW8XR3TFN6VYD5HAC",
    terminal_id: DEV_SERVER_TERMINAL.id,
    actor_kind: "human",
    actor_id: "pedro",
    at: RECORDING_AT,
    recording_id: "rec-1",
    ...overrides,
  };
}

describe("useTerminalCatalogStream", () => {
  it("Should open exactly one source for the scope it was given", () => {
    const { opened } = renderStream("work");

    expect(opened).toHaveLength(1);
    expect(opened[0].url).toContain(`/api/workspaces/${WORKSPACE}/terminals/stream`);
    expect(opened[0].url).toContain("profile=work");
    expect(opened[0].fake.listenerCount()).toBeGreaterThan(0);
  });

  it("Should subscribe every concrete owner without sending the aggregate cache label", () => {
    const { opened } = renderAggregateStream();

    expect(opened).toHaveLength(2);
    expect(opened.map(entry => entry.url)).toEqual([
      expect.stringContaining("profile=personal"),
      expect.stringContaining("profile=work"),
    ]);
    for (const entry of opened) expect(entry.url).not.toContain("profile=%40all");
  });

  it("Should replace only the owner covered by an aggregate snapshot", () => {
    const { opened, client } = renderAggregateStream();
    const personal = { ...PSQL_TERMINAL, profile_name: "personal", profile_id: "profile-personal" };
    const personalSource = opened.find(entry => entry.url.includes("profile=personal"));
    const workSource = opened.find(entry => entry.url.includes("profile=work"));
    if (!personalSource || !workSource) throw new Error("expected both profile streams");

    workSource.fake.emit("terminal.snapshot", { terminals: [DEV_SERVER_TERMINAL] });
    personalSource.fake.emit("terminal.snapshot", { terminals: [personal] });
    expect(catalog(client, "@all")?.map(terminal => terminal.id)).toEqual([
      DEV_SERVER_TERMINAL.id,
      personal.id,
    ]);

    workSource.fake.emit("terminal.snapshot", { terminals: [] });
    expect(catalog(client, "@all")?.map(terminal => terminal.id)).toEqual([personal.id]);
  });

  it("Should fold a snapshot and its patches into the scope's own cache entry", () => {
    const { opened, client } = renderStream("work");
    const terminal = {
      ...DEV_SERVER_TERMINAL,
      bound_run: { session_id: "session-a", run_id: "run-a", generation: 7 },
    };

    opened[0].fake.emit("terminal.snapshot", { terminals: [terminal] });
    expect(catalog(client, "work")?.map(terminal => terminal.id)).toEqual([DEV_SERVER_TERMINAL.id]);
    expect(catalog(client, "work")?.[0].bound_run?.generation).toBe(7);

    opened[0].fake.emit("terminal.title_changed", {
      terminal_id: DEV_SERVER_TERMINAL.id,
      title: "dev server (staging)",
    });
    expect(catalog(client, "work")?.[0].title).toBe("dev server (staging)");
  });

  it("Should close the previous source and open the next one on a profile switch", () => {
    const { opened, rerender } = renderStream("work");

    rerender({ profileKey: "personal", workspaceId: WORKSPACE });

    expect(opened).toHaveLength(2);
    expect(opened[0].fake.closed).toBe(true);
    expect(opened[0].fake.listenerCount()).toBe(0);
    expect(opened[1].url).toContain("profile=personal");
    expect(opened[1].fake.closed).toBe(false);
  });

  it("Should close the previous source and open the next one on a workspace switch", () => {
    const { opened, rerender } = renderStream("work");

    rerender({ profileKey: "work", workspaceId: "ws-other" });

    expect(opened).toHaveLength(2);
    expect(opened[0].fake.closed).toBe(true);
    expect(opened[1].url).toContain("/api/workspaces/ws-other/terminals/stream");
  });

  it("Should ignore a late frame from a source that has already been replaced", () => {
    const { opened, client, rerender } = renderStream("work");
    opened[0].fake.emit("terminal.snapshot", { terminals: [DEV_SERVER_TERMINAL] });

    const deliverQueued = opened[0].fake.queue("terminal.created", { terminal: PSQL_TERMINAL });
    rerender({ profileKey: "personal", workspaceId: WORKSPACE });
    // The socket is closed, but its last event was already queued by the browser.
    deliverQueued();

    // Neither cache entry may take it: the new scope never saw it, and the old
    // one is no longer being read.
    expect(catalog(client, "personal")).toBeUndefined();
    expect(catalog(client, "work")?.map(terminal => terminal.id)).toEqual([DEV_SERVER_TERMINAL.id]);
  });

  it("Should invalidate the exact catalog after a malformed known event without merging it", () => {
    const { opened, client } = renderStream("work");
    opened[0].fake.emit("terminal.snapshot", { terminals: [DEV_SERVER_TERMINAL] });
    const invalidate = vi.spyOn(client, "invalidateQueries");

    opened[0].fake.emit("terminal.created", { terminal: { id: "term-broken" } });

    expect(invalidate).toHaveBeenCalledWith({
      queryKey: terminalKeys.catalog({ workspaceId: WORKSPACE, profileKey: "work" }),
      exact: true,
    });
    expect(catalog(client, "work")?.map(terminal => terminal.id)).toEqual([DEV_SERVER_TERMINAL.id]);
  });

  it("Should invalidate the exact catalog after malformed JSON without merging it", () => {
    const { opened, client } = renderStream("work");
    opened[0].fake.emit("terminal.snapshot", { terminals: [DEV_SERVER_TERMINAL] });
    const invalidate = vi.spyOn(client, "invalidateQueries");

    opened[0].fake.emitRaw("terminal.created", "{not-json");

    expect(invalidate).toHaveBeenCalledWith({
      queryKey: terminalKeys.catalog({ workspaceId: WORKSPACE, profileKey: "work" }),
      exact: true,
    });
    expect(catalog(client, "work")?.map(terminal => terminal.id)).toEqual([DEV_SERVER_TERMINAL.id]);
  });

  it.each([
    [
      "an unknown enum",
      "terminal.mode_changed",
      { terminal_id: DEV_SERVER_TERMINAL.id, mode: "future" },
    ],
    [
      "an unknown extra field",
      "terminal.title_changed",
      { terminal_id: DEV_SERVER_TERMINAL.id, title: "new title", extra: true },
    ],
    [
      "an available lease with a controller",
      "terminal.lease_changed",
      {
        terminal_id: DEV_SERVER_TERMINAL.id,
        lease: "available",
        controller_kind: "human",
        controller_id: "viewer-1",
        reason: "released",
      },
    ],
    [
      "human ownership without an actor",
      "terminal.lease_changed",
      {
        terminal_id: DEV_SERVER_TERMINAL.id,
        lease: "human_owned",
        controller_kind: "",
        controller_id: "",
        reason: "claimed",
      },
    ],
    [
      "agent ownership with a human controller",
      "terminal.lease_changed",
      {
        terminal_id: DEV_SERVER_TERMINAL.id,
        lease: "agent_owned",
        controller_kind: "human",
        controller_id: "viewer-1",
        reason: "claimed",
      },
    ],
  ])("Should invalidate without merging %s", (_case, eventName, payload) => {
    const { opened, client } = renderStream("work");
    opened[0].fake.emit("terminal.snapshot", { terminals: [DEV_SERVER_TERMINAL] });
    const invalidate = vi.spyOn(client, "invalidateQueries");

    opened[0].fake.emit(eventName, payload);

    expect(invalidate).toHaveBeenCalledWith({
      queryKey: terminalKeys.catalog({ workspaceId: WORKSPACE, profileKey: "work" }),
      exact: true,
    });
    expect(catalog(client, "work")).toEqual([DEV_SERVER_TERMINAL]);
  });

  it("Should close its source when the surface goes away", () => {
    const { opened, unmount } = renderStream("work");

    unmount();

    expect(opened[0].fake.closed).toBe(true);
    expect(opened[0].fake.listenerCount()).toBe(0);
  });

  it("Should share one source when two subscribers use the same query client and scope", () => {
    const opened: { url: string; fake: ReturnType<typeof createFakeSource> }[] = [];
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const factory = (url: string) => {
      const fake = createFakeSource();
      opened.push({ url, fake });
      return fake.source as unknown as StreamEventSource;
    };
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    );
    const first = renderHook(
      () =>
        useTerminalCatalogStream({
          workspaceId: WORKSPACE,
          profileKey: "work",
          eventSourceFactory: factory,
        }),
      { wrapper }
    );
    const second = renderHook(
      () =>
        useTerminalCatalogStream({
          workspaceId: WORKSPACE,
          profileKey: "work",
          eventSourceFactory: factory,
        }),
      { wrapper }
    );

    expect(opened).toHaveLength(1);
    first.unmount();
    expect(opened[0].fake.closed).toBe(false);
    second.unmount();
    expect(opened[0].fake.closed).toBe(true);
    expect(opened[0].fake.listenerCount()).toBe(0);
  });

  it("Should stay closed while disabled", () => {
    const client = new QueryClient();
    const factory = vi.fn();
    renderHook(
      () =>
        useTerminalCatalogStream({
          workspaceId: WORKSPACE,
          profileKey: "work",
          enabled: false,
          eventSourceFactory: factory,
        }),
      {
        wrapper: ({ children }) => (
          <QueryClientProvider client={client}>{children}</QueryClientProvider>
        ),
      }
    );

    expect(factory).not.toHaveBeenCalled();
  });

  it("Should reread input requests from GET when the catalog stream opens", () => {
    const { opened, client } = renderStream("work");
    const invalidate = vi.spyOn(client, "invalidateQueries");

    opened[0].fake.emit("open", {});

    expect(invalidate).toHaveBeenCalledWith({
      queryKey: terminalKeys.catalog({ workspaceId: WORKSPACE, profileKey: "work" }),
      exact: true,
    });
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: terminalKeys.inputRequests({ workspaceId: WORKSPACE, profileKey: "work" }),
      exact: true,
    });
  });

  it("Should write a started recording into the scope's recordings cache", () => {
    const { opened, client } = renderStream("work");

    opened[0].fake.emit("terminal.recording_started", recordingStarted());

    expect(recordings(client, "work")?.[DEV_SERVER_TERMINAL.id]).toEqual({
      recordingId: "rec-1",
      at: RECORDING_AT,
      profileKey: "work",
    });
    expect(recordings(client, "personal")).toBeUndefined();
  });

  it("Should drop a recording on stop and ignore a frame without a terminal id", () => {
    const { opened, client } = renderStream("work");
    opened[0].fake.emit("terminal.recording_started", recordingStarted());

    opened[0].fake.emit("terminal.recording_started", recordingStarted({ terminal_id: "" }));
    expect(recordings(client, "work")?.[DEV_SERVER_TERMINAL.id]?.recordingId).toBe("rec-1");

    opened[0].fake.emit(
      "terminal.recording_stopped",
      recordingStarted({ event: "terminal.recording_stopped" })
    );
    expect(recordings(client, "work")).toEqual({});
  });

  it("Should ignore a recording frame addressed to another workspace", () => {
    const { opened, client } = renderStream("work");

    opened[0].fake.emit(
      "terminal.recording_started",
      recordingStarted({ workspace_id: "ws-other" })
    );

    expect(recordings(client, "work")?.[DEV_SERVER_TERMINAL.id]).toBeUndefined();
  });

  it("Should rehydrate active recordings after a matching snapshot or reconnect open", () => {
    const { opened, client } = renderStream("work");
    const liveAt = "2026-08-25T12:05:00.000Z";
    opened[0].fake.emit(
      "terminal.recording_started",
      recordingStarted({ recording_id: "rec-stale" })
    );

    opened[0].fake.emit("terminal.snapshot", { terminals: [DEV_SERVER_TERMINAL] });
    expect(recordings(client, "work")).toEqual({});

    opened[0].fake.emit(
      "terminal.recording_started",
      recordingStarted({ recording_id: "rec-live", at: liveAt })
    );
    expect(recordings(client, "work")?.[DEV_SERVER_TERMINAL.id]).toEqual({
      recordingId: "rec-live",
      at: liveAt,
      profileKey: "work",
    });

    opened[0].fake.emit("open", {});
    expect(recordings(client, "work")).toEqual({});
    opened[0].fake.emit("terminal.snapshot", { terminals: [DEV_SERVER_TERMINAL] });
    expect(recordings(client, "work")).toEqual({});
    opened[0].fake.emit(
      "terminal.recording_started",
      recordingStarted({ recording_id: "rec-live", at: liveAt })
    );
    expect(recordings(client, "work")?.[DEV_SERVER_TERMINAL.id]?.recordingId).toBe("rec-live");

    opened[0].fake.emit(
      "terminal.recording_started",
      recordingStarted({ workspace_id: "ws-other", recording_id: "rec-foreign" })
    );
    opened[0].fake.emit(
      "terminal.recording_stopped",
      recordingStarted({
        event: "terminal.recording_stopped",
        terminal_id: PSQL_TERMINAL.id,
        recording_id: "rec-stopped",
      })
    );
    expect(recordings(client, "work")).toEqual({
      [DEV_SERVER_TERMINAL.id]: {
        recordingId: "rec-live",
        at: liveAt,
        profileKey: "work",
      },
    });

    opened[0].fake.emit("terminal.snapshot", { terminals: [DEV_SERVER_TERMINAL] });
    expect(recordings(client, "work")).toEqual({});
  });

  it("Should drop a closed terminal's recording so its timer cannot linger", () => {
    const { opened, client } = renderStream("work");
    opened[0].fake.emit("terminal.snapshot", { terminals: [DEV_SERVER_TERMINAL] });
    opened[0].fake.emit("terminal.recording_started", recordingStarted());

    opened[0].fake.emit("terminal.closed", { terminal_id: DEV_SERVER_TERMINAL.id, exit: null });

    expect(recordings(client, "work")?.[DEV_SERVER_TERMINAL.id]).toBeUndefined();
  });

  it("Should replace only the owner covered by an aggregate recording snapshot", () => {
    const { opened, client } = renderAggregateStream();
    const personal = opened.find(entry => entry.url.includes("profile=personal"));
    const work = opened.find(entry => entry.url.includes("profile=work"));
    if (!personal || !work) throw new Error("expected both profile streams");

    work.fake.emit("terminal.recording_started", recordingStarted());
    personal.fake.emit(
      "terminal.recording_started",
      recordingStarted({
        terminal_id: PSQL_TERMINAL.id,
        recording_id: "rec-personal",
      })
    );
    expect(Object.keys(recordings(client, "@all") ?? {}).sort()).toEqual(
      [DEV_SERVER_TERMINAL.id, PSQL_TERMINAL.id].sort()
    );

    work.fake.emit("terminal.snapshot", { terminals: [DEV_SERVER_TERMINAL] });
    expect(Object.keys(recordings(client, "@all") ?? {})).toEqual([PSQL_TERMINAL.id]);

    work.fake.emit("terminal.recording_started", recordingStarted({ recording_id: "rec-work-2" }));
    expect(recordings(client, "@all")?.[DEV_SERVER_TERMINAL.id]).toEqual({
      recordingId: "rec-work-2",
      at: RECORDING_AT,
      profileKey: "work",
    });
    expect(recordings(client, "@all")?.[PSQL_TERMINAL.id]?.recordingId).toBe("rec-personal");
  });

  it("Should ignore a late recording frame from a source that has already been replaced", () => {
    const { opened, client, rerender } = renderStream("work");
    const deliverQueued = opened[0].fake.queue("terminal.recording_started", recordingStarted());

    rerender({ profileKey: "personal", workspaceId: WORKSPACE });
    deliverQueued();

    expect(recordings(client, "personal")).toBeUndefined();
    expect(recordings(client, "work")).toBeUndefined();
  });

  it("Should ignore hook event names that are not catalog frames", () => {
    const { opened, client } = renderStream("work");
    opened[0].fake.emit("terminal.snapshot", { terminals: [DEV_SERVER_TERMINAL] });
    const invalidate = vi.spyOn(client, "invalidateQueries");

    opened[0].fake.emit("terminal.input_requested", {
      terminal_id: DEV_SERVER_TERMINAL.id,
      request_id: "req-3f8a",
      redacted: true,
    });
    opened[0].fake.emit("terminal.input_provided", {
      terminal_id: DEV_SERVER_TERMINAL.id,
      request_id: "req-3f8a",
      outcome: "answered",
      redacted: true,
      length: 10,
    });

    expect(invalidate).not.toHaveBeenCalled();
    expect(catalog(client, "work")).toEqual([DEV_SERVER_TERMINAL]);
  });
});
