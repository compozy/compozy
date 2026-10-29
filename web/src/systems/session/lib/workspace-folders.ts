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
 * Adds a group for every placed project the facets left out because it has no
 * sessions in scope, so a folder still shows (and can move) every project the
 * operator put in it. Only registered projects and User qualify.
 */
export function withPlacedProjectGroups(
  groups: readonly SessionListGroupSpec[],
  placements: readonly WorkspacePlacementPayload[],
  input: {
    workspaces: ReadonlyArray<{ id: string; name: string }>;
    lens: SessionListGroupSpec["lens"];
  }
): SessionListGroupSpec[] {
  const shown = new Set(groups.flatMap(group => projectGroupWorkspaceId(group) ?? []));
  const registered = new Set(input.workspaces.map(workspace => workspace.id));
  const placed = placements.flatMap(row => {
    if (shown.has(row.workspace_id)) return [];
    if (row.workspace_id === USER_SCOPE_SELECTOR)
      return [projectSessionGroup("", EMPTY_GROUP_FACETS, input)];
    return registered.has(row.workspace_id)
      ? [projectSessionGroup(row.workspace_id, EMPTY_GROUP_FACETS, input)]
      : [];
  });
  return [...groups, ...placed];
}

/** Arranges served project groups (switcher order, User last) by the profile's folders. */
export function buildProjectGroupTree(
  groups: readonly SessionListGroupSpec[],
  layout: {
    folders: readonly WorkspaceFolderPayload[];
    placements: readonly WorkspacePlacementPayload[];
  }
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
  return {
    folders: [...layout.folders]
      .sort((left, right) => left.position - right.position)
      .map(folder => ({ folder, groups: byPosition(inFolder.get(folder.id) ?? []) })),
    root: [...byPosition(placedRoot), ...unplaced],
  };
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
