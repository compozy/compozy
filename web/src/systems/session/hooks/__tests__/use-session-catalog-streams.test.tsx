import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { resetProfileViews, setProfileView } from "@/systems/profiles";

import { sessionKeys } from "../../lib/query-keys";
import {
  useSessionCatalogStreams,
  type SessionCatalogEventSource,
} from "../use-session-catalog-streams";
import { sessionCatalogStreamsLogic } from "../session-catalog-streams-store";

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
    expect(invalidate).toHaveBeenCalledWith({ queryKey: sessionKeys.workspaceLists("ws_beta") });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: sessionKeys.workspaceLists("") });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: sessionKeys.attentionSummary() });
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
    expect(invalidate).toHaveBeenCalledWith({ queryKey: sessionKeys.workspaceLists("") });
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: sessionKeys.detail("", "sess_global"),
      exact: true,
    });

    act(() => sources[0]?.emit("open"));
    expect(invalidate).toHaveBeenCalledWith({ queryKey: sessionKeys.workspaceLists("") });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: sessionKeys.attentionSummary() });

    unmount();
    expect(sources[0]?.closed).toBe(true);
    expect([...sources[0]!.listeners.values()].every(listeners => listeners.size === 0)).toBe(true);
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
    resetProfileViews();
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
    expect(invalidate).toHaveBeenCalledWith({ queryKey: sessionKeys.workspaceLists(alpha.id) });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: sessionKeys.workspaceLists("") });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: sessionKeys.attentionSummary() });

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
