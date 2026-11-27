import { isRemoteComputerId, LOCAL_COMPUTER_ID, type ComputerId } from "@/lib/computer-scope";
import type { ScopedComputerFailureItem } from "@/systems/computers";
import { useProfileReadScope, type ProfileScopeParams } from "@/systems/profiles";
import { workspaceBadgeIdentity, type DataScope, type WorkspacePayload } from "@/systems/workspace";

import type { SessionBoardColumnId, SessionBoardPayload } from "../lib/session-board-contract";
import type { SessionRowComputerInfo } from "../lib/session-list-computer";
import { computerWorkspaceBadgeIdentity } from "../lib/session-list-computer-filter";
import {
  profileSessionGroups,
  projectSessionGroups,
  type SessionListGroupSpec,
} from "../lib/session-list-groups";
import type { SessionListGroupBy, SessionListSort } from "../lib/session-list-preferences";
import {
  buildTogetherProfileGroups,
  buildTogetherProjectGroups,
} from "../lib/session-list-together";
import { sessionComputerOf, sessionRowKey } from "../lib/session-row-ref";
import { computerFoldKey, resolveComputerSectionFold } from "../lib/session-sidebar-scope";
import { sessionCatalogScopedFilters } from "../lib/session-catalog-scope";
import type { SessionListFilters } from "../types";
import {
  useComputerSessionsSources,
  type ComputerSessionsSource,
} from "./use-computer-sessions-sources";
import { useSessionComputerScope } from "./use-session-computer-scope";
import { qualifySessionGroupFoldKey, sessionGroupFoldLens } from "./use-session-group-folds";
import { useSessionSidebarState } from "./use-session-sidebar-state";
import { useSessionWindowOrder } from "./use-session-window-order";
import type {
  SessionsBoardColumnView,
  SessionsBoardLane,
  SessionsBoardShelf,
} from "./use-sessions-board";
import {
  boardFiltersForSource,
  useMultiComputerBoardShelf,
} from "./use-sessions-board-multi-pages";
import { useTogetherBoardColumns } from "./use-together-board-columns";

export interface SessionsBoardComputerBand {
  source: ComputerSessionsSource;
  collapsed: boolean;
  onToggle: () => void;
  /** Filters this band's ungrouped columns read. */
  baseFilters: SessionListFilters;
  /** True when topbar focuses a project absent on this remote computer (`Opus 5`, `Astra 10`). */
  emptyRemoteMatch?: boolean;
  /** Project or profile swimlanes inside this band; `null` when the board is ungrouped. */
  lanes: SessionsBoardLane[] | null;
  /** Exact server counts for the 5 columns from `facets.board`; undefined when unknown (`K3`, `K5`). */
  boardCounts: Record<SessionBoardColumnId, number> | undefined;
  /** Resolves a card's workspace name on this computer (never raw `ws_…`, `P1`). */
  workspaceNameOf: (session: SessionBoardPayload) => string | null;
}

