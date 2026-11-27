import { useInfiniteQuery, useQuery } from "@tanstack/react-query";

import { isRemoteComputerId, LOCAL_COMPUTER_ID, type ComputerId } from "@/lib/computer-scope";
import type { ScopedComputerFailureItem } from "@/systems/computers";
import { useProfileReadScope } from "@/systems/profiles";
import {
  useActiveWorkspace,
  workspaceBadgeIdentity,
  workspaceSelectorOrAll,
  type DataScope,
} from "@/systems/workspace";

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
import type { SessionRowComputerInfo } from "../lib/session-list-computer";
import type { SessionListGroupSpec } from "../lib/session-list-groups";
import {
  availableSessionListGroupBys,
  effectiveSessionListGroupBy,
  sessionBoardSort,
  type SessionListGroupBy,
  type SessionListSort,
} from "../lib/session-list-preferences";
import { sessionListShelfFilters } from "../lib/session-list-shelf";
import type { SessionListFilters } from "../types";
import { useSessionGroupFolds } from "./use-session-group-folds";
import { useSessionListGroups } from "./use-session-list-groups";
import {
  useSessionsBoardComputers,
  type SessionsBoardComputerBand,
} from "./use-sessions-board-computers";
import { useSessionWindowOrder } from "./use-session-window-order";
import { useProjectFolderGroups } from "./use-workspace-layout";

export type { SessionsBoardComputerBand } from "./use-sessions-board-computers";

/**
 * Which read of a page failed. `initial`: nothing loaded, so the error takes
 * the page's place. `more` (the next page) and `refresh` (a background
 * re-read): the loaded cards stay and the failure gets a local retry.
 */
export type SessionsBoardFailure = "initial" | "more" | "refresh";

/** One paged catalog read: a board column, the Archive bar or the Later bar. */
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

/**
 * A bottom bar's read (Archive, Later). `count` is the page total, `undefined`
 * until a read succeeds, so a failed or pending read is never mistaken for zero.
 */
export type SessionsBoardShelf = SessionsBoardPage & { count: number | undefined };

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
  /** The same fold as this group in the window's List. */
  collapsed: boolean;
  onToggle: () => void;
}

export interface SessionsBoardView {
  /** The ungrouped board's columns; empty and unread while the board is grouped or banded. */
  columns: SessionsBoardColumnView[];
  /** Group-by swimlanes from server facets; `null` while ungrouped or banded. */
  lanes: SessionsBoardLane[] | null;
  /** Computer bands under All computers + Separate (`F13`, `K2`, `K3`); `null` elsewhere. */
  computerBands?: SessionsBoardComputerBand[] | null;
  lanesLoading: boolean;
  lanesError?: { retry: () => void };
  /** The saved project order failed to load: lanes stay, in the served order, until a retry. */
  layoutError?: { retry: () => void };
  /** The grouping that renders: an inapplicable stored value renders ungrouped. */
  groupBy: SessionListGroupBy;
  /** Groupings available in the current breadth and profile lens. */
  groupByOptions: readonly SessionListGroupBy[];
  /** The archived sessions, outside the columns. */
  archive: SessionsBoardShelf;
  /** The sessions parked in Later, read and counted the same way as the archive. */
  later: SessionsBoardShelf;
  /** All projects: each card's workspace name; `null` while one scope is listed. */
  workspaceNameOf: (session: SessionBoardPayload) => string | null;
  /** The board's one concrete scope for its top row; `null` under All projects. */
  focusedWorkspaceId: string | null;
  /** The board's identity for its selection: scope, layer, breadth, profile lens and grouping. */
  selectionKey: string;
  /** Remote-computer provenance for multi-computer cards (`_uiux.md` U5, `K7`). */
  computerBySessionId?: ReadonlyMap<string, SessionRowComputerInfo>;
  /** Remote computers whose board read failed (Together mode strip only, `K6`). */
  failedComputers?: readonly ScopedComputerFailureItem[];
  /** True when at least one remote computer is enabled for this window (`F9`, `F13`). */
  hasRemoteComputers?: boolean;
}

function useSessionsBoardPage(
  filters: SessionListFilters,
  enabled: boolean,
  computerId: ComputerId = LOCAL_COMPUTER_ID
): SessionsBoardPage {
  const page = useInfiniteQuery({ ...sessionBoardPageOptions(filters, computerId), enabled });
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
    retry: () => {
      if (page.isFetching) return;
      if (failure === "more") void page.fetchNextPage();
      else void page.refetch();
    },
    total: page.data?.pages[0]?.page.total ?? undefined,
  };
}

/**
 * The five derived columns for one catalog query on one daemon: each column is its own
 * read filtered by `board_column` before pagination, and counts come from `facets.board`.
 */
export function useSessionsBoardColumns(
  base: SessionListFilters,
  active: boolean,
  computerId: ComputerId = LOCAL_COMPUTER_ID
): SessionsBoardColumnView[] {
  const column = (id: SessionBoardColumnId): SessionListFilters => ({
    ...base,
    board_column: [id],
    skip_total: true,
  });
  const needsYou = useSessionsBoardPage(column("needs_you"), active, computerId);
  const working = useSessionsBoardPage(column("working"), active, computerId);
  const waiting = useSessionsBoardPage(column("waiting"), active, computerId);
  const inReview = useSessionsBoardPage(column("in_review"), active, computerId);
  const done = useSessionsBoardPage(column("done"), active, computerId);
  const facets = useQuery({
    ...sessionFacetsOptions(base, computerId),
    refetchInterval: SESSION_BOARD_REFETCH_MS,
    enabled: active,
  });
  const counts = facets.data?.facets.board;
  const pages = { needs_you: needsYou, working, waiting, in_review: inReview, done };
  return SESSION_BOARD_COLUMNS.map(({ id, label }) => ({
    ...pages[id],
    id,
    label,
    count: counts?.[id],
  }));
}

