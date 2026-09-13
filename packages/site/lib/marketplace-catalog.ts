import type { z } from "zod";
import extensionsFeed from "../../../catalog/extensions.json";
import mcpFeed from "../../../catalog/mcp.json";
import skillsFeed from "../../../catalog/skills.json";
import extensionsV3Feed from "../../../catalog/v3/extensions.json";
import presetsV3Feed from "../../../catalog/v3/marketplaces.json";
import {
  skillEntrySchema,
  mcpEntrySchema,
  skillFeedSchema,
  extensionFeedSchema,
  mcpFeedSchema,
} from "./marketplace-catalog-schema";
import {
  extensionV3EntrySchema,
  extensionV3FeedSchema,
  marketplacePresetsSchema,
} from "./marketplace-catalog-v3-schema";
export {
  skillEntrySchema,
  extensionEntrySchema,
  mcpEntrySchema,
  skillFeedSchema,
  extensionFeedSchema,
  mcpFeedSchema,
} from "./marketplace-catalog-schema";
export {
  extensionV3EntrySchema,
  extensionV3FeedSchema,
  marketplacePresetsSchema,
} from "./marketplace-catalog-v3-schema";

export type SkillEntry = z.infer<typeof skillEntrySchema>;
export type ExtensionEntry = z.infer<typeof extensionV3EntrySchema>;
export type MCPEntry = z.infer<typeof mcpEntrySchema>;

export type MarketplaceKind = "skills" | "extensions" | "mcp";
export type MarketplaceEntry = SkillEntry | ExtensionEntry | MCPEntry;

interface MarketplaceEntryByKind {
  skills: SkillEntry;
  extensions: ExtensionEntry;
  mcp: MCPEntry;
}

export const MARKETPLACE_KINDS: readonly MarketplaceKind[] = ["skills", "extensions", "mcp"];
export const MARKETPLACE_FEED_FILENAMES = ["skills.json", "extensions.json", "mcp.json"] as const;
export const MARKETPLACE_SEARCH_COMMAND = "compozy marketplace search";

export const skillEntries: SkillEntry[] = skillFeedSchema.parse(skillsFeed).entries;
export const retainedExtensionEntries = extensionFeedSchema.parse(extensionsFeed).entries;
export const extensionEntries: ExtensionEntry[] =
  extensionV3FeedSchema.parse(extensionsV3Feed).entries;
export const marketplacePresets = marketplacePresetsSchema.parse(presetsV3Feed).entries;
export const mcpEntries: MCPEntry[] = mcpFeedSchema.parse(mcpFeed).entries;

export function parseMarketplaceCatalog(kind: "skills", feed: unknown): SkillEntry[];
export function parseMarketplaceCatalog(kind: "extensions", feed: unknown): ExtensionEntry[];
export function parseMarketplaceCatalog(kind: "mcp", feed: unknown): MCPEntry[];
export function parseMarketplaceCatalog(kind: MarketplaceKind, feed: unknown): MarketplaceEntry[] {
  switch (kind) {
    case "skills":
      return skillFeedSchema.parse(feed).entries;
    case "extensions":
      return extensionFeedSchema.parse(feed).entries;
    case "mcp":
      return mcpFeedSchema.parse(feed).entries;
  }
}

export function isMarketplaceKind(value: string): value is MarketplaceKind {
  return (MARKETPLACE_KINDS as readonly string[]).includes(value);
}

export function entriesForKind(kind: MarketplaceKind): MarketplaceEntry[] {
  switch (kind) {
    case "skills":
      return skillEntries;
    case "extensions":
      return extensionEntries;
    case "mcp":
      return mcpEntries;
  }
}

export function findEntry(kind: MarketplaceKind, entryId: string): MarketplaceEntry | undefined {
  return entriesForKind(kind).find(entry => entry.entry_id === entryId);
}

export function marketplaceSearchCommand(kind: MarketplaceKind, entry: MarketplaceEntry): string {
  const cliKind = kind === "skills" ? "skill" : kind === "extensions" ? "extension" : "mcp";
  return `${MARKETPLACE_SEARCH_COMMAND} ${entry.entry_id} --kind ${cliKind}`;
}

const installCommandByKind: {
  [Kind in MarketplaceKind]: (entry: MarketplaceEntryByKind[Kind]) => string;
} = {
  skills: entry => `compozy skill install ${entry.install_slug}`,
  extensions: entry => `compozy extension install ${entry.install_slug}`,
  mcp: entry => `compozy mcp install ${entry.entry_id}`,
};

/** The CLI owns installation; this exact command is valid only after the daemon finds the entry. */
export function installCommand<Kind extends MarketplaceKind>(
  kind: Kind,
  entry: MarketplaceEntryByKind[Kind]
): string {
  return installCommandByKind[kind](entry);
}
