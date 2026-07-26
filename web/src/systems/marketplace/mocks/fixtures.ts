import type {
  BundlePreviewResponse,
  MarketplaceEntryResponse,
  MarketplaceKind,
  MarketplaceKindResponse,
  MarketplaceListing,
  MarketplaceSearchResponse,
} from "@/systems/marketplace";

const warningUnsigned = {
  category: "supply_chain",
  code: "extension_unverified",
  data_freshness: "live",
  id: "extension_unverified",
  message: "The publisher is not on the trusted registry tier.",
  severity: "warning",
  suggested_command: "agh extensions verify slack-notify",
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

export const marketplaceListings: Record<MarketplaceKind, MarketplaceListing[]> = {
  skill: [
    {
      author: "@agh",
      description: "Branch, review, and land PRs with the project's own checks as the gate.",
      downloads: 3400,
      entry_id: "git-flow",
      install_slug: "agh/git-flow",
      installed: true,
      installed_name: "git-flow",
      installed_version: "1.4.2",
      kind: "skill",
      manage_path: "/marketplace/skills?tab=installed",
      name: "git-flow",
      source: "registry",
      update_available: false,
      version: "1.4.2",
    },
    {
      author: "@agh",
      description:
        "Keep packages/site docs aligned with OpenAPI and CLI help after contract changes.",
      downloads: 840,
      entry_id: "docs-sync",
      install_slug: "agh/docs-sync",
      installed: false,
      kind: "skill",
      name: "docs-sync",
      source: "registry",
      update_available: false,
      version: "0.9.1",
    },
    {
      author: "@agh",
      description:
        "Spin an isolated AGH_HOME lab with ports, provider homes, and a bootstrap manifest.",
      downloads: 2100,
      entry_id: "qa-bootstrap",
      install_slug: "agh/qa-bootstrap",
      installed: true,
      installed_name: "qa-bootstrap",
      installed_version: "2.0.0",
      kind: "skill",
      manage_path: "/marketplace/skills?tab=installed",
      name: "qa-bootstrap",
      source: "registry",
      update_available: true,
      version: "2.2.0",
    },
    {
      author: "@compozy",
      description:
        "Validate PRD and TechSpec inputs against the authoring playbook before drafting.",
      downloads: 620,
      entry_id: "spec-preflight",
      install_slug: "compozy/spec-preflight",
      installed: false,
      kind: "skill",
      name: "spec-preflight",
      source: "registry",
      update_available: false,
      version: "1.0.3",
    },
  ],
  extension: [
    {
      author: "@agh",
      description: "Export session and tool spans to an OpenTelemetry collector.",
      downloads: 1200,
      entry_id: "otel-bridge",
      install_slug: "agh/otel-bridge",
      installed: false,
      kind: "extension",
      name: "otel-bridge",
      source: "registry",
      tier: "verified",
      trust: {
        allow_unverified: false,
        checksum_verified: false,
        decision: "verified",
        registry_tier: "verified",
        warnings: [],
      },
      update_available: false,
      version: "0.6.0",
    },
    {
      author: "@community",
      description: "Post run summaries to a Slack channel when a loop finishes.",
      downloads: 840,
      entry_id: "slack-notify",
      install_slug: "community/slack-notify",
      installed: false,
      kind: "extension",
      name: "slack-notify",
      source: "registry",
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
      downloads: 210,
      entry_id: "policy-blocked",
      install_slug: "community/policy-blocked",
      installed: false,
      kind: "extension",
      name: "policy-blocked",
      source: "registry",
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
  ],
  bundle: [
    {
      author: "@agh",
      description:
        "Skills and extensions for dependency review, lockfile hygiene, and release notes.",
      entry_id: "dep-kit",
      installed: false,
      kind: "bundle",
      name: "dep-kit",
      source: "registry",
      update_available: false,
      version: "1.2.0",
    },
    {
      author: "@agh",
      description: "MCP servers and skills for incident triage and status page updates.",
      entry_id: "ops-starter",
      installed: true,
      installed_version: "0.8.0",
      kind: "bundle",
      manage_path: "/marketplace/bundles/activations/activation-ops-starter",
      name: "ops-starter",
      source: "registry",
      update_available: false,
      version: "0.8.0",
    },
  ],
  mcp: [
    {
      author: "@modelcontextprotocol",
      description:
        "Repository issues, pull requests, and code search through a local stdio server.",
      entry_id: "github",
      install_slug: "github",
      installed: false,
      kind: "mcp",
      name: "github",
      source: "curated",
      transport: "stdio",
      update_available: false,
      version: "0.6.2",
    },
    {
      author: "@linear",
      description: "Linear issues and projects through a remote OAuth server.",
      entry_id: "linear",
      install_slug: "linear",
      installed: false,
      kind: "mcp",
      name: "linear",
      source: "curated",
      transport: "sse",
      update_available: false,
      version: "1.0.0",
    },
  ],
};

export const marketplaceSearchFixture: MarketplaceSearchResponse = {
  query: "",
  kinds: (["skill", "extension", "bundle", "mcp"] as const).map(kind => ({
    items: marketplaceListings[kind],
    kind,
    stale: false,
    total: marketplaceListings[kind].length,
  })),
};

export function marketplaceKindFixture(kind: MarketplaceKind): MarketplaceKindResponse {
  return {
    items: marketplaceListings[kind],
    kind,
    stale: false,
    total: marketplaceListings[kind].length,
  };
}

const skillDetail: MarketplaceEntryResponse = {
  entry: marketplaceListings.skill[0]!,
  skill: {
    display_name: "Git flow",
    install_slug: "agh/git-flow",
    license: "MIT",
    readme:
      "## What it does\n\ngit-flow drives the full branch lifecycle from inside a session. It reads the repository's own checks and treats a non-zero exit as a hard stop.\n\n## Usage\n\n```sh\nagh skill run git-flow --branch feat/session-cache\n```",
    repository: "https://github.com/agh-network/skill-git-flow",
    tags: ["git", "review"],
    versions: ["1.4.2", "1.4.1", "1.3.0"],
  },
};

const extensionDetail: MarketplaceEntryResponse = {
  entry: marketplaceListings.extension[1]!,
  extension: {
    artifact_url:
      "https://github.com/community/slack-notify/releases/download/v1.1.4/slack-notify-v1.1.4.tar.gz",
    digest_sha256: "9be4d36a7bf48f5df88bb0ee3564561eaeaaeef2bcf76c2e2ad19856c045ef98",
    install_slug: "community/slack-notify",
    repository: "https://github.com/community/slack-notify",
  },
};

const blockedExtensionDetail: MarketplaceEntryResponse = {
  entry: marketplaceListings.extension[2]!,
  extension: {
    artifact_url: "https://downloads.example.test/policy-blocked-v0.3.2.tar.gz",
    digest_sha256: "4e5cc61de69ff94319af04e1297b2b92af605e200a5a2bd851e7c214f7da32e8",
    install_slug: "community/policy-blocked",
  },
};

const bundleDetail: MarketplaceEntryResponse = {
  bundle: {
    extension_name: "agh-foundations",
    profiles: [
      {
        agents: 1,
        bridges: 0,
        channels: 1,
        description: "Dependency review with a weekly audit job and lockfile-change trigger.",
        jobs: 1,
        layouts: 0,
        name: "default",
        triggers: 1,
      },
      {
        agents: 1,
        bridges: 1,
        channels: 1,
        description: "Dependency review with release notifications.",
        jobs: 2,
        layouts: 0,
        name: "release",
        triggers: 2,
      },
    ],
  },
  entry: marketplaceListings.bundle[0]!,
};

const mcpStdioDetail: MarketplaceEntryResponse = {
  entry: marketplaceListings.mcp[0]!,
  mcp: {
    args: ["-y", "@modelcontextprotocol/server-github"],
    command: "npx",
    default_scope: "workspace",
    env: [
      {
        name: "GITHUB_TOKEN",
        prompt: "GitHub personal access token",
        required: true,
        secret: true,
      },
    ],
    transport: "stdio",
  },
};

const mcpRemoteDetail: MarketplaceEntryResponse = {
  entry: marketplaceListings.mcp[1]!,
  mcp: {
    default_scope: "workspace",
    oauth: {
      client_id: "agh-marketplace",
      issuer_url: "https://linear.app",
      scopes: ["read", "issues:create"],
    },
    transport: "sse",
    url: "https://mcp.linear.app/sse",
  },
};

export const marketplaceDetails: Record<string, MarketplaceEntryResponse> = {
  "skill:git-flow": skillDetail,
  "extension:slack-notify": extensionDetail,
  "extension:policy-blocked": blockedExtensionDetail,
  "bundle:dep-kit": bundleDetail,
  "mcp:github": mcpStdioDetail,
  "mcp:linear": mcpRemoteDetail,
};

export const marketplaceBundlePreviewFixture: BundlePreviewResponse = {
  activation: {
    agents: [{ id: "dep-reviewer", name: "dep-reviewer", provider: "anthropic" }],
    bundle_description: "Dependency review and release hygiene.",
    bundle_name: "dep-kit",
    channels: [{ name: "dependency-review", primary: true }],
    created_at: "2026-07-14T12:00:00Z",
    extension_name: "agh-foundations",
    id: "bundle-preview-dep-kit",
    inventory: [
      { resource_id: "dep-audit", resource_kind: "skill", resource_name: "dep-audit" },
      {
        resource_id: "lockfile-hygiene",
        resource_kind: "skill",
        resource_name: "lockfile-hygiene",
      },
      {
        resource_id: "release-notes",
        resource_kind: "extension",
        resource_name: "release-notes",
      },
    ],
    network_requirement_digest: "sha256:live-network-requirement",
    profile_description: "Dependency review with a weekly audit job.",
    profile_name: "default",
    scope: "workspace",
    spec_drift: false,
    updated_at: "2026-07-14T12:00:00Z",
    version: 1,
    workspace_id: "ws_story_fintech",
  },
};
