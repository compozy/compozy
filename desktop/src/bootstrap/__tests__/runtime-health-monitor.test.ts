import { afterEach, describe, expect, it, vi } from "vitest";

import { RuntimeHealthMonitor } from "../runtime-health-monitor";
import { monitorProductRuntime, type ProductRuntimeShell } from "../runtime-reconnect";

const ORIGIN = "http://127.0.0.1:2123";

function runtimeFetcher(isUp: () => boolean) {
  return async () => {
    if (!isUp()) throw new Error("runtime unavailable");
    return new Response(null, { status: 200 });
  };
}

afterEach(() => {
  vi.useRealTimers();
});

// Invariant: a product window is never left bound to a runtime that stopped answering, and a
// daemon restart within the reconnect window keeps the window instead of dropping it.
describe("runtime health monitor", () => {
  it("Should probe the bounded runtime identity endpoint", async () => {
    const requests: string[] = [];
    const monitor = new RuntimeHealthMonitor({
      origin: ORIGIN,
      intervalMs: 1,
      fetcher: async input => {
        requests.push(input);
        return new Response(null, { status: 200 });
      },
      onDisconnected: async () => {},
      onReconnected: async () => {},
      onLost: async () => {},
    });

    monitor.start();
    try {
      await expect.poll(() => requests[0]).toBe(`${ORIGIN}/api/status/identity`);
    } finally {
      monitor.stop();
    }
  });

  it("Should report a restart outage once and its reconnection without losing the runtime", async () => {
    vi.useFakeTimers();
    let up = true;
    const events: string[] = [];
    const monitor = new RuntimeHealthMonitor({
      origin: ORIGIN,
      intervalMs: 2,
      failureThreshold: 2,
      reconnectTimeoutMs: 100,
      fetcher: runtimeFetcher(() => up),
      onDisconnected: async () => void events.push("disconnected"),
      onReconnected: async () => void events.push("reconnected"),
      onLost: async () => void events.push("lost"),
    });
    monitor.start();
    try {
      await vi.advanceTimersByTimeAsync(2);
      up = false;
      await vi.advanceTimersByTimeAsync(2);
      expect(events).toEqual([]);
      await vi.advanceTimersByTimeAsync(20);
      expect(events).toEqual(["disconnected"]);
      up = true;
      await vi.advanceTimersByTimeAsync(2);
      expect(events).toEqual(["disconnected", "reconnected"]);
      await vi.advanceTimersByTimeAsync(200);
      expect(events).toEqual(["disconnected", "reconnected"]);
    } finally {
      monitor.stop();
    }
  });

  it("Should report a lost runtime once after the reconnect window and stop probing", async () => {
    vi.useFakeTimers();
    let probes = 0;
    const events: string[] = [];
    const monitor = new RuntimeHealthMonitor({
      origin: ORIGIN,
      intervalMs: 2,
      failureThreshold: 2,
      reconnectTimeoutMs: 10,
      fetcher: async () => {
        probes += 1;
        throw new Error("runtime unavailable");
      },
      onDisconnected: async () => void events.push("disconnected"),
      onReconnected: async () => void events.push("reconnected"),
      onLost: async () => void events.push("lost"),
    });
    monitor.start();
    try {
      await vi.advanceTimersByTimeAsync(4);
      expect(events).toEqual(["disconnected"]);
      await vi.advanceTimersByTimeAsync(10);
      expect(events).toEqual(["disconnected", "lost"]);
      const settled = probes;
      await vi.advanceTimersByTimeAsync(20);
      expect(events).toEqual(["disconnected", "lost"]);
      expect(probes).toBe(settled);
    } finally {
      monitor.stop();
    }
  });
});

// Invariant: the shell always holds a window while it follows its runtime, so a daemon restart
// never quits the app; the product window reloads on reconnect and hands off to boot when lost.
describe("product runtime follower", () => {
  function shell(steps: string[]): ProductRuntimeShell {
    return {
      stopped: () => false,
      publish: async snapshot => void steps.push(`publish:${snapshot.state}`),
      reloadProduct: async () => void steps.push("reload"),
      showBoot: () => void steps.push("show-boot"),
      closeProduct: async () => void steps.push("close-product"),
      resolveRuntime: async () => void steps.push("resolve"),
      logger: { info: () => undefined, error: () => undefined },
    };
  }

  it("Should keep the product window through a restart and reload it on reconnect", async () => {
    vi.useFakeTimers();
    let up = true;
    const steps: string[] = [];
    const monitor = monitorProductRuntime({ origin: ORIGIN, owned: true }, shell(steps), {
      intervalMs: 2,
      failureThreshold: 2,
      reconnectTimeoutMs: 100,
      fetcher: runtimeFetcher(() => up),
    });
    monitor.start();
    try {
      up = false;
      await vi.advanceTimersByTimeAsync(4);
      up = true;
      await vi.advanceTimersByTimeAsync(2);
      expect(steps).toEqual(["publish:disconnected", "publish:product", "reload"]);
    } finally {
      monitor.stop();
    }
  });

  it("Should open the boot window before closing the product when the runtime is lost", async () => {
    vi.useFakeTimers();
    const steps: string[] = [];
    const monitor = monitorProductRuntime({ origin: ORIGIN, owned: false }, shell(steps), {
      intervalMs: 2,
      failureThreshold: 1,
      reconnectTimeoutMs: 4,
      fetcher: runtimeFetcher(() => false),
    });
    monitor.start();
    try {
      await vi.advanceTimersByTimeAsync(10);
      expect(steps).toEqual([
        "publish:disconnected",
        "publish:disconnected",
        "show-boot",
        "close-product",
        "resolve",
      ]);
    } finally {
      monitor.stop();
    }
  });
});
