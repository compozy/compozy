import { useEffect, useState } from "react";

import { useDebouncedInput } from "@/hooks/use-debounced-input";

import { useProfileReadScope, type ProfileOwner, type ProfileOwnerLabel } from "@/systems/profiles";
import {
  useActiveWorkspace,
  workspaceBadgeIdentity,
  workspaceSelectorOrAll,
  type DataScope,
  type WorkspaceBadgeIdentity,
} from "@/systems/workspace";

import type { SessionListGroupSpec } from "../lib/session-list-groups";
import { sessionListPair } from "../lib/session-list-groups";
import { sessionListShelfFilters, type SessionListShelf } from "../lib/session-list-shelf";
import type { ProjectGroupTree } from "../lib/workspace-folders";
import {
  availableSessionListGroupBys,
  effectiveSessionListGroupBy,
  sessionListSortParam,
  sessionListSubagentsParam,
  type SessionListGroupBy,
  type SessionListSort,
} from "../lib/session-list-preferences";
import type { SessionListFilters, SessionPayload } from "../types";

import { useSessionCatalog } from "./use-session-catalog";
import { useSessionFolders, type SessionListLayout } from "./use-session-folders";
import {
  useSessionListGroups,
  useSessionListLaterCount,
  useSessionListTotals,
  type SessionListTotals,
} from "./use-session-list-groups";
import { sessionSidebarStore, useSessionSidebarState } from "./use-session-sidebar-state";
import { useSessionWindowOrder } from "./use-session-window-order";
import { useProjectFolderGroups, type WorkspaceLayout } from "./use-workspace-layout";

/** Shown in the toolbar while the stored `manual` sort cannot apply to what is listed. */
export const SESSION_LIST_MANUAL_SORT_NOTE =
  "Manual order applies inside one project and profile. Group the list to use it.";

const SESSION_LIST_PAGE_SIZE = 100;

export type { SessionListTotals } from "./use-session-list-groups";

export interface SessionListViewModel {
  /** The flat catalog; absent while the list is grouped (each group owns its page). */
  catalog?: ReturnType<typeof useSessionCatalog>;
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
  setSort: (sort: SessionListSort) => void;
  setGroupBy: (groupBy: SessionListGroupBy) => void;
  /** Project grouping also lists projects with no sessions in view (#1085); per window. */
  showEmptyProjects: boolean;
  setShowEmptyProjects: (show: boolean) => void;
  setShelf: (shelf: SessionListShelf) => void;
  /** Group-by groups from server facets; empty while flat. */
  groups: SessionListGroupSpec[];
  groupsLoading: boolean;
  groupsError?: { retry: () => void };
  collapsedGroupKeys: ReadonlySet<string>;
  toggleGroup: (key: string) => void;
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
  focusedScopeName: string;
  /** Resolves a row's owner tag. Only called while `aggregate` is true. */
  ownerOf: (session: ProfileOwnerLabel) => ProfileOwner;
  /** Resolves a row's project badge; present only under All projects (BR-10). */
  badgeOf?: (session: SessionPayload) => WorkspaceBadgeIdentity;
  /** Folders and manual order of the one (workspace, profile) pair the flat list reads. */
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
}

/**
 * View model behind every session list: the topbar breadth and profile lens,
 * the window's order and grouping (shared with its Kanban), and either one flat catalog or
 * facets-driven groups (ADR-004).
 *
 * Folders and manual order apply only where a query is exactly one
 * (workspace, profile) pair (BR-11): the flat list of a focused project or User
 * under one profile, or a group whose own query is one pair.
 *
 * Group collapse is rail state, like thread and folder collapse: it lives in
 * the window's persisted sessions-rail entry (a new window starts from the
 * last-used one), keyed per list lens (scope, profile, grouping). Selecting a
 * session remounts the window's content, so state held here would reset; held
 * in the rail, a selection, a focus change, or a reload never reopens a group
 * the operator closed. Only the operator's toggle opens or closes one.
 * The archive and Later stay deliberately transient — ways of looking at the
 * list right now, not preferences worth keeping.
 */
