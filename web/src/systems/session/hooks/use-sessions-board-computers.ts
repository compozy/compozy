import { useQueries } from "@tanstack/react-query";

import { isRemoteComputerId, LOCAL_COMPUTER_ID, type ComputerId } from "@/lib/computer-scope";
import type { ScopedComputerFailureItem } from "@/systems/computers";
import type { ProfileScopeParams } from "@/systems/profiles";
import { workspaceBadgeIdentity, type DataScope, type WorkspacePayload } from "@/systems/workspace";

import { fetchSessionCatalogPage } from "../adapters/session-catalog-api";
import {
  SESSION_BOARD_PAGE_SIZE,
  SESSION_BOARD_STALE_MS,
  sessionBoardPageOptions,
} from "../lib/session-catalog-options";
import { sessionCatalogScopedFilters } from "../lib/session-catalog-scope";
import type { SessionBoardColumnId, SessionBoardPayload } from "../lib/session-board-contract";
import type { SessionRowComputerInfo } from "../lib/session-list-computer";
import { computerWorkspaceBadgeIdentity } from "../lib/session-list-computer-filter";
import { projectSessionGroups, type SessionListGroupSpec } from "../lib/session-list-groups";
import type { SessionListGroupBy } from "../lib/session-list-preferences";
import { normalizeSessionListFilters } from "../lib/session-list-query";
import { sessionListShelfFilters } from "../lib/session-list-shelf";
import { buildTogetherProjectGroups } from "../lib/session-list-together";
import {
  sessionComputerOf,
  sessionRowKey,
  withSessionsComputer,
} from "../lib/session-row-ref";
import { computerFoldKey } from "../lib/session-sidebar-scope";
import type { SessionListFilters } from "../types";
import {
  useComputerSessionsSources,
  type ComputerSessionsSource,
} from "./use-computer-sessions-sources";
import { useSessionComputerScope } from "./use-session-computer-scope";
import { useSessionSidebarState } from "./use-session-sidebar-state";
import { useSessionWindowOrder } from "./use-session-window-order";
import type { SessionsBoardLane, SessionsBoardShelf } from "./use-sessions-board";

export interface SessionsBoardComputerBand {
  source: ComputerSessionsSource;
  collapsed: boolean;
  onToggle: () => void;
  /** Filters this band's ungrouped columns read. */
  baseFilters: SessionListFilters;
  /** Project swimlanes inside this band; `null` when the board is ungrouped. */
  lanes: SessionsBoardLane[] | null;
  /** Exact server counts for the 5 columns from `facets.board`; undefined when unknown (`K3`, `K5`). */
  boardCounts: Record<SessionBoardColumnId, number> | undefined;
  /** Resolves a card's workspace name on this computer (never raw `ws_…`, `P1`). */
  workspaceNameOf: (session: SessionBoardPayload) => string | null;
}

export interface MultiComputerBoardModel {
  hasRemoteComputers: boolean;
  /** Active effective computer scope (`"all"`, `"local"`, or a remote computer id). */
  effectiveScope: string;
  /** Non-null when scoped to a single remote computer (`F8`). */
  scopedRemoteSource: ComputerSessionsSource | null;
  /** Non-null in All computers + Separate mode (`K2`, `K3`). */
  computerBands: SessionsBoardComputerBand[] | null;
  /** Non-null in All computers + Together + Project mode (`K1`, `F6`). */
  togetherLanes: SessionsBoardLane[] | null;
  /** Combined provenance map for Together / multi-computer shelf cards (`K7`). */
  computerBySessionId: ReadonlyMap<string, SessionRowComputerInfo>;
  /** Failures for the compact strip (Together mode only; Separate shows them in bands, `K6`). */
  failedComputers: readonly ScopedComputerFailureItem[];
  /** Multi-computer combined Archive shelf when spanning multiple computers (`K7`). */
  multiArchive?: SessionsBoardShelf;
  /** Multi-computer combined Later shelf when spanning multiple computers (`K7`). */
  multiLater?: SessionsBoardShelf;
  /** Multi-computer project name resolver (`P1`). */
  workspaceNameOf: (session: SessionBoardPayload) => string | null;
}

function boardFiltersForSource(
  source: ComputerSessionsSource,
  scope: DataScope | null,
  shared: SessionListFilters,
  localLens: ProfileScopeParams
): SessionListFilters {
  if (source.isLocal) {
    return sessionCatalogScopedFilters(scope, shared, localLens);
  }
  if (scope?.kind === "project") {
    const matched = source.matchedWorkspaceIds ?? [];
    return {
      ...shared,
      all_workspaces: true,
      all_profiles: true,
      ...(matched.length > 0 ? { workspace_ids: [...matched] } : {}),
    };
  }
  return sessionCatalogScopedFilters(scope?.kind === "user" ? scope : { kind: "all" }, shared, {
    all_profiles: true,
  });
}

function defaultBandFolded(source: ComputerSessionsSource): boolean {
  if (source.read === "offline" || source.read === "offline_empty") return true;
  if (source.read === "ok" && source.count === 0) return true;
  return false;
}

