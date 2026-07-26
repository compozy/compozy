// @vitest-environment jsdom

import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  useNetworkThreads: vi.fn(),
  useNetworkDirects: vi.fn(),
  useActiveNetworkSession: vi.fn(),
}));

const emptyCatalog = {
  total: 0,
  hasMore: false,
  isLoading: false,
  isLoadingMore: false,
  loadMore: vi.fn().mockResolvedValue(undefined),
  error: null,
};

vi.mock("../use-threads", () => ({
  useNetworkThreads: mocks.useNetworkThreads,
}));
vi.mock("../use-directs", () => ({
  useNetworkDirects: mocks.useNetworkDirects,
}));
vi.mock("../use-active-session", () => ({
  useActiveNetworkSession: mocks.useActiveNetworkSession,
}));
vi.mock("../use-last-read", () => ({
  useLastRead: () => ({
    lastReadAt: vi.fn(() => null),
    markRead: vi.fn(),
  }),
}));

import { createNetworkChipFilter, useNetworkListFilters } from "../use-network-list-filters";

describe("useNetworkListFilters", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.useNetworkThreads.mockReturnValue({ ...emptyCatalog, threads: [] });
    mocks.useNetworkDirects.mockReturnValue({ ...emptyCatalog, directs: [] });
    mocks.useActiveNetworkSession.mockReturnValue({
      session: { peerId: "peer-url", sessionId: "session-url" },
      disabledReason: null,
      isLoading: false,
    });
  });

  it("Should push URL workspace, search, work, self-session, and sort to both server catalogs", async () => {
    const { result } = renderHook(() =>
      useNetworkListFilters({ workspaceId: "ws-url", channel: "ops" })
    );

    act(() => {
      result.current.setSearchQuery(" release ");
      result.current.setSort("alphabetical");
      result.current.setFilters([
        createNetworkChipFilter("has_work"),
        createNetworkChipFilter("includes_me"),
      ]);
    });

    await waitFor(() => {
      expect(mocks.useNetworkThreads).toHaveBeenLastCalledWith("ops", {
        workspaceId: "ws-url",
        enabled: true,
        query: {
          query: "release",
          has_work: true,
          session_id: "session-url",
          sort: "alphabetical",
        },
      });
    });
    expect(mocks.useNetworkDirects).toHaveBeenLastCalledWith("ops", {
      workspaceId: "ws-url",
      enabled: true,
      query: {
        query: "release",
        has_work: true,
        session_id: "session-url",
        sort: "alphabetical",
      },
    });
    expect(mocks.useActiveNetworkSession).toHaveBeenCalledWith("ops", {
      workspaceId: "ws-url",
    });
  });
});
