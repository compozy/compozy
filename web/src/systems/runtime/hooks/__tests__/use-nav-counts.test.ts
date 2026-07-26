import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => ({ activeWorkspaceId: "ws_alpha" }),
}));

import {
  type NavCountFetchers,
  type NavCountKey,
  type NavCountsEventSource,
  TASK_NAV_COUNT_EVENT_TYPES,
  createNavCountsStore,
} from "../nav-counts-store";
import { selectNavCount, useNavCounts } from "../use-nav-counts";

class FakeEventSource implements NavCountsEventSource {
  public close = vi.fn();
  public onopen: ((event: Event) => void) | null = null;
  public onerror: ((event: Event) => void) | null = null;
  private readonly listeners = new Map<string, Array<(event: MessageEvent) => void>>();

  addEventListener(type: string, listener: (event: MessageEvent) => void): void {
    const current = this.listeners.get(type) ?? [];
    current.push(listener);
    this.listeners.set(type, current);
  }

  removeEventListener(type: string, listener: (event: MessageEvent) => void): void {
    const current = this.listeners.get(type) ?? [];
    this.listeners.set(
      type,
      current.filter(candidate => candidate !== listener)
    );
  }

  emit(type: string, options: { lastEventId?: string } = {}): void {
    const event = new MessageEvent(type, { lastEventId: options.lastEventId ?? "" });
    for (const listener of this.listeners.get(type) ?? []) {
      listener(event);
    }
  }

  emitError(): void {
    if (this.onerror) this.onerror(new Event("error"));
  }

  emitOpen(): void {
    if (this.onopen) this.onopen(new Event("open"));
  }
}

interface FetcherSpec {
  count?: number;
  delayMs?: number;
  shouldReject?: boolean;
}

function createFakeFetchers(spec: Partial<Record<NavCountKey, FetcherSpec>> = {}): {
  fetchers: NavCountFetchers;
  callCounts: Record<NavCountKey, number>;
} {
  const callCounts: Record<NavCountKey, number> = {
    tasks: 0,
    jobs: 0,
    network: 0,
    triggers: 0,
    agents: 0,
    knowledge: 0,
    skills: 0,
    bridges: 0,
  };
  const fetchers: Partial<NavCountFetchers> = {};
  for (const key of Object.keys(callCounts) as NavCountKey[]) {
    const cfg = spec[key] ?? {};
    const baseCount = cfg.count ?? 1;
    fetchers[key] = async signal => {
      callCounts[key] += 1;
      if (cfg.delayMs && cfg.delayMs > 0) {
        await new Promise<void>((resolve, reject) => {
          const handle = setTimeout(() => {
            signal.removeEventListener("abort", onAbort);
            resolve();
          }, cfg.delayMs);
          const onAbort = () => {
            clearTimeout(handle);
            reject(new DOMException("Aborted", "AbortError"));
          };
          signal.addEventListener("abort", onAbort);
        });
      }
      if (cfg.shouldReject) {
        throw new Error(`fetcher ${key} forced rejection`);
      }
      return { count: baseCount };
    };
  }
  return { fetchers: fetchers as NavCountFetchers, callCounts };
}

interface StoreHarnessOverrides {
  fetcherSpec?: Partial<Record<NavCountKey, FetcherSpec>>;
  pollIntervalMs?: number;
  heartbeatWindowMs?: number;
  staleCheckIntervalMs?: number;
}

function createStoreHarness(overrides: StoreHarnessOverrides = {}) {
  const eventSource = new FakeEventSource();
  const { fetchers, callCounts } = createFakeFetchers(overrides.fetcherSpec);
  const store = createNavCountsStore({
    eventSourceFactory: () => eventSource,
    observeStreamUrl: "/api/logs/stream?workspace_id=ws_alpha",
    fetchers,
    pollIntervalMs: overrides.pollIntervalMs ?? 5_000,
    heartbeatWindowMs: overrides.heartbeatWindowMs ?? 5_000,
    staleCheckIntervalMs: overrides.staleCheckIntervalMs ?? 1_000,
  });
  return { store, eventSource, callCounts };
}

