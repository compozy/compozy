// Suite: worktree catalog stream consumption
// Invariant (UT-141): every named event is registered through addEventListener (never onmessage
// alone), invalidation keys are workspace-qualified and dropped for unauthorized workspaces, and
// both the listeners and the source are released on unmount/disconnect (B-015, L-017/L-018 class).
// Owning layer: workspace stream hook. Canonical suite: this hook test.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { workspaceKeys } from "../../lib/query-keys";
import {
  useWorktreeCatalogStream,
  type WorktreeCatalogEventSource,
} from "../use-worktree-catalog-stream";
import { worktreeCatalogStreamLogic } from "../worktree-catalog-stream-store";

class FakeCatalogEventSource implements WorktreeCatalogEventSource {
  listeners = new Map<string, Set<EventListenerOrEventListenerObject>>();
  closed = false;

  addEventListener(type: string, listener: EventListenerOrEventListenerObject) {
    const existing = this.listeners.get(type) ?? new Set();
    existing.add(listener);
    this.listeners.set(type, existing);
  }

  removeEventListener(type: string, listener: EventListenerOrEventListenerObject) {
    this.listeners.get(type)?.delete(listener);
  }

  close() {
    this.closed = true;
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

  /** Listener count across every registered type. */
  get listenerCount(): number {
    let total = 0;
    for (const set of this.listeners.values()) total += set.size;
    return total;
  }
}

function setup(workspaces: Array<{ id: string }>) {
  const source = new FakeCatalogEventSource();
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const invalidate = vi.spyOn(queryClient, "invalidateQueries");
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  const view = renderHook(
    () => useWorktreeCatalogStream(workspaces, { eventSourceFactory: () => source }),
    { wrapper }
  );
  return { source, invalidate, queryClient, view };
}

describe("useWorktreeCatalogStream", () => {
  it("Should register named listeners rather than relying on onmessage", () => {
    const { source } = setup([{ id: "ws_alpha" }]);

    // A bare onmessage handler would silently miss every named frame.
    expect(source.listeners.has("worktree_catalog_changed")).toBe(true);
    expect(source.listeners.has("open")).toBe(true);
    expect(source.listeners.has("error")).toBe(true);
  });

  it("Should invalidate the workspace-qualified key named by the frame", () => {
    const { source, invalidate } = setup([{ id: "ws_alpha" }, { id: "ws_beta" }]);
    invalidate.mockClear();

    act(() => {
      source.emit("worktree_catalog_changed", {
        kind: "upserted",
        workspace_id: "ws_beta",
        worktree_id: "wt_one",
      });
    });

    expect(invalidate).toHaveBeenCalledWith({ queryKey: workspaceKeys.worktrees("ws_beta") });
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: workspaceKeys.worktreeDetail("ws_beta", "wt_one"),
      exact: true,
    });
    // The other workspace's cache is untouched.
    expect(invalidate).not.toHaveBeenCalledWith({
      queryKey: workspaceKeys.worktrees("ws_alpha"),
    });
  });

  it("Should retain the daemon failure reason after the accepted row rolls back", () => {
    const { source, queryClient } = setup([{ id: "ws_alpha" }]);

    act(() => {
      source.emit("worktree_catalog_changed", {
        kind: "failed",
        workspace_id: "ws_alpha",
        worktree_id: "wt_one",
        error: "checkout failed",
      });
    });

    expect(
      queryClient.getQueryData(workspaceKeys.worktreeMaterializationFailure("ws_alpha", "wt_one"))
    ).toBe("checkout failed");
  });

  it("Should drop a frame for a workspace this client does not hold", () => {
    const { source, invalidate } = setup([{ id: "ws_alpha" }]);
    invalidate.mockClear();

    act(() => {
      source.emit("worktree_catalog_changed", {
        kind: "upserted",
        workspace_id: "ws_intruder",
        worktree_id: "wt_one",
      });
    });

    // Nothing crosses the workspace boundary, not even an invalidation.
    expect(invalidate).not.toHaveBeenCalled();
  });

  it("Should ignore a malformed frame instead of invalidating blindly", () => {
    const { source, invalidate } = setup([{ id: "ws_alpha" }]);
    invalidate.mockClear();

    act(() => {
      source.emit("worktree_catalog_changed", { kind: "upserted", workspace_id: "  " });
      source.emit("worktree_catalog_changed");
    });

    expect(invalidate).not.toHaveBeenCalled();
  });

  it("Should reread every authorized workspace on reconnect", () => {
    const { source, invalidate } = setup([{ id: "ws_alpha" }, { id: "ws_beta" }]);
    invalidate.mockClear();

    // A reconnect may have missed frames, so each workspace rereads.
    act(() => source.emit("open"));

    expect(invalidate).toHaveBeenCalledWith({ queryKey: workspaceKeys.worktrees("ws_alpha") });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: workspaceKeys.worktrees("ws_beta") });
  });

  it("Should promote the stream to live and back to stale within one generation", () => {
    const store = worktreeCatalogStreamLogic.createStore();
    const [connecting] = store.transition(store.getInitialSnapshot(), {
      type: "connectionRequested",
      connect: () => () => {},
    });

    const [live] = store.transition(connecting, {
      type: "connectionLive",
      generation: connecting.context.generation,
    });
    expect(live.context.status).toBe("live");

    const [stale] = store.transition(live, {
      type: "connectionStale",
      generation: connecting.context.generation,
    });
    expect(stale.context.status).toBe("stale");
  });

  it("Should keep a late status event behind the current connection generation", () => {
    const store = worktreeCatalogStreamLogic.createStore();
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

    // A superseded connection cannot resurrect its own status.
    expect(afterLateFailure.context.status).toBe("connecting");
  });

  it("Should release every listener and close the source on unmount", () => {
    const { source, view } = setup([{ id: "ws_alpha" }]);
    expect(source.listenerCount).toBeGreaterThan(0);

    view.unmount();

    expect(source.listenerCount).toBe(0);
    expect(source.closed).toBe(true);
  });

  it("Should stay disabled with no authorized workspaces", () => {
    const { source, view } = setup([]);

    expect(view.result.current).toBe("disabled");
    expect(source.listenerCount).toBe(0);
  });
});
