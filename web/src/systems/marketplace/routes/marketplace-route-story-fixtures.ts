import type { MarketplaceCatalogListing, MarketplaceSource } from "@/systems/marketplace";
import type { ExtensionEntry } from "@/systems/extensions";

import {
  installedExtension,
  installedListing,
  localExtension,
  storyCatalog,
  storyContext7Server,
  storyGithubServer,
  storyPostgresServer,
  storyMarketplacePlugins,
  storySources,
} from "./marketplace-story-data";

export const herdrInstalled = installedListing(storyCatalog.herdrBridge, {
  installedVersion: "0.3.2",
  updateAvailable: true,
});
export const context7Installed = installedListing(storyCatalog.context7);
export const githubInstalled = installedListing(storyCatalog.github);
const repositoryInstalled = installedListing(storyCatalog.repositoryOrientation);

/** Today's catalog with two installed rows, one of them updatable. */
export const defaultCatalog = [
  storyCatalog.repositoryOrientation,
  storyCatalog.batuta,
  herdrInstalled,
  context7Installed,
  storyCatalog.github,
  storyCatalog.postgres,
  storyCatalog.slackNotify,
  storyCatalog.policyBlocked,
  storyCatalog.acmeTools,
];

export const defaultExtensions = [
  installedExtension(herdrInstalled, {
    contents: { agents: 0, bridges: 1, hooks: 0, loops: 0, mcp_servers: 0, skills: 2 },
  }),
  installedExtension(context7Installed, {
    contents: { agents: 0, bridges: 0, hooks: 0, loops: 0, mcp_servers: 1, skills: 0 },
  }),
];

export const installedExtensions: ExtensionEntry[] = [
  ...defaultExtensions,
  installedExtension(githubInstalled, {
    contents: { agents: 0, bridges: 0, hooks: 0, loops: 0, mcp_servers: 1, skills: 0 },
    enabled: false,
  }),
  installedExtension(repositoryInstalled, {
    contents: { agents: 0, bridges: 0, hooks: 0, loops: 0, mcp_servers: 0, skills: 1 },
  }),
];

export const installedServerExtensions: ExtensionEntry[] = [
  installedExtension(githubInstalled, {
    contents: { agents: 0, bridges: 0, hooks: 0, loops: 0, mcp_servers: 1, skills: 0 },
    mcp_servers: [
      storyGithubServer({ runtime_name: "github.github", status: "needs_authorization" }),
    ],
  }),
  installedExtension(context7Installed, {
    contents: { agents: 0, bridges: 0, hooks: 0, loops: 0, mcp_servers: 1, skills: 0 },
    mcp_servers: [storyContext7Server({ runtime_name: "context7", status: "running" })],
  }),
  installedExtension(installedListing(storyCatalog.postgres), {
    contents: { agents: 0, bridges: 0, hooks: 0, loops: 0, mcp_servers: 1, skills: 0 },
    inputs: [{ active: false, id: "database_url", set: false, type: "secret" }],
    mcp_servers: [storyPostgresServer({ status: "stopped" })],
    missing_inputs: ["database_url"],
    workspace_id: "ws_story_fintech",
  }),
  installedExtension(herdrInstalled, {
    contents: { agents: 0, bridges: 1, hooks: 0, loops: 0, mcp_servers: 0, skills: 2 },
  }),
];

export const installedAuthorizeExtensions: ExtensionEntry[] = [
  installedExtension(githubInstalled, {
    contents: { agents: 0, bridges: 0, hooks: 0, loops: 0, mcp_servers: 1, skills: 0 },
    mcp_servers: [
      storyGithubServer({ runtime_name: "github.github", status: "needs_authorization" }),
    ],
  }),
];

export const installedUpdateExtensions: ExtensionEntry[] = [
  installedExtension(herdrInstalled, {
    contents: { agents: 0, bridges: 1, hooks: 0, loops: 0, mcp_servers: 0, skills: 2 },
  }),
  installedExtension(
    installedListing(storyCatalog.batuta, {
      installedVersion: "0.4.0",
      updateAvailable: true,
    }),
    { contents: { agents: 0, bridges: 0, hooks: 0, loops: 2, mcp_servers: 0, skills: 1 } }
  ),
  installedExtension(context7Installed),
];

export const installedLocalExtensions: ExtensionEntry[] = [
  installedExtension(context7Installed),
  localExtension("ops-notes", {
    description: "Local incident notes and handoff tools",
    profile: "engineering",
    installation_profile: "engineering",
  }),
  localExtension("acme-runbooks", { workspace_id: "ws_story_fintech" }),
];

export const installedPartialUpdateExtensions: ExtensionEntry[] = [
  installedExtension(herdrInstalled),
  installedExtension(
    installedListing(storyCatalog.batuta, {
      installedVersion: "0.4.0",
      updateAvailable: true,
    }),
    { workspace_id: "ws_story_fintech" }
  ),
];

/** The feed plus two plugin marketplaces: one section per source, in the daemon's order. */
export const multiSourceCatalog = [
  ...defaultCatalog,
  storyMarketplacePlugins.featureDev,
  storyMarketplacePlugins.codeReview,
  storyMarketplacePlugins.releaseNotes,
  storyMarketplacePlugins.legacyTool,
];

export const multiSourceSources = [
  storySources.feed,
  storySources.presetOn,
  storySources.presetOff,
  { ...storySources.customDegraded, error: undefined, error_class: undefined, state: "ok" },
];

export const conflictingCatalog: MarketplaceCatalogListing[] = [
  {
    ...storyCatalog.batuta,
    name_conflict: {
      source: "team-plugins",
      source_ref: "github:acme/team-plugins",
      entry_id: "batuta",
    },
  },
];

export const zeroPluginSources: MarketplaceSource[] = [
  storySources.feed,
  storySources.presetOn,
  {
    ...storySources.customDegraded,
    error: undefined,
    error_class: undefined,
    installable: 0,
    plugins: 0,
    state: "ok",
  },
];

export const logoLadderCatalog = [
  storyCatalog.context7,
  storyCatalog.github,
  storyCatalog.batuta,
  storyCatalog.postgres,
];

export const trailStatesCatalog = [
  storyCatalog.batuta,
  herdrInstalled,
  context7Installed,
  storyCatalog.slackNotify,
  storyCatalog.policyBlocked,
];
