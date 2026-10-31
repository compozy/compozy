import {
  USER_SCOPE_SELECTOR,
  workspaceSelectorForScope,
  type FocusedDataScope,
} from "@/systems/workspace";

import type { WorkspaceFolderPayload, WorkspacePlacementPayload } from "../types";
import { projectSessionGroup, type SessionListGroupSpec } from "./session-list-groups";

/** One project folder and the project groups it holds, in its manual order. */
export interface WorkspaceFolderNode {
  folder: WorkspaceFolderPayload;
  groups: SessionListGroupSpec[];
}

/**
 * Project groups arranged by one profile's project folders: folders first, in
 * their order, then the root — placed projects in their manual order, then the
 * projects never placed, in the order they were served (User last).
 */
export interface ProjectGroupTree {
  folders: WorkspaceFolderNode[];
  root: SessionListGroupSpec[];
}

/** The workspace selector a project group stands for: its project id, or `@user`. */
export function projectGroupWorkspaceId(group: Pick<SessionListGroupSpec, "scope">): string | null {
  const { scope } = group;
  if (scope.kind === "all") return null;
  return workspaceSelectorForScope(scope as FocusedDataScope);
}

const EMPTY_GROUP_FACETS = { all: 0, working: 0, needs_you: 0 };

/**
 * The project groups a view shows (#1085). By default only projects with
 * sessions in the current view (shelf, search, worktree) show. When the
 * operator shows empty projects, every registered project joins at zero — and
 * User too once placed in the profile's folders — ranked in switcher order with
 * User last, so a folder still holds every project the operator put in it.
 */
export function projectGroupsForView(
  groups: readonly SessionListGroupSpec[],
  input: {
    showEmpty: boolean;
    workspaces: ReadonlyArray<{ id: string; name: string }>;
    lens: SessionListGroupSpec["lens"];
    placements?: readonly WorkspacePlacementPayload[];
  }
): SessionListGroupSpec[] {
  if (!input.showEmpty) return groups.filter(group => group.total > 0);
  const shown = new Set(groups.flatMap(group => projectGroupWorkspaceId(group) ?? []));
  const added = input.workspaces.flatMap(workspace =>
    shown.has(workspace.id) ? [] : [projectSessionGroup(workspace.id, EMPTY_GROUP_FACETS, input)]
  );
  const userPlaced = (input.placements ?? []).some(row => row.workspace_id === USER_SCOPE_SELECTOR);
  if (userPlaced && !shown.has(USER_SCOPE_SELECTOR)) {
    added.push(projectSessionGroup("", EMPTY_GROUP_FACETS, input));
  }
  const order = new Map(input.workspaces.map((workspace, index) => [workspace.id, index]));
  const rank = (group: SessionListGroupSpec) => {
    const id = projectGroupWorkspaceId(group);
    if (id === USER_SCOPE_SELECTOR) return Number.MAX_SAFE_INTEGER;
    return (id === null ? undefined : order.get(id)) ?? order.size;
  };
  return [...groups, ...added]
    .map((group, index) => ({ group, index }))
    .sort((left, right) => rank(left.group) - rank(right.group) || left.index - right.index)
    .map(entry => entry.group);
}

/** Arranges served project groups (switcher order, User last) by the profile's folders. */
export function buildProjectGroupTree(
  groups: readonly SessionListGroupSpec[],
  layout: {
    folders: readonly WorkspaceFolderPayload[];
    placements: readonly WorkspacePlacementPayload[];
  },
  options: { hideEmptyFolders?: boolean } = {}
): ProjectGroupTree {
  const placements = new Map(layout.placements.map(row => [row.workspace_id, row]));
  const folderIds = new Set(layout.folders.map(folder => folder.id));
  const placementOf = (group: SessionListGroupSpec): WorkspacePlacementPayload | undefined => {
    const id = projectGroupWorkspaceId(group);
    const placement = id === null ? undefined : placements.get(id);
    // A placement into a folder the read no longer lists falls back to the root.
    if (placement?.folder_id && !folderIds.has(placement.folder_id)) return undefined;
    return placement;
  };
  const byPosition = (members: SessionListGroupSpec[]) =>
    members
      .map((group, index) => ({ group, index, position: placementOf(group)?.position ?? 0 }))
      .sort((left, right) => left.position - right.position || left.index - right.index)
      .map(entry => entry.group);

  const inFolder = new Map<string, SessionListGroupSpec[]>();
  const placedRoot: SessionListGroupSpec[] = [];
  const unplaced: SessionListGroupSpec[] = [];
  for (const group of groups) {
    const placement = placementOf(group);
    if (!placement) unplaced.push(group);
    else if (placement.folder_id) {
      const members = inFolder.get(placement.folder_id) ?? [];
      members.push(group);
      inFolder.set(placement.folder_id, members);
    } else placedRoot.push(group);
  }
  // A folder whose projects all have no sessions in view hides with them (#1085);
  // a folder holding no project at all stays, so a new folder can take a drop.
  const holdsProjects = new Set(
    layout.placements.flatMap(row => (row.folder_id ? [row.folder_id] : []))
  );
  return {
    folders: [...layout.folders]
      .sort((left, right) => left.position - right.position)
      .flatMap(folder => {
        const members = byPosition(inFolder.get(folder.id) ?? []);
        const hidden =
          options.hideEmptyFolders && members.length === 0 && holdsProjects.has(folder.id);
        return hidden ? [] : [{ folder, groups: members }];
      }),
    root: [...byPosition(placedRoot), ...unplaced],
  };
}

/**
 * A full folder order from a reorder of the folders on screen: hidden folders
 * keep their slots and the shown ones fill the rest in their new order, since
 * the daemon takes only a complete folder order.
 */
export function mergeShownFolderOrder(
  allIds: readonly string[],
  shownOrder: readonly string[]
): string[] {
  const shown = new Set(shownOrder);
  let next = 0;
  return allIds.map(id => (shown.has(id) ? (shownOrder[next++] ?? id) : id));
}

/** Every project group of the tree in on-screen order, folders first. */
export function projectGroupTreeGroups(tree: ProjectGroupTree): SessionListGroupSpec[] {
  return [...tree.folders.flatMap(node => node.groups), ...tree.root];
}

/** The workspace ids one container shows, in order; `folderId` null is the root. */
export function projectGroupContainerIds(
  tree: ProjectGroupTree,
  folderId: string | null
): string[] {
  const groups =
    folderId === null
      ? tree.root
      : (tree.folders.find(node => node.folder.id === folderId)?.groups ?? []);
  return groups.flatMap(group => {
    const id = projectGroupWorkspaceId(group);
    return id === null ? [] : [id];
  });
}
