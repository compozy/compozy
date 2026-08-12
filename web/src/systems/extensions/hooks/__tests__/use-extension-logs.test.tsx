// Suite: Extension log stream
// Invariant: one `(name, workspace)` Query snapshot owns log data, while the XState store owns only
// follow and connection lifecycle. Every resumed cursor is paired with its stream epoch, and a
// daemon ring replacement atomically replaces the cached snapshot before later deltas append.
// Boundary IN: useExtensionLogs stream lifecycle and canonical cache updates.
// Boundary OUT: Panel rendering and daemon-side redaction.

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { ExtensionLogEntry, ExtensionLogsSnapshot } from "../../types";
import { extensionLogsOptions } from "../../lib/query-options";

const mocks = vi.hoisted(() => ({ listExtensionLogs: vi.fn() }));

vi.mock("../../adapters/extensions-api", () => ({
  getExtensionProvenance: vi.fn(),
  listExtensionLogs: mocks.listExtensionLogs,
  listExtensions: vi.fn(),
}));

import { extensionLogsLogic } from "../extension-logs-store";
import {
  EXTENSION_LOG_EVENT_NAME,
  EXTENSION_LOG_RESET_EVENT_NAME,
  useExtensionLogs,
} from "../use-extension-logs";

class FakeEventSource {
  public close = vi.fn();
  public onerror: ((event: Event) => void) | null = null;
  public onopen: ((event: Event) => void) | null = null;
  private readonly listeners = new Map<string, EventListenerOrEventListenerObject[]>();
  private readonly historicalListeners = new Map<string, EventListenerOrEventListenerObject[]>();

  constructor(public readonly url: string) {}

  addEventListener(type: string, listener: EventListenerOrEventListenerObject) {
    this.listeners.set(type, [...(this.listeners.get(type) ?? []), listener]);
    this.historicalListeners.set(type, [...(this.historicalListeners.get(type) ?? []), listener]);
  }

  removeEventListener(type: string, listener: EventListenerOrEventListenerObject) {
    this.listeners.set(
      type,
      (this.listeners.get(type) ?? []).filter(candidate => candidate !== listener)
    );
  }

  emit(type: string, data: unknown) {
    this.emitTo(this.listeners.get(type) ?? [], type, data);
  }

  emitAfterClose(type: string, data: unknown) {
    this.emitTo(this.historicalListeners.get(type) ?? [], type, data);
  }

  private emitTo(
    listeners: readonly EventListenerOrEventListenerObject[],
    type: string,
    data: unknown
  ) {
    const event = new MessageEvent(type, { data: JSON.stringify(data) });
    for (const listener of listeners) {
      if (typeof listener === "function") listener(event);
      else listener.handleEvent(event);
    }
  }
}

function logEntry(sequence: number, message: string, streamEpoch = "epoch-a"): ExtensionLogEntry {
  return {
    message,
    sequence,
    stream_epoch: streamEpoch,
    timestamp: "2026-07-20T10:00:00Z",
  };
}

function logSnapshot(
  streamEpoch: string,
  logs: readonly ExtensionLogEntry[]
): ExtensionLogsSnapshot {
  return { logs: [...logs], stream_epoch: streamEpoch };
}

