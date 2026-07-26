import { createServer, type Server } from "node:http";

import type { BrowserMarketplaceCatalogSeed } from "./runtime-seed";

const DEFAULT_HOST = "127.0.0.1";
const DEFAULT_GENERATED_AT = "2026-07-14T00:00:00Z";

export interface MarketplaceCatalogTestServer {
  baseURL: string;
  server: Server;
}

export async function startMarketplaceCatalogServer(
  seed: BrowserMarketplaceCatalogSeed | undefined
): Promise<MarketplaceCatalogTestServer | undefined> {
  if (seed === undefined) {
    return undefined;
  }

  const documents = new Map<string, unknown[]>([
    ["/mcp.json", seed.mcp ?? []],
    ["/extensions.json", seed.extensions ?? []],
    ["/skills.json", seed.skills ?? []],
  ]);
  const server = createServer((request, response) => {
    const requestURL = new URL(request.url ?? "/", `http://${DEFAULT_HOST}`);
    const entries = documents.get(requestURL.pathname);
    if (request.method !== "GET" || entries === undefined) {
      response.statusCode = 404;
      response.setHeader("content-type", "application/json");
      response.end(JSON.stringify({ error: "not_found" }));
      return;
    }

    response.statusCode = 200;
    response.setHeader("content-type", "application/json");
    response.end(
      JSON.stringify({
        manifest_version: 1,
        generated_at: seed.generatedAt ?? DEFAULT_GENERATED_AT,
        entries,
      })
    );
  });

  await new Promise<void>((resolve, reject) => {
    const cleanup = () => {
      server.off("error", handleError);
      server.off("listening", handleListening);
    };
    const handleError = (error: Error) => {
      cleanup();
      reject(error);
    };
    const handleListening = () => {
      cleanup();
      resolve();
    };
    server.once("error", handleError);
    server.once("listening", handleListening);
    server.listen(0, DEFAULT_HOST);
  });

  const address = server.address();
  if (address === null || typeof address === "string") {
    await closeMarketplaceCatalogServer(server);
    throw new Error("failed to resolve browser marketplace catalog server address");
  }
  return { baseURL: `http://${DEFAULT_HOST}:${address.port}`, server };
}

export async function closeMarketplaceCatalogServer(server: Server | undefined): Promise<void> {
  if (server === undefined || !server.listening) {
    return;
  }
  await new Promise<void>((resolve, reject) => {
    server.close(error => {
      if (error) {
        reject(error);
        return;
      }
      resolve();
    });
  });
}
