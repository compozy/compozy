import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { useLoopStream } from "@/systems/loops";
import type { LoopRunEventFrame, LoopStreamEventSource } from "@/systems/loops";

// The twelve enumerated Loop-run SSE kinds the run page binds (techspec §observability,
// L-017). Kept local to the test so a drift in the hook's subscription list is caught.
const LOOP_EVENT_KINDS = [
  "node_running",
  "node_succeeded",
  "node_failed",
  "gate_verdict",
  "generation_started",
  "channel_msg",
  "token_tick",
  "needs_approval",
  "status_changed",
  "goal_turn_started",
  "goal_turn_completed",
  "goal_status_changed",
] as const;

class FakeLoopStreamEventSource {
  public close = vi.fn();
  public onmessage: ((event: MessageEvent) => void) | null = null;
  public onerror: ((event: Event) => void) | null = null;
  public readonly addEventListener = vi.fn(this.recordAddListener.bind(this));
  public readonly removeEventListener = vi.fn(this.recordRemoveListener.bind(this));
  private readonly listeners = new Map<string, EventListenerOrEventListenerObject[]>();

  private recordAddListener(type: string, listener: EventListenerOrEventListenerObject) {
    const current = this.listeners.get(type) ?? [];
    current.push(listener);
    this.listeners.set(type, current);
  }

  private recordRemoveListener(type: string, listener: EventListenerOrEventListenerObject) {
    const current = this.listeners.get(type) ?? [];
    this.listeners.set(
      type,
      current.filter(candidate => candidate !== listener)
    );
  }

  hasListener(type: string): boolean {
    return (this.listeners.get(type) ?? []).length > 0;
  }

  emitMessage(data: unknown) {
    if (!this.onmessage) {
      return;
    }
    this.onmessage(
      new MessageEvent("message", {
        data: typeof data === "string" ? data : JSON.stringify(data),
      })
    );
  }

  emitNamed(type: string, data: unknown) {
    const current = this.listeners.get(type) ?? [];
    if (current.length === 0) {
      return;
    }
    const event = new MessageEvent(type, {
      data: typeof data === "string" ? data : JSON.stringify(data),
    });
    for (const listener of current) {
      if (typeof listener === "function") {
        listener(event);
      } else if (listener && typeof listener.handleEvent === "function") {
        listener.handleEvent(event);
      }
    }
  }

  emitError(event?: Event) {
    if (!this.onerror) {
      return;
    }
    this.onerror(event ?? new Event("error"));
  }
}

function createNamedOnlySource(): {
  source: LoopStreamEventSource;
  hasListener: (type: string) => boolean;
} {
  const listeners = new Map<string, EventListenerOrEventListenerObject[]>();
  const source: LoopStreamEventSource = {
    onmessage: null,
    onerror: null,
    close: vi.fn(),
    addEventListener: (type, listener) => {
      const current = listeners.get(type) ?? [];
      current.push(listener);
      listeners.set(type, current);
    },
    // Intentionally omit removeEventListener to exercise the cleanup branch that
    // must still close the source and clear onmessage/onerror without it.
  };
  return {
    source,
    hasListener: type => (listeners.get(type) ?? []).length > 0,
  };
}

