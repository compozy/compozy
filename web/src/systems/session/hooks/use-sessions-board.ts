import { useContext } from "react";
import { QueryClientContext, useInfiniteQuery, useQuery } from "@tanstack/react-query";

import { isRemoteComputerId, LOCAL_COMPUTER_ID, type ComputerId } from "@/lib/computer-scope";
import type { ScopedComputerFailureItem } from "@/systems/computers";
import { useProfileReadScope, type ProfileScopeParams } from "@/systems/profiles";
import {
  useActiveWorkspace,
  workspaceBadgeIdentity,
  type DataScope,
  type WorkspacePayload,
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
import { readCachedOfflineRowsForComputer } from "../lib/session-offline-catalog";
import { scopedCatalogFilters } from "./use-session-catalog";
import { useSessionGroupFolds } from "./use-session-group-folds";
import { useSessionListGroups } from "./use-session-list-groups";
import {
  boardHeaderScope,
  useSessionsBoardComputers,
  type MultiComputerBoardModel,
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
  scopedComputerId?: ComputerId;
  hideAlwaysOnTop?: boolean;
  /** The board's identity for its selection: scope, layer, breadth, profile lens and grouping. */
  selectionKey: string;
  /** Remote-computer provenance for multi-computer cards (`_uiux.md` U5, `K7`). */
  computerBySessionId?: ReadonlyMap<string, SessionRowComputerInfo>;
  /** Remote computers whose board read failed (Together mode strip only, `K6`). */
  failedComputers?: readonly ScopedComputerFailureItem[];
  /** True when at least one remote computer is enabled for this window (`F9`, `F13`). */
  hasRemoteComputers?: boolean;
  writableComputers?: ReadonlySet<ComputerId>;
}

function useSessionsBoardPage(
  filters: SessionListFilters,
  enabled: boolean,
  computerId: ComputerId = LOCAL_COMPUTER_ID
): SessionsBoardPage {
  const page = useInfiniteQuery({ ...sessionBoardPageOptions(filters, computerId), enabled });
  if (!enabled) return emptyBoardShelf();
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
  computerId: ComputerId = LOCAL_COMPUTER_ID,
  offline = false
): SessionsBoardColumnView[] {
  const queryClient = useContext(QueryClientContext);
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
    ...(isRemoteComputerId(computerId) ? {} : { refetchInterval: SESSION_BOARD_REFETCH_MS }),
    enabled: active,
  });
  const counts = active ? facets.data?.facets.board : undefined;
  const pages = { needs_you: needsYou, working, waiting, in_review: inReview, done };
  return SESSION_BOARD_COLUMNS.map(({ id, label }) => ({
    ...pages[id],
    ...(offline
      ? { sessions: readCachedOfflineRowsForComputer(queryClient, computerId, column(id)) }
      : {}),
    id,
    label,
    count: counts?.[id],
  }));
}

function emptyBoardShelf(): SessionsBoardShelf {
  return {
    sessions: [],
    loading: false,
    failure: null,
    retrying: false,
    fetching: false,
    hasMore: false,
    loadingMore: false,
    loadMore: () => undefined,
    retry: () => undefined,
    total: 0,
    count: 0,
  };
}

function resolveBoardColumns(input: {
  grouped: boolean;
  banded: boolean;
  multi: MultiComputerBoardModel;
  singleColumns: SessionsBoardColumnView[];
}): SessionsBoardColumnView[] {
  if (input.grouped || input.banded) return [];
  if (input.multi.scopedRemoteSource?.workspacesLoading) {
    return input.singleColumns.map(column => ({ ...column, loading: true }));
  }
  return input.multi.togetherColumns ?? input.singleColumns;
}

function resolveBoardLanes(input: {
  banded: boolean;
  togetherGrouped: boolean;
  grouped: boolean;
  multi: MultiComputerBoardModel;
  arrangedGroups: readonly SessionListGroupSpec[];
  shared: SessionListFilters;
  folds: ReturnType<typeof useSessionGroupFolds>;
}): SessionsBoardLane[] | null {
  if (input.banded) return null;
  if (input.togetherGrouped) return input.multi.togetherLanes;
  if (!input.grouped) return null;
  if (input.multi.scopedOfflineLanes) return input.multi.scopedOfflineLanes;
  return input.arrangedGroups.map(group => ({
    group,
    filters: scopedCatalogFilters(
      group.scope,
      input.shared,
      group.lens,
      group.computerId,
      group.matchedWorkspaceIds
    ).scoped,
    collapsed: input.folds.isCollapsed(group.key),
    onToggle: () => input.folds.toggle(group.key),
  }));
}

