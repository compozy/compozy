import { HttpResponse, type HttpHandler } from "msw";
import type { OperationResponse } from "@/lib/api-contract";
import { compozyApiMock } from "@/storybook/openapi-msw";
import type { AddMarketplaceSourceRequest, MarketplaceSource } from "../types";
import {
  marketplaceCatalogFixture,
  marketplaceCatalogDetailFixture,
  marketplaceSourceAddedFixture,
  marketplaceSourcePreviewFixture,
  marketplaceSourcesFixture,
} from "./fixtures";

function queryText(request: Request): string {
  return new URL(request.url).searchParams.get("q")?.trim().toLowerCase() ?? "";
}

function matchesQuery(name: string, description: string, query: string): boolean {
  return query === "" || `${name} ${description}`.toLowerCase().includes(query);
}

/** The daemon's structured refusals for a reference or name, keyed by what the person typed. */
function sourceRefusal(
  body: AddMarketplaceSourceRequest,
  sources: readonly MarketplaceSource[]
): HttpResponse<OperationResponse<"addMarketplaceSource", 409>> | null {
  const ref = body.ref.trim();
  const name = body.name?.trim() ?? "";
  if (/dotfiles|not-a-marketplace/u.test(ref)) {
    return HttpResponse.json(
      {
        checked: ["marketplace.json", ".claude-plugin/marketplace.json"],
        code: "marketplace_not_a_marketplace",
        error: `${ref} has no plugin list`,
      },
      { status: 422 }
    );
  }
  if (/too-large/u.test(ref)) {
    return HttpResponse.json(
      {
        code: "marketplace_document_too_large",
        error: "marketplace.json is 3.1 MiB; the limit is 2 MiB",
      },
      { status: 422 }
    );
  }
  if (name === "compozy" || name === "compozy-catalog") {
    return HttpResponse.json(
      { code: "marketplace_source_name_reserved", error: `${name} is reserved` },
      { status: 422 }
    );
  }
  if (name === "team-plugins" || /retained/u.test(ref)) {
    return HttpResponse.json(
      {
        code: "marketplace_source_name_retained",
        error: "the name team-plugins is still used by installed extensions",
        retained_by: ["feature-dev", "release-notes"],
      },
      { status: 409 }
    );
  }
  const preview = marketplaceSourcePreviewFixture;
  const finalName = name || preview.name;
  if (sources.some(source => source.name === finalName)) {
    return HttpResponse.json(
      {
        code: "marketplace_source_exists",
        error: `a marketplace named ${finalName} is already registered`,
        suggested_name: `${finalName}-${(preview.owner ?? "owner").toLowerCase()}`,
      },
      { status: 409 }
    );
  }
  return null;
}

/**
 * Sources routes over one in-memory list. Registration answers with the preview fixture under the
 * requested name; toggles, removals, and refreshes mutate the list the way the daemon would.
 */
export function marketplaceSourceHandlers(
  initial: readonly MarketplaceSource[] = marketplaceSourcesFixture.sources
): HttpHandler[] {
  let sources: MarketplaceSource[] = structuredClone(initial) as MarketplaceSource[];
  const find = (name: string) => sources.find(source => source.name === name);
  return [
    compozyApiMock.get("/api/marketplace/sources", () => HttpResponse.json({ sources })),
    compozyApiMock.post("/api/marketplace/sources", async ({ request }) => {
      const body = (await request.json()) as AddMarketplaceSourceRequest;
      const dryRun = new URL(request.url).searchParams.get("dry_run") === "true";
      const refusal = sourceRefusal(body, sources);
      if (refusal) return refusal;
      const name = body.name?.trim() || marketplaceSourcePreviewFixture.name;
      if (dryRun) {
        return HttpResponse.json({ ...marketplaceSourcePreviewFixture, name });
      }
      const source: MarketplaceSource = { ...marketplaceSourceAddedFixture, name };
      sources = [...sources, source];
      return HttpResponse.json({ source }, { status: 201 });
    }),
    compozyApiMock.patch("/api/marketplace/sources/{name}", async ({ params, request }) => {
      const source = find(String(params.name));
      if (!source) {
        return HttpResponse.json(
          { code: "marketplace_source_not_found", error: "Source not found" },
          { status: 404 }
        );
      }
      const body = (await request.json()) as { enabled: boolean | null };
      if (typeof body.enabled === "boolean") source.enabled = body.enabled;
      if (source.kind === "feed") source.enabled = true;
      return HttpResponse.json({ source });
    }),
    compozyApiMock.delete("/api/marketplace/sources/{name}", ({ params }) => {
      const source = find(String(params.name));
      if (!source) {
        return HttpResponse.json(
          { code: "marketplace_source_not_found", error: "Source not found" },
          { status: 404 }
        );
      }
      if (source.kind !== "custom") {
        return HttpResponse.json(
          { code: "marketplace_source_preset_readonly", error: `${source.name} is a preset` },
          { status: 403 }
        );
      }
      sources = sources.filter(item => item.name !== source.name);
      return new HttpResponse(null, { status: 204 });
    }),
    compozyApiMock.post("/api/marketplace/sources/{name}/refresh", ({ params }) => {
      const source = find(String(params.name));
      if (!source) {
        return HttpResponse.json(
          { code: "marketplace_source_not_found", error: "Source not found" },
          { status: 404 }
        );
      }
      return HttpResponse.json({ source });
    }),
  ];
}

export const handlers: HttpHandler[] = [
  compozyApiMock.get("/api/marketplace", ({ request }) => {
    const query = queryText(request);
    const items = marketplaceCatalogFixture.items.filter(item =>
      matchesQuery(item.name, item.description, query)
    );
    return HttpResponse.json({ ...marketplaceCatalogFixture, items, total: items.length });
  }),
  compozyApiMock.get("/api/marketplace/entries/{entry_id}", ({ params }) => {
    const detail = marketplaceCatalogDetailFixture(String(params.entry_id));
    return detail
      ? HttpResponse.json(detail)
      : HttpResponse.json({ error: "Marketplace entry not found" }, { status: 404 });
  }),
  compozyApiMock.post("/api/marketplace/refresh", () =>
    HttpResponse.json({
      sources: [
        {
          entry_count: marketplaceCatalogFixture.items.length,
          source: "compozy-catalog",
          outcome: "refreshed",
          stale: false,
        },
      ],
    })
  ),
  ...marketplaceSourceHandlers(),
];
