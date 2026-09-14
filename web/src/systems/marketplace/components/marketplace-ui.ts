import { ExtensionsApiError } from "@/systems/extensions";
import { MarketplaceApiError } from "../adapters/marketplace-api-error";
import type { MarketplaceCatalogListing } from "../types";

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
