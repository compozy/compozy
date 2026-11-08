import { delay, HttpResponse } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";
import { storybookMswParameters } from "@/storybook/msw";
import type { ExtensionEntry } from "@/systems/extensions";
import type {
  MarketplaceCatalogListing,
  MarketplaceCatalogResponse,
  MarketplaceSource,
} from "@/systems/marketplace";
import {
  marketplaceCatalogFixture,
  marketplaceCategoryCounts,
  marketplaceCategoryFixtures,
  marketplaceCategoryItems,
  marketplaceSectionsFromCatalog,
  marketplaceSourceFixtures,
  marketplaceSourceHandlers,
} from "@/systems/marketplace/mocks";
import type { SettingsMCPServerEntry, SettingsMCPServerRequest } from "@/systems/settings";
import {
  settingsAppliedMutationFixture,
  settingsMCPServersCollectionFixture,
} from "@/systems/settings/mocks";

import type { StoryExtensionDetail } from "./marketplace-story-servers";

/** A 28px feed icon as a data URL: rung 1 of the logo ladder without a network fetch. */
export const STORY_FEED_ICON =
  "data:image/svg+xml," +
  encodeURIComponent(
    '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 28 28"><rect width="28" height="28" rx="6" fill="#2f6df6"/><path d="M7 14.5l4.5 4.5L21 9.5" stroke="#fff" stroke-width="3" fill="none" stroke-linecap="round" stroke-linejoin="round"/></svg>'
  );

const [otelBridge, slackNotify, policyBlocked, acmeTools] = marketplaceCatalogFixture.items as [
  MarketplaceCatalogListing,
  MarketplaceCatalogListing,
  MarketplaceCatalogListing,
  MarketplaceCatalogListing,
];

function catalogEntry(
  base: MarketplaceCatalogListing,
  overrides: Partial<MarketplaceCatalogListing>
): MarketplaceCatalogListing {
  return {
    ...base,
    installed: false,
    installed_name: undefined,
    installed_version: undefined,
    update_available: false,
    featured: false,
    ...overrides,
    install_slug: `compozy/${overrides.entry_id ?? base.entry_id}`,
  };
}

/** Today's catalog as the stories see it: real feed names, every logo rung represented. */
export const storyCatalog = {
  otelBridge: catalogEntry(otelBridge, {
    category: "cloud",
    category_label: "Cloud & Infrastructure",
  }),
  slackNotify: catalogEntry(slackNotify, {}),
  policyBlocked: catalogEntry(policyBlocked, {}),
  acmeTools: catalogEntry(acmeTools, {}),
  github: catalogEntry(otelBridge, {
    author: "CompozyOS",
    description:
      "Inspect GitHub repositories, issues, pull requests, and releases with GitHub's official MCP server.",
    category: "development",
    category_label: "Development",
    featured: true,
    entry_id: "github",
    name: "GitHub",
    tier: "official",
    version: "1.8.0",
  }),
  context7: catalogEntry(otelBridge, {
    author: "CompozyOS",
    description: "Fetch current, version-specific library documentation and code examples.",
    category: "research",
    category_label: "Research & Learning",
    featured: true,
    entry_id: "context7",
    icon: STORY_FEED_ICON,
    name: "Context7",
    tier: "official",
    version: "3.2.3",
  }),
  herdrBridge: catalogEntry(otelBridge, {
    author: "Alexandre Akira",
    description:
      "Follow CompozyOS agents and Loops in herdr with live status, readable messages, and automatic pane cleanup",
    category: "ai-agents",
    category_label: "AI & Agents",
    entry_id: "herdr-bridge",
    name: "herdr bridge",
    tier: "community",
    version: "0.3.3",
  }),
  grafana: catalogEntry(otelBridge, {
    author: "Grafana Labs",
    description: "Query dashboards, alerts, and logs from your Grafana instance",
    category: "data",
    category_label: "Data & Analytics",
    entry_id: "grafana",
    name: "Grafana",
    tier: "community",
    version: "0.4.1",
  }),
  repositoryOrientation: catalogEntry(otelBridge, {
    author: "CompozyOS",
    description:
      "Map an unfamiliar repository before changing it, with explicit ownership, invariants, and verification paths",
    category: "development",
    category_label: "Development",
    entry_id: "repository-orientation",
    name: "Repository Orientation",
    tier: "official",
    version: "1.2.0",
  }),
  postgres: catalogEntry(otelBridge, {
    author: "CompozyOS",
    description: "Query and inspect PostgreSQL databases with read-only access by default.",
    category: "data",
    category_label: "Data & Analytics",
    featured: true,
    entry_id: "postgres",
    name: "PostgreSQL (Yaw Labs)",
    tier: "official",
    version: "0.7.0",
  }),
};

