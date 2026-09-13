// Suite: marketplace query hooks
// Invariant: marketplace reads retain kind and entry scope while honoring liveness gates.
// Boundary IN: query options, pagination state, cache keys, and hook projections.
// Boundary OUT: installation interactions and rendering, owned by component suites.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { marketplaceDetails, marketplaceKindFixture, marketplaceSearchFixture } from "../../mocks";
import { useMarketplaceExtensionMCPServer } from "../use-marketplace-detail-mcp-server";
import type { MarketplaceExtensionServer } from "../../types";
import { MarketplaceApiError } from "../../adapters/marketplace-api-error";
import {
  useMarketplaceCatalog,
  useMarketplaceEntry,
  useMarketplaceKind,
  useMarketplaceSearch,
} from "../use-marketplace";

const mocks = vi.hoisted(() => ({
  catalog: vi.fn(),
  mcpDetail: vi.fn(),
  catalogDetail: vi.fn(),
  browse: vi.fn(),
  detail: vi.fn(),
  search: vi.fn(),
}));

vi.mock("../../adapters/marketplace-api", () => ({
  browseMarketplace: mocks.catalog,
  getMarketplaceCatalogEntry: mocks.catalogDetail,
  browseMarketplaceKind: mocks.browse,
  getMarketplaceEntry: mocks.detail,
  searchMarketplace: mocks.search,
}));

vi.mock("@/systems/settings/adapters/settings-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/settings/adapters/settings-api")>()),
  getSettingsMCPServer: mocks.mcpDetail,
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

// Invariant: revision conflicts replace the entire paged cache; source scope and liveness stay independent.
// Owning layer: catalog query lifecycle; canonical suite: use-marketplace.test.tsx (UT-035).
describe("one catalog query lifecycle", () => {
  beforeEach(() => {
    mocks.catalog.mockReset();
    mocks.catalogDetail.mockReset();
  });
  it("Should discard old pages and restart from page one after a stale cursor", async () => {
    const first = {
      total: 2,
      revision: "old",
      stale: false,
      sources: [],
      items: [{ entry_id: "old-entry" }],
      next_cursor: "old-next",
    };
    const current = {
      total: 1,
      revision: "new",
      stale: false,
      sources: [],
      items: [{ entry_id: "new-entry" }],
    };
    mocks.catalog
      .mockResolvedValueOnce(first)
      .mockRejectedValueOnce(
        new MarketplaceApiError("Catalog changed", 409, "marketplace_cursor_stale", true)
      )
      .mockResolvedValueOnce(current);
    const { client, wrapper } = setup();
    const { result, unmount } = renderHook(
      () => useMarketplaceCatalog({ q: "audit", profileName: "work" }),
      { wrapper }
    );
    await waitFor(() => expect(result.current.data?.pages).toEqual([first]));
    await act(async () => {
      await result.current.fetchNextPage();
    });
    await waitFor(() => expect(result.current.data?.pages).toEqual([current]));
    expect(result.current.data?.pageParams).toEqual([undefined]);
    expect(mocks.catalog.mock.calls.map(call => call[0].cursor)).toEqual([
      undefined,
      "old-next",
      undefined,
    ]);
    expect(client.getQueryCache().getAll()).toHaveLength(1);
    unmount();
    client.clear();
  });

  it("Should retain the envelope on ordinary errors and pause inactive catalog requests", async () => {
    const first = {
      total: 2,
      revision: "same",
      stale: false,
      sources: [],
      items: [],
      next_cursor: "next",
    };
    mocks.catalog
      .mockResolvedValueOnce(first)
      .mockRejectedValueOnce(new MarketplaceApiError("Invalid request", 400));
    const { client, wrapper } = setup();
    const { result, unmount } = renderHook(() => useMarketplaceCatalog({ profileName: "work" }), {
      wrapper,
    });
    await waitFor(() => expect(result.current.data?.pages).toEqual([first]));
    await act(async () => {
      await result.current.fetchNextPage();
    });
    await waitFor(() => expect(result.current.isFetchNextPageError).toBe(true));
    expect(result.current.data?.pages).toEqual([first]);
    expect(mocks.catalog).toHaveBeenCalledTimes(2);
    const inactive = renderHook(() => useMarketplaceCatalog({ profileName: "personal" }, false), {
      wrapper,
    });
    expect(inactive.result.current.fetchStatus).toBe("idle");
    expect(mocks.catalog).toHaveBeenCalledTimes(2);
    inactive.unmount();
    unmount();
    client.clear();
  });
});

// Invariant: server actions follow the published instance selectors and never infer a manual owner.
// Owner: marketplace MCP query hook. Canonical suite: use-marketplace.test.tsx.
describe("extension MCP definition reads", () => {
  beforeEach(() => {
    mocks.mcpDetail.mockReset();
  });
  it.each([
    {
      scope: "global",
      profile: "default",
      workspace_id: undefined,
      expected: { scope: "user", owner: "extension:kit" },
    },
    {
      scope: "global",
      profile: "work",
      workspace_id: undefined,
      expected: { scope: "profile", profile: "work", owner: "extension:kit" },
    },
    {
      scope: "workspace",
      profile: "default",
      workspace_id: "ws-a",
      expected: { scope: "workspace", workspace_id: "ws-a", owner: "extension:kit" },
    },
    {
      scope: "workspace",
      profile: "work",
      workspace_id: "ws-a",
      expected: { scope: "profile", profile: "work", workspace_id: "ws-a", owner: "extension:kit" },
    },
  ])("Should read the exact $scope/$profile definition", async ({ expected, ...scope }) => {
    const published: MarketplaceExtensionServer = {
      name: "shared",
      owner: "extension:kit",
      runtime_name: "kit.shared",
      transport: "http",
      launch: "https://mcp.example.test",
      ...scope,
    };
    const response = { server: { name: "shared", owner: "extension:kit" } };
    mocks.mcpDetail.mockResolvedValue(response);
    const { client, wrapper } = setup();
    const { result, unmount } = renderHook(() => useMarketplaceExtensionMCPServer(published), {
      wrapper,
    });
    await waitFor(() => expect(result.current.query.isSuccess).toBe(true));
    expect(mocks.mcpDetail).toHaveBeenCalledWith("shared", expected, expect.any(AbortSignal));
    expect(result.current.server).toEqual(response.server);
    expect(result.current.filter).toEqual(expected);
    unmount();
    client.clear();
  });
  it("Should suspend pre-install, ownerless, and inactive definition reads", () => {
    const published: MarketplaceExtensionServer = {
      name: "shared",
      owner: "extension:kit",
      runtime_name: "kit.shared",
      scope: "global",
      transport: "http",
      launch: "https://mcp.example.test",
    };
    const { client, wrapper } = setup();
    for (const [server, enabled] of [
      [{ ...published, runtime_name: undefined }, true],
      [{ ...published, owner: "manual" }, true],
      [{ ...published, scope: "workspace", workspace_id: undefined }, true],
      [published, false],
    ] as const) {
      const { result, unmount } = renderHook(
        () => useMarketplaceExtensionMCPServer(server, enabled),
        { wrapper }
      );
      expect(result.current.query.fetchStatus).toBe("idle");
      unmount();
    }
    expect(mocks.mcpDetail).not.toHaveBeenCalled();
    client.clear();
  });
});
