// Suite: Extension log stream
// Invariant: the panel binds one `(name, workspace)` instance, consumes only the named
// `extension_log` event, keeps sequences monotonic across a reconnect, and closes its source.
// Boundary IN: useExtensionLogs stream lifecycle and buffer merge.
// Boundary OUT: Panel rendering and daemon-side redaction.

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ExtensionLogEntry } from "../../types";

const mocks = vi.hoisted(() => ({ listExtensionLogs: vi.fn() }));

vi.mock("../../adapters/extensions-api", () => ({
  getExtensionProvenance: vi.fn(),
  listExtensionLogs: mocks.listExtensionLogs,
  listExtensions: vi.fn(),
}));

import { useExtensionLogs } from "../use-extension-logs";

class FakeEventSource {
  public close = vi.fn();
  public onerror: ((event: Event) => void) | null = null;
  public onopen: ((event: Event) => void) | null = null;
  private readonly listeners = new Map<string, EventListenerOrEventListenerObject[]>();

  constructor(public readonly url: string) {}

  addEventListener(type: string, listener: EventListenerOrEventListenerObject) {
    this.listeners.set(type, [...(this.listeners.get(type) ?? []), listener]);
  }

  removeEventListener(type: string, listener: EventListenerOrEventListenerObject) {
    this.listeners.set(
      type,
      (this.listeners.get(type) ?? []).filter(candidate => candidate !== listener)
    );
  }

  emit(type: string, data: unknown) {
    const event = new MessageEvent(type, { data: JSON.stringify(data) });
    for (const listener of this.listeners.get(type) ?? []) {
      if (typeof listener === "function") listener(event);
      else listener.handleEvent(event);
    }
  }
}

function logEntry(sequence: number, message: string): ExtensionLogEntry {
  return { message, sequence, timestamp: "2026-07-20T10:00:00Z" };
}

function setup(name: string, workspaceId: string | null) {
  const sources: FakeEventSource[] = [];
  const eventSourceFactory = (url: string) => {
    const source = new FakeEventSource(url);
    sources.push(source);
    return source;
  };
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client }, children);
  const view = renderHook(
    (props: { name: string; workspaceId: string | null }) =>
      useExtensionLogs({ ...props, eventSourceFactory }),
    { initialProps: { name, workspaceId }, wrapper }
  );
  return { sources, ...view };
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.listExtensionLogs.mockResolvedValue([logEntry(1, "boot")]);
});

describe("useExtensionLogs", () => {
  it("Should bind the workspace instance and append only newer sequences from the named event", async () => {
    const view = setup("ops-extension", "ws_northstar");

    await waitFor(() => expect(view.result.current.entries).toHaveLength(1));
    expect(view.sources[0]?.url).toBe(
      "/api/extensions/ops-extension/logs?follow=1&workspace=ws_northstar"
    );

    act(() => {
      view.sources[0]?.emit("extension_log", logEntry(2, "listening"));
      view.sources[0]?.emit("extension_log", logEntry(2, "duplicate"));
      view.sources[0]?.emit("extension_log", logEntry(1, "replayed"));
      view.sources[0]?.emit("message", logEntry(3, "unnamed events are ignored"));
    });

    expect(view.result.current.entries.map(entry => entry.message)).toEqual(["boot", "listening"]);
    expect(view.result.current.status).toBe("live");
  });

  it("Should retain rendered lines while reconnecting and stop the stream when follow is off", async () => {
    const view = setup("ops-extension", null);

    await waitFor(() => expect(view.result.current.entries).toHaveLength(1));
    act(() => {
      view.sources[0]?.emit("extension_log", logEntry(4, "still here"));
      view.sources[0]?.onerror?.(new Event("error"));
    });

    expect(view.result.current.status).toBe("reconnecting");
    expect(view.result.current.entries).toHaveLength(2);

    act(() => view.result.current.setFollow(false));

    expect(view.sources[0]?.close).toHaveBeenCalled();
    expect(view.result.current.status).toBe("paused");
    expect(view.result.current.entries).toHaveLength(2);
  });

  it("Should recreate the source and drop streamed rows when the instance changes", async () => {
    const view = setup("ops-extension", "ws_northstar");

    await waitFor(() => expect(view.sources).toHaveLength(1));
    act(() => {
      view.sources[0]?.emit("extension_log", logEntry(9, "workspace overlay line"));
    });
    expect(view.result.current.entries).toHaveLength(2);

    mocks.listExtensionLogs.mockResolvedValue([]);
    view.rerender({ name: "ops-extension", workspaceId: "ws_atlas" });

    await waitFor(() => expect(view.sources).toHaveLength(2));
    expect(view.sources[0]?.close).toHaveBeenCalled();
    expect(view.sources[1]?.url).toBe(
      "/api/extensions/ops-extension/logs?follow=1&workspace=ws_atlas"
    );
    await waitFor(() => expect(view.result.current.entries).toHaveLength(0));
  });

  it("Should close the source on unmount", async () => {
    const view = setup("ops-extension", null);

    await waitFor(() => expect(view.sources).toHaveLength(1));
    view.unmount();

    expect(view.sources[0]?.close).toHaveBeenCalled();
  });
});
