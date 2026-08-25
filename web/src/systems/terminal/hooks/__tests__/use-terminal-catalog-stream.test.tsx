import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import type { StreamEventSource } from "@/lib/ticketed-event-source";

import { terminalKeys } from "../../lib/query-keys";
import { DEV_SERVER_TERMINAL, PSQL_TERMINAL } from "../../mocks/terminal-fixtures";
import type { TerminalInfo } from "../../types";
import { useTerminalCatalogStream } from "../use-terminal-catalog-stream";

/**
 * Canonical suite for the live terminal catalog (part of UT-113).
 *
 * Invariant: exactly one catalog subscription exists per `(workspace, profile)`;
 * switching either closes the previous source before opening the next, and a
 * frame that arrives from a closed source never reaches the cache that replaced
 * it. The store's own rebinding reducer is covered where the store lives —
 * this file owns the subscription's lifetime.
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

function renderStream(initialProfile: string) {
  const opened: { url: string; fake: ReturnType<typeof createFakeSource> }[] = [];
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  const view = renderHook(
    ({ profileKey }: { profileKey: string }) =>
      useTerminalCatalogStream({
        workspaceId: WORKSPACE,
        profileKey,
        eventSourceFactory: (url: string) => {
          const fake = createFakeSource();
          opened.push({ url, fake });
          return fake.source as unknown as StreamEventSource;
        },
      }),
    { initialProps: { profileKey: initialProfile }, wrapper }
  );
  return { ...view, client, opened };
}

function catalog(client: QueryClient, profileKey: string): TerminalInfo[] | undefined {
  return client.getQueryData(terminalKeys.catalog({ workspaceId: WORKSPACE, profileKey }));
}

describe("useTerminalCatalogStream", () => {
  it("Should open exactly one source for the scope it was given", () => {
    const { opened } = renderStream("work");

    expect(opened).toHaveLength(1);
    expect(opened[0].url).toContain(`/api/workspaces/${WORKSPACE}/terminals/stream`);
    expect(opened[0].url).toContain("profile=work");
    expect(opened[0].fake.listenerCount()).toBeGreaterThan(0);
  });

  it("Should fold a snapshot and its patches into the scope's own cache entry", () => {
    const { opened, client } = renderStream("work");

    opened[0].fake.emit("terminal.snapshot", { terminals: [DEV_SERVER_TERMINAL] });
    expect(catalog(client, "work")?.map(terminal => terminal.id)).toEqual([DEV_SERVER_TERMINAL.id]);

    opened[0].fake.emit("terminal.title_changed", {
      terminal_id: DEV_SERVER_TERMINAL.id,
      title: "dev server (staging)",
    });
    expect(catalog(client, "work")?.[0].title).toBe("dev server (staging)");
  });

  it("Should close the previous source and open the next one on a profile switch", () => {
    const { opened, rerender } = renderStream("work");

    rerender({ profileKey: "personal" });

    expect(opened).toHaveLength(2);
    expect(opened[0].fake.closed).toBe(true);
    expect(opened[0].fake.listenerCount()).toBe(0);
    expect(opened[1].url).toContain("profile=personal");
    expect(opened[1].fake.closed).toBe(false);
  });

  it("Should ignore a late frame from a source that has already been replaced", () => {
    const { opened, client, rerender } = renderStream("work");
    opened[0].fake.emit("terminal.snapshot", { terminals: [DEV_SERVER_TERMINAL] });

    rerender({ profileKey: "personal" });
    // The socket is closing, but its last event was already queued.
    opened[0].fake.emit("terminal.created", { terminal: PSQL_TERMINAL });

    // Neither cache entry may take it: the new scope never saw it, and the old
    // one is no longer being read.
    expect(catalog(client, "personal")).toBeUndefined();
    expect(catalog(client, "work")?.map(terminal => terminal.id)).toEqual([DEV_SERVER_TERMINAL.id]);
  });

  it("Should drop a frame it cannot read rather than merge it half-understood", () => {
    const { opened, client } = renderStream("work");
    opened[0].fake.emit("terminal.snapshot", { terminals: [DEV_SERVER_TERMINAL] });

    opened[0].fake.emit("terminal.created", { terminal: { id: "term-broken" } });

    expect(catalog(client, "work")?.map(terminal => terminal.id)).toEqual([DEV_SERVER_TERMINAL.id]);
  });

  it("Should close its source when the surface goes away", () => {
    const { opened, unmount } = renderStream("work");

    unmount();

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
});