function workspaceNameResolverFor(
  workspaces: readonly WorkspacePayload[],
  allProjects: boolean,
  isRemote: boolean
): (session: SessionBoardPayload) => string | null {
  if (!allProjects) return () => null;
  const names = new Map(workspaces.map(workspace => [workspace.id, workspace.name]));
  return (session: SessionBoardPayload) =>
    isRemote
      ? computerWorkspaceBadgeIdentity(session.workspace_id, names, { isRemote: true }).name
      : workspaceBadgeIdentity(session.workspace_id, names).name;
}

function useMultiComputerBoardShelf(
  sources: readonly ComputerSessionsSource[],
  scope: DataScope | null,
  shared: SessionListFilters,
  localLens: ProfileScopeParams,
  shelf: "archive" | "later",
  enabled: boolean
): SessionsBoardShelf | undefined {
  const shelfFilters = sessionListShelfFilters(shelf);
  const answered = enabled
    ? sources.filter(source => source.read === "ok" || source.read === "offline")
    : [];
  const queries = useQueries({
    queries: answered.map(source => {
      const filters = {
        ...boardFiltersForSource(source, scope, shared, localLens),
        ...shelfFilters,
      };
      const emptyRemoteMatch =
        !source.isLocal &&
        scope?.kind === "project" &&
        (source.matchedWorkspaceIds?.length ?? 0) === 0;
      const opts = sessionBoardPageOptions(filters, source.key);
      const normalized = normalizeSessionListFilters({
        ...filters,
        limit: SESSION_BOARD_PAGE_SIZE,
      });
      return {
        queryKey: [...opts.queryKey, "shelf-first"] as const,
        queryFn: async ({ signal }: { signal: AbortSignal }) => {
          const page = isRemoteComputerId(source.key)
            ? await fetchSessionCatalogPage(normalized, signal, source.key)
            : await fetchSessionCatalogPage(normalized, signal);
          return isRemoteComputerId(source.key)
            ? { ...page, sessions: withSessionsComputer(page.sessions, source.key) }
            : page;
        },
        enabled: enabled && source.read === "ok" && !emptyRemoteMatch && scope !== null,
        staleTime: SESSION_BOARD_STALE_MS,
        retry: false as const,
      };
    }),
  });
  if (!enabled) return undefined;

  const sessions: SessionBoardPayload[] = [];
  let total = 0;
  let hasTotal = false;
  let loading = false;
  let fetching = false;
  let allFailed = answered.length > 0;

  for (let index = 0; index < answered.length; index++) {
    const query = queries[index];
    const source = answered[index]!;
    if (query?.isLoading && source.read === "ok") loading = true;
    if (query?.isFetching) fetching = true;
    if (!query?.isError) allFailed = false;
    const page = query?.data;
    if (!page) continue;
    for (const session of page.sessions) sessions.push(session as SessionBoardPayload);
    if (page.page.total != null) {
      hasTotal = true;
      total += page.page.total;
    } else if (shelf === "later") {
      hasTotal = true;
      total += source.laterCount;
    }
  }
  sessions.sort((left, right) => Date.parse(right.updated_at) - Date.parse(left.updated_at));
  return {
    sessions,
    loading: loading && sessions.length === 0,
    failure: allFailed && sessions.length === 0 ? "initial" : null,
    retrying: false,
    fetching,
    hasMore: false,
    loadingMore: false,
    loadMore: () => undefined,
    retry: () => {
      for (const query of queries) void query.refetch();
    },
    total: hasTotal ? total : undefined,
    count: hasTotal ? total : undefined,
  };
}

/**
 * Multi-computer projection for Kanban (`F13`, `K1`–`K7`):
 * - Separate: computer bands holding project lanes or columns, with folded 5-column facet counts.
 * - Together: flat `project · computer` lanes with computer marks, or merged columns.
 * - Scoped: one computer's board with no band header.
 */
