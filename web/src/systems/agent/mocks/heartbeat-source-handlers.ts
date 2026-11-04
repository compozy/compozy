import { HttpResponse, type HttpHandler } from "msw";
import { compozyApiMock } from "@/storybook/openapi-msw";

import { heartbeatSourceCatalogFixture } from "./fixtures";

const approvedScripts = new Map<string, Set<string>>();

export function resetHeartbeatSourceMock(): void {
  approvedScripts.clear();
}

/** The check-in source catalog and script approval, for agents `known` says exist. */
export function heartbeatSourceHandlers(known: (name: string) => boolean): HttpHandler[] {
  return [
    compozyApiMock.get("/api/agents/{name}/heartbeat/sources", ({ params }) => {
      const name = String(params.name);
      if (!known(name)) {
        return HttpResponse.json({ error: `Agent not found: ${name}` }, { status: 404 });
      }
      return HttpResponse.json(heartbeatSourceCatalogFixture(name, approvedScripts.get(name)));
    }),
    compozyApiMock.post(
      "/api/agents/{name}/heartbeat/scripts/{script}/approve",
      async ({ params, request }) => {
        const name = String(params.name);
        const script = String(params.script);
        const { sha256 } = (await request.json()) as { sha256: string };
        const current = heartbeatSourceCatalogFixture(name).scripts.find(
          item => item.name === script
        );
        if (!current || current.sha256 !== sha256) {
          return HttpResponse.json(
            { error: "script contents changed", code: "source_script_changed" },
            { status: 409 }
          );
        }
        approvedScripts.set(name, new Set([...(approvedScripts.get(name) ?? []), script]));
        return HttpResponse.json({ script, sha256, approved_at: new Date().toISOString() });
      }
    ),
  ];
}
