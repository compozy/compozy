import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { marketplaceDetails, marketplaceKindFixture, marketplaceSearchFixture } from "../../mocks";
import { useMarketplaceEntry, useMarketplaceKind, useMarketplaceSearch } from "../use-marketplace";

const mocks = vi.hoisted(() => ({
  browse: vi.fn(),
  detail: vi.fn(),
  search: vi.fn(),
}));

vi.mock("../../adapters/marketplace-api", () => ({
  browseMarketplaceKind: mocks.browse,
  getMarketplaceEntry: mocks.detail,
  searchMarketplace: mocks.search,
}));

function setup() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client }, children);
  return { client, wrapper };
}

describe("marketplace query hooks", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("Should load grouped discovery through the public hook", async () => {
    mocks.search.mockResolvedValue(marketplaceSearchFixture);
    const { wrapper } = setup();
    const options = { limit: 3, q: "audit", workspaceId: "ws-a" };

    const { result } = renderHook(() => useMarketplaceSearch(options), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual(marketplaceSearchFixture);
    expect(mocks.search).toHaveBeenCalledWith(options, expect.any(AbortSignal));
  });

  it("Should isolate kind browse and entry detail in their own hooks", async () => {
    const kindResponse = marketplaceKindFixture("bundle");
    const detailResponse = marketplaceDetails["bundle:dep-kit"]!;
    mocks.browse.mockResolvedValue(kindResponse);
    mocks.detail.mockResolvedValue(detailResponse);
    const { client, wrapper } = setup();

    const kind = renderHook(
      () => useMarketplaceKind({ kind: "bundle", limit: 20, workspaceId: "ws-a" }),
      { wrapper }
    );
    const detail = renderHook(
      () => useMarketplaceEntry({ entryId: "dep-kit", kind: "bundle", workspaceId: "ws-a" }),
      { wrapper }
    );

    await waitFor(() => expect(kind.result.current.data?.pages).toEqual([kindResponse]));
    await waitFor(() => expect(detail.result.current.data).toEqual(detailResponse));
    expect(client.getQueryCache().getAll()).toHaveLength(2);
  });
});
