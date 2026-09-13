import type {
  MarketplaceCatalogResponse,
  MarketplaceCatalogEntryResponse,
  MarketplaceCatalogListing,
  MarketplaceSource,
  MarketplaceSourcePreview,
  MarketplaceSourcesResponse,
} from "../types";

const warningUnsigned = {
  category: "supply_chain",
  code: "extension_unverified",
  data_freshness: "live",
  id: "extension_unverified",
  message: "The publisher is not on the trusted registry tier.",
  severity: "warning",
  suggested_command: "compozy extensions verify slack-notify",
  title: "Unsigned package",
};

const warningNetwork = {
  category: "network",
  code: "extension_network_egress",
  data_freshness: "catalog",
  id: "extension_network_egress",
  message: "This extension opens outbound connections to Slack.",
  severity: "info",
  title: "Network egress",
};

const entries: MarketplaceCatalogListing[] = [
  {
    author: "@compozy",
    description: "Export session and tool spans to an OpenTelemetry collector.",
    digest_sha256: "a".repeat(64),
    installable: true,
    entry_id: "otel-bridge",
    install_slug: "compozy/otel-bridge",
    installed: false,
    name: "otel-bridge",
    source: "compozy-catalog",
    source_ref: "catalog:compozy",
    manage_path: "/marketplace/installed",
    tier: "official",
    trust: {
      allow_unverified: false,
      checksum_verified: false,
      decision: "verified",
      registry_tier: "official",
      warnings: [],
    },
    update_available: false,
    version: "0.6.0",
  },
  {
    author: "@community",
    description: "Post run summaries to a Slack channel when a loop finishes.",
    digest_sha256: "a".repeat(64),
    installable: true,
    entry_id: "slack-notify",
    install_slug: "compozy/slack-notify",
    installed: false,
    name: "slack-notify",
    source: "compozy-catalog",
    source_ref: "catalog:compozy",
    manage_path: "/marketplace/installed",
    tier: "unverified",
    trust: {
      allow_unverified: true,
      checksum_verified: false,
      decision: "allowed_unverified",
      registry_tier: "unverified",
      warnings: [warningUnsigned, warningNetwork],
    },
    update_available: false,
    version: "1.1.4",
  },
  {
    author: "@community",
    description: "An unverified extension blocked by the active daemon policy.",
    digest_sha256: "a".repeat(64),
    installable: true,
    entry_id: "policy-blocked",
    install_slug: "compozy/policy-blocked",
    installed: false,
    name: "policy-blocked",
    source: "compozy-catalog",
    source_ref: "catalog:compozy",
    manage_path: "/marketplace/installed",
    tier: "unverified",
    trust: {
      allow_unverified: false,
      checksum_verified: false,
      decision: "blocked",
      registry_tier: "unverified",
      warnings: [warningUnsigned],
    },
    update_available: false,
    version: "0.3.2",
  },
  /** Curated entry declaring the portable format; the marker is display metadata only. */
  {
    author: "@acme",
    description: "Deploy checks and a tools API server, packaged in the Agent Plugins format.",
    digest_sha256: "a".repeat(64),
    installable: true,
    entry_id: "acme-tools",
    format: "agent-plugin",
    install_slug: "compozy/acme-tools",
    installed: false,
    name: "acme.tools",
    source: "compozy-catalog",
    source_ref: "catalog:compozy",
    manage_path: "/marketplace/installed",
    tier: "community",
    trust: {
      allow_unverified: true,
      checksum_verified: false,
      decision: "allowed_unverified",
      registry_tier: "community",
      warnings: [warningUnsigned],
    },
    update_available: false,
    version: "2.1.0",
  },
];

export const marketplaceCatalogFixture: MarketplaceCatalogResponse = {
  total: entries.length,
  revision: "catalog-fixture-v1",
  stale: false,
  sources: [{ name: "compozy-catalog", kind: "feed", state: "ok", count: entries.length }],
  items: entries,
};