function createWrapper(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

function buildFrame(overrides: Partial<LoopRunEventFrame> = {}): LoopRunEventFrame {
  return {
    id: "loopevt_1",
    seq: 42,
    kind: "status_changed",
    loop_run_id: "looprun_1",
    workspace_id: "ws_1",
    at: "2026-07-05T12:00:00Z",
    payload: { from: "running", to: "paused", cause: "operator_pause" },
    ...overrides,
  };
}

describe("useLoopStream", () => {
  it("Should open a workspace-scoped stream seeded with after_sequence and subscribe to every enumerated kind", () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const eventSource = new FakeLoopStreamEventSource();
    const factory = vi.fn(() => eventSource);

    renderHook(
      () => useLoopStream("ws_1", "looprun_1", { afterSequence: 14, eventSourceFactory: factory }),
      { wrapper: createWrapper(queryClient) }
    );

    expect(factory).toHaveBeenCalledTimes(1);
    expect(factory).toHaveBeenCalledWith(
      "/api/workspaces/ws_1/loop-runs/looprun_1/events?after_sequence=14"
    );
    for (const kind of LOOP_EVENT_KINDS) {
      expect(eventSource.hasListener(kind)).toBe(true);
    }
  });

  it("Should apply each SSE event kind to onEvent (no kind silently dropped)", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const eventSource = new FakeLoopStreamEventSource();
    const factory = vi.fn(() => eventSource);
    const onEvent = vi.fn();

    renderHook(
      () =>
        useLoopStream("ws_1", "looprun_1", {
          afterSequence: 0,
          eventSourceFactory: factory,
          onEvent,
        }),
      { wrapper: createWrapper(queryClient) }
    );

    for (const [index, kind] of LOOP_EVENT_KINDS.entries()) {
      const frame = buildFrame({ kind, seq: index + 1 });
      act(() => {
        eventSource.emitNamed(kind, frame);
      });
      // Every enumerated kind reaches onEvent with the parsed frame — the L-017
      // named-listener invariant.
      await waitFor(() => {
        expect(onEvent).toHaveBeenCalledWith(frame);
      });
    }

    expect(onEvent).toHaveBeenCalledTimes(LOOP_EVENT_KINDS.length);
  });

  it("Should invalidate the run queries on lifecycle kinds but not on display kinds", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const invalidateQueries = vi.spyOn(queryClient, "invalidateQueries");
    const eventSource = new FakeLoopStreamEventSource();
    const factory = vi.fn(() => eventSource);
    const onEvent = vi.fn();

    renderHook(() => useLoopStream("ws_1", "looprun_1", { eventSourceFactory: factory, onEvent }), {
      wrapper: createWrapper(queryClient),
    });

    act(() => {
      eventSource.emitNamed("status_changed", buildFrame({ kind: "status_changed" }));
    });
    await waitFor(() => expect(onEvent).toHaveBeenCalledTimes(1));
    expect(invalidateQueries).toHaveBeenCalledWith({
      queryKey: ["loops", "run-detail", "ws_1", "looprun_1"],
    });
    // Only this workspace's runs lists — not the unscoped root across all workspaces.
    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: ["loops", "runs", "ws_1"] });
    // Status changes own the catalog's latest-run and aggregate projections.
    expect(invalidateQueries).toHaveBeenCalledWith({
      queryKey: ["loops", "catalog", "ws_1"],
    });

    invalidateQueries.mockClear();
    onEvent.mockClear();

    // High-frequency display frames are applied locally via onEvent, never invalidating.
    act(() => {
      eventSource.emitNamed("token_tick", buildFrame({ kind: "token_tick" }));
      eventSource.emitNamed("channel_msg", buildFrame({ kind: "channel_msg" }));
      eventSource.emitNamed("goal_turn_started", buildFrame({ kind: "goal_turn_started" }));
      eventSource.emitNamed("goal_turn_completed", buildFrame({ kind: "goal_turn_completed" }));
    });
    await waitFor(() => expect(onEvent).toHaveBeenCalledTimes(4));
    expect(invalidateQueries).not.toHaveBeenCalled();

    act(() => {
      eventSource.emitNamed("goal_status_changed", buildFrame({ kind: "goal_status_changed" }));
    });
    await waitFor(() => expect(onEvent).toHaveBeenCalledTimes(5));
    expect(invalidateQueries).toHaveBeenCalledWith({
      queryKey: ["loops", "run-detail", "ws_1", "looprun_1"],
    });
    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: ["loops", "runs", "ws_1"] });
  });

  it("Should deliver a terminal status frame before closing its replay source", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const eventSource = new FakeLoopStreamEventSource();
    const factory = vi.fn(() => eventSource);
    const onEvent = vi.fn();

    renderHook(() => useLoopStream("ws_1", "looprun_1", { eventSourceFactory: factory, onEvent }), {
      wrapper: createWrapper(queryClient),
    });

    const terminal = buildFrame({
      kind: "status_changed",
      payload: { status: "failed", failure: { kind: "coordinator_failure" } },
    });
    act(() => {
      eventSource.emitNamed("status_changed", terminal);
    });

    await waitFor(() => expect(onEvent).toHaveBeenCalledWith(terminal));
    expect(eventSource.close).toHaveBeenCalledTimes(1);
  });

  it("Should not reopen the EventSource when only onEvent identity changes", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const eventSource = new FakeLoopStreamEventSource();
    const factory = vi.fn(() => eventSource);

    const { rerender } = renderHook(
      ({ onEvent }: { onEvent: () => void }) =>
        useLoopStream("ws_1", "looprun_1", { eventSourceFactory: factory, onEvent }),
      { wrapper: createWrapper(queryClient), initialProps: { onEvent: vi.fn() } }
    );

    expect(factory).toHaveBeenCalledTimes(1);

    // A fresh (inline-style) callback each render must not tear down the stream.
    const nextOnEvent = vi.fn();
    rerender({ onEvent: nextOnEvent });
    expect(factory).toHaveBeenCalledTimes(1);
    expect(eventSource.close).not.toHaveBeenCalled();

    // The latest callback is the one invoked (ref stays current).
    act(() => {
      eventSource.emitNamed("status_changed", buildFrame());
    });
    await waitFor(() => expect(nextOnEvent).toHaveBeenCalledTimes(1));
  });

  it("Should seed after_sequence=0 into the stream URL to opt into Last-Event-ID:0 precedence", () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const factory = vi.fn(() => new FakeLoopStreamEventSource());

    renderHook(
      () => useLoopStream("ws_1", "looprun_1", { afterSequence: 0, eventSourceFactory: factory }),
      { wrapper: createWrapper(queryClient) }
    );

    expect(factory).toHaveBeenCalledWith(
      "/api/workspaces/ws_1/loop-runs/looprun_1/events?after_sequence=0"
    );
  });

  it("Should omit the query string when no after_sequence is provided", () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const factory = vi.fn(() => new FakeLoopStreamEventSource());

    renderHook(() => useLoopStream("ws_1", "looprun_1", { eventSourceFactory: factory }), {
      wrapper: createWrapper(queryClient),
    });

    expect(factory).toHaveBeenCalledWith("/api/workspaces/ws_1/loop-runs/looprun_1/events");
  });

  it("Should still parse defensive unnamed message frames via onmessage", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const eventSource = new FakeLoopStreamEventSource();
    const factory = vi.fn(() => eventSource);
    const onEvent = vi.fn();

    renderHook(() => useLoopStream("ws_1", "looprun_1", { eventSourceFactory: factory, onEvent }), {
      wrapper: createWrapper(queryClient),
    });

    const frame = buildFrame();
    act(() => {
      eventSource.emitMessage(frame);
    });

    await waitFor(() => {
      expect(onEvent).toHaveBeenCalledWith(frame);
    });
  });

  it("Should remove every listener and close the source on cleanup", () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const eventSource = new FakeLoopStreamEventSource();
    const factory = vi.fn(() => eventSource);

    const { unmount } = renderHook(
      () => useLoopStream("ws_1", "looprun_1", { afterSequence: 4, eventSourceFactory: factory }),
      { wrapper: createWrapper(queryClient) }
    );

    unmount();
    expect(eventSource.close).toHaveBeenCalledTimes(1);
    expect(eventSource.onmessage).toBeNull();
    expect(eventSource.onerror).toBeNull();
    for (const kind of LOOP_EVENT_KINDS) {
      expect(eventSource.removeEventListener).toHaveBeenCalledWith(kind, expect.any(Function));
      expect(eventSource.hasListener(kind)).toBe(false);
    }
  });

  it("Should close the source on cleanup even when removeEventListener is missing", () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const namedOnly = createNamedOnlySource();
    const factory = vi.fn(() => namedOnly.source);

    const { unmount } = renderHook(
      () => useLoopStream("ws_1", "looprun_1", { afterSequence: 1, eventSourceFactory: factory }),
      { wrapper: createWrapper(queryClient) }
    );

    expect(namedOnly.hasListener("status_changed")).toBe(true);
    expect(() => unmount()).not.toThrow();
    expect(namedOnly.source.close).toHaveBeenCalledTimes(1);
    expect(namedOnly.source.onmessage).toBeNull();
    expect(namedOnly.source.onerror).toBeNull();
  });

  it("Should not open a source when disabled", () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const factory = vi.fn(() => new FakeLoopStreamEventSource());

    renderHook(
      () =>
        useLoopStream("ws_1", "looprun_1", {
          enabled: false,
          afterSequence: 1,
          eventSourceFactory: factory,
        }),
      { wrapper: createWrapper(queryClient) }
    );

    expect(factory).not.toHaveBeenCalled();
  });

  it("Should not open a source for an empty workspace or run id", () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const factory = vi.fn(() => new FakeLoopStreamEventSource());

    const { rerender } = renderHook(
      ({ ws, run }: { ws: string; run: string }) =>
        useLoopStream(ws, run, { afterSequence: 1, eventSourceFactory: factory }),
      { wrapper: createWrapper(queryClient), initialProps: { ws: "   ", run: "looprun_1" } }
    );
    expect(factory).not.toHaveBeenCalled();

    rerender({ ws: "ws_1", run: "  " });
    expect(factory).not.toHaveBeenCalled();
  });

  it("Should call onError when a named SSE payload cannot be parsed", () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const eventSource = new FakeLoopStreamEventSource();
    const factory = vi.fn(() => eventSource);
    const onError = vi.fn();
    const onEvent = vi.fn();

    renderHook(
      () =>
        useLoopStream("ws_1", "looprun_1", {
          afterSequence: 7,
          eventSourceFactory: factory,
          onEvent,
          onError,
        }),
      { wrapper: createWrapper(queryClient) }
    );

    expect(() => {
      act(() => {
        eventSource.emitNamed("status_changed", "{not valid json");
      });
    }).not.toThrow();

    expect(onError).toHaveBeenCalledTimes(1);
    expect(onEvent).not.toHaveBeenCalled();
  });

  it("Should let a throw from the onEvent consumer propagate instead of misreporting it as a parse error", () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const eventSource = new FakeLoopStreamEventSource();
    const factory = vi.fn(() => eventSource);
    const onError = vi.fn();
    const onEvent = vi.fn(() => {
      throw new Error("consumer boom");
    });

    renderHook(
      () => useLoopStream("ws_1", "looprun_1", { eventSourceFactory: factory, onEvent, onError }),
      { wrapper: createWrapper(queryClient) }
    );

    expect(() => {
      act(() => {
        eventSource.emitNamed("status_changed", buildFrame());
      });
    }).toThrow("consumer boom");
    // A consumer bug must not be swallowed and relabeled a stream-parse failure.
    expect(onError).not.toHaveBeenCalled();
  });

  it("Should forward connection errors to onError", () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const eventSource = new FakeLoopStreamEventSource();
    const factory = vi.fn(() => eventSource);
    const onError = vi.fn();

    renderHook(
      () =>
        useLoopStream("ws_1", "looprun_1", {
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

  it("Should reuse the same source across re-renders with the same inputs and reopen on resume change", () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const factory = vi.fn(() => new FakeLoopStreamEventSource());

    const { rerender } = renderHook(
      ({ seq }: { seq: number }) =>
        useLoopStream("ws_1", "looprun_1", { afterSequence: seq, eventSourceFactory: factory }),
      { wrapper: createWrapper(queryClient), initialProps: { seq: 14 } }
    );

    expect(factory).toHaveBeenCalledTimes(1);
    rerender({ seq: 14 });
    expect(factory).toHaveBeenCalledTimes(1);
    rerender({ seq: 22 });
    expect(factory).toHaveBeenCalledTimes(2);
  });
});
