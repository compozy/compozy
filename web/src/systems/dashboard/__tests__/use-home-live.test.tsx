import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it } from "vitest";

import { tasksKeys } from "@/systems/tasks";

import { useHomeLive, type HomeLiveEventSource } from "../hooks/use-home-live";
import { dashboardKeys } from "../lib/query-keys";
import { homeActivityOptions } from "../lib/query-options";
import type { HomeActivityEvent } from "../types";

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

describe("useHomeLive", () => {
  it("Should prepend stream events into the activity cache without duplicates", () => {
    const { queryClient, sources, factory, wrapper } = harness();
    const key = homeActivityOptions().queryKey;
    queryClient.setQueryData<HomeActivityEvent[]>(key, [activityEvent("evt-1", "message")]);

    renderHook(() => useHomeLive({ eventSourceFactory: factory }), { wrapper });
    expect(sources).toHaveLength(1);
    expect(sources[0]?.url).toBe("/api/logs/stream");

    act(() => {
      sources[0]?.emit(activityEvent("evt-2", "message"));
      sources[0]?.emit(activityEvent("evt-2", "message"));
    });

    const cached = queryClient.getQueryData<HomeActivityEvent[]>(key);
    expect(cached?.map(event => event.id)).toEqual(["evt-2", "evt-1"]);
  });

  it("Should refresh task aggregates immediately on task lifecycle events", () => {
    const { queryClient, sources, factory, wrapper } = harness();
    queryClient.setQueryData(dashboardKeys.overviewRoot(), { seeded: true });
    queryClient.setQueryData(tasksKeys.dashboardRoot(), { seeded: true });
    queryClient.setQueryData(tasksKeys.inboxRoot(), { seeded: true });

    renderHook(() => useHomeLive({ eventSourceFactory: factory }), { wrapper });
    act(() => {
      sources[0]?.emit(activityEvent("evt-task", "task.run_completed"));
    });

    expect(queryClient.getQueryState(dashboardKeys.overviewRoot())?.isInvalidated).toBe(true);
    expect(queryClient.getQueryState(tasksKeys.dashboardRoot())?.isInvalidated).toBe(true);
    expect(queryClient.getQueryState(tasksKeys.inboxRoot())?.isInvalidated).toBe(true);
  });

  it("Should throttle overview refreshes for non-lifecycle events", () => {
    const { queryClient, sources, factory, wrapper } = harness();
    queryClient.setQueryData(dashboardKeys.overviewRoot(), { tick: 0 });

    renderHook(() => useHomeLive({ eventSourceFactory: factory }), { wrapper });
    act(() => {
      sources[0]?.emit(activityEvent("evt-a", "message"));
    });
    expect(queryClient.getQueryState(dashboardKeys.overviewRoot())?.isInvalidated).toBe(true);

    // Re-seeding clears the invalidation; a second non-lifecycle event inside the
    // 60s window must not invalidate the overview again.
    queryClient.setQueryData(dashboardKeys.overviewRoot(), { tick: 1 });
    act(() => {
      sources[0]?.emit(activityEvent("evt-b", "message"));
    });
    expect(queryClient.getQueryState(dashboardKeys.overviewRoot())?.isInvalidated).toBe(false);
  });

  it("Should reject malformed stream payloads before the cache write", () => {
    const { queryClient, sources, factory, wrapper } = harness();
    const key = homeActivityOptions().queryKey;
    queryClient.setQueryData<HomeActivityEvent[]>(key, [activityEvent("evt-1", "message")]);
    const errors: unknown[] = [];
    const onError = (error: unknown) => {
      errors.push(error);
    };

    renderHook(() => useHomeLive({ eventSourceFactory: factory, onError }), { wrapper });
    act(() => {
      sources[0]?.emit({ type: "message" });
    });

    expect(errors).toHaveLength(1);
    const cached = queryClient.getQueryData<HomeActivityEvent[]>(key);
    expect(cached?.map(event => event.id)).toEqual(["evt-1"]);
  });

  it("Should reject a stream event whose timestamp is not a real ISO instant", () => {
    const { queryClient, sources, factory, wrapper } = harness();
    const key = homeActivityOptions().queryKey;
    queryClient.setQueryData<HomeActivityEvent[]>(key, [activityEvent("evt-1", "message")]);
    const errors: unknown[] = [];
    const onError = (error: unknown) => {
      errors.push(error);
    };

    renderHook(() => useHomeLive({ eventSourceFactory: factory, onError }), { wrapper });
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
      () => useHomeLive({ workspaceId: "ws-1", eventSourceFactory: factory }),
      { wrapper }
    );

    expect(sources[0]?.url).toBe("/api/logs/stream?workspace_id=ws-1");
    unmount();
    expect(sources[0]?.closed).toBe(true);
  });
});
