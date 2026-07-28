import { QueryClient, QueryClientProvider, useInfiniteQuery } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { buildTaskStreamUrl, tasksKeys, useTaskStream } from "@/systems/tasks";
import type { TaskListPage, TaskStreamPayload } from "@/systems/tasks";

class FakeTaskStreamEventSource {
  public close = vi.fn();
  public onmessage: ((event: MessageEvent) => void) | null = null;
  public onerror: ((event: Event) => void) | null = null;

  emitMessage(data: unknown) {
    if (!this.onmessage) {
      return;
    }
    const event = new MessageEvent("message", {
      data: typeof data === "string" ? data : JSON.stringify(data),
    });
    this.onmessage(event);
  }

  emitError(event?: Event) {
    if (!this.onerror) {
      return;
    }
    this.onerror(event ?? new Event("error"));
  }
}

function createWrapper(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

function buildStreamPayload(overrides: Partial<TaskStreamPayload> = {}): TaskStreamPayload {
  const base: TaskStreamPayload = {
    sequence: 25,
    type: "task.run_started",
    timeline: {
      event_id: "evt_25",
      sequence: 25,
      timestamp: "2026-04-17T18:05:00Z",
      event_type: "task.run_started",
      origin: { kind: "agent_session", ref: "session_a" },
      actor: { kind: "agent_session", ref: "session_a" },
    },
  } as TaskStreamPayload;
  return { ...base, ...overrides };
}

describe("buildTaskStreamUrl", () => {
  it("Should build a stream URL with after_sequence query when seed is provided", () => {
    expect(buildTaskStreamUrl("task_001", { after_sequence: 14 })).toBe(
      "/api/tasks/task_001/stream?after_sequence=14"
    );
  });

  it("Should include after_sequence=0 to opt into deterministic Last-Event-ID:0 precedence", () => {
    expect(buildTaskStreamUrl("task_001", { after_sequence: 0 })).toBe(
      "/api/tasks/task_001/stream?after_sequence=0"
    );
  });

  it("Should omit query string when no seed is provided", () => {
    expect(buildTaskStreamUrl("task_001")).toBe("/api/tasks/task_001/stream");
  });

  it("Should encode unsafe characters in the task id segment", () => {
    expect(buildTaskStreamUrl("task with spaces", { after_sequence: 1 })).toBe(
      "/api/tasks/task%20with%20spaces/stream?after_sequence=1"
    );
  });

  it("Should reject empty task ids", () => {
    expect(() => buildTaskStreamUrl("")).toThrow(/task id is required/);
    expect(() => buildTaskStreamUrl("   ")).toThrow(/task id is required/);
  });
});

describe("useTaskStream", () => {
  it("Should mark catalogs stale once per burst and reconcile them once on stream cleanup", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const filters = { limit: 1, scope: "workspace" as const, workspace: "ws_alpha" };
    const firstPage: TaskListPage = {
      facets: { owners: [], statuses: [{ count: 2, status: "ready" }] },
      page: { has_more: true, limit: 1, next_cursor: "tasks:1", total: 2 },
      tasks: [],
    };
    const secondPage: TaskListPage = {
      facets: firstPage.facets,
      page: { has_more: false, limit: 1, total: 2 },
      tasks: [],
    };
    queryClient.setQueryData(tasksKeys.list(filters), {
      pageParams: [undefined, "tasks:1"],
      pages: [firstPage, secondPage],
    });
    const catalogQuery = vi.fn(({ pageParam }: { pageParam: unknown }) =>
      Promise.resolve(pageParam === "tasks:1" ? secondPage : firstPage)
    );
    const catalog = renderHook(
      () =>
        useInfiniteQuery({
          getNextPageParam: lastPage => lastPage.page.next_cursor,
          initialPageParam: undefined as string | undefined,
          queryFn: catalogQuery,
          queryKey: tasksKeys.list(filters),
          staleTime: Number.POSITIVE_INFINITY,
        }),
      { wrapper: createWrapper(queryClient) }
    );
    await waitFor(() => expect(catalog.result.current.data?.pages).toHaveLength(2));
    expect(catalogQuery).not.toHaveBeenCalled();

    const invalidateQueries = vi.spyOn(queryClient, "invalidateQueries");
    const eventSource = new FakeTaskStreamEventSource();
    const onEvent = vi.fn();
    const stream = renderHook(
      ({ afterSequence }: { afterSequence: number }) =>
        useTaskStream("task_001", {
          afterSequence,
          eventSourceFactory: () => eventSource,
          onEvent,
        }),
      { initialProps: { afterSequence: 24 }, wrapper: createWrapper(queryClient) }
    );

    act(() => {
      eventSource.emitMessage(buildStreamPayload({ sequence: 25 }));
      eventSource.emitMessage(buildStreamPayload({ sequence: 26 }));
      eventSource.emitMessage(buildStreamPayload({ sequence: 27 }));
    });
    await waitFor(() => expect(onEvent).toHaveBeenCalledTimes(3));

    expect(catalogQuery).not.toHaveBeenCalled();
    const catalogInvalidations = invalidateQueries.mock.calls.filter(
      ([filtersArg]) => JSON.stringify(filtersArg?.queryKey) === JSON.stringify(tasksKeys.lists())
    );
    expect(catalogInvalidations).toEqual([[{ queryKey: tasksKeys.lists(), refetchType: "none" }]]);

    stream.rerender({ afterSequence: 27 });
    await Promise.resolve();
    expect(catalogQuery).not.toHaveBeenCalled();

    stream.unmount();
    await waitFor(() => expect(catalogQuery).toHaveBeenCalledTimes(2));
    catalog.unmount();
  });

  it("Should open a seeded stream and parse standard SSE message frames", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const invalidateQueries = vi.spyOn(queryClient, "invalidateQueries");
    const eventSource = new FakeTaskStreamEventSource();
    const factory = vi.fn(() => eventSource);
    const onEvent = vi.fn();

    const { unmount } = renderHook(
      () =>
        useTaskStream("task_001", {
          afterSequence: 14,
          eventSourceFactory: factory,
          onEvent,
        }),
      { wrapper: createWrapper(queryClient) }
    );

    expect(factory).toHaveBeenCalledTimes(1);
    expect(factory).toHaveBeenCalledWith("/api/tasks/task_001/stream?after_sequence=14");

    const payload = buildStreamPayload();

    act(() => {
      eventSource.emitMessage(payload);
    });

    await waitFor(() => {
      expect(onEvent).toHaveBeenCalledWith(payload);
    });

    expect(invalidateQueries).toHaveBeenCalledWith({
      queryKey: ["tasks", "detail", "task_001"],
    });
    expect(invalidateQueries).toHaveBeenCalledWith({
      queryKey: ["tasks", "timeline"],
    });
    expect(invalidateQueries).toHaveBeenCalledWith({
      queryKey: ["tasks", "context-bundle"],
    });

    unmount();
    expect(eventSource.close).toHaveBeenCalledTimes(1);
    expect(eventSource.onmessage).toBeNull();
    expect(eventSource.onerror).toBeNull();
  });

  it("Should route each payload type through the standard message contract", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const invalidateQueries = vi.spyOn(queryClient, "invalidateQueries");
    const eventSource = new FakeTaskStreamEventSource();
    const factory = vi.fn(() => eventSource);
    const onEvent = vi.fn();

    renderHook(
      () =>
        useTaskStream("task_001", {
          afterSequence: 30,
          eventSourceFactory: factory,
          onEvent,
        }),
      { wrapper: createWrapper(queryClient) }
    );

    const completed = buildStreamPayload({ sequence: 31, type: "task.run.completed" });

    act(() => {
      eventSource.emitMessage(completed);
    });

    await waitFor(() => {
      expect(onEvent).toHaveBeenCalledWith(completed);
    });
    expect(invalidateQueries).toHaveBeenCalledWith({
      queryKey: ["tasks", "detail", "task_001"],
    });

    invalidateQueries.mockClear();
    onEvent.mockClear();

    const statusChanged = buildStreamPayload({ sequence: 32, type: "task.status_changed" });

    act(() => {
      eventSource.emitMessage(statusChanged);
    });

    await waitFor(() => {
      expect(onEvent).toHaveBeenCalledWith(statusChanged);
    });
    expect(invalidateQueries).toHaveBeenCalledWith({
      queryKey: ["tasks", "detail", "task_001"],
    });

    invalidateQueries.mockClear();
    onEvent.mockClear();

    const blockExpired = buildStreamPayload({ sequence: 33, type: "task.block.expired" });

    act(() => {
      eventSource.emitMessage(blockExpired);
    });

    await waitFor(() => {
      expect(onEvent).toHaveBeenCalledWith(blockExpired);
    });
    expect(invalidateQueries).toHaveBeenCalledWith({
      queryKey: ["tasks", "detail", "task_001"],
    });
  });

  it("Should not open a source when disabled", () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const factory = vi.fn(() => new FakeTaskStreamEventSource());

    renderHook(
      () =>
        useTaskStream("task_001", {
          enabled: false,
          afterSequence: 1,
          eventSourceFactory: factory,
        }),
      { wrapper: createWrapper(queryClient) }
    );

    expect(factory).not.toHaveBeenCalled();
  });

  it("Should not open a source for an empty task id", () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const factory = vi.fn(() => new FakeTaskStreamEventSource());

    renderHook(
      () =>
        useTaskStream("   ", {
          afterSequence: 1,
          eventSourceFactory: factory,
        }),
      { wrapper: createWrapper(queryClient) }
    );

    expect(factory).not.toHaveBeenCalled();
  });

  it("Should call onError when a message payload cannot be parsed", () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const eventSource = new FakeTaskStreamEventSource();
    const factory = vi.fn(() => eventSource);
    const onError = vi.fn();
    const onEvent = vi.fn();

    renderHook(
      () =>
        useTaskStream("task_001", {
          afterSequence: 7,
          eventSourceFactory: factory,
          onEvent,
          onError,
        }),
      { wrapper: createWrapper(queryClient) }
    );

    expect(() => {
      act(() => {
        eventSource.emitMessage("{not valid json");
      });
    }).not.toThrow();

    expect(onError).toHaveBeenCalledTimes(1);
    expect(onEvent).not.toHaveBeenCalled();
  });

  it("Should forward connection errors to onError", () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const eventSource = new FakeTaskStreamEventSource();
    const factory = vi.fn(() => eventSource);
    const onError = vi.fn();

    renderHook(
      () =>
        useTaskStream("task_001", {
          afterSequence: 1,
          eventSourceFactory: factory,
          onError,
        }),
      { wrapper: createWrapper(queryClient) }
    );

    const errorEvent = new Event("error");
    act(() => {
      eventSource.emitError(errorEvent);
    });

    expect(onError).toHaveBeenCalledWith(errorEvent);
  });

  it("Should reuse the same source when re-rendered with the same inputs", () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const factory = vi.fn(() => new FakeTaskStreamEventSource());

    const { rerender } = renderHook(
      ({ seq }: { seq: number }) =>
        useTaskStream("task_001", {
          afterSequence: seq,
          eventSourceFactory: factory,
        }),
      {
        wrapper: createWrapper(queryClient),
        initialProps: { seq: 14 },
      }
    );

    expect(factory).toHaveBeenCalledTimes(1);

    rerender({ seq: 14 });
    expect(factory).toHaveBeenCalledTimes(1);

    rerender({ seq: 22 });
    expect(factory).toHaveBeenCalledTimes(2);
  });

  it("Should keep the source open when callback identities change", () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const eventSource = new FakeTaskStreamEventSource();
    const factory = vi.fn(() => eventSource);
    const firstOnEvent = vi.fn();
    const nextOnEvent = vi.fn();

    const { rerender } = renderHook(
      ({ onEvent }: { onEvent: (payload: TaskStreamPayload) => void }) =>
        useTaskStream("task_001", {
          afterSequence: 14,
          eventSourceFactory: factory,
          onEvent,
        }),
      {
        wrapper: createWrapper(queryClient),
        initialProps: { onEvent: firstOnEvent },
      }
    );

    rerender({ onEvent: nextOnEvent });
    expect(factory).toHaveBeenCalledTimes(1);
    expect(eventSource.close).not.toHaveBeenCalled();

    const payload = buildStreamPayload();
    act(() => {
      eventSource.emitMessage(payload);
    });

    expect(firstOnEvent).not.toHaveBeenCalled();
    expect(nextOnEvent).toHaveBeenCalledWith(payload);
  });
});
