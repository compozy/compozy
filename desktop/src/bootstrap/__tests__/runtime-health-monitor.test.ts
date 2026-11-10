import { afterEach, describe, expect, it, vi } from "vitest";

import { daemonFetcher, RuntimeHealthMonitor } from "../runtime-health-monitor";
import { monitorProductRuntime, type ProductRuntimeShell } from "../runtime-reconnect";

const ORIGIN = "http://127.0.0.1:2123";

function runtimeFetcher(isUp: () => boolean) {
  return async () => {
    if (!isUp()) throw new Error("runtime unavailable");
    return new Response(null, { status: 200 });
  };
}

const TRACEPARENT = /^00-[0-9a-f]{32}-[0-9a-f]{16}-00$/;

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

function stubFetch() {
  const fetchMock = vi.fn(async (_input: string, _init?: RequestInit) => new Response(null));
  vi.stubGlobal("fetch", fetchMock);
  return (call: number) => new Headers(fetchMock.mock.calls[call]?.[1]?.headers).get("traceparent");
}

// Invariant: only daemon-origin requests carry trace context, always unsampled.
describe("runtime health monitor trace propagation", () => {
  it("Should send an unsampled traceparent on the default identity probe", async () => {
    const traceparent = stubFetch();
    const monitor = new RuntimeHealthMonitor({
      origin: ORIGIN,
      intervalMs: 1,
      onDisconnected: async () => {},
      onReconnected: async () => {},
      onLost: async () => {},
    });

    monitor.start();
    try {
      await expect.poll(() => traceparent(0)).toMatch(TRACEPARENT);
    } finally {
      monitor.stop();
    }
  });

  it("Should not send a traceparent to the update feed or other origins", async () => {
    const traceparent = stubFetch();
    const fetcher = daemonFetcher(ORIGIN);

    await fetcher("https://github.com/compozy/compozy/releases/latest/download/latest.yml", {});
    await fetcher("http://127.0.0.1:9999/api/status/identity", {});

    expect(traceparent(0)).toBeNull();
    expect(traceparent(1)).toBeNull();
  });
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

  it("Should retry a failed reconnection on the next healthy probe", async () => {
    vi.useFakeTimers();
    let up = false;
    let attempts = 0;
    const errors: unknown[] = [];
    const monitor = new RuntimeHealthMonitor({
      origin: ORIGIN,
      intervalMs: 2,
      failureThreshold: 1,
      reconnectTimeoutMs: 100,
      fetcher: runtimeFetcher(() => up),
      onDisconnected: async () => {},
      onReconnected: async () => {
        attempts += 1;
        if (attempts === 1) throw new Error("reload raced another drop");
      },
      onLost: async () => {},
      onError: error => void errors.push(error),
    });
    monitor.start();
    try {
      await vi.advanceTimersByTimeAsync(2);
      up = true;
      await vi.advanceTimersByTimeAsync(2);
      expect(attempts).toBe(1);
      expect(errors).toHaveLength(1);
      await vi.advanceTimersByTimeAsync(2);
      expect(attempts).toBe(2);
      await vi.advanceTimersByTimeAsync(10);
      expect(attempts).toBe(2);
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
  function shell(
    steps: string[],
    overrides: Partial<ProductRuntimeShell> = {}
  ): ProductRuntimeShell {
    return {
      stopped: () => false,
      publish: async snapshot =>
        void steps.push(
          `publish:${snapshot.state}${snapshot.error ? `:${snapshot.error.code}` : ""}`
        ),
      reloadProduct: async () => void steps.push("reload"),
      showBoot: () => void steps.push("show-boot"),
      renderBoot: snapshot =>
        void steps.push(`render-boot:${snapshot.state}:${snapshot.error?.code ?? ""}`),
      closeProduct: async () => void steps.push("close-product"),
      resolveRuntime: async () => void steps.push("resolve"),
      logger: { info: () => undefined, error: () => undefined },
      logPath: "/tmp/desktop.log",
      ...overrides,
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

  it("Should surface a retryable error when the product window cannot close for recovery", async () => {
    vi.useFakeTimers();
    const steps: string[] = [];
    const monitor = monitorProductRuntime(
      { origin: ORIGIN, owned: true },
      shell(steps, {
        closeProduct: async () => {
          steps.push("close-product");
          throw new Error("window state write failed");
        },
      }),
      {
        intervalMs: 2,
        failureThreshold: 1,
        reconnectTimeoutMs: 4,
        fetcher: runtimeFetcher(() => false),
      }
    );
    monitor.start();
    try {
      await vi.advanceTimersByTimeAsync(10);
      expect(steps).toEqual([
        "publish:disconnected",
        "publish:disconnected",
        "show-boot",
        "close-product",
        "render-boot:error:runtime_recovery_failed",
        "publish:error:runtime_recovery_failed",
      ]);
    } finally {
      monitor.stop();
    }
  });

  it("Should still hand off to boot when the lost-runtime snapshot cannot be written", async () => {
    vi.useFakeTimers();
    const steps: string[] = [];
    let writes = 0;
    const monitor = monitorProductRuntime(
      { origin: ORIGIN, owned: true },
      shell(steps, {
        publish: async snapshot => {
          writes += 1;
          if (writes === 2) throw new Error("app record write failed");
          steps.push(`publish:${snapshot.state}`);
        },
      }),
      {
        intervalMs: 2,
        failureThreshold: 1,
        reconnectTimeoutMs: 4,
        fetcher: runtimeFetcher(() => false),
      }
    );
    monitor.start();
    try {
      await vi.advanceTimersByTimeAsync(10);
      expect(steps).toEqual(["publish:disconnected", "show-boot", "close-product", "resolve"]);
    } finally {
      monitor.stop();
    }
  });

  it("Should not touch windows when shutdown starts while the lost runtime is published", async () => {
    vi.useFakeTimers();
    const steps: string[] = [];
    let stopping = false;
    let writes = 0;
    const monitor = monitorProductRuntime(
      { origin: ORIGIN, owned: true },
      shell(steps, {
        stopped: () => stopping,
        publish: async snapshot => {
          writes += 1;
          if (writes === 2) stopping = true;
          steps.push(`publish:${snapshot.state}`);
        },
      }),
      {
        intervalMs: 2,
        failureThreshold: 1,
        reconnectTimeoutMs: 4,
        fetcher: runtimeFetcher(() => false),
      }
    );
    monitor.start();
    try {
      await vi.advanceTimersByTimeAsync(10);
      expect(steps).toEqual(["publish:disconnected", "publish:disconnected"]);
    } finally {
      monitor.stop();
    }
  });

  it("Should show the recovery error in the boot window when it cannot be persisted", async () => {
    vi.useFakeTimers();
    const steps: string[] = [];
    const monitor = monitorProductRuntime(
      { origin: ORIGIN, owned: true },
      shell(steps, {
        publish: async snapshot => {
          if (snapshot.state === "error") throw new Error("app record write failed");
          steps.push(`publish:${snapshot.state}`);
        },
        closeProduct: async () => {
          steps.push("close-product");
          throw new Error("window state write failed");
        },
      }),
      {
        intervalMs: 2,
        failureThreshold: 1,
        reconnectTimeoutMs: 4,
        fetcher: runtimeFetcher(() => false),
      }
    );
    monitor.start();
    try {
      await vi.advanceTimersByTimeAsync(10);
      expect(steps.slice(-2)).toEqual([
        "close-product",
        "render-boot:error:runtime_recovery_failed",
      ]);
    } finally {
      monitor.stop();
    }
  });

  it("Should reload the product on reconnect even when its state cannot be written", async () => {
    vi.useFakeTimers();
    let up = false;
    const steps: string[] = [];
    const monitor = monitorProductRuntime(
      { origin: ORIGIN, owned: true },
      shell(steps, {
        publish: async snapshot => {
          if (snapshot.state === "product") throw new Error("app record write failed");
          steps.push(`publish:${snapshot.state}`);
        },
      }),
      {
        intervalMs: 2,
        failureThreshold: 1,
        reconnectTimeoutMs: 100,
        fetcher: runtimeFetcher(() => up),
      }
    );
    monitor.start();
    try {
      await vi.advanceTimersByTimeAsync(2);
      up = true;
      await vi.advanceTimersByTimeAsync(2);
      expect(steps).toEqual(["publish:disconnected", "reload"]);
    } finally {
      monitor.stop();
    }
  });
});
