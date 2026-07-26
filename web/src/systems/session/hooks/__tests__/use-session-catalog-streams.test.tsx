import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import type { WorkspacePayload } from "@/systems/workspace";

import { sessionKeys } from "../../lib/query-keys";
import {
  useSessionCatalogStreams,
  type SessionCatalogEventSource,
} from "../use-session-catalog-streams";

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

function workspace(id: string): WorkspacePayload {
  return {
    id,
    name: id,
    root_dir: `/tmp/${id}`,
    add_dirs: [],
    created_at: "2026-07-13T12:00:00Z",
    updated_at: "2026-07-13T12:00:00Z",
  };
}

function wrapper(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useSessionCatalogStreams", () => {
  it("Should own one global source and scope every reconciliation by workspace", () => {
    const queryClient = new QueryClient();
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    const sources: FakeCatalogEventSource[] = [];
    const factory = (url: string) => {
      const source = new FakeCatalogEventSource(url);
      sources.push(source);
      return source;
    };
    const alpha = workspace("ws_alpha");
    const beta = workspace("ws_beta");
    const { rerender, unmount } = renderHook(
      ({ workspaces }) => useSessionCatalogStreams(workspaces, { eventSourceFactory: factory }),
      {
        initialProps: { workspaces: [alpha, beta, alpha] },
        wrapper: wrapper(queryClient),
      }
    );

    expect(sources.map(source => source.url)).toEqual(["/api/sessions/catalog-stream"]);

    act(() => {
      sources[0]?.emit("session_catalog_changed", {
        kind: "deleted",
        workspace_id: beta.id,
        session_id: "sess_beta",
      });
    });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: sessionKeys.workspaceLists(beta.id) });
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: sessionKeys.detail(beta.id, "sess_beta"),
      exact: true,
    });
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: sessionKeys.byId("sess_beta"),
      exact: true,
    });
    expect(invalidate).not.toHaveBeenCalledWith({
      queryKey: sessionKeys.workspaceLists(alpha.id),
    });

    invalidate.mockClear();
    act(() => {
      sources[0]?.emit("session_catalog_changed", {
        kind: "upserted",
        workspace_id: "ws_unknown",
        session_id: "sess_unknown",
      });
    });
    expect(invalidate).not.toHaveBeenCalled();

    act(() => sources[0]?.emit("open"));
    expect(invalidate).toHaveBeenCalledWith({ queryKey: sessionKeys.workspaceLists(alpha.id) });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: sessionKeys.workspaceLists(beta.id) });

    rerender({ workspaces: [alpha] });
    expect(sources[0]?.closed).toBe(true);
    expect([...sources[0]!.listeners.values()].every(listeners => listeners.size === 0)).toBe(true);

    const remaining = sources[1];
    expect(remaining?.url).toBe("/api/sessions/catalog-stream");
    unmount();
    expect(remaining?.closed).toBe(true);
    expect([...remaining!.listeners.values()].every(listeners => listeners.size === 0)).toBe(true);
  });

  it("Should keep the shell alive when global source construction fails", () => {
    const queryClient = new QueryClient();
    const factory = vi.fn(() => {
      throw new Error("EventSource unavailable");
    });

    renderHook(
      () =>
        useSessionCatalogStreams([workspace("ws_alpha"), workspace("ws_beta")], {
          eventSourceFactory: factory,
        }),
      { wrapper: wrapper(queryClient) }
    );

    expect(factory).toHaveBeenCalledTimes(1);
  });
});
