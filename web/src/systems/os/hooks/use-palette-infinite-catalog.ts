import { useEffect } from "react";

export interface PaletteInfiniteCatalog {
  readonly hasNextPage: boolean;
  readonly isFetchingNextPage: boolean;
  readonly isError: boolean;
  readonly fetchNextPage: () => Promise<unknown>;
}

/** Follow the cursor until the filtered catalog is complete. */
export function usePaletteInfiniteCatalog(query: PaletteInfiniteCatalog, enabled: boolean): void {
  useEffect(() => {
    if (!enabled || !query.hasNextPage || query.isFetchingNextPage || query.isError) return;
    // React Query records a continuation failure on the query; the consumer
    // reads that state, so the promise itself needs no second error channel.
    void query.fetchNextPage().catch(() => undefined);
  }, [enabled, query]);
}
