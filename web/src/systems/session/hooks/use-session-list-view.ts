import { useDebouncedInput } from "@/hooks/use-debounced-input";
import { isRemoteComputerId, LOCAL_COMPUTER_ID, type ComputerId } from "@/lib/computer-scope";

import { useProfileReadScope, type ProfileScopeParams } from "@/systems/profiles";
import { useActiveWorkspace, workspaceSelectorOrAll, type DataScope } from "@/systems/workspace";

import type { SessionListPair } from "../lib/session-list-groups";
import { sessionListPair } from "../lib/session-list-groups";
import {
  availableSessionListGroupBys,
  effectiveSessionListGroupBy,
  type SessionListGroupBy,
} from "../lib/session-list-preferences";
import {
  awaitingReads,
  focusedScopeName,
  hiddenProjectGroupKeys,
  manualSortNote,
  newFolderUnavailableReason,
  SESSION_LIST_MANUAL_SORT_NOTE,
  sessionBadgeResolver,
  sessionListGroupFiltersFor,
  settledSearchCatalog,
} from "../lib/session-list-view-helpers";
import type { SessionListFilters } from "../types";
import type { SessionListViewModel, UseSessionListViewOptions } from "./session-list-view-types";

import { useSessionListContainerCatalog } from "./use-session-list-group-catalog";
import { useSessionFolders, type SessionListLayout } from "./use-session-folders";
import {
  useSessionListGroups,
  useSessionListArchivedCount,
  useSessionListLaterCount,
  useSessionListTotals,
  type SessionListTotals,
} from "./use-session-list-groups";
import { useSessionGroupFolds } from "./use-session-group-folds";
import {
  useSessionListPinnedRead,
  type SessionListPinnedRead,
} from "./use-session-list-pinned-read";
import { useSessionSidebarState } from "./use-session-sidebar-state";
import { useSessionWindowOrder } from "./use-session-window-order";
import { useProjectFolderGroups } from "./use-workspace-layout";

export { SESSION_LIST_MANUAL_SORT_NOTE };
export type { SessionListTotals } from "./use-session-list-groups";
export type { SessionListPinnedRead } from "./use-session-list-pinned-read";
export type { SessionListViewModel, UseSessionListViewOptions } from "./session-list-view-types";

function resolveViewLayoutFields(input: {
  flatLayout: boolean;
  readOnly: boolean;
  foldersLayout: SessionListLayout | undefined;
  foldersError: { retry: () => void } | undefined;
  arrangedError: { retry: () => void } | undefined;
}): Pick<SessionListViewModel, "layout" | "layoutError"> {
  if (input.flatLayout) {
    return {
      layout: input.readOnly ? undefined : input.foldersLayout,
      layoutError: input.foldersError,
    };
  }
  return { layout: undefined, layoutError: input.arrangedError };
}

