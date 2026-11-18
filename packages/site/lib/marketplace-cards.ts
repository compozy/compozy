/**
 * The marketplace card shape and the pure helpers over it. This module holds no catalog data, so
 * client components can import it without pulling the plugin snapshot into their bundle.
 */
export const MARKETPLACE_CATEGORY_IDS = [
  "productivity",
  "communication",
  "design",
  "development",
  "data",
  "cloud",
  "finance",
  "sales-marketing",
  "research",
  "security",
  "ai-agents",
  "more",
] as const;

export type MarketplaceCategoryId = (typeof MARKETPLACE_CATEGORY_IDS)[number];

export function isCategoryId(value: string | null | undefined): value is MarketplaceCategoryId {
  return MARKETPLACE_CATEGORY_IDS.some(id => id === value);
}

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
