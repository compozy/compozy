import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest, mockJsonResponse } from "@/test/fetch-test-utils";
import {
  BridgesApiError,
  createBridge,
  deleteBridgeSecretBinding,
  disableBridge,
  enableBridge,
  getBridge,
  listBridgeSecretBindings,
  listBridgeProviders,
  listBridgeRoutes,
  listBridgeTargets,
  listBridges,
  putBridgeSecretBinding,
  restartBridge,
  resolveBridgeTarget,
  testBridgeDelivery,
  updateBridge,
} from "@/systems/bridges/adapters/bridges-api";
import {
  getSlackBridgeManifest,
  registerBridgeWebhook,
  sendBridgeTest,
  verifyBridge,
} from "@/systems/bridges/adapters/bridge-setup-api";

const bridgeFixture = {
  created_at: "2026-04-13T12:00:00Z",
  dm_policy: "open",
  display_name: "Support",
  enabled: true,
  extension_name: "ext-telegram",
  id: "brg_support",
  platform: "telegram",
  provider_config: {
    mode: "bot",
  },
  routing_policy: {
    include_group: true,
    include_peer: true,
    include_thread: true,
  },
  scope: "workspace" as const,
  status: "ready" as const,
  updated_at: "2026-04-13T12:30:00Z",
  workspace_id: "ws_test",
};

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("listBridges", () => {
  it("calls GET /api/bridges and returns the typed payload", async () => {
    mockJsonResponse({
      bridge_health: {
        brg_support: {
          auth_failures_total: 0,
          bridge_instance_id: "brg_support",
          delivery_backlog: 1,
          delivery_dropped_total: 0,
          delivery_failures_total: 0,
          route_count: 2,
          status: "ready",
        },
      },
      bridges: [bridgeFixture],
      facets: {
        platforms: { telegram: 1 },
        statuses: {
          auth_required: 0,
          degraded: 0,
          disabled: 0,
          error: 0,
          ready: 1,
          starting: 0,
        },
      },
      page: { has_more: false, limit: 50, total: 1 },
    });

    const result = await listBridges();

    expect(result.bridges).toEqual([bridgeFixture]);
    expect(result.page).toEqual({ has_more: false, limit: 50, total: 1 });
    expect(result.facets.platforms).toEqual({ telegram: 1 });
    await expectFetchRequest({ path: "/api/bridges" });
  });

  it("sends every bridge catalog filter and cursor", async () => {
    mockJsonResponse({
      bridge_health: {},
      bridges: [],
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
      page: { has_more: false, limit: 25, total: 0 },
    });

    await listBridges({
      cursor: " cursor-2 ",
      limit: 25,
      platform: " slack ",
      q: " ops ",
      scope: "all",
      sort: "name",
      status: "ready",
      workspace_id: " ws_alpha ",
    });

    await expectFetchRequest({
      path: "/api/bridges?scope=all&workspace_id=ws_alpha&q=ops&platform=slack&status=ready&sort=name&cursor=cursor-2&limit=25",
    });
  });

  it("passes abort signal through to fetch", async () => {
    mockJsonResponse({ bridges: [] });

    const controller = new AbortController();
    await listBridges({}, controller.signal);

    await expectFetchRequest({
      path: "/api/bridges",
      signal: controller.signal,
    });
  });

  it("throws BridgesApiError on non-2xx response", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 503 }));

    await expect(listBridges()).rejects.toThrow(BridgesApiError);
    await expect(listBridges()).rejects.toThrow("Failed to fetch bridges: 503");
  });
});

describe("listBridgeProviders", () => {
  it("calls GET /api/bridges/providers", async () => {
    mockJsonResponse({
      providers: [
        {
          config_schema: {
            schema: "provider-config",
            version: "2026-04-15",
          },
          display_name: "Telegram",
          enabled: true,
          extension_name: "ext-telegram",
          health: "healthy",
          platform: "telegram",
          secret_slots: [
            {
              description: "Bot token",
              name: "bot_token",
              required: true,
            },
          ],
          state: "active",
        },
      ],
    });

    const result = await listBridgeProviders();

    expect(result).toEqual([
      expect.objectContaining({
        display_name: "Telegram",
        extension_name: "ext-telegram",
        secret_slots: [
          {
            description: "Bot token",
            name: "bot_token",
            required: true,
          },
        ],
      }),
    ]);
    await expectFetchRequest({ path: "/api/bridges/providers" });
  });

  it("throws BridgesApiError when provider lookup fails", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 503 }));

    await expect(listBridgeProviders()).rejects.toThrow(BridgesApiError);
    await expect(listBridgeProviders()).rejects.toThrow("Failed to fetch bridge providers: 503");
  });
});

