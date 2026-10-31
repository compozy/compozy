// Suite: per-worktree stream consumption
// Invariant: lifecycle frames reread the worktree scope (list, detail, status, exit) once per
// quiet window rather than once per frame, so a replayed history or a burst of frames cannot
// multiply the same four reads or cancel its own in-flight fetches; a pending reread is paid when
// the connection goes away. Owning layer: workspace stream hook. Canonical suite: this hook test.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { workspaceKeys } from "../../lib/query-keys";
import {
  useWorktreeStream,
  WORKTREE_REREAD_COALESCE_MS,
  type WorktreeEventSource,
} from "../use-worktree-stream";

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
  return { source, invalidate, view };
}

describe("useWorktreeStream", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
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
      queryKey: workspaceKeys.worktreeExit(WORKSPACE_ID, WORKTREE_ID),
      exact: true,
    });
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