export function marketplaceCatalogDetailFixture(
  entryId: string
): MarketplaceCatalogEntryResponse | null {
  const entry = entries.find(item => item.entry_id === entryId);
  if (!entry) return null;
  return {
    entry,
    extension: {
      artifact_url: `https://example.test/${encodeURIComponent(entry.entry_id)}.tar.gz`,
      contents: { skills: 0, mcp_servers: 0, hooks: 0, loops: 0, agents: 0, bridges: 0 },
      inputs: [],
      mcp_servers: [],
      digest_sha256: entry.digest_sha256,
      install_slug: entry.install_slug ?? "",
    },
  };
}

const READ_AT = "2026-09-13T09:12:00Z";
const DEGRADED_READ_AT = "2026-09-10T14:02:11Z";

/** Every source row exactly as `GET /api/marketplace/sources` returns it, in authoritative order. */
export const marketplaceSourceFixtures = {
  feed: {
    diagnostics: [],
    document_path: "v3/extensions.json",
    enabled: true,
    installable: entries.length,
    kind: "feed",
    last_read_at: READ_AT,
    name: "compozy-catalog",
    plugins: entries.length,
    source: "https://raw.githubusercontent.com/compozy/compozy/main/catalog",
    stability: "experimental",
    state: "ok",
  },
  presetOn: {
    diagnostics: [],
    document_path: ".claude-plugin/marketplace.json",
    enabled: true,
    installable: 6,
    kind: "preset",
    last_read_at: READ_AT,
    name: "claude-plugins-official",
    owner: "Anthropic",
    plugins: 6,
    source: "github:anthropics/claude-plugins-official",
    stability: "experimental",
    state: "ok",
  },
  presetOff: {
    diagnostics: [],
    enabled: false,
    installable: 0,
    kind: "preset",
    last_read_at: null,
    name: "openai-codex",
    plugins: 0,
    source: "github:openai/codex",
    stability: "experimental",
    state: "never",
  },
  customDegraded: {
    diagnostics: [
      {
        category: "marketplace",
        code: "load_failed",
        data_freshness: "cached",
        id: "load_failed:legacy-tool",
        message:
          'plugin "legacy-tool": no manifest found (checked plugin.json, .claude-plugin/plugin.json, .codex-plugin/plugin.json, .cursor-plugin/plugin.json)',
        severity: "warning",
        title: "Plugin dropped",
      },
    ],
    document_path: "marketplace.json",
    enabled: true,
    error: "ENOENT",
    error_class: "source_unreachable",
    installable: 3,
    kind: "custom",
    last_read_at: DEGRADED_READ_AT,
    name: "team-plugins",
    owner: "Compozy team",
    plugins: 4,
    source: "file:///Users/pedro/Dev/team-plugins",
    stability: "experimental",
    state: "degraded",
  },
} satisfies Record<string, MarketplaceSource>;

export const marketplaceSourcesFixture: MarketplaceSourcesResponse = {
  sources: [
    marketplaceSourceFixtures.feed,
    marketplaceSourceFixtures.presetOn,
    marketplaceSourceFixtures.presetOff,
    marketplaceSourceFixtures.customDegraded,
  ],
};

/** What `POST /api/marketplace/sources?dry_run=true` reports for a readable reference. */
export const marketplaceSourcePreviewFixture: MarketplaceSourcePreview = {
  diagnostics: [],
  document_path: ".claude-plugin/marketplace.json",
  installable: 6,
  name: "claude-plugins-official",
  owner: "Anthropic",
  plugins: 6,
};

/** The registered row a successful `POST /api/marketplace/sources` returns for that preview. */
export const marketplaceSourceAddedFixture: MarketplaceSource = {
  diagnostics: [],
  document_path: marketplaceSourcePreviewFixture.document_path,
  enabled: true,
  installable: marketplaceSourcePreviewFixture.installable,
  kind: "custom",
  last_read_at: READ_AT,
  name: marketplaceSourcePreviewFixture.name,
  owner: marketplaceSourcePreviewFixture.owner,
  plugins: marketplaceSourcePreviewFixture.plugins,
  source: "github:anthropics/claude-plugins-official",
  stability: "experimental",
  state: "ok",
};
