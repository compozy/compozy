import type { ExtensionEntry, InstalledExtensionView } from "@/systems/extensions";

import type { MarketplaceCatalogListing } from "../types";

export const COMPOZY_CATALOG_SOURCE = "compozy-catalog";

/** Stable origin key for a catalog row: `source_ref` when the daemon reports it, else the source name. */
export function marketplaceOriginKey(entry: {
  entry_id: string;
  source: string;
  source_ref?: string | null;
}): string {
  return `${entry.source_ref?.trim() || entry.source}:${entry.entry_id}`;
}

/** Pending/flash identity of an installed row: the local installed name plus its profile axis. */
export function installedExtensionKey(item: InstalledExtensionView): string {
  return `installed:${item.extension.profile}:${item.extension.name}`;
}

/** Logo identity for an installed row: the joined listing, else the origin entry id, else the name. */
export function installedLogoEntry(item: InstalledExtensionView): {
  entry_id: string;
  name: string;
  icon?: string | null;
  install_slug?: string | null;
} {
  if (item.listing) return item.listing;
  return {
    entry_id: item.extension.origin?.entry_id ?? item.extension.name,
    name: item.extension.name,
  };
}

/** The row title: the catalog display name when joined, else the local installed name. */
export function installedDisplayName(item: InstalledExtensionView): string {
  return item.listing?.name ?? item.extension.name;
}

/** Marketplace origin word for rows that came from a source other than the CompozyOS catalog. */
export function installedOriginWord(item: InstalledExtensionView): string | null {
  const source = item.extension.origin?.source?.trim();
  return source && source !== COMPOZY_CATALOG_SOURCE ? source : null;
}

/** Scope word only when the instance is not global. */
export function installedScopeWord(extension: ExtensionEntry): string | null {
  const workspace = extension.workspace_id?.trim();
  if (workspace) return `workspace · ${workspace}`;
  const profile = extension.installation_profile?.trim();
  if (profile) return `profile · ${profile}`;
  const hasProfileServer = extension.placements?.some(
    placement =>
      placement.kind === "mcp_server" &&
      !placement.dormant &&
      placement.profile === extension.profile
  );
  return hasProfileServer ? `profile · ${extension.profile}` : null;
}

const CONTENT_NOUNS: ReadonlyArray<
  [key: keyof ExtensionEntry["contents"], singular: string, plural: string]
> = [
  ["mcp_servers", "MCP server", "MCP servers"],
  ["skills", "skill", "skills"],
  ["agents", "agent", "agents"],
  ["loops", "loop", "loops"],
  ["hooks", "hook", "hooks"],
  ["bridges", "bridge", "bridges"],
];

/** "1 MCP server · 2 skills" from the inventory summary; zero of a thing renders nothing. */
export function formatExtensionContents(contents: ExtensionEntry["contents"] | undefined): string {
  if (!contents) return "";
  return CONTENT_NOUNS.flatMap(([key, singular, plural]) => {
    const count = contents[key] ?? 0;
    return count > 0 ? [`${count} ${count === 1 ? singular : plural}`] : [];
  }).join(" · ");
}

/** Catalog detail search that keeps the installed identity of a row. */
export function installedDetailSearch(item: InstalledExtensionView): {
  installed_name: string;
  scope: "user" | "workspace";
  profile: string;
  workspace_id?: string;
  source?: string;
} {
  const source = item.listing?.source ?? item.extension.origin?.source;
  return {
    installed_name: item.extension.name,
    profile: item.extension.profile,
    scope: item.extension.workspace_id ? "workspace" : "user",
    ...(item.extension.workspace_id ? { workspace_id: item.extension.workspace_id } : {}),
    ...(source ? { source } : {}),
  };
}

/** Detail search for a catalog row: the source name, and the installed name once joined. */
export function catalogDetailSearch(entry: MarketplaceCatalogListing): {
  source: string;
  installed_name?: string;
} {
  return {
    source: entry.source,
    ...(entry.installed && entry.installed_name ? { installed_name: entry.installed_name } : {}),
  };
}

/** The entry id the detail route resolves for an installed row. */
export function installedEntryId(item: InstalledExtensionView): string {
  return item.listing?.entry_id ?? item.extension.origin?.entry_id ?? item.extension.name;
}
