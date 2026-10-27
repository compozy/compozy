// Suite: Loops catalog query-state contract
// Invariant: the catalog is pending only while it has no rows and no failure, whichever read backs
// it, and while a search or filter change shows the previous rows the cursor behind them is not
// the new filter's, so paging waits for the new first page.
// Owning layer: `loopsCatalogQueryState`, the one loading/paging contract the catalog location reads.
import { describe, expect, it, vi } from "vitest";

import { loopsCatalogQueryState } from "../loops-catalog-model";

type Batch = Parameters<typeof loopsCatalogQueryState>[1];
type Paged = Parameters<typeof loopsCatalogQueryState>[2];

function paged(overrides: Partial<Record<keyof Paged, unknown>> = {}): Paged {
  return {
    isPending: false,
    isPlaceholderData: false,
    isFetchingNextPage: false,
    hasNextPage: true,
    fetchNextPage: vi.fn(),
    refetch: vi.fn(),
    error: null,
    ...overrides,
  } as unknown as Paged;
}

function batch(overrides: Partial<Record<keyof Batch, unknown>> = {}): Batch {
  return { isPending: false, error: null, refetch: vi.fn(), ...overrides } as unknown as Batch;
}

describe("loopsCatalogQueryState", () => {
  it("Should report the read that backs the catalog as pending, and only that one", () => {
    // The project catalog reads pages; the All projects batch is disabled there, so it is
    // permanently "pending" by TanStack's definition and must never show through.
    expect(loopsCatalogQueryState(false, batch({ isPending: true }), paged()).isPending).toBe(
      false
    );
    expect(loopsCatalogQueryState(false, batch(), paged({ isPending: true })).isPending).toBe(true);
    // And the other way round under All projects.
    expect(loopsCatalogQueryState(true, batch({ isPending: true }), paged()).isPending).toBe(true);
    expect(loopsCatalogQueryState(true, batch(), paged({ isPending: true })).isPending).toBe(false);
  });

  it("Should hold paging while the rows shown belong to the previous filters", () => {
    const fetchNextPage = vi.fn();
    const settled = loopsCatalogQueryState(false, batch(), paged({ fetchNextPage }));
    expect(settled.hasNextPage).toBe(true);
    settled.fetchNextPage();
    expect(fetchNextPage).toHaveBeenCalledOnce();

    fetchNextPage.mockClear();
    const bridging = loopsCatalogQueryState(
      false,
      batch(),
      paged({ fetchNextPage, isPlaceholderData: true })
    );
    expect(bridging.hasNextPage).toBe(false);
    bridging.fetchNextPage();
    expect(fetchNextPage).not.toHaveBeenCalled();
  });
});