export function useSessionListView(
  options: {
    /** The list's data scope; omitted follows the menubar destination. */
    scope?: DataScope | null;
    worktreeId?: string;
    enabled?: boolean;
    /**
     * The list is open but still waiting on what it reads under (the destination or its
     * worktree filter): it reads as loading, not as an empty list, until they settle.
     */
    pending?: boolean;
  } = {}
): SessionListViewModel {
  const preferences = useSessionWindowOrder();
  const { registeredWorkspaces: workspaces, dataScope } = useActiveWorkspace();
  const profile = useProfileReadScope();
  const sidebar = useSessionSidebarState();
  const {
    draftValue: search,
    committedValue: remoteSearch,
    setDraftValue: setSearch,
  } = useDebouncedInput({ externalValue: "", onCommit: () => undefined });
  const [shelf, setShelf] = useState<SessionListShelf>("active");
  const enabled = options.enabled ?? true;
  const searchSettled = search === remoteSearch;
  const scope = options.scope === undefined ? dataScope : options.scope;
  const breadth = { allProjects: scope?.kind === "all", allProfiles: profile.aggregate };
  const groupBy = effectiveSessionListGroupBy(preferences.groupBy, breadth);
  const flat = groupBy === "none";
  const pair = sessionListPair(scope, profile.params);

  const groupFilters = sessionListGroupFiltersFor(remoteSearch, options.worktreeId, shelf);
  const catalog = useSessionCatalog(
    scope,
    { ...groupFilters, sort: sessionListSortParam(preferences.sort, pair !== null) },
    enabled && flat
  );
  const grouping = useSessionListGroups({
    groupBy,
    scope,
    lens: profile.params,
    workspaces,
    shelf,
    search: remoteSearch,
    worktreeId: options.worktreeId,
    enabled,
  });
  const arranged = useProjectFolderGroups({
    groups: grouping.groups,
    groupBy,
    breadth,
    lens: profile.params,
    workspaces,
    showEmptyProjects: sidebar.showEmptyProjects,
    enabled,
  });
  const groups = arranged.groups;
  const folders = useSessionFolders({
    workspaceId: pair?.workspaceId ?? null,
    profile: pair?.profile ?? profile.destination,
    enabled: enabled && flat && pair !== null,
  });
  const totals = useSessionListTotals({
    scope,
    lens: profile.params,
    search: remoteSearch,
    worktreeId: options.worktreeId,
    shelf,
    enabled,
  });
  const laterCount = useSessionListLaterCount({
    scope,
    lens: profile.params,
    worktreeId: options.worktreeId,
    enabled,
  });
  const awaiting = awaitingReads(
    flat ? settledSearchCatalog(catalog, searchSettled) : undefined,
    grouping.loading,
    options.pending === true
  );
  const selectionKey = JSON.stringify([scope, profile.key, groupBy]);
  const foldLens = useSessionListFoldLens(scope, profile, groupBy);
  const groupId = (key: string) => sessionListGroupCollapseId(foldLens, key);
  const collapsedGroupIds = new Set(sidebar.collapsedGroupIds);
  const collapsedGroupKeys = groups.flatMap(group =>
    collapsedGroupIds.has(groupId(group.key)) ? [group.key] : []
  );
  // A group with no pair of its own splits along the other axis into pairs,
  // so only a flat aggregate leaves manual order and folders with no owner.
  const manualBlocked = flat && pair === null;
  const flatLayout = flat && pair !== null;

  return {
    ...(flatLayout
      ? { layout: folders.layout, layoutError: folders.loadError }
      : { layout: undefined, layoutError: arranged.loadError }),
    projectFolders: arranged.projectFolders,
    worktreeId: options.worktreeId,
    catalog: awaiting.catalog,
    search,
    setSearch,
    sort: preferences.sort,
    storedGroupBy: preferences.groupBy,
    groupBy,
    groupByOptions: availableSessionListGroupBys(breadth),
    sortNote: manualSortNote(preferences.sort, manualBlocked),
    newFolderUnavailable: newFolderUnavailableReason(flat, pair !== null, breadth, groupBy),
    shelf,
    laterCount,
    setSort: preferences.setSort,
    setGroupBy: preferences.setGroupBy,
    showEmptyProjects: sidebar.showEmptyProjects,
    setShowEmptyProjects: sidebar.setShowEmptyProjects,
    setShelf,
    groups,
    groupsLoading: awaiting.groupsLoading,
    groupsError: grouping.error,
    collapsedGroupKeys: new Set([
      ...collapsedGroupKeys,
      // A collapsed project folder hides its projects: navigation skips them too.
      ...hiddenProjectGroupKeys(arranged.projectFolders?.tree, sidebar.collapsedFolderIds),
    ]),
    toggleGroup: key => sidebar.toggleGroup(groupId(key)),
    groupFilters,
    aggregate: profile.aggregate,
    scopeLabel: profile.scopeLabel,
    focusedWorkspaceId: scope ? workspaceSelectorOrAll(scope) : null,
    focusedScopeName: focusedScopeName(scope, workspaces),
    ownerOf: profile.ownerOf,
    badgeOf: breadth.allProjects ? sessionBadgeResolver(workspaces) : undefined,
    selectionKey,
    totals,
  };
}

