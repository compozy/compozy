import {
  MARKETPLACE_KINDS,
  type MarketplaceEntryResponse,
  type MarketplaceKind,
  type MarketplaceKindResponse,
  type MarketplaceListing,
  type MarketplaceSearchResponse,
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

export const marketplaceListings: Record<MarketplaceKind, MarketplaceListing[]> = {
  skill: [
    {
      author: "@compozy",
      description: "Branch, review, and land PRs with the project's own checks as the gate.",
      downloads: 3400,
      entry_id: "git-flow",
      install_slug: "compozy/git-flow",
      installed: true,
      installed_name: "git-flow",
      installed_version: "1.4.2",
      kind: "skill",
      manage_path: "/marketplace/skills",
      name: "git-flow",
      source: "registry",
      update_available: false,
      version: "1.4.2",
    },
    {
      author: "@compozy",
      description:
        "Keep packages/site docs aligned with OpenAPI and CLI help after contract changes.",
      downloads: 840,
      entry_id: "docs-sync",
      install_slug: "compozy/docs-sync",
      installed: false,
      kind: "skill",
      name: "docs-sync",
      source: "registry",
      update_available: false,
      version: "0.9.1",
    },
    {
      author: "@compozy",
      description:
        "Spin an isolated COMPOZY_HOME lab with ports, provider homes, and a bootstrap manifest.",
      downloads: 2100,
      entry_id: "qa-bootstrap",
      install_slug: "compozy/qa-bootstrap",
      installed: true,
      installed_name: "qa-bootstrap",
      installed_version: "2.0.0",
      kind: "skill",
      manage_path: "/marketplace/skills",
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
      author: "@compozy",
      description: "Export session and tool spans to an OpenTelemetry collector.",
      downloads: 1200,
      entry_id: "otel-bridge",
      install_slug: "compozy/otel-bridge",
      installed: false,
      kind: "extension",
      name: "otel-bridge",
      source: "registry",
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
      transport: "http",
      update_available: false,
      version: "1.0.0",
    },
  ],
};

export const marketplaceSearchFixture: MarketplaceSearchResponse = {
  query: "",
  kinds: MARKETPLACE_KINDS.map(kind => ({
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
    install_slug: "compozy/git-flow",
    license: "MIT",
    readme:
      "## What it does\n\ngit-flow drives the full branch lifecycle from inside a session. It reads the repository's own checks and treats a non-zero exit as a hard stop.\n\n## Usage\n\n```sh\ncompozy skill run git-flow --branch feat/session-cache\n```",
    repository: "https://github.com/compozy/skill-git-flow",
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

const mcpStdioDetail: MarketplaceEntryResponse = {
  entry: marketplaceListings.mcp[0]!,
  mcp: {
    default_scope: "workspace",
    inputs: [
      {
        binding: { name: "GITHUB_PERSONAL_ACCESS_TOKEN", type: "env" },
        id: "github_personal_access_token",
        prompt: "GitHub personal access token",
        required: true,
        type: "secret",
      },
    ],
    launch: {
      args: ["--read-only"],
      digest: "sha256:d5a18c04b92714c309eb46a2305087e91a4dbd80420f6e462656699f95093520",
      image: "ghcr.io/github/github-mcp-server",
      type: "docker",
    },
  },
};

const mcpRemoteDetail: MarketplaceEntryResponse = {
  entry: marketplaceListings.mcp[1]!,
  mcp: {
    auth: {
      method: "oauth",
      registration: "auto",
    },
    default_scope: "workspace",
    inputs: [],
    launch: {
      type: "remote",
      url: "https://mcp.linear.app/mcp",
    },
  },
};

export const marketplaceDetails: Record<string, MarketplaceEntryResponse> = {
  "skill:git-flow": skillDetail,
  "extension:slack-notify": extensionDetail,
  "extension:policy-blocked": blockedExtensionDetail,
  "mcp:github": mcpStdioDetail,
  "mcp:linear": mcpRemoteDetail,
};
