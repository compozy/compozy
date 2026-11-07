import { createServer, type Server } from "node:http";
import { cp, mkdir, mkdtemp, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

import type { BrowserExtensionRegistrySeed } from "./extension-registry-server";
import type {
  BrowserMarketplaceCatalogSeed,
  BrowserMarketplaceExtensionEntrySeed,
} from "./runtime-seed";

const DEFAULT_HOST = "127.0.0.1";
const DEFAULT_GENERATED_AT = "2026-07-14T00:00:00Z";

const VERIFIED_REPOSITORY = "compozy/verified-kit";
const VERIFIED_TAG = "v1.0.0";

/**
 * A verified, gate-free curated entry: an `official` feed row whose archive the fake registry
 * serves under the listed digest, with no inputs, servers or gateway requirement. Add on it has
 * nothing to decide, so it installs and enables inline (US-005.AC-1, E2E-003). Seed both halves:
 * `extensionRegistry` serves the archive and `marketplaceCatalog` lists the entry.
 */
export const verifiedCatalogFixture: {
  entryId: string;
  extensionRegistry: BrowserExtensionRegistrySeed;
  entry: BrowserMarketplaceExtensionEntrySeed;
} = {
  entryId: "verified-kit",
  extensionRegistry: {
    repository: VERIFIED_REPOSITORY,
    extensionName: "verified-kit",
    description: "Verified kit with nothing to configure",
    releases: [{ tag: VERIFIED_TAG, version: "1.0.0" }],
  },
  entry: {
    entry_id: "verified-kit",
    name: "Verified Kit",
    description: "Verified kit with nothing to configure",
    version: "1.0.0",
    install_slug: VERIFIED_REPOSITORY,
    repository: `https://github.com/${VERIFIED_REPOSITORY}`,
    // Resolved by the runtime fixture to the registry's artifact URL and archive digest.
    artifact_url: "",
    digest_release_tag: VERIFIED_TAG,
    tier: "official",
    category: "design",
  },
};

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

/** A client-layout plugin whose root `.mcp.json` serves one remote server behind OAuth. */
async function writeOAuthPlugin(plugin: string, authServerURL: string) {
  await mkdir(path.join(plugin, ".claude-plugin"), { recursive: true });
  await writeFile(
    path.join(plugin, ".claude-plugin", "plugin.json"),
    JSON.stringify({ name: "linear", description: "Linear issues", version: "1.0.0" })
  );
  await writeFile(
    path.join(plugin, ".mcp.json"),
    JSON.stringify({
      mcpServers: {
        linear: {
          type: "http",
          url: `${authServerURL}/mcp`,
          auth: { CLIENT_ID: "compozy-e2e-public", scopes: ["read", "write"] },
        },
      },
    })
  );
}

/**
 * An isolated real client-layout source used by browser acquisition and digest-race journeys.
 * With `oauthServerURL`, the source also lists `linear`, a plugin whose MCP server needs sign-in.
 */
export async function createPluginMarketplaceFixture(options: { oauthServerURL?: string } = {}) {
  const root = await mkdtemp(path.join(os.tmpdir(), "compozy-browser-marketplace-"));
  const source = path.join(root, "source");
  const plugin = path.join(source, "tool");
  const fixture = fileURLToPath(
    new URL("../../../internal/extension/testdata/client-plugins/loop-engineering", import.meta.url)
  );
  try {
    await mkdir(source, { recursive: true });
    await cp(fixture, plugin, { recursive: true });
    const plugins = [
      { name: "tool", description: "Loop engineering skills for daily triage", source: "./tool" },
    ];
    if (options.oauthServerURL) {
      await writeOAuthPlugin(path.join(source, "linear"), options.oauthServerURL);
      plugins.push({ name: "linear", description: "Linear issues", source: "./linear" });
    }
    await writeFile(
      path.join(source, "marketplace.json"),
      JSON.stringify({ name: "team", owner: { name: "Acme" }, plugins })
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
