import { extensionFixtures, type ExtensionEntry } from "@/systems/extensions";
import type { MarketplaceCatalogListing } from "@/systems/marketplace";

// Installed-side story rows: the daemon-joined listing and the inventory row it joins to.

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
    contents: { agents: 0, hooks: 0, loops: 0, mcp_servers: 0, skills: 0 },
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
    contents: { agents: 1, hooks: 0, loops: 0, mcp_servers: 0, skills: 2 },
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
