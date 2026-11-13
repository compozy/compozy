/**
 * The one project menu every project surface opens, right-click or `…`: New ▸, Open files,
 * Move ▸, then Archive and Remove last. A surface passes the verbs it can reach; a verb it
 * cannot perform is absent, and the tidy pass drops the groups that leaves empty.
 */
import {
  Archive,
  ArchiveRestore,
  ArrowDown,
  ArrowUp,
  Bot,
  Folder,
  FolderInput,
  FolderOpen,
  FolderOutput,
  FolderPlus,
  Plus,
  Trash2,
  type LucideIcon,
} from "@compozy/ui/icons";
import { tidyMenuTree, type MenuTreeNode } from "@compozy/ui";

import type { WorkspacePayload } from "../types";

export type ProjectMenuAction = "archive" | "remove";

/** Where a project sits among its profile's project folders, and how it moves. */
export interface ProjectMenuMove {
  /** Every folder of the layout; the one holding the project is drawn checked and disabled. */
  folders: readonly { id: string; name: string }[];
  /** The folder holding the project; `null` at the root. */
  folderId: string | null;
  onMove: (folderId: string | null) => void;
  /**
   * Present where the surface shows the project in manual order. A missing step is the end
   * of the project's container, drawn disabled.
   */
  order?: { up?: () => void; down?: () => void };
  /** Test id prefix of the move rows; defaults to the menu's. */
  testId?: string;
}

/** The verbs a surface reaches for one project; each absent verb is hidden. */
export interface ProjectMenuVerbs {
  newSession?: () => void;
  newFolder?: () => void;
  addAlwaysOn?: () => void;
  openFiles?: () => void;
  move?: ProjectMenuMove;
}

/** `over`'s verbs where it has them, `base`'s otherwise. */
export function mergeProjectMenuVerbs(
  base: ProjectMenuVerbs | undefined,
  over: ProjectMenuVerbs | undefined
): ProjectMenuVerbs {
  return {
    newSession: over?.newSession ?? base?.newSession,
    newFolder: over?.newFolder ?? base?.newFolder,
    addAlwaysOn: over?.addAlwaysOn ?? base?.addAlwaysOn,
    openFiles: over?.openFiles ?? base?.openFiles,
    move: over?.move ?? base?.move,
  };
}

/** Whether any verb is reachable, so a surface without a project still has a menu. */
export function hasProjectMenuVerbs(verbs: ProjectMenuVerbs): boolean {
  return Object.values(verbs).some(verb => verb !== undefined);
}

/**
 * The project menu tree. `workspace` is the project behind the surface; without one (the User
 * group in project folders) only the surface's own verbs remain. An archived project offers
 * Unarchive and Remove only: it is out of navigation, so nothing starts or moves in it.
 */
export function projectMenuTree({
  workspace,
  verbs,
  choose,
  testId = `project-menu-${workspace?.id ?? "none"}`,
}: {
  workspace: Pick<WorkspacePayload, "id" | "archived_at"> | undefined;
  verbs: ProjectMenuVerbs;
  choose: (action: ProjectMenuAction) => void;
  testId?: string;
}): MenuTreeNode[] {
  const archived = Boolean(workspace?.archived_at);
  const live: ProjectMenuVerbs = archived ? {} : verbs;
  return tidyMenuTree([
    {
      kind: "sub",
      id: "new",
      label: "New",
      icon: Plus,
      testId: `${testId}-new`,
      children: [
        item("new-session", "New session", Plus, live.newSession, `${testId}-new-session`),
        item("new-folder", "New folder", FolderPlus, live.newFolder, `${testId}-new-folder`),
        item(
          "add-always-on",
          "Add always-on agent",
          Bot,
          live.addAlwaysOn,
          `${testId}-add-always-on`
        ),
      ].filter(isNode),
    },
    ...[
      item("open-files", "Open files", FolderOpen, live.openFiles, `${testId}-open-files`),
    ].filter(isNode),
    { kind: "separator", id: "s-move" },
    ...(live.move ? [moveSubmenu(live.move, live.move.testId ?? testId)] : []),
    { kind: "separator", id: "s-lifecycle" },
    ...(workspace ? lifecycleItems(archived, choose, testId) : []),
  ]);
}

function moveSubmenu(move: ProjectMenuMove, testId: string): MenuTreeNode {
  // The current folder is checked, not hidden, as in the session row's Move ▸.
  const folderRows: MenuTreeNode[] = move.folders.map(folder => {
    const current = folder.id === move.folderId;
    return {
      kind: "item",
      id: `folder-${folder.id}`,
      label: folder.name,
      icon: Folder,
      checked: current,
      disabled: current,
      testId: `${testId}-move-to-${folder.id}`,
      onSelect: () => move.onMove(folder.id),
    };
  });
  if (move.folderId !== null) {
    folderRows.push({
      kind: "item",
      id: "move-out",
      label: "Move out of folder",
      icon: FolderOutput,
      testId: `${testId}-move-out`,
      onSelect: () => move.onMove(null),
    });
  }
  const order: MenuTreeNode[] = move.order
    ? [
        orderItem("up", "Move project up", ArrowUp, move.order.up, `${testId}-up`),
        orderItem("down", "Move project down", ArrowDown, move.order.down, `${testId}-down`),
      ]
    : [];
  return {
    kind: "sub",
    id: "move",
    label: "Move",
    icon: FolderInput,
    testId: `${testId}-move-to`,
    children: [
      { kind: "group", id: "folder", label: "Folder", children: folderRows },
      { kind: "separator", id: "s-order" },
      ...order,
    ],
  };
}

function lifecycleItems(
  archived: boolean,
  choose: (action: ProjectMenuAction) => void,
  testId: string
): MenuTreeNode[] {
  return [
    {
      kind: "item",
      id: "archive",
      label: archived ? "Unarchive project" : "Archive project",
      icon: archived ? ArchiveRestore : Archive,
      testId: `${testId}-archive`,
      onSelect: () => choose("archive"),
    },
    {
      kind: "item",
      id: "remove",
      label: "Remove project",
      icon: Trash2,
      destructive: true,
      testId: `${testId}-remove`,
      onSelect: () => choose("remove"),
    },
  ];
}

function item(
  id: string,
  label: string,
  icon: LucideIcon,
  onSelect: (() => void) | undefined,
  testId: string
): MenuTreeNode | null {
  return onSelect ? { kind: "item", id, label, icon, testId, onSelect } : null;
}

function orderItem(
  id: string,
  label: string,
  icon: LucideIcon,
  onSelect: (() => void) | undefined,
  testId: string
): MenuTreeNode {
  return { kind: "item", id, label, icon, testId, disabled: onSelect === undefined, onSelect };
}

function isNode(node: MenuTreeNode | null): node is MenuTreeNode {
  return node !== null;
}
