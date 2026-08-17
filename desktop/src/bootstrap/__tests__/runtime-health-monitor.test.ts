import { describe, expect, it } from "vitest";

import { RuntimeHealthMonitor } from "../runtime-health-monitor";

// Invariant: a product window cannot remain healthy after the runtime becomes unreachable.
describe("runtime health monitor", () => {
  it("Should probe the bounded runtime identity endpoint", async () => {
    const requests: string[] = [];
    const monitor = new RuntimeHealthMonitor({
      origin: "http://127.0.0.1:2123",
      intervalMs: 1,
      fetcher: async input => {
        requests.push(input);
        return new Response(null, { status: 200 });
      },
      onDisconnected: async () => {},
    });

    monitor.start();
    try {
      await expect.poll(() => requests[0]).toBe("http://127.0.0.1:2123/api/status/identity");
    } finally {
      monitor.stop();
    }
  });

  it("Should report one disconnect after the bounded failure threshold", async () => {
    let disconnects = 0;
    const monitor = new RuntimeHealthMonitor({
      origin: "http://127.0.0.1:2123",
      intervalMs: 2,
      timeoutMs: 20,
      failureThreshold: 2,
      fetcher: async () => {
        throw new Error("runtime unavailable");
      },
      onDisconnected: async () => {
        disconnects += 1;
      },
    });
    monitor.start();
    await new Promise(resolve => setTimeout(resolve, 30));
    monitor.stop();
    expect(disconnects).toBe(1);
  });
});
