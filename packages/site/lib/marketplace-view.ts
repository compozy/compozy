/**
 * The marketplace page's view model, derived from the catalog loaders (server only: it reads the
 * plugin snapshot; client components import the plain helpers from `lib/marketplace-cards`): one card shape for curated
 * extensions and preset-marketplace plugins, the daemon's fixed category order, featured, and the
 * flat search the page runs on the client.
 */
import {
  extensionEntries,
  marketplaceEntryPath,
  type ExtensionEntry,
} from "@/lib/marketplace-catalog";
import { inCategory, sortCards, type MarketplaceCard } from "@/lib/marketplace-cards";
import {
  CATALOG_SOURCE,
  marketplacePluginPath,
  marketplacePlugins,
  pluginDisplayName,
  pluginSnapshot,
  type MarketplacePlugin,
} from "@/lib/marketplace-plugins";

export { CATALOG_SOURCE };
export * from "@/lib/marketplace-cards";

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

export const marketplaceCards: MarketplaceCard[] = sortCards([
  ...extensionEntries.map(toCard),
  ...marketplacePlugins.map(pluginCard),
]);

/** The same entry's neighbours in its category, for the detail page. */
export function relatedCards(
  card: Pick<MarketplaceCard, "id" | "category">,
  limit = 6
): MarketplaceCard[] {
  return inCategory(marketplaceCards, card.category)
    .filter(item => item.id !== card.id)
    .slice(0, limit);
}
