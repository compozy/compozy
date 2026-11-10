/**
 * The preset plugin marketplaces, read from the checked-in snapshot and projected the way the daemon
 * projects them (`internal/marketplace/{category,projection,sections}.go`): category resolution,
 * curated featured flags, and the supersede rule against the curated extensions.
 */
import { extensionEntries } from "@/lib/marketplace-catalog";
import { MARKETPLACE_CATEGORY_IDS } from "@/lib/marketplace-catalog-schema";
import { pluginSnapshotSchema, type SnapshotPlugin } from "@/lib/marketplace-plugins-schema";
import snapshotFile from "@/lib/marketplace-plugins.snapshot.json";

export type MarketplaceCategoryId = (typeof MARKETPLACE_CATEGORY_IDS)[number];

/** The name the daemon gives the curated extension feed's source. */
export const CATALOG_SOURCE = "compozy-catalog";

export const pluginSnapshot = pluginSnapshotSchema.parse(snapshotFile);

/** `categoryAliases`: a publisher category, lowercased with `-`/`_` as spaces, maps to one id. */
const CATEGORY_ALIASES: Record<MarketplaceCategoryId, readonly string[]> = {
  productivity: [
    "productivity",
    "documents and files",
    "scheduling",
    "automation",
    "knowledge",
    "notes",
  ],
  communication: ["communication", "inbox and collaboration", "customer support", "chat", "email"],
  design: ["design", "canvas", "creativity"],
  development: ["development", "developer tools", "testing", "migration", "code"],
  data: ["data", "database", "data analytics", "data and analytics", "analytics"],
  cloud: ["infrastructure", "deployment", "monitoring", "cloud", "devops"],
  finance: ["finance", "payments", "finance and legal"],
  "sales-marketing": ["sales marketing", "sales", "marketing"],
  research: ["research", "learning", "education and research", "math"],
  security: ["security"],
  "ai-agents": ["ai agents", "agent orchestration", "ai", "agents"],
  more: ["more", "location"],
};

export function isCategoryId(value: string | null | undefined): value is MarketplaceCategoryId {
  return MARKETPLACE_CATEGORY_IDS.some(id => id === value);
}

/** `ResolveCategory`: an explicit CompozyOS category wins, then the alias table, then More. */
export function resolveCategory(explicit: string | undefined, sourceCategory: string | undefined) {
  if (isCategoryId(explicit)) return explicit;
  const normalized = (sourceCategory ?? "")
    .toLowerCase()
    .replace(/[-_]/g, " ")
    .split(/\s+/)
    .filter(Boolean)
    .join(" ");
  for (const id of MARKETPLACE_CATEGORY_IDS) {
    if (CATEGORY_ALIASES[id].includes(normalized)) return id;
  }
  return "more" satisfies MarketplaceCategoryId;
}

/** One listed plugin with its public route id and resolved category. */
export interface MarketplacePlugin {
  /** Route segment under `/marketplace/`; see `pluginEntryId`. */
  id: string;
  category: MarketplaceCategoryId;
  plugin: SnapshotPlugin;
}

/**
 * `visibleCategoryCandidates` with nothing installed: every superseded `<source>/<entry_id>` is hidden.
 * The public site has no daemon, so each superseding entry counts as installable.
 */
function supersededKeys(plugins: readonly SnapshotPlugin[]): Set<string> {
  const hidden = new Set<string>();
  for (const entry of extensionEntries) {
    for (const key of entry["x-compozy"]?.supersedes ?? []) {
      if (key !== `${CATALOG_SOURCE}/${entry.entry_id}`) hidden.add(key);
    }
  }
  for (const plugin of plugins) {
    for (const key of plugin.supersedes ?? []) {
      if (key !== `${plugin.source}/${plugin.name}`) hidden.add(key);
    }
  }
  return hidden;
}

/**
 * Route ids never collide: curated extensions keep their entry id; a plugin takes its bare name when
 * no extension and no other listed plugin uses it, and `<name>~<source>` otherwise (`~` cannot appear
 * in a plugin name, so the suffixed form cannot clash with a bare one).
 */
export function pluginEntryIds(
  plugins: readonly SnapshotPlugin[],
  extensionIds: ReadonlySet<string>
): Map<SnapshotPlugin, string> {
  const uses = new Map<string, number>();
  for (const plugin of plugins) uses.set(plugin.name, (uses.get(plugin.name) ?? 0) + 1);
  const ids = new Map<SnapshotPlugin, string>();
  for (const plugin of plugins) {
    const shared = extensionIds.has(plugin.name) || (uses.get(plugin.name) ?? 0) > 1;
    ids.set(plugin, shared ? `${plugin.name}~${plugin.source}` : plugin.name);
  }
  return ids;
}

export function listedPlugins(snapshotPlugins: readonly SnapshotPlugin[]): MarketplacePlugin[] {
  const hidden = supersededKeys(snapshotPlugins);
  const visible = snapshotPlugins.filter(plugin => !hidden.has(`${plugin.source}/${plugin.name}`));
  const ids = pluginEntryIds(visible, new Set(extensionEntries.map(entry => entry.entry_id)));
  return visible.map(plugin => ({
    id: ids.get(plugin) ?? plugin.name,
    category: resolveCategory(plugin.compozy_category, plugin.category),
    plugin,
  }));
}

export const marketplacePlugins: MarketplacePlugin[] = listedPlugins(pluginSnapshot.plugins);

/** Words the draft's snapshot script spelled in capitals when it humanised a slug. */
const ACRONYMS: Record<string, string> = {
  aws: "AWS",
  cli: "CLI",
  api: "API",
  mcp: "MCP",
  sql: "SQL",
  ai: "AI",
  db: "DB",
  gcp: "GCP",
  sdk: "SDK",
  twg: "TWG",
  pr: "PR",
  seo: "SEO",
  crm: "CRM",
  adlc: "ADLC",
  lsp: "LSP",
};

/** Python's `str.capitalize`: first character upper, the rest lower. */
function capitalize(word: string): string {
  return word.slice(0, 1).toUpperCase() + word.slice(1).toLowerCase();
}

/**
 * The name the public page shows (the approved draft's rule): the feed's `displayName`, otherwise
 * the slug humanised: kept as is when it already has capitals and no separators, else split on
 * `-`/`_` with each word capitalised or spelled as a known acronym. Routes, ids and commands keep
 * the raw slug.
 */
export function pluginDisplayName(plugin: Pick<SnapshotPlugin, "name" | "display_name">): string {
  if (plugin.display_name) return plugin.display_name;
  const { name } = plugin;
  if (!/[-_]/.test(name) && name !== name.toLowerCase()) return name;
  return name
    .split(/[-_]/)
    .map(word => ACRONYMS[word.toLowerCase()] ?? capitalize(word))
    .join(" ");
}

export function findPlugin(id: string): MarketplacePlugin | undefined {
  return marketplacePlugins.find(entry => entry.id === id);
}

export function marketplacePluginPath(entry: Pick<MarketplacePlugin, "id">): string {
  return `/marketplace/${encodeURIComponent(entry.id)}`;
}

export function pluginSearchCommand(entry: MarketplacePlugin): string {
  return `compozy marketplace search ${entry.plugin.name}`;
}