/** A plugin listed by a registered marketplace: its own source, slug, and manifest layout. */
export function pluginListing(
  source: string,
  overrides: Partial<MarketplaceCatalogListing> &
    Pick<MarketplaceCatalogListing, "entry_id" | "name">
): MarketplaceCatalogListing {
  return {
    ...acmeTools,
    author: "@community",
    format: "agent-plugin",
    installed: false,
    installed_name: undefined,
    installed_version: undefined,
    layout: "claude-plugin",
    source,
    source_ref: `marketplace:${source}`,
    tier: "unverified",
    update_available: false,
    ...overrides,
    install_slug: `${source}/${overrides.entry_id}`,
  };
}

/** The sources list as Settings and Browse see it: feed · preset on · preset off · custom. */
export const storySources = marketplaceSourceFixtures;

/** Plugins from two marketplaces, one of them blocked because its package could not be loaded. */
export const storyMarketplacePlugins = {
  featureDev: pluginListing("claude-plugins-official", {
    description: "Guided feature development: explore, design, implement, and verify in one flow.",
    entry_id: "feature-dev",
    name: "Feature Dev",
    version: "1.2.0",
  }),
  codeReview: pluginListing("claude-plugins-official", {
    description: "Structured review passes with findings ranked by severity.",
    entry_id: "code-review",
    name: "Code Review",
    version: "0.9.1",
  }),
  releaseNotes: pluginListing("team-plugins", {
    description: "Draft release notes from merged pull requests since the last tag.",
    entry_id: "release-notes",
    name: "Release Notes",
    version: "0.3.0",
  }),
  legacyTool: pluginListing("team-plugins", {
    description: "An older package whose manifest CompozyOS could not locate.",
    entry_id: "legacy-tool",
    install_blocker:
      "no manifest found (checked plugin.json, .claude-plugin/plugin.json, .codex-plugin/plugin.json, .cursor-plugin/plugin.json)",
    installable: false,
    layout: undefined,
    name: "Legacy Tool",
    version: "0.1.0",
  }),
};

/** Catalog `sources[]` summaries derived from the registered rows and what the catalog lists. */
export function catalogSourceSummaries(
  sources: readonly MarketplaceSource[],
  items: readonly MarketplaceCatalogListing[]
): MarketplaceCatalogResponse["sources"] {
  const summaries: MarketplaceCatalogResponse["sources"] = [];
  for (const source of sources) {
    if (!source.enabled) continue;
    summaries.push({
      count: items.filter(item => item.source === source.name).length,
      kind: source.kind,
      last_read_at: source.last_read_at,
      name: source.name,
      state: source.state,
    });
  }
  return summaries;
}

export function catalogResponse(
  items: MarketplaceCatalogListing[],
  overrides: Partial<MarketplaceCatalogResponse> = {}
): MarketplaceCatalogResponse {
  return {
    ...marketplaceCatalogFixture,
    items,
    sources: [{ count: items.length, kind: "feed", name: "compozy-catalog", state: "ok" }],
    total: items.length,
    ...overrides,
  };
}

function matchesQuery(entry: MarketplaceCatalogListing, query: string): boolean {
  return (
    query === "" || `${entry.name} ${entry.description}`.toLowerCase().includes(query.toLowerCase())
  );
}

