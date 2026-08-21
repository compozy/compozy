// Suite: marketplace query hooks
// Invariant: marketplace reads retain kind and entry scope while honoring liveness gates.
// Boundary IN: query options, pagination state, cache keys, and hook projections.
// Boundary OUT: installation interactions and rendering, owned by component suites.
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
    const kindResponse = marketplaceKindFixture("mcp");
    const detailResponse = marketplaceDetails["mcp:github"]!;
    mocks.browse.mockResolvedValue(kindResponse);
    mocks.detail.mockResolvedValue(detailResponse);
    const { client, wrapper } = setup();

    const kind = renderHook(
      () => useMarketplaceKind({ kind: "mcp", limit: 20, workspaceId: "ws-a" }),
      { wrapper }
    );
    const detail = renderHook(
      () => useMarketplaceEntry({ entryId: "github", kind: "mcp", workspaceId: "ws-a" }),
      { wrapper }
    );

    await waitFor(() => expect(kind.result.current.data?.pages).toEqual([kindResponse]));
    await waitFor(() => expect(detail.result.current.data).toEqual(detailResponse));
    expect(client.getQueryCache().getAll()).toHaveLength(2);
  });

  it("Should keep the search enabled flag inside the query-options factory", () => {
    const { wrapper } = setup();

    const { result } = renderHook(
      () => useMarketplaceSearch({ q: "audit", workspaceId: "ws-a" }, false),
      { wrapper }
    );

    expect(result.current.fetchStatus).toBe("idle");
    expect(mocks.search).not.toHaveBeenCalled();
  });

  it("Should suspend kind browsing when its retained window is inactive", () => {
    const { wrapper } = setup();

    const { result } = renderHook(
      () => useMarketplaceKind({ kind: "skill", limit: 20, workspaceId: "ws-a" }, false),
      { wrapper }
    );

    expect(result.current.fetchStatus).toBe("idle");
    expect(mocks.browse).not.toHaveBeenCalled();
  });
});
