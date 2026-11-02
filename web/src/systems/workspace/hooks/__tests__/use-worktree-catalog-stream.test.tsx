// Suite: worktree catalog stream consumption
// Invariant (UT-141): every named event is registered through addEventListener (never onmessage
// alone), invalidation keys are workspace-qualified and dropped for unauthorized workspaces, and
// both the listeners and the source are released on unmount/disconnect (B-015, L-017/L-018 class).
// Owning layer: workspace stream hook. Canonical suite: this hook test.
import { QueryClient, QueryClientProvider, QueryObserver } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { workspaceKeys } from "../../lib/query-keys";
import {
  WORKTREE_REREAD_COALESCE_MS,
  WORKTREE_REREAD_MAX_WAIT_MS,
} from "../../lib/worktree-reread";
import { useWorktrees, useWorktreeListings } from "../use-worktrees";
import * as worktreeApi from "../../adapters/worktree-api";
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
  const eventSourceFactory = () => source;
  const view = renderHook(() => useWorktreeCatalogStream(workspaces, { eventSourceFactory }), {
    wrapper,
  });
  return { source, invalidate, queryClient, view };
}

describe("useWorktreeCatalogStream", () => {
  it("Should reconcile every affected workspace by the ceiling during continuous frames", () => {
    vi.useFakeTimers();
    const { source, queryClient, view } = setup([{ id: "a" }, { id: "b" }]);
    const keys = [workspaceKeys.worktrees("a"), workspaceKeys.worktrees("b")];
    for (const key of keys) queryClient.setQueryData(key, {});
    try {
      act(() => {
        for (let elapsed = 0; elapsed < WORKTREE_REREAD_MAX_WAIT_MS; elapsed += 100) {
          source.emit("worktree_catalog_changed", {
            kind: "updated",
            workspace_id: elapsed % 200 === 0 ? "a" : "b",
            worktree_id: "wt",
          });
          vi.advanceTimersByTime(99);
          for (const key of keys) expect(queryClient.getQueryState(key)?.isInvalidated).toBe(false);
          vi.advanceTimersByTime(1);
        }
      });
      for (const key of keys) expect(queryClient.getQueryState(key)?.isInvalidated).toBe(true);
    } finally {
      view.unmount();
      queryClient.clear();
      vi.useRealTimers();
    }
  });

  it.each(["error", "unmount"])("Should flush pending work on %s without a late reread", reason => {
    vi.useFakeTimers();
    const { source, queryClient, view, invalidate } = setup([{ id: "a" }, { id: "b" }]);
    const affected = workspaceKeys.worktreeDetail("a", "wt");
    const untouched = workspaceKeys.worktreeDetail("b", "wt");
    queryClient.setQueryData(affected, {});
    queryClient.setQueryData(untouched, {});
    try {
      act(() => {
        source.emit("worktree_catalog_changed", {
          kind: "updated",
          workspace_id: "a",
          worktree_id: "wt",
        });
      });
      expect(queryClient.getQueryState(affected)?.isInvalidated).toBe(false);
      act(() => {
        if (reason === "error") source.emit("error");
        else view.unmount();
      });
      expect(queryClient.getQueryState(affected)?.isInvalidated).toBe(true);
      expect(queryClient.getQueryState(untouched)?.isInvalidated).toBe(false);
      if (reason === "error") expect(view.result.current).toBe("stale");
      invalidate.mockClear();
      act(() => vi.advanceTimersByTime(WORKTREE_REREAD_MAX_WAIT_MS));
      expect(invalidate).not.toHaveBeenCalled();
    } finally {
      view.unmount();
      queryClient.clear();
      vi.useRealTimers();
    }
  });

  it("Should replace authorized scopes and ignore frames from the disposed connection", () => {
    vi.useFakeTimers();
    const sources: FakeCatalogEventSource[] = [];
    const factory = () => {
      const source = new FakeCatalogEventSource();
      sources.push(source);
      return source;
    };
    const client = new QueryClient();
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    );
    const view = renderHook(
      ({ workspaces }) => useWorktreeCatalogStream(workspaces, { eventSourceFactory: factory }),
      { wrapper, initialProps: { workspaces: [{ id: "a" }] } }
    );
    try {
      const alpha = workspaceKeys.worktrees("a");
      const beta = workspaceKeys.worktrees("b");
      client.setQueryData(alpha, {});
      client.setQueryData(beta, {});
      act(() =>
        sources[0].emit("worktree_catalog_changed", {
          kind: "updated",
          workspace_id: "a",
          worktree_id: "wt",
        })
      );
      view.rerender({ workspaces: [{ id: "b" }] });
      expect(client.getQueryState(alpha)?.isInvalidated).toBe(true);
      expect(sources[0].closed).toBe(true);
      client.setQueryData(alpha, {});
      act(() => {
        sources[0].emit("worktree_catalog_changed", {
          kind: "updated",
          workspace_id: "a",
          worktree_id: "wt",
        });
        sources[1].emit("worktree_catalog_changed", {
          kind: "updated",
          workspace_id: "a",
          worktree_id: "wt",
        });
        sources[1].emit("worktree_catalog_changed", {
          kind: "updated",
          workspace_id: "b",
          worktree_id: "wt",
        });
        vi.advanceTimersByTime(WORKTREE_REREAD_COALESCE_MS);
      });
      expect(client.getQueryState(alpha)?.isInvalidated).toBe(false);
      expect(client.getQueryState(beta)?.isInvalidated).toBe(true);
    } finally {
      view.unmount();
      client.clear();
      vi.useRealTimers();
    }
  });

  // Invariant: a catalog burst reconciles every affected live read to the latest generation.
  // Owner: catalog-stream cache boundary; measured through real Query observers.
  it("Should reconcile a burst to the latest catalog generation with bounded rereads", async () => {
    vi.useFakeTimers();
    const { source, queryClient, view } = setup([{ id: "ws_alpha" }]);
    let generation = 0;
    const reads = [0, 0, 0];
    const keys = [
      workspaceKeys.worktrees("ws_alpha"),
      workspaceKeys.worktreeAggregate([]),
      workspaceKeys.worktreeDetail("ws_alpha", "wt_one"),
    ];
    const subscriptions = keys.map((queryKey, index) => {
      queryClient.setQueryData(queryKey, { generation });
      return new QueryObserver(queryClient, {
        queryKey,
        staleTime: Infinity,
        queryFn: async ({ signal }) => {
          void signal.aborted;
          reads[index] += 1;
          return { generation };
        },
      }).subscribe(() => {});
    });
    try {
      await act(async () => {
        for (generation = 1; generation <= 100; generation += 1) {
          source.emit("worktree_catalog_changed", {
            kind: "updated",
            workspace_id: "ws_alpha",
            worktree_id: "wt_one",
          });
        }
        generation = 100;
        await vi.advanceTimersByTimeAsync(1000);
      });
      for (const key of keys) expect(queryClient.getQueryData(key)).toEqual({ generation: 100 });
      expect(reads).toEqual([1, 1, 1]);
      console.info("worktree catalog burst requests", JSON.stringify(reads));
    } finally {
      subscriptions.forEach(unsubscribe => unsubscribe());
      view.unmount();
      queryClient.clear();
      vi.useRealTimers();
    }
  });

  it.each([useWorktrees, () => useWorktreeListings([{ id: "a" }])])(
    "Should recover gone-workspace reads through catalog observations (%s)",
    async useListing => {
      const fetch = vi
        .spyOn(worktreeApi, "fetchWorktrees")
        .mockRejectedValue(
          new worktreeApi.WorktreeApiError("Missing root", 410, { code: "workspace_root_missing" })
        );
      const source = new FakeCatalogEventSource();
      const factory = () => source;
      const client = new QueryClient({ defaultOptions: { queries: { retry: 1, retryDelay: 0 } } });
      const wrapper = ({ children }: { children: ReactNode }) => (
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
      );
      client.setQueryData(workspaceKeys.list(), [{ id: "a", root_missing: false }]);
      const view = renderHook(
        () => {
          useWorktreeCatalogStream([{ id: "a" }], { eventSourceFactory: factory });
          return useListing("a");
        },
        { wrapper }
      );
      try {
        await waitFor(() =>
          expect(client.getQueryState(workspaceKeys.worktrees("a"))?.status).toBe("error")
        );
        await act(async () => {
          source.emit("open");
          source.emit("worktree_catalog_changed", {
            kind: "updated",
            workspace_id: "a",
            worktree_id: "wt",
          });
          await client.invalidateQueries({ queryKey: workspaceKeys.worktrees("a") });
        });
        expect(fetch).toHaveBeenCalledTimes(1);
        const observedAt = client.getQueryState(workspaceKeys.worktrees("a"))!.errorUpdatedAt + 1;
        await act(async () => {
          client.setQueryData(workspaceKeys.list(), [{ id: "a", root_missing: true }], {
            updatedAt: observedAt,
          });
        });
        expect(fetch).toHaveBeenCalledTimes(1);
        fetch.mockResolvedValue({
          worktrees: [],
          discovered: [],
          repo: { git_available: true, git_backed: true },
        });
        await act(async () => {
          client.setQueryData(workspaceKeys.list(), [{ id: "a", root_missing: false }], {
            updatedAt: observedAt + 1,
          });
        });
        await waitFor(() =>
          expect(client.getQueryState(workspaceKeys.worktrees("a"))?.status).toBe("success")
        );
        expect(fetch).toHaveBeenCalledTimes(2);
      } finally {
        view.unmount();
        client.clear();
        fetch.mockRestore();
      }
    }
  );

  it("Should keep the connection when the same workspace population is reordered", () => {
    const source = new FakeCatalogEventSource();
    const factory = vi.fn(() => source);
    const client = new QueryClient();
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    );
    const view = renderHook(
      ({ workspaces }) => useWorktreeCatalogStream(workspaces, { eventSourceFactory: factory }),
      {
        wrapper,
        initialProps: { workspaces: [{ id: "a" }, { id: "b" }] },
      }
    );
    view.rerender({ workspaces: [{ id: "b" }, { id: "a" }] });
    expect(factory).toHaveBeenCalledTimes(1);
    expect(source.closed).toBe(false);
    view.unmount();
    expect(source.closed).toBe(true);
  });

  it("Should register named listeners rather than relying on onmessage", () => {
    const { source } = setup([{ id: "ws_alpha" }]);

    // A bare onmessage handler would silently miss every named frame.
    expect(source.listeners.has("worktree_catalog_changed")).toBe(true);
    expect(source.listeners.has("open")).toBe(true);
    expect(source.listeners.has("error")).toBe(true);
  });

  it("Should invalidate the workspace-qualified key named by the frame", async () => {
    const { source, invalidate, queryClient } = setup([{ id: "ws_alpha" }, { id: "ws_beta" }]);
    invalidate.mockClear();
    queryClient.setQueryData(workspaceKeys.worktreeDetail("ws_beta", "wt_one"), {});
    queryClient.setQueryData(workspaceKeys.worktreeDetail("ws_alpha", "wt_other"), {});

    act(() => {
      source.emit("worktree_catalog_changed", {
        kind: "upserted",
        workspace_id: "ws_beta",
        worktree_id: "wt_one",
      });
    });

    await waitFor(() =>
      expect(invalidate).toHaveBeenCalledWith({ queryKey: workspaceKeys.worktrees("ws_beta") })
    );
    expect(
      queryClient.getQueryState(workspaceKeys.worktreeDetail("ws_beta", "wt_one"))?.isInvalidated
    ).toBe(true);
    expect(
      queryClient.getQueryState(workspaceKeys.worktreeDetail("ws_alpha", "wt_other"))?.isInvalidated
    ).toBe(false);
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

  it("Should reread every authorized workspace on reconnect", async () => {
    const { source, invalidate } = setup([{ id: "ws_alpha" }, { id: "ws_beta" }]);
    invalidate.mockClear();

    // A reconnect may have missed frames, so each workspace rereads.
    act(() => source.emit("open"));

    await waitFor(() =>
      expect(invalidate).toHaveBeenCalledWith({ queryKey: workspaceKeys.worktrees("ws_alpha") })
    );
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
