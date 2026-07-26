import { HttpResponse, type HttpHandler } from "msw";

import { aghApiMock } from "@/storybook/openapi-msw";

import {
  marketplaceBundlePreviewFixture,
  marketplaceDetails,
  marketplaceKindFixture,
  marketplaceListings,
  marketplaceSearchFixture,
} from "./fixtures";
import type { MarketplaceKind } from "../types";

function queryText(request: Request): string {
  return new URL(request.url).searchParams.get("q")?.trim().toLowerCase() ?? "";
}

function matchesQuery(name: string, description: string, query: string): boolean {
  return query === "" || `${name} ${description}`.toLowerCase().includes(query);
}

export const handlers: HttpHandler[] = [
  aghApiMock.get("/api/marketplace/search", ({ request }) => {
    const query = queryText(request);
    return HttpResponse.json({
      ...marketplaceSearchFixture,
      query,
      kinds: marketplaceSearchFixture.kinds.map(result => {
        const items = result.items.filter(item => matchesQuery(item.name, item.description, query));
        return { ...result, items, total: items.length };
      }),
    });
  }),
  aghApiMock.post("/api/marketplace/refresh", () =>
    HttpResponse.json({
      kinds: [
        {
          entry_count: marketplaceListings.skill.length,
          kind: "skill",
          outcome: "refreshed",
          stale: false,
        },
        {
          entry_count: marketplaceListings.extension.length,
          kind: "extension",
          outcome: "refreshed",
          stale: false,
        },
        {
          entry_count: marketplaceListings.mcp.length,
          kind: "mcp",
          outcome: "refreshed",
          stale: false,
        },
      ],
    })
  ),
  aghApiMock.get("/api/marketplace/{kind}/{entry_id}", ({ params }) => {
    const key = `${String(params.kind)}:${String(params.entry_id)}`;
    const detail = marketplaceDetails[key];
    return detail
      ? HttpResponse.json(detail)
      : HttpResponse.json({ error: `Marketplace entry not found: ${key}` }, { status: 404 });
  }),
  aghApiMock.get("/api/marketplace/{kind}", ({ params, request }) => {
    const kind = String(params.kind) as MarketplaceKind;
    if (!(kind in marketplaceListings)) {
      return HttpResponse.json({ error: `Marketplace kind not found: ${kind}` }, { status: 404 });
    }
    const query = queryText(request);
    const fixture = marketplaceKindFixture(kind);
    const items = fixture.items.filter(item => matchesQuery(item.name, item.description, query));
    return HttpResponse.json({ ...fixture, items, total: items.length });
  }),
  aghApiMock.post("/api/bundles/preview", () => HttpResponse.json(marketplaceBundlePreviewFixture)),
  aghApiMock.post("/api/bundles/activations", () =>
    HttpResponse.json(marketplaceBundlePreviewFixture, { status: 201 })
  ),
  aghApiMock.post("/api/settings/mcp-servers/install", async ({ request }) => {
    const body = (await request.json()) as { entry_id?: string; name?: string; scope?: string };
    const remote = body.entry_id === "linear";
    return HttpResponse.json({
      apply: {
        active_config_hash: "sha256:active-live",
        active_generation: 42,
        applied: true,
        apply_record_id: "cfg_apply_mcp_live_add",
        lifecycle: "live-add",
        next_action: "none",
        restart_required: false,
        scope: body.scope === "workspace" ? "workspace" : "global",
        warnings: [],
        write_target: body.scope === "workspace" ? "workspace-mcp-sidecar" : "global-mcp-sidecar",
      },
      mcp_server: {
        auth: remote ? { type: "oauth2", client_secret_configured: false } : null,
        auth_status: null,
        catalog_entry: body.entry_id,
        name: body.name ?? body.entry_id ?? "mcp-server",
        scope: body.scope === "workspace" ? "workspace" : "global",
        source_metadata: {
          available_targets: ["global-config", "workspace-config"],
          effective_source: {
            kind: body.scope === "workspace" ? "workspace-config" : "global-config",
            scope: body.scope === "workspace" ? "workspace" : "global",
          },
          shadowed_sources: [],
        },
        transport: remote ? "sse" : "stdio",
        url: remote ? "https://mcp.linear.app/sse" : undefined,
      },
      next_step: remote ? "authorize" : "none",
    });
  }),
];
