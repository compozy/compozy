import { browseMarketplace } from "../adapters/marketplace-api";
import { isMarketplaceCursorStale, MarketplaceApiError } from "../adapters/marketplace-api-error";
import type { MarketplaceCatalogOptions, MarketplaceCatalogResponse } from "../types";

/** Publish a complete revision atomically so counts and sections never include partial pages. */
export async function readMarketplaceSnapshot(
  options: MarketplaceCatalogOptions,
  signal: AbortSignal
) {
  for (let restarts = 0; ; restarts++) {
    const pages: MarketplaceCatalogResponse[] = [];
    const cursors = new Set<string>();
    let cursor: string | undefined;
    try {
      do {
        signal.throwIfAborted();
        const page = await browseMarketplace({ ...options, cursor }, signal);
        if (pages.length > 0 && page.revision !== pages[0]?.revision) {
          throw new MarketplaceApiError(
            "The catalog changed while loading",
            409,
            "marketplace_cursor_stale",
            true
          );
        }
        pages.push(page);
        cursor = page.next_cursor || undefined;
        if (cursor && cursors.has(cursor)) {
          throw new MarketplaceApiError("The catalog returned a repeated page cursor", 400);
        }
        if (cursor) cursors.add(cursor);
      } while (cursor);
      return { pages };
    } catch (error) {
      if (!isMarketplaceCursorStale(error) || restarts >= 3 || signal.aborted) throw error;
    }
  }
}
