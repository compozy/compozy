import { useInfiniteQuery, useQuery } from "@tanstack/react-query";

import { useProfileReadScope } from "@/systems/profiles";
import { useActiveWorkspace, workspaceBadgeIdentity, type DataScope } from "@/systems/workspace";

import {
  SESSION_BOARD_COLUMNS,
  type SessionBoardColumnId,
  type SessionBoardPayload,
} from "../lib/session-board-contract";
import {
  SESSION_BOARD_REFETCH_MS,
  sessionBoardPageOptions,
  sessionFacetsOptions,
} from "../lib/session-catalog-options";
import { sessionCatalogScopedFilters } from "../lib/session-catalog-scope";
import type { SessionListGroupSpec } from "../lib/session-list-groups";
import {
  availableSessionListGroupBys,
  effectiveSessionListGroupBy,
  sessionBoardSort,
  type SessionListGroupBy,
  type SessionListSort,
} from "../lib/session-list-preferences";
import type { SessionListFilters } from "../types";
import { useSessionListGroups } from "./use-session-list-groups";

/**
 * Which read of a page failed. `initial`: nothing loaded, so the error takes
 * the page's place. `more` (the next page) and `refresh` (a background
 * re-read): the loaded cards stay and the failure gets a local retry.
 */
export type SessionsBoardFailure = "initial" | "more" | "refresh";

/** One paged catalog read: a board column or the Archive bar. */
export interface SessionsBoardPage {
  sessions: SessionBoardPayload[];
  loading: boolean;
  failure: SessionsBoardFailure | null;
  /** A retry of the failed read is in flight. */
  retrying: boolean;
  /** Any read of the page is in flight, a background re-read included. */
  fetching: boolean;
  hasMore: boolean;
  loadingMore: boolean;
  loadMore: () => void;
  retry: () => void;
  /** The page's population total, when the read asked for one. */
  total: number | undefined;
}

export interface SessionsBoardColumnView extends SessionsBoardPage {
  id: SessionBoardColumnId;
  label: string;
  /** From `facets.board`; `undefined` until the counts load. */
  count: number | undefined;
}

/** One Group-by swimlane: its group and the catalog filters every column of it reads. */
export interface SessionsBoardLane {
  group: SessionListGroupSpec;
  filters: SessionListFilters;
}

export interface SessionsBoardView {
  /** The ungrouped board's columns; empty and unread while the board is grouped. */
  columns: SessionsBoardColumnView[];
  /** Group-by swimlanes from server facets; `null` while the board is ungrouped. */
  lanes: SessionsBoardLane[] | null;
  lanesLoading: boolean;
  lanesError?: { retry: () => void };
  /** The grouping that renders: an inapplicable stored value renders ungrouped. */
  groupBy: SessionListGroupBy;
  /** Groupings available in the current breadth and profile lens. */
  groupByOptions: readonly SessionListGroupBy[];
  /**
   * `count` is the archive's total, `undefined` until a read succeeds, so a
   * failed or pending read is never mistaken for zero archived sessions.
   */
  archive: SessionsBoardPage & { count: number | undefined };
  /** All projects: each card's workspace name; `null` while one scope is listed. */
  workspaceNameOf: (session: SessionBoardPayload) => string | null;
  /** The board's identity for its selection: breadth, profile lens and grouping. */
  selectionKey: string;
}

function useSessionsBoardPage(filters: SessionListFilters, enabled: boolean): SessionsBoardPage {
  const page = useInfiniteQuery({ ...sessionBoardPageOptions(filters), enabled });
  const failure: SessionsBoardFailure | null = !page.isError
    ? null
    : page.data === undefined
      ? "initial"
      : page.isFetchNextPageError
        ? "more"
        : "refresh";
  return {
    sessions: (page.data?.pages ?? []).flatMap(item => item.sessions),
    loading: page.isLoading,
    failure,
    retrying: failure !== null && page.isFetching,
    fetching: page.isFetching,
    hasMore: page.hasNextPage,
    loadingMore: page.isFetchingNextPage,
    loadMore: () => {
      if (!page.isFetching) void page.fetchNextPage({ cancelRefetch: false });
    },
    // A failed next page is fetched again; any other failure re-reads the loaded pages.
    retry: () => {
      if (page.isFetching) return;
      if (failure === "more") void page.fetchNextPage();
      else void page.refetch();
    },
    total: page.data?.pages[0]?.page.total ?? undefined,
  };
}

