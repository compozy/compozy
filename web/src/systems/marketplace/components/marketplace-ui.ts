import { Box, Plug, Puzzle, Wrench, type LucideIcon } from "lucide-react";

import type { MarketplaceKind, MarketplaceListing } from "../types";

export type MarketplaceViewSort = "relevance" | "downloads" | "name";

const MARKETPLACE_KIND_ICON: Record<MarketplaceKind, LucideIcon> = {
  skill: Wrench,
  extension: Puzzle,
  bundle: Box,
  mcp: Plug,
};

export function marketplaceKindIcon(kind: MarketplaceKind): LucideIcon {
  return MARKETPLACE_KIND_ICON[kind] ?? Box;
}

export const MARKETPLACE_KIND_ORDER: readonly MarketplaceKind[] = [
  "skill",
  "mcp",
  "extension",
  "bundle",
];

export const MARKETPLACE_KIND_LABEL: Record<MarketplaceKind, string> = {
  skill: "Skills",
  extension: "Extensions",
  bundle: "Bundles",
  mcp: "MCPs",
};

export const MARKETPLACE_KIND_SINGULAR: Record<MarketplaceKind, string> = {
  skill: "skill",
  extension: "extension",
  bundle: "bundle",
  mcp: "MCP server",
};

const STANDARD_COUNT_FORMATTER = new Intl.NumberFormat("en", {
  notation: "standard",
  maximumFractionDigits: 1,
});
const COMPACT_COUNT_FORMATTER = new Intl.NumberFormat("en", {
  notation: "compact",
  maximumFractionDigits: 1,
});

export function isMarketplaceKind(value: unknown): value is MarketplaceKind {
  return typeof value === "string" && MARKETPLACE_KIND_ORDER.includes(value as MarketplaceKind);
}

export function isMarketplaceViewSort(value: unknown): value is MarketplaceViewSort {
  return value === "relevance" || value === "downloads" || value === "name";
}

export function marketplaceEntrySlug(entry: MarketplaceListing): string {
  return entry.install_slug?.trim() || entry.entry_id;
}

export function formatMarketplaceCount(value: number): string {
  return (value >= 1_000 ? COMPACT_COUNT_FORMATTER : STANDARD_COUNT_FORMATTER).format(value);
}

export function sortMarketplaceEntries(
  entries: readonly MarketplaceListing[],
  sort: MarketplaceViewSort
): MarketplaceListing[] {
  const sorted = [...entries];
  if (sort === "downloads") {
    return sorted.sort((left, right) => (right.downloads ?? -1) - (left.downloads ?? -1));
  }
  if (sort === "name") {
    return sorted.sort((left, right) => left.name.localeCompare(right.name));
  }
  return sorted;
}

export function marketplaceErrorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message.trim() !== "" ? error.message : fallback;
}
