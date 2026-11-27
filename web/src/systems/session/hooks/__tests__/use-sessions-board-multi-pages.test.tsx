// Suite: Multi-computer Kanban columns, shelves, and project-scoped remote filters (Opus 4, Opus 5, Astra 9, Astra 10)
// Invariant: Project-scoped remote boards pass matched `workspace_ids` and skip reads on 0 matches;
//            Together + None merges per-computer column pages sorted by window sort, sums `facets.board`,
//            and records computer provenance keyed by qualified `sessionRowKey` only; multi-computer
//            shelves page forward via cursor `loadMore`.
// Owning layer: hooks (systems/session/hooks)
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../adapters/session-catalog-api", () => ({
  fetchSessionCatalogPage: vi.fn(),
  fetchSessionFacets: vi.fn(),
}));

import { LOCAL_COMPUTER_ID } from "@/lib/computer-scope";
import {
  LOCAL_COMPUTER,
  makeComputerFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures";

import type { DataScope } from "@/systems/workspace";
import { fetchSessionCatalogPage } from "../../adapters/session-catalog-api";
import {
  sessionCatalogPageQueryOptions,
  SESSION_BOARD_PAGE_SIZE,
} from "../../lib/session-catalog-options";
import { sessionListShelfFilters } from "../../lib/session-list-shelf";
import { sessionRowKey } from "../../lib/session-row-ref";
import { primarySessionFixture } from "../../mocks/fixtures";
import type { ComputerSessionsSource } from "../use-computer-sessions-sources";
import {
  boardFiltersForSource,
  useMultiComputerBoardShelf,
} from "../use-sessions-board-multi-pages";
import { useTogetherBoardColumns } from "../use-together-board-columns";

function wrap(client: QueryClient) {
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client }, children);
}

function makeSource(
  overrides: Partial<ComputerSessionsSource> & { key: string; name: string }
): ComputerSessionsSource {
  const isLocal = overrides.key === LOCAL_COMPUTER_ID;
  return {
    isLocal,
    computer: isLocal
      ? LOCAL_COMPUTER
      : makeComputerFixture({ id: overrides.key, name: overrides.name, status: "connected" }),
    read: "ok",
    detail: isLocal ? "This computer" : "via LAN",
    canOperate: true,
    canCreate: true,
    facets: undefined,
    count: 2,
    needsYou: 0,
    working: 0,
    laterCount: 0,
    workspaces: [],
    workspacesLoading: false,
    workspacesError: false,
    matchedWorkspaceIds: null,
    retry: vi.fn(),
    ...overrides,
  };
}

