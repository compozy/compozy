// Suite: cmd-palette catalog invalidation stream
// Invariant: the consumer subscribes through a NAMED event listener (L-017),
// one `cmd_palette.catalog.changed` frame produces exactly one catalog
// reconciliation converging on the daemon revision, frames for another
// workspace are dropped before any cache write, and teardown detaches.
// Boundary IN: the stream lib's listener wiring and the hook's invalidation.
// Boundary OUT: catalog hydration, availability evaluation, and dispatch.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { act, type ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import type { StreamEventSource } from "@/lib/ticketed-event-source";

import {
  CMD_PALETTE_CATALOG_CHANGED_EVENT,
  cmdPaletteStreamUrl,
} from "../../lib/cmd-palette-stream";
import { useCmdPaletteStream } from "../use-cmd-palette-stream";

const WORKSPACE = "ws-hq";

class FakeEventSource implements StreamEventSource {
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  onopen: ((event: Event) => void) | null = null;
  readonly listeners = new Map<string, Set<EventListenerOrEventListenerObject>>();
  closed = false;

  constructor(readonly url: string) {}

  addEventListener(type: string, listener: EventListenerOrEventListenerObject): void {
    const existing = this.listeners.get(type);
    if (existing) existing.add(listener);
    else this.listeners.set(type, new Set([listener]));
  }

  removeEventListener(type: string, listener: EventListenerOrEventListenerObject): void {
    this.listeners.get(type)?.delete(listener);
  }

  close(): void {
    this.closed = true;
  }

  emit(type: string, data?: unknown): void {
    const event =
      data === undefined ? new Event(type) : new MessageEvent(type, { data: JSON.stringify(data) });
    for (const listener of Array.from(this.listeners.get(type) ?? [])) {
      if (typeof listener === "function") listener(event);
      else listener.handleEvent(event);
    }
  }

  listenerCount(type: string): number {
    return this.listeners.get(type)?.size ?? 0;
  }
}

function harness(workspaceId: string | null = WORKSPACE) {
  const sources: FakeEventSource[] = [];
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const invalidate = vi.spyOn(queryClient, "invalidateQueries");
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  const rendered = renderHook(
    () =>
      useCmdPaletteStream({
        workspaceId,
        eventSourceFactory: url => {
          const source = new FakeEventSource(url);
          sources.push(source);
          return source;
        },
      }),
    { wrapper }
  );
  return { ...rendered, invalidate, sources };
}

describe("useCmdPaletteStream (UT-104)", () => {
  it("Should subscribe through the named catalog event and never a bare message handler", () => {
    const { sources } = harness();
    const [source] = sources;
    expect(source?.url).toBe(cmdPaletteStreamUrl(WORKSPACE));
    expect(source?.listenerCount(CMD_PALETTE_CATALOG_CHANGED_EVENT)).toBe(1);
    // L-017: a bare `onmessage` would swallow every other family on the socket.
    expect(source?.onmessage).toBeNull();
    expect(source?.listenerCount("message")).toBe(0);
  });

  it("Should reconcile exactly once per catalog event", async () => {
    const { invalidate, sources } = harness();
    const [source] = sources;
    invalidate.mockClear();
    source?.emit(CMD_PALETTE_CATALOG_CHANGED_EVENT, {
      workspace: WORKSPACE,
      catalog_revision: "sha256:catalog-2",
    });
    await waitFor(() => expect(invalidate).toHaveBeenCalledTimes(1));
    const call = invalidate.mock.calls[0]?.[0];
    expect(call).toEqual({ predicate: expect.any(Function) });
    expect(
      call?.predicate?.({ queryKey: ["cmd-palette", "catalog", WORKSPACE, "client-a"] } as never)
    ).toBe(true);
    expect(
      call?.predicate?.({ queryKey: ["cmd-palette", "rank-signals", WORKSPACE] } as never)
    ).toBe(true);
    expect(call?.predicate?.({ queryKey: ["cmd-palette", "catalog", "ws-other"] } as never)).toBe(
      false
    );
  });

  it("Should reconcile on open because the stream carries no replay cursor", async () => {
    const { invalidate, sources } = harness();
    const [source] = sources;
    invalidate.mockClear();
    act(() => source?.emit("open"));
    await waitFor(() => expect(invalidate).toHaveBeenCalledTimes(1));
  });

  it("Should drop frames belonging to another workspace before any cache write", async () => {
    const { invalidate, sources } = harness();
    const [source] = sources;
    invalidate.mockClear();
    source?.emit(CMD_PALETTE_CATALOG_CHANGED_EVENT, {
      workspace: "ws-other",
      catalog_revision: "sha256:foreign",
    });
    source?.emit(CMD_PALETTE_CATALOG_CHANGED_EVENT, { workspace: WORKSPACE });
    await new Promise(resolve => setTimeout(resolve, 0));
    expect(invalidate).not.toHaveBeenCalled();
  });

  it("Should report the stream stale on error and live again on open", async () => {
    const { result, sources } = harness();
    const [source] = sources;
    act(() => source?.emit("error"));
    await waitFor(() => expect(result.current).toBe("stale"));
    act(() => source?.emit("open"));
    await waitFor(() => expect(result.current).toBe("live"));
  });

  it("Should detach every listener and close the socket on teardown", () => {
    const { sources, unmount } = harness();
    const [source] = sources;
    unmount();
    expect(source?.closed).toBe(true);
    expect(source?.listenerCount(CMD_PALETTE_CATALOG_CHANGED_EVENT)).toBe(0);
    expect(source?.listenerCount("open")).toBe(0);
    expect(source?.listenerCount("error")).toBe(0);
  });

  it("Should stay disabled without a workspace", () => {
    const { result, sources } = harness(null);
    expect(result.current).toBe("disabled");
    expect(sources).toHaveLength(0);
  });
});
