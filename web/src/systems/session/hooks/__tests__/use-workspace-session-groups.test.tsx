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
});