export function useSessionsBoardComputers(options: {
  scope: DataScope | null;
  shared: SessionListFilters;
  localLens: ProfileScopeParams;
  groupBy: SessionListGroupBy;
  showAutomation: boolean;
  enabled: boolean;
}): MultiComputerBoardModel {
  const { scope, shared, localLens, groupBy, showAutomation, enabled } = options;
  const order = useSessionWindowOrder();
  const sidebar = useSessionSidebarState();
  const allProjects = scope?.kind === "all";

  const sourcesResult = useComputerSessionsSources({
    scope,
    lens: localLens,
    shelf: "active",
    showAutomation,
    enabled,
  });
  const scopeState = useSessionComputerScope();
  const hasRemote = sourcesResult.hasRemoteComputers;
  const effectiveScope = hasRemote ? scopeState.scope : "all";
  const scopedRemoteSource =
    hasRemote && isRemoteComputerId(effectiveScope)
      ? (sourcesResult.sources.find(source => source.key === effectiveScope) ?? null)
      : null;
  const multiVisible = hasRemote && effectiveScope === "all";
  const separate = multiVisible && order.computers === "separate";
  const together = multiVisible && order.computers === "together";

  const activeSources = scopedRemoteSource
    ? [scopedRemoteSource]
    : multiVisible
      ? sourcesResult.sources
      : [];

  const multiArchive = useMultiComputerBoardShelf(
    activeSources,
    scope,
    shared,
    localLens,
    "archive",
    enabled && multiVisible
  );
  const multiLater = useMultiComputerBoardShelf(
    activeSources,
    scope,
    shared,
    localLens,
    "later",
    enabled && multiVisible
  );

  const computerBySessionId = new Map<string, SessionRowComputerInfo>();
  const namesByComputer = new Map<ComputerId, ReadonlyMap<string, string>>();
  for (const source of sourcesResult.sources) {
    namesByComputer.set(
      source.key,
      new Map(source.workspaces.map(workspace => [workspace.id, workspace.name]))
    );
  }
  const addShelfProvenance = (shelf: SessionsBoardShelf | undefined) => {
    if (!shelf) return;
    for (const session of shelf.sessions) {
      const computerId = sessionComputerOf(session);
      if (!isRemoteComputerId(computerId)) continue;
      const source = sourcesResult.sources.find(item => item.key === computerId);
      if (!source) continue;
      const info: SessionRowComputerInfo = {
        id: source.key,
        name: source.name,
        platform: source.computer.platform,
        iconKind: source.computer.iconKind,
        offline: source.read === "offline",
      };
      computerBySessionId.set(
        sessionRowKey({ computerId: source.key, sessionId: session.id }),
        info
      );
      computerBySessionId.set(session.id, info);
    }
  };
  addShelfProvenance(multiArchive);
  addShelfProvenance(multiLater);

  const makeLane = (
    group: SessionListGroupSpec,
    source: ComputerSessionsSource
  ): SessionsBoardLane => {
    const foldKey = `board:${source.key}:${group.key}`;
    const explicit = sidebar.groupFolds.has(foldKey);
    const collapsed = sidebar.foldAll === "collapsed" ? !explicit : explicit;
    return {
      group,
      filters: sessionCatalogScopedFilters(group.scope, shared, group.lens),
      collapsed,
      onToggle: () => sidebar.toggleGroup(foldKey),
    };
  };

  const computerBands: SessionsBoardComputerBand[] | null = separate
    ? sourcesResult.sources.map(source => {
        const foldKey = computerFoldKey(source.key);
        const explicit = sidebar.groupFolds.has(foldKey);
        const defaultFolded = defaultBandFolded(source);
        const collapsed =
          sidebar.foldAll === "collapsed"
            ? !explicit
            : sidebar.foldAll === "groups"
              ? explicit
              : defaultFolded
                ? !explicit
                : explicit;
        const lens: ProfileScopeParams = source.isLocal ? localLens : { all_profiles: true };
        const lanes =
          groupBy === "project" && (source.read === "ok" || source.read === "offline")
            ? projectSessionGroups(source.facets, {
                workspaces: source.workspaces,
                lens,
                computerId: source.key,
                offline: source.read === "offline",
              }).map(group => makeLane(group, source))
            : null;
        return {
          source,
          collapsed,
          onToggle: () => sidebar.toggleGroup(foldKey),
          baseFilters: boardFiltersForSource(source, scope, shared, localLens),
          lanes,
          boardCounts: source.facets?.facets.board,
          workspaceNameOf: workspaceNameResolverFor(
            source.workspaces,
            allProjects,
            !source.isLocal
          ),
        };
      })
    : null;

  const togetherLanes: SessionsBoardLane[] | null =
    together && groupBy === "project"
      ? buildTogetherProjectGroups(sourcesResult.sources, localLens).map(group => {
          const source =
            sourcesResult.sources.find(
              item => item.key === (group.computerId ?? LOCAL_COMPUTER_ID)
            ) ?? sourcesResult.sources[0]!;
          return makeLane(group, source);
        })
      : null;

  const failedComputers: ScopedComputerFailureItem[] = [];
  if (together) {
    for (const source of sourcesResult.sources) {
      if (source.read === "failed") {
        failedComputers.push({ id: source.key, name: source.name, retry: source.retry });
      }
    }
  }

  const workspaceNameOf = (session: SessionBoardPayload): string | null => {
    if (!allProjects) return null;
    const computerId = sessionComputerOf(session) ?? LOCAL_COMPUTER_ID;
    const names = namesByComputer.get(computerId) ?? new Map<string, string>();
    return isRemoteComputerId(computerId)
      ? computerWorkspaceBadgeIdentity(session.workspace_id, names, { isRemote: true }).name
      : workspaceBadgeIdentity(session.workspace_id, names).name;
  };

  return {
    hasRemoteComputers: hasRemote,
    effectiveScope,
    scopedRemoteSource,
    computerBands,
    togetherLanes,
    computerBySessionId,
    failedComputers,
    multiArchive,
    multiLater,
    workspaceNameOf,
  };
}