describe("getBridge", () => {
  it("calls GET /api/bridges/:id", async () => {
    mockJsonResponse({
      bridge: bridgeFixture,
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

    const result = await getBridge("brg_support");

    expect(result.bridge).toEqual(bridgeFixture);
    await expectFetchRequest({ path: "/api/bridges/brg_support" });
  });

  it("throws a not found error for unknown bridges", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(getBridge("missing")).rejects.toThrow("Bridge not found: missing");
  });

  it("throws a typed error for non-404 bridge fetch failures", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));

    await expect(getBridge("brg_support")).rejects.toThrow(
      'Failed to load bridge "brg_support": 500'
    );
  });
});

describe("listBridgeRoutes", () => {
  it("calls GET /api/bridges/:id/routes", async () => {
    mockJsonResponse({
      routes: [
        {
          agent_name: "support-agent",
          bridge_instance_id: "brg_support",
          created_at: "2026-04-13T12:00:00Z",
          last_activity_at: "2026-04-13T12:15:00Z",
          peer_id: "peer_123",
          routing_key_hash: "abc123",
          scope: "workspace",
          session_id: "sess_123",
          updated_at: "2026-04-13T12:15:00Z",
          workspace_id: "ws_test",
        },
      ],
    });

    const result = await listBridgeRoutes("brg_support");

    expect(result).toHaveLength(1);
    await expectFetchRequest({ path: "/api/bridges/brg_support/routes" });
  });

  it("throws a not found error for missing route sets", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(listBridgeRoutes("missing")).rejects.toThrow("Bridge not found: missing");
  });
});

describe("listBridgeTargets", () => {
  it("calls GET /api/bridges/:id/targets with filters", async () => {
    mockJsonResponse({
      bridge_id: "brg_support",
      cache_stale: false,
      generated_at: "2026-05-21T12:00:00Z",
      last_successful_refresh_at: "2026-05-21T11:58:00Z",
      targets: [
        {
          bridge_id: "brg_support",
          canonical_route: "slack://T123/C456",
          capabilities: ["send", "thread"],
          display_name: "Launch Room",
          last_seen_at: "2026-05-21T11:58:00Z",
          normalized: "launch room",
          qualifier: "northstar",
          target_type: "channel",
          updated_at: "2026-05-21T11:58:00Z",
        },
      ],
      total: 1,
    });

    const result = await listBridgeTargets("brg_support", { limit: 25, q: " launch " });

    expect(result.targets).toHaveLength(1);
    expect(result.targets[0]?.canonical_route).toBe("slack://T123/C456");
    await expectFetchRequest({ path: "/api/bridges/brg_support/targets?q=launch&limit=25" });
  });
});

describe("resolveBridgeTarget", () => {
  it("calls POST /api/bridges/:id/resolve and returns the match", async () => {
    mockJsonResponse({
      result: {
        ambiguous: false,
        match: {
          bridge_id: "brg_support",
          canonical_route: "slack://T123/C456",
          capabilities: ["send"],
          display_name: "Launch Room",
          normalized: "launch room",
          qualifier: "northstar",
          target_type: "channel",
          updated_at: "2026-05-21T11:58:00Z",
        },
        step: 2,
      },
    });

    const payload = { name: "Launch Room" };
    const result = await resolveBridgeTarget("brg_support", payload);

    expect(result.result.match?.canonical_route).toBe("slack://T123/C456");
    await expectFetchRequest({
      body: payload,
      method: "POST",
      path: "/api/bridges/brg_support/resolve",
    });
  });

  it("returns ambiguous diagnostics without throwing", async () => {
    mockJsonResponse(
      {
        diagnostic: {
          category: "bridge",
          code: "target_ambiguous",
          data_freshness: "fresh",
          id: "diag_target_ambiguous",
          message: "Target lookup matched multiple candidates.",
          severity: "warning",
          title: "Target lookup is ambiguous",
        },
        result: {
          ambiguous: true,
          candidates: [
            {
              bridge_id: "brg_support",
              canonical_route: "slack://T123/C456",
              capabilities: ["send"],
              display_name: "Launch Room",
              normalized: "launch room",
              qualifier: "northstar",
              target_type: "channel",
              updated_at: "2026-05-21T11:58:00Z",
            },
          ],
          step: 4,
        },
      },
      { status: 422 }
    );

    const result = await resolveBridgeTarget("brg_support", { name: "launch" });

    expect(result.result.ambiguous).toBe(true);
    expect(result.diagnostic?.code).toBe("target_ambiguous");
  });
});

