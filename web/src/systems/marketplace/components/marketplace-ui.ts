import { ExtensionsApiError } from "@/systems/extensions";
import { MarketplaceApiError } from "../adapters/marketplace-api-error";
import type { MarketplaceCatalogListing } from "../types";

/**
 * The row-card layout every catalog and Installed card shares: logo · text · trail. The entry card
 * and its loading skeleton both take it, so a placeholder card cannot drift from the real one.
 */
export const MARKETPLACE_ENTRY_CARD_CLASS =
  "grid min-h-15 grid-cols-[var(--size-provider-logo-well)_minmax(0,1fr)_auto] items-center gap-3 px-3 py-2.5";

export function marketplaceEntrySlug(entry: MarketplaceCatalogListing): string {
  return entry.install_slug?.trim() || entry.entry_id;
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

export function marketplaceErrorCode(error: unknown): string | undefined {
  if (error instanceof MarketplaceApiError) return error.diagnosticCode;
  if (error instanceof ExtensionsApiError) return error.code;
  return undefined;
}

interface MarketplaceTrustSubject {
  tier?: string;
  trust?: { checksum_verified: boolean } | null;
}

/** Plain-language tier word: the catalog's own tier, never the registry enum. */
export function marketplaceTierLabel(tier: string | null | undefined): string {
  return tier === "official" ? "Official" : "Community";
}

/** One-sentence trust summary shared by the detail rail and the install summary. */
export function marketplaceTrustSentence(entry: MarketplaceTrustSubject): string {
  const checked = entry.trust?.checksum_verified ? "checked by CompozyOS" : "not verified";
  return `${marketplaceTierLabel(entry.tier)} · ${checked}`;
}
