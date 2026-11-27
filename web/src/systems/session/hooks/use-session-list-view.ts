import { useDebouncedInput } from "@/hooks/use-debounced-input";
import { isRemoteComputerId, LOCAL_COMPUTER_ID, type ComputerId } from "@/lib/computer-scope";

import {
  useProfileReadScope,
  type ProfileOwner,
  type ProfileOwnerLabel,
  type ProfileScopeParams,
} from "@/systems/profiles";
import {
  useActiveWorkspace,
  workspaceSelectorOrAll,
  type DataScope,
  type WorkspaceBadgeIdentity,
} from "@/systems/workspace";

import type { SessionListGroupSpec } from "../lib/session-list-groups";
import { sessionListPair } from "../lib/session-list-groups";
import type { SessionListShelf } from "../lib/session-list-shelf";
import type { ProjectGroupTree } from "../lib/workspace-folders";
import {
  availableSessionListGroupBys,
  effectiveSessionListGroupBy,
  type SessionListComputerLayout,
  type SessionListGroupBy,
  type SessionListSort,
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
import type { SessionWindowComputerScope } from "../lib/session-sidebar-scope";
import type { SessionListFilters, SessionPayload } from "../types";

import { useSessionCatalog } from "./use-session-catalog";
import type { SessionListPartitions } from "./use-session-list-partitions";
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
import {
  useSessionSidebarState,
  type SessionFoldAll,
  type SessionFoldSet,
} from "./use-session-sidebar-state";
import { useSessionWindowOrder } from "./use-session-window-order";
import { useProjectFolderGroups, type WorkspaceLayout } from "./use-workspace-layout";

export { SESSION_LIST_MANUAL_SORT_NOTE };
export type { SessionListTotals } from "./use-session-list-groups";
export type { SessionListPinnedRead } from "./use-session-list-pinned-read";

export interface SessionListViewModel {
  /** Owning computer for this view (`"local"` or a remote computer id). */
  computerId?: ComputerId;
  /** True when the computer is offline or lacks `sessions:operate` (`F10`, `F11`). */
  readOnly?: boolean;
  /** Workspaces catalog for this computer (used by split subgroups and badges). */
  workspaces?: ReadonlyArray<{ id: string; name: string }>;
  /** Matched workspace ids on a remote computer when topbar focuses one project (`Q5`). */
  matchedWorkspaceIds?: readonly string[] | null;
  /** Active per-window Computers layer (`together` | `separate`). */
  computers?: SessionListComputerLayout;
  setComputers?: (computers: SessionListComputerLayout) => void;
  /** Active per-window computer scope (`all` | `local` | `<computerId>`). */
  computerScope?: SessionWindowComputerScope;
  setComputerScope?: (scope: SessionWindowComputerScope) => void;
  /** The flat catalog; absent while the list is grouped (each group owns its page). */
  catalog?: ReturnType<typeof useSessionCatalog>;
  partitions?: SessionListPartitions;
  folderOf?: ReadonlyMap<string, string | null>;
  search?: string;
  setSearch?: (search: string) => void;
  /** The stored sort, kept even where `manual` cannot apply. */
  sort: SessionListSort;
  /** The stored grouping; `groupBy` below is what renders. */
  storedGroupBy: SessionListGroupBy;
  /** The grouping that renders: an inapplicable stored value renders as `none`. */
  groupBy: SessionListGroupBy;
  /** Group-by options available in the current breadth and profile lens. */
  groupByOptions: readonly SessionListGroupBy[];
  /** Muted toolbar note while `manual` falls back to `last_activity`. */
  sortNote: string | null;
  /**
   * Why the list's New folder is unavailable: a flat aggregate spans pairs, and
   * a grouped list adds folders from each group's own `+`. `null` where it applies:
   * one pair's session folders, or the project folders of All projects by project.
   */
  newFolderUnavailable: string | null;
  /** What the list shows: the active catalog, the archive, or the sessions in Later. */
  shelf: SessionListShelf;
  /** Sessions parked in Later across the list's scope; `undefined` until counted. */
  laterCount?: number;
  /** Archived sessions across the list's scope, counted on the archive shelf only. */
  archivedCount?: number;
  setSort: (sort: SessionListSort) => void;
  setGroupBy: (groupBy: SessionListGroupBy) => void;
  /** Project grouping also lists projects with no sessions in view (#1085); per window. */
  showEmptyProjects: boolean;
  setShowEmptyProjects: (show: boolean) => void;
  /** "Show automation runs" (#1355): automation job and trigger sessions join the list. */
  showAutomation: boolean;
  setShowAutomation: (show: boolean) => void;
  setShelf: (shelf: SessionListShelf) => void;
  /** Group-by groups from server facets; empty while flat. */
  groups: SessionListGroupSpec[];
  groupsLoading: boolean;
  groupsError?: { retry: () => void };
  collapsedGroupKeys: SessionFoldSet;
  toggleGroup: (key: string) => void;
  /** The last Collapse all or Expand all; shared with this window's Kanban. */
  foldAll: SessionFoldAll;
  setFoldAll: (foldAll: Exclude<SessionFoldAll, null>) => void;
  /** Threads opened one by one, which stay open under Collapse all. */
  openThreadIds: readonly string[];
  /** Filters every group query shares; each group adds its scope, lens, and order. */
  groupFilters: SessionListFilters;
  /** The aggregate is on — rows name their owner (US-011.AC-1). */
  aggregate: boolean;
  /**
   * The profile the list is bounded by, or `null` under the aggregate. Named by
   * the empty state; the create target is deliberately not used there.
   */
  scopeLabel: string | null;
  /** The list's one concrete scope: a workspace id or `@user`; `null` under All projects. */
  focusedWorkspaceId: string | null;
  /** The list spans every project (and User scope). */
  allProjects?: boolean;
  focusedScopeName: string;
  /** Resolves a row's owner tag. Only called while `aggregate` is true. */
  ownerOf: (session: ProfileOwnerLabel) => ProfileOwner;
  /** Resolves a row's project badge; present only under All projects (BR-10). */
  badgeOf?: (session: SessionPayload) => WorkspaceBadgeIdentity;
  /** Folders and manual order of the one (computer, workspace, profile) pair the flat list reads. */
  layout?: SessionListLayout;
  /**
   * Project folders of the one profile an All projects list grouped by project
   * reads (issue #1044): the layout verbs and the groups arranged into folders.
   */
  projectFolders?: { layout: WorkspaceLayout; tree: ProjectGroupTree };
  /** The worktree the catalog is narrowed to; other worktrees' sessions are hidden. */
  worktreeId?: string;
  /** The folder list failed to load; rows stay flat and unorderable until a retry loads it. */
  layoutError?: { retry: () => void };
  /** Identity of the list's bulk selection, shared by every group of a grouped list. */
  selectionKey: string;
  /** Exact live-state totals of the list's whole scope, from facets; absent in the archive. */
  totals?: SessionListTotals;
  /** The Pinned section's read, above every group and folder; absent off the active shelf. */
  pinned?: SessionListPinnedRead;
}

export interface UseSessionListViewOptions {
  /** The list's data scope; omitted follows the menubar destination. */
  scope?: DataScope | null;
  worktreeId?: string;
  enabled?: boolean;
  /**
   * The list is open but still waiting on what it reads under (the destination or its
   * worktree filter): it reads as loading, not as an empty list, until they settle.
   */
  pending?: boolean;
  /** Target computer for this view (`F5`); omitted reads the local daemon. */
  computerId?: ComputerId;
  /** Workspaces catalog for a remote computer (`P1`). */
  workspaces?: ReadonlyArray<{ id: string; name: string }>;
  /** Matched workspace ids on a remote computer when topbar focuses one project (`Q5`). */
  matchedWorkspaceIds?: readonly string[] | null;
  /** True when the computer is offline or lacks `sessions:operate`. */
  readOnly?: boolean;
  /** Controlled search string from a parent multi-computer container (`F7`, `F12`). */
  search?: string;
  /** Whether the window currently shows multiple computers (`O3`, `Q-G`). */
  multiComputer?: boolean;
  /** Skip the Pinned read when a parent container renders one combined Pinned section (`F12`). */
  includePinned?: boolean;
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
  const searchSettled = search === remoteSearch;
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
  const groups = arranged.groups;
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
    flat ? settledSearchCatalog(catalog, searchSettled) : undefined,
    grouping.loading,
    pending
  );
  const selectionKey = JSON.stringify([
    preferences.computerScope,
    preferences.computers,
    scope,
    profile.key,
    groupBy,
  ]);
  const groupFolds = useSessionGroupFolds(scope, groupBy, remote ? computerId : undefined);
  const searching = remoteSearch.trim() !== "";
  const hiddenGroupKeys = new Set(
    hiddenProjectGroupKeys(arranged.projectFolders?.tree, sidebar.folderFolds)
  );
  const manualBlocked = flat && pair === null;
  const flatLayout = flat && pair !== null;

  return {
    ...(remote ? { computerId } : {}),
    ...(readOnly ? { readOnly: true } : {}),
    workspaces,
    matchedWorkspaceIds: options.matchedWorkspaceIds,
    computers: preferences.computers,
    setComputers: preferences.setComputers,
    computerScope: preferences.computerScope,
    setComputerScope: preferences.setComputerScope,
    ...(flatLayout
      ? { layout: readOnly ? undefined : folders.layout, layoutError: folders.loadError }
      : {
          layout: undefined,
          layoutError: arranged.loadError,
        }),
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
    shelf,
    laterCount,
    archivedCount,
    setSort: preferences.setSort,
    setGroupBy: preferences.setGroupBy,
    showEmptyProjects: sidebar.showEmptyProjects,
    setShowEmptyProjects: sidebar.setShowEmptyProjects,
    showAutomation,
    setShowAutomation: sidebar.setShowAutomation,
    setShelf: sidebar.setShelf,
    groups,
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