function buildSessionListViewModel(input: {
  options: UseSessionListViewOptions;
  preferences: ReturnType<typeof useSessionWindowOrder>;
  profile: ReturnType<typeof useProfileReadScope>;
  sidebar: ReturnType<typeof useSessionSidebarState>;
  computerId: ComputerId;
  remote: boolean;
  readOnly: boolean;
  workspaces: ReadonlyArray<{ id: string; name: string }>;
  scope: DataScope | null;
  aggregate: boolean;
  breadth: { allProjects: boolean; allProfiles: boolean };
  groupBy: SessionListGroupBy;
  flat: boolean;
  pair: SessionListPair | null;
  search: string;
  remoteSearch: string;
  setSearch: ((search: string) => void) | undefined;
  groupFilters: SessionListFilters;
  grouping: ReturnType<typeof useSessionListGroups>;
  pinned: SessionListPinnedRead | undefined;
  arranged: ReturnType<typeof useProjectFolderGroups>;
  folders: ReturnType<typeof useSessionFolders>;
  awaiting: ReturnType<typeof awaitingReads>;
  partitions: ReturnType<typeof useSessionListContainerCatalog>["partitions"];
  totals: SessionListTotals | undefined;
  laterCount: number | undefined;
  archivedCount: number | undefined;
  groupFolds: ReturnType<typeof useSessionGroupFolds>;
}): SessionListViewModel {
  const {
    options,
    preferences,
    profile,
    sidebar,
    computerId,
    remote,
    readOnly,
    workspaces,
    scope,
    aggregate,
    breadth,
    groupBy,
    flat,
    pair,
    search,
    remoteSearch,
    setSearch,
    groupFilters,
    grouping,
    pinned,
    arranged,
    folders,
    awaiting,
    partitions,
    totals,
    laterCount,
    archivedCount,
    groupFolds,
  } = input;
  const searching = remoteSearch.trim() !== "";
  const hiddenGroupKeys = new Set(
    hiddenProjectGroupKeys(arranged.projectFolders?.tree, sidebar.folderFolds)
  );
  const manualBlocked = flat && pair === null;
  const flatLayout = flat && pair !== null;
  const selectionKey = JSON.stringify([
    preferences.computerScope,
    preferences.computers,
    scope,
    profile.key,
    groupBy,
  ]);

  return {
    ...(remote ? { computerId } : {}),
    ...(readOnly ? { readOnly: true } : {}),
    workspaces,
    matchedWorkspaceIds: options.matchedWorkspaceIds,
    computers: preferences.computers,
    setComputers: preferences.setComputers,
    computerScope: preferences.computerScope,
    setComputerScope: preferences.setComputerScope,
    ...resolveViewLayoutFields({
      flatLayout,
      readOnly,
      foldersLayout: folders.layout,
      foldersError: folders.loadError,
      arrangedError: arranged.loadError,
    }),
    pair,
    projectFolders: readOnly ? undefined : arranged.projectFolders,
    worktreeId: options.worktreeId,
    catalog: awaiting.catalog,
    partitions: partitions?.pages,
    folderOf: partitions?.folderOf,
    search,
    setSearch,
    sort: preferences.sort,
    storedGroupBy: preferences.groupBy,
    groupBy,
    groupByOptions: availableSessionListGroupBys(breadth),
    sortNote: manualSortNote(preferences.sort, manualBlocked, options.multiComputer),
    newFolderUnavailable: newFolderUnavailableReason(
      flat,
      pair !== null,
      breadth,
      groupBy,
      options.multiComputer
    ),
    shelf: sidebar.shelf,
    laterCount,
    archivedCount,
    setSort: preferences.setSort,
    setGroupBy: preferences.setGroupBy,
    showEmptyProjects: sidebar.showEmptyProjects,
    setShowEmptyProjects: sidebar.setShowEmptyProjects,
    showAutomation: sidebar.showAutomation,
    setShowAutomation: sidebar.setShowAutomation,
    setShelf: sidebar.setShelf,
    groups: arranged.groups,
    groupsLoading: awaiting.groupsLoading,
    groupsError: grouping.error,
    collapsedGroupKeys: {
      has: key => !searching && (groupFolds.isCollapsed(key) || hiddenGroupKeys.has(key)),
    },
    toggleGroup: key => {
      if (!searching) groupFolds.toggle(key);
    },
    foldAll: sidebar.foldAll,
    setFoldAll: sidebar.setFoldAll,
    openThreadIds: sidebar.openThreadIds,
    groupFilters,
    aggregate,
    scopeLabel: remote ? null : profile.scopeLabel,
    focusedWorkspaceId: scope ? workspaceSelectorOrAll(scope) : null,
    allProjects: breadth.allProjects,
    focusedScopeName: focusedScopeName(scope, workspaces),
    ownerOf: profile.ownerOf,
    badgeOf: breadth.allProjects ? sessionBadgeResolver(workspaces, remote) : undefined,
    selectionKey,
    totals,
    pinned,
  };
}

/**
 * View model behind every session list: the topbar breadth and profile lens,
 * the window's order and grouping (shared with its Kanban), and either one flat catalog or
 * facets-driven groups (ADR-004).
 */
