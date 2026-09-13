import { HttpResponse, type HttpHandler } from "msw";
import { compozyApiMock } from "@/storybook/openapi-msw";
import { marketplaceCatalogFixture, marketplaceCatalogDetailFixture } from "./fixtures";

function queryText(request: Request): string {
  return new URL(request.url).searchParams.get("q")?.trim().toLowerCase() ?? "";
}

function matchesQuery(name: string, description: string, query: string): boolean {
  return query === "" || `${name} ${description}`.toLowerCase().includes(query);
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
      kinds: [
        {
          entry_count: marketplaceCatalogFixture.items.length,
          kind: "extension",
          outcome: "refreshed",
          stale: false,
        },
      ],
    })
  ),
];
