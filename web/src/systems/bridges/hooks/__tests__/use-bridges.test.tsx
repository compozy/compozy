import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  useBridge,
  useBridgeProviders,
  useBridgeRoutes,
  useBridgeSecretBindings,
  useBridges,
  useSlackBridgeManifest,
} from "@/systems/bridges/hooks/use-bridges";

vi.mock("@/systems/bridges/adapters/bridges-api", () => ({
  createBridge: vi.fn(),
  deleteBridgeSecretBinding: vi.fn(),
  disableBridge: vi.fn(),
  enableBridge: vi.fn(),
  getBridge: vi.fn(),
  listBridgeSecretBindings: vi.fn(),
  listBridgeProviders: vi.fn(),
  listBridgeRoutes: vi.fn(),
  listBridges: vi.fn(),
  putBridgeSecretBinding: vi.fn(),
  restartBridge: vi.fn(),
  testBridgeDelivery: vi.fn(),
  updateBridge: vi.fn(),
}));

vi.mock("@/systems/bridges/adapters/bridge-setup-api", () => ({
  getSlackBridgeManifest: vi.fn(),
  registerBridgeWebhook: vi.fn(),
  sendBridgeTest: vi.fn(),
  verifyBridge: vi.fn(),
}));

import {
  getBridge,
  listBridgeSecretBindings,
  listBridgeProviders,
  listBridgeRoutes,
  listBridges,
} from "@/systems/bridges/adapters/bridges-api";
import { getSlackBridgeManifest } from "@/systems/bridges/adapters/bridge-setup-api";
import type { BridgesListResponse } from "@/systems/bridges/types";

function bridgePage(
  bridges: BridgesListResponse["bridges"] = [],
  page: BridgesListResponse["page"] = { has_more: false, limit: 50, total: bridges.length }
): BridgesListResponse {
  return {
    bridge_health: {},
    bridges,
    facets: {
      platforms: {},
      statuses: {
        auth_required: 0,
        degraded: 0,
        disabled: 0,
        error: 0,
        ready: 0,
        starting: 0,
      },
    },
    page,
  };
}

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

describe("useBridges", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("loads the bridges list", async () => {
    vi.mocked(listBridges).mockResolvedValue(bridgePage());

    const { result } = renderHook(() => useBridges(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.bridges).toEqual([]);
    });

    expect(listBridges).toHaveBeenCalledWith(expect.any(Object), expect.any(AbortSignal));
  });

  it("passes bridge list filters to the adapter", async () => {
    vi.mocked(listBridges).mockResolvedValue(bridgePage());

    const { result } = renderHook(() => useBridges({ scope: "all", workspace_id: "ws_alpha" }), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.bridges).toEqual([]);
    });

    expect(listBridges).toHaveBeenCalledWith(
      expect.objectContaining({ scope: "all", workspace_id: "ws_alpha" }),
      expect.any(AbortSignal)
    );
  });

  it("appends cursor pages and preserves the exact server total", async () => {
    const firstBridge = { id: "brg_1" } as BridgesListResponse["bridges"][number];
    const secondBridge = { id: "brg_2" } as BridgesListResponse["bridges"][number];
    const firstPage = bridgePage([firstBridge], {
      has_more: true,
      limit: 1,
      next_cursor: "cursor-2",
      total: 2,
    });
    firstPage.facets.platforms = { slack: 2 };
    vi.mocked(listBridges)
      .mockResolvedValueOnce(firstPage)
      .mockResolvedValueOnce(bridgePage([secondBridge], { has_more: false, limit: 1, total: 2 }));

    const { result } = renderHook(() => useBridges({ limit: 1, q: "ops" }), {
      wrapper: createWrapper(),
    });
    await waitFor(() => expect(result.current.bridges).toHaveLength(1));
    await result.current.fetchNextPage();
    await waitFor(() =>
      expect(result.current.bridges.map(bridge => bridge.id)).toEqual(["brg_1", "brg_2"])
    );

    expect(result.current.total).toBe(2);
    expect(result.current.facets?.platforms).toEqual({ slack: 2 });
    expect(listBridges).toHaveBeenLastCalledWith(
      expect.objectContaining({ cursor: "cursor-2", limit: 1, q: "ops" }),
      expect.any(AbortSignal)
    );
  });
});

describe("useBridgeProviders", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("loads installed bridge providers", async () => {
    vi.mocked(listBridgeProviders).mockResolvedValue([
      {
        display_name: "Telegram",
        enabled: true,
        extension_name: "ext-telegram",
        health: "healthy",
        platform: "telegram",
        state: "active",
      },
    ]);

    const { result } = renderHook(() => useBridgeProviders(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.data).toHaveLength(1);
    });

    expect(listBridgeProviders).toHaveBeenCalledWith(expect.any(AbortSignal));
  });
});

