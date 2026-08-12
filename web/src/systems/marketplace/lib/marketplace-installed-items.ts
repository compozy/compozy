import type { MarketplaceKind, MarketplaceListing } from "../types";
import {
  extensionTrustFacts,
  type ExtensionTrustFacts,
  type ExtensionTrustSource,
} from "@/systems/extensions";
import { mcpManagementScopeLabel, type SettingsMCPServerEntry } from "@/systems/settings";
import type { SkillPayload } from "@/systems/skill";

export interface MarketplaceInstalledItem {
  entry: MarketplaceListing;
  skill?: SkillPayload;
  extensionEnabled?: boolean;
  /** Distribution truth for an installed extension; absent for every other kind. */
  extensionFacts?: ExtensionTrustFacts;
  scopeLabel?: string;
  mcpServer?: SettingsMCPServerEntry;
}

interface InstalledItemsInput {
  kind: MarketplaceKind;
  query: string;
  marketItems: readonly MarketplaceListing[];
  skills: readonly SkillPayload[];
  extensions: readonly {
    extension: ExtensionTrustSource & {
      name: string;
      version: string;
      enabled: boolean;
      marketplace?: MarketplaceListing | null;
    };
    listing: MarketplaceListing | null;
    updateAvailable: boolean;
  }[];
  mcpServers: readonly SettingsMCPServerEntry[];
  listingBySlug: Map<string, MarketplaceListing>;
  listingByEntryId: Map<string, MarketplaceListing>;
  listingByInstalledName: Map<string, MarketplaceListing>;
  listingByName: Map<string, MarketplaceListing>;
}

function matchesMarketplaceQuery(haystack: string, query: string): boolean {
  if (!query) return true;
  return haystack.toLowerCase().includes(query.toLowerCase());
}

function marketplaceListingHaystack(entry: MarketplaceListing): string {
  return [entry.name, entry.description, entry.author, entry.transport, entry.source]
    .filter(Boolean)
    .join(" ");
}

function mergeMCPServers(
  globalServers: readonly SettingsMCPServerEntry[],
  workspaceServers: readonly SettingsMCPServerEntry[]
): SettingsMCPServerEntry[] {
  const byName = new Map<string, SettingsMCPServerEntry>();
  for (const server of globalServers) byName.set(server.name, server);
  for (const server of workspaceServers) byName.set(server.name, server);
  return Array.from(byName.values());
}

function buildInstalledItems(input: InstalledItemsInput): MarketplaceInstalledItem[] {
  switch (input.kind) {
    case "extension":
      return buildInstalledExtensionItems(input);
    case "mcp":
      return buildInstalledMCPItems(input);
    case "skill":
      return buildInstalledSkillItems(input);
  }
}

function buildInstalledMCPItems(input: InstalledItemsInput): MarketplaceInstalledItem[] {
  const items: MarketplaceInstalledItem[] = [];
  for (const server of input.mcpServers) {
    const catalogEntry = server.catalog_entry?.trim();
    const listing =
      (catalogEntry ? input.listingByEntryId.get(catalogEntry) : undefined) ??
      input.listingByInstalledName.get(server.name) ??
      input.listingByName.get(server.name);
    const entry: MarketplaceListing = listing
      ? {
          ...listing,
          installed: true,
          installed_name: server.name,
          installed_version: server.catalog_version ?? listing.installed_version,
          transport: server.transport || listing.transport,
          update_available: listing.update_available,
        }
      : {
          entry_id: catalogEntry || server.name,
          kind: "mcp",
          name: server.name,
          description: "",
          installed: true,
          installed_name: server.name,
          installed_version: server.catalog_version,
          update_available: false,
          transport: server.transport,
          source: "installed",
          version: server.catalog_version,
        };
    const installed: MarketplaceInstalledItem = {
      entry,
      mcpServer: server,
      scopeLabel: mcpManagementScopeLabel(server) ?? undefined,
    };
    if (
      matchesMarketplaceQuery(
        [
          installed.entry.name,
          installed.entry.description,
          installed.entry.transport,
          installed.scopeLabel,
        ]
          .filter(Boolean)
          .join(" "),
        input.query
      )
    ) {
      items.push(installed);
    }
  }
  return items;
}

function buildInstalledSkillItems(input: InstalledItemsInput): MarketplaceInstalledItem[] {
  const items: MarketplaceInstalledItem[] = [];
  for (const skill of input.skills) {
    const slug = skill.provenance?.slug?.trim();
    const listing =
      (slug ? input.listingBySlug.get(slug) : undefined) ??
      input.listingByInstalledName.get(skill.name) ??
      input.listingByName.get(skill.name) ??
      input.listingByEntryId.get(skill.name);
    const entry: MarketplaceListing = listing
      ? {
          ...listing,
          installed: true,
          installed_name: skill.name,
          installed_version: skill.version ?? listing.installed_version,
          update_available: listing.update_available,
        }
      : {
          entry_id: skill.name,
          kind: "skill",
          name: skill.name,
          description: skill.description ?? "",
          installed: true,
          installed_name: skill.name,
          installed_version: skill.version,
          update_available: false,
          version: skill.version,
          source: skill.source,
        };
    const item: MarketplaceInstalledItem = { entry, skill };
    if (
      matchesMarketplaceQuery(
        [
          item.entry.name,
          item.entry.description,
          ...(Array.isArray(skill.metadata?.tags)
            ? skill.metadata.tags.filter((tag): tag is string => typeof tag === "string")
            : []),
        ]
          .filter(Boolean)
          .join(" "),
        input.query
      )
    ) {
      items.push(item);
    }
  }
  return items;
}

function buildInstalledExtensionItems(input: InstalledItemsInput): MarketplaceInstalledItem[] {
  const items: MarketplaceInstalledItem[] = [];
  for (const item of input.extensions) {
    const extension = item.extension;
    const listing = item.listing;
    const entry: MarketplaceListing = listing
      ? {
          ...listing,
          installed: true,
          installed_name: extension.name,
          installed_version: extension.version,
          update_available: item.updateAvailable,
        }
      : {
          entry_id: extension.name,
          kind: "extension",
          name: extension.name,
          description: "",
          installed: true,
          installed_name: extension.name,
          installed_version: extension.version,
          update_available: item.updateAvailable,
          version: extension.version,
          source: extension.source ?? "",
        };
    const installed: MarketplaceInstalledItem = {
      entry,
      extensionEnabled: extension.enabled,
      extensionFacts: extensionTrustFacts(extension),
    };
    if (matchesMarketplaceQuery(marketplaceListingHaystack(installed.entry), input.query)) {
      items.push(installed);
    }
  }
  return items;
}

export {
  buildInstalledItems,
  marketplaceListingHaystack,
  matchesMarketplaceQuery,
  mergeMCPServers,
};
