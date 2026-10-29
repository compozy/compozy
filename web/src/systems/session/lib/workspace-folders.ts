import { workspaceSelectorForScope, type FocusedDataScope } from "@/systems/workspace";

import type {
  WorkspaceFolderPayload,
  WorkspaceLayoutResponse,
  WorkspacePlacementPayload,
} from "../types";
import type { SessionListGroupSpec } from "./session-list-groups";

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

/**
 * Optimistic order: the listed projects take the container and the head of its
 * order; the container's other members keep their relative order after them.
 */
export function applyWorkspaceOrder(
  layout: WorkspaceLayoutResponse,
  folderId: string | null,
  workspaceIds: readonly string[]
): WorkspaceLayoutResponse {
  const listed = new Set(workspaceIds);
  const container = (row: WorkspacePlacementPayload) => (row.folder_id ?? null) === folderId;
  const others = layout.placements
    .filter(row => container(row) && !listed.has(row.workspace_id))
    .sort((left, right) => left.position - right.position);
  const order = [...workspaceIds, ...others.map(row => row.workspace_id)];
  const placed = order.map((workspace_id, position) => placement(workspace_id, folderId, position));
  return {
    ...layout,
    placements: [
      ...layout.placements.filter(row => !container(row) && !listed.has(row.workspace_id)),
      ...placed,
    ],
  };
}

/** Optimistic move: the project joins the end of a folder, or of the placed root. */
export function applyWorkspaceMove(
  layout: WorkspaceLayoutResponse,
  workspaceId: string,
  folderId: string | null
): WorkspaceLayoutResponse {
  const rest = layout.placements.filter(row => row.workspace_id !== workspaceId);
  const last = rest
    .filter(row => (row.folder_id ?? null) === folderId)
    .reduce((max, row) => Math.max(max, row.position), -1);
  return { ...layout, placements: [...rest, placement(workspaceId, folderId, last + 1)] };
}

/** Optimistic delete: the folder goes and its projects return to the root, unplaced. */
export function applyWorkspaceFolderRemoval(
  layout: WorkspaceLayoutResponse,
  folderId: string
): WorkspaceLayoutResponse {
  return {
    folders: layout.folders.filter(folder => folder.id !== folderId),
    placements: layout.placements.filter(row => row.folder_id !== folderId),
  };
}

/** Optimistic folder order: listed folders take positions in the listed order. */
export function applyWorkspaceFolderOrder(
  layout: WorkspaceLayoutResponse,
  folderIds: readonly string[]
): WorkspaceLayoutResponse {
  const byId = new Map(layout.folders.map(folder => [folder.id, folder]));
  return {
    ...layout,
    folders: folderIds.flatMap((id, position) => {
      const folder = byId.get(id);
      return folder ? [{ ...folder, position }] : [];
    }),
  };
}

function placement(
  workspaceId: string,
  folderId: string | null,
  position: number
): WorkspacePlacementPayload {
  return { workspace_id: workspaceId, position, ...(folderId ? { folder_id: folderId } : {}) };
}