/**
 * The Sessions board read model for a data scope (`_uiux.md` S4, `F13`).
 */
export function useSessionsBoard({
  scope,
  hideChildren,
  sort,
  groupBy,
  showEmptyProjects = false,
  showAutomation = false,
  enabled = true,
}: {
  scope: DataScope | null;
  hideChildren: boolean;
  /** "Show automation runs" (#1355), shared with the window's List. */
  showAutomation?: boolean;
  sort: SessionListSort;
  groupBy: SessionListGroupBy;
  /** Lanes for projects with no sessions too, as the window's List shows them. */
  showEmptyProjects?: boolean;
  enabled?: boolean;
}): SessionsBoardView {
  const { params, aggregate } = useProfileReadScope();
  const { registeredWorkspaces } = useActiveWorkspace();
  const order = useSessionWindowOrder();
  const active = enabled && scope !== null;
  const breadth = { allProjects: scope?.kind === "all", allProfiles: aggregate };
  const effectiveGroupBy = effectiveSessionListGroupBy(groupBy, breadth);
  const grouped = effectiveGroupBy !== "none";
  const population = hideChildren ? { children: "exclude" as const } : {};
  const shared: SessionListFilters = {
    ...population,
    ...sessionListShelfFilters("active", showAutomation),
    sort: sessionBoardSort(sort),
  };

  const multi = useSessionsBoardComputers({
    scope,
    shared,
    localLens: params,
    groupBy: effectiveGroupBy,
    showAutomation,
    enabled: active,
  });

  const targetComputerId: ComputerId = multi.scopedRemoteSource?.key ?? LOCAL_COMPUTER_ID;
  const isRemoteScoped = isRemoteComputerId(targetComputerId);
  const targetLens = isRemoteScoped ? { all_profiles: true } : params;
  const targetWorkspaces = multi.scopedRemoteSource
    ? multi.scopedRemoteSource.workspaces
    : registeredWorkspaces;
  const matchedWorkspaceIds = multi.scopedRemoteSource?.matchedWorkspaceIds ?? null;
  const base = sessionCatalogScopedFilters(scope, shared, targetLens);

  const banded = multi.computerBands !== null;
  const togetherGrouped = multi.togetherLanes !== null;
  const singleColumnsActive = active && !grouped && !banded;
  const columns = useSessionsBoardColumns(base, singleColumnsActive, targetComputerId);

  const singleGroupingActive = active && grouped && !banded && !togetherGrouped;
  const grouping = useSessionListGroups({
    groupBy: effectiveGroupBy,
    scope,
    lens: targetLens,
    workspaces: targetWorkspaces,
    shelf: "active",
    showAutomation,
    enabled: singleGroupingActive,
    population,
    refetchInterval: SESSION_BOARD_REFETCH_MS,
    computerId: targetComputerId,
    matchedWorkspaceIds,
  });
  const arranged = useProjectFolderGroups({
    groups: grouping.groups,
    groupBy: effectiveGroupBy,
    breadth: {
      allProjects: breadth.allProjects,
      allProfiles: isRemoteScoped ? true : breadth.allProfiles,
    },
    lens: targetLens,
    workspaces: targetWorkspaces,
    showEmptyProjects,
    enabled: singleGroupingActive,
    computerId: targetComputerId,
  });
  const folds = useSessionGroupFolds(
    scope,
    effectiveGroupBy,
    isRemoteScoped ? targetComputerId : undefined
  );
  const localShelfActive = active && !multi.multiArchive;
  const archive = useSessionsBoardPage(
    { ...base, ...sessionListShelfFilters("archive") },
    localShelfActive,
    targetComputerId
  );
  const later = useSessionsBoardPage(
    { ...base, ...sessionListShelfFilters("later") },
    localShelfActive,
    targetComputerId
  );
  const names = new Map(targetWorkspaces.map(workspace => [workspace.id, workspace.name]));
  const allProjects = scope?.kind === "all";

  const lanes: SessionsBoardLane[] | null = banded
    ? null
    : togetherGrouped
      ? multi.togetherLanes
      : grouped
        ? arranged.groups.map(group => ({
            group,
            filters: sessionCatalogScopedFilters(group.scope, shared, group.lens),
            collapsed: folds.isCollapsed(group.key),
            onToggle: () => folds.toggle(group.key),
          }))
        : null;

  return {
    focusedWorkspaceId: scope ? workspaceSelectorOrAll(scope) : null,
    columns: grouped || banded ? [] : columns,
    lanes,
    computerBands: multi.computerBands,
    lanesLoading: singleGroupingActive ? grouping.loading : false,
    lanesError: singleGroupingActive ? grouping.error : undefined,
    layoutError: singleGroupingActive ? arranged.loadError : undefined,
    groupBy: effectiveGroupBy,
    groupByOptions: availableSessionListGroupBys(breadth),
    archive: multi.multiArchive ?? { ...archive, count: archive.total },
    later: multi.multiLater ?? { ...later, count: later.total },
    workspaceNameOf: multi.hasRemoteComputers
      ? multi.workspaceNameOf
      : session => (allProjects ? workspaceBadgeIdentity(session.workspace_id, names).name : null),
    selectionKey: JSON.stringify([
      multi.effectiveScope,
      order.computers,
      scope,
      params,
      effectiveGroupBy,
    ]),
    computerBySessionId: multi.computerBySessionId,
    failedComputers: multi.failedComputers,
    hasRemoteComputers: multi.hasRemoteComputers,
  };
}
