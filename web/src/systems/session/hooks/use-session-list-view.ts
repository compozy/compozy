import { useState } from "react";

import { useDebouncedInput } from "@/hooks/use-debounced-input";

import { useProfileReadScope, type ProfileOwner, type ProfileOwnerLabel } from "@/systems/profiles";
import {
  useActiveWorkspace,
  workspaceBadgeIdentity,
  type DataScope,
  type WorkspaceBadgeIdentity,
} from "@/systems/workspace";

import type { SessionListGroupSpec } from "../lib/session-list-groups";
import { sessionListPair } from "../lib/session-list-groups";
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
import { useSessionListGroups } from "./use-session-list-groups";
import { useSessionListPreferences } from "./use-session-list-preferences";
import { useSessionSidebarState } from "./use-session-sidebar-state";

/** Shown in the toolbar while the stored `manual` sort cannot apply to what is listed. */
export const SESSION_LIST_MANUAL_SORT_NOTE = "Manual order applies inside one project";

const SESSION_LIST_PAGE_SIZE = 100;

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
  /** True while the list shows the archive instead of the active catalog. */
  archived: boolean;
  saving: boolean;
  setSort: (sort: SessionListSort) => void;
  setGroupBy: (groupBy: SessionListGroupBy) => void;
  setArchived: (archived: boolean) => void;
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
  /** Resolves a row's owner tag. Only called while `aggregate` is true. */
  ownerOf: (session: ProfileOwnerLabel) => ProfileOwner;
  /** Resolves a row's project badge; present only under All projects (BR-10). */
  badgeOf?: (session: SessionPayload) => WorkspaceBadgeIdentity;
  /** Folders and manual order of the one (workspace, profile) pair the flat list reads. */
  layout?: SessionListLayout;
  /** The worktree the catalog is narrowed to; other worktrees' sessions are hidden. */
  worktreeId?: string;
  /** The folder list failed to load; rows stay flat and unorderable until a retry loads it. */
  layoutError?: { retry: () => void };
  /** Identity of the list's bulk selection; grouped lists are read-only. */
  selectionKey: string;
}

/**
 * View model behind every session list: the topbar breadth and profile lens,
 * the operator's persisted order and grouping, and either one flat catalog or
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
 * The archive stays deliberately transient — a way of looking at the list
 * right now, not a preference worth keeping.
 */
export function useSessionListView(
  options: {
    /** The list's data scope; omitted follows the menubar destination. */
    scope?: DataScope | null;
    worktreeId?: string;
    enabled?: boolean;
  } = {}
): SessionListViewModel {
  const preferences = useSessionListPreferences();
  const { registeredWorkspaces: workspaces, dataScope } = useActiveWorkspace();
  const profile = useProfileReadScope();
  const sidebar = useSessionSidebarState();
  const {
    draftValue: search,
    committedValue: remoteSearch,
    setDraftValue: setSearch,
  } = useDebouncedInput({ externalValue: "", onCommit: () => undefined });
  const [archived, setArchived] = useState(false);
  const enabled = options.enabled ?? true;
  const searchSettled = search === remoteSearch;
  const scope = options.scope === undefined ? dataScope : options.scope;
  const breadth = { allProjects: scope?.kind === "all", allProfiles: profile.aggregate };
  const groupBy = effectiveSessionListGroupBy(preferences.groupBy, breadth);
  const flat = groupBy === "none";
  const pair = sessionListPair(scope, profile.params);

  const groupFilters = sessionListGroupFiltersFor(remoteSearch, options.worktreeId, archived);
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
    archived,
    search: remoteSearch,
    enabled,
  });
  const folders = useSessionFolders({
    workspaceId: pair?.workspaceId ?? null,
    profile: pair?.profile ?? profile.destination,
    enabled: enabled && flat && pair !== null,
  });
  const selectionKey = JSON.stringify([scope, profile.key, groupBy]);
  const groupId = (key: string) => sessionListGroupCollapseId(selectionKey, key);
  const collapsedGroupIds = new Set(sidebar.collapsedGroupIds);
  const manualBlocked = flat ? pair === null : grouping.groups.some(group => group.pair === null);
  const flatLayout = flat && pair !== null;

  return {
    layout: flatLayout ? folders.layout : undefined,
    layoutError: flatLayout ? folders.loadError : undefined,
    worktreeId: options.worktreeId,
    catalog: flat ? settledSearchCatalog(catalog, searchSettled) : undefined,
    search,
    setSearch,
    sort: preferences.sort,
    storedGroupBy: preferences.groupBy,
    groupBy,
    groupByOptions: availableSessionListGroupBys(breadth),
    sortNote: manualSortNote(preferences.sort, manualBlocked),
    archived,
    saving: preferences.saving,
    setSort: preferences.setSort,
    setGroupBy: preferences.setGroupBy,
    setArchived,
    groups: grouping.groups,
    groupsLoading: grouping.loading,
    groupsError: grouping.error,
    collapsedGroupKeys: new Set(
      grouping.groups.flatMap(group =>
        collapsedGroupIds.has(groupId(group.key)) ? [group.key] : []
      )
    ),
    toggleGroup: key => sidebar.toggleGroup(groupId(key)),
    groupFilters,
    aggregate: profile.aggregate,
    scopeLabel: profile.scopeLabel,
    ownerOf: profile.ownerOf,
    badgeOf: breadth.allProjects ? sessionBadgeResolver(workspaces) : undefined,
    selectionKey,
  };
}

function sessionListGroupFiltersFor(
  search: string,
  worktreeId: string | undefined,
  archived: boolean
): SessionListFilters {
  return {
    include_health: true,
    limit: SESSION_LIST_PAGE_SIZE,
    q: search,
    search_fields: "title_agent",
    subagents: sessionListSubagentsParam(search),
    worktree: worktreeId,
    ...(archived ? { archive: "only" as const } : {}),
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

/** A group's collapse id: its key qualified by the list lens it renders under. */
function sessionListGroupCollapseId(selectionKey: string, groupKey: string): string {
  return `${selectionKey}\u0000${groupKey}`;
}

function sessionBadgeResolver(workspaces: readonly { id: string; name: string }[]) {
  const names = new Map(workspaces.map(workspace => [workspace.id, workspace.name]));
  return (session: SessionPayload) => workspaceBadgeIdentity(session.workspace_id, names);
}
