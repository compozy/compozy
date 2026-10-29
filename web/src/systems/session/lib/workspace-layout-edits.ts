import type { WorkspaceLayoutResponse, WorkspacePlacementPayload } from "../types";

// Pure edits of one profile's project folder layout, shared by the optimistic
// cache updates and the MSW mock. Kept free of system barrels so mocks can load
// it during test setup without pulling the workspace and profile systems.

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
