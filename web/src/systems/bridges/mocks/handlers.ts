import { HttpResponse, type HttpHandler } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";

import {
  bridgeDetailFixture,
  bridgeProvidersFixture,
  bridgeResolveTargetFixture,
  bridgeRoutesFixture,
  bridgeSecretBindingsFixture,
  bridgeTargetsFixture,
  bridgeVerifyFixture,
  bridgesListFixture,
  createBridgeFixture,
  sendBridgeTestFixture,
  slackBridgeManifestFixture,
  testBridgeDeliveryFixture,
  updateBridgeFixture,
} from "./fixtures";

const bridgeHealthStreamEncoder = new TextEncoder();

function createBridgeHealthStreamResponse(): Response {
  const snapshot = {
    bridge_health: bridgesListFixture.bridge_health ?? {},
    generated_at: "2026-04-17T18:10:00Z",
  };
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      controller.enqueue(
        bridgeHealthStreamEncoder.encode(`event: snapshot\ndata: ${JSON.stringify(snapshot)}\n\n`)
      );
    },
  });

  return new Response(stream, {
    status: 200,
    headers: {
      "Cache-Control": "no-cache",
      Connection: "keep-alive",
      "Content-Type": "text/event-stream",
    },
  });
}

export const handlers: HttpHandler[] = [
  aghApiMock.get("/api/bridges", () => HttpResponse.json(bridgesListFixture)),
  aghApiMock.get("/api/bridges/providers", () =>
    HttpResponse.json({ providers: bridgeProvidersFixture })
  ),
  aghApiMock.get("/api/bridges/providers/slack/manifest", ({ request }) => {
    const instanceID = new URL(request.url).searchParams.get("instance");
    if (instanceID !== bridgeDetailFixture.bridge.id) {
      return HttpResponse.json({ error: "Slack bridge instance not found" }, { status: 404 });
    }

    return HttpResponse.json(slackBridgeManifestFixture);
  }),
  aghApiMock.get("/api/bridges/health/stream", ({ response }) =>
    response.untyped(createBridgeHealthStreamResponse())
  ),
  aghApiMock.get("/api/bridges/{id}", ({ params }) => {
    const id = String(params.id);

    if (id !== bridgeDetailFixture.bridge.id) {
      return HttpResponse.json({ error: `Bridge not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json(bridgeDetailFixture);
  }),
  aghApiMock.get("/api/bridges/{id}/routes", ({ params }) => {
    const id = String(params.id);

    if (id !== bridgeDetailFixture.bridge.id) {
      return HttpResponse.json({ error: `Bridge not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json({ routes: bridgeRoutesFixture });
  }),
  aghApiMock.get("/api/bridges/{id}/targets", ({ params, request }) => {
    const id = String(params.id);

    if (id !== bridgeDetailFixture.bridge.id) {
      return HttpResponse.json({ error: `Bridge not found: ${id}` }, { status: 404 });
    }

    const url = new URL(request.url);
    const query = url.searchParams.get("q")?.trim().toLowerCase();
    const targets = query
      ? bridgeTargetsFixture.targets.filter(
          target =>
            target.display_name.toLowerCase().includes(query) ||
            target.canonical_route.toLowerCase().includes(query) ||
            target.qualifier?.toLowerCase().includes(query)
        )
      : bridgeTargetsFixture.targets;

    return HttpResponse.json({
      ...bridgeTargetsFixture,
      targets,
      total: targets.length,
    });
  }),
  aghApiMock.post("/api/bridges/{id}/resolve", async ({ params, request }) => {
    const id = String(params.id);

    if (id !== bridgeDetailFixture.bridge.id) {
      return HttpResponse.json({ error: `Bridge not found: ${id}` }, { status: 404 });
    }

    const body = (await request.json()) as { name?: string };
    const query = body.name?.trim().toLowerCase() ?? "";
    if (query === "launch" || query === "launch room") {
      return HttpResponse.json(bridgeResolveTargetFixture);
    }
    if (query === "merchant") {
      return HttpResponse.json(
        {
          diagnostic: {
            category: "bridge",
            code: "target_ambiguous",
            data_freshness: "live",
            evidence: {
              candidates: 2,
              query,
            },
            id: "bridge_target_resolve:brg_launch_room",
            message: 'Bridge target "merchant" matched 2 candidates',
            severity: "warn",
            title: "Bridge target is ambiguous",
          },
          result: {
            ambiguous: true,
            candidates: bridgeTargetsFixture.targets,
            step: 4,
          },
        },
        { status: 422 }
      );
    }

    return HttpResponse.json(
      {
        diagnostic: {
          category: "bridge",
          code: "target_unknown",
          data_freshness: "live",
          evidence: {
            candidates: 0,
            query,
          },
          id: "bridge_target_resolve:brg_launch_room",
          message: `Bridge target "${query}" could not be resolved`,
          severity: "warn",
          title: "Bridge target resolution failed",
        },
        result: {
          ambiguous: false,
          candidates: [],
          match: null,
          step: 0,
        },
      },
      { status: 404 }
    );
  }),
  aghApiMock.get("/api/bridges/{id}/secret-bindings", ({ params }) => {
    const id = String(params.id);

    if (id !== bridgeDetailFixture.bridge.id) {
      return HttpResponse.json({ error: `Bridge not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json({ bindings: bridgeSecretBindingsFixture });
  }),
  aghApiMock.post("/api/bridges", async ({ request }) => {
    const body = (await request.json()) as {
      delivery_defaults?: (typeof createBridgeFixture.bridge)["delivery_defaults"];
      display_name?: string;
      enabled?: boolean;
      provider_config?: (typeof createBridgeFixture.bridge)["provider_config"];
      scope?: "global" | "workspace";
      workspace_id?: string;
    };

    return HttpResponse.json(
      {
        ...createBridgeFixture,
        bridge: {
          ...createBridgeFixture.bridge,
          delivery_defaults: body.delivery_defaults,
          display_name: body.display_name?.trim() || createBridgeFixture.bridge.display_name,
          enabled: body.enabled ?? false,
          provider_config: body.provider_config,
          scope: body.scope ?? createBridgeFixture.bridge.scope,
          status: body.enabled ? createBridgeFixture.bridge.status : "disabled",
          workspace_id: body.workspace_id ?? createBridgeFixture.bridge.workspace_id,
        },
        health: {
          ...createBridgeFixture.health,
          status: body.enabled ? createBridgeFixture.health.status : "disabled",
        },
      },
      { status: 201 }
    );
  }),
  aghApiMock.patch("/api/bridges/{id}", async ({ params, request }) => {
    const id = String(params.id);

    if (id !== bridgeDetailFixture.bridge.id) {
      return HttpResponse.json({ error: `Bridge not found: ${id}` }, { status: 404 });
    }

    const body = (await request.json()) as {
      display_name?: string | null;
      dm_policy?: "open" | "allowlist" | "pairing";
      delivery_defaults?: (typeof updateBridgeFixture.bridge)["delivery_defaults"] | null;
      provider_config?: Record<string, unknown> | null;
      routing_policy?: {
        include_group: boolean;
        include_peer: boolean;
        include_thread: boolean;
      } | null;
    };

    return HttpResponse.json({
      ...updateBridgeFixture,
      bridge: {
        ...updateBridgeFixture.bridge,
        delivery_defaults: body.delivery_defaults ?? undefined,
        display_name: body.display_name ?? updateBridgeFixture.bridge.display_name,
        dm_policy: body.dm_policy ?? updateBridgeFixture.bridge.dm_policy,
        provider_config: body.provider_config ?? updateBridgeFixture.bridge.provider_config,
        routing_policy: body.routing_policy ?? updateBridgeFixture.bridge.routing_policy,
      },
    });
  }),
  aghApiMock.put("/api/bridges/{id}/secret-bindings/{binding_name}", ({ params }) => {
    const id = String(params.id);
    const bindingName = String(params.binding_name);

    if (id !== bridgeDetailFixture.bridge.id) {
      return HttpResponse.json({ error: `Bridge not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json({
      binding: {
        ...bridgeSecretBindingsFixture[0],
        binding_name: bindingName,
      },
    });
  }),
  aghApiMock.delete("/api/bridges/{id}/secret-bindings/{binding_name}", ({ params }) => {
    const id = String(params.id);

    if (id !== bridgeDetailFixture.bridge.id) {
      return HttpResponse.json({ error: `Bridge not found: ${id}` }, { status: 404 });
    }

    return new HttpResponse(null, { status: 204 });
  }),
  aghApiMock.post("/api/bridges/{id}/enable", ({ params }) => {
    const id = String(params.id);

    if (id !== bridgeDetailFixture.bridge.id) {
      return HttpResponse.json({ error: `Bridge not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json({
      ...bridgeDetailFixture,
      bridge: {
        ...bridgeDetailFixture.bridge,
        enabled: true,
        status: "ready",
      },
    });
  }),
  aghApiMock.post("/api/bridges/{id}/disable", ({ params }) => {
    const id = String(params.id);

    if (id !== bridgeDetailFixture.bridge.id) {
      return HttpResponse.json({ error: `Bridge not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json({
      ...bridgeDetailFixture,
      bridge: {
        ...bridgeDetailFixture.bridge,
        enabled: false,
        status: "disabled",
      },
    });
  }),
  aghApiMock.post("/api/bridges/{id}/restart", ({ params }) => {
    const id = String(params.id);

    if (id !== bridgeDetailFixture.bridge.id) {
      return HttpResponse.json({ error: `Bridge not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json(bridgeDetailFixture);
  }),
  aghApiMock.post("/api/bridges/{id}/test-delivery", ({ params }) => {
    const id = String(params.id);

    if (id !== bridgeDetailFixture.bridge.id) {
      return HttpResponse.json({ error: `Bridge not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json(testBridgeDeliveryFixture);
  }),
  aghApiMock.post("/api/bridges/{id}/verify", ({ params }) => {
    const id = String(params.id);
    if (id !== bridgeDetailFixture.bridge.id) {
      return HttpResponse.json({ error: `Bridge not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json(bridgeVerifyFixture);
  }),
  aghApiMock.post("/api/bridges/{id}/send-test", ({ params }) => {
    const id = String(params.id);
    if (id !== bridgeDetailFixture.bridge.id) {
      return HttpResponse.json({ error: `Bridge not found: ${id}` }, { status: 404 });
    }

    return HttpResponse.json(sendBridgeTestFixture);
  }),
];