function resolveBoardRead(input: {
  multi: MultiComputerBoardModel;
  params: ProfileScopeParams;
  registeredWorkspaces: ReturnType<typeof useActiveWorkspace>["registeredWorkspaces"];
  scope: DataScope | null;
  shared: SessionListFilters;
  active: boolean;
  grouped: boolean;
}) {
  const { multi, params, registeredWorkspaces, scope, shared, active, grouped } = input;
  const targetComputerId: ComputerId = multi.scopedRemoteSource?.key ?? LOCAL_COMPUTER_ID;
  const isRemoteScoped = isRemoteComputerId(targetComputerId);
  const targetLens: ProfileScopeParams = isRemoteScoped ? { all_profiles: true } : params;
  const targetWorkspaces = multi.scopedRemoteSource?.workspaces ?? registeredWorkspaces;
  const matchedWorkspaceIds = multi.scopedRemoteSource?.matchedWorkspaceIds ?? null;
  const { scoped: base, emptyRemoteMatch } = scopedCatalogFilters(
    scope,
    shared,
    targetLens,
    targetComputerId,
    matchedWorkspaceIds
  );

  const banded = multi.computerBands !== null;
  const togetherGrouped = multi.togetherLanes !== null;
  const togetherFlat = multi.togetherColumns !== null;
  const source = multi.scopedRemoteSource;
  const canRead = active && !emptyRemoteMatch && (source === null || source.read === "ok");
  const singleColumnsActive = canRead && !grouped && !banded && !togetherFlat;
  const singleGroupingActive = canRead && grouped && !banded && !togetherGrouped;
  const offlineColumns = active && !emptyRemoteMatch && !grouped && source?.read === "offline";
  return {
    targetComputerId,
    isRemoteScoped,
    targetLens,
    targetWorkspaces,
    matchedWorkspaceIds,
    base,
    emptyRemoteMatch,
    banded,
    togetherGrouped,
    singleColumnsActive,
    singleGroupingActive,
    offlineColumns,
  };
}

function resolveBoardShelf(
  multi: SessionsBoardShelf | undefined,
  single: SessionsBoardPage,
  emptyRemoteMatch: boolean
): SessionsBoardShelf {
  if (multi) return multi;
  if (emptyRemoteMatch) return emptyBoardShelf();
  return { ...single, count: single.total };
}

function boardBreadth(scope: DataScope | null, computerScope: ComputerId, aggregate: boolean) {
  return {
    allProjects: scope?.kind === "all",
    allProfiles: aggregate || (computerScope !== "all" && isRemoteComputerId(computerScope)),
  };
}

function boardWorkspaceNames(
  multi: MultiComputerBoardModel,
  workspaces: readonly WorkspacePayload[],
  allProjects: boolean
): SessionsBoardView["workspaceNameOf"] {
  if (multi.hasRemoteComputers) return multi.workspaceNameOf;
  if (!allProjects) return () => null;
  const names = new Map(workspaces.map(workspace => [workspace.id, workspace.name]));
  return session => workspaceBadgeIdentity(session.workspace_id, names).name;
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
  const breadth = boardBreadth(scope, order.computerScope, aggregate);
  const effectiveGroupBy = effectiveSessionListGroupBy(groupBy, breadth);
  const grouped = effectiveGroupBy !== "none";
  const population = {
    children: hideChildren ? ("exclude" as const) : undefined,
    // Override the List filter and use the catalog default for Kanban.
    subagents: undefined,
  };
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
    sort,
    showAutomation,
    population,
    enabled: active,
  });

  const {
    targetComputerId,
    isRemoteScoped,
    targetLens,
    targetWorkspaces,
    matchedWorkspaceIds,
    base,
    emptyRemoteMatch,
    banded,
    togetherGrouped,
    singleColumnsActive,
    singleGroupingActive,
    offlineColumns,
  } = resolveBoardRead({ multi, params, registeredWorkspaces, scope, shared, active, grouped });
  const columns = useSessionsBoardColumns(
    base,
    singleColumnsActive,
    targetComputerId,
    offlineColumns
  );

  const grouping = useSessionListGroups({
    groupBy: effectiveGroupBy,
    scope,
    lens: targetLens,
    workspaces: targetWorkspaces,
    shelf: "active",
    showAutomation,
    enabled: singleGroupingActive,
    population,
    refetchInterval: isRemoteScoped ? undefined : SESSION_BOARD_REFETCH_MS,
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
  const localShelfActive = active && !multi.multiArchive && !emptyRemoteMatch;
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
  return {
    ...boardHeaderScope(scope, multi.scopedRemoteSource),
    columns: resolveBoardColumns({ grouped, banded, multi, singleColumns: columns }),
    lanes: resolveBoardLanes({
      banded,
      togetherGrouped,
      grouped,
      multi,
      arrangedGroups: arranged.groups,
      shared,
      folds,
    }),
    computerBands: multi.computerBands,
    lanesLoading: singleGroupingActive
      ? grouping.loading
      : grouped && multi.scopedRemoteSource?.workspacesLoading === true,
    lanesError: singleGroupingActive ? grouping.error : undefined,
    layoutError: singleGroupingActive ? arranged.loadError : undefined,
    groupBy: effectiveGroupBy,
    groupByOptions: availableSessionListGroupBys(breadth),
    archive: resolveBoardShelf(multi.multiArchive, archive, emptyRemoteMatch),
    later: resolveBoardShelf(multi.multiLater, later, emptyRemoteMatch),
    workspaceNameOf: boardWorkspaceNames(multi, targetWorkspaces, breadth.allProjects),
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
    writableComputers: multi.writableComputers,
  };
}
