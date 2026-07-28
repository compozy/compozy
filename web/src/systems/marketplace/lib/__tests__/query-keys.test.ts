import { describe, expect, it, vi } from "vitest";

import {
  marketplaceEntryOptions,
  marketplaceKindOptions,
  marketplaceSearchOptions,
} from "../query-options";
import { marketplaceKeys } from "../query-keys";

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

describe("marketplaceKeys", () => {
  it("Should isolate global and workspace discovery caches", () => {
    const global = marketplaceKeys.search({ limit: 20, q: "audit", workspaceId: null });
    const workspaceA = marketplaceKeys.search({ limit: 20, q: "audit", workspaceId: "ws-a" });
    const workspaceB = marketplaceKeys.search({ limit: 20, q: "audit", workspaceId: "ws-b" });

    expect(global).not.toEqual(workspaceA);
    expect(workspaceA).not.toEqual(workspaceB);
    expect(workspaceA).toContain("ws-a");
  });

  it("Should normalize text identity across kind and detail keys", () => {
    expect(
      marketplaceKeys.kind({ kind: "extension", limit: 20, q: " audit ", workspaceId: " ws-a " })
    ).toEqual(
      marketplaceKeys.kind({
        kind: "extension",
        limit: 20,
        q: "audit",
        workspaceId: "ws-a",
      })
    );
    expect(
      marketplaceKeys.detail({
        entryId: " item ",
        installedName: " local-item ",
        kind: "extension",
        workspaceId: " ws-a ",
      })
    ).toEqual(
      marketplaceKeys.detail({
        entryId: "item",
        installedName: "local-item",
        kind: "extension",
        workspaceId: "ws-a",
      })
    );
  });
});

describe("marketplace query options", () => {
  it("Should execute grouped discovery with the normalized cache identity and abort signal", async () => {
    const request = { limit: 3, q: " audit ", workspaceId: " ws-a " };
    const signal = new AbortController().signal;
    mocks.search.mockResolvedValue({ kinds: [], query: "audit" });

    const options = marketplaceSearchOptions(request);
    await options.queryFn?.({ pageParam: "cursor-a", signal } as never);

    expect(options.queryKey).toEqual(marketplaceKeys.search(request));
    expect(options.staleTime).toBe(60_000);
    expect(mocks.search).toHaveBeenCalledWith(request, signal);
  });

  it("Should execute kind browse with the scoped kind cache identity", async () => {
    const request = { kind: "extension" as const, limit: 20, q: "audit", workspaceId: "ws-a" };
    const signal = new AbortController().signal;
    mocks.browse.mockResolvedValue({ items: [], kind: "extension", total: 0 });

    const options = marketplaceKindOptions(request);
    await options.queryFn?.({ pageParam: "cursor-a", signal } as never);

    expect(options.queryKey).toEqual(marketplaceKeys.kind(request));
    expect(mocks.browse).toHaveBeenCalledWith({ ...request, cursor: "cursor-a" }, signal);
    expect(options.getNextPageParam?.({ next_cursor: "cursor-b" } as never, [], "", [])).toBe(
      "cursor-b"
    );
  });

  it("Should disable blank detail identities and execute stable entry ids", async () => {
    const blank = marketplaceEntryOptions({ entryId: "  ", kind: "skill", workspaceId: null });
    expect(blank.enabled).toBe(false);

    const request = { entryId: "audit-log", kind: "extension" as const, workspaceId: "ws-a" };
    const signal = new AbortController().signal;
    mocks.detail.mockResolvedValue({ entry: { entry_id: "audit-log" } });

    const options = marketplaceEntryOptions(request);
    await options.queryFn?.({ signal } as never);

    expect(options.enabled).toBe(true);
    expect(options.queryKey).toEqual(marketplaceKeys.detail(request));
    expect(mocks.detail).toHaveBeenCalledWith(request, signal);
  });
});