describe("Multi-computer Kanban columns, shelves, and project-scoped filters", () => {
  beforeEach(() => vi.clearAllMocks());

  it("Should pass matched workspace_ids on remote project match and skip reads when no remote workspace matches (Opus 5 / Astra 10)", () => {
    const matchedRemote = makeSource({
      key: "cmp_devbox",
      name: "devbox",
      matchedWorkspaceIds: ["ws_remote_a", "ws_remote_b"],
    });
    const matchedSpec = boardFiltersForSource(
      matchedRemote,
      { kind: "project", workspaceId: "ws_local_1" },
      { sort: "last_activity" },
      { profile: "default" }
    );
    expect(matchedSpec.emptyRemoteMatch).toBe(false);
    expect(matchedSpec.filters.workspace_ids).toEqual(["ws_remote_a", "ws_remote_b"]);
    expect(matchedSpec.filters.all_workspaces).toBe(true);

    const unmatchedRemote = makeSource({
      key: "cmp_studio",
      name: "studio",
      matchedWorkspaceIds: [],
    });
    const unmatchedSpec = boardFiltersForSource(
      unmatchedRemote,
      { kind: "project", workspaceId: "ws_local_1" },
      { sort: "last_activity" },
      { profile: "default" }
    );
    expect(unmatchedSpec.emptyRemoteMatch).toBe(true);

    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const { result } = renderHook(
      () =>
        useMultiComputerBoardShelf(
          [unmatchedRemote],
          { kind: "project", workspaceId: "ws_local_1" },
          { sort: "last_activity" },
          { profile: "default" },
          "archive",
          true
        ),
      { wrapper: wrap(client) }
    );
    expect(result.current?.sessions).toEqual([]);
    expect(fetchSessionCatalogPage).not.toHaveBeenCalled();
  });

  it("Should exclude cached archive rows and totals when the remote repository does not match", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const remote = makeSource({ key: "cmp_studio", name: "studio", matchedWorkspaceIds: [] });
    const row = { ...primarySessionFixture, id: "remote-archived" };
    const all: DataScope = { kind: "all" };
    const lens = { all_profiles: true } as const;
    const filters = {
      ...boardFiltersForSource(remote, all, {}, lens).filters,
      ...sessionListShelfFilters("archive"),
      limit: SESSION_BOARD_PAGE_SIZE,
      skip_total: false,
    };
    const cached = {
      sessions: [row],
      page: { total: 9, has_more: true, next_cursor: "other", limit: 50 },
    };
    client.setQueryData(
      sessionCatalogPageQueryOptions(filters, undefined, remote.key).queryKey,
      cached
    );
    vi.mocked(fetchSessionCatalogPage).mockResolvedValue(cached);
    const { result, rerender } = renderHook<
      ReturnType<typeof useMultiComputerBoardShelf>,
      { scope: DataScope }
    >(({ scope }) => useMultiComputerBoardShelf([remote], scope, {}, lens, "archive", true), {
      initialProps: { scope: all },
      wrapper: wrap(client),
    });

    await waitFor(() =>
      expect(result.current?.sessions.map(session => session.id)).toEqual([row.id])
    );
    expect(result.current?.total).toBe(9);
    rerender({ scope: { kind: "project", workspaceId: "ws-unmatched" } });
    expect(result.current?.sessions).toEqual([]);
    expect(result.current?.total).toBeUndefined();
    expect(result.current?.hasMore).toBe(false);
    expect(result.current?.failure).toBeNull();
  });

  it("Should merge Together + None columns, sum facets.board, qualify computer provenance, and page loadMore (Opus 4 / Astra 9 / Opus 11)", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    vi.mocked(fetchSessionCatalogPage).mockImplementation(async (filters, _signal, computerId) => {
      const col = filters?.board_column?.[0];
      if (col !== "working") {
        return { sessions: [], page: { has_more: false, next_cursor: "", limit: 50 } };
      }
      if (computerId === "cmp_devbox") {
        if (filters?.cursor === "cur_dev_2") {
          return {
            sessions: [
              {
                ...primarySessionFixture,
                id: "dev_page_2",
                name: "Devbox page 2",
                board_column: "working",
                badge: "running",
                updated_at: "2026-04-10T07:00:00.000Z",
              },
            ],
            page: { has_more: false, next_cursor: "", limit: 50 },
          };
        }
        return {
          sessions: [
            {
              ...primarySessionFixture,
              id: "shared_card",
              name: "Devbox working",
              board_column: "working",
              badge: "running",
              updated_at: "2026-04-10T09:00:00.000Z",
            },
          ],
          page: { has_more: true, next_cursor: "cur_dev_2", limit: 50 },
        };
      }
      return {
        sessions: [
          {
            ...primarySessionFixture,
            id: "shared_card",
            name: "Local working",
            board_column: "working",
            badge: "running",
            updated_at: "2026-04-10T11:00:00.000Z",
          },
        ],
        page: { has_more: false, next_cursor: "", limit: 50 },
      };
    });

    const local = makeSource({
      key: LOCAL_COMPUTER_ID,
      name: "MacBook Pro",
      facets: {
        facets: {
          all: 3,
          needs_you: 1,
          working: 1,
          idle: 1,
          board: { needs_you: 1, working: 1, waiting: 0, in_review: 0, done: 1 },
        } as never,
        by_workspace: [],
        by_profile: [],
        later: 0,
      },
    });
    const devbox = makeSource({
      key: "cmp_devbox",
      name: "devbox",
      facets: {
        facets: {
          all: 4,
          needs_you: 0,
          working: 2,
          idle: 2,
          board: { needs_you: 0, working: 2, waiting: 1, in_review: 0, done: 1 },
        } as never,
        by_workspace: [],
        by_profile: [],
        later: 0,
      },
    });

    const { result } = renderHook(
      () =>
        useTogetherBoardColumns({
          sources: [local, devbox],
          scope: { kind: "all" },
          shared: { sort: "last_activity" },
          localLens: { all_profiles: true },
          sort: "last_activity",
          enabled: true,
        }),
      { wrapper: wrap(client) }
    );

    await waitFor(() => {
      const working = result.current.columns?.find(col => col.id === "working");
      expect(working?.sessions).toHaveLength(2);
    });

    const working = result.current.columns!.find(col => col.id === "working")!;
    expect(working.count).toBe(3);
    expect(working.sessions.map(session => session.name)).toEqual([
      "Local working",
      "Devbox working",
    ]);
    expect(working.hasMore).toBe(true);

    // Qualified key only — bare `shared_card` is never stored so local cards never inherit remote labels.
    expect(
      result.current.computerBySessionId.get(
        sessionRowKey({ computerId: "cmp_devbox", sessionId: "shared_card" })
      )?.name
    ).toBe("devbox");
    expect(result.current.computerBySessionId.has("shared_card")).toBe(false);

    act(() => {
      working.loadMore();
    });
    await waitFor(() => {
      const updated = result.current.columns?.find(col => col.id === "working");
      expect(updated?.sessions).toHaveLength(3);
      expect(updated?.hasMore).toBe(false);
    });
  });
});