describe("useNavCounts contract", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("Should expose the typed { counts, refresh, status } contract", () => {
    const { store } = createStoreHarness();
    const { result, unmount } = renderHook(() => useNavCounts({ store }));
    expect(Object.keys(result.current).sort()).toEqual(["counts", "refresh", "status"]);
    expect(typeof result.current.refresh).toBe("function");
    expect(result.current.status).toBe("loading");
    unmount();
  });

  it("Should hydrate counts from the initial snapshot pass", async () => {
    const { store, callCounts } = createStoreHarness({
      fetcherSpec: {
        tasks: { count: 12 },
        jobs: { count: 4 },
        network: { count: 3 },
        triggers: { count: 7 },
        agents: { count: 5 },
        knowledge: { count: 8 },
        skills: { count: 9 },
        bridges: { count: 2 },
      },
    });
    const { result, unmount } = renderHook(() => useNavCounts({ store }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(result.current.status).toBe("ready");
    expect(result.current.counts.tasks).toEqual({ count: 12, stale: false });
    expect(result.current.counts.jobs).toEqual({ count: 4, stale: false });
    expect(result.current.counts.network).toEqual({ count: 3, stale: false });
    expect(result.current.counts.triggers).toEqual({ count: 7, stale: false });
    expect(result.current.counts.agents).toEqual({ count: 5, stale: false });
    expect(result.current.counts.knowledge).toEqual({ count: 8, stale: false });
    expect(result.current.counts.skills).toEqual({ count: 9, stale: false });
    expect(result.current.counts.bridges).toEqual({ count: 2, stale: false });
    expect(callCounts.tasks).toBe(1);
    expect(callCounts.jobs).toBe(1);
    unmount();
  });

  it("Should increment tasks count when an SSE task.run_enqueued event arrives", async () => {
    const { store, eventSource } = createStoreHarness({
      fetcherSpec: { tasks: { count: 3 } },
    });
    const { result, unmount } = renderHook(() => useNavCounts({ store }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(result.current.counts.tasks?.count).toBe(3);
    act(() => {
      eventSource.emit("task.run_enqueued", { lastEventId: "2026-05-11T12:00:00Z|0001" });
    });
    expect(result.current.counts.tasks?.count).toBe(4);
    unmount();
  });

  it("Should decrement tasks count when an SSE task.canceled event arrives", async () => {
    const { store, eventSource } = createStoreHarness({
      fetcherSpec: { tasks: { count: 5 } },
    });
    const { result, unmount } = renderHook(() => useNavCounts({ store }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    act(() => {
      eventSource.emit("task.canceled", { lastEventId: "2026-05-11T12:00:01Z|0002" });
    });
    expect(result.current.counts.tasks?.count).toBe(4);
    unmount();
  });

  it("Should drop SSE events whose lastEventId is not greater than the cursor", async () => {
    const { store, eventSource } = createStoreHarness({
      fetcherSpec: { tasks: { count: 0 } },
    });
    const { result, unmount } = renderHook(() => useNavCounts({ store }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    act(() => {
      eventSource.emit("task.run_enqueued", { lastEventId: "cursor-002" });
    });
    expect(result.current.counts.tasks?.count).toBe(1);
    act(() => {
      eventSource.emit("task.run_enqueued", { lastEventId: "cursor-001" });
    });
    expect(result.current.counts.tasks?.count).toBe(1);
    unmount();
  });

  it("Should re-poll non-task keys on every poll-interval tick", async () => {
    const { store, callCounts } = createStoreHarness({
      fetcherSpec: { jobs: { count: 1 } },
      pollIntervalMs: 5_000,
    });
    const { unmount } = renderHook(() => useNavCounts({ store }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(callCounts.jobs).toBe(1);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(5_000);
    });
    expect(callCounts.jobs).toBe(2);
    expect(callCounts.tasks).toBe(1);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(5_000);
    });
    expect(callCounts.jobs).toBe(3);
    unmount();
  });

  it("Should flip tasks.stale=true when SSE drops and the heartbeat window elapses", async () => {
    const { store, eventSource } = createStoreHarness({
      fetcherSpec: { tasks: { count: 1 } },
      heartbeatWindowMs: 5_000,
      staleCheckIntervalMs: 1_000,
    });
    const { result, unmount } = renderHook(() => useNavCounts({ store }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(result.current.counts.tasks?.stale).toBe(false);
    act(() => {
      eventSource.emitOpen();
      eventSource.emitError();
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(6_000);
    });
    expect(result.current.counts.tasks?.stale).toBe(true);
    unmount();
  });

  it("Should clear tasks.stale when an SSE event arrives after a drop", async () => {
    const { store, eventSource } = createStoreHarness({
      fetcherSpec: { tasks: { count: 0 } },
      heartbeatWindowMs: 5_000,
      staleCheckIntervalMs: 1_000,
    });
    const { result, unmount } = renderHook(() => useNavCounts({ store }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    act(() => {
      eventSource.emitOpen();
      eventSource.emitError();
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(6_000);
    });
    expect(result.current.counts.tasks?.stale).toBe(true);
    act(() => {
      eventSource.emit("task.run_enqueued", { lastEventId: "cursor-100" });
    });
    expect(result.current.counts.tasks?.stale).toBe(false);
    expect(result.current.counts.tasks?.count).toBe(1);
    unmount();
  });

  it("Should clear tasks.stale when EventSource reconnects without a business event", async () => {
    const { store, eventSource } = createStoreHarness({
      fetcherSpec: { tasks: { count: 7 } },
      heartbeatWindowMs: 5_000,
      staleCheckIntervalMs: 1_000,
    });
    const { result, unmount } = renderHook(() => useNavCounts({ store }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    act(() => {
      eventSource.emitOpen();
      eventSource.emitError();
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(6_000);
    });
    expect(result.current.counts.tasks).toEqual({ count: 7, stale: true });

    act(() => {
      eventSource.emitOpen();
    });

    expect(result.current.counts.tasks).toEqual({ count: 7, stale: false });
    unmount();
  });

  it("Should flip non-task keys to stale when polls fail for longer than 2 × pollInterval", async () => {
    const eventSource = new FakeEventSource();
    let jobsCallIndex = 0;
    const fetchers: NavCountFetchers = {
      tasks: async () => ({ count: 0 }),
      jobs: async () => {
        jobsCallIndex += 1;
        if (jobsCallIndex === 1) return { count: 4 };
        throw new Error("jobs poll failed");
      },
      network: async () => ({ count: 0 }),
      triggers: async () => ({ count: 0 }),
      agents: async () => ({ count: 0 }),
      knowledge: async () => ({ count: 0 }),
      skills: async () => ({ count: 0 }),
      bridges: async () => ({ count: 0 }),
    };
    const store = createNavCountsStore({
      eventSourceFactory: () => eventSource,
      fetchers,
      pollIntervalMs: 5_000,
      heartbeatWindowMs: 5_000,
      staleCheckIntervalMs: 1_000,
    });
    const { result, unmount } = renderHook(() => useNavCounts({ store }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(result.current.counts.jobs).toEqual({ count: 4, stale: false });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(12_000);
    });
    expect(result.current.counts.jobs?.stale).toBe(true);
    expect(result.current.counts.jobs?.count).toBe(4);
    unmount();
  });

  it("Should keep the higher-seq SSE delta when a polling snapshot resolves later (N-009)", async () => {
    const { store, eventSource } = createStoreHarness({
      fetcherSpec: {
        tasks: { count: 9, delayMs: 50 },
      },
    });
    const { result, unmount } = renderHook(() => useNavCounts({ store }));
    // The dashboard fetch is in flight (delayed 50 ms). The SSE event arrives
    // first — store reflects the delta. The stale snapshot must NOT overwrite.
    act(() => {
      eventSource.emit("task.run_enqueued", { lastEventId: "cursor-200" });
    });
    expect(result.current.counts.tasks?.count).toBe(1);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(60);
    });
    expect(result.current.counts.tasks?.count).toBe(1);
    expect(result.current.counts.tasks?.stale).toBe(false);
    unmount();
  });

  it("Should flip status to loading when refresh() runs and back to ready when it resolves", async () => {
    const { store } = createStoreHarness({
      fetcherSpec: { tasks: { count: 2 }, jobs: { count: 4 } },
    });
    const { result, unmount } = renderHook(() => useNavCounts({ store }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(result.current.status).toBe("ready");
    act(() => {
      result.current.refresh();
    });
    expect(result.current.status).toBe("loading");
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(result.current.status).toBe("ready");
    unmount();
  });

  it("Should flip status to error when a snapshot fetcher rejects", async () => {
    const { store } = createStoreHarness({
      fetcherSpec: { tasks: { count: 0, shouldReject: true } },
    });
    const { result, unmount } = renderHook(() => useNavCounts({ store }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(result.current.status).toBe("error");
    unmount();
  });

  it("Should close the EventSource and clear polling timers when the last consumer unmounts", async () => {
    const { store, eventSource } = createStoreHarness({
      fetcherSpec: { tasks: { count: 0 } },
    });
    const clearInterval = vi.spyOn(globalThis, "clearInterval");
    const { unmount } = renderHook(() => useNavCounts({ store }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    unmount();
    expect(eventSource.close).toHaveBeenCalledTimes(1);
    expect(clearInterval).toHaveBeenCalled();
    clearInterval.mockRestore();
  });

  it("Should share a single SSE channel across two mounted consumers", async () => {
    const eventSource = new FakeEventSource();
    const factory = vi.fn(() => eventSource);
    const { fetchers } = createFakeFetchers({ tasks: { count: 1 } });
    const store = createNavCountsStore({
      eventSourceFactory: factory,
      observeStreamUrl: "/api/logs/stream?workspace_id=ws_alpha",
      fetchers,
      pollIntervalMs: 5_000,
      heartbeatWindowMs: 5_000,
      staleCheckIntervalMs: 1_000,
    });
    const first = renderHook(() => useNavCounts({ store }));
    const second = renderHook(() => useNavCounts({ store }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(factory).toHaveBeenCalledTimes(1);
    expect(first.result.current.counts.tasks?.count).toBe(1);
    expect(second.result.current.counts.tasks?.count).toBe(1);
    first.unmount();
    expect(eventSource.close).not.toHaveBeenCalled();
    second.unmount();
    expect(eventSource.close).toHaveBeenCalledTimes(1);
  });

  it("Should cover every TASK_NAV_COUNT_EVENT_TYPES entry on the event source", async () => {
    const { store, eventSource } = createStoreHarness();
    const { unmount } = renderHook(() => useNavCounts({ store }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    // Internal listener registration is verified by emitting each event and
    // confirming the store handles it without throwing.
    act(() => {
      let seq = 0;
      for (const type of TASK_NAV_COUNT_EVENT_TYPES) {
        seq += 1;
        eventSource.emit(type, { lastEventId: `cursor-${String(seq).padStart(4, "0")}` });
      }
    });
    unmount();
  });
});

describe("selectNavCount", () => {
  it("Should return undefined when the key is undefined", () => {
    const result = {
      counts: { tasks: { count: 3, stale: false } },
      refresh: () => undefined,
      status: "ready" as const,
    };
    expect(selectNavCount(result, undefined)).toBeUndefined();
  });

  it("Should look up the count entry by key", () => {
    const result = {
      counts: { tasks: { count: 3, stale: false } },
      refresh: () => undefined,
      status: "ready" as const,
    };
    expect(selectNavCount(result, "tasks")).toEqual({ count: 3, stale: false });
  });

  it("Should return undefined when the key is absent from the counts map", () => {
    const result = {
      counts: { tasks: { count: 3, stale: false } },
      refresh: () => undefined,
      status: "ready" as const,
    };
    expect(selectNavCount(result, "jobs")).toBeUndefined();
  });
});
