import type { MarketplaceCatalogListing } from "../types";

const STANDARD_COUNT_FORMATTER = new Intl.NumberFormat("en", {
  notation: "standard",
  maximumFractionDigits: 1,
});
const COMPACT_COUNT_FORMATTER = new Intl.NumberFormat("en", {
  notation: "compact",
  maximumFractionDigits: 1,
});

export function marketplaceEntrySlug(entry: MarketplaceCatalogListing): string {
  return entry.install_slug?.trim() || entry.entry_id;
}

export function formatMarketplaceCount(value: number): string {
  return (value >= 1_000 ? COMPACT_COUNT_FORMATTER : STANDARD_COUNT_FORMATTER).format(value);
}

/** Public marketplace versions can arrive as exact release tags with a v/V prefix. */
export function formatMarketplaceVersion(version: string | null | undefined): string | null {
  const trimmed = version?.trim();
  if (!trimmed) return null;
  return `v${trimmed.replace(/^[vV]+/, "")}`;
}

export function marketplaceErrorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message.trim() !== "" ? error.message : fallback;
}
