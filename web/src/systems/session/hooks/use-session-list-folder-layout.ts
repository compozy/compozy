import { useState } from "react";

import { isRemoteComputerId } from "@/lib/computer-scope";

import {
  buildSessionListSections,
  sectionsVisibleThreads,
  type SessionListSections,
} from "../lib/session-folders";
import { folderComputerFoldKey, type SessionFoldSet } from "../lib/session-sidebar-scope";
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
  "layout" | "sort" | "shelf" | "worktreeId" | "catalog" | "partitions" | "folderOf" | "computerId"
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
  const { folderFolds: rawCollapsedFolders, toggleFolder: rawToggleFolder } =
    useSessionSidebarState();
  const remote = isRemoteComputerId(view.computerId);
  const collapsedFolders: SessionFoldSet = remote
    ? { has: id => rawCollapsedFolders.has(folderComputerFoldKey(view.computerId!, id)) }
    : rawCollapsedFolders;
  const toggleFolder = remote
    ? (id: string) => rawToggleFolder(folderComputerFoldKey(view.computerId!, id))
    : rawToggleFolder;
  const loadedFolderFolds: SessionFoldSet = {
    has: id => view.partitions?.get(id)?.countOnly ?? (!filtering && collapsedFolders.has(id)),
  };
  const sections: SessionListSections | null = layout
    ? buildSessionListSections(threads, layout.folders, {
        filtering,
        shelved: view.shelf !== "active",
        folderOf: view.folderOf,
        showFolder: folder => {
          const page = view.partitions?.get(folder);
          return page !== undefined && (page.loading || page.failed || (page.total ?? 0) > 0);
        },
      })
    : null;
  const orderable =
    view.sort === "manual" &&
    view.shelf === "active" &&
    !filtering &&
    !view.worktreeId &&
    (view.partitions !== undefined ||
      (!view.catalog?.previous && !view.catalog?.next && !view.catalog?.paging));
  const dnd = useSessionListDnd(layout, {
    manual: view.sort === "manual",
    reorderable: orderable,
    computerId: view.computerId,
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
    visibleThreads: sections ? sectionsVisibleThreads(sections, loadedFolderFolds, false) : threads,
    dnd,
    dialog,
    setDialog,
  };
}
