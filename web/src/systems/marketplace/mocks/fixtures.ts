import type {
  MarketplaceCatalogResponse,
  MarketplaceCatalogEntryResponse,
  MarketplaceCatalogListing,
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
