import { createServer, type Server } from "node:http";
import { cp, mkdir, mkdtemp, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

import type { BrowserMarketplaceCatalogSeed } from "./runtime-seed";

const DEFAULT_HOST = "127.0.0.1";
const DEFAULT_GENERATED_AT = "2026-07-14T00:00:00Z";

export interface MarketplaceCatalogTestServer {
  baseURL: string;
  server: Server;
  replace(seed: BrowserMarketplaceCatalogSeed): void;
}

export async function startMarketplaceCatalogServer(
  seed: BrowserMarketplaceCatalogSeed | undefined
): Promise<MarketplaceCatalogTestServer | undefined> {
  if (seed === undefined) {
    return undefined;
  }

  const documents = new Map<string, unknown[]>([
    ["/v3/extensions.json", seed.extensions ?? []],
    ["/v3/marketplaces.json", seed.presets ?? []],
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
        manifest_version: 3,
        generated_at: seed?.generatedAt ?? DEFAULT_GENERATED_AT,
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
  return {
    baseURL: `http://${DEFAULT_HOST}:${address.port}`,
    server,
    replace(next) {
      seed = next;
      documents.set("/v3/extensions.json", next.extensions ?? []);
      documents.set("/v3/marketplaces.json", next.presets ?? []);
    },
  };
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

/** An isolated real client-layout source used by browser acquisition and digest-race journeys. */
export async function createPluginMarketplaceFixture() {
  const root = await mkdtemp(path.join(os.tmpdir(), "compozy-browser-marketplace-"));
  const source = path.join(root, "source");
  const plugin = path.join(source, "tool");
  const fixture = fileURLToPath(
    new URL("../../../internal/extension/testdata/client-plugins/loop-engineering", import.meta.url)
  );
  try {
    await mkdir(source, { recursive: true });
    await cp(fixture, plugin, { recursive: true });
    await writeFile(
      path.join(source, "marketplace.json"),
      JSON.stringify({
        name: "team",
        owner: { name: "Acme" },
        plugins: [
          {
            name: "tool",
            description: "Loop engineering skills for daily triage",
            source: "./tool",
          },
        ],
      })
    );
    return {
      root,
      source,
      plugin,
      instanceName: "loop-engineering",
      async changePackage() {
        await writeFile(path.join(plugin, "CHANGELOG.md"), "Changed after approval\n");
      },
      async cleanup() {
        await rm(root, { recursive: true, force: true });
      },
    };
  } catch (error) {
    await rm(root, { recursive: true, force: true });
    throw error;
  }
}
