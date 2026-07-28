import type { BundleActivation } from "@/systems/extensions";
import { mcpManagementScopeLabel, type SettingsMCPServerEntry } from "@/systems/settings";
import type { SkillPayload } from "@/systems/skill";

import type { MarketplaceKind, MarketplaceListing } from "../types";

export interface MarketplaceInstalledItem {
  entry: MarketplaceListing;
  skill?: SkillPayload;
  extensionEnabled?: boolean;
  viaBundle?: string | null;
  activationId?: string;
  activationVersion?: number;
  profileName?: string;
  scopeLabel?: string;
  mcpServer?: SettingsMCPServerEntry;
}

interface InstalledItemsInput {
  kind: MarketplaceKind;
  query: string;
  marketItems: readonly MarketplaceListing[];
  skills: readonly SkillPayload[];
  extensions: readonly {
    extension: {
      name: string;
      version: string;
      enabled: boolean;
      marketplace?: MarketplaceListing | null;
    };
    listing: MarketplaceListing | null;
    updateAvailable: boolean;
  }[];
  activations: readonly BundleActivation[];
  mcpServers: readonly SettingsMCPServerEntry[];
  listingBySlug: Map<string, MarketplaceListing>;
  listingByEntryId: Map<string, MarketplaceListing>;
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
  if (input.kind === "mcp") return buildInstalledMCPItems(input);
  if (input.kind === "skill") return buildInstalledSkillItems(input);
  if (input.kind === "extension") return buildInstalledExtensionItems(input);
  return buildInstalledBundleItems(input);
}

function buildInstalledMCPItems(input: InstalledItemsInput): MarketplaceInstalledItem[] {
  const items: MarketplaceInstalledItem[] = [];
  for (const server of input.mcpServers) {
    const catalogEntry = server.catalog_entry?.trim();
    const listing =
      (catalogEntry ? input.listingByEntryId.get(catalogEntry) : undefined) ??
      input.marketItems.find(
        entry =>
          entry.entry_id === catalogEntry ||
          entry.installed_name === server.name ||
          entry.name === server.name
      );
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
  const activationsById = new Map(
    input.activations.map(activation => [activation.id, activation] as const)
  );
  const items: MarketplaceInstalledItem[] = [];
  for (const skill of input.skills) {
    const installedFromBundle = skill.provenance?.installed_from_bundle ?? null;
    const bundleActivation = installedFromBundle
      ? activationsById.get(installedFromBundle)
      : undefined;
    const slug = skill.provenance?.slug?.trim();
    const listing =
      (slug ? input.listingBySlug.get(slug) : undefined) ??
      input.marketItems.find(
        entry =>
          entry.installed_name === skill.name ||
          entry.name === skill.name ||
          entry.entry_id === skill.name
      );
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
    const item: MarketplaceInstalledItem = {
      entry,
      skill,
      viaBundle: bundleActivation
        ? `${bundleActivation.bundle_name}/${bundleActivation.profile_name}`
        : installedFromBundle,
      activationId: bundleActivation?.id,
    };
    if (
      matchesMarketplaceQuery(
        [
          item.entry.name,
          item.entry.description,
          item.viaBundle,
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
          source: "",
        };
    const installed: MarketplaceInstalledItem = {
      entry,
      extensionEnabled: extension.enabled,
    };
    if (matchesMarketplaceQuery(marketplaceListingHaystack(installed.entry), input.query)) {
      items.push(installed);
    }
  }
  return items;
}

function buildInstalledBundleItems(input: InstalledItemsInput): MarketplaceInstalledItem[] {
  const items: MarketplaceInstalledItem[] = [];
  const listingByIdentity = new Map<string, MarketplaceListing>();
  for (const entry of input.marketItems) {
    listingByIdentity.set(`${entry.source}\u0000${entry.name}`, entry);
  }
  for (const activation of input.activations) {
    const listing = listingByIdentity.get(
      `${activation.extension_name}\u0000${activation.bundle_name}`
    );
    const entry: MarketplaceListing = listing
      ? {
          ...listing,
          installed: true,
          installed_name: activation.bundle_name,
          update_available: activation.spec_drift === true,
        }
      : {
          entry_id: activation.bundle_name,
          kind: "bundle",
          name: activation.bundle_name,
          description: "",
          installed: true,
          installed_name: activation.bundle_name,
          update_available: activation.spec_drift === true,
          source: activation.extension_name,
        };
    const installed: MarketplaceInstalledItem = {
      entry,
      activationId: activation.id,
      activationVersion: activation.version,
      profileName: activation.profile_name,
      scopeLabel: activation.workspace_id
        ? `${activation.scope} · ${activation.workspace_id}`
        : activation.scope,
    };
    if (
      matchesMarketplaceQuery(
        [
          installed.entry.name,
          installed.entry.description,
          installed.profileName,
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

export {
  buildInstalledItems,
  marketplaceListingHaystack,
  matchesMarketplaceQuery,
  mergeMCPServers,
};