export function useSessionListView(options: UseSessionListViewOptions = {}): SessionListViewModel {
  const preferences = useSessionWindowOrder();
  const { registeredWorkspaces: localWorkspaces, dataScope } = useActiveWorkspace();
  const profile = useProfileReadScope();
  const sidebar = useSessionSidebarState();
  const {
    draftValue: localSearch,
    committedValue: localRemoteSearch,
    setDraftValue: setLocalSearch,
  } = useDebouncedInput({ externalValue: "", onCommit: () => undefined });
  const controlledSearch = options.search !== undefined;
  const search = controlledSearch ? options.search! : localSearch;
  const remoteSearch = controlledSearch ? options.search! : localRemoteSearch;
  const setSearch = controlledSearch ? undefined : setLocalSearch;

  const computerId = options.computerId ?? LOCAL_COMPUTER_ID;
  const remote = isRemoteComputerId(computerId);
  const workspaces = options.workspaces ?? localWorkspaces;
  const readOnly = options.readOnly === true;
  const enabled = options.enabled ?? true;
  const scope = options.scope === undefined ? dataScope : options.scope;

  // Remote computers always read all of their own profiles (Q4).
  const lens: ProfileScopeParams = remote ? { all_profiles: true } : profile.params;
  const aggregate = remote ? true : profile.aggregate;
  const breadth = { allProjects: scope?.kind === "all", allProfiles: aggregate };
  const groupBy = effectiveSessionListGroupBy(preferences.groupBy, breadth);
  const flat = groupBy === "none";
  const pair = sessionListPair(scope, lens, remote ? computerId : undefined);

  const showAutomation = sidebar.showAutomation;
  const shelf = sidebar.shelf;
  const groupFilters = sessionListGroupFiltersFor(
    remoteSearch,
    options.worktreeId,
    shelf,
    showAutomation
  );
  const grouping = useSessionListGroups({
    groupBy,
    scope,
    lens,
    workspaces,
    shelf,
    showAutomation,
    search: remoteSearch,
    worktreeId: options.worktreeId,
    pinned: groupFilters.pinned,
    enabled,
    computerId,
    matchedWorkspaceIds: options.matchedWorkspaceIds,
  });
  const pending = options.pending === true;
  const pinned = useSessionListPinnedRead({
    scope,
    pair,
    groupFilters,
    enabled: enabled && options.includePinned !== false,
    shelf,
    pending,
    computerId,
    lens,
    matchedWorkspaceIds: options.matchedWorkspaceIds,
    allowReorder: !options.multiComputer && !readOnly,
  });
  const arranged = useProjectFolderGroups({
    groups: grouping.groups,
    groupBy,
    breadth,
    lens,
    workspaces,
    showEmptyProjects: sidebar.showEmptyProjects,
    enabled: enabled && !readOnly,
    computerId,
  });
  const folders = useSessionFolders({
    workspaceId: pair?.workspaceId ?? null,
    profile: pair?.profile ?? (remote ? "" : profile.destination),
    enabled: enabled && flat && pair !== null && !readOnly,
    computerId,
  });
  const { catalog, partitions } = useSessionListContainerCatalog(
    {
      scope,
      pair,
      lens,
      computerId: remote ? computerId : undefined,
      matchedWorkspaceIds: options.matchedWorkspaceIds,
    },
    { groupFilters, sort: preferences.sort, shelf },
    folders,
    enabled && flat,
    search
  );
  const totals = useSessionListTotals({
    scope,
    lens,
    search: remoteSearch,
    worktreeId: options.worktreeId,
    shelf,
    showAutomation,
    enabled,
    computerId,
    matchedWorkspaceIds: options.matchedWorkspaceIds,
  });
  const laterCount = useSessionListLaterCount({
    scope,
    lens,
    worktreeId: options.worktreeId,
    enabled,
    computerId,
    matchedWorkspaceIds: options.matchedWorkspaceIds,
  });
  const archivedCount = useSessionListArchivedCount({
    scope,
    lens,
    enabled: enabled && shelf === "archive",
    computerId,
    matchedWorkspaceIds: options.matchedWorkspaceIds,
  });
  const awaiting = awaitingReads(
    flat ? settledSearchCatalog(catalog, search === remoteSearch) : undefined,
    grouping.loading,
    pending
  );
  const groupFolds = useSessionGroupFolds(scope, groupBy, remote ? computerId : undefined);

  return buildSessionListViewModel({
    options,
    preferences,
    profile,
    sidebar,
    computerId,
    remote,
    readOnly,
    workspaces,
    scope,
    aggregate,
    breadth,
    groupBy,
    flat,
    pair,
    search,
    remoteSearch,
    setSearch,
    groupFilters,
    grouping,
    pinned,
    arranged,
    folders,
    awaiting,
    partitions,
    totals,
    laterCount,
    archivedCount,
    groupFolds,
  });
}
