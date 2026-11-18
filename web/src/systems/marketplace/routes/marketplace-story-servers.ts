import type {
  MarketplaceCatalogEntryResponse,
  MarketplaceExtensionServer,
} from "@/systems/marketplace";

export type StoryExtensionDetail = NonNullable<MarketplaceCatalogEntryResponse["extension"]>;

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
