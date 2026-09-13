import { delay, HttpResponse } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";
import { storybookMswParameters } from "@/storybook/msw";
import { extensionFixtures, type ExtensionEntry } from "@/systems/extensions";
import type {
  MarketplaceCatalogEntryResponse,
  MarketplaceCatalogListing,
  MarketplaceCatalogResponse,
  MarketplaceExtensionServer,
} from "@/systems/marketplace";
import { marketplaceCatalogFixture } from "@/systems/marketplace/mocks";

/** A 28px feed icon as a data URL: rung 1 of the logo ladder without a network fetch. */
export const STORY_FEED_ICON =
  "data:image/svg+xml;utf8," +
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
    ...overrides,
    install_slug: `compozy/${overrides.entry_id ?? base.entry_id}`,
  };
}

/** Today's catalog as the stories see it: real feed names, every logo rung represented. */
export const storyCatalog = {
  otelBridge: catalogEntry(otelBridge, {}),
  slackNotify: catalogEntry(slackNotify, {}),
  policyBlocked: catalogEntry(policyBlocked, {}),
  acmeTools: catalogEntry(acmeTools, {}),
  github: catalogEntry(otelBridge, {
    author: "CompozyOS",
    description:
      "Inspect GitHub repositories, issues, pull requests, and releases with GitHub's official MCP server.",
    entry_id: "github",
    name: "GitHub",
    tier: "official",
    version: "1.8.0",
  }),
  context7: catalogEntry(otelBridge, {
    author: "CompozyOS",
    description: "Fetch current, version-specific library documentation and code examples.",
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
    entry_id: "herdr-bridge",
    name: "herdr bridge",
    tier: "community",
    version: "0.3.3",
  }),
  batuta: catalogEntry(otelBridge, {
    author: "Francisross Soares",
    description:
      "Route CompozyOS spec-cycle work to the cheapest capable executor while preserving review and verification gates",
    entry_id: "batuta",
    name: "Batuta",
    tier: "community",
    version: "0.4.1",
  }),
  repositoryOrientation: catalogEntry(otelBridge, {
    author: "CompozyOS",
    description:
      "Map an unfamiliar repository before changing it, with explicit ownership, invariants, and verification paths",
    entry_id: "repository-orientation",
    name: "Repository Orientation",
    tier: "official",
    version: "1.2.0",
  }),
  postgres: catalogEntry(otelBridge, {
    author: "CompozyOS",
    description: "Query and inspect PostgreSQL databases with read-only access by default.",
    entry_id: "postgres",
    name: "PostgreSQL (Yaw Labs)",
    tier: "official",
    version: "0.7.0",
  }),
};

/** The daemon-joined installed state of a listing: origin match, never a name match. */
export function installedListing(
  listing: MarketplaceCatalogListing,
  options: { installedVersion?: string; updateAvailable?: boolean } = {}
): MarketplaceCatalogListing {
  return {
    ...listing,
    installed: true,
    installed_name: listing.entry_id,
    installed_version: options.installedVersion ?? listing.version,
    update_available: options.updateAvailable ?? false,
  };
}

const [extensionBase] = extensionFixtures as [ExtensionEntry, ...ExtensionEntry[]];

/** An installed extension row carrying the origin the listing was joined by. */
export function installedExtension(
  listing: MarketplaceCatalogListing,
  overrides: Partial<ExtensionEntry> = {}
): ExtensionEntry {
  return {
    ...extensionBase,
    contents: { agents: 0, bridges: 0, hooks: 0, loops: 0, mcp_servers: 0, skills: 0 },
    enabled: true,
    marketplace: listing,
    name: listing.installed_name ?? listing.entry_id,
    origin: {
      entry_id: listing.entry_id,
      source: listing.source,
      source_ref: listing.source_ref ?? "catalog:compozy",
    },
    remote_version: listing.update_available ? listing.version : undefined,
    update_available: listing.update_available,
    version: listing.installed_version ?? listing.version ?? extensionBase.version,
    ...overrides,
  };
}

