// Suite: bounded per-workspace session groups
// Invariant: exact counts come from facets and collapsed groups never walk history.
// Owning layer: grouped session query hook. No prior suite owned this projection.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../adapters/session-api", () => ({ fetchSessions: vi.fn() }));
vi.mock("../../adapters/session-catalog-api", () => ({
  fetchSessionCatalogPage: vi.fn(),
  fetchSessionFacets: vi.fn(),
}));

import { fetchSessions } from "../../adapters/session-api";
import { fetchSessionCatalogPage, fetchSessionFacets } from "../../adapters/session-catalog-api";
import { sessionCatalogOptions, sessionFacetsOptions } from "../../lib/session-catalog-options";
import { useWorkspaceSessionGroups } from "../use-workspace-session-groups";

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return createElement(QueryClientProvider, { client }, children);
}

describe("useWorkspaceSessionGroups", () => {
  beforeEach(() => vi.clearAllMocks());

  it("Should read exact group counts without walking the session cursor chain", async () => {
    vi.mocked(fetchSessionFacets).mockResolvedValue({
      facets: { terminal_approvals: 0, all: 201, needs_you: 0, working: 0, finished: 0, idle: 201 },
      by_workspace: [
        {
          workspace_id: "ws-alpha",
          facets: {
            terminal_approvals: 0,
            all: 201,
            needs_you: 0,
            working: 0,
            finished: 0,
            idle: 201,
          },
        },
      ],
    });
    const { result } = renderHook(
      () =>
        useWorkspaceSessionGroups({
          workspaces: [{ id: "ws-alpha", name: "Alpha" }],
          sort: "attention",
          archived: false,
          enabled: true,
        }),
      { wrapper }
    );
    await waitFor(() => expect(result.current[0]?.total).toBe(201));
    expect(result.current[0]?.catalogFilters).toMatchObject({
      workspace_id: "ws-alpha",
      limit: 100,
      profile: "default",
      subagents: "exclude",
    });
    expect(fetchSessionCatalogPage).not.toHaveBeenCalled();
    expect(fetchSessions).not.toHaveBeenCalled();
  });

  it("Should scope archived group counts before requesting pages", async () => {
    vi.mocked(fetchSessionFacets).mockResolvedValue({
      facets: { terminal_approvals: 0, all: 0, needs_you: 0, working: 0, finished: 0, idle: 0 },
      by_workspace: [],
    });
    const { result } = renderHook(
      () =>
        useWorkspaceSessionGroups({
          workspaces: [{ id: "ws-alpha", name: "Alpha" }],
          sort: "attention",
          archived: true,
          enabled: true,
        }),
      { wrapper }
    );
    await waitFor(() => expect(result.current[0]?.total).toBe(0));
    expect(fetchSessionFacets).toHaveBeenCalledWith(
      expect.objectContaining({ archive: "only", profile: "default", all_workspaces: true }),
      expect.any(AbortSignal)
    );
  });

  // UT-W18: the sidebar page excludes subagent sessions unless a search is active.
  it("Should exclude subagent sessions from group pages until a search widens them", () => {
    vi.mocked(fetchSessionFacets).mockResolvedValue({
      facets: { terminal_approvals: 0, all: 0, needs_you: 0, working: 0, finished: 0, idle: 0 },
      by_workspace: [],
    });
    const input = {
      workspaces: [{ id: "ws-alpha", name: "Alpha" }],
      sort: "attention" as const,
      archived: false,
      enabled: true,
    };
    const { result, rerender } = renderHook(
      ({ search }: { search?: string }) => useWorkspaceSessionGroups({ ...input, search }),
      { wrapper, initialProps: {} }
    );
    expect(result.current[0]?.catalogFilters?.subagents).toBe("exclude");
    rerender({ search: "  " });
    expect(result.current[0]?.catalogFilters?.subagents).toBe("exclude");
    rerender({ search: "review" });
    expect(result.current[0]?.catalogFilters?.subagents).toBe("include");
    // Counts use the same visibility as the rows they head.
    expect(fetchSessionFacets).toHaveBeenCalledWith(
      expect.objectContaining({ subagents: "exclude", all_workspaces: true }),
      expect.any(AbortSignal)
    );
  });

  // UT-W18: visibility survives normalization into the wire request and the cache key, for both
  // the page and its facet counts, so `exclude` and `include` never share a cache entry.
  it("Should carry subagent visibility into page and facet requests and keys", async () => {
    const page = (subagents: "include" | "exclude") =>
      sessionCatalogOptions({ workspace_id: "ws-alpha", subagents });
    const facets = (subagents: "include" | "exclude") =>
      sessionFacetsOptions({ workspace_id: "ws-alpha", subagents });
    expect(page("exclude").queryKey[4]).toMatchObject({ subagents: "exclude" });
    expect(page("exclude").queryKey).not.toEqual(page("include").queryKey);
    expect(facets("exclude").queryKey).not.toEqual(facets("include").queryKey);

    vi.mocked(fetchSessionCatalogPage).mockResolvedValue({
      sessions: [],
      page: { has_more: false, limit: 100, next_cursor: null },
    } as never);
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    await client.fetchInfiniteQuery(page("exclude"));
    expect(fetchSessionCatalogPage).toHaveBeenCalledWith(
      expect.objectContaining({ subagents: "exclude" }),
      expect.any(AbortSignal)
    );
  });
});
