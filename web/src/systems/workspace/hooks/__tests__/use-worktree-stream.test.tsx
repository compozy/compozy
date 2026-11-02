// Suite: per-worktree stream consumption
// Invariant: lifecycle frames reread the worktree scope (list, detail, status, exit) once per
// quiet window rather than once per frame, so a replayed history or a burst of frames cannot
// multiply the same four reads or cancel its own in-flight fetches; a pending reread is paid when
// the connection goes away. Owning layer: workspace stream hook. Canonical suite: this hook test.
import { QueryClient, QueryClientProvider, QueryObserver } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { workspaceKeys } from "../../lib/query-keys";
import {
  WORKTREE_REREAD_COALESCE_MS,
  WORKTREE_REREAD_MAX_WAIT_MS,
} from "../../lib/worktree-reread";
import { useWorktreeStream, type WorktreeEventSource } from "../use-worktree-stream";

const WORKSPACE_ID = "ws-a";
const WORKTREE_ID = "wt-a";
/** The four worktree-qualified reads one scope reread invalidates. */
const SCOPE_READS = 4;

class FakeWorktreeEventSource implements WorktreeEventSource {
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
}

function statusFrame(sequence: number) {
  return { worktree_id: WORKTREE_ID, workspace_id: WORKSPACE_ID, sequence };
}

function setup() {
  const source = new FakeWorktreeEventSource();
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const invalidate = vi.spyOn(queryClient, "invalidateQueries");
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  // One factory identity across rerenders: the stream effect keys on it, and a
  // fresh function per render would reconnect on every status change.
  const eventSourceFactory = () => source;
  const view = renderHook(
    () => useWorktreeStream(WORKSPACE_ID, WORKTREE_ID, { eventSourceFactory }),
    { wrapper }
  );
  return { source, invalidate, view, queryClient };
}

describe("useWorktreeStream", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  // Invariant: one scope wake fetches each owned read once and leaves sibling details alone.
  it("Should reconcile each live query once without refreshing sibling worktrees", async () => {
    const { source, queryClient, view } = setup();
    const reads = [0, 0, 0, 0, 0];
    const keys = [
      workspaceKeys.worktrees(WORKSPACE_ID),
      workspaceKeys.worktreeDetail(WORKSPACE_ID, WORKTREE_ID),
      workspaceKeys.worktreeStatus(WORKSPACE_ID, WORKTREE_ID),
      workspaceKeys.worktreeExit(WORKSPACE_ID, WORKTREE_ID),
      workspaceKeys.worktreeDetail(WORKSPACE_ID, "other"),
    ];
    let generation = 0;
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
        generation = 1;
        source.emit("worktree.status_refreshed", statusFrame(1));
        await vi.advanceTimersByTimeAsync(WORKTREE_REREAD_COALESCE_MS);
      });
      for (const key of keys.slice(0, 4))
        expect(queryClient.getQueryData(key)).toEqual({ generation: 1 });
      expect(queryClient.getQueryData(keys[4])).toEqual({ generation: 0 });
      expect(reads).toEqual([1, 1, 1, 1, 0]);
      console.info("worktree scope wake requests", JSON.stringify(reads));
    } finally {
      subscriptions.forEach(unsubscribe => unsubscribe());
      view.unmount();
      queryClient.clear();
    }
  });

  it("Should reread the worktree scope once for a replayed burst of status frames", () => {
    const { source, invalidate } = setup();

    act(() => {
      source.emit("open");
      for (let sequence = 1; sequence <= 500; sequence += 1) {
        source.emit("worktree.status_refreshed", statusFrame(sequence));
      }
    });
    expect(invalidate).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(WORKTREE_REREAD_COALESCE_MS);
    });
    expect(invalidate).toHaveBeenCalledTimes(SCOPE_READS);
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: workspaceKeys.worktrees(WORKSPACE_ID),
      exact: true,
    });
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: workspaceKeys.worktreeDetail(WORKSPACE_ID, WORKTREE_ID),
      exact: true,
    });
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: workspaceKeys.worktreeStatus(WORKSPACE_ID, WORKTREE_ID),
      exact: true,
    });
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: workspaceKeys.worktreeExit(WORKSPACE_ID, WORKTREE_ID),
      exact: true,
    });
  });

  it("Should reread the worktree scope once when the stream opens", () => {
    const { source, invalidate } = setup();

    act(() => {
      source.emit("open");
    });
    expect(invalidate).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(WORKTREE_REREAD_COALESCE_MS);
    });
    expect(invalidate).toHaveBeenCalledTimes(SCOPE_READS);
  });

  it("Should reread once per ceiling while frames keep landing inside the quiet window", () => {
    const { source, invalidate } = setup();
    const spacingMs = WORKTREE_REREAD_COALESCE_MS - 50;
    // Two ceiling periods: a ceiling that fires once and never re-arms rereads only once.
    const ceilingPeriods = 2;
    const frames = Math.ceil((WORKTREE_REREAD_MAX_WAIT_MS * ceilingPeriods) / spacingMs);

    act(() => {
      for (let sequence = 1; sequence <= frames; sequence += 1) {
        source.emit("worktree.status_refreshed", statusFrame(sequence));
        vi.advanceTimersByTime(spacingMs);
      }
    });
    expect(invalidate).toHaveBeenCalledTimes(SCOPE_READS * ceilingPeriods);

    act(() => {
      vi.advanceTimersByTime(WORKTREE_REREAD_COALESCE_MS);
    });
    expect(invalidate).toHaveBeenCalledTimes(SCOPE_READS * ceilingPeriods);
  });

  it("Should reread again for a frame that arrives after the quiet window", () => {
    const { source, invalidate } = setup();

    act(() => {
      source.emit("worktree.status_refreshed", statusFrame(1));
      vi.advanceTimersByTime(WORKTREE_REREAD_COALESCE_MS);
    });
    expect(invalidate).toHaveBeenCalledTimes(SCOPE_READS);

    act(() => {
      source.emit("worktree.exit_action_completed", { op_id: "op-1", action: "push" });
      vi.advanceTimersByTime(WORKTREE_REREAD_COALESCE_MS);
    });
    expect(invalidate).toHaveBeenCalledTimes(SCOPE_READS * 2);
  });

  it("Should pay a pending reread once when the stream disconnects", () => {
    const { source, invalidate, view } = setup();

    act(() => {
      source.emit("worktree.status_refreshed", statusFrame(1));
    });
    expect(invalidate).not.toHaveBeenCalled();
    view.unmount();
    expect(invalidate).toHaveBeenCalledTimes(SCOPE_READS);

    act(() => {
      vi.advanceTimersByTime(WORKTREE_REREAD_COALESCE_MS * 2);
    });
    expect(invalidate).toHaveBeenCalledTimes(SCOPE_READS);
    expect(source.closed).toBe(true);
  });
});
