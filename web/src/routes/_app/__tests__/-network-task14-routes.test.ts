import { QueryClient } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";

import { Route as NetworkRootRoute } from "../network";
import { Route as ThreadsRoute } from "../network.$workspaceId.$channel.threads";
import { Route as ThreadDetailRoute } from "../network.$workspaceId.$channel.threads.$threadId";
import { Route as DirectsRoute } from "../network.$workspaceId.$channel.directs";
import { Route as DirectDetailRoute } from "../network.$workspaceId.$channel.directs.$directId";
import { Route as ActivityRoute } from "../network.$workspaceId.$channel.activity";
import { preloadNetworkThreadDetailRoute } from "../-network-preload";
import { networkThreadDetailOptions, networkThreadMessagesOptions } from "@/systems/network";

const adapterMocks = vi.hoisted(() => ({
  getNetworkStatus: vi.fn().mockResolvedValue({ enabled: true }),
  listNetworkChannels: vi.fn().mockResolvedValue({ channels: [], recents: [] }),
  getNetworkChannel: vi.fn().mockResolvedValue({ channel: "ops", peers: [] }),
  getNetworkThread: vi.fn().mockResolvedValue({
    channel: "ops",
    message_count: 1,
    open_work_count: 0,
    participant_count: 1,
    root_message_id: "root",
    thread_id: "thread-url",
  }),
  listNetworkThreadMessages: vi.fn().mockResolvedValue({
    messages: [],
    page: { has_more: false, limit: 120 },
  }),
}));

vi.mock("../../../systems/network/adapters/network-api", async () => ({
  ...(await vi.importActual("../../../systems/network/adapters/network-api")),
  ...adapterMocks,
}));

interface RouteShape {
  options?: {
    component?: unknown;
    loader?: unknown;
    validateSearch?: (search: Record<string, unknown>) => unknown;
  };
}

describe("network task_14 route components", () => {
  it("Should bind thread detail route to a component", () => {
    const route = ThreadDetailRoute as RouteShape;
    expect(typeof route.options?.component).toBe("function");
  });

  it("Should bind direct detail route to a component", () => {
    const route = DirectDetailRoute as RouteShape;
    expect(typeof route.options?.component).toBe("function");
  });

  it("Should bind activity tab route to a component", () => {
    const route = ActivityRoute as RouteShape;
    expect(typeof route.options?.component).toBe("function");
  });

  it("Should preload every Network leaf from its URL-scoped workspace", () => {
    for (const route of [
      NetworkRootRoute,
      ThreadsRoute,
      ThreadDetailRoute,
      DirectsRoute,
      DirectDetailRoute,
      ActivityRoute,
    ] as RouteShape[]) {
      expect(typeof route.options?.loader).toBe("function");
    }
  });

  it("Should reuse the exact URL-scoped preload cache on mount without a duplicate read", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    await preloadNetworkThreadDetailRoute(queryClient, "ws-url", "ops", "thread-url");
    await Promise.all([
      queryClient.ensureQueryData(networkThreadDetailOptions("ws-url", "ops", "thread-url")),
      queryClient.ensureInfiniteQueryData(
        networkThreadMessagesOptions("ws-url", "ops", "thread-url")
      ),
    ]);

    expect(adapterMocks.getNetworkThread).toHaveBeenCalledTimes(1);
    expect(adapterMocks.getNetworkThread).toHaveBeenCalledWith(
      "ws-url",
      "ops",
      "thread-url",
      expect.any(AbortSignal)
    );
    expect(adapterMocks.listNetworkThreadMessages).toHaveBeenCalledTimes(1);
    expect(adapterMocks.listNetworkThreadMessages).toHaveBeenCalledWith(
      "ws-url",
      "ops",
      "thread-url",
      { limit: 120 },
      expect.any(AbortSignal)
    );
  });

  it("Should validate `view=full` search param on the thread detail route", () => {
    const route = ThreadDetailRoute as RouteShape;
    const validate = route.options?.validateSearch;
    expect(typeof validate).toBe("function");
    if (typeof validate === "function") {
      expect(validate({ view: "full" })).toEqual({ view: "full" });
      expect(validate({ view: "anything-else" })).toEqual({ view: undefined });
      expect(validate({})).toEqual({ view: undefined });
    }
  });
});
