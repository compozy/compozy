import { afterEach, describe, expect, it } from "vitest";

import { installFetchStub, matchPath } from "@/test/utils";

import { fetchDashboard } from "./dashboard-api";

const dashboardPayload = {
  dashboard: {
    workspace: {
      id: "ws-1",
      name: "one",
      root_dir: "/tmp/one",
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    },
    daemon: {
      active_run_count: 0,
      pid: 42,
      started_at: "2026-01-01T00:00:00Z",
      workspace_count: 1,
    },
    health: { ready: true, degraded: false, details: [] },
    pending_reviews: 0,
    queue: { active: 0, canceled: 0, completed: 0, failed: 0, total: 0 },
    workflows: [],
  },
};

describe("fetchDashboard", () => {
  let restore: (() => void) | null = null;

  afterEach(() => {
    restore?.();
    restore = null;
  });

  it("Should send the active workspace header and return the dashboard payload", async () => {
    const stub = installFetchStub([
      { matcher: matchPath("/api/ui/dashboard"), status: 200, body: dashboardPayload },
    ]);
    restore = stub.restore;
    const result = await fetchDashboard("ws-1");
    expect(result.pending_reviews).toBe(0);
    expect(stub.calls[0]?.headers["x-compozy-workspace-id"]).toBe("ws-1");
  });

  it("Should surface stale-workspace errors", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/ui/dashboard"),
        status: 412,
        body: {
          code: "workspace_context_stale",
          message: "stale",
          request_id: "req-1",
        },
      },
    ]);
    restore = stub.restore;
    await expect(fetchDashboard("ws-99")).rejects.toThrow(/stale/);
  });
});