export interface MultiComputerBoardModel {
  hasRemoteComputers: boolean;
  writableComputers: ReadonlySet<ComputerId>;
  /** Active effective computer scope (`"all"`, `"local"`, or a remote computer id). */
  effectiveScope: string;
  /** Non-null when scoped to a single remote computer (`F8`). */
  scopedRemoteSource: ComputerSessionsSource | null;
  /** Non-null in All computers + Separate mode (`K2`, `K3`). */
  computerBands: SessionsBoardComputerBand[] | null;
  /** Non-null in All computers + Together + Project/Profile mode (`K1`, `F6`, `Opus 3`). */
  togetherLanes: SessionsBoardLane[] | null;
  /** Non-null in All computers + Together + None mode (`Opus 4`, `Astra 9`). */
  togetherColumns: SessionsBoardColumnView[] | null;
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

function laneFiltersForGroup(
  group: SessionListGroupSpec,
  source: ComputerSessionsSource,
  scope: DataScope | null,
  shared: SessionListFilters
): SessionListFilters {
  if (group.kind === "profile" && !source.isLocal && scope?.kind === "project") {
    const matched = source.matchedWorkspaceIds ?? [];
    return {
      ...shared,
      all_workspaces: true,
      ...(matched.length > 0 ? { workspace_ids: [...matched] } : {}),
      ...group.lens,
    };
  }
  return sessionCatalogScopedFilters(group.scope, shared, group.lens);
}

function buildLane(input: {
  group: SessionListGroupSpec;
  source: ComputerSessionsSource;
  scope: DataScope | null;
  shared: SessionListFilters;
  foldLens: string;
  sidebar: ReturnType<typeof useSessionSidebarState>;
}): SessionsBoardLane {
  const { group, source, scope, shared, foldLens, sidebar } = input;
  const foldKey = qualifySessionGroupFoldKey(foldLens, group.key, source.key);
  const collapsed = sidebar.groupFlag(foldKey) ?? sidebar.groupFolds.has(foldKey);
  return {
    group,
    filters: laneFiltersForGroup(group, source, scope, shared),
    collapsed,
    onToggle: () => sidebar.toggleGroup(foldKey),
  };
}

function buildBandLanes(input: {
  source: ComputerSessionsSource;
  groupBy: SessionListGroupBy;
  scope: DataScope | null;
  localLens: ProfileScopeParams;
  shared: SessionListFilters;
  foldLens: string;
  sidebar: ReturnType<typeof useSessionSidebarState>;
}): SessionsBoardLane[] | null {
  const { source, groupBy, scope, localLens, shared, foldLens, sidebar } = input;
  if (groupBy === "none") return null;
  if (source.read !== "ok" && source.read !== "offline") return null;
  if (groupBy === "project") {
    const lens: ProfileScopeParams = source.isLocal ? localLens : { all_profiles: true };
    return projectSessionGroups(source.facets, {
      workspaces: source.workspaces,
      lens,
      computerId: source.key,
      offline: source.read === "offline",
    }).map(group => buildLane({ group, source, scope, shared, foldLens, sidebar }));
  }
  const emptyRemoteProject =
    !source.isLocal && scope?.kind === "project" && (source.matchedWorkspaceIds?.length ?? 0) === 0;
  if (emptyRemoteProject) return [];
  const profileScope: DataScope =
    !source.isLocal && scope?.kind === "project" ? { kind: "all" } : (scope ?? { kind: "all" });
  return profileSessionGroups(source.facets, {
    profiles: source.profiles ?? [],
    scope: profileScope,
    computerId: source.key,
    offline: source.read === "offline",
  }).map(group => buildLane({ group, source, scope, shared, foldLens, sidebar }));
}

function buildTogetherLanes(input: {
  enabled: boolean;
  groupBy: SessionListGroupBy;
  sources: readonly ComputerSessionsSource[];
  sourcesById: ReadonlyMap<ComputerId, ComputerSessionsSource>;
  scope: DataScope | null;
  localLens: ProfileScopeParams;
  shared: SessionListFilters;
  foldLens: string;
  sidebar: ReturnType<typeof useSessionSidebarState>;
}): SessionsBoardLane[] | null {
  const { enabled, groupBy, sources, sourcesById, scope, localLens, shared, foldLens, sidebar } =
    input;
  if (!enabled || groupBy === "none") return null;
  const specs =
    groupBy === "project"
      ? buildTogetherProjectGroups(sources, localLens)
      : buildTogetherProfileGroups(sources, scope ?? { kind: "all" });
  return specs.flatMap(group => {
    const source = sourcesById.get(group.computerId ?? LOCAL_COMPUTER_ID);
    return source ? [buildLane({ group, source, scope, shared, foldLens, sidebar })] : [];
  });
}

function addShelfProvenance(
  target: Map<string, SessionRowComputerInfo>,
  shelf: SessionsBoardShelf | undefined,
  sourcesById: ReadonlyMap<ComputerId, ComputerSessionsSource>
): void {
  if (!shelf) return;
  for (const session of shelf.sessions) {
    const computerId = sessionComputerOf(session);
    if (!isRemoteComputerId(computerId)) continue;
    const source = sourcesById.get(computerId);
    if (!source) continue;
    target.set(sessionRowKey({ computerId: source.key, sessionId: session.id }), {
      id: source.key,
      name: source.name,
      platform: source.computer.platform,
      iconKind: source.computer.iconKind,
      offline: source.read === "offline",
    });
  }
}

function buildComputerBands(input: {
  enabled: boolean;
  sources: readonly ComputerSessionsSource[];
  scope: DataScope | null;
  shared: SessionListFilters;
  localLens: ProfileScopeParams;
  groupBy: SessionListGroupBy;
  foldLens: string;
  sidebar: ReturnType<typeof useSessionSidebarState>;
  allProjects: boolean;
}): SessionsBoardComputerBand[] | null {
  const { enabled, sources, scope, shared, localLens, groupBy, foldLens, sidebar, allProjects } =
    input;
  if (!enabled) return null;
  return sources.map(source => {
    const foldKey = computerFoldKey(source.key);
    const defaultFolded = defaultBandFolded(source);
    const collapsed = resolveComputerSectionFold({
      flag: sidebar.groupFlag(foldKey),
      foldAll: sidebar.foldAll,
      defaultFolded,
    });
    const filterSpec = boardFiltersForSource(source, scope, shared, localLens);
    return {
      source,
      collapsed,
      onToggle: () => sidebar.toggleGroup(foldKey, defaultFolded),
      baseFilters: filterSpec.filters,
      emptyRemoteMatch: filterSpec.emptyRemoteMatch,
      lanes: buildBandLanes({ source, groupBy, scope, localLens, shared, foldLens, sidebar }),
      boardCounts: source.facets?.facets.board,
      workspaceNameOf: workspaceNameResolverFor(source.workspaces, allProjects, !source.isLocal),
    };
  });
}

function boardWorkspaceNameResolver(
  sources: readonly ComputerSessionsSource[],
  allProjects: boolean
) {
  const namesByComputer = new Map<ComputerId, ReadonlyMap<string, string>>(
    sources.map(source => [
      source.key,
      new Map(source.workspaces.map(workspace => [workspace.id, workspace.name])),
    ])
  );
  return (session: SessionBoardPayload): string | null => {
    if (!allProjects) return null;
    const computerId = sessionComputerOf(session) ?? LOCAL_COMPUTER_ID;
    const names = namesByComputer.get(computerId) ?? new Map<string, string>();
    return isRemoteComputerId(computerId)
      ? computerWorkspaceBadgeIdentity(session.workspace_id, names, { isRemote: true }).name
      : workspaceBadgeIdentity(session.workspace_id, names).name;
  };
}

/**
 * Multi-computer projection for Kanban (`F13`, `K1`–`K7`):
 * - Separate: computer bands holding project/profile lanes or columns, with folded 5-column counts.
 * - Together: `project · computer` / `profile · computer` lanes, or merged columns (`Opus 3`, `Opus 4`).
 * - Scoped: one computer's board with no band header.
 */
export function useSessionsBoardComputers(options: {
  scope: DataScope | null;
  shared: SessionListFilters;
  localLens: ProfileScopeParams;
  groupBy: SessionListGroupBy;
  sort: SessionListSort;
  showAutomation: boolean;
  population?: Pick<SessionListFilters, "children" | "subagents">;
  enabled: boolean;
}): MultiComputerBoardModel {
  const { scope, shared, localLens, groupBy, sort, showAutomation, population, enabled } = options;
  const order = useSessionWindowOrder();
  const sidebar = useSessionSidebarState();
  const profile = useProfileReadScope();
  const allProjects = scope?.kind === "all";

  const sourcesResult = useComputerSessionsSources({
    scope,
    lens: localLens,
    shelf: "active",
    showAutomation,
    population,
    includeProfiles: groupBy === "profile",
    enabled,
  });
  const scopeState = useSessionComputerScope();
  const hasRemote = sourcesResult.hasRemoteComputers;
  const effectiveScope = hasRemote ? scopeState.scope : "all";
  const sourcesById = new Map<ComputerId, ComputerSessionsSource>(
    sourcesResult.sources.map(source => [source.key, source])
  );
  const scopedRemoteSource =
    hasRemote && isRemoteComputerId(effectiveScope)
      ? (sourcesById.get(effectiveScope) ?? null)
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
    enabled && (multiVisible || scopedRemoteSource !== null)
  );
  const multiLater = useMultiComputerBoardShelf(
    activeSources,
    scope,
    shared,
    localLens,
    "later",
    enabled && (multiVisible || scopedRemoteSource !== null)
  );
  const togetherBoard = useTogetherBoardColumns({
    sources: activeSources,
    scope,
    shared,
    localLens,
    sort,
    enabled: enabled && together && groupBy === "none",
  });

