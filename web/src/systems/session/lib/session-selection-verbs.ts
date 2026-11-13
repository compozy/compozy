/**
 * What a selection of sessions can do, as the selection menu (`selectionMenuTree`) reads it. The
 * list's bulk selection hooks build these; the selection bar, a selected row's or card's
 * right-click and the Kanban selection hand them on.
 */
import type { SessionFolderPayload, SessionPayload } from "../types";

/** The selection menu's Folder group under Move ▸. */
export interface SessionBulkFolderMenu {
  folders: readonly SessionFolderPayload[];
  /** Why the selection cannot move, shown inside the submenu instead of targets. */
  blockedReason: string | null;
  /** The folder every movable session already sits in; moving there changes nothing. */
  sharedFolderId: string | null | undefined;
  /** At least one movable session sits in a folder, so No folder applies. */
  canUnfile: boolean;
  onMove: (folderId: string | null) => void;
  onNewFolder: () => void;
}

/** What the selection can do, shared by the selection bar's menu and a selected row's context menu. */
export interface SessionSelectionVerbs {
  count: number;
  markSeenable: number;
  topLevelable: number;
  stoppable: number;
  archivable: number;
  /** Archivable sessions that archiving stops first. */
  archivableLive: number;
  unarchivable: number;
  /** Top-level sessions not yet in Later. */
  laterable: number;
  /** Top-level sessions in Later. */
  backFromLaterable: number;
  disabled: boolean;
  onDelete?: () => void;
  onStop?: () => void;
  onArchive?: () => void;
  onUnarchive?: () => void;
  onMoveToLater?: () => void;
  onBackFromLater?: () => void;
  onCopyIds: () => void;
  onCopyTitles: () => void;
  onMarkSeen?: () => void;
  onMakeTopLevel?: () => void;
  /** Selected sessions with a branch, for Copy branch names. */
  branches: number;
  onCopyBranches: () => void;
  /** Nest under: the listed sessions some selected session can move under; absent without a host verb. */
  nest?: SessionSelectionNest;
  /** Move to folder; absent while the list has no folders to move into. */
  folders?: SessionBulkFolderMenu;
}

export interface SessionSelectionNest {
  parents: readonly SessionPayload[];
  /** Selected sessions that can move under at least one of the parents. */
  nestable: number;
  onNest: (parent: SessionPayload) => void;
}
