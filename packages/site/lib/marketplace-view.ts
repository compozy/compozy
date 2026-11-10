/**
 * The marketplace page's view model, derived from the catalog loaders: one card shape for curated
 * extensions and preset-marketplace plugins, the daemon's fixed category order, featured, and the
 * flat search the page runs on the client.
 */
import {
  extensionEntries,
  marketplaceEntryPath,
  type ExtensionEntry,
} from "@/lib/marketplace-catalog";
import { MARKETPLACE_CATEGORY_IDS } from "@/lib/marketplace-catalog-schema";
import {
  CATALOG_SOURCE,
  isCategoryId,
  marketplacePluginPath,
  marketplacePlugins,
  pluginDisplayName,
  pluginSnapshot,
  type MarketplaceCategoryId,
  type MarketplacePlugin,
} from "@/lib/marketplace-plugins";

export { CATALOG_SOURCE, isCategoryId, type MarketplaceCategoryId };

export const CATEGORY_LABELS: Record<MarketplaceCategoryId, string> = {
  productivity: "Productivity",
  communication: "Communication",
  design: "Design",
  development: "Development",
  data: "Data & Analytics",
  cloud: "Cloud & Infrastructure",
  finance: "Finance & Payments",
  "sales-marketing": "Sales & Marketing",
  research: "Research & Learning",
  security: "Security",
  "ai-agents": "AI & Agents",
  more: "More",
};

/** Plain data, safe to pass from the server pages into the client browser. */
export interface MarketplaceCard {
  id: string;
  kind: "extension" | "plugin";
  name: string;
  author: string | null;
  category: MarketplaceCategoryId;
  description: string;
  icon: string | null;
  /** Logo tile hints: the logo fills the tile, or needs the light tile. */
  bleed: boolean;
  light: boolean;
  featured: boolean;
  href: string;
}

export function toCard(entry: ExtensionEntry): MarketplaceCard {
  return {
    id: entry.entry_id,
    kind: "extension",
    name: entry.name,
    author: entry.author?.trim() || null,
    category: entry["x-compozy"]?.category ?? "more",
    description: entry.description,
    icon: entry.icon?.trim() || null,
    bleed: pluginSnapshot.extension_icons[entry.entry_id]?.bleed === true,
    light: pluginSnapshot.extension_icons[entry.entry_id]?.light === true,
    featured: entry["x-compozy"]?.featured === true,
    href: marketplaceEntryPath(entry),
  };
}

export function pluginCard({ id, category, plugin }: MarketplacePlugin): MarketplaceCard {
  return {
    id,
    kind: "plugin",
    name: pluginDisplayName(plugin),
    author: plugin.author?.trim() || null,
    category,
    description: plugin.description?.trim() ?? "",
    icon: plugin.icon ?? null,
    bleed: plugin.bleed === true,
    light: plugin.light === true,
    featured: plugin.featured === true,
    href: marketplacePluginPath({ id }),
  };
}

const NAME_COLLATOR = new Intl.Collator("en", { sensitivity: "base" });

/**
 * Featured first, then entries with an icon, so monograms never push logos out of a section's
 * visible cards; inside each group the daemon's `sortCategoryCandidates` order (shown name, then id).
 */
export function sortCards(cards: readonly MarketplaceCard[]): MarketplaceCard[] {
  return cards.toSorted(
    (a, b) =>
      Number(b.featured) - Number(a.featured) ||
      Number(Boolean(b.icon)) - Number(Boolean(a.icon)) ||
      NAME_COLLATOR.compare(a.name, b.name) ||
      (a.id < b.id ? -1 : a.id > b.id ? 1 : 0)
  );
}

export const marketplaceCards: MarketplaceCard[] = sortCards([
  ...extensionEntries.map(toCard),
  ...marketplacePlugins.map(pluginCard),
]);

export function categoryLabel(id: string): string {
  return isCategoryId(id) ? CATEGORY_LABELS[id] : CATEGORY_LABELS.more;
}

export function inCategory(cards: readonly MarketplaceCard[], id: string): MarketplaceCard[] {
  return cards.filter(card => card.category === id);
}

/** Categories in the daemon's order, with their counts; empty ones are left out. */
export function categoriesWithEntries(
  cards: readonly MarketplaceCard[]
): Array<{ id: MarketplaceCategoryId; label: string; count: number }> {
  const categories: Array<{ id: MarketplaceCategoryId; label: string; count: number }> = [];
  for (const id of MARKETPLACE_CATEGORY_IDS) {
    const count = inCategory(cards, id).length;
    if (count > 0) categories.push({ id, label: CATEGORY_LABELS[id], count });
  }
  return categories;
}

/** Feed descriptions run long; a card only needs the first sentence. */
export function firstSentence(text: string): string {
  const match = /^.+?[.!?](?=\s|$)/.exec(text);
  return match ? match[0] : text;
}

export function matchesCard(card: MarketplaceCard, needle: string): boolean {
  return `${card.name} ${card.author ?? ""} ${card.description} ${card.id}`
    .toLowerCase()
    .includes(needle);
}

/** The same entry's neighbours in its category, for the detail page. */
export function relatedCards(
  card: Pick<MarketplaceCard, "id" | "category">,
  limit = 6
): MarketplaceCard[] {
  return inCategory(marketplaceCards, card.category)
    .filter(item => item.id !== card.id)
    .slice(0, limit);
}

export function tierLabel(tier: string): string {
  return tier.slice(0, 1).toUpperCase() + tier.slice(1);
}

export function hostnameOf(url: string): string {
  try {
    return new URL(url).hostname.replace(/^www\./, "");
  } catch {
    return url;
  }
}
