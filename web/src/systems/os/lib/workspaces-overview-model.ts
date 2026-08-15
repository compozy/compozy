import type { WorkspacesMenuNavRow } from "../hooks/use-workspaces-switcher";
import {
  truncateWorktreeNest,
  type WorkspacePayload,
  type WorkspaceTreeNode,
  type WorktreeNestEntry,
} from "@/systems/workspace";

/** Footer "New worktree" nav-row key (no worktree record behind it). */
export const WORKSPACES_MENU_CREATE_KEY = "__create";

export interface OsWorkspacesWorktreeMenuModel {
  node: WorkspaceTreeNode<WorkspacePayload>;
  /** Locked truncation applied; footer create row follows. */
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
  const { visible } = truncateWorktreeNest(node.worktrees);
  const navRows: WorkspacesMenuNavRow[] = [];
  for (const entry of visible) {
    if (entry.selectable) navRows.push({ key: entry.key, kind: "worktree", entry });
  }
  if (canCreate) {
    navRows.push({ key: WORKSPACES_MENU_CREATE_KEY, kind: "create", entry: null });
  }
  return { node, visible, navRows };
}
