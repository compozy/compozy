import { useState } from "react";

import {
  buildSessionListSections,
  canGroupSessionThreads,
  sectionsVisibleThreads,
  type SessionListSections,
} from "../lib/session-folders";
import type { SessionThreadModel } from "../lib/session-list-threads";
import type { SessionListViewModel } from "./use-session-list-view";
import { useSessionListDnd } from "./use-session-list-dnd";
import { useSessionSidebarState } from "./use-session-sidebar-state";

/** Which folder dialog is open, if any. */
export type SessionFolderDialogState =
  | { kind: "create" }
  | { kind: "rename"; folder: NonNullable<SessionListViewModel["layout"]>["folders"][number] }
  | { kind: "delete"; folder: NonNullable<SessionListViewModel["layout"]>["folders"][number] }
  | null;

/**
 * Folder presentation for one session list: the sections built from the served
 * threads, persisted folder collapse, drag wiring, and the open folder dialog.
 * The widest scope keeps rows flat, so it carries no layout.
 */
export function useSessionListFolderLayout(
  view: SessionListViewModel,
  threads: readonly SessionThreadModel[],
  filtering: boolean
) {
  const layout = view.scope === "all-workspaces" ? undefined : view.layout;
  const { collapsedFolderIds, toggleFolder } = useSessionSidebarState();
  const collapsedFolders = new Set(collapsedFolderIds);
  const sections: SessionListSections | null = layout
    ? buildSessionListSections(threads, layout.folders, { filtering, archived: view.archived })
    : null;
  // A reorder pins the listed rows ahead of every container member the request
  // omits. Rows hidden by a filter, a worktree scope, or a page boundary would be
  // pushed behind the visible ones, so manual reorder needs the whole container on
  // screen: unfiltered, unscoped, and the first settled catalog page. Archived
  // members never render alongside active ones, so the archive split keeps each
  // list's own relative order.
  const orderable =
    view.sort === "manual" &&
    !filtering &&
    !view.worktreeId &&
    !view.catalog?.previous &&
    !view.catalog?.paging;
  const dnd = useSessionListDnd(layout, {
    manual: view.sort === "manual",
    reorderable: orderable,
  });
  const groups = canGroupSessionThreads(threads, view.profileFolderGroups)
    ? view.profileFolderGroups
    : undefined;
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
      : groups?.length
        ? groups.flatMap(group => {
            const owned = threads.filter(thread => thread.session.profile_id === group.owner.id);
            return group.folders
              ? sectionsVisibleThreads(
                  buildSessionListSections(owned, group.folders, {
                    filtering,
                    archived: view.archived,
                  }),
                  collapsedFolders,
                  filtering
                )
              : owned;
          })
        : threads,
    dnd,
    dialog,
    setDialog,
  };
}
