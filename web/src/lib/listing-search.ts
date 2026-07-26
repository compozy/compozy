import type { ListingViewMode } from "@agh/ui";

/** Trim a raw search-param value; blank/whitespace/non-string → undefined. */
export function normalizeListingSearchValue(value: unknown): string | undefined {
  if (typeof value !== "string") {
    return undefined;
  }
  const trimmed = value.trim();
  return trimmed === "" ? undefined : trimmed;
}

/** Parse the `view` search param; anything other than the two modes → undefined. */
export function parseListingView(value: unknown): ListingViewMode | undefined {
  return value === "rows" || value === "cards" ? value : undefined;
}