  const computerBySessionId = new Map<string, SessionRowComputerInfo>(
    togetherBoard.computerBySessionId
  );
  addShelfProvenance(computerBySessionId, multiArchive, sourcesById);
  addShelfProvenance(computerBySessionId, multiLater, sourcesById);

  const foldLens = sessionGroupFoldLens(scope, profile, groupBy);
  const computerBands = buildComputerBands({
    enabled: separate,
    sources: sourcesResult.sources,
    scope,
    shared,
    localLens,
    groupBy,
    foldLens,
    sidebar,
    allProjects,
  });

  const togetherLanes = buildTogetherLanes({
    enabled: together,
    groupBy,
    sources: sourcesResult.sources,
    sourcesById,
    scope,
    localLens,
    shared,
    foldLens,
    sidebar,
  });

  const failedComputers: ScopedComputerFailureItem[] = together
    ? sourcesResult.sources.flatMap(source =>
        source.read === "failed" ? [{ id: source.key, name: source.name, retry: source.retry }] : []
      )
    : [];

  const workspaceNameOf = boardWorkspaceNameResolver(sourcesResult.sources, allProjects);

  return {
    hasRemoteComputers: hasRemote,
    writableComputers: new Set(
      sourcesResult.sources.flatMap(source => (source.canOperate ? [source.key] : []))
    ),
    effectiveScope,
    scopedRemoteSource,
    computerBands,
    togetherLanes,
    togetherColumns: togetherBoard.columns,
    computerBySessionId,
    failedComputers,
    multiArchive,
    multiLater,
    workspaceNameOf,
  };
}