describe("createBridge", () => {
  it("calls POST /api/bridges with the create payload", async () => {
    mockJsonResponse(
      {
        bridge: bridgeFixture,
        health: {
          auth_failures_total: 0,
          bridge_instance_id: "brg_support",
          delivery_backlog: 0,
          delivery_dropped_total: 0,
          delivery_failures_total: 0,
          route_count: 0,
          status: "starting",
        },
      },
      { status: 201 }
    );

    const payload = {
      dm_policy: "open" as const,
      display_name: "Support",
      enabled: true,
      extension_name: "ext-telegram",
      notification_suppress: false,
      platform: "telegram",
      provider_config: {
        mode: "bot",
      },
      routing_policy: {
        include_group: true,
        include_peer: true,
        include_thread: true,
      },
      scope: "workspace" as const,
      workspace_id: "ws_test",
    };

    const result = await createBridge(payload);

    expect(result.bridge).toEqual(bridgeFixture);
    await expectFetchRequest({
      body: payload,
      method: "POST",
      path: "/api/bridges",
    });
  });

  it("throws BridgesApiError when bridge creation fails", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 400 }));

    await expect(
      createBridge({
        display_name: "Support",
        enabled: true,
        extension_name: "ext-telegram",
        notification_suppress: false,
        platform: "telegram",
        routing_policy: {
          include_group: true,
          include_peer: true,
          include_thread: true,
        },
        scope: "workspace",
        workspace_id: "ws_test",
      })
    ).rejects.toThrow("Failed to create bridge: 400");
  });
});

describe("updateBridge", () => {
  it("calls PATCH /api/bridges/:id with the update payload", async () => {
    mockJsonResponse({
      bridge: {
        ...bridgeFixture,
        display_name: "Support Ops",
      },
      health: {
        auth_failures_total: 0,
        bridge_instance_id: "brg_support",
        delivery_backlog: 0,
        delivery_dropped_total: 0,
        delivery_failures_total: 0,
        route_count: 0,
        status: "ready",
      },
    });

    const payload = {
      delivery_defaults: {
        peer_id: "peer_default",
      },
      display_name: "Support Ops",
      dm_policy: "allowlist" as const,
      provider_config: {
        mode: "bot",
      },
      routing_policy: {
        include_group: true,
        include_peer: false,
        include_thread: true,
      },
    };

    const result = await updateBridge("brg_support", payload);

    expect(result.bridge.display_name).toBe("Support Ops");
    await expectFetchRequest({
      body: payload,
      method: "PATCH",
      path: "/api/bridges/brg_support",
    });
  });
});

describe("listBridgeSecretBindings", () => {
  it("calls GET /api/bridges/:id/secret-bindings", async () => {
    mockJsonResponse({
      bindings: [
        {
          binding_name: "bot_token",
          bridge_instance_id: "brg_support",
          created_at: "2026-04-13T12:00:00Z",
          kind: "bot_token",
          updated_at: "2026-04-13T12:00:00Z",
          secret_ref: "vault:bridges/brg_support/bot_token",
        },
      ],
    });

    const result = await listBridgeSecretBindings("brg_support");

    expect(result).toHaveLength(1);
    await expectFetchRequest({ path: "/api/bridges/brg_support/secret-bindings" });
  });
});

