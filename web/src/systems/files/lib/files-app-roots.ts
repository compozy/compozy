import type { WorkspacePayload, WorktreePayload } from "@/systems/workspace";

/** One entry of the Files app's root picker: the project checkout or one ready worktree. */
export interface FilesAppRootOption {
  /** `null` names the project checkout. */
  worktreeId: string | null;
  label: string;
  branch: string | null;
}

export const FILES_PROJECT_ROOT_LABEL = "Project root";

/** The checkout first, then every worktree whose checkout is on disk, by name (US-002.AC-3). */
export function filesAppRootOptions(
  worktrees: readonly WorktreePayload[] | undefined
): FilesAppRootOption[] {
  const ready: FilesAppRootOption[] = [];
  for (const worktree of worktrees ?? []) {
    if (worktree.state !== "ready") continue;
    const branch = worktree.branch.trim();
    ready.push({ worktreeId: worktree.id, label: worktree.name, branch: branch || null });
  }
  ready.sort((left, right) => left.label.localeCompare(right.label));
  return [{ worktreeId: null, label: FILES_PROJECT_ROOT_LABEL, branch: null }, ...ready];
}

/** The project switcher lists live projects only; an archived one leaves it (US-002.EC-3). */
export function filesSwitcherProjects(
  workspaces: readonly WorkspacePayload[] | undefined
): WorkspacePayload[] {
  return (workspaces ?? []).filter(workspace => !workspace.archived_at);
}

export type FilesAppProjectStatus = "loading" | "ready" | "archived" | "missing";

/** Where the window's project stands in the catalog: live, archived, or gone. */
export function filesAppProjectStatus(
  workspaceId: string,
  catalog: {
    active: readonly WorkspacePayload[] | undefined;
    archived: readonly WorkspacePayload[] | undefined;
  }
): FilesAppProjectStatus {
  if (catalog.active === undefined) return "loading";
  const live = catalog.active.find(workspace => workspace.id === workspaceId);
  if (live) return live.archived_at ? "archived" : "ready";
  if (catalog.archived === undefined) return "loading";
  return catalog.archived.some(workspace => workspace.id === workspaceId) ? "archived" : "missing";
}
