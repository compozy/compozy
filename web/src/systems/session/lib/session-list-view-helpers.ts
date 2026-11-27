import {
  workspaceBadgeIdentity,
  type DataScope,
  type WorkspaceBadgeIdentity,
} from "@/systems/workspace";

import { computerWorkspaceBadgeIdentity } from "./session-list-computer-filter";
import { sessionListPinnedExclusion } from "./session-list-pinned";
import {
  availableSessionListGroupBys,
  sessionListSubagentsParam,
  type SessionListGroupBy,
} from "./session-list-preferences";
import { sessionListShelfFilters, type SessionListShelf } from "./session-list-shelf";
import type { SessionFoldSet } from "./session-sidebar-scope";
import type { ProjectGroupTree } from "./workspace-folders";
import type { SessionListFilters, SessionPayload } from "../types";

export const SESSION_LIST_PAGE_SIZE = 100;

/** Shown in the toolbar while the stored `manual` sort cannot apply to what is listed. */
export const SESSION_LIST_MANUAL_SORT_NOTE =
  "Manual order applies inside one project and profile. Group the list to use it.";

/** Shown when multiple computers are visible (`O3`). */
export const SESSION_LIST_MANUAL_SORT_MULTI_COMPUTER_NOTE =
  "Manual order applies inside one project and profile on one computer.";

/** Shown on the disabled toolbar New folder button while multiple computers are visible (`Q-G`). */
export const SESSION_LIST_NEW_FOLDER_MULTI_COMPUTER_NOTE =
  "Folders live on one computer. Use the + of a computer or project.";

export function focusedScopeName(
  scope: DataScope | null,
  workspaces: readonly { id: string; name: string }[]
) {
  if (scope?.kind !== "project") return "Global";
  return workspaces.find(workspace => workspace.id === scope.workspaceId)?.name ?? "this project";
}

export function hiddenProjectGroupKeys(
  tree: ProjectGroupTree | undefined,
  folderFolds: SessionFoldSet
): string[] {
  if (!tree) return [];
  return tree.folders.flatMap(node =>
    folderFolds.has(node.folder.id) ? node.groups.map(group => group.key) : []
  );
}

export function sessionListGroupFiltersFor(
  search: string,
  worktreeId: string | undefined,
  shelf: SessionListShelf,
  showAutomation: boolean
): SessionListFilters {
  return {
    include_health: true,
    limit: SESSION_LIST_PAGE_SIZE,
    q: search,
    search_fields: "title_agent",
    subagents: sessionListSubagentsParam(search),
    worktree: worktreeId,
    ...sessionListShelfFilters(shelf, showAutomation),
    ...sessionListPinnedExclusion(shelf),
  };
}

/**
 * An open list still waiting on what it reads under (`pending`) reads as loading, never as empty:
 * the flat catalog, or the group facets of a grouped list, stand in for an answer not yet asked for.
 */
export function awaitingReads<C extends { loading: boolean }>(
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
export function settledSearchCatalog<
  C extends { paging: boolean; next: boolean; previous: boolean },
>(catalog: C, searchSettled: boolean): C {
  return {
    ...catalog,
    paging: catalog.paging || !searchSettled,
    next: searchSettled && catalog.next,
    previous: searchSettled && catalog.previous,
  };
}

export function manualSortNote(
  sort: string,
  manualBlocked: boolean,
  multiComputer = false
): string | null {
  if (sort !== "manual" || !manualBlocked) return null;
  return multiComputer ? SESSION_LIST_MANUAL_SORT_MULTI_COMPUTER_NOTE : SESSION_LIST_MANUAL_SORT_NOTE;
}

export function newFolderUnavailableReason(
  flat: boolean,
  paired: boolean,
  breadth: { allProjects: boolean; allProfiles: boolean },
  groupBy: SessionListGroupBy,
  multiComputer = false
): string | null {
  if (multiComputer) return SESSION_LIST_NEW_FOLDER_MULTI_COMPUTER_NOTE;
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

export function sessionBadgeResolver(
  workspaces: readonly { id: string; name: string }[],
  isRemote = false
): (session: SessionPayload) => WorkspaceBadgeIdentity {
  const names = new Map(workspaces.map(workspace => [workspace.id, workspace.name]));
  if (isRemote) {
    return (session: SessionPayload) =>
      computerWorkspaceBadgeIdentity(session.workspace_id, names, { isRemote: true });
  }
  return (session: SessionPayload) => workspaceBadgeIdentity(session.workspace_id, names);
}
