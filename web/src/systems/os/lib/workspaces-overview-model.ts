import type { WorkspacePayload, WorkspaceTreeNode, WorktreeNestEntry } from "@/systems/workspace";

/** Footer "New worktree" nav-row key (no worktree record behind it). */
export const WORKSPACES_MENU_CREATE_KEY = "__create";

/** Arrow-reachable rows of the focused workspace's worktree menu. */
interface WorkspacesMenuWorktreeNavRow {
  key: string;
  kind: "worktree";
  entry: WorktreeNestEntry;
}

interface WorkspacesMenuCreateNavRow {
  key: typeof WORKSPACES_MENU_CREATE_KEY;
  kind: "create";
  entry: null;
}

export type WorkspacesMenuNavRow = WorkspacesMenuWorktreeNavRow | WorkspacesMenuCreateNavRow;

export interface OsWorkspacesWorktreeMenuModel {
  node: WorkspaceTreeNode<WorkspacePayload>;
  /** Complete locked-order list; compact host menus own truncation. */
  visible: WorktreeNestEntry[];
  /** Arrow-reachable rows in render order (inert rows excluded). */
  navRows: WorkspacesMenuNavRow[];
}

/**
 * Derives the focused workspace's menu model. Absence rules are the caller's
 * render gate: `null` means render nothing at all (non-git — absent, never
 * disabled); an empty `visible` with a create row means the lone button.
 */
export function buildWorkspacesWorktreeMenuModel(
  node: WorkspaceTreeNode<WorkspacePayload> | null | undefined,
  canCreate: boolean
): OsWorkspacesWorktreeMenuModel | null {
  if (!node?.gitBacked) return null;
  const visible = node.worktrees;
  const navRows: WorkspacesMenuNavRow[] = [];
  for (const entry of visible) {
    if (entry.selectable) navRows.push({ key: entry.key, kind: "worktree", entry });
  }
  if (canCreate) {
    navRows.push({ key: WORKSPACES_MENU_CREATE_KEY, kind: "create", entry: null });
  }
  return { node, visible, navRows };
}