/**
 * The lens fold state persists under. It settles on the profile's immutable id once the profile
 * list resolves, carrying folds kept under the name that stood in for it.
 */
function useSessionListFoldLens(
  scope: DataScope | null,
  profile: ReturnType<typeof useProfileReadScope>,
  groupBy: SessionListGroupBy
) {
  const foldLens = JSON.stringify([scope, profileFoldIdentity(profile), groupBy]);
  const fallbackLens = JSON.stringify([scope, `name:${profile.key}`, groupBy]);
  const resolvedProfile = !profile.aggregate && profile.destinationOwner !== null;
  useEffect(() => {
    if (resolvedProfile) {
      sessionSidebarStore.trigger.groupLensResolved({ from: fallbackLens, to: foldLens });
    }
  }, [resolvedProfile, fallbackLens, foldLens]);
  return foldLens;
}

function focusedScopeName(
  scope: DataScope | null,
  workspaces: readonly { id: string; name: string }[]
) {
  if (scope?.kind !== "project") return "Global";
  return workspaces.find(workspace => workspace.id === scope.workspaceId)?.name ?? "this project";
}

function hiddenProjectGroupKeys(
  tree: ProjectGroupTree | undefined,
  collapsedFolderIds: readonly string[]
): string[] {
  if (!tree) return [];
  const collapsed = new Set(collapsedFolderIds);
  return tree.folders.flatMap(node =>
    collapsed.has(node.folder.id) ? node.groups.map(group => group.key) : []
  );
}

function sessionListGroupFiltersFor(
  search: string,
  worktreeId: string | undefined,
  shelf: SessionListShelf
): SessionListFilters {
  return {
    include_health: true,
    limit: SESSION_LIST_PAGE_SIZE,
    q: search,
    search_fields: "title_agent",
    subagents: sessionListSubagentsParam(search),
    worktree: worktreeId,
    ...sessionListShelfFilters(shelf),
  };
}

/**
 * An open list still waiting on what it reads under (`pending`) reads as loading, never as empty:
 * the flat catalog, or the group facets of a grouped list, stand in for an answer not yet asked for.
 */
function awaitingReads<C extends { loading: boolean }>(
  catalog: C | undefined,
  groupsLoading: boolean,
  pending: boolean
): { catalog: C | undefined; groupsLoading: boolean } {
  return {
    catalog: catalog ? { ...catalog, loading: catalog.loading || pending } : undefined,
    groupsLoading: groupsLoading || (pending && catalog === undefined),
  };
}

/** While a search is still debouncing, the page is pending and paging waits for it. */
function settledSearchCatalog<C extends { paging: boolean; next: boolean; previous: boolean }>(
  catalog: C,
  searchSettled: boolean
): C {
  return {
    ...catalog,
    paging: catalog.paging || !searchSettled,
    next: searchSettled && catalog.next,
    previous: searchSettled && catalog.previous,
  };
}

function manualSortNote(sort: string, manualBlocked: boolean): string | null {
  return sort === "manual" && manualBlocked ? SESSION_LIST_MANUAL_SORT_NOTE : null;
}

/**
 * The profile half of a persisted fold's lens. Folds outlive the page, so they
 * name the profile by its immutable id: a rename keeps them, and a later
 * profile that reuses the old name does not inherit them. Until the profile
 * list resolves the id, the name stands in.
 */
function profileFoldIdentity(profile: {
  aggregate: boolean;
  key: string;
  destinationOwner: { id: string } | null;
}): string {
  if (profile.aggregate) return profile.key;
  return profile.destinationOwner?.id ?? `name:${profile.key}`;
}

/** A group's collapse id: its key qualified by the list lens it renders under. */
function sessionListGroupCollapseId(lens: string, groupKey: string): string {
  return `${lens}\u0000${groupKey}`;
}

function newFolderUnavailableReason(
  flat: boolean,
  paired: boolean,
  breadth: { allProjects: boolean; allProfiles: boolean },
  groupBy: SessionListGroupBy
): string | null {
  if (groupBy === "project" && breadth.allProjects) {
    return breadth.allProfiles
      ? "Project folders belong to one profile. Choose a profile to add one."
      : null;
  }
  if (!flat) return "Add a folder from a group's +.";
  if (paired) return null;
  const axes = availableSessionListGroupBys(breadth).filter(option => option !== "none");
  return axes.length === 0
    ? "Folders live inside one project and profile."
    : `Folders live inside one project and profile. Group by ${axes.join(" or ")} to add one.`;
}

function sessionBadgeResolver(workspaces: readonly { id: string; name: string }[]) {
  const names = new Map(workspaces.map(workspace => [workspace.id, workspace.name]));
  return (session: SessionPayload) => workspaceBadgeIdentity(session.workspace_id, names);
}