function setup(
  name: string,
  workspaceId: string | null,
  enabled = true,
  seed?: ExtensionLogsSnapshot,
  provideEventSourceFactory = true
) {
  const sources: FakeEventSource[] = [];
  const eventSourceFactory = (url: string) => {
    const source = new FakeEventSource(url);
    sources.push(source);
    return source;
  };
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  if (seed) {
    client.setQueryData(extensionLogsOptions(name, { workspaceId }).queryKey, seed);
  }
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client }, children);
  const view = renderHook(
    (props: { enabled?: boolean; name: string; workspaceId: string | null }) =>
      useExtensionLogs({
        ...props,
        ...(provideEventSourceFactory ? { eventSourceFactory } : {}),
      }),
    { initialProps: { enabled, name, workspaceId }, wrapper }
  );
  return { client, sources, ...view };
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.listExtensionLogs.mockResolvedValue(logSnapshot("epoch-a", [logEntry(1, "boot")]));
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("useExtensionLogs", () => {
  it("Should admit only current-generation stream lifecycle transitions", () => {
    const store = extensionLogsLogic.createStore();

    store.trigger.streamOpened({ generation: 0 });
    expect(store.getSnapshot().context.streamStatus).toBe("idle");

    store.trigger.connecting();
    const generation = store.getSnapshot().context.generation;
    store.trigger.streamOpened({ generation: generation - 1 });
    expect(store.getSnapshot().context.streamStatus).toBe("connecting");

    store.trigger.streamOpened({ generation });
    expect(store.getSnapshot().context.streamStatus).toBe("live");
    store.trigger.streamFailed({ generation });
    expect(store.getSnapshot().context.streamStatus).toBe("reconnecting");

    store.trigger.retryRequested();
    expect(store.getSnapshot().context).toMatchObject({
      generation: generation + 1,
      retryToken: 1,
      streamStatus: "idle",
    });
    store.trigger.streamSuspended({ generation });
    expect(store.getSnapshot().context.generation).toBe(generation + 1);
    store.trigger.streamFailed({ generation });
    expect(store.getSnapshot().context.streamStatus).toBe("idle");
  });

  it("Should finish a fresh baseline read before opening the source even with stale cache", async () => {
    let resolveBaseline: ((snapshot: ExtensionLogsSnapshot) => void) | undefined;
    mocks.listExtensionLogs.mockReturnValueOnce(
      new Promise<ExtensionLogsSnapshot>(resolve => {
        resolveBaseline = resolve;
      })
    );
    const retained = logSnapshot("epoch-old", [logEntry(8, "retained", "epoch-old")]);
    const view = setup("ops-extension", "ws_northstar", true, retained);

    expect(view.result.current.entries.map(entry => entry.message)).toEqual(["retained"]);
    expect(view.sources).toHaveLength(0);

    resolveBaseline?.(logSnapshot("epoch-new", [logEntry(1, "fresh", "epoch-new")]));
    await waitFor(() => expect(view.sources).toHaveLength(1));
    expect(view.sources[0]?.url).toBe(
      "/api/extensions/ops-extension/logs?follow=1&workspace=ws_northstar&after=1&stream_epoch=epoch-new"
    );
    expect(view.result.current.entries.map(entry => entry.message)).toEqual(["fresh"]);
  });

  it("Should still load one-shot history when the runtime has no EventSource", async () => {
    vi.stubGlobal("EventSource", undefined);
    const view = setup("ops-extension", null, true, undefined, false);

    await waitFor(() =>
      expect(view.result.current.entries.map(entry => entry.message)).toEqual(["boot"])
    );
    expect(view.sources).toHaveLength(0);
    expect(view.result.current.isLoading).toBe(false);
    expect(view.result.current.status).toBe("idle");
  });

  it("Should expose a source-construction failure through the lifecycle store", async () => {
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => undefined);
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client }, children);
    const view = renderHook(
      () =>
        useExtensionLogs({
          eventSourceFactory: () => {
            throw new Error("source unavailable");
          },
          name: "ops-extension",
        }),
      { wrapper }
    );

    await waitFor(() => expect(view.result.current.error?.message).toBe("source unavailable"));
    expect(view.result.current.status).toBe("reconnecting");
    expect(view.result.current.entries.map(entry => entry.message)).toEqual(["boot"]);
    errorSpy.mockRestore();
  });

  it("Should append only same-epoch deltas to the workspace Query snapshot", async () => {
    const view = setup("ops-extension", "ws_northstar");

    await waitFor(() => expect(view.sources).toHaveLength(1));
    expect(view.sources[0]?.url).toBe(
      "/api/extensions/ops-extension/logs?follow=1&workspace=ws_northstar&after=1&stream_epoch=epoch-a"
    );

    act(() => {
      view.sources[0]?.emit(EXTENSION_LOG_EVENT_NAME, logEntry(2, "listening"));
      view.sources[0]?.emit(EXTENSION_LOG_EVENT_NAME, logEntry(2, "duplicate"));
      view.sources[0]?.emit(EXTENSION_LOG_EVENT_NAME, logEntry(1, "replayed"));
      view.sources[0]?.emit(
        EXTENSION_LOG_EVENT_NAME,
        logEntry(1, "wrong ring without reset", "epoch-b")
      );
      view.sources[0]?.emit("message", logEntry(3, "unnamed events are ignored"));
    });

    const expected = logSnapshot("epoch-a", [logEntry(1, "boot"), logEntry(2, "listening")]);
    await waitFor(() => expect(view.result.current.entries).toEqual(expected.logs));
    expect(
      view.client.getQueryData(
        extensionLogsOptions("ops-extension", { workspaceId: "ws_northstar" }).queryKey
      )
    ).toEqual(expected);
    expect(view.result.current.status).toBe("live");
  });

  it("Should apply replacement snapshots atomically before appending later deltas", async () => {
    mocks.listExtensionLogs.mockResolvedValueOnce(
      logSnapshot("epoch-a", [logEntry(8, "prior daemon", "epoch-a")])
    );
    const view = setup("ops-extension", "ws_northstar");
    await waitFor(() => expect(view.sources).toHaveLength(1));

    act(() => {
      view.sources[0]?.emit(
        EXTENSION_LOG_RESET_EVENT_NAME,
        logSnapshot("epoch-b", [
          logEntry(2, "second", "epoch-b"),
          logEntry(1, "restarted ring", "epoch-b"),
        ])
      );
      view.sources[0]?.emit(EXTENSION_LOG_EVENT_NAME, logEntry(3, "live after reset", "epoch-b"));
    });

    await waitFor(() =>
      expect(view.result.current.entries.map(entry => entry.message)).toEqual([
        "restarted ring",
        "second",
        "live after reset",
      ])
    );
    expect(
      view.client.getQueryData<ExtensionLogsSnapshot>(
        extensionLogsOptions("ops-extension", { workspaceId: "ws_northstar" }).queryKey
      )?.stream_epoch
    ).toBe("epoch-b");

    act(() => {
      view.sources[0]?.emit(EXTENSION_LOG_RESET_EVENT_NAME, logSnapshot("epoch-c", []));
    });
    await waitFor(() => expect(view.result.current.entries).toEqual([]));
    expect(
      view.client.getQueryData(
        extensionLogsOptions("ops-extension", { workspaceId: "ws_northstar" }).queryKey
      )
    ).toEqual(logSnapshot("epoch-c", []));

    act(() => {
      view.sources[0]?.emit(EXTENSION_LOG_RESET_EVENT_NAME, {
        logs: [logEntry(1, "mismatched reset", "epoch-e")],
        stream_epoch: "epoch-d",
      });
    });
    expect(
      view.client.getQueryData(
        extensionLogsOptions("ops-extension", { workspaceId: "ws_northstar" }).queryKey
      )
    ).toEqual(logSnapshot("epoch-c", []));
  });

  it("Should retain cached lines while the EventSource reconnects and follow is paused", async () => {
    const view = setup("ops-extension", null);
    await waitFor(() => expect(view.sources).toHaveLength(1));

    act(() => {
      view.sources[0]?.emit(EXTENSION_LOG_EVENT_NAME, logEntry(2, "still here"));
      view.sources[0]?.onerror?.(new Event("error"));
    });
    expect(view.result.current.status).toBe("reconnecting");
    await waitFor(() => expect(view.result.current.entries).toHaveLength(2));

    act(() => view.sources[0]?.onopen?.(new Event("open")));
    expect(view.result.current.status).toBe("live");
    act(() => view.result.current.setFollow(false));

    expect(view.sources[0]?.close).toHaveBeenCalledOnce();
    expect(view.result.current.status).toBe("paused");
    expect(view.result.current.entries.map(entry => entry.message)).toEqual(["boot", "still here"]);
  });

  it("Should resume safely from a retained epoch when the fresh baseline is unavailable", async () => {
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => undefined);
    mocks.listExtensionLogs.mockRejectedValueOnce(new Error("history unavailable"));
    const retained = logSnapshot("epoch-old", [logEntry(4, "retained", "epoch-old")]);
    const view = setup("ops-extension", null, true, retained);

    await waitFor(() => expect(view.sources).toHaveLength(1));
    expect(view.sources[0]?.url).toBe(
      "/api/extensions/ops-extension/logs?follow=1&after=4&stream_epoch=epoch-old"
    );
    act(() => {
      view.sources[0]?.emit(
        EXTENSION_LOG_RESET_EVENT_NAME,
        logSnapshot("epoch-new", [logEntry(1, "recovered", "epoch-new")])
      );
    });
    await waitFor(() =>
      expect(view.result.current.entries.map(entry => entry.message)).toEqual(["recovered"])
    );
    errorSpy.mockRestore();
  });

  it("Should expose a failed empty baseline without loading forever while paused", async () => {
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => undefined);
    mocks.listExtensionLogs
      .mockRejectedValueOnce(new Error("history unavailable"))
      .mockResolvedValueOnce(logSnapshot("epoch-retry", [logEntry(1, "ready", "epoch-retry")]));
    const view = setup("ops-extension", null);

    await waitFor(() => expect(view.result.current.error?.message).toBe("history unavailable"));
    expect(view.sources).toHaveLength(0);
    expect(view.result.current.status).toBe("reconnecting");

    act(() => view.result.current.setFollow(false));
    expect(view.result.current.isLoading).toBe(false);
    expect(view.result.current.error?.message).toBe("history unavailable");

    act(() => view.result.current.setFollow(true));
    await waitFor(() => expect(view.sources).toHaveLength(1));
    expect(view.sources[0]?.url).toContain("stream_epoch=epoch-retry");
    expect(view.result.current.entries.map(entry => entry.message)).toEqual(["ready"]);
    errorSpy.mockRestore();
  });

  it("Should normalize and bound a complete history before caching it", async () => {
    mocks.listExtensionLogs.mockResolvedValueOnce(
      logSnapshot(
        "epoch-a",
        Array.from({ length: 520 }, (_, index) => logEntry(520 - index, `line ${520 - index}`))
      )
    );
    const view = setup(" ops-extension ", " ws_northstar ");

    await waitFor(() => expect(view.result.current.entries).toHaveLength(500));
    expect(view.result.current.entries[0]?.sequence).toBe(21);
    expect(view.result.current.entries[499]?.sequence).toBe(520);
    expect(view.sources[0]?.url).toBe(
      "/api/extensions/ops-extension/logs?follow=1&workspace=ws_northstar&after=520&stream_epoch=epoch-a"
    );
    expect(
      extensionLogsOptions(" ops-extension ", { workspaceId: " ws_northstar " }).queryKey
    ).toEqual(extensionLogsOptions("ops-extension", { workspaceId: "ws_northstar" }).queryKey);
  });

  it("Should fence the old source while a requested fresh snapshot is pending", async () => {
    const view = setup("ops-extension", "ws_northstar");
    await waitFor(() => expect(view.sources).toHaveLength(1));
    const cancelSpy = vi.spyOn(view.client, "cancelQueries");
    let resolveRefresh: ((snapshot: ExtensionLogsSnapshot) => void) | undefined;
    mocks.listExtensionLogs.mockReturnValueOnce(
      new Promise<ExtensionLogsSnapshot>(resolve => {
        resolveRefresh = resolve;
      })
    );

    act(() => {
      view.sources[0]?.emit(EXTENSION_LOG_EVENT_NAME, logEntry(2, "queued old line"));
      view.result.current.refetch();
    });
    expect(view.sources[0]?.close).toHaveBeenCalledOnce();
    act(() => {
      view.sources[0]?.emitAfterClose(EXTENSION_LOG_EVENT_NAME, logEntry(2, "late old source"));
    });
    expect(view.sources).toHaveLength(1);

    resolveRefresh?.(logSnapshot("epoch-b", [logEntry(1, "fresh source", "epoch-b")]));
    await waitFor(() => expect(view.sources).toHaveLength(2));
    expect(cancelSpy).toHaveBeenCalledOnce();
    expect(view.result.current.entries.map(entry => entry.message)).toEqual(["fresh source"]);
    expect(view.sources[1]?.url).toContain("stream_epoch=epoch-b");
  });

  it("Should suspend a live source and reconnect when its owner becomes active again", async () => {
    const view = setup("ops-extension", null);

    await waitFor(() => expect(view.sources).toHaveLength(1));
    act(() => view.sources[0]?.onopen?.(new Event("open")));
    expect(view.result.current.status).toBe("live");

    view.rerender({ enabled: false, name: "ops-extension", workspaceId: null });
    expect(view.sources[0]?.close).toHaveBeenCalledOnce();
    expect(view.result.current.status).toBe("idle");

    view.rerender({ enabled: true, name: "ops-extension", workspaceId: null });
    await waitFor(() => expect(view.sources).toHaveLength(2));
    act(() => view.sources[1]?.onopen?.(new Event("open")));
    expect(view.result.current.status).toBe("live");
  });

  it("Should recreate the scoped Query and reject late frames when the instance changes", async () => {
    const view = setup("ops-extension", "ws_northstar");
    await waitFor(() => expect(view.sources).toHaveLength(1));
    mocks.listExtensionLogs.mockResolvedValueOnce(logSnapshot("epoch-atlas", []));

    view.rerender({ enabled: true, name: "ops-extension", workspaceId: "ws_atlas" });
    await waitFor(() => expect(view.sources).toHaveLength(2));
    expect(view.sources[0]?.close).toHaveBeenCalledOnce();
    expect(view.sources[1]?.url).toBe(
      "/api/extensions/ops-extension/logs?follow=1&workspace=ws_atlas&stream_epoch=epoch-atlas"
    );
    await waitFor(() => expect(view.result.current.entries).toEqual([]));

    act(() => {
      view.sources[0]?.emitAfterClose(EXTENSION_LOG_EVENT_NAME, logEntry(2, "late old workspace"));
      view.sources[0]?.emitAfterClose(
        EXTENSION_LOG_RESET_EVENT_NAME,
        logSnapshot("epoch-late", [logEntry(1, "late reset", "epoch-late")])
      );
    });
    expect(
      view.client.getQueryData(
        extensionLogsOptions("ops-extension", { workspaceId: "ws_atlas" }).queryKey
      )
    ).toEqual(logSnapshot("epoch-atlas", []));
  });

  it("Should close the source on unmount", async () => {
    const view = setup("ops-extension", null);

    await waitFor(() => expect(view.sources).toHaveLength(1));
    view.unmount();

    expect(view.sources[0]?.close).toHaveBeenCalledOnce();
  });
});
