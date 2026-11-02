import { QueryClient, QueryClientProvider, QueryObserver } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it } from "vitest";

import type { ProfileScopeParams } from "@/systems/profiles";
import { tasksKeys } from "@/systems/tasks";

import { makeHomeOverview } from "../mocks/fixtures";
import { useHomeLive, type HomeLiveEventSource } from "../hooks/use-home-live";
import { homeActivityOptions, homeOverviewOptions } from "../lib/query-options";
import type { HomeActivityEvent } from "../types";

// Every read the home surface makes states its lens, so the stream states one
// too — there is no unscoped home activity.
const SCOPED: ProfileScopeParams = { profile: "default" };
const AGGREGATE: ProfileScopeParams = { all_profiles: true };

class FakeHomeEventSource implements HomeLiveEventSource {
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  closed = false;
  constructor(readonly url: string) {}

  close() {
    this.closed = true;
  }

  emit(payload: unknown) {
    this.onmessage?.(new MessageEvent("message", { data: JSON.stringify(payload) }));
  }
}

function activityEvent(id: string, type: string): HomeActivityEvent {
  return {
    id,
    type,
    agent_name: "coder",
    session_id: "sess-1",
    spawn_depth: 0,
    timestamp: "2026-07-23T12:00:00Z",
  } as HomeActivityEvent;
}

function harness() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const sources: FakeHomeEventSource[] = [];
  const factory = (url: string) => {
    const source = new FakeHomeEventSource(url);
    sources.push(source);
    return source;
  };
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return { queryClient, sources, factory, wrapper };
}

function observeAggregateReads(queryClient: QueryClient, revision: () => number, seeded = true) {
  const keys = [
    homeOverviewOptions({ profile: "default" }).queryKey,
    tasksKeys.dashboardRoot(),
    tasksKeys.inboxRoot(),
  ];
  const pending: ((error?: Error) => void)[] = [];
  let requests = 0;
  let cancellations = 0;
  const observers = keys.map(queryKey => {
    const observer = new QueryObserver(queryClient, {
      queryKey,
      initialData: seeded ? 0 : undefined,
      staleTime: Infinity,
      queryFn: ({ signal }) => {
        requests += 1;
        const captured = revision();
        signal.addEventListener("abort", () => (cancellations += 1), { once: true });
        return new Promise<number>((resolve, reject) => {
          pending.push(error => (error ? reject(error) : resolve(captured)));
        });
      },
    });
    return { observer, unsubscribe: observer.subscribe(() => {}) };
  });
  return {
    requests: () => requests,
    cancellations: () => cancellations,
    values: () => keys.map(key => queryClient.getQueryData(key)),
    invalidated: () => keys.map(key => queryClient.getQueryState(key)?.isInvalidated),
    setEnabled: (enabled: boolean) => {
      observers.forEach(({ observer }) => observer.setOptions({ ...observer.options, enabled }));
    },
    release: (error?: Error) => pending.splice(0).forEach(settle => settle(error)),
    close: () => {
      observers.forEach(({ unsubscribe }) => unsubscribe());
      queryClient.clear();
    },
  };
}

