/** The hero's floating logo field, filled from the live catalog. */
import type { MarketplaceCard } from "@/lib/marketplace-view";

/** position across its gutter %, top px, drift depth, tile size. Depth reads as distance: far tiles are smaller and dimmer. */
type Slot = readonly [number, number, number, "md" | "lg" | "xl"];

/** The draft's field, with the entry it would prefer in each slot. */
const LEFT: ReadonlyArray<readonly [string, ...Slot]> = [
  ["figma", 18, 214, 0.9, "xl"],
  ["linear", 72, 150, 0.6, "lg"],
  ["github", 44, 352, 0.45, "md"],
  ["sentry", 0, 420, 0.5, "md"],
  ["notion", 100, 292, 1, "xl"],
  ["datadog", 30, 512, 0.7, "lg"],
  ["stripe", 92, 470, 0.5, "md"],
];

const RIGHT: ReadonlyArray<readonly [string, ...Slot]> = [
  ["slack", 0, 268, 1, "xl"],
  ["supabase", 62, 180, 0.7, "lg"],
  ["vercel", 20, 150, 0.45, "md"],
  ["asana", 88, 348, 0.5, "md"],
  ["atlassian", 40, 416, 0.85, "lg"],
  ["miro", 94, 520, 0.6, "lg"],
  ["gitlab", 0, 500, 0.45, "md"],
];

export interface Tile {
  entry: MarketplaceCard;
  slot: Slot;
}

/**
 * Fills the field from the live catalog: a slot keeps its preferred entry when the catalog has it,
 * and otherwise takes the next entry with an icon that is not placed yet. Slots left over stay empty.
 */
export function fieldTiles(entries: readonly MarketplaceCard[]): { left: Tile[]; right: Tile[] } {
  const withIcon = entries.filter(entry => entry.icon);
  const byId = new Map(withIcon.map(entry => [entry.id, entry]));
  const slots = [...LEFT, ...RIGHT];
  const placed = new Set<string>();
  for (const [id] of slots) if (byId.has(id)) placed.add(id);
  const spare = withIcon.filter(entry => !placed.has(entry.id));
  const tiles = slots.map(([id, ...slot]) => {
    const entry = byId.get(id) ?? spare.shift();
    return entry ? { entry, slot } : null;
  });
  return {
    left: tiles.slice(0, LEFT.length).filter(tile => tile !== null),
    right: tiles.slice(LEFT.length).filter(tile => tile !== null),
  };
}