/** One MSW group set per story: a second `storybookMswParameters` spread would replace the first. */
export function marketplaceStoryHandlers(options: {
  catalog?: MarketplaceCatalogListing[];
  catalogOverrides?: Partial<MarketplaceCatalogResponse>;
  catalogStatus?: number;
  catalogDelay?: "infinite";
  /** The installed inventory (`/api/extensions`) never settles: shelf, Installed and Manage wait. */
  extensionsDelay?: "infinite";
  /** The catalog entry read (the detail page) never settles. */
  entryDelay?: "infinite";
  /** The installed extension's kit inventory never settles. */
  inventoryDelay?: "infinite";
  extensions?: ExtensionEntry[];
  /** Per-entry detail payload overrides (servers, inputs, contents) keyed by entry id. */
  details?: Record<string, Partial<StoryExtensionDetail>>;
  installDelayMs?: number;
  installResult?: ExtensionEntry;
  updateDelay?: "infinite";
  pendingUpdates?: readonly string[];
  /** Registered sources; the catalog `sources[]` summaries derive from the enabled ones. */
  sources?: readonly MarketplaceSource[];
  /** The MCPs view's managed server list (`/api/settings/mcp-servers`); omitted keeps the default. */
  mcpServers?: SettingsMCPServerEntry[];
  /** Add installs but its enable fails: the row reads Couldn't turn on with Retry. */
  installEnableError?: { code: string; error: string };
}) {
  let catalog = options.catalog ?? [];
  let extensions = options.extensions ?? [];
  const installResult = options.installResult;
  const pendingUpdates = new Set(options.pendingUpdates);
  const details = options.details ?? {};
  const sources = options.sources ?? [marketplaceSourceFixtures.feed];
  // Saves land in this list so a refetch after Add MCP server keeps the new row.
  let mcpServers = options.mcpServers ? [...options.mcpServers] : undefined;
  return storybookMswParameters({
    marketplace: [
      compozyApiMock.get("/api/marketplace", async ({ request }) => {
        if (options.catalogDelay === "infinite") await delay("infinite");
        if (options.catalogStatus) {
          return HttpResponse.json(
            { error: "catalog fetch: dial tcp: lookup raw.githubusercontent.com: no such host" },
            { status: options.catalogStatus }
          );
        }
        const params = new URL(request.url).searchParams;
        const query = params.get("q")?.trim() ?? "";
        const category = params.get("category")?.trim() ?? "";
        if (category && !marketplaceCategoryFixtures.some(item => item.id === category)) {
          return HttpResponse.json(
            { code: "marketplace_category_unknown", error: `unknown category "${category}"` },
            { status: 400 }
          );
        }
        const matches = catalog.filter(entry => matchesQuery(entry, query));
        const items = category ? marketplaceCategoryItems(matches, category) : matches;
        return HttpResponse.json(
          catalogResponse(items, {
            ...options.catalogOverrides,
            categories: marketplaceCategoryCounts(matches),
            sources: catalogSourceSummaries(sources, catalog),
          })
        );
      }),
      compozyApiMock.get("/api/marketplace/sections", async () => {
        if (options.catalogDelay === "infinite") await delay("infinite");
        if (options.catalogStatus) {
          return HttpResponse.json(
            { error: "catalog fetch: dial tcp: lookup raw.githubusercontent.com: no such host" },
            { status: options.catalogStatus }
          );
        }
        return HttpResponse.json(
          marketplaceSectionsFromCatalog(
            catalogResponse(catalog, {
              ...options.catalogOverrides,
              sources: catalogSourceSummaries(sources, catalog),
            })
          )
        );
      }),
      ...marketplaceSourceHandlers(sources),
      compozyApiMock.get("/api/marketplace/entries/{entry_id}", async ({ params }) => {
        if (options.entryDelay === "infinite") await delay("infinite");
        const entry = catalog.find(item => item.entry_id === String(params.entry_id));
        if (!entry) {
          return HttpResponse.json({ error: "Marketplace entry not found" }, { status: 404 });
        }
        return HttpResponse.json({
          entry,
          extension: {
            artifact_url: `https://example.test/${entry.entry_id}.tar.gz`,
            contents: { agents: 0, hooks: 0, loops: 0, mcp_servers: 0, skills: 0 },
            digest_sha256: entry.digest_sha256,
            inputs: [],
            install_slug: entry.install_slug ?? "",
            mcp_servers: [],
            ...details[entry.entry_id],
          },
        });
      }),
    ],
    extensions: [
      ...(installResult
        ? [
            compozyApiMock.post("/api/extensions", async ({ request }) => {
              // Add lands like the daemon reports it: the row is installed, the inventory has it.
              const { ref } = (await request.json()) as { ref?: string };
              const enabled = !options.installEnableError;
              const extension = { ...installResult, enabled };
              catalog = catalog.map(entry =>
                entry.install_slug === ref
                  ? { ...entry, installed: true, installed_name: installResult.name }
                  : entry
              );
              extensions = [...extensions.filter(item => item.name !== extension.name), extension];
              return HttpResponse.json(
                {
                  extension,
                  ...(options.installEnableError
                    ? { enable_error: options.installEnableError }
                    : {}),
                },
                { status: 201 }
              );
            }),
          ]
        : []),
      compozyApiMock.get("/api/extensions", async () => {
        if (options.extensionsDelay === "infinite") await delay("infinite");
        return HttpResponse.json({ extensions });
      }),
      compozyApiMock.get("/api/extensions/{name}/inventory", async ({ params }) => {
        if (options.inventoryDelay === "infinite") await delay("infinite");
        const extension = extensions.find(item => item.name === params.name);
        if (!extension) return HttpResponse.json({ error: "Extension not found" }, { status: 404 });
        return HttpResponse.json({
          extension: extension.name,
          enabled: extension.enabled,
          format: "compozy",
          items: [],
        });
      }),
      compozyApiMock.post("/api/extensions/update", async ({ request }) => {
        const body = (await request.json()) as { names?: string[] };
        const names = body.names ?? [];
        const selectedNames = new Set(names);
        if (options.updateDelay || names.some(name => pendingUpdates.has(name))) {
          await delay("infinite");
        }
        extensions = extensions.map(extension =>
          selectedNames.has(extension.name)
            ? {
                ...extension,
                update_available: false,
                marketplace: extension.marketplace
                  ? { ...extension.marketplace, update_available: false }
                  : null,
              }
            : extension
        );
        return HttpResponse.json({
          updates: names.map(name => ({
            name,
            path: `/var/lib/compozy/extensions/${name}`,
            registry: "compozy",
            slug: name,
            status: "updated" as const,
          })),
        });
      }),
      compozyApiMock.post("/api/extensions/preview-install", async ({ request }) => {
        const body = (await request.json()) as { ref?: string };
        if (options.installDelayMs) await delay(options.installDelayMs);
        return HttpResponse.json({
          declared_profiles: [{ create: false, credentials: [], name: "default" }],
          inputs: [],
          name: body.ref?.split("/").pop() ?? "",
          placements: [],
        });
      }),
    ],
    ...(mcpServers
      ? {
          settings: [
            compozyApiMock.get("/api/settings/mcp-servers", () =>
              HttpResponse.json({
                ...settingsMCPServersCollectionFixture,
                mcp_servers: mcpServers ?? [],
              })
            ),
            compozyApiMock.put("/api/settings/mcp-servers/{name}", async ({ params, request }) => {
              const body = (await request.json()) as SettingsMCPServerRequest;
              const name = String(params.name);
              const existing = (mcpServers ?? []).find(server => server.name === name);
              mcpServers = [
                ...(mcpServers ?? []).filter(server => server.name !== name),
                // An edit keeps its row's provenance; a new one reads as a personal settings-file
                // row. The saved connection wins, projected into the read shape the editor reopens.
                storyMCPRow(existing ?? settingsMCPServersCollectionFixture.mcp_servers[0]!, body),
              ];
              return HttpResponse.json({
                ...settingsAppliedMutationFixture,
                section: "mcp-servers",
              });
            }),
            compozyApiMock.delete("/api/settings/mcp-servers/{name}", ({ params }) => {
              mcpServers = (mcpServers ?? []).filter(server => server.name !== String(params.name));
              return HttpResponse.json({
                ...settingsAppliedMutationFixture,
                section: "mcp-servers",
              });
            }),
          ],
        }
      : {}),
  });
}

/** The read row the daemon would return for a saved write: connection fields plus key names. */
function storyMCPRow(
  base: SettingsMCPServerEntry,
  body: SettingsMCPServerRequest
): SettingsMCPServerEntry {
  const { server } = body;
  const unique = (keys: string[]) => [...new Set(keys)].sort();
  const envKeys = unique([...Object.keys(server.env ?? {}), ...(body.preserve_env ?? [])]);
  const secretEnvKeys = unique([
    ...Object.keys(server.secret_env ?? {}),
    ...(body.preserve_secrets?.secret_env ?? []),
    ...Object.keys(body.secret_values?.secret_env ?? {}),
  ]);
  return {
    ...base,
    name: server.name,
    transport: server.transport ?? base.transport,
    command: server.command,
    args: server.args,
    url: server.url,
    auth: server.auth ? { ...server.auth, client_secret_configured: false } : null,
    env_keys: envKeys.length > 0 ? envKeys : undefined,
    secret_env_keys: secretEnvKeys.length > 0 ? secretEnvKeys : undefined,
  };
}