describe("putBridgeSecretBinding", () => {
  it("calls PUT /api/bridges/:id/secret-bindings/:binding_name", async () => {
    mockJsonResponse({
      binding: {
        binding_name: "bot_token",
        bridge_instance_id: "brg_support",
        created_at: "2026-04-13T12:00:00Z",
        kind: "bot_token",
        updated_at: "2026-04-13T12:30:00Z",
        secret_ref: "vault:bridges/brg_support/bot_token",
      },
    });

    const payload = {
      kind: "bot_token",
      secret_ref: "vault:bridges/brg_support/bot_token",
      secret_value: "telegram-token",
    };

    const result = await putBridgeSecretBinding("brg_support", "bot_token", payload);

    expect(result.secret_ref).toBe("vault:bridges/brg_support/bot_token");
    await expectFetchRequest({
      body: payload,
      method: "PUT",
      path: "/api/bridges/brg_support/secret-bindings/bot_token",
    });
  });
});

describe("deleteBridgeSecretBinding", () => {
  it("calls DELETE /api/bridges/:id/secret-bindings/:binding_name", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 204 }));

    await deleteBridgeSecretBinding("brg_support", "bot_token");

    await expectFetchRequest({
      method: "DELETE",
      path: "/api/bridges/brg_support/secret-bindings/bot_token",
    });
  });
});

describe("bridge lifecycle", () => {
  it("calls the enable, disable, and restart endpoints", async () => {
    mockJsonResponse({
      bridge: bridgeFixture,
      health: {
        auth_failures_total: 0,
        bridge_instance_id: "brg_support",
        delivery_backlog: 0,
        delivery_dropped_total: 0,
        delivery_failures_total: 0,
        route_count: 0,
        status: "starting",
      },
    });
    await enableBridge("brg_support");
    await expectFetchRequest({
      callIndex: 0,
      method: "POST",
      path: "/api/bridges/brg_support/enable",
    });

    mockJsonResponse({
      bridge: bridgeFixture,
      health: {
        auth_failures_total: 0,
        bridge_instance_id: "brg_support",
        delivery_backlog: 0,
        delivery_dropped_total: 0,
        delivery_failures_total: 0,
        route_count: 0,
        status: "disabled",
      },
    });
    await disableBridge("brg_support");
    await expectFetchRequest({
      callIndex: 1,
      method: "POST",
      path: "/api/bridges/brg_support/disable",
    });

    mockJsonResponse({
      bridge: bridgeFixture,
      health: {
        auth_failures_total: 0,
        bridge_instance_id: "brg_support",
        delivery_backlog: 0,
        delivery_dropped_total: 0,
        delivery_failures_total: 0,
        route_count: 0,
        status: "starting",
      },
    });
    await restartBridge("brg_support");
    await expectFetchRequest({
      callIndex: 2,
      method: "POST",
      path: "/api/bridges/brg_support/restart",
    });
  });
});

describe("testBridgeDelivery", () => {
  it("calls POST /api/bridges/:id/test-delivery with the typed target payload", async () => {
    mockJsonResponse({
      delivery_target: {
        bridge_instance_id: "brg_support",
        mode: "reply",
        peer_id: "peer_123",
      },
      message: "Ping",
      status: "resolved",
    });

    const payload = {
      message: "Ping",
      target: {
        bridge_instance_id: "brg_support",
        mode: "reply" as const,
        peer_id: "peer_123",
      },
    };

    const result = await testBridgeDelivery("brg_support", payload);

    expect(result.status).toBe("resolved");
    await expectFetchRequest({
      body: payload,
      method: "POST",
      path: "/api/bridges/brg_support/test-delivery",
    });
  });

  it("throws a bridge unavailable error on 409", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 409 }));

    await expect(
      testBridgeDelivery("brg_support", {
        target: {
          bridge_instance_id: "brg_support",
          peer_id: "peer_123",
        },
      })
    ).rejects.toThrow('Bridge "brg_support" is unavailable: 409');
  });

  it("throws a generic typed error for other delivery failures", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));

    await expect(
      testBridgeDelivery("brg_support", {
        target: {
          bridge_instance_id: "brg_support",
        },
      })
    ).rejects.toThrow('Failed to test delivery for bridge "brg_support": 500');
  });
});

