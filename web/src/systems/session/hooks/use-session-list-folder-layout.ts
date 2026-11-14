import { useState } from "react";

import {
  buildSessionListSections,
  sectionsVisibleThreads,
  type SessionListSections,
} from "../lib/session-folders";
import type { SessionThreadModel } from "../lib/session-list-threads";
import type { SessionListViewModel } from "./use-session-list-view";
import { useSessionListDnd } from "./use-session-list-dnd";
import { useSessionSidebarState } from "./use-session-sidebar-state";

/** A folder as its dialogs need it: a session folder by default, or a project folder. */
type DialogFolder = { id: string; name: string };

/** Which folder dialog is open, if any. */
export type SessionFolderDialogState<
  F extends DialogFolder = NonNullable<SessionListViewModel["layout"]>["folders"][number],
> =
  /** `moveCount` set: the folder is created around the selection, which moves into it. */
  | { kind: "create"; moveCount?: number }
  | { kind: "rename"; folder: F }
  | { kind: "delete"; folder: F }
  | null;

/** What one folder-capable query contributes: its layout (if one pair), order, and page. */
export type SessionListFolderSource = Pick<
  SessionListViewModel,
  "layout" | "sort" | "shelf" | "worktreeId" | "catalog"
>;

/**
 * Folder presentation for one session query: the sections built from the
 * served threads, persisted folder collapse, drag wiring, and the open folder
 * dialog. Only a query that resolves to one (workspace, profile) pair carries
 * a layout; any other query keeps rows flat (BR-11).
 */
export function useSessionListFolderLayout(
  view: SessionListFolderSource,
  threads: readonly SessionThreadModel[],
  filtering: boolean
) {
  const layout = view.layout;
  const { folderFolds: collapsedFolders, toggleFolder } = useSessionSidebarState();
  const sections: SessionListSections | null = layout
    ? buildSessionListSections(threads, layout.folders, {
        filtering,
        shelved: view.shelf !== "active",
      })
    : null;
  // A reorder pins the listed rows ahead of every container member the request
  // omits. Rows hidden by a filter, a worktree scope, or a page boundary would be
  // pushed behind the visible ones, so manual reorder needs the whole container on
  // screen: unfiltered, unscoped, and the first settled catalog page. The archive
  // and Later are served in activity order rather than manual order (#1716), so
  // their rows carry no order verbs.
  const orderable =
    view.sort === "manual" &&
    view.shelf === "active" &&
    !filtering &&
    !view.worktreeId &&
    !view.catalog?.previous &&
    !view.catalog?.paging;
  const dnd = useSessionListDnd(layout, {
    manual: view.sort === "manual",
    reorderable: orderable,
  });
  const [dialog, setDialog] = useState<SessionFolderDialogState>(null);
  return {
    layout,
    sections,
    collapsedFolders,
    toggleFolder,
    /** Drag reorder and Move up/down: only when every member of a container is listed. */
    orderable,
    /** Threads on screen in order; bulk selection and range picks follow it. */
    visibleThreads: sections
      ? sectionsVisibleThreads(sections, collapsedFolders, filtering)
      : threads,
    dnd,
    dialog,
    setDialog,
  };
}