/**
 * The four derived columns for one catalog query: each column is its own read
 * filtered by the daemon's `board_column` before pagination, and the counts come
 * from `facets.board`, so they include cards that are not loaded yet. The
 * ungrouped board reads one set; each swimlane reads its own.
 */
export function useSessionsBoardColumns(
  base: SessionListFilters,
  active: boolean
): SessionsBoardColumnView[] {
  const column = (id: SessionBoardColumnId): SessionListFilters => ({
    ...base,
    board_column: [id],
    skip_total: true,
  });
  const needsYou = useSessionsBoardPage(column("needs_you"), active);
  const working = useSessionsBoardPage(column("working"), active);
  const inReview = useSessionsBoardPage(column("in_review"), active);
  const done = useSessionsBoardPage(column("done"), active);
  const facets = useQuery({
    ...sessionFacetsOptions(base),
    refetchInterval: SESSION_BOARD_REFETCH_MS,
    enabled: active,
  });
  const counts = facets.data?.facets.board;
  const pages = { needs_you: needsYou, working, in_review: inReview, done };
  return SESSION_BOARD_COLUMNS.map(({ id, label }) => ({
    ...pages[id],
    id,
    label,
    count: counts?.[id],
  }));
}

/**
 * The Sessions board read model for a data scope (`_uiux.md` S4). Columns read
 * in the window's sort (a List-only `manual` reads `last_activity`). Grouped by
 * project or profile, the board becomes swimlanes: membership comes from the
 * same facets the List groups use, and each lane reads its own columns. Archived
 * sessions appear only in the Archive bar, which stays ungrouped. All reads live
 * under the catalog list family, so stream wakes and the 60 s visible re-read
 * keep them current.
 */
export function useSessionsBoard({
  scope,
  hideChildren,
  sort,
  groupBy,
  enabled = true,
}: {
  scope: DataScope | null;
  hideChildren: boolean;
  sort: SessionListSort;
  groupBy: SessionListGroupBy;
  enabled?: boolean;
}): SessionsBoardView {
  const { params, aggregate } = useProfileReadScope();
  const { registeredWorkspaces } = useActiveWorkspace();
  const active = enabled && scope !== null;
  const breadth = { allProjects: scope?.kind === "all", allProfiles: aggregate };
  const effectiveGroupBy = effectiveSessionListGroupBy(groupBy, breadth);
  const grouped = effectiveGroupBy !== "none";
  const population = hideChildren ? { children: "exclude" as const } : {};
  const shared: SessionListFilters = { ...population, sort: sessionBoardSort(sort) };
  const base = sessionCatalogScopedFilters(scope, shared, params);
  const columns = useSessionsBoardColumns(base, active && !grouped);
  const grouping = useSessionListGroups({
    groupBy: effectiveGroupBy,
    scope,
    lens: params,
    workspaces: registeredWorkspaces,
    archived: false,
    enabled: active,
    // Lane membership and totals count the rows the lane's columns read.
    population,
  });
  const archive = useSessionsBoardPage({ ...base, archive: "only" }, active);
  const names = new Map(registeredWorkspaces.map(workspace => [workspace.id, workspace.name]));
  const allProjects = scope?.kind === "all";

  return {
    columns: grouped ? [] : columns,
    lanes: grouped
      ? grouping.groups.map(group => ({
          group,
          filters: sessionCatalogScopedFilters(group.scope, shared, group.lens),
        }))
      : null,
    lanesLoading: grouping.loading,
    lanesError: grouping.error,
    groupBy: effectiveGroupBy,
    groupByOptions: availableSessionListGroupBys(breadth),
    archive: { ...archive, count: archive.total },
    workspaceNameOf: session =>
      allProjects ? workspaceBadgeIdentity(session.workspace_id, names).name : null,
    selectionKey: JSON.stringify([scope, params, effectiveGroupBy]),
  };
}