describe("getSlackBridgeManifest", () => {
  it("fetches the instance-derived Slack manifest and propagates cancellation", async () => {
    mockJsonResponse({
      manifest: {
        _metadata: { major_version: 1, minor_version: 1 },
        display_information: {
          background_color: "#E8572A",
          description: "AGH Slack bridge",
          name: "AGH",
        },
        features: {
          bot_user: { always_online: false, display_name: "AGH" },
          slash_commands: [
            {
              command: "/agh",
              description: "Message AGH",
              should_escape: false,
              url: "https://bridge.example.test/slack",
              usage_hint: "<message>",
            },
          ],
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
    const controller = new AbortController();

    const result = await getSlackBridgeManifest(" brg_slack ", controller.signal);

    expect(result.manifest.display_information.name).toBe("AGH");
    await expectFetchRequest({
      path: "/api/bridges/providers/slack/manifest?instance=brg_slack",
      signal: controller.signal,
    });
  });

  it("throws a typed error when the persisted Slack instance is unavailable", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(getSlackBridgeManifest("missing")).rejects.toMatchObject({
      message: 'Failed to load Slack manifest for bridge "missing": 404',
      name: "BridgesApiError",
      status: 404,
    });
  });
});

describe("verifyBridge", () => {
  it("posts the provider verification request and preserves structured checks", async () => {
    mockJsonResponse({
      bridge_instance_id: "brg_support",
      checks: [
        { check: "provider.identity", remediation: "", status: "pass" },
        {
          check: "webhook.reachability",
          remediation: "Enable the bridge, then run bridge verification again.",
          status: "skipped",
        },
      ],
    });
    const controller = new AbortController();

    const result = await verifyBridge(" brg_support ", controller.signal);

    expect(result.checks.map(check => check.status)).toEqual(["pass", "skipped"]);
    await expectFetchRequest({
      method: "POST",
      path: "/api/bridges/brg_support/verify",
      signal: controller.signal,
    });
  });

  it("throws a typed error when provider verification is unavailable", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 503 }));

    await expect(verifyBridge("brg_support")).rejects.toMatchObject({
      message: 'Failed to verify bridge "brg_support": 503',
      name: "BridgesApiError",
      status: 503,
    });
  });
});

describe("sendBridgeTest", () => {
  it("posts a real provider delivery with the typed message and target", async () => {
    mockJsonResponse({
      bridge_instance_id: "brg_support",
      delivery_id: "delivery_123",
      delivery_target: {
        bridge_instance_id: "brg_support",
        mode: "direct-send",
        peer_id: "peer_123",
      },
      remote_message_id: "remote_123",
      status: "delivered",
    });
    const payload = {
      message: "Operator test",
      target: {
        bridge_instance_id: "brg_support",
        mode: "direct-send" as const,
        peer_id: "peer_123",
      },
    };
    const controller = new AbortController();

    const result = await sendBridgeTest(" brg_support ", payload, controller.signal);

    expect(result.delivery_id).toBe("delivery_123");
    await expectFetchRequest({
      body: payload,
      method: "POST",
      path: "/api/bridges/brg_support/send-test",
      signal: controller.signal,
    });
  });

  it("throws a typed error when the real delivery path is unavailable", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 409 }));

    await expect(
      sendBridgeTest("brg_support", {
        message: "Operator test",
        target: { bridge_instance_id: "brg_support" },
      })
    ).rejects.toMatchObject({
      message: 'Failed to send a test through bridge "brg_support": 409',
      name: "BridgesApiError",
      status: 409,
    });
  });
});

describe("registerBridgeWebhook", () => {
  it("posts webhook registration and preserves the provider result", async () => {
    mockJsonResponse({
      bridge_instance_id: "brg_telegram",
      remediation: "",
      status: "pass",
    });
    const controller = new AbortController();

    const result = await registerBridgeWebhook(" brg_telegram ", controller.signal);

    expect(result.status).toBe("pass");
    await expectFetchRequest({
      method: "POST",
      path: "/api/bridges/brg_telegram/webhook/register",
      signal: controller.signal,
    });
  });

  it("throws a typed error when webhook registration fails at the API boundary", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));

    await expect(registerBridgeWebhook("brg_telegram")).rejects.toMatchObject({
      message: 'Failed to register the webhook for bridge "brg_telegram": 500',
      name: "BridgesApiError",
      status: 500,
    });
  });
});