describe("useSlackBridgeManifest", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("loads the manifest only for a persisted nonblank bridge instance", async () => {
    vi.mocked(getSlackBridgeManifest).mockResolvedValue({
      manifest: {
        _metadata: { major_version: 1, minor_version: 1 },
        display_information: {
          background_color: "#E8572A",
          description: "AGH Slack bridge",
          name: "AGH",
        },
        features: {
          bot_user: { always_online: false, display_name: "AGH" },
          slash_commands: [],
        },
        oauth_config: { scopes: { bot: ["app_mentions:read"] } },
        settings: {
          event_subscriptions: {
            bot_events: ["app_mention"],
            request_url: "https://bridge.example.test/slack",
          },
          interactivity: {
            is_enabled: true,
            request_url: "https://bridge.example.test/slack",
          },
          org_deploy_enabled: false,
          socket_mode_enabled: false,
          token_rotation_enabled: false,
        },
      },
    });

    const { result } = renderHook(() => useSlackBridgeManifest(" brg_slack "), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.data?.manifest.display_information.name).toBe("AGH");
    });
    expect(getSlackBridgeManifest).toHaveBeenCalledWith("brg_slack", expect.any(AbortSignal));
  });

  it("does not fetch a manifest without a persisted bridge instance", () => {
    renderHook(() => useSlackBridgeManifest("   "), {
      wrapper: createWrapper(),
    });

    expect(getSlackBridgeManifest).not.toHaveBeenCalled();
  });
});

describe("useBridge", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("loads a selected bridge detail", async () => {
    vi.mocked(getBridge).mockResolvedValue({
      bridge: {
        created_at: "2026-04-13T12:00:00Z",
        display_name: "Support",
        enabled: true,
        extension_name: "ext-telegram",
        id: "brg_support",
        notification_suppress: false,
        platform: "telegram",
        routing_policy: { include_group: true, include_peer: true, include_thread: true },
        scope: "workspace",
        status: "ready",
        updated_at: "2026-04-13T12:30:00Z",
        workspace_id: "ws_test",
      },
      health: {
        auth_failures_total: 0,
        bridge_instance_id: "brg_support",
        delivery_backlog: 0,
        delivery_dropped_total: 0,
        delivery_failures_total: 0,
        route_count: 1,
        status: "ready",
      },
    });

    const { result } = renderHook(() => useBridge("brg_support"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.data?.bridge.id).toBe("brg_support");
    });

    expect(getBridge).toHaveBeenCalledWith("brg_support", expect.any(AbortSignal));
  });

  it("does not fetch when bridge id is empty", () => {
    renderHook(() => useBridge(""), {
      wrapper: createWrapper(),
    });

    expect(getBridge).not.toHaveBeenCalled();
  });
});

describe("useBridgeRoutes", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("loads routes for the selected bridge", async () => {
    vi.mocked(listBridgeRoutes).mockResolvedValue([
      {
        agent_name: "support-agent",
        bridge_instance_id: "brg_support",
        created_at: "2026-04-13T12:00:00Z",
        last_activity_at: "2026-04-13T12:10:00Z",
        peer_id: "peer_123",
        routing_key_hash: "abc123",
        scope: "workspace",
        session_id: "sess_123",
        updated_at: "2026-04-13T12:10:00Z",
        workspace_id: "ws_test",
      },
    ]);

    const { result } = renderHook(() => useBridgeRoutes("brg_support"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.data).toHaveLength(1);
    });

    expect(listBridgeRoutes).toHaveBeenCalledWith("brg_support", expect.any(AbortSignal));
  });
});

describe("useBridgeSecretBindings", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("loads secret bindings for the selected bridge", async () => {
    vi.mocked(listBridgeSecretBindings).mockResolvedValue([
      {
        binding_name: "bot_token",
        bridge_instance_id: "brg_support",
        created_at: "2026-04-13T12:00:00Z",
        kind: "bot_token",
        updated_at: "2026-04-13T12:10:00Z",
        secret_ref: "vault:bridges/brg_support/bot_token",
      },
    ]);

    const { result } = renderHook(() => useBridgeSecretBindings("brg_support"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.data).toHaveLength(1);
    });

    expect(listBridgeSecretBindings).toHaveBeenCalledWith("brg_support", expect.any(AbortSignal));
  });
});