describe("useHomeLive", () => {
  it("Should prepend stream events into the activity cache without duplicates", () => {
    const { queryClient, sources, factory, wrapper } = harness();
    const key = homeActivityOptions(SCOPED).queryKey;
    queryClient.setQueryData<HomeActivityEvent[]>(key, [activityEvent("evt-1", "message")]);

    renderHook(() => useHomeLive({ scope: SCOPED, eventSourceFactory: factory }), { wrapper });
    expect(sources).toHaveLength(1);
    expect(sources[0]?.url).toBe("/api/logs/stream?profile=default");

    act(() => {
      sources[0]?.emit(activityEvent("evt-2", "message"));
      sources[0]?.emit(activityEvent("evt-2", "message"));
    });

    const cached = queryClient.getQueryData<HomeActivityEvent[]>(key);
    expect(cached?.map(event => event.id)).toEqual(["evt-2", "evt-1"]);
  });

  it("Should refresh task aggregates immediately on task lifecycle events", () => {
    const { queryClient, sources, factory, wrapper } = harness();
    const overviewKey = homeOverviewOptions({ profile: "default" }).queryKey;
    queryClient.setQueryData(overviewKey, makeHomeOverview());
    queryClient.setQueryData(tasksKeys.dashboardRoot(), { seeded: true });
    queryClient.setQueryData(tasksKeys.inboxRoot(), { seeded: true });

    renderHook(() => useHomeLive({ scope: SCOPED, eventSourceFactory: factory }), { wrapper });
    act(() => {
      sources[0]?.emit(activityEvent("evt-task", "task.run_completed"));
    });

    expect(queryClient.getQueryState(overviewKey)?.isInvalidated).toBe(true);
    expect(queryClient.getQueryState(tasksKeys.dashboardRoot())?.isInvalidated).toBe(true);
    expect(queryClient.getQueryState(tasksKeys.inboxRoot())?.isInvalidated).toBe(true);
  });

  it("Should coalesce in-flight lifecycle changes and preserve the latest aggregate data", async () => {
    const { queryClient, sources, factory, wrapper } = harness();
    let revision = 0;
    const reads = observeAggregateReads(queryClient, () => revision);
    const { unmount } = renderHook(
      () => useHomeLive({ scope: SCOPED, eventSourceFactory: factory }),
      { wrapper }
    );

    act(() => {
      for (revision = 1; revision <= 100; revision += 1) {
        sources[0]?.emit(activityEvent(`evt-${revision}`, "task.run_completed"));
      }
      revision = 100;
    });
    expect(reads.requests()).toBe(3);
    expect(reads.cancellations()).toBe(0);

    await act(async () => reads.release());
    expect(reads.requests()).toBe(6);
    expect(reads.cancellations()).toBe(0);
    await act(async () => reads.release());
    expect(reads.values()).toEqual([100, 100, 100]);
    expect(reads.requests()).toBe(6);

    unmount();
    reads.close();
  });

  it("Should preserve a trailing refresh after the leading request fails", async () => {
    const { queryClient, sources, factory, wrapper } = harness();
    let revision = 1;
    const reads = observeAggregateReads(queryClient, () => revision);
    const { unmount } = renderHook(
      () => useHomeLive({ scope: SCOPED, eventSourceFactory: factory }),
      { wrapper }
    );
    act(() => {
      sources[0]?.emit(activityEvent("evt-first", "task.run_started"));
      revision = 2;
      sources[0]?.emit(activityEvent("evt-final", "task.run_completed"));
    });

    await act(async () => reads.release(new Error("transient network failure")));
    expect(reads.requests()).toBe(6);
    expect(reads.cancellations()).toBe(0);
    await act(async () => reads.release());
    expect(reads.values()).toEqual([2, 2, 2]);

    unmount();
    reads.close();
  });

  it("Should reconcile queued lifecycle changes after the closing stream read settles", async () => {
    const { queryClient, sources, factory, wrapper } = harness();
    let revision = 1;
    const reads = observeAggregateReads(queryClient, () => revision);
    const { unmount } = renderHook(
      () => useHomeLive({ scope: SCOPED, eventSourceFactory: factory }),
      { wrapper }
    );
    act(() => {
      sources[0]?.emit(activityEvent("evt-first", "task.run_started"));
      revision = 2;
      sources[0]?.emit(activityEvent("evt-final", "task.run_completed"));
    });
    expect(reads.requests()).toBe(3);

    unmount();
    expect(reads.requests()).toBe(3);
    await act(async () => reads.release());
    expect(reads.requests()).toBe(6);
    expect(reads.cancellations()).toBe(0);
    await act(async () => reads.release());
    expect(reads.values()).toEqual([2, 2, 2]);
    expect(reads.requests()).toBe(6);
    reads.close();
  });

  it.each(["disabled", "initially empty"] as const)(
    "Should preserve pending lifecycle changes when a %s observer outlives the stream",
    async mode => {
      const { queryClient, sources, factory, wrapper } = harness();
      let revision = 0;
      const reads = observeAggregateReads(queryClient, () => revision, mode !== "initially empty");
      const { unmount } = renderHook(
        () => useHomeLive({ scope: SCOPED, eventSourceFactory: factory }),
        { wrapper }
      );
      act(() => {
        revision = 1;
        sources[0]?.emit(activityEvent("evt-first", "task.run_started"));
        revision = 2;
        sources[0]?.emit(activityEvent("evt-final", "task.run_completed"));
        if (mode === "disabled") reads.setEnabled(false);
      });
      expect(reads.requests()).toBe(3);
      unmount();
      await act(async () => reads.release());
      if (mode === "disabled") {
        expect(reads.values()).toEqual([1, 1, 1]);
        expect(reads.invalidated()).toEqual([true, true, true]);
        expect(reads.requests()).toBe(3);
        act(() => reads.setEnabled(true));
      }
      expect(reads.requests()).toBe(6);
      await act(async () => reads.release());
      expect(reads.values()).toEqual([2, 2, 2]);
      expect(reads.invalidated()).toEqual([false, false, false]);
      reads.close();
    }
  );

  it("Should preserve retired refreshes across profile and QueryClient changes", async () => {
    const { queryClient: originalClient, sources, factory } = harness();
    const replacementClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    let queryClient = originalClient;
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    let revision = 1;
    let scope: ProfileScopeParams = SCOPED;
    const retiredReads = observeAggregateReads(originalClient, () => revision);
    const currentReads = observeAggregateReads(replacementClient, () => revision);
    const { rerender, unmount } = renderHook(
      () => useHomeLive({ scope, eventSourceFactory: factory }),
      { wrapper }
    );
    act(() => {
      sources[0]?.emit(activityEvent("evt-old", "task.run_started"));
      revision = 2;
      sources[0]?.emit(activityEvent("evt-old-final", "task.run_completed"));
    });
    scope = AGGREGATE;
    rerender();
    scope = SCOPED;
    queryClient = replacementClient;
    rerender();
    act(() => {
      revision = 3;
      sources[2]?.emit(activityEvent("evt-current", "task.run_started"));
      revision = 4;
      sources[2]?.emit(activityEvent("evt-current-final", "task.run_completed"));
    });
    expect(retiredReads.requests()).toBe(3);
    expect(currentReads.requests()).toBe(3);

    await act(async () => retiredReads.release());
    expect(retiredReads.requests()).toBe(6);
    expect(currentReads.requests()).toBe(3);
    // Settling the retired generation must not consume the replacement generation's trailing read.
    await act(async () => currentReads.release());
    expect(currentReads.requests()).toBe(6);
    await act(async () => {
      retiredReads.release();
      currentReads.release();
    });
    expect(retiredReads.values()).toEqual([4, 4, 4]);
    expect(currentReads.values()).toEqual([4, 4, 4]);
    expect(retiredReads.requests()).toBe(6);
    expect(currentReads.requests()).toBe(6);
    expect(retiredReads.cancellations()).toBe(0);
    expect(currentReads.cancellations()).toBe(0);

    unmount();
    retiredReads.close();
    currentReads.close();
  });

  it("Should throttle overview refreshes for non-lifecycle events", () => {
    const { queryClient, sources, factory, wrapper } = harness();
    const overviewKey = homeOverviewOptions({ profile: "default" }).queryKey;
    queryClient.setQueryData(
      overviewKey,
      makeHomeOverview({ generated_at: "2026-07-23T12:00:00Z" })
    );

    renderHook(() => useHomeLive({ scope: SCOPED, eventSourceFactory: factory }), { wrapper });
    act(() => {
      sources[0]?.emit(activityEvent("evt-a", "message"));
    });
    expect(queryClient.getQueryState(overviewKey)?.isInvalidated).toBe(true);

    // Re-seeding clears the invalidation; a second non-lifecycle event inside the
    // 60s window must not invalidate the overview again.
    queryClient.setQueryData(
      overviewKey,
      makeHomeOverview({ generated_at: "2026-07-23T12:01:00Z" })
    );
    act(() => {
      sources[0]?.emit(activityEvent("evt-b", "message"));
    });
    expect(queryClient.getQueryState(overviewKey)?.isInvalidated).toBe(false);
  });

  it("Should reject malformed stream payloads before the cache write", () => {
    const { queryClient, sources, factory, wrapper } = harness();
    const key = homeActivityOptions(SCOPED).queryKey;
    queryClient.setQueryData<HomeActivityEvent[]>(key, [activityEvent("evt-1", "message")]);
    const errors: unknown[] = [];
    const onError = (error: unknown) => {
      errors.push(error);
    };

    renderHook(() => useHomeLive({ scope: SCOPED, eventSourceFactory: factory, onError }), {
      wrapper,
    });
    act(() => {
      sources[0]?.emit({ type: "message" });
    });

    expect(errors).toHaveLength(1);
    const cached = queryClient.getQueryData<HomeActivityEvent[]>(key);
    expect(cached?.map(event => event.id)).toEqual(["evt-1"]);
  });

  it("Should reject a stream event whose timestamp is not a real ISO instant", () => {
    const { queryClient, sources, factory, wrapper } = harness();
    const key = homeActivityOptions(SCOPED).queryKey;
    queryClient.setQueryData<HomeActivityEvent[]>(key, [activityEvent("evt-1", "message")]);
    const errors: unknown[] = [];
    const onError = (error: unknown) => {
      errors.push(error);
    };

    renderHook(() => useHomeLive({ scope: SCOPED, eventSourceFactory: factory, onError }), {
      wrapper,
    });
    act(() => {
      // Valid id/type but a non-RFC3339 timestamp the old min(1) guard accepted;
      // <Time> would render "Invalid Date". The schema must fail it closed.
      sources[0]?.emit({ id: "evt-bad", type: "message", timestamp: "yesterday" });
    });

    expect(errors).toHaveLength(1);
    const cached = queryClient.getQueryData<HomeActivityEvent[]>(key);
    expect(cached?.map(event => event.id)).toEqual(["evt-1"]);
  });

  it("Should scope the stream url and close the source on unmount", () => {
    const { sources, factory, wrapper } = harness();
    const { unmount } = renderHook(
      () => useHomeLive({ workspaceId: "ws-1", scope: SCOPED, eventSourceFactory: factory }),
      { wrapper }
    );

    expect(sources[0]?.url).toBe("/api/logs/stream?workspace_id=ws-1&profile=default");
    unmount();
    expect(sources[0]?.closed).toBe(true);
  });

  it("Should drop a queued activity frame after the stream closes", () => {
    const { queryClient, sources, factory, wrapper } = harness();
    const key = homeActivityOptions(SCOPED).queryKey;
    queryClient.setQueryData<HomeActivityEvent[]>(key, [activityEvent("evt-current", "message")]);
    const { unmount } = renderHook(
      () => useHomeLive({ scope: SCOPED, eventSourceFactory: factory }),
      { wrapper }
    );
    const deliverQueued = sources[0]?.onmessage;
    unmount();
    act(() => {
      deliverQueued?.(
        new MessageEvent("message", {
          data: JSON.stringify(activityEvent("evt-late", "task.run_completed")),
        })
      );
    });
    expect(queryClient.getQueryData<HomeActivityEvent[]>(key)?.map(event => event.id)).toEqual([
      "evt-current",
    ]);
  });

  it("Should name the aggregate on the wire rather than falling back to default", () => {
    const { sources, factory, wrapper } = harness();
    renderHook(() => useHomeLive({ scope: AGGREGATE, eventSourceFactory: factory }), { wrapper });

    // An omitted selector resolves to `default` server-side, so the aggregate
    // has to be explicit or the feed silently narrows to one profile.
    expect(sources[0]?.url).toBe("/api/logs/stream?all_profiles=true");
  });

  it("Should re-open the stream on a profile switch and leave the old lens alone", () => {
    const { queryClient, sources, factory, wrapper } = harness();
    const scopedKey = homeActivityOptions(SCOPED).queryKey;
    const aggregateKey = homeActivityOptions(AGGREGATE).queryKey;
    queryClient.setQueryData<HomeActivityEvent[]>(scopedKey, [activityEvent("evt-1", "message")]);
    queryClient.setQueryData<HomeActivityEvent[]>(aggregateKey, [
      activityEvent("evt-9", "message"),
    ]);

    let scope: ProfileScopeParams = SCOPED;
    const { rerender } = renderHook(() => useHomeLive({ scope, eventSourceFactory: factory }), {
      wrapper,
    });
    expect(sources).toHaveLength(1);

    scope = AGGREGATE;
    rerender();

    // A distinct url, and the source that spoke for the old lens is shut.
    expect(sources).toHaveLength(2);
    expect(sources[0]?.closed).toBe(true);
    expect(sources[1]?.url).not.toBe(sources[0]?.url);

    act(() => {
      sources[1]?.emit(activityEvent("evt-2", "message"));
    });

    expect(queryClient.getQueryData<HomeActivityEvent[]>(aggregateKey)?.map(e => e.id)).toEqual([
      "evt-2",
      "evt-9",
    ]);
    // The profile the operator left keeps exactly what it had.
    expect(queryClient.getQueryData<HomeActivityEvent[]>(scopedKey)?.map(e => e.id)).toEqual([
      "evt-1",
    ]);
  });
});
