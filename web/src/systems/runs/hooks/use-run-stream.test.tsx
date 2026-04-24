import { act, renderHook } from "@testing-library/react";
import type { ReactElement, ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { useRunStream } from "./use-run-stream";
import type {
  OpenRunStreamOptions,
  RunStreamController,
  RunStreamFactory,
  RunStreamHandler,
} from "../lib/stream";

interface FakeController extends RunStreamController {
  emit: RunStreamHandler;
  closed: boolean;
  options: OpenRunStreamOptions;
}

function createStreamHarness() {
  const controllers: FakeController[] = [];
  const factory: RunStreamFactory = (options, handler) => {
    const controller: FakeController = {
      emit: handler,
      closed: false,
      options,
      close() {
        controller.closed = true;
      },
    };
    controllers.push(controller);
    return controller;
  };
  return { factory, controllers };
}

function collectReconnects() {
  const scheduled: Array<() => void> = [];
  const cancels: Array<() => void> = [];
  const schedule = vi.fn((callback: () => void, _delay: number) => {
    scheduled.push(callback);
    const cancel = vi.fn();
    cancels.push(cancel);
    return cancel;
  });
  return { schedule, scheduled, cancels };
}

function wrapper({ children }: { children: ReactNode }): ReactElement {
  return <>{children}</>;
}

describe("useRunStream", () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it("Should open a stream and update cursor on run.event", () => {
    const harness = createStreamHarness();
    const onEvent = vi.fn();
    const { result } = renderHook(
      () =>
        useRunStream({
          runId: "r-1",
          enabled: true,
          initialCursor: null,
          factory: harness.factory,
          onEvent,
        }),
      { wrapper }
    );
    expect(result.current.status).toBe("connecting");
    expect(harness.controllers).toHaveLength(1);
    const controller = harness.controllers[0]!;
    act(() => controller.emit({ type: "open" }));
    expect(result.current.status).toBe("open");
    act(() =>
      controller.emit({
        type: "event",
        eventId: "2026-01-01T00:00:01Z|00000000000000000001",
        payload: { kind: "run.started" },
      })
    );
    expect(result.current.eventCount).toBe(1);
    expect(result.current.lastCursor).toBe("2026-01-01T00:00:01Z|00000000000000000001");
    expect(onEvent).toHaveBeenCalledTimes(1);
  });

  it("Should record heartbeats and update cursor when present", () => {
    const harness = createStreamHarness();
    const { result } = renderHook(() => useRunStream({ runId: "r-1", factory: harness.factory }));
    const controller = harness.controllers[0]!;
    act(() => controller.emit({ type: "open" }));
    act(() =>
      controller.emit({
        type: "heartbeat",
        eventId: "2026-01-01T00:00:15Z|00000000000000000002",
        payload: { cursor: "2026-01-01T00:00:15Z|00000000000000000002", ts: "x" },
      })
    );
    expect(result.current.lastHeartbeat).not.toBeNull();
    expect(result.current.lastCursor).toBe("2026-01-01T00:00:15Z|00000000000000000002");
  });

  it("Should mark the stream as overflowed and fire onOverflow", () => {
    const harness = createStreamHarness();
    const onOverflow = vi.fn();
    const { result } = renderHook(() =>
      useRunStream({ runId: "r-1", factory: harness.factory, onOverflow })
    );
    const controller = harness.controllers[0]!;
    act(() => controller.emit({ type: "open" }));
    act(() =>
      controller.emit({
        type: "overflow",
        eventId: "2026-01-01T00:01:00Z|00000000000000000010",
        payload: { reason: "replay truncated" },
      })
    );
    expect(result.current.status).toBe("overflowed");
    expect(result.current.lastOverflow?.reason).toBe("replay truncated");
    expect(onOverflow).toHaveBeenCalledTimes(1);
  });

  it("Should reconnect after an error signal with a scheduled callback", () => {
    const harness = createStreamHarness();
    const schedule = collectReconnects();
    const { result } = renderHook(() =>
      useRunStream({
        runId: "r-1",
        factory: harness.factory,
        scheduleReconnect: schedule.schedule,
        reconnectDelayMs: 50,
      })
    );
    const first = harness.controllers[0]!;
    act(() => first.emit({ type: "open" }));
    act(() =>
      first.emit({
        type: "event",
        eventId: "2026-01-01T00:00:02Z|00000000000000000003",
        payload: {},
      })
    );
    act(() => first.emit({ type: "error", error: new Error("boom") }));
    expect(result.current.status).toBe("reconnecting");
    expect(schedule.schedule).toHaveBeenCalledTimes(1);
    act(() => schedule.scheduled[0]?.());
    expect(harness.controllers).toHaveLength(2);
    const second = harness.controllers[1]!;
    expect(second.options.lastEventId).toBe("2026-01-01T00:00:02Z|00000000000000000003");
    expect(result.current.reconnectCount).toBe(1);
  });

  it("Should close the previous controller when resuming from a cursor", () => {
    const harness = createStreamHarness();
    const { result } = renderHook(() => useRunStream({ runId: "r-1", factory: harness.factory }));
    const first = harness.controllers[0]!;
    act(() => first.emit({ type: "open" }));
    act(() => result.current.resumeFromCursor("2026-01-01T00:01:00Z|00000000000000000020"));
    expect(first.closed).toBe(true);
    expect(harness.controllers).toHaveLength(2);
    const second = harness.controllers[1]!;
    expect(second.options.lastEventId).toBe("2026-01-01T00:01:00Z|00000000000000000020");
    expect(result.current.lastCursor).toBe("2026-01-01T00:01:00Z|00000000000000000020");
  });

  it("Should not open a stream when disabled", () => {
    const harness = createStreamHarness();
    const { result } = renderHook(() =>
      useRunStream({ runId: "r-1", factory: harness.factory, enabled: false })
    );
    expect(harness.controllers).toHaveLength(0);
    expect(result.current.status).toBe("closed");
  });

  it("Should stop reconnecting after reaching the max attempts", () => {
    const harness = createStreamHarness();
    const schedule = collectReconnects();
    const { result } = renderHook(() =>
      useRunStream({
        runId: "r-1",
        factory: harness.factory,
        scheduleReconnect: schedule.schedule,
        reconnectDelayMs: 10,
        maxReconnectAttempts: 1,
      })
    );
    act(() => harness.controllers[0]!.emit({ type: "error", error: new Error("1") }));
    act(() => schedule.scheduled[0]?.());
    act(() => harness.controllers[1]!.emit({ type: "error", error: new Error("2") }));
    expect(result.current.status).toBe("closed");
  });

  it("Should ignore error signals after an explicit close", () => {
    const harness = createStreamHarness();
    const schedule = collectReconnects();
    const { result } = renderHook(() =>
      useRunStream({
        runId: "r-1",
        factory: harness.factory,
        scheduleReconnect: schedule.schedule,
      })
    );

    const controller = harness.controllers[0]!;
    act(() => controller.emit({ type: "open" }));
    act(() => result.current.close());
    act(() => controller.emit({ type: "error", error: new Error("closed by server") }));

    expect(result.current.status).toBe("closed");
    expect(result.current.error).toBeNull();
    expect(schedule.schedule).not.toHaveBeenCalled();
  });
});