/** A sideloaded extension with no catalog origin: it stays visible in Installed as itself. */
export function localExtension(
  name: string,
  overrides: Partial<ExtensionEntry> = {}
): ExtensionEntry {
  return {
    ...extensionBase,
    contents: { agents: 1, bridges: 0, hooks: 0, loops: 0, mcp_servers: 0, skills: 2 },
    enabled: true,
    marketplace: null,
    name,
    origin: null,
    provenance: undefined,
    remote_version: undefined,
    source: "local_path",
    trust: undefined,
    update_available: false,
    version: "0.1.0",
    ...overrides,
  };
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

type StoryExtensionDetail = NonNullable<MarketplaceCatalogEntryResponse["extension"]>;

/** The GitHub server as the daemon publishes it for the `compozy/github` extension (ADR-008). */
export function storyGithubServer(
  overrides: Partial<MarketplaceExtensionServer> = {}
): MarketplaceExtensionServer {
  return {
    auth: {
      issuer_url: "https://github.com/login/oauth",
      method: "oauth",
      registration: "dynamic",
      scopes: ["repo", "read:org"],
    },
    launch: "https://api.githubcopilot.com",
    name: "github",
    owner: "extension:github",
    profile: "default",
    scope: "global",
    transport: "http",
    ...overrides,
  };
}

/** The Context7 server: no auth, running once installed. */
export function storyContext7Server(
  overrides: Partial<MarketplaceExtensionServer> = {}
): MarketplaceExtensionServer {
  return {
    auth: { method: "none" },
    launch: "https://mcp.context7.com",
    name: "context7",
    owner: "extension:context7",
    profile: "default",
    scope: "global",
    transport: "http",
    ...overrides,
  };
}

/** The Postgres server: a local process with one required secret input. */
export function storyPostgresServer(
  overrides: Partial<MarketplaceExtensionServer> = {}
): MarketplaceExtensionServer {
  return {
    auth: { method: "none" },
    launch: "postgres-mcp",
    name: "postgres",
    owner: "extension:postgres",
    profile: "default",
    scope: "workspace",
    transport: "stdio",
    workspace_id: "ws_story_fintech",
    ...overrides,
  };
}

export const storyPostgresInputs: StoryExtensionDetail["inputs"] = [
  {
    binding: { name: "DATABASE_URL", type: "env" },
    id: "database_url",
    prompt: "Connection string",
    required: true,
    type: "secret",
  },
];

/** One MSW group set per story: a second `storybookMswParameters` spread would replace the first. */
export function marketplaceStoryHandlers(options: {
  catalog?: MarketplaceCatalogListing[];
  catalogOverrides?: Partial<MarketplaceCatalogResponse>;
  catalogStatus?: number;
  catalogDelay?: "infinite";
  extensions?: ExtensionEntry[];
  /** Per-entry detail payload overrides (servers, inputs, contents) keyed by entry id. */
  details?: Record<string, Partial<StoryExtensionDetail>>;
  installDelayMs?: number;
}) {
  const catalog = options.catalog ?? [];
  const extensions = options.extensions ?? [];
  const details = options.details ?? {};
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
        const query = new URL(request.url).searchParams.get("q")?.trim() ?? "";
        const items = catalog.filter(entry => matchesQuery(entry, query));
        return HttpResponse.json(
          catalogResponse(items, {
            ...options.catalogOverrides,
            sources: [
              { count: catalog.length, kind: "feed", name: "compozy-catalog", state: "ok" },
            ],
          })
        );
      }),
      compozyApiMock.get("/api/marketplace/entries/{entry_id}", ({ params }) => {
        const entry = catalog.find(item => item.entry_id === String(params.entry_id));
        if (!entry) {
          return HttpResponse.json({ error: "Marketplace entry not found" }, { status: 404 });
        }
        return HttpResponse.json({
          entry,
          extension: {
            artifact_url: `https://example.test/${entry.entry_id}.tar.gz`,
            contents: { agents: 0, bridges: 0, hooks: 0, loops: 0, mcp_servers: 0, skills: 0 },
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
      compozyApiMock.get("/api/extensions", () => HttpResponse.json({ extensions })),
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
  });
}
